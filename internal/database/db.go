// Package database provides database initialization, migration, and management utilities
// for the 3x-ui panel using GORM with SQLite or PostgreSQL.
package database

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

const (
	DialectSQLite   = "sqlite"
	DialectPostgres = "postgres"
)

// IsPostgres reports whether the active connection is a PostgreSQL backend.
func IsPostgres() bool {
	if db == nil {
		return config.GetDBKind() == "postgres"
	}
	return db.Name() == "postgres"
}

// Dialect returns the active GORM dialect name, or "" if the DB is not open.
func Dialect() string {
	if db == nil {
		return ""
	}
	return db.Name()
}

const (
	defaultUsername = "admin"
	defaultPassword = "admin"
)

func initModels() error {
	models := []any{
		&model.User{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
		&model.Node{},
		&model.ApiToken{},
		&model.ClientRecord{},
		&model.ClientInbound{},
		&model.ClientExternalLink{},
		&model.ClientGroup{},
		&model.InboundFallback{},
		&model.Host{},
		&model.NodeClientTraffic{},
		&model.NodeClientIp{},
		&model.ClientGlobalTraffic{},
		&model.OutboundSubscription{},
	}
	for _, mdl := range models {
		if IsPostgres() && postgresModelSettled(mdl) {
			continue
		}
		if err := db.AutoMigrate(mdl); err != nil {
			if isIgnorableDuplicateColumnErr(err, mdl) {
				log.Printf("Ignoring duplicate column during auto migration for %T: %v", mdl, err)
				continue
			}
			log.Printf("Error auto migrating model: %v", err)
			return err
		}
	}
	if err := migrateHostVerifyPeerCertByNameColumn(); err != nil {
		return err
	}
	if err := normalizeApiTokenCreatedAtSeconds(); err != nil {
		return err
	}
	if err := dropLegacyForeignKeys(); err != nil {
		return err
	}
	if err := pruneOrphanedClientInbounds(); err != nil {
		return err
	}
	if err := pruneOrphanedHosts(); err != nil {
		return err
	}
	if err := normalizeInboundSubSortIndex(); err != nil {
		return err
	}
	if err := repairOverflowedTrafficCounters(); err != nil {
		return err
	}
	if err := dedupeInboundSettingsClients(); err != nil {
		return err
	}
	if err := migrateLegacySocksInboundsToMixed(); err != nil {
		return err
	}
	if IsPostgres() {
		if err := resyncPostgresSequences(db, models); err != nil {
			log.Printf("Error resyncing postgres sequences: %v", err)
			return err
		}
	}
	return nil
}

// postgresModelSettled skips AutoMigrate when table, columns, and indexes all exist:
// its catalog-filtered column probe misdetects on some setups and re-ADDs columns forever (#5665).
func postgresModelSettled(mdl any) bool {
	migrator := db.Migrator()
	if !migrator.HasTable(mdl) {
		return false
	}
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(mdl); err != nil || stmt.Schema == nil {
		return false
	}
	for _, dbName := range stmt.Schema.DBNames {
		if !migrator.HasColumn(mdl, dbName) {
			return false
		}
	}
	for _, idx := range stmt.Schema.ParseIndexes() {
		if !migrator.HasIndex(mdl, idx.Name) {
			return false
		}
	}
	return true
}

func dropLegacyForeignKeys() error {
	if !IsPostgres() {
		return nil
	}
	if err := db.Exec("ALTER TABLE client_traffics DROP CONSTRAINT IF EXISTS fk_inbounds_client_stats").Error; err != nil {
		log.Printf("Error dropping legacy foreign key fk_inbounds_client_stats: %v", err)
		return err
	}
	return nil
}

// migrateHostVerifyPeerCertByNameColumn converts hosts.verify_peer_cert_by_name
// from its original boolean shape to the comma-separated string xray-core's
// verifyPeerCertByName (vcn) actually expects. The legacy boolean was dead
// (never emitted into links), so its value carries no meaning and is discarded.
// Idempotent by construction (no HistoryOfSeeders row — writing one here would
// flip the fresh-DB detection in runSeeders). Runs right after AutoMigrate,
// before anything reads or writes Host rows (critical on Postgres, where the
// column stays boolean-typed until the ALTER below).
func migrateHostVerifyPeerCertByNameColumn() error {
	if !db.Migrator().HasColumn(&model.Host{}, "verify_peer_cert_by_name") {
		return nil
	}
	if IsPostgres() {
		// Only convert a still-boolean column; once it is text this is a no-op,
		// so a user-set name is never wiped on a later restart.
		var dataType string
		if err := db.Raw(
			`SELECT data_type FROM information_schema.columns WHERE table_name = 'hosts' AND column_name = 'verify_peer_cert_by_name'`,
		).Scan(&dataType).Error; err != nil {
			return err
		}
		if dataType != "boolean" {
			return nil
		}
		if err := db.Exec(`ALTER TABLE hosts ALTER COLUMN verify_peer_cert_by_name DROP DEFAULT`).Error; err != nil {
			return err
		}
		return db.Exec(`ALTER TABLE hosts ALTER COLUMN verify_peer_cert_by_name TYPE text USING ''`).Error
	}
	// SQLite keeps the original numeric-affinity column; blank any legacy
	// integer/null value so it doesn't read back as "0"/"1". After conversion
	// every value is text, so re-running touches nothing.
	return db.Exec(`UPDATE hosts SET verify_peer_cert_by_name = '' WHERE verify_peer_cert_by_name IS NULL OR typeof(verify_peer_cert_by_name) <> 'text'`).Error
}

// seedHostsFromExternalProxy is a one-time, self-gated migration that creates a
// Host row for every legacy externalProxy entry on every inbound. Additive: the
// externalProxy arrays are left intact in StreamSettings.
func seedHostsFromExternalProxy() error {
	var history []string
	if err := db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &history).Error; err != nil {
		return err
	}
	if slices.Contains(history, "HostsFromExternalProxy") {
		return nil
	}

	var inbounds []model.Inbound
	if err := db.Find(&inbounds).Error; err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, inbound := range inbounds {
			if _, err := CreateHostsFromExternalProxy(tx, inbound.Id, inbound.StreamSettings); err != nil {
				return err
			}
		}
		return tx.Create(&model.HistoryOfSeeders{SeederName: "HostsFromExternalProxy"}).Error
	})
}

