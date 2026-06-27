package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

func (s *InboundService) MigrationRemoveOrphanedTraffics() {
	db := database.GetDB()
	query := fmt.Sprintf(
		"DELETE FROM client_traffics WHERE email NOT IN (SELECT %s %s)",
		database.JSONFieldText("client.value", "email"),
		database.JSONClientsFromInbound(),
	)
	db.Exec(query)
}

func (s *InboundService) MigrationRequirements() {
	db := database.GetDB()
	tx := db.Begin()
	var err error
	defer func() {
		if err == nil {
			tx.Commit()
			if !database.IsPostgres() {
				if dbErr := db.Exec(`VACUUM "main"`).Error; dbErr != nil {
					logger.Warningf("VACUUM failed: %v", dbErr)
				}
			}
		} else {
			tx.Rollback()
		}
	}()

	if tx.Migrator().HasColumn(&model.Inbound{}, "all_time") {
		if err = tx.Migrator().DropColumn(&model.Inbound{}, "all_time"); err != nil {
			return
		}
	}
	if tx.Migrator().HasColumn(&xray.ClientTraffic{}, "all_time") {
		if err = tx.Migrator().DropColumn(&xray.ClientTraffic{}, "all_time"); err != nil {
			return
		}
	}
	if err = normalizeInboundShareAddressColumns(tx); err != nil {
		return
	}

	// Normalize "enable" columns to boolean on Postgres. Legacy SQLite data
	// (0/1 integers), partial migrations, or mixed write paths (public API
	// inbound updates that flow through UpdateClientStat + client syncs, plus
	// node traffic merge deltas) can leave the column as integer or with mixed
	// interpretation. This (combined with the dialect-aware
	// ClientTrafficEnableMergeExpr) prevents type problems in the node traffic
	// sync merge (SetRemoteTraffic) and makes the sync robust even when
	// inbounds are updated via the public API (incl. ones carrying
	// externalProxy in streamSettings). The same expression is also safe on
	// SQLite (no PG :: casts).
	if database.IsPostgres() {
		// Use DO block so it is idempotent and doesn't fail if already boolean.
		normalizeBool := func(table, col string) {
			tx.Exec(fmt.Sprintf(`
				DO $$
				BEGIN
					IF EXISTS (
						SELECT 1 FROM information_schema.columns
						WHERE table_name = '%s' AND column_name = '%s'
						  AND data_type <> 'boolean'
					) THEN
						ALTER TABLE %s ALTER COLUMN %s
							TYPE boolean USING (CASE WHEN %s::text IN ('1','true','t','yes') THEN true ELSE false END);
					END IF;
				END $$;`, table, col, table, col, col))
		}
		normalizeBool("inbounds", "enable")
		normalizeBool("client_traffics", "enable")
		normalizeBool("nodes", "enable")
		normalizeBool("clients", "enable")
		normalizeBool("api_tokens", "enabled")
		normalizeBool("outbound_subscriptions", "enabled")
	}

	// Fix inbounds based problems
	var inbounds []*model.Inbound
	err = tx.Model(model.Inbound{}).Where("protocol IN (?)", []string{"vmess", "vless", "trojan", "shadowsocks", "hysteria"}).Find(&inbounds).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	for inbound_index := range inbounds {
		settings := map[string]any{}
		_ = json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
		if raw, exists := settings["clients"]; exists && raw == nil {
			settings["clients"] = []any{}
		}
		clients, ok := settings["clients"].([]any)
		if ok {
			// Fix Client configuration problems
			newClients := make([]any, 0, len(clients))
			hasVisionFlow := false
			for client_index := range clients {
				c := clients[client_index].(map[string]any)

				// Add email='' if it is not exists
				if _, ok := c["email"]; !ok {
					c["email"] = ""
				}

				// Convert string tgId to int64
				if _, ok := c["tgId"]; ok {
					tgId := c["tgId"]
					if tgIdStr, ok2 := tgId.(string); ok2 {
						tgIdInt64, err := strconv.ParseInt(strings.ReplaceAll(tgIdStr, " ", ""), 10, 64)
						if err == nil {
							c["tgId"] = tgIdInt64
						}
					}
				}

				// Remove "flow": "xtls-rprx-direct"
				if _, ok := c["flow"]; ok {
					if c["flow"] == "xtls-rprx-direct" {
						c["flow"] = ""
					}
				}
				if flow, _ := c["flow"].(string); flow == "xtls-rprx-vision" {
					hasVisionFlow = true
				}
				// Backfill created_at and updated_at
				if _, ok := c["created_at"]; !ok {
					c["created_at"] = time.Now().Unix() * 1000
				}
				c["updated_at"] = time.Now().Unix() * 1000
				newClients = append(newClients, any(c))
			}
			settings["clients"] = newClients

			// Drop orphaned testseed: VLESS-only field, only meaningful when at least
			// one client uses the exact xtls-rprx-vision flow. Older versions saved it
			// for any non-empty flow (including the UDP variant) or kept it after the
			// flow was cleared from the client modal — clean those up here.
			if inbounds[inbound_index].Protocol == model.VLESS && !hasVisionFlow {
				delete(settings, "testseed")
			}

			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return
			}

			inbounds[inbound_index].Settings = string(modifiedSettings)
		}

		// Add client traffic row for all clients which has email
		modelClients, err := s.GetClients(inbounds[inbound_index])
		if err != nil {
			return
		}
		for _, modelClient := range modelClients {
			if len(modelClient.Email) > 0 {
				var count int64
				tx.Model(xray.ClientTraffic{}).Where("email = ?", modelClient.Email).Count(&count)
				if count == 0 {
					_ = s.AddClientStat(tx, inbounds[inbound_index].Id, &modelClient)
				}
			}
		}

		// Heal clients table for installs where the one-shot seeder
		// skipped clients due to a tgId-string unmarshal error.
		if syncErr := s.clientService.SyncInbound(tx, inbounds[inbound_index].Id, modelClients); syncErr != nil {
			logger.Warning("MigrationRequirements sync clients failed:", syncErr)
		}
	}
	tx.Save(inbounds)

	// Remove orphaned traffics
	tx.Where("inbound_id = 0").Delete(xray.ClientTraffic{})

	// Migrate old MultiDomain to External Proxy
	var externalProxy []struct {
		Id             int
		Port           int
		StreamSettings string // text column on both DBs; safer than []byte for cross-DB scan
	}
	externalProxyQuery := `select id, port, stream_settings
	from inbounds
	WHERE protocol in ('vmess','vless','trojan')
	  AND json_extract(stream_settings, '$.security') = 'tls'
	  AND json_extract(stream_settings, '$.tlsSettings.settings.domains') IS NOT NULL`
	if database.IsPostgres() {
		externalProxyQuery = `select id, port, stream_settings
	from inbounds
	WHERE protocol in ('vmess','vless','trojan')
	  AND NULLIF(stream_settings, '')::jsonb #>> '{security}' = 'tls'
	  AND NULLIF(stream_settings, '')::jsonb #> '{tlsSettings,settings,domains}' IS NOT NULL`
	}
	err = tx.Raw(externalProxyQuery).Scan(&externalProxy).Error
	if err != nil || len(externalProxy) == 0 {
		return
	}

	for _, ep := range externalProxy {
		var reverses any
		var stream map[string]any
		_ = json.Unmarshal([]byte(ep.StreamSettings), &stream)
		if tlsSettings, ok := stream["tlsSettings"].(map[string]any); ok {
			if settings, ok := tlsSettings["settings"].(map[string]any); ok {
				if domains, ok := settings["domains"].([]any); ok {
					for _, domain := range domains {
						if domainMap, ok := domain.(map[string]any); ok {
							domainMap["forceTls"] = "same"
							domainMap["port"] = ep.Port
							domainMap["dest"] = domainMap["domain"].(string)
							delete(domainMap, "domain")
						}
					}
				}
				reverses = settings["domains"]
				delete(settings, "domains")
			}
		}
		stream["externalProxy"] = reverses
		newStream, _ := json.MarshalIndent(stream, " ", "  ")
		tx.Model(model.Inbound{}).Where("id = ?", ep.Id).Update("stream_settings", newStream)
	}

	// Legacy tag cleanup for old auto-generated tags (e.g. "0.0.0.0:443-...").
	// Must be cross-DB: INSTR/REPLACE work on SQLite; Postgres needs position().
	tagCleanup := `UPDATE inbounds
		SET tag = REPLACE(tag, '0.0.0.0:', '')
		WHERE INSTR(tag, '0.0.0.0:') > 0;`
	if database.IsPostgres() {
		tagCleanup = `UPDATE inbounds
			SET tag = REPLACE(tag, '0.0.0.0:', '')
			WHERE position('0.0.0.0:' in tag) > 0;`
	}
	err = tx.Exec(tagCleanup).Error
	if err != nil {
		return
	}
}

