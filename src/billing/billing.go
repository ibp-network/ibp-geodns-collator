package billing

// ─────────────────────────────────────────────────────────────────────────────
//  Stake Plus Inc. – IBP GeoDNS / IBPCollator – Billing subsystem
// ─────────────────────────────────────────────────────────────────────────────

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	cfg "github.com/ibp-network/ibp-geodns-libs/config"
	log "github.com/ibp-network/ibp-geodns-libs/logging"
)

// ─────────────────────────────────────────────────────────────────────────────
//  Public structures and accessors
// ─────────────────────────────────────────────────────────────────────────────

// MemberCost houses the breakdown of costs that *one* member incurs.
type MemberCost struct {
	MemberName   string
	ServiceCosts map[string]float64 // serviceName → $ cost
	Total        float64
}

// ServiceCost houses the breakdown of costs *per service* across all members.
type ServiceCost struct {
	ServiceName string
	MemberCosts map[string]float64 // memberName → $ cost
	Total       float64
}

// Summary keeps both perspectives together (no mutex — read-only snapshot).
type Summary struct {
	Members  map[string]MemberCost
	Services map[string]ServiceCost
	Refresh  time.Time
}

// internal store guarded by a mutex
var billingStore struct {
	sync.RWMutex
	Summary
}

// Track last generated billing month to avoid duplicates
var (
	lastGeneratedBillingMonth   time.Time
	billingGenMutex             sync.Mutex
	billingGenerationInProgress bool
)

// marker file to persist last generated month across restarts
const billingMarkerFile = "billing_last_generated.marker"

// GetSummary returns a deep-copy of the current billing snapshot.
func GetSummary() Summary {
	billingStore.RLock()
	defer billingStore.RUnlock()

	// deep copy to ensure immutability
	mCopy := make(map[string]MemberCost, len(billingStore.Members))
	for k, v := range billingStore.Members {
		svcCopy := make(map[string]float64, len(v.ServiceCosts))
		for sk, sv := range v.ServiceCosts {
			svcCopy[sk] = sv
		}
		mCopy[k] = MemberCost{MemberName: v.MemberName, ServiceCosts: svcCopy, Total: v.Total}
	}

	sCopy := make(map[string]ServiceCost, len(billingStore.Services))
	for k, v := range billingStore.Services {
		memCopy := make(map[string]float64, len(v.MemberCosts))
		for mk, mv := range v.MemberCosts {
			memCopy[mk] = mv
		}
		sCopy[k] = ServiceCost{ServiceName: v.ServiceName, MemberCosts: memCopy, Total: v.Total}
	}

	return Summary{Members: mCopy, Services: sCopy, Refresh: billingStore.Refresh}
}

// ─────────────────────────────────────────────────────────────────────────────
//  Initialisation
// ─────────────────────────────────────────────────────────────────────────────

// Init kicks off periodic billing refreshes and monthly billing PDF generation.
func Init() {
	// synchronous first refresh with verbose output
	refresh(true)

	loadLastGeneratedMonthMarker()

	// hourly refresh (top of the hour, UTC)
	go func() {
		for {
			next := time.Now().UTC().Truncate(time.Hour).Add(time.Hour)
			time.Sleep(time.Until(next))
			refresh(false)
		}
	}()

	// Daily service cost PDF generation at 00:05 UTC
	go func() {
		for {
			next := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour).Add(5 * time.Minute)
			time.Sleep(time.Until(next))
			generateServiceCostPDF()
		}
	}()

	// Monthly member billing PDF generation
	go func() {
		for {
			now := time.Now().UTC()
			nextMonth := nextMonthlyBillingRun(now)
			waitDuration := time.Until(nextMonth)
			log.Log(log.Info, "[billing] Next member billing PDF generation scheduled for %s (in %v)",
				nextMonth.Format("2006-01-02 15:04:05"), waitDuration)

			time.Sleep(waitDuration)
			generateMonthlyBillingPDF()
		}
	}()

	// Generate initial PDFs if we haven't generated for the previous month yet
	go func() {
		time.Sleep(5 * time.Second) // Reduced delay since DB is now ready

		// Check if we need to generate last month's billing
		now := time.Now().UTC()
		lastMonth := now.AddDate(0, -1, 0)
		lastMonthStart := time.Date(lastMonth.Year(), lastMonth.Month(), 1, 0, 0, 0, 0, time.UTC)

		billingGenMutex.Lock()
		needsGeneration := lastGeneratedBillingMonth.Before(lastMonthStart)
		billingGenMutex.Unlock()

		if needsGeneration {
			log.Log(log.Info, "[billing] Generating initial member billing PDF for previous month")
			generateMonthlyBillingPDF()
		}

		// Always generate current service cost PDF
		generateServiceCostPDF()
	}()
}

