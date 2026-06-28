package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestResetClientExpiryTimeByEmail_MultiInbound reproduces #5039: a client
// attached to several inbounds had its expiry patched only on the first
// inbound's JSON, so the stale siblings reverted the change on the next sync.
func TestResetClientExpiryTimeByEmail_MultiInbound(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	const email = "multi@example.com"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c111"
	const oldExpiry = int64(1700000000000)
	const newExpiry = int64(1800000000000)

	clientJSON := func(expiry int64) string {
		b, _ := json.Marshal(map[string]any{"clients": []map[string]any{{
			"email": email, "id": uid, "enable": true, "expiryTime": expiry, "subId": "sub-multi-1",
		}}})
		return string(b)
	}

	first := &model.Inbound{
		Tag: "vless-a", Enable: true, Port: 50001, Protocol: model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality"}`, Settings: clientJSON(oldExpiry),
	}
	second := &model.Inbound{
		Tag: "vless-b", Enable: true, Port: 50002, Protocol: model.VLESS,
		StreamSettings: `{"network":"ws","security":"tls"}`, Settings: clientJSON(oldExpiry),
	}
	for _, ib := range []*model.Inbound{first, second} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", ib.Tag, err)
		}
	}

	clientSvc := ClientService{}
	inboundSvc := InboundService{}
	for _, ib := range []*model.Inbound{first, second} {
		clients, err := inboundSvc.GetClients(ib)
		if err != nil {
			t.Fatalf("GetClients(%s): %v", ib.Tag, err)
		}
		if err := clientSvc.SyncInbound(nil, ib.Id, clients); err != nil {
			t.Fatalf("SyncInbound(%s): %v", ib.Tag, err)
		}
	}

	if _, err := clientSvc.ResetClientExpiryTimeByEmail(&inboundSvc, email, newExpiry); err != nil {
		t.Fatalf("ResetClientExpiryTimeByEmail: %v", err)
	}

	for _, ib := range []*model.Inbound{first, second} {
		fresh, err := inboundSvc.GetInbound(ib.Id)
		if err != nil {
			t.Fatalf("GetInbound(%s): %v", ib.Tag, err)
		}
		clients, err := inboundSvc.GetClients(fresh)
		if err != nil {
			t.Fatalf("GetClients(%s): %v", ib.Tag, err)
		}
		if len(clients) != 1 || clients[0].ExpiryTime != newExpiry {
			t.Errorf("inbound %s settings expiry = %d, want %d (#5039)", ib.Tag, clients[0].ExpiryTime, newExpiry)
		}
	}

	rec, err := clientSvc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.ExpiryTime != newExpiry {
		t.Errorf("client record expiry = %d, want %d", rec.ExpiryTime, newExpiry)
	}
}