// seedWireguardPeersToClients is a one-time, self-gated migration that converts
// legacy single-config WireGuard inbounds into the multi-client model: each
// settings.peers[] entry becomes a managed client in the clients table attached
// to the inbound, and the inbound settings are rewritten so peers becomes a
// clients[] array (GetXrayConfig re-projects clients back to peers for xray).
// Idempotent: gated on the history row and skipped per-inbound once it already
// has client links.
func seedWireguardPeersToClients() error {
	var history []string
	if err := db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &history).Error; err != nil {
		return err
	}
	if slices.Contains(history, "WireguardPeersToClients") {
		return nil
	}

	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", string(model.WireGuard)).Find(&inbounds).Error; err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		usedEmails := map[string]struct{}{}
		var existingEmails []string
		if err := tx.Model(&model.ClientRecord{}).Pluck("email", &existingEmails).Error; err != nil {
			return err
		}
		for _, e := range existingEmails {
			usedEmails[e] = struct{}{}
		}

		for _, inbound := range inbounds {
			if strings.TrimSpace(inbound.Settings) == "" {
				continue
			}
			var settings map[string]any
			if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
				log.Printf("WireguardPeersToClients: skip inbound %d (invalid settings json): %v", inbound.Id, err)
				continue
			}
			peers, ok := settings["peers"].([]any)
			if !ok || len(peers) == 0 {
				continue
			}

			var linkCount int64
			if err := tx.Model(&model.ClientInbound{}).Where("inbound_id = ?", inbound.Id).Count(&linkCount).Error; err != nil {
				return err
			}
			if linkCount > 0 {
				continue
			}

			clientObjs := make([]any, 0, len(peers))
			for i, raw := range peers {
				obj, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				email := wireguardPeerEmail(inbound.Remark, obj, i, usedEmails)
				usedEmails[email] = struct{}{}
				obj["email"] = email
				if sub, _ := obj["subId"].(string); strings.TrimSpace(sub) == "" {
					obj["subId"] = random.NumLower(16)
				}
				if _, ok := obj["enable"]; !ok {
					obj["enable"] = true
				}

				blob, err := json.Marshal(obj)
				if err != nil {
					continue
				}
				var c model.Client
				if err := json.Unmarshal(blob, &c); err != nil {
					log.Printf("WireguardPeersToClients: skip peer in inbound %d: %v", inbound.Id, err)
					continue
				}
				c.Email = email

				incoming := c.ToRecord()
				var row model.ClientRecord
				err = tx.Where("email = ?", email).First(&row).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					if err := tx.Create(incoming).Error; err != nil {
						return err
					}
					row = *incoming
				} else if err != nil {
					return err
				} else {
					model.MergeClientRecord(&row, incoming)
					if err := tx.Save(&row).Error; err != nil {
						return err
					}
				}

				link := model.ClientInbound{ClientId: row.Id, InboundId: inbound.Id}
				if err := tx.Where("client_id = ? AND inbound_id = ?", row.Id, inbound.Id).
					FirstOrCreate(&link).Error; err != nil {
					return err
				}

				clientObjs = append(clientObjs, obj)
			}

			delete(settings, "peers")
			settings["clients"] = clientObjs
			newSettings, err := json.Marshal(settings)
			if err != nil {
				return err
			}
			if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
				Update("settings", string(newSettings)).Error; err != nil {
				return err
			}
		}
		return tx.Create(&model.HistoryOfSeeders{SeederName: "WireguardPeersToClients"}).Error
	})
}

// wireguardPeerEmail derives a stable, unique client email for a migrated peer
// from the inbound remark plus the peer's comment (or its 1-based index).
func wireguardPeerEmail(remark string, peer map[string]any, index int, used map[string]struct{}) string {
	base := strings.TrimSpace(remark)
	if base == "" {
		base = "wg"
	}
	suffix := strconv.Itoa(index + 1)
	if c, ok := peer["comment"].(string); ok && strings.TrimSpace(c) != "" {
		suffix = strings.TrimSpace(c)
	}
	email := strings.ReplaceAll(base+"-"+suffix, " ", "-")
	candidate := email
	for n := 2; ; n++ {
		if _, taken := used[candidate]; !taken {
			return candidate
		}
		candidate = email + "-" + strconv.Itoa(n)
	}
}

