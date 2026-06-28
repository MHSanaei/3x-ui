package database

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func initWGMigrationDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
}

func createWGInbound(t *testing.T, remark string, port int, peers []any) *model.Inbound {
	t.Helper()
	settings, err := json.Marshal(map[string]any{
		"secretKey": "c2VjcmV0LWtleS1iYXNlNjQtMzJieXRlcy1wbGFjZWg=",
		"mtu":       1420,
		"peers":     peers,
	})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	in := &model.Inbound{
		UserId:   1,
		Remark:   remark,
		Port:     port,
		Protocol: model.WireGuard,
		Settings: string(settings),
		Tag:      remark,
	}
	if err := db.Create(in).Error; err != nil {
		t.Fatalf("create wg inbound: %v", err)
	}
	return in
}

func clearWGMigrationHistory(t *testing.T) {
	t.Helper()
	if err := db.Where("seeder_name = ?", "WireguardPeersToClients").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear history: %v", err)
	}
}

func reloadInboundSettings(t *testing.T, id int) map[string]any {
	t.Helper()
	var in model.Inbound
	if err := db.First(&in, id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(in.Settings), &settings); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	return settings
}

func wgPeer(comment, priv, pub, ip string, keepAlive int) any {
	m := map[string]any{
		"privateKey": priv,
		"publicKey":  pub,
		"allowedIPs": []any{ip},
		"keepAlive":  keepAlive,
	}
	if comment != "" {
		m["comment"] = comment
	}
	return m
}

func TestSeedWireguardPeersToClientsCreatesClients(t *testing.T) {
	initWGMigrationDB(t)
	in := createWGInbound(t, "wg-server", 51820, []any{
		wgPeer("laptop", "priv-1", "pub-1", "10.0.0.2/32", 25),
	})
	clearWGMigrationHistory(t)

	if err := seedWireguardPeersToClients(); err != nil {
		t.Fatalf("seedWireguardPeersToClients: %v", err)
	}

	var rec model.ClientRecord
	if err := db.Where("email = ?", "wg-server-laptop").First(&rec).Error; err != nil {
		t.Fatalf("migrated client not found: %v", err)
	}
	if rec.PrivateKey != "priv-1" || rec.PublicKey != "pub-1" || rec.AllowedIPs != "10.0.0.2/32" {
		t.Fatalf("wg columns not migrated: %+v", rec)
	}

	var linkCount int64
	db.Model(&model.ClientInbound{}).Where("inbound_id = ? AND client_id = ?", in.Id, rec.Id).Count(&linkCount)
	if linkCount != 1 {
		t.Fatalf("expected 1 client_inbounds link, got %d", linkCount)
	}

	settings := reloadInboundSettings(t, in.Id)
	if _, ok := settings["peers"]; ok {
		t.Fatalf("peers key must be removed from stored settings")
	}
	clients, ok := settings["clients"].([]any)
	if !ok || len(clients) != 1 {
		t.Fatalf("settings.clients not written: %v", settings["clients"])
	}
	if settings["secretKey"] == nil || settings["mtu"] == nil {
		t.Fatalf("server fields not preserved: %v", settings)
	}
}

func TestSeedWireguardPeersToClientsIdempotent(t *testing.T) {
	initWGMigrationDB(t)
	in := createWGInbound(t, "wg-idem", 51823, []any{
		wgPeer("", "priv-a", "pub-a", "10.0.0.2/32", 0),
	})

	clearWGMigrationHistory(t)
	if err := seedWireguardPeersToClients(); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := seedWireguardPeersToClients(); err != nil {
		t.Fatalf("second run (history gate): %v", err)
	}
	clearWGMigrationHistory(t)
	if err := seedWireguardPeersToClients(); err != nil {
		t.Fatalf("third run (linkCount gate): %v", err)
	}

	var clientCount int64
	db.Model(&model.ClientInbound{}).Where("inbound_id = ?", in.Id).Count(&clientCount)
	if clientCount != 1 {
		t.Fatalf("expected exactly 1 link after repeated runs, got %d", clientCount)
	}
}

func TestSeedWireguardPeersToClientsSkipsNonWireguard(t *testing.T) {
	initWGMigrationDB(t)
	vless := &model.Inbound{UserId: 1, Port: 41001, Protocol: model.VLESS, Tag: "vless-x", Settings: `{"clients":[]}`}
	if err := db.Create(vless).Error; err != nil {
		t.Fatalf("create vless: %v", err)
	}
	clearWGMigrationHistory(t)
	if err := seedWireguardPeersToClients(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var linkCount int64
	db.Model(&model.ClientInbound{}).Where("inbound_id = ?", vless.Id).Count(&linkCount)
	if linkCount != 0 {
		t.Fatalf("vless inbound must be untouched, got %d links", linkCount)
	}
}

func TestSeedWireguardPeersToClientsMultiplePeers(t *testing.T) {
	initWGMigrationDB(t)
	in := createWGInbound(t, "wg-multi", 51824, []any{
		wgPeer("alpha", "p1", "pub1", "10.0.0.2/32", 0),
		wgPeer("beta", "p2", "pub2", "10.0.0.3/32", 0),
	})
	clearWGMigrationHistory(t)
	if err := seedWireguardPeersToClients(); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var links []model.ClientInbound
	if err := db.Where("inbound_id = ?", in.Id).Find(&links).Error; err != nil {
		t.Fatalf("load links: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}

	settings := reloadInboundSettings(t, in.Id)
	clients := settings["clients"].([]any)
	ips := map[string]bool{}
	emails := map[string]bool{}
	for _, c := range clients {
		m := c.(map[string]any)
		emails[m["email"].(string)] = true
		ip := m["allowedIPs"].([]any)[0].(string)
		ips[ip] = true
	}
	if len(ips) != 2 || len(emails) != 2 {
		t.Fatalf("expected distinct emails/ips, got emails=%v ips=%v", emails, ips)
	}
}
