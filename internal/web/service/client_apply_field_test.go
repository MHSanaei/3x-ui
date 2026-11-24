package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestResetClientExpiryTimeByEmail_MultiInbound reproduces #5039: a client
// attached to several inbounds had its expiry patched only on the first
// inbound's JSON, so the stale siblings reverted the change on the next sync.
func TestResetClientExpiryTimeByEmail_MultiInbound(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

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

func TestSetClientEnableByEmail_MultiInbound(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

	db := database.GetDB()

	const email = "multienable@example.com"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c222"
	clientJSON := `{"clients":[{"email":"` + email + `","id":"` + uid + `","enable":true,"subId":"sub-en-1"}]}`

	first := &model.Inbound{
		Tag: "vless-en-a", Enable: true, Port: 50011, Protocol: model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality"}`, Settings: clientJSON,
	}
	second := &model.Inbound{
		Tag: "vless-en-b", Enable: true, Port: 50012, Protocol: model.VLESS,
		StreamSettings: `{"network":"ws","security":"tls"}`, Settings: clientJSON,
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
	if err := db.Create(&xray.ClientTraffic{InboundId: first.Id, Email: email, Enable: true}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	if _, _, err := clientSvc.SetClientEnableByEmail(&inboundSvc, email, false); err != nil {
		t.Fatalf("SetClientEnableByEmail: %v", err)
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
		if len(clients) != 1 || clients[0].Enable {
			t.Errorf("inbound %s: client still enabled after disable-by-email; a sibling inbound kept access", ib.Tag)
		}
	}
}

// Regression test: removing a client's Telegram user cleared tgId on one inbound
// only, so the next sync of the sibling bound the removed account again.
func TestSetClientTelegramUserID_MultiInbound(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

	db := database.GetDB()

	const email = "multitg@example.com"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c333"
	clientJSON := `{"clients":[{"email":"` + email + `","id":"` + uid + `","enable":true,"subId":"sub-tg-1","tgId":4242}]}`

	first := &model.Inbound{
		Tag: "vless-tg-a", Enable: true, Port: 50021, Protocol: model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality"}`, Settings: clientJSON,
	}
	second := &model.Inbound{
		Tag: "vless-tg-b", Enable: true, Port: 50022, Protocol: model.VLESS,
		StreamSettings: `{"network":"ws","security":"tls"}`, Settings: clientJSON,
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
	traffic := &xray.ClientTraffic{InboundId: first.Id, Email: email, Enable: true}
	if err := db.Create(traffic).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	if _, err := clientSvc.SetClientTelegramUserID(&inboundSvc, traffic.Id, 0); err != nil {
		t.Fatalf("SetClientTelegramUserID: %v", err)
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
		if len(clients) != 1 || clients[0].TgID != 0 {
			t.Errorf("inbound %s still binds the removed Telegram user: %+v", ib.Tag, clients)
		}
		if err := clientSvc.SyncInbound(nil, ib.Id, clients); err != nil {
			t.Fatalf("SyncInbound(%s): %v", ib.Tag, err)
		}
	}

	rec, err := clientSvc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.TgID != 0 {
		t.Errorf("client record tgId = %d after the siblings were synced, want 0", rec.TgID)
	}
}