// CreateHostsFromExternalProxy parses a legacy streamSettings.externalProxy array
// and inserts one Host row per entry on tx, returning the number of rows created.
// It is the shared core of both the one-time seedHostsFromExternalProxy startup
// migration and the inbound-import path: an inbound exported from a build that
// predated the hosts table carries its external proxies inline in
// streamSettings.externalProxy, and the startup migration is gated off after its
// first run, so a freshly imported inbound must be converted here instead. Blank
// or malformed streamSettings, or one without externalProxy entries, is a no-op.
func CreateHostsFromExternalProxy(tx *gorm.DB, inboundId int, streamSettings string) (int, error) {
	if strings.TrimSpace(streamSettings) == "" {
		return 0, nil
	}
	var stream map[string]any
	if err := json.Unmarshal([]byte(streamSettings), &stream); err != nil {
		return 0, nil
	}
	eps, ok := stream["externalProxy"].([]any)
	if !ok || len(eps) == 0 {
		return 0, nil
	}
	created := 0
	for i, raw := range eps {
		ep, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if err := tx.Create(externalProxyEntryToHost(inboundId, i, ep)).Error; err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// externalProxyEntryToHost maps one legacy externalProxy entry onto a Host.
// forceTls (same|tls|none) maps straight to Security; an unknown value falls back
// to "same" (inherit). An empty remark gets a stable generated label so the row
// stays valid/editable, and the remark is capped at the model's 256-char limit.
func externalProxyEntryToHost(inboundId, index int, ep map[string]any) *model.Host {
	security, _ := ep["forceTls"].(string)
	switch security {
	case "same", "tls", "none":
	default:
		security = "same"
	}
	dest, _ := ep["dest"].(string)
	port := 0
	if p, ok := ep["port"].(float64); ok {
		port = int(p)
	}
	remark, _ := ep["remark"].(string)
	if strings.TrimSpace(remark) == "" {
		remark = "imported " + strconv.Itoa(index+1)
	}
	if len(remark) > 256 {
		remark = remark[:256]
	}
	sni, _ := ep["sni"].(string)
	fingerprint, _ := ep["fingerprint"].(string)
	ech, _ := ep["echConfigList"].(string)
	return &model.Host{
		InboundId:            inboundId,
		SortOrder:            index,
		Remark:               remark,
		Address:              dest,
		Port:                 port,
		Security:             security,
		Sni:                  sni,
		Fingerprint:          fingerprint,
		Alpn:                 anyToNonEmptyStrings(ep["alpn"]),
		PinnedPeerCertSha256: anyToNonEmptyStrings(ep["pinnedPeerCertSha256"]),
		EchConfigList:        ech,
	}
}

func anyToNonEmptyStrings(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(t))
		for _, s := range t {
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func pruneOrphanedHosts() error {
	res := db.Exec("DELETE FROM hosts WHERE inbound_id NOT IN (SELECT id FROM inbounds)")
	if res.Error != nil {
		log.Printf("Error pruning orphaned hosts rows: %v", res.Error)
		return res.Error
	}
	if res.RowsAffected > 0 {
		log.Printf("Pruned %d orphaned hosts row(s)", res.RowsAffected)
	}
	return nil
}

func pruneOrphanedClientInbounds() error {
	res := db.Exec("DELETE FROM client_inbounds WHERE inbound_id NOT IN (SELECT id FROM inbounds)")
	if res.Error != nil {
		log.Printf("Error pruning orphaned client_inbounds rows: %v", res.Error)
		return res.Error
	}
	if res.RowsAffected > 0 {
		log.Printf("Pruned %d orphaned client_inbounds row(s)", res.RowsAffected)
	}
	return nil
}

// migrateLegacySocksInboundsToMixed renames legacy socks inbounds to mixed.
// The protocol enum dropped socks in favor of mixed (identical settings shape,
// same behavior plus HTTP on the shared port), so rows predating the rename
// fail model validation — most visibly when pushed to a node, where one legacy
// inbound stalled the entire node's config and traffic sync (#5685).
func migrateLegacySocksInboundsToMixed() error {
	res := db.Exec("UPDATE inbounds SET protocol = 'mixed' WHERE protocol = 'socks'")
	if res.Error != nil {
		log.Printf("Error migrating legacy socks inbounds to mixed: %v", res.Error)
		return res.Error
	}
	if res.RowsAffected > 0 {
		log.Printf("Migrated %d legacy socks inbound(s) to mixed", res.RowsAffected)
	}
	return nil
}

// normalizeInboundSubSortIndex lifts sub_sort_index values below the 1-based
// minimum (rows written by builds that defaulted the column to 0, or by nodes
// predating the field) so they cannot sort ahead of explicitly ranked inbounds.
func normalizeInboundSubSortIndex() error {
	res := db.Exec("UPDATE inbounds SET sub_sort_index = 1 WHERE sub_sort_index < 1")
	if res.Error != nil {
		log.Printf("Error normalizing inbound sub_sort_index: %v", res.Error)
		return res.Error
	}
	if res.RowsAffected > 0 {
		log.Printf("Normalized sub_sort_index on %d inbound(s)", res.RowsAffected)
	}
	return nil
}

// repairOverflowedTrafficCounters heals traffic counters that historic
// compounding bugs pushed past int64: on SQLite an overflowing INTEGER is
// silently promoted to REAL, after which the column no longer scans into the
// Go int64 field and every reader of the table fails (#5762). REAL cells are
// cast back to INTEGER (SQLite caps the cast at math.MaxInt64), then values
// are clamped into [0, TrafficMax] on both backends so the next delta cannot
// overflow again.
func repairOverflowedTrafficCounters() error {
	targets := []struct {
		table   string
		columns []string
	}{
		{"client_traffics", []string{"up", "down"}},
		{"inbounds", []string{"up", "down"}},
		{"outbound_traffics", []string{"up", "down", "total"}},
		{"node_client_traffics", []string{"up", "down"}},
	}
	for _, target := range targets {
		for _, col := range target.columns {
			statements := []string{
				fmt.Sprintf("UPDATE %s SET %s = %d WHERE %s > %d", target.table, col, TrafficMax, col, TrafficMax),
				fmt.Sprintf("UPDATE %s SET %s = 0 WHERE %s < 0", target.table, col, col),
			}
			if !IsPostgres() {
				statements = append([]string{
					fmt.Sprintf("UPDATE %s SET %s = CAST(%s AS INTEGER) WHERE typeof(%s) = 'real'", target.table, col, col, col),
				}, statements...)
			}
			var repaired int64
			for _, statement := range statements {
				res := db.Exec(statement)
				if res.Error != nil {
					log.Printf("Error repairing %s.%s: %v", target.table, col, res.Error)
					return res.Error
				}
				repaired += res.RowsAffected
			}
			if repaired > 0 {
				log.Printf("Repaired %d overflowed %s.%s value(s)", repaired, target.table, col)
			}
		}
	}
	return nil
}

// dedupeInboundSettingsClients collapses duplicate same-email entries inside
// every inbound's settings.clients array, keeping the first occurrence.
// Retried or raced multi-node client adds on older builds appended the same
// client several times (#5770), which the client lists then rendered as
// phantom duplicates. Runs on every start (idempotent, writes only changed
// rows) because a restored backup or a not-yet-upgraded node's snapshot can
// reintroduce duplicates.
func dedupeInboundSettingsClients() error {
	var inbounds []model.Inbound
	if err := db.Find(&inbounds).Error; err != nil {
		return err
	}
	repaired := int64(0)
	for _, inbound := range inbounds {
		if strings.TrimSpace(inbound.Settings) == "" {
			continue
		}
		var settings map[string]any
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			continue
		}
		clients, _ := settings["clients"].([]any)
		if len(clients) < 2 {
			continue
		}
		seen := make(map[string]struct{}, len(clients))
		kept := make([]any, 0, len(clients))
		for _, c := range clients {
			if cm, ok := c.(map[string]any); ok {
				if email, _ := cm["email"].(string); email != "" {
					key := strings.ToLower(email)
					if _, dup := seen[key]; dup {
						continue
					}
					seen[key] = struct{}{}
				}
			}
			kept = append(kept, c)
		}
		if len(kept) == len(clients) {
			continue
		}
		settings["clients"] = kept
		newSettings, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			log.Printf("dedupeInboundSettingsClients: skip inbound %d (marshal failed): %v", inbound.Id, err)
			continue
		}
		if err := db.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
			Update("settings", string(newSettings)).Error; err != nil {
			return err
		}
		repaired++
	}
	if repaired > 0 {
		log.Printf("Removed duplicate client entries from %d inbound(s)", repaired)
	}
	return nil
}

func isIgnorableDuplicateColumnErr(err error, mdl any) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	// SQLite: "duplicate column name: foo"
	// Postgres: `pq: column "foo" of relation "bar" already exists` / `sqlstate 42701`
	const sqlitePrefix = "duplicate column name:"
	if _, after, ok := strings.Cut(errMsg, sqlitePrefix); ok {
		col := strings.TrimSpace(after)
		col = strings.Trim(col, "`\"[]")
		return col != "" && db != nil && db.Migrator().HasColumn(mdl, col)
	}
	if strings.Contains(errMsg, "already exists") && strings.Contains(errMsg, "column ") {
		// Best effort: extract the column name between the first pair of double quotes.
		if _, after, ok := strings.Cut(errMsg, "column \""); ok {
			rest := after
			if e := strings.Index(rest, "\""); e > 0 {
				col := rest[:e]
				return col != "" && db != nil && db.Migrator().HasColumn(mdl, col)
			}
		}
	}
	return false
}