// ─────────────────────────────────────────────────────────────────────────────
//  Refresh logic
// ─────────────────────────────────────────────────────────────────────────────

func refresh(verbose bool) {
	start := time.Now()
	c := cfg.GetConfig()

	newMemberCosts := make(map[string]MemberCost)
	newServiceCosts := make(map[string]ServiceCost)
	missingPricing := []string{}

	// indices (case-insensitive)
	svcByName := make(map[string]cfg.Service)
	for n, s := range c.Services {
		svcByName[strings.ToLower(strings.TrimSpace(n))] = s
	}

	priceByRegion := make(map[string]cfg.IaasPricing)
	for r, p := range c.Pricing {
		priceByRegion[strings.ToLower(strings.TrimSpace(r))] = p
	}

	for memName, mem := range c.Members {
		regionKey := strings.ToLower(strings.TrimSpace(mem.Location.Region))
		price, ok := priceByRegion[regionKey]
		if !ok {
			if def, okDef := priceByRegion["default"]; okDef {
				price = def
				log.Log(log.Warn, "[billing] region %q has no pricing entry — member %s falling back to default pricing", mem.Location.Region, memName)
			} else {
				log.Log(log.Error, "[billing] region %q has no pricing entry and no default — member %s skipped", mem.Location.Region, memName)
				missingPricing = append(missingPricing, memName)
				continue
			}
		}

		// Skip members that are disabled or explicitly overridden
		if mem.Service.Active != 1 {
			log.Log(log.Debug, "[billing] skipping inactive member %s", memName)
			continue
		}
		if mem.Override {
			log.Log(log.Debug, "[billing] skipping override member %s", memName)
			continue
		}

		memCost := MemberCost{
			MemberName:   memName,
			ServiceCosts: map[string]float64{},
		}

		for _, svcList := range mem.ServiceAssignments {
			for _, svcName := range svcList {
				svc, exists := svcByName[strings.ToLower(strings.TrimSpace(svcName))]
				if !exists {
					log.Log(log.Warn, "[billing] unknown service %q referenced by member %s — skipped", svcName, memName)
					continue
				}

				if svc.Configuration.Active != 1 {
					log.Log(log.Debug, "[billing] skipping inactive service %q for member %s", svcName, memName)
					continue
				}

				cost := costForServiceInstance(svc.Resources, price)
				memCost.ServiceCosts[svcName] += cost
				memCost.Total += cost

				sc := newServiceCosts[svcName]
				if sc.ServiceName == "" {
					sc.ServiceName = svcName
					sc.MemberCosts = map[string]float64{}
				}
				sc.MemberCosts[memName] += cost
				sc.Total += cost
				newServiceCosts[svcName] = sc
			}
		}

		if memCost.Total > 0 {
			newMemberCosts[memName] = memCost
		}
	}

	// If any member is missing pricing (and no default), abort publish to avoid silent underbilling.
	if len(missingPricing) > 0 {
		log.Log(log.Error, "[billing] refresh aborted; missing pricing for members: %v", missingPricing)
		return
	}

	// publish atomically
	billingStore.Lock()
	billingStore.Members = newMemberCosts
	billingStore.Services = newServiceCosts
	billingStore.Refresh = time.Now().UTC()
	billingStore.Unlock()

	duration := time.Since(start).Round(time.Millisecond)
	log.Log(log.Info, "[billing] refresh complete — %d members, %d services, in %s",
		len(newMemberCosts), len(newServiceCosts), duration)

	if verbose {
		logDetails(newMemberCosts, newServiceCosts)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
//  PDF Generation
// ─────────────────────────────────────────────────────────────────────────────

func generateServiceCostPDF() {
	tmpDir, err := ensureTempDir()
	if err != nil {
		log.Log(log.Warn, "[billing] tmp directory unavailable — service cost PDF skipped: %v", err)
		return
	}
	if tmpDir == "" {
		log.Log(log.Warn, "[billing] tmp directory not configured — service cost PDF skipped")
		return
	}

	snap := GetSummary()
	if err := writeServiceCostPDF(&snap, tmpDir); err != nil {
		log.Log(log.Error, "[billing] failed to write service-cost PDF: %v", err)
	}
}

func generateMonthlyBillingPDF() {
	// Get the previous month
	now := time.Now().UTC()
	previousMonth := now.AddDate(0, -1, 0)
	billingMonth := time.Date(previousMonth.Year(), previousMonth.Month(), 1, 0, 0, 0, 0, time.UTC)

	// Optional gate for multi-node deployments without shared storage
	if os.Getenv("BILLING_GENERATOR_ENABLED") == "0" {
		log.Log(log.Info, "[billing] BILLING_GENERATOR_ENABLED=0; skipping monthly billing generation")
		return
	}
	if os.Getenv("BILLING_MULTI_NODE") == "1" && os.Getenv("BILLING_LOCK_SHARED_PATH") == "" {
		log.Log(log.Warn, "[billing] BILLING_MULTI_NODE=1 but BILLING_LOCK_SHARED_PATH unset; skipping to avoid duplicate generation")
		return
	}

	// Check if we've already generated for this month
	billingGenMutex.Lock()
	if !lastGeneratedBillingMonth.Before(billingMonth) {
		billingGenMutex.Unlock()
		log.Log(log.Info, "[billing] Member billing PDF already generated for %s", billingMonth.Format("January 2006"))
		return
	}
	if billingGenerationInProgress {
		billingGenMutex.Unlock()
		log.Log(log.Info, "[billing] Member billing PDF generation already in progress for %s", billingMonth.Format("January 2006"))
		return
	}
	billingGenerationInProgress = true
	billingGenMutex.Unlock()

	success := false
	defer func() {
		billingGenMutex.Lock()
		billingGenerationInProgress = false
		if success && lastGeneratedBillingMonth.Before(billingMonth) {
			lastGeneratedBillingMonth = billingMonth
		}
		billingGenMutex.Unlock()
	}()

	log.Log(log.Info, "[billing] Starting member billing PDF generation for %s", billingMonth.Format("January 2006"))

	tmpDir, err := ensureTempDir()
	if err != nil {
		log.Log(log.Warn, "[billing] tmp directory unavailable — member billing PDF skipped: %v", err)
		return
	}
	if tmpDir == "" {
		log.Log(log.Warn, "[billing] tmp directory not configured — member billing PDF skipped")
		return
	}

	// Create month directory (YYYY-MM format)
	monthDir := filepath.Join(tmpDir, billingMonth.Format("2006-01"))
	if err := os.MkdirAll(monthDir, 0755); err != nil {
		log.Log(log.Error, "[billing] Failed to create month directory: %v", err)
		return
	}

	// Acquire a simple lock file to avoid duplicate generation on shared storage.
	lockBase := os.Getenv("BILLING_LOCK_SHARED_PATH")
	if lockBase == "" {
		lockBase = monthDir
	}
	lockFile := filepath.Join(lockBase, "billing.lock")
	lockFd, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		// If lock exists, check staleness and retry once if stale.
		if info, statErr := os.Stat(lockFile); statErr == nil {
			if time.Since(info.ModTime()) > time.Minute {
				log.Log(log.Warn, "[billing] stale lock detected for %s, removing", billingMonth.Format("January 2006"))
				_ = os.Remove(lockFile)
				lockFd, err = os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL, 0644)
			}
		}
	}
	if err != nil {
		log.Log(log.Info, "[billing] another node is generating PDFs for %s; skipping this run", billingMonth.Format("January 2006"))
		return
	}
	lockFd.Close()
	defer os.Remove(lockFile)

	snap := GetSummary()

	// If PDFs already exist for all members and overview, mark as generated and exit.
	if monthComplete(monthDir, billingMonth, &snap) {
		log.Log(log.Info, "[billing] Monthly PDFs already complete for %s — marking done", billingMonth.Format("January 2006"))
		success = true
		if err := writeLastGeneratedMonthMarker(billingMonth); err != nil {
			log.Log(log.Warn, "[billing] failed to write billing marker after detecting complete set: %v", err)
		}
		return
	}

	// Calculate SLA for the billing month
	sla, err := CalculateSLAAdjustments(billingMonth, &snap)
	if err != nil {
		log.Log(log.Error, "[billing] failed SLA calculation: %v", err)
		log.Log(log.Warn, "[billing] Monthly billing generation for %s skipped until SLA data can be calculated", billingMonth.Format("January 2006"))
		return
	}

	// Log members not meeting SLA
	violationCount := 0
	for memberName, services := range sla {
		for serviceName, breakdown := range services {
			if !breakdown.MeetsSLA {
				violationCount++
				log.Log(log.Warn, "[billing] SLA VIOLATION: %s / %s - Uptime: %.2f%% (Required: %.2f%%), Down: %.2f hrs",
					memberName, serviceName, breakdown.Uptime, breakdown.SLAThreshold, breakdown.HoursDown)
			}
		}
	}

	if violationCount == 0 {
		log.Log(log.Info, "[billing] No SLA violations detected for %s", billingMonth.Format("January 2006"))
	} else {
		log.Log(log.Info, "[billing] Total SLA violations for %s: %d", billingMonth.Format("January 2006"), violationCount)
	}

	// Generate the monthly overview PDF
	hadError := false
	if err := writeMonthlyOverviewPDF(&snap, sla, monthDir, billingMonth); err != nil {
		hadError = true
		log.Log(log.Error, "[billing] failed to write monthly overview PDF: %v", err)
	}

	// Generate individual member PDFs
	for memberName := range snap.Members {
		if err := writeMemberPDF(memberName, &snap, sla, monthDir, billingMonth); err != nil {
			hadError = true
			log.Log(log.Error, "[billing] failed to write member PDF for %s: %v", memberName, err)
		}
	}

	if hadError {
		log.Log(log.Warn, "[billing] Monthly billing generation for %s completed with errors; will retry on next run", billingMonth.Format("January 2006"))
		return
	}

	success = true
	if err := writeLastGeneratedMonthMarker(billingMonth); err != nil {
		log.Log(log.Warn, "[billing] failed to write billing marker: %v", err)
	}
	log.Log(log.Info, "[billing] Monthly billing generation completed for %s", billingMonth.Format("January 2006"))
}

