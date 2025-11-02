package main

import (
	"time"

	data2 "github.com/ibp-network/ibp-geodns-libs/data2"
	log "github.com/ibp-network/ibp-geodns-libs/logging"
)

const checkTypeNormalizeInterval = 5 * time.Second

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
