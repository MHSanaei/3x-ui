package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// A settings blob can carry a malformed clients entry (hand-edited JSON, a
// half-migrated import). The traffic tick must skip it instead of panicking,
// and must keep the entry rather than silently dropping user data.
func TestAdjustTrafficsMalformedClientEntryDoesNotPanic(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	client := &model.ClientRecord{Email: "a@x", SubID: "s-adjust"}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	settings := "{\"clients\":[\"bogus-entry\",{\"email\":\"a@x\"}]}"
	inbound := &model.Inbound{UserId: 1, Tag: "adjust", Enable: true, Port: 8443, Protocol: model.VLESS, Settings: settings}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatalf("link client: %v", err)
	}

	traffics := []*xray.ClientTraffic{{Email: "a@x", ExpiryTime: -3600000}}
	updated, _, err := (&InboundService{}).adjustTraffics(db, traffics)
	if err != nil {
		t.Fatalf("adjustTraffics: %v", err)
	}
	if updated[0].ExpiryTime <= 0 {
		t.Fatalf("expiry was not converted: %d", updated[0].ExpiryTime)
	}

	var stored model.Inbound
	if err := db.First(&stored, inbound.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(stored.Settings), &parsed); err != nil {
		t.Fatalf("settings no longer parse: %v", err)
	}
	entries := parsed["clients"].([]any)
	if len(entries) != 2 {
		t.Fatalf("clients entries = %d, want 2 preserved: %v", len(entries), entries)
	}
	if entries[0] != "bogus-entry" {
		t.Fatalf("malformed entry was rewritten: %v", entries[0])
	}
}
