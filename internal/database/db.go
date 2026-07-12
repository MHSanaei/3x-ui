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

func IsPostgres() bool {
	if db == nil {
		return config.GetDBKind() == "postgres"
	}
	return db.Name() == "postgres"
}

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

func allModels() []any {
	return []any{
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
}

func initModels() error {
	models := allModels()
	for _, mdl := range models {
		if IsPostgres() && postgresModelSettled(mdl) {
			continue
		}
		if err := db.AutoMigrate(mdl); err != nil {
			if isIgnorableDuplicateColumnErr(db, err, mdl) {
				log.Printf("Ignoring duplicate column during auto migration for %T: %v", mdl, err)
				continue
			}
			log.Printf("Error auto migrating model: %v", err)
			return err
		}
	}
	if err := dropLegacyInboundPortUnique(); err != nil {
		return err
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
	if err := migrateShadowsocksRemovedCiphers(); err != nil {
		return err
	}
	if err := migrateVmessRemovedSecurities(); err != nil {
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

type sqliteIndexListRow struct {
	Name   string `gorm:"column:name"`
	Unique int    `gorm:"column:unique"`
	Origin string `gorm:"column:origin"`
}

func sqliteUniquePortIndexes() (autoIndexes, explicitIndexes []string, err error) {
	var list []sqliteIndexListRow
	if err = db.Raw(`PRAGMA index_list('inbounds')`).Scan(&list).Error; err != nil {
		return nil, nil, err
	}
	for _, idx := range list {
		if idx.Unique != 1 {
			continue
		}
		var cols []struct {
			Name string `gorm:"column:name"`
		}
		if err = db.Raw(`PRAGMA index_info("` + idx.Name + `")`).Scan(&cols).Error; err != nil {
			return nil, nil, err
		}
		if len(cols) != 1 || cols[0].Name != "port" {
			continue
		}
		if idx.Origin == "c" {
			explicitIndexes = append(explicitIndexes, idx.Name)
		} else {
			autoIndexes = append(autoIndexes, idx.Name)
		}
	}
	return autoIndexes, explicitIndexes, nil
}

// dropLegacyInboundPortUnique removes the pre-multi-node UNIQUE on inbounds.port,
// which AutoMigrate never drops and which blocks cross-node port reuse on old SQLite DBs.
func dropLegacyInboundPortUnique() error {
	if IsPostgres() {
		return nil
	}
	autoIndexes, explicitIndexes, err := sqliteUniquePortIndexes()
	if err != nil {
		return err
	}
	for _, name := range explicitIndexes {
		if err := db.Exec(`DROP INDEX IF EXISTS "` + name + `"`).Error; err != nil {
			return err
		}
	}
	if len(autoIndexes) == 0 {
		return nil
	}
	log.Printf("Rebuilding inbounds table to drop the legacy UNIQUE constraint on port")
	return rebuildInboundsWithoutInlineUniquePort()
}

func sqliteTableColumns(tx *gorm.DB, table string) ([]string, error) {
	var rows []struct {
		Name string `gorm:"column:name"`
	}
	if err := tx.Raw(`PRAGMA table_info("` + table + `")`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	cols := make([]string, 0, len(rows))
	for _, r := range rows {
		cols = append(cols, r.Name)
	}
	return cols, nil
}

func rebuildInboundsWithoutInlineUniquePort() error {
	return db.Transaction(func(tx *gorm.DB) error {
		var list []sqliteIndexListRow
		if err := tx.Raw(`PRAGMA index_list('inbounds')`).Scan(&list).Error; err != nil {
			return err
		}
		for _, idx := range list {
			if idx.Origin != "c" {
				continue
			}
			if err := tx.Exec(`DROP INDEX IF EXISTS "` + idx.Name + `"`).Error; err != nil {
				return err
			}
		}
		if err := tx.Exec(`ALTER TABLE inbounds RENAME TO inbounds_legacy_rebuild`).Error; err != nil {
			return err
		}
		if err := tx.Migrator().CreateTable(&model.Inbound{}); err != nil {
			return err
		}
		newCols, err := sqliteTableColumns(tx, "inbounds")
		if err != nil {
			return err
		}
		oldCols, err := sqliteTableColumns(tx, "inbounds_legacy_rebuild")
		if err != nil {
			return err
		}
		oldSet := make(map[string]struct{}, len(oldCols))
		for _, c := range oldCols {
			oldSet[c] = struct{}{}
		}
		shared := make([]string, 0, len(newCols))
		for _, c := range newCols {
			if _, ok := oldSet[c]; ok {
				shared = append(shared, `"`+c+`"`)
			}
		}
		colList := strings.Join(shared, ", ")
		if err := tx.Exec(`INSERT INTO inbounds (` + colList + `) SELECT ` + colList + ` FROM inbounds_legacy_rebuild`).Error; err != nil {
			return err
		}
		return tx.Exec(`DROP TABLE inbounds_legacy_rebuild`).Error
	})
}

func migrateHostVerifyPeerCertByNameColumn() error {
	if !db.Migrator().HasColumn(&model.Host{}, "verify_peer_cert_by_name") {
		return nil
	}
	if IsPostgres() {

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

	return db.Exec(`UPDATE hosts SET verify_peer_cert_by_name = '' WHERE verify_peer_cert_by_name IS NULL OR typeof(verify_peer_cert_by_name) <> 'text'`).Error
}

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

// seedMtprotoSecretsToClients converts each legacy single-secret mtproto inbound
// into a one-client inbound so MTProto joins the shared multi-client model: the
// inbound-level secret becomes the first client's FakeTLS secret, and a
// ClientRecord + client_inbounds link are created so per-client traffic, limits,
// and share links work exactly like every other protocol. One-time, self-gated
// on the "MtprotoSecretsToClients" seeder row. Mirrors seedWireguardPeersToClients.
func seedMtprotoSecretsToClients() error {
	var history []string
	if err := db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &history).Error; err != nil {
		return err
	}
	if slices.Contains(history, "MtprotoSecretsToClients") {
		return nil
	}

	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", string(model.MTProto)).Find(&inbounds).Error; err != nil {
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
				log.Printf("MtprotoSecretsToClients: skip inbound %d (invalid settings json): %v", inbound.Id, err)
				continue
			}
			if clients, ok := settings["clients"].([]any); ok && len(clients) > 0 {
				continue
			}

			var linkCount int64
			if err := tx.Model(&model.ClientInbound{}).Where("inbound_id = ?", inbound.Id).Count(&linkCount).Error; err != nil {
				return err
			}
			if linkCount > 0 {
				continue
			}

			secret, _ := settings["secret"].(string)
			secret = strings.TrimSpace(secret)
			if secret == "" {
				domain, _ := settings["fakeTlsDomain"].(string)
				secret = model.GenerateFakeTLSSecret(strings.TrimSpace(domain))
			}

			email := mtprotoInboundClientEmail(inbound.Remark, usedEmails)
			usedEmails[email] = struct{}{}

			obj := map[string]any{
				"email":  email,
				"secret": secret,
				"enable": true,
				"subId":  random.NumLower(16),
			}
			c := model.Client{Email: email, Secret: secret, Enable: true, SubID: obj["subId"].(string)}

			incoming := c.ToRecord()
			var row model.ClientRecord
			err := tx.Where("email = ?", email).First(&row).Error
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

			delete(settings, "secret")
			settings["clients"] = []any{obj}
			newSettings, err := json.Marshal(settings)
			if err != nil {
				return err
			}
			if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
				Update("settings", string(newSettings)).Error; err != nil {
				return err
			}
		}
		return tx.Create(&model.HistoryOfSeeders{SeederName: "MtprotoSecretsToClients"}).Error
	})
}

// stripMtprotoInboundSecrets removes the vestigial inbound-level `secret` from
// every mtproto inbound. seedMtprotoSecretsToClients already drops it while
// converting legacy single-secret inbounds, but inbounds that already had clients
// kept the dead field, and the old HealMtprotoSecret regenerated it on every
// save. mtg and every share link read only per-client secrets, so the
// inbound-level value is dead data that once leaked into stale, unusable links.
// One-time, self-gated on the "StripMtprotoInboundSecrets" seeder row.
func stripMtprotoInboundSecrets() error {
	var history []string
	if err := db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &history).Error; err != nil {
		return err
	}
	if slices.Contains(history, "StripMtprotoInboundSecrets") {
		return nil
	}

	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", string(model.MTProto)).Find(&inbounds).Error; err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, inbound := range inbounds {
			stripped, ok := model.StripMtprotoInboundSecret(inbound.Settings)
			if !ok {
				continue
			}
			if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
				Update("settings", stripped).Error; err != nil {
				return err
			}
		}
		return tx.Create(&model.HistoryOfSeeders{SeederName: "StripMtprotoInboundSecrets"}).Error
	})
}

