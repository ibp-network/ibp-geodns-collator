package main

import (
	data2 "github.com/ibp-network/ibp-geodns-libs/data2"
	log "github.com/ibp-network/ibp-geodns-libs/logging"
)

func migrateMemberEventCheckTypesToNumeric() error {
	if data2.DB == nil {
		return nil
	}

	res, err := data2.DB.Exec(`
		UPDATE member_events
		SET check_type = CASE check_type
			WHEN 'site' THEN '1'
			WHEN 'domain' THEN '2'
			WHEN 'endpoint' THEN '3'
			ELSE check_type
		END
		WHERE check_type IN ('site','domain','endpoint')
	`)
	if err != nil {
		return err
	}

	if rows, err := res.RowsAffected(); err == nil && rows > 0 {
		log.Log(log.Debug, "[collator] migrated %d member_events check_type row(s) to numeric form", rows)
	}

	return nil
}