// initUser creates a default admin user if the users table is empty.
func initUser() error {
	empty, err := isTableEmpty("users")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
		return err
	}
	if empty {
		hashedPassword, err := crypto.HashPasswordAsBcrypt(defaultPassword)
		if err != nil {
			log.Printf("Error hashing default password: %v", err)
			return err
		}

		user := &model.User{
			Username: defaultUsername,
			Password: hashedPassword,
		}
		return db.Create(user).Error
	}
	return nil
}

// runSeeders migrates user passwords to bcrypt and records seeder execution to prevent re-running.
func runSeeders(isUsersEmpty bool) error {
	empty, err := isTableEmpty("history_of_seeders")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
		return err
	}

	if empty && isUsersEmpty {
		seeders := []string{"UserPasswordHash", "ClientsTable", "InboundClientsArrayFix", "InboundClientTgIdFix", "InboundClientSubIdFix", "FreedomFinalRulesReverseFix", "ApiTokensHash", "LegacyProxySettingsCleanup", "WireguardPeersToClients"}
		for _, name := range seeders {
			if err := db.Create(&model.HistoryOfSeeders{SeederName: name}).Error; err != nil {
				return err
			}
		}
		return seedApiTokens()
	}

	var seedersHistory []string
	if err := db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &seedersHistory).Error; err != nil {
		log.Printf("Error fetching seeder history: %v", err)
		return err
	}

	if !slices.Contains(seedersHistory, "UserPasswordHash") && !isUsersEmpty {
		var users []model.User
		if err := db.Find(&users).Error; err != nil {
			log.Printf("Error fetching users for password migration: %v", err)
			return err
		}

		for _, user := range users {
			if crypto.IsHashed(user.Password) {
				continue
			}
			hashedPassword, err := crypto.HashPasswordAsBcrypt(user.Password)
			if err != nil {
				log.Printf("Error hashing password for user '%s': %v", user.Username, err)
				return err
			}
			if err := db.Model(&user).Update("password", hashedPassword).Error; err != nil {
				log.Printf("Error updating password for user '%s': %v", user.Username, err)
				return err
			}
		}

		hashSeeder := &model.HistoryOfSeeders{
			SeederName: "UserPasswordHash",
		}
		if err := db.Create(hashSeeder).Error; err != nil {
			return err
		}
	}

	if !slices.Contains(seedersHistory, "ApiTokensTable") {
		if err := seedApiTokens(); err != nil {
			return err
		}
	}

	if !slices.Contains(seedersHistory, "ApiTokensHash") {
		if err := hashExistingApiTokens(); err != nil {
			return err
		}
	}

	if !slices.Contains(seedersHistory, "ClientsTable") {
		if err := seedClientsFromInboundJSON(); err != nil {
			return err
		}
	}

	if !slices.Contains(seedersHistory, "InboundClientsArrayFix") {
		if err := normalizeInboundClientsArray(); err != nil {
			return err
		}
	}

	if !slices.Contains(seedersHistory, "InboundClientTgIdFix") {
		if err := normalizeInboundClientTgId(); err != nil {
			return err
		}
	}

	if !slices.Contains(seedersHistory, "InboundClientSubIdFix") {
		if err := normalizeInboundClientSubId(); err != nil {
			return err
		}
	}

	if !slices.Contains(seedersHistory, "FreedomFinalRulesReverseFix") {
		if err := normalizeFreedomFinalRules(); err != nil {
			return err
		}
	}

	if !slices.Contains(seedersHistory, "LegacyProxySettingsCleanup") {
		if err := clearLegacyProxySettings(); err != nil {
			return err
		}
	}

	// Self-gated on the "HostsFromExternalProxy" row, so it is safe to call
	// unconditionally here.
	if err := seedHostsFromExternalProxy(); err != nil {
		return err
	}

	// Self-gated on the "ResetIpLimitNoFail2ban" row.
	if err := resetIpLimitsWithoutFail2ban(); err != nil {
		return err
	}

	// Self-gated on the "WireguardPeersToClients" row.
	if err := seedWireguardPeersToClients(); err != nil {
		return err
	}

	// Idempotent, not seeder-gated: bad values can re-enter via a restored
	// backup, so re-check on every start.
	return normalizeSettingPaths()
}

