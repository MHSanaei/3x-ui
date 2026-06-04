// Package database provides database initialization, migration, and management utilities
// for the 3x-ui panel using GORM with SQLite or PostgreSQL.
package database

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/config"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/util/random"
	"github.com/mhsanaei/3x-ui/v3/xray"

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
	return db.Dialector.Name() == "postgres"
}

// Dialect returns the active GORM dialect name, or "" if the DB is not open.
func Dialect() string {
	if db == nil {
		return ""
	}
	return db.Dialector.Name()
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
		&model.CustomGeoResource{},
		&model.Node{},
		&model.ApiToken{},
		&model.ClientRecord{},
		&model.ClientInbound{},
		&model.ClientGroup{},
		&model.InboundFallback{},
		&model.NodeClientTraffic{},
	}
	for _, mdl := range models {
		if err := db.AutoMigrate(mdl); err != nil {
			if isIgnorableDuplicateColumnErr(err, mdl) {
				log.Printf("Ignoring duplicate column during auto migration for %T: %v", mdl, err)
				continue
			}
			log.Printf("Error auto migrating model: %v", err)
			return err
		}
	}
	if err := dropLegacyForeignKeys(); err != nil {
		return err
	}
	if err := pruneOrphanedClientInbounds(); err != nil {
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
		seeders := []string{"UserPasswordHash", "ClientsTable", "InboundClientsArrayFix", "InboundClientTgIdFix", "InboundClientSubIdFix", "FreedomFinalRulesReverseFix", "ApiTokensHash"}
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
		if err = os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=10000&_synchronous=NORMAL&_txlock=immediate"
		db, err = gorm.Open(sqlite.Open(dsn), c)
		if err != nil {
			return err
		}
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return err
		}
		if _, err := sqlDB.Exec("PRAGMA busy_timeout=10000"); err != nil {
			return err
		}
		if _, err := sqlDB.Exec("PRAGMA synchronous=NORMAL"); err != nil {
			return err
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