// ─────────────────────────────────────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────────────────────────────────────

func costForServiceInstance(res cfg.Resources, price cfg.IaasPricing) float64 {
	if res.Nodes == 0 {
		return 0
	}
	perNode := (float64(res.Cores) * price.Cores) +
		(float64(res.Memory) * price.Memory) +
		(float64(res.Disk) * price.Disk) +
		(float64(res.Bandwidth) * price.Bandwidth)
	return perNode * float64(res.Nodes)
}

func logDetails(memCosts map[string]MemberCost, svcCosts map[string]ServiceCost) {
	log.Log(log.Info, "[billing] ---------------------- per member cost breakdown ----------------------")
	memberNames := make([]string, 0, len(memCosts))
	for n := range memCosts {
		memberNames = append(memberNames, n)
	}
	sort.Strings(memberNames)

	for _, m := range memberNames {
		mc := memCosts[m]
		log.Log(log.Info, "[billing] %s — $%.2f", mc.MemberName, mc.Total)
		svcNames := make([]string, 0, len(mc.ServiceCosts))
		for s := range mc.ServiceCosts {
			svcNames = append(svcNames, s)
		}
		sort.Strings(svcNames)
		for _, s := range svcNames {
			log.Log(log.Info, "[billing]   • %s — $%.2f", s, mc.ServiceCosts[s])
		}
	}

	log.Log(log.Info, "[billing] ---------------------- per service cost breakdown --------------------")
	serviceNames := make([]string, 0, len(svcCosts))
	for s := range svcCosts {
		serviceNames = append(serviceNames, s)
	}
	sort.Strings(serviceNames)

	for _, s := range serviceNames {
		sc := svcCosts[s]
		log.Log(log.Info, "[billing] %s — $%.2f", sc.ServiceName, sc.Total)
		memNames := make([]string, 0, len(sc.MemberCosts))
		for m := range sc.MemberCosts {
			memNames = append(memNames, m)
		}
		sort.Strings(memNames)
		for _, m := range memNames {
			log.Log(log.Info, "[billing]   • %s — $%.2f", m, sc.MemberCosts[m])
		}
	}
}