// resetIpLimitsWithoutFail2ban zeroes every client's IP limit on hosts where
// fail2ban can't enforce it (not installed, or the integration disabled). The
// limit silently does nothing there yet kept logging a repeated warning, so a
// stale value is just misleading — the panel also disables the field on these
// hosts. One-time, self-gated on the seeder row.
func resetIpLimitsWithoutFail2ban() error {
	var history []string
	if err := db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &history).Error; err != nil {
		return err
	}
	if slices.Contains(history, "ResetIpLimitNoFail2ban") {
		return nil
	}

	if fail2banCanEnforce() {
		return db.Create(&model.HistoryOfSeeders{SeederName: "ResetIpLimitNoFail2ban"}).Error
	}

	var inbounds []model.Inbound
	if err := db.Find(&inbounds).Error; err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, inbound := range inbounds {
			if strings.TrimSpace(inbound.Settings) == "" {
				continue
			}
			var settings map[string]any
			if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
				log.Printf("ResetIpLimitNoFail2ban: skip inbound %d (invalid settings json): %v", inbound.Id, err)
				continue
			}
			clients, ok := settings["clients"].([]any)
			if !ok {
				continue
			}
			mutated := false
			for i, raw := range clients {
				obj, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				v, present := obj["limitIp"]
				if !present {
					continue
				}
				if n, isNum := v.(float64); isNum && n == 0 {
					continue
				}
				obj["limitIp"] = 0
				clients[i] = obj
				mutated = true
			}
			if !mutated {
				continue
			}
			settings["clients"] = clients
			newSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				log.Printf("ResetIpLimitNoFail2ban: skip inbound %d (marshal failed): %v", inbound.Id, err)
				continue
			}
			if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
				Update("settings", string(newSettings)).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.ClientRecord{}).Where("limit_ip <> ?", 0).
			Update("limit_ip", 0).Error; err != nil {
			return err
		}
		return tx.Create(&model.HistoryOfSeeders{SeederName: "ResetIpLimitNoFail2ban"}).Error
	})
}

// fail2banCanEnforce reports whether per-client IP limits can actually be
// enforced on this host: the integration must be enabled (XUI_ENABLE_FAIL2BAN)
// and fail2ban-client must be present. Mirrors the service-layer check, kept
// local to avoid an import cycle.
func fail2banCanEnforce() bool {
	if v, ok := os.LookupEnv("XUI_ENABLE_FAIL2BAN"); ok && v != "true" {
		return false
	}
	if runtime.GOOS == "windows" {
		return false
	}
	return exec.CommandContext(context.Background(), "fail2ban-client", "-h").Run() == nil
}

// clearLegacyProxySettings drops the deprecated panelProxy/tgBotProxy rows so a
// stale tgBotProxy no longer masks the panelOutbound egress fallback.
func clearLegacyProxySettings() error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("key IN ?", []string{"panelProxy", "tgBotProxy"}).
			Delete(&model.Setting{}).Error; err != nil {
			return err
		}
		return tx.Create(&model.HistoryOfSeeders{SeederName: "LegacyProxySettingsCleanup"}).Error
	})
}