// mtprotoInboundClientEmail derives a stable, unique client email for a migrated
// mtproto inbound from its remark.
func mtprotoInboundClientEmail(remark string, used map[string]struct{}) string {
	base := strings.TrimSpace(remark)
	if base == "" {
		base = "mtproto"
	}
	email := strings.ReplaceAll(base, " ", "-")
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

// migrateShadowsocksRemovedCiphers rewrites shadowsocks inbounds still using
// the "none"/"plain" ciphers that xray-core v26.7.11 removed; one such row
// makes the whole generated config unbuildable and keeps xray from starting.
func migrateShadowsocksRemovedCiphers() error {
	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", model.Shadowsocks).Find(&inbounds).Error; err != nil {
		return err
	}
	migrated := int64(0)
	for _, inbound := range inbounds {
		if strings.TrimSpace(inbound.Settings) == "" {
			continue
		}
		var settings map[string]any
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			continue
		}
		changed := false
		if method, _ := settings["method"].(string); method != "" {
			if replacement, removed := model.ReplaceRemovedShadowsocksCipher(method); removed {
				settings["method"] = replacement
				changed = true
			}
		}
		if clients, ok := settings["clients"].([]any); ok {
			for i := range clients {
				cm, ok := clients[i].(map[string]any)
				if !ok {
					continue
				}
				method, _ := cm["method"].(string)
				if replacement, removed := model.ReplaceRemovedShadowsocksCipher(method); removed {
					cm["method"] = replacement
					clients[i] = cm
					changed = true
				}
			}
		}
		if !changed {
			continue
		}
		newSettings, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			log.Printf("migrateShadowsocksRemovedCiphers: skip inbound %d (marshal failed): %v", inbound.Id, err)
			continue
		}
		if err := db.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
			Update("settings", string(newSettings)).Error; err != nil {
			return err
		}
		migrated++
	}
	if migrated > 0 {
		log.Printf("Rewrote removed shadowsocks cipher on %d inbound(s)", migrated)
	}
	return nil
}

