package main

import (
	"flag"
	"os"
	"time"

	billing "github.com/ibp-network/ibp-geodns-collator/src/billing"

	api "github.com/ibp-network/ibp-geodns-collator/src/api"

	nats "github.com/ibp-network/ibp-geodns-libs/nats"

	cfg "github.com/ibp-network/ibp-geodns-libs/config"
	data2 "github.com/ibp-network/ibp-geodns-libs/data2"
	log "github.com/ibp-network/ibp-geodns-libs/logging"
	"github.com/ibp-network/ibp-geodns-libs/matrix"
)

var version = cfg.GetVersion()

const checkTypeNormalizeInterval = 5 * time.Second

func main() {
	log.Log(log.Info, "IBPCollator v%s starting …", version)

	cfgPath := flag.String("config", "ibpcollator.json", "Path to configuration file")
	flag.Parse()

	if _, err := os.Stat(*cfgPath); os.IsNotExist(err) {
		log.Log(log.Fatal, "configuration file not found: %s", *cfgPath)
		os.Exit(1)
	}

	// ── load configuration ──────────────────────────────────────────────────────
	cfg.Init(*cfgPath)
	c := cfg.GetConfig()
	log.SetLogLevel(log.ParseLogLevel(c.Local.System.LogLevel))

	// ── subsystems ──────────────────────────────────────────────────────────────
	matrix.Init() // outbound Matrix alerts
	data2.Init()  // collator local DB layer - CHANGED: now synchronous

	// Normalize any legacy check_type values and keep future ones tidy
	if err := normalizeMemberEventCheckTypes(); err != nil {
		log.Log(log.Error, "[collator] initial check_type normalization failed: %v", err)
	}
	startMemberEventCheckTypeNormalizer()

	// Wait a moment to ensure DB is fully ready
	time.Sleep(2 * time.Second)

	billing.Init() // ← billing subsystem
	api.Init()     // ← NEW: API subsystem

	if err := nats.Connect(); err != nil {
		log.Log(log.Fatal, "NATS connect: %v", err)
		os.Exit(1)
	}

	// ── register with the NATS cluster ──────────────────────────────────────────
	nats.State.NodeID = c.Local.Nats.NodeID
	nats.State.ThisNode = nats.NodeInfo{
		NodeID:        c.Local.Nats.NodeID,
		ListenAddress: "0.0.0.0",
		ListenPort:    "0",
		NodeRole:      "IBPCollator",
	}

	if err := nats.EnableCollatorRole(); err != nil {
		log.Log(log.Fatal, "enable collator role: %v", err)
		os.Exit(1)
	}

	// kick-off background collectors
	go nats.StartUsageCollector()
	go nats.StartMemoryJanitor()

	log.Log(log.Info, "[collator] started – awaiting events")

	for {
		time.Sleep(1 * time.Hour)
	}
}

func startMemberEventCheckTypeNormalizer() {
	go func() {
		ticker := time.NewTicker(checkTypeNormalizeInterval)
		defer ticker.Stop()

		for {
			if err := normalizeMemberEventCheckTypes(); err != nil {
				log.Log(log.Error, "[collator] normalize member_events check_type: %v", err)
			}

			<-ticker.C
		}
	}()
}

func normalizeMemberEventCheckTypes() error {
	if data2.DB == nil {
		return nil
	}

	res, err := data2.DB.Exec(`
		UPDATE member_events
		SET check_type = CASE check_type
			WHEN '1' THEN 'site'
			WHEN '2' THEN 'domain'
			WHEN '3' THEN 'endpoint'
			ELSE check_type
		END
		WHERE check_type IN ('1','2','3')
	`)
	if err != nil {
		return err
	}

	if rows, err := res.RowsAffected(); err == nil && rows > 0 {
		log.Log(log.Debug, "[collator] normalized %d member_events check_type row(s)", rows)
	}

	return nil
}