// normalizeSettingPaths repairs URI-path settings persisted before the
// leading/trailing-slash rules existed (or restored from an old backup),
// mirroring entity.AllSetting.CheckValid. CheckValid self-heals these on save,
// but the frontend rejects the whole Settings form on the bad stored value
// before a save can ever reach it (#5726), so the stored rows themselves must
// be fixed. Idempotent; runs on every start.
func normalizeSettingPaths() error {
	pathKeys := []string{"webBasePath", "subPath", "subJsonPath", "subClashPath"}
	var rows []model.Setting
	if err := db.Where("key IN ?", pathKeys).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		fixed := row.Value
		if !strings.HasPrefix(fixed, "/") {
			fixed = "/" + fixed
		}
		if !strings.HasSuffix(fixed, "/") {
			fixed += "/"
		}
		if fixed == row.Value {
			continue
		}
		if err := db.Model(&model.Setting{}).Where("id = ?", row.Id).
			Update("value", fixed).Error; err != nil {
			return err
		}
	}
	return nil
}

func normalizeInboundClientTgId() error {
	var inbounds []model.Inbound
	if err := db.Find(&inbounds).Error; err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, inbound := range inbounds {
			if strings.TrimSpace(inbound.Settings) == "" {
				continue
			}
			var settings map[string]any
			if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
				log.Printf("InboundClientTgIdFix: skip inbound %d (invalid settings json): %v", inbound.Id, err)
				continue
			}
			clients, ok := settings["clients"].([]any)
			if !ok {
				continue
			}
			mutated := false
			for i, raw := range clients {
				obj, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				tgRaw, present := obj["tgId"]
				if !present {
					continue
				}
				v, isFloat := tgRaw.(float64)
				if isFloat && !math.IsNaN(v) && !math.IsInf(v, 0) && v == math.Trunc(v) {
					continue
				}
				obj["tgId"] = int64(0)
				clients[i] = obj
				mutated = true
			}
			if !mutated {
				continue
			}
			settings["clients"] = clients
			newSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				log.Printf("InboundClientTgIdFix: skip inbound %d (marshal failed): %v", inbound.Id, err)
				continue
			}
			if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
				Update("settings", string(newSettings)).Error; err != nil {
				return err
			}
		}
		return tx.Create(&model.HistoryOfSeeders{SeederName: "InboundClientTgIdFix"}).Error
	})
}

func normalizeInboundClientSubId() error {
	var inbounds []model.Inbound
	if err := db.Find(&inbounds).Error; err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, inbound := range inbounds {
			if strings.TrimSpace(inbound.Settings) == "" {
				continue
			}
			var settings map[string]any
			if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
				log.Printf("InboundClientSubIdFix: skip inbound %d (invalid settings json): %v", inbound.Id, err)
				continue
			}
			clients, ok := settings["clients"].([]any)
			if !ok {
				continue
			}
			mutated := false
			for i, raw := range clients {
				obj, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				existing, _ := obj["subId"].(string)
				if strings.TrimSpace(existing) != "" {
					continue
				}
				obj["subId"] = random.NumLower(16)
				clients[i] = obj
				mutated = true
			}
			if !mutated {
				continue
			}
			settings["clients"] = clients
			newSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				log.Printf("InboundClientSubIdFix: skip inbound %d (marshal failed): %v", inbound.Id, err)
				continue
			}
			if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
				Update("settings", string(newSettings)).Error; err != nil {
				return err
			}
		}
		return tx.Create(&model.HistoryOfSeeders{SeederName: "InboundClientSubIdFix"}).Error
	})
}

func normalizeInboundClientsArray() error {
	var inbounds []model.Inbound
	if err := db.Find(&inbounds).Error; err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, inbound := range inbounds {
			if strings.TrimSpace(inbound.Settings) == "" {
				continue
			}
			var settings map[string]any
			if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
				log.Printf("InboundClientsArrayFix: skip inbound %d (invalid settings json): %v", inbound.Id, err)
				continue
			}
			raw, exists := settings["clients"]
			if !exists || raw != nil {
				continue
			}
			settings["clients"] = []any{}
			newSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				log.Printf("InboundClientsArrayFix: skip inbound %d (marshal failed): %v", inbound.Id, err)
				continue
			}
			if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
				Update("settings", string(newSettings)).Error; err != nil {
				return err
			}
		}
		return tx.Create(&model.HistoryOfSeeders{SeederName: "InboundClientsArrayFix"}).Error
	})
}

func normalizeFreedomFinalRules() error {
	var setting model.Setting
	err := db.Model(model.Setting{}).Where("key = ?", "xrayTemplateConfig").First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&model.HistoryOfSeeders{SeederName: "FreedomFinalRulesReverseFix"}).Error
	}
	if err != nil {
		return err
	}

	updated, changed, rErr := rewriteFreedomFinalRules(setting.Value)
	if rErr != nil {
		log.Printf("FreedomFinalRulesReverseFix: skip (invalid xrayTemplateConfig json): %v", rErr)
		return db.Create(&model.HistoryOfSeeders{SeederName: "FreedomFinalRulesReverseFix"}).Error
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if changed {
			if err := tx.Model(&model.Setting{}).Where("key = ?", "xrayTemplateConfig").
				Update("value", updated).Error; err != nil {
				return err
			}
		}
		return tx.Create(&model.HistoryOfSeeders{SeederName: "FreedomFinalRulesReverseFix"}).Error
	})
}

func rewriteFreedomFinalRules(raw string) (string, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return raw, false, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return raw, false, err
	}
	outbounds, ok := cfg["outbounds"].([]any)
	if !ok {
		return raw, false, nil
	}
	changed := false
	for _, ob := range outbounds {
		obj, ok := ob.(map[string]any)
		if !ok {
			continue
		}
		if proto, _ := obj["protocol"].(string); proto != "freedom" {
			continue
		}
		settings, ok := obj["settings"].(map[string]any)
		if !ok {
			continue
		}
		if !isLegacyPrivateOnlyFinalRules(settings["finalRules"]) {
			continue
		}
		settings["finalRules"] = []any{map[string]any{"action": "allow"}}
		changed = true
	}
	if !changed {
		return raw, false, nil
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return raw, false, err
	}
	return string(out), true, nil
}