// migrateVmessRemovedSecurities rewrites the vmess "none"/"zero" security
// values that xray-core v26.7.11 removed to "auto" (what the core now treats
// them as), on both the clients column and each vmess inbound's settings.
func migrateVmessRemovedSecurities() error {
	res := db.Exec("UPDATE clients SET security = 'auto' WHERE security IN ('none', 'zero')")
	if res.Error != nil {
		log.Printf("Error migrating removed vmess security values on clients: %v", res.Error)
		return res.Error
	}
	if res.RowsAffected > 0 {
		log.Printf("Migrated %d client(s) off removed vmess security values", res.RowsAffected)
	}
	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", model.VMESS).Find(&inbounds).Error; err != nil {
		return err
	}
	migrated := int64(0)
	for _, inbound := range inbounds {
		if strings.TrimSpace(inbound.Settings) == "" {
			continue
		}
		var settings map[string]any
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			continue
		}
		clients, ok := settings["clients"].([]any)
		if !ok {
			continue
		}
		changed := false
		for i := range clients {
			cm, ok := clients[i].(map[string]any)
			if !ok {
				continue
			}
			if security, _ := cm["security"].(string); security == "none" || security == "zero" {
				cm["security"] = "auto"
				clients[i] = cm
				changed = true
			}
		}
		if !changed {
			continue
		}
		newSettings, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			log.Printf("migrateVmessRemovedSecurities: skip inbound %d (marshal failed): %v", inbound.Id, err)
			continue
		}
		if err := db.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
			Update("settings", string(newSettings)).Error; err != nil {
			return err
		}
		migrated++
	}
	if migrated > 0 {
		log.Printf("Rewrote removed vmess security values on %d inbound(s)", migrated)
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

func isIgnorableDuplicateColumnErr(gdb *gorm.DB, err error, mdl any) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())

	const sqlitePrefix = "duplicate column name:"
	if _, after, ok := strings.Cut(errMsg, sqlitePrefix); ok {
		col := strings.TrimSpace(after)
		col = strings.Trim(col, "`\"[]")
		return col != "" && gdb != nil && gdb.Migrator().HasColumn(mdl, col)
	}
	if strings.Contains(errMsg, "already exists") && strings.Contains(errMsg, "column ") {
		if _, after, ok := strings.Cut(errMsg, "column \""); ok {
			rest := after
			if e := strings.Index(rest, "\""); e > 0 {
				col := rest[:e]
				return col != "" && gdb != nil && gdb.Migrator().HasColumn(mdl, col)
			}
		}
	}
	return false
}

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

