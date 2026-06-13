package sub

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestGetSubs_DuplicateSettingsClients_Deduped reproduces #5134: multi-node
// sync/import drift can leave the same client twice inside an inbound's
// legacy settings.clients JSON while the normalized client_inbounds table
// stays clean. The subscription output must still contain one profile per
// (inbound, client).
func TestGetSubs_DuplicateSettingsClients_Deduped(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const subId = "sub-dup"
	const email = "dup@example.com"
	const uuid = "f1b9265f-26a8-4b75-9be2-c64a94b15de1"

	db := database.GetDB()
	settings := fmt.Sprintf(`{"clients": [
		{"id": %q, "email": %q, "subId": %q, "enable": true},
		{"id": %q, "email": %q, "subId": %q, "enable": true}
	]}`, uuid, email, subId, uuid, email, subId)
	ib := &model.Inbound{
		UserId:         1,
		Tag:            "dup-in",
		Enable:         true,
		Port:           42001,
		Protocol:       model.VLESS,
		Settings:       settings,
		StreamSettings: `{"network": "tcp", "security": "none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := &model.ClientRecord{Email: email, SubID: subId, UUID: uuid, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound: %v", err)
	}

	s := NewSubService(false, "-ieo")
	links, emails, _, _, err := s.GetSubs(subId, "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("links = %d, want 1 (duplicate settings.clients entries must collapse)", len(links))
	}
	if len(emails) != 1 {
		t.Fatalf("emails = %d, want 1, got %v", len(emails), emails)
	}
}