func isLegacyPrivateOnlyFinalRules(v any) bool {
	rules, ok := v.([]any)
	if !ok || len(rules) != 1 {
		return false
	}
	rule, ok := rules[0].(map[string]any)
	if !ok {
		return false
	}
	if action, _ := rule["action"].(string); action != "allow" {
		return false
	}
	ips, ok := rule["ip"].([]any)
	if !ok || len(ips) != 1 {
		return false
	}
	if s, _ := ips[0].(string); s != "geoip:private" {
		return false
	}
	for k := range rule {
		if k != "action" && k != "ip" {
			return false
		}
	}
	return true
}

// normalizeClientJSONFields coerces loosely-typed numeric fields in a raw
// settings.clients entry so json.Unmarshal into model.Client doesn't fail
// when older rows wrote tgId/limitIp/totalGB/etc. as strings. Empty strings
// drop the key so the field falls back to its zero value.
func normalizeClientJSONFields(obj map[string]any) {
	normalizeInt := func(key string) {
		raw, exists := obj[key]
		if !exists {
			return
		}
		s, ok := raw.(string)
		if !ok {
			return
		}
		trimmed := strings.ReplaceAll(strings.TrimSpace(s), " ", "")
		if trimmed == "" {
			delete(obj, key)
			return
		}
		if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			obj[key] = n
		} else {
			delete(obj, key)
		}
	}
	for _, k := range []string{"tgId", "limitIp", "totalGB", "expiryTime", "reset", "created_at", "updated_at"} {
		normalizeInt(k)
	}
}

func seedClientsFromInboundJSON() error {
	var inbounds []model.Inbound
	if err := db.Find(&inbounds).Error; err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		byEmail := map[string]*model.ClientRecord{}

		var existing []model.ClientRecord
		if err := tx.Find(&existing).Error; err != nil {
			return err
		}
		for i := range existing {
			byEmail[existing[i].Email] = &existing[i]
		}

		for _, inbound := range inbounds {
			if strings.TrimSpace(inbound.Settings) == "" {
				continue
			}
			var settings map[string]any
			if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
				log.Printf("ClientsTable seed: skip inbound %d (invalid settings json): %v", inbound.Id, err)
				continue
			}
			rawList, ok := settings["clients"].([]any)
			if !ok {
				continue
			}

			for _, raw := range rawList {
				obj, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				normalizeClientJSONFields(obj)
				blob, err := json.Marshal(obj)
				if err != nil {
					continue
				}
				var c model.Client
				if err := json.Unmarshal(blob, &c); err != nil {
					log.Printf("ClientsTable seed: skip client in inbound %d (unmarshal failed): %v; payload=%s",
						inbound.Id, err, string(blob))
					continue
				}
				email := strings.TrimSpace(c.Email)
				if email == "" {
					continue
				}
				incoming := c.ToRecord()

				row, dup := byEmail[email]
				if !dup {
					if err := tx.Create(incoming).Error; err != nil {
						return err
					}
					byEmail[email] = incoming
					row = incoming
				} else {
					conflicts := model.MergeClientRecord(row, incoming)
					for _, x := range conflicts {
						log.Printf("client merge: email=%s conflict on %s old=%v new=%v kept=%v",
							email, x.Field, x.Old, x.New, x.Kept)
					}
					if err := tx.Save(row).Error; err != nil {
						return err
					}
				}

				link := model.ClientInbound{
					ClientId:     row.Id,
					InboundId:    inbound.Id,
					FlowOverride: c.Flow,
				}
				if err := tx.Where("client_id = ? AND inbound_id = ?", row.Id, inbound.Id).
					FirstOrCreate(&link).Error; err != nil {
					return err
				}
			}
		}

		return tx.Create(&model.HistoryOfSeeders{SeederName: "ClientsTable"}).Error
	})
}

// seedApiTokens copies the legacy `apiToken` setting into the new
// api_tokens table as a row named "default" so existing central panels
// keep working after the upgrade. Idempotent — records itself in
// history_of_seeders and only runs when api_tokens is empty.
func seedApiTokens() error {
	empty, err := isTableEmpty("api_tokens")
	if err != nil {
		return err
	}
	if empty {
		var legacy model.Setting
		err := db.Model(model.Setting{}).Where("key = ?", "apiToken").First(&legacy).Error
		if err == nil && legacy.Value != "" {
			row := &model.ApiToken{
				Name:    "default",
				Token:   legacy.Value,
				Enabled: true,
			}
			if err := db.Create(row).Error; err != nil {
				log.Printf("Error migrating legacy apiToken: %v", err)
				return err
			}
		}
	}
	return db.Create(&model.HistoryOfSeeders{SeederName: "ApiTokensTable"}).Error
}

// hashExistingApiTokens replaces any plaintext token stored before tokens were
// hashed at rest with its SHA-256 digest. Callers keep their plaintext copy
// (used on remote nodes), so existing tokens keep authenticating; the panel
// just can no longer reveal them. Idempotent — already-hashed rows are skipped.
func hashExistingApiTokens() error {
	var rows []*model.ApiToken
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		if crypto.IsSHA256Hex(r.Token) {
			continue
		}
		hashed := crypto.HashTokenSHA256(r.Token)
		if err := db.Model(model.ApiToken{}).Where("id = ?", r.Id).Update("token", hashed).Error; err != nil {
			log.Printf("Error hashing api token %d: %v", r.Id, err)
			return err
		}
	}
	return db.Create(&model.HistoryOfSeeders{SeederName: "ApiTokensHash"}).Error
}