func runSeeders(isUsersEmpty bool) error {
	empty, err := isTableEmpty("history_of_seeders")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
		return err
	}

	if empty && isUsersEmpty {
		seeders := []string{"UserPasswordHash", "ClientsTable", "InboundClientsArrayFix", "InboundClientTgIdFix", "InboundClientSubIdFix", "FreedomFinalRulesReverseFix", "ApiTokensHash", "LegacyProxySettingsCleanup", "WireguardPeersToClients", "MtprotoSecretsToClients", "NodeInboundsAdopted"}
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

	if !slices.Contains(seedersHistory, "NodeInboundsAdopted") {
		if err := seedNodeInboundsAdopted(); err != nil {
			return err
		}
	}

	if err := seedHostsFromExternalProxy(); err != nil {
		return err
	}

	if err := resetIpLimitsWithoutFail2ban(); err != nil {
		return err
	}

	if err := seedWireguardPeersToClients(); err != nil {
		return err
	}

	if err := seedHostGroupIds(); err != nil {
		return err
	}

	// Self-gated on the "MtprotoSecretsToClients" row.
	if err := seedMtprotoSecretsToClients(); err != nil {
		return err
	}

	// Self-gated on the "StripMtprotoInboundSecrets" row. Must run after the
	// seeder above so legacy single-secret inbounds are first converted to a
	// client (which preserves the secret) before the inbound-level copy is
	// dropped from every mtproto inbound.
	if err := stripMtprotoInboundSecrets(); err != nil {
		return err
	}

	// Idempotent, not seeder-gated: bad values can re-enter via a restored
	// backup, so re-check on every start.
	return normalizeSettingPaths()
}

// seedNodeInboundsAdopted keeps the pre-existing reconcile behavior for nodes
// that were already syncing before the inbounds_adopted_at gate was introduced.
func seedNodeInboundsAdopted() error {
	if err := db.Model(&model.Node{}).
		Where("inbounds_adopted_at = 0").
		Update("inbounds_adopted_at", time.Now().Unix()).Error; err != nil {
		return err
	}
	return db.Create(&model.HistoryOfSeeders{SeederName: "NodeInboundsAdopted"}).Error
}

func seedHostGroupIds() error {
	var history []string
	if err := db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &history).Error; err != nil {
		return err
	}
	if slices.Contains(history, "HostGroupIds") {
		return nil
	}

	var hosts []*model.Host
	if err := db.Where("group_id = '' OR group_id IS NULL").Find(&hosts).Error; err != nil {
		return err
	}

	if len(hosts) > 0 {
		err := db.Transaction(func(tx *gorm.DB) error {
			for _, h := range hosts {
				h.GroupId = random.NumLower(16)
				if err := tx.Model(h).Update("group_id", h.GroupId).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return db.Create(&model.HistoryOfSeeders{SeederName: "HostGroupIds"}).Error
}

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

func fail2banCanEnforce() bool {
	if v, ok := os.LookupEnv("XUI_ENABLE_FAIL2BAN"); ok && v != "true" {
		return false
	}
	if runtime.GOOS == "windows" {
		return false
	}
	return exec.CommandContext(context.Background(), "fail2ban-client", "-h").Run() == nil
}

func clearLegacyProxySettings() error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("key IN ?", []string{"panelProxy", "tgBotProxy"}).
			Delete(&model.Setting{}).Error; err != nil {
			return err
		}
		return tx.Create(&model.HistoryOfSeeders{SeederName: "LegacyProxySettingsCleanup"}).Error
	})
}

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

func isTableEmpty(tableName string) (bool, error) {
	var count int64
	err := db.Table(tableName).Count(&count).Error
	return count == 0, err
}

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

func normalizeApiTokenCreatedAtSeconds() error {
	return db.Model(&model.ApiToken{}).
		Where("created_at >= ?", model.ApiTokenUnixMillisecondsThreshold).
		UpdateColumn("created_at", gorm.Expr("created_at / ?", 1000)).Error
}

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

func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func IsSQLiteDB(file io.ReaderAt) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := file.ReadAt(buf, 0)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

func Checkpoint() error {
	if IsPostgres() {
		return nil
	}
	return db.Exec("PRAGMA wal_checkpoint;").Error
}

func ValidateSQLiteDB(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil {
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
