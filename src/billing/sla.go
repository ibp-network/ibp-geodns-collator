package billing

import (
	"fmt"
	"strings"
	"time"

	common "github.com/ibp-network/ibp-geodns-collator/src/common"
	cfg "github.com/ibp-network/ibp-geodns-libs/config"
	data2 "github.com/ibp-network/ibp-geodns-libs/data2"
	log "github.com/ibp-network/ibp-geodns-libs/logging"
)

// SLABreakdown captures the availability of a single <member,service> pair.
type SLABreakdown struct {
	HoursTotal   float64
	HoursDown    float64
	HoursUp      float64
	Uptime       float64 // 0-100 percentage
	SLAThreshold float64 // SLA threshold in percentage (e.g., 99.99)
	SLAHours     float64 // SLA threshold in hours
	MeetsSLA     bool
}

// SLASummary maps member → service → breakdown.
type SLASummary map[string]map[string]SLABreakdown

// Default SLA threshold
const DefaultSLAPercentage = 99.99

// downtimePeriod represents a period of downtime
type downtimePeriod struct {
	start time.Time
	end   time.Time
}

// CalculateSLAAdjustments calculates actual uptime from the member_events table
func CalculateSLAAdjustments(month time.Time, sum *Summary) (SLASummary, error) {
	out := make(SLASummary)

	// Check if database is initialized
	if data2.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Calculate the time range for the month
	startTime := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	endTime := startTime.AddDate(0, 1, 0).Add(-time.Nanosecond) // Last nanosecond of the month

	// Total hours in the month
	totalHours := endTime.Sub(startTime).Hours()
	slaHours := totalHours * (DefaultSLAPercentage / 100.0)

	// Get configuration for member name mapping
	c := cfg.GetConfig()

	// Build member ID to DB name mapping
	memberIDToDBName := make(map[string]string)
	for id, member := range c.Members {
		if member.Details.Name != "" {
			memberIDToDBName[id] = member.Details.Name
		} else {
			memberIDToDBName[id] = id
		}
	}

	// Build service to domains mapping (active services only)
	serviceToDomains := make(map[string][]string)
	for svcName, svc := range c.Services {
		if svc.Configuration.Active != 1 {
			continue
		}
		domains := []string{}
		for _, provider := range svc.Providers {
			for _, rpcUrl := range provider.RpcUrls {
				if domain := extractDomainFromURL(rpcUrl); domain != "" {
					domains = append(domains, domain)
				}
			}
		}
		serviceToDomains[svcName] = common.NormalizeHosts(domains)
	}

	// Calculate downtime for each member/service combination
	for memberID, m := range sum.Members {
		if _, ok := out[memberID]; !ok {
			out[memberID] = make(map[string]SLABreakdown)
		}

		dbMemberName := memberIDToDBName[memberID]

		for svcKey := range m.ServiceCosts {
			svcConfig, svcExists := c.Services[svcKey]
			if !svcExists || svcConfig.Configuration.Active != 1 {
				continue
			}

			// Calculate downtime for this specific service
			downtime := calculateServiceDowntime(dbMemberName, svcKey, serviceToDomains[svcKey], startTime, endTime)

			// Calculate uptime
			uptime := totalHours - downtime
			if uptime < 0 {
				uptime = 0
			}

			uptimePercent := 100.0
			if totalHours > 0 {
				uptimePercent = (uptime / totalHours) * 100.0
			}

			meetsSLA := uptimePercent >= DefaultSLAPercentage

			out[memberID][svcKey] = SLABreakdown{
				HoursTotal:   totalHours,
				HoursDown:    downtime,
				HoursUp:      uptime,
				Uptime:       uptimePercent,
				SLAThreshold: DefaultSLAPercentage,
				SLAHours:     slaHours,
				MeetsSLA:     meetsSLA,
			}

			if downtime > 0 {
				log.Log(log.Info, "[SLA] %s/%s - Total downtime: %.2f hours (%.2f%% uptime)",
					memberID, svcKey, downtime, uptimePercent)
			}
		}
	}

	return out, nil
}