// isTableEmpty returns true if the named table contains zero rows.
func isTableEmpty(tableName string) (bool, error) {
	var count int64
	err := db.Table(tableName).Count(&count).Error
	return count == 0, err
}

// InitDB sets up the database connection, migrates models, and runs seeders.
// When XUI_DB_TYPE=postgres, dbPath is ignored and XUI_DB_DSN is used instead.
func InitDB(dbPath string) error {
	var gormLogger logger.Interface
	if config.IsDebug() {
		gormLogger = logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logger.Info,
				IgnoreRecordNotFoundError: true,
				Colorful:                  true,
			},
		)
	} else {
		gormLogger = logger.Discard
	}
	c := &gorm.Config{Logger: gormLogger, DisableForeignKeyConstraintWhenMigrating: true}

	var err error
	switch config.GetDBKind() {
	case "postgres":
		dsn := config.GetDBDSN()
		if dsn == "" {
			return errors.New("XUI_DB_TYPE=postgres but XUI_DB_DSN is empty")
		}
		db, err = gorm.Open(postgres.Open(dsn), c)
		if err != nil {
			return err
		}
	default:
		dir := path.Dir(dbPath)
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		// Keep journal_mode=DELETE so the DB stays a single file (no -wal/-shm
		// sidecars). synchronous defaults to FULL for durability but is tunable.
		sync := sqliteSynchronous()
		dsn := dbPath + "?_journal_mode=DELETE&_busy_timeout=10000&_synchronous=" + sync + "&_txlock=immediate"
		db, err = gorm.Open(sqlite.Open(dsn), c)
		if err != nil {
			return err
		}
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		// Re-assert the DSN pragmas plus scan-friendly ones for large datasets.
		// cache_size/mmap_size/temp_store create no extra files, so the single-file
		// guarantee holds; they just cut disk I/O on the 50k-row hot paths.
		pragmas := []string{
			"PRAGMA journal_mode=DELETE",
			"PRAGMA busy_timeout=10000",
			"PRAGMA synchronous=" + sync,
			fmt.Sprintf("PRAGMA cache_size=-%d", envInt("XUI_DB_CACHE_MB", 32)*1024),
			fmt.Sprintf("PRAGMA mmap_size=%d", int64(envInt("XUI_DB_MMAP_MB", 256))*1024*1024),
			"PRAGMA temp_store=MEMORY",
		}
		for _, p := range pragmas {
			if _, err := sqlDB.ExecContext(context.Background(), p); err != nil {
				return err
			}
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	var maxOpen, maxIdle int
	switch config.GetDBKind() {
	case "postgres":
		maxOpen = envInt("XUI_DB_MAX_OPEN_CONNS", 25)
		maxIdle = envInt("XUI_DB_MAX_IDLE_CONNS", 25)
	default:
		maxOpen = envInt("XUI_DB_MAX_OPEN_CONNS", 8)
		maxIdle = envInt("XUI_DB_MAX_IDLE_CONNS", 4)
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)

	if err := initModels(); err != nil {
		return err
	}

	isUsersEmpty, err := isTableEmpty("users")
	if err != nil {
		return err
	}

	if err := initUser(); err != nil {
		return err
	}
	return runSeeders(isUsersEmpty)
}

// normalizeApiTokenCreatedAtSeconds repairs rows written while ApiToken used
// autoCreateTime:milli. The threshold separates modern Unix milliseconds from
// Unix seconds and makes this safe to run on every startup.
func normalizeApiTokenCreatedAtSeconds() error {
	return db.Model(&model.ApiToken{}).
		Where("created_at >= ?", model.ApiTokenUnixMillisecondsThreshold).
		UpdateColumn("created_at", gorm.Expr("created_at / ?", 1000)).Error
}

// sqliteSynchronous returns the SQLite synchronous mode, defaulting to FULL.
// Whitelisted because the value is interpolated directly into a PRAGMA string.
func sqliteSynchronous() string {
	switch strings.ToUpper(strings.TrimSpace(os.Getenv("XUI_DB_SYNCHRONOUS"))) {
	case "OFF":
		return "OFF"
	case "NORMAL":
		return "NORMAL"
	case "EXTRA":
		return "EXTRA"
	default:
		return "FULL"
	}
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// CloseDB closes the database connection if it exists.
func CloseDB() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// GetDB returns the global GORM database instance.
func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// IsSQLiteDB checks if the given file is a valid SQLite database by reading its signature.
func IsSQLiteDB(file io.ReaderAt) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := file.ReadAt(buf, 0)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

// Checkpoint performs a WAL checkpoint on the SQLite database to ensure data consistency.
// No-op on PostgreSQL (WAL there is managed by the server).
func Checkpoint() error {
	if IsPostgres() {
		return nil
	}
	return db.Exec("PRAGMA wal_checkpoint;").Error
}

// ValidateSQLiteDB opens the provided sqlite DB path with a throw-away connection
// and runs a PRAGMA integrity_check to ensure the file is structurally sound.
// It does not mutate global state or run migrations.
func ValidateSQLiteDB(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil { // file must exist
		return err
	}
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	var res string
	if err := gdb.Raw("PRAGMA integrity_check;").Scan(&res).Error; err != nil {
		return err
	}
	if res != "ok" {
		return errors.New("sqlite integrity check failed: " + res)
	}
	return nil
}
