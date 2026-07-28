package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestAPIUserFromClientCarriesShadowsocksCipher pins what the renew path must
// hand the runtime API. A shadowsocks client object holds no cipher of its own,
// and xray's legacy and 2022 inbounds take different account types that they
// cast without checking — an account built for the wrong one panics the core.
func TestAPIUserFromClientCarriesShadowsocksCipher(t *testing.T) {
	client := map[string]any{"email": "a@x", "password": "pw", "method": "aes-256-gcm"}

	user := apiUserFromClient(client, "aes-256-gcm")
	if got := user["cipher"]; got != "aes-256-gcm" {
		t.Fatalf("cipher = %v, want the inbound method", got)
	}
	if _, polluted := client["cipher"]; polluted {
		t.Fatal("the stored client object was mutated; the API-only cipher would be persisted into the inbound settings")
	}

	user["email"] = "changed@x"
	if client["email"] != "a@x" {
		t.Fatal("the API user shares storage with the stored client object")
	}
}

func TestAPIUserFromClientWithoutCipher(t *testing.T) {
	client := map[string]any{"email": "a@x", "id": "11111111-1111-1111-1111-111111111111"}
	user := apiUserFromClient(client, "")
	if _, ok := user["cipher"]; ok {
		t.Fatal("a non-shadowsocks client must not gain a cipher key")
	}
	if user["id"] != client["id"] {
		t.Fatalf("id = %v, want it copied from the client", user["id"])
	}
}

// TestAutoRenewShadowsocksKeepsSettingsClean renews a shadowsocks client and
// checks the inbound settings the panel writes back: the cipher the API needs
// must not leak into a stored client object, where it would end up in the
// generated xray config as a per-user key.
func TestAutoRenewShadowsocksKeepsSettingsClean(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	past := time.Now().Add(-48 * time.Hour).UnixMilli()
	clients := []model.Client{
		{Email: "ss@x", Password: "pw", Enable: false, Reset: 30, ExpiryTime: past},
	}

	settings := map[string]any{
		"method":  "aes-256-gcm",
		"network": "tcp,udp",
		"clients": []map[string]any{
			{"email": "ss@x", "password": "pw", "method": "aes-256-gcm", "enable": false, "expiryTime": past, "reset": 30},
		},
	}
	raw, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	ib := mkInbound(t, 30011, model.Shadowsocks, string(raw))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := db.Create(&[]xray.ClientTraffic{
		{InboundId: ib.Id, Email: "ss@x", Enable: false, Up: 10, Down: 20, Reset: 30, ExpiryTime: past},
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	if _, count, err := svc.autoRenewClients(db); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	} else if count != 1 {
		t.Fatalf("renewed count = %d, want 1", count)
	}

	var stored model.Inbound
	if err := db.Where("id = ?", ib.Id).First(&stored).Error; err != nil {
		t.Fatalf("read inbound: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(stored.Settings), &parsed); err != nil {
		t.Fatalf("unmarshal stored settings: %v", err)
	}
	storedClients, _ := parsed["clients"].([]any)
	if len(storedClients) != 1 {
		t.Fatalf("stored clients = %d, want 1", len(storedClients))
	}
	client, _ := storedClients[0].(map[string]any)
	if _, polluted := client["cipher"]; polluted {
		t.Fatalf("the renewed client was persisted with an API-only cipher key: %+v", client)
	}
	if client["method"] != "aes-256-gcm" {
		t.Fatalf("the client's method was lost: %+v", client)
	}
	if enabled, _ := client["enable"].(bool); !enabled {
		t.Fatalf("the renewed client was not re-enabled: %+v", client)
	}
}