func resolveTempDir() string {
	c := cfg.GetConfig()
	return filepath.Join(c.Local.System.WorkDir, "tmp")
}

func ensureTempDir() (string, error) {
	tmpDir := resolveTempDir()
	if tmpDir == "" {
		return "", nil
	}
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}
	return tmpDir, nil
}

func nextMonthlyBillingRun(now time.Time) time.Time {
	now = now.UTC()
	currentMonthRun := time.Date(now.Year(), now.Month(), 1, 0, 5, 0, 0, time.UTC)
	if now.Day() == 1 && now.Before(currentMonthRun) {
		return currentMonthRun
	}
	return currentMonthRun.AddDate(0, 1, 0)
}

// loadLastGeneratedMonthMarker restores lastGeneratedBillingMonth from marker file (best-effort).
func loadLastGeneratedMonthMarker() {
	tmpDir := resolveTempDir()
	if tmpDir == "" {
		return
	}
	path := filepath.Join(tmpDir, billingMarkerFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err == nil {
		billingGenMutex.Lock()
		lastGeneratedBillingMonth = t
		billingGenMutex.Unlock()
	}
}

// writeLastGeneratedMonthMarker persists the last generated month to disk (best-effort).
func writeLastGeneratedMonthMarker(month time.Time) error {
	tmpDir := resolveTempDir()
	if tmpDir == "" {
		return fmt.Errorf("tmp dir not configured")
	}
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(tmpDir, billingMarkerFile)
	return os.WriteFile(path, []byte(month.Format(time.RFC3339)), 0644)
}

// monthComplete checks whether overview and all member PDFs already exist.
func monthComplete(monthDir string, month time.Time, snap *Summary) bool {
	overviewName := fmt.Sprintf("%d_%02d-Monthly_Overview.pdf", month.Year(), int(month.Month()))
	if _, err := os.Stat(filepath.Join(monthDir, overviewName)); err != nil {
		return false
	}
	for memberName := range snap.Members {
		memberNameSafe := sanitizeFilename(memberName)
		memberFile := fmt.Sprintf("%d_%02d-IBP-Service_%s.pdf", month.Year(), int(month.Month()), memberNameSafe)
		if _, err := os.Stat(filepath.Join(monthDir, memberFile)); err != nil {
			return false
		}
	}
	return true
}
