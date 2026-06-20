package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func initMigrateDB(t *testing.T) {
	t.Helper()
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
}

func seedInboundWithStream(t *testing.T, tag string, port int, stream string) *model.Inbound {
	t.Helper()
	ib := &model.Inbound{
		UserId: 1, Tag: tag, Enable: true, Port: port, Protocol: model.VLESS,
		Remark: tag, Settings: `{"clients":[]}`, StreamSettings: stream,
	}
	if err := GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound %s: %v", tag, err)
	}
	return ib
}

const epMigrationStream = `{"network":"ws","security":"tls","externalProxy":[
	{"forceTls":"tls","dest":"a.cdn.com","port":8443,"remark":"A","sni":"a.sni","fingerprint":"chrome","alpn":["h2","h3"],"pinnedPeerCertSha256":["AAAA"],"echConfigList":"ECHV"},
	{"forceTls":"none","dest":"b.cdn.com","port":80,"remark":"B"}
]}`

// #1 — each externalProxy entry becomes one host row with the exact field
// mapping; sort_order is the entry index; inbound_id is correct.
func TestMigrate_ExternalProxyToHosts(t *testing.T) {
	initMigrateDB(t)
	ib := seedInboundWithStream(t, "m1", 5551, epMigrationStream)

	if err := seedHostsFromExternalProxy(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var hosts []model.Host
	if err := GetDB().Where("inbound_id = ?", ib.Id).Order("sort_order asc").Find(&hosts).Error; err != nil {
		t.Fatalf("load hosts: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("hosts = %d, want 2", len(hosts))
	}
	a := hosts[0]
	if a.InboundId != ib.Id || a.SortOrder != 0 || a.Security != "tls" || a.Address != "a.cdn.com" ||
		a.Port != 8443 || a.Remark != "A" || a.Sni != "a.sni" || a.Fingerprint != "chrome" || a.EchConfigList != "ECHV" {
		t.Fatalf("host A mapping wrong: %+v", a)
	}
	if len(a.Alpn) != 2 || a.Alpn[0] != "h2" || a.Alpn[1] != "h3" {
		t.Fatalf("host A alpn = %v, want [h2 h3]", a.Alpn)
	}
	if len(a.PinnedPeerCertSha256) != 1 || a.PinnedPeerCertSha256[0] != "AAAA" {
		t.Fatalf("host A pins = %v, want [AAAA]", a.PinnedPeerCertSha256)
	}
	b := hosts[1]
	if b.InboundId != ib.Id || b.SortOrder != 1 || b.Security != "none" || b.Address != "b.cdn.com" ||
		b.Port != 80 || b.Remark != "B" {
		t.Fatalf("host B mapping wrong: %+v", b)
	}
}

// #2 — a second run is a no-op (the HistoryOfSeeders gate).
func TestMigrate_Idempotent(t *testing.T) {
	initMigrateDB(t)
	seedInboundWithStream(t, "m2", 5552, epMigrationStream)

	if err := seedHostsFromExternalProxy(); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := seedHostsFromExternalProxy(); err != nil {
		t.Fatalf("second run: %v", err)
	}
	var count int64
	GetDB().Model(&model.Host{}).Count(&count)
	if count != 2 {
		t.Fatalf("host count = %d, want 2 (second run must be a no-op)", count)
	}
}

// #3 — inbounds without externalProxy create no hosts.
func TestMigrate_NoExternalProxy_NoHosts(t *testing.T) {
	initMigrateDB(t)
	seedInboundWithStream(t, "m3", 5553, `{"network":"tcp","security":"none"}`)

	if err := seedHostsFromExternalProxy(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var count int64
	GetDB().Model(&model.Host{}).Count(&count)
	if count != 0 {
		t.Fatalf("host count = %d, want 0", count)
	}
}

// #4 — externalProxy stays in StreamSettings (additive, rollback-safe).
func TestMigrate_KeepsExternalProxyIntact(t *testing.T) {
	initMigrateDB(t)
	ib := seedInboundWithStream(t, "m4", 5554, epMigrationStream)

	if err := seedHostsFromExternalProxy(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var got model.Inbound
	if err := GetDB().First(&got, ib.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	if !strings.Contains(got.StreamSettings, "externalProxy") || !strings.Contains(got.StreamSettings, "a.cdn.com") {
		t.Fatalf("externalProxy must remain in StreamSettings: %s", got.StreamSettings)
	}
}

// #5 — same against a real Postgres DSN (sequence resync); skips without a DSN.
func TestMigrate_Postgres(t *testing.T) {
	if strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" || os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run the postgres migration test")
	}
	if err := InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	// Clean slate so this run owns the migration regardless of prior tests.
	GetDB().Exec("TRUNCATE TABLE hosts, inbounds RESTART IDENTITY CASCADE")
	GetDB().Where("seeder_name = ?", "HostsFromExternalProxy").Delete(&model.HistoryOfSeeders{})

	seedInboundWithStream(t, "mpg", 5555, epMigrationStream)
	if err := seedHostsFromExternalProxy(); err != nil {
		t.Fatalf("migrate pg: %v", err)
	}
	var count int64
	GetDB().Model(&model.Host{}).Count(&count)
	if count != 2 {
		t.Fatalf("pg host count = %d, want 2", count)
	}
	if err := seedHostsFromExternalProxy(); err != nil {
		t.Fatalf("migrate pg (2nd): %v", err)
	}
	GetDB().Model(&model.Host{}).Count(&count)
	if count != 2 {
		t.Fatalf("pg host count after 2nd run = %d, want 2 (idempotent)", count)
	}
}