func (s *InboundService) MigrateDB() {
	s.MigrationRequirements()
	s.MigrationRemoveOrphanedTraffics()
	s.MigrationRestoreVisionFlow()
}

// MigrationRestoreVisionFlow repairs VLESS inbounds whose clients lost their
// XTLS Vision flow because the inbound was not flow-eligible when the client was
// written (e.g. an XHTTP inbound whose vlessenc encryption was enabled only
// later). For each now-eligible inbound it restores flow=xtls-rprx-vision on
// clients whose intended flow (their flow_override on a sibling inbound) is
// Vision. Idempotent: once a client carries the flow it is skipped, so this is a
// no-op on healthy installs and on subsequent boots.
func (s *InboundService) MigrationRestoreVisionFlow() {
	db := database.GetDB()
	var inbounds []*model.Inbound
	if err := db.Model(&model.Inbound{}).
		Where("protocol = ?", model.VLESS).
		Find(&inbounds).Error; err != nil {
		logger.Warning("MigrationRestoreVisionFlow: load inbounds failed:", err)
		return
	}
	for _, ib := range inbounds {
		restored, changed := s.restoreVisionFlowForEligibleInbound(nil, ib.Settings, ib.StreamSettings, ib.Protocol)
		if !changed {
			continue
		}
		clients, err := s.GetClients(&model.Inbound{Settings: restored})
		if err != nil {
			logger.Warning("MigrationRestoreVisionFlow: parse clients for inbound", ib.Id, "failed:", err)
			continue
		}
		err = db.Transaction(func(tx *gorm.DB) error {
			if e := tx.Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", restored).Error; e != nil {
				return e
			}
			return s.clientService.SyncInbound(tx, ib.Id, clients)
		})
		if err != nil {
			logger.Warning("MigrationRestoreVisionFlow: update inbound", ib.Id, "failed:", err)
			continue
		}
		logger.Info("MigrationRestoreVisionFlow: restored XTLS Vision flow on inbound", ib.Id)
	}
}
