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

	if err := migrateMemberEventCheckTypesToNumeric(); err != nil {
		log.Log(log.Error, "[collator] check_type numeric migration failed: %v", err)
	}

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