// calculateServiceDowntime calculates total downtime hours for a specific service
func calculateServiceDowntime(memberName, serviceName string, domains []string, startTime, endTime time.Time) float64 {
	if data2.DB == nil {
		return 0
	}

	// Collect all downtime periods
	allPeriods := []downtimePeriod{}

	// Query for site-level checks (affects all services)
	// This handles scenarios A, B, C, D, and E for site-level events
	siteQuery := `
		SELECT  
			start_time,
			end_time
		FROM member_events
		WHERE member_name = ?
		AND LOWER(check_type) IN ('site', '1')
		AND status = 0
		AND (
			-- Event starts before period and ends during or after period
			(start_time < ? AND (end_time IS NULL OR end_time > ?))
			OR
			-- Event starts during period
			(start_time >= ? AND start_time < ?)
		)
	`

	rows, err := data2.DB.Query(siteQuery, memberName, endTime, startTime, startTime, endTime)
	if err != nil {
		log.Log(log.Error, "[SLA] Failed to query site downtime: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var eventStart time.Time
			var eventEnd *time.Time

			if err := rows.Scan(&eventStart, &eventEnd); err != nil {
				log.Log(log.Error, "[SLA] Failed to scan site event: %v", err)
				continue
			}

			// Adjust times to month boundaries
			if eventStart.Before(startTime) {
				eventStart = startTime
			}

			actualEnd := endTime
			if eventEnd != nil && eventEnd.Before(endTime) {
				actualEnd = *eventEnd
			}

			allPeriods = append(allPeriods, downtimePeriod{start: eventStart, end: actualEnd})
		}
		if err := rows.Err(); err != nil {
			log.Log(log.Error, "[SLA] Site downtime row iteration failed: %v", err)
		}
	}

	// Query for domain/endpoint checks specific to this service
	if len(domains) > 0 {
		serviceQuery := `
			SELECT  
				check_type,
				domain_name,
				endpoint,
				start_time,
				end_time
			FROM member_events
			WHERE member_name = ?
			AND LOWER(check_type) IN ('domain', '2', 'endpoint', '3')
			AND status = 0
			AND (
				-- Event starts before period and ends during or after period
				(start_time < ? AND (end_time IS NULL OR end_time > ?))
				OR
				-- Event starts during period
				(start_time >= ? AND start_time < ?)
			)
		`

		rows, err := data2.DB.Query(serviceQuery, memberName, endTime, startTime, startTime, endTime)
		if err != nil {
			log.Log(log.Error, "[SLA] Failed to query service downtime: %v", err)
		} else {
			defer rows.Close()

			for rows.Next() {
				var checkType string
				var domainName, endpoint string
				var eventStart time.Time
				var eventEnd *time.Time

				if err := rows.Scan(&checkType, &domainName, &endpoint, &eventStart, &eventEnd); err != nil {
					log.Log(log.Error, "[SLA] Failed to scan service event: %v", err)
					continue
				}
				if !common.EventMatchesService(domainName, endpoint, domains) {
					continue
				}

				// Adjust times to month boundaries
				if eventStart.Before(startTime) {
					eventStart = startTime
				}

				actualEnd := endTime
				if eventEnd != nil && eventEnd.Before(endTime) {
					actualEnd = *eventEnd
				}

				allPeriods = append(allPeriods, downtimePeriod{start: eventStart, end: actualEnd})
			}
			if err := rows.Err(); err != nil {
				log.Log(log.Error, "[SLA] Service downtime row iteration failed: %v", err)
			}
		}
	}

	// Merge overlapping periods to avoid double-counting
	if len(allPeriods) == 0 {
		return 0
	}

	merged := mergeOverlappingPeriods(allPeriods)
	totalDowntime := 0.0

	for _, period := range merged {
		hours := period.end.Sub(period.start).Hours()
		totalDowntime += hours
	}
	return totalDowntime
}

// mergeOverlappingPeriods merges overlapping downtime periods to avoid double-counting
func mergeOverlappingPeriods(periods []downtimePeriod) []downtimePeriod {
	if len(periods) <= 1 {
		return periods
	}

	// Sort by start time
	for i := 0; i < len(periods)-1; i++ {
		for j := i + 1; j < len(periods); j++ {
			if periods[j].start.Before(periods[i].start) {
				periods[i], periods[j] = periods[j], periods[i]
			}
		}
	}

	merged := []downtimePeriod{periods[0]}

	for i := 1; i < len(periods); i++ {
		last := &merged[len(merged)-1]
		current := periods[i]

		// If current period overlaps with last, merge them
		if current.start.Before(last.end) || current.start.Equal(last.end) {
			if current.end.After(last.end) {
				last.end = current.end
			}
		} else {
			// No overlap, add as new period
			merged = append(merged, current)
		}
	}

	return merged
}

// extractDomainFromURL extracts the domain from an RPC URL
func extractDomainFromURL(rpcUrl string) string {
	return common.ExtractHost(rpcUrl)
}

// extractHostFromEndpoint tries to derive a host from an endpoint string (URL or host:port).
func extractHostFromEndpoint(endpoint string) string {
	return common.ExtractHost(endpoint)
}

// mapDomainToService maps a domain name to a service name
func mapDomainToService(domain, checkType, endpoint string) string {
	if checkType == "site" {
		// Site-level checks don't map to a specific service
		return ""
	}

	cleanDomain := common.ExtractHost(domain)
	c := cfg.GetConfig()

	// If domain is empty, try deriving from endpoint host
	if cleanDomain == "" && endpoint != "" {
		cleanDomain = common.ExtractHost(endpoint)
	}
	if cleanDomain == "" {
		return ""
	}

	exactMatches := []string{}
	suffixMatches := []string{}

	for svcName, svc := range c.Services {
		for _, provider := range svc.Providers {
			for _, rpcUrl := range provider.RpcUrls {
				rpcHost := extractDomainFromURL(rpcUrl)
				if rpcHost == "" {
					continue
				}
				if rpcHost == cleanDomain {
					exactMatches = append(exactMatches, svcName)
					break
				}
				if strings.HasSuffix(rpcHost, "."+cleanDomain) {
					suffixMatches = append(suffixMatches, svcName)
					break
				}
			}
		}
	}

	// Prefer exact matches; if multiple, treat as ambiguous to avoid mis-attribution
	if len(exactMatches) == 1 {
		return exactMatches[0]
	}
	if len(exactMatches) > 1 {
		return ""
	}

	if len(suffixMatches) == 1 {
		return suffixMatches[0]
	}
	if len(suffixMatches) > 1 {
		return ""
	}

	return ""
}
