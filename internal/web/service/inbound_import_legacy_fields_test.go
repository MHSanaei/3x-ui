package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Exports from v2.x store tgId as a string ("" when unset); the stored copies
// are healed by a startup migration, but an imported export never passes it.
func TestAddInboundAcceptsLegacyStringClientFields(t *testing.T) {
	setupConflictDB(t)
	settings := `{"clients":[` +
		`{"id":"11111111-1111-1111-1111-111111111111","email":"legacy-a","tgId":"","subId":"s-a","enable":true,"limitIp":0,"totalGB":0,"expiryTime":0,"reset":0},` +
		`{"id":"22222222-2222-2222-2222-222222222222","email":"legacy-b","tgId":"123456","subId":"s-b","enable":true,"limitIp":0,"totalGB":0,"expiryTime":0,"reset":0}` +
		`],"decryption":"none","fallbacks":[]}`
	in := makeImportInbound("in-9201-tcp", 9201, settings, nil)

	saved, _, err := (&InboundService{}).AddInbound(in)
	if err != nil {
		t.Fatalf("AddInbound: %v", err)
	}

	var rec model.ClientRecord
	if err := database.GetDB().Where("email = ?", "legacy-b").First(&rec).Error; err != nil {
		t.Fatalf("read client: %v", err)
	}
	if rec.TgID != 123456 {
		t.Fatalf("client tgId = %d, want 123456 parsed from the string", rec.TgID)
	}
	var stored struct {
		Clients []map[string]any `json:"clients"`
	}
	if err := json.Unmarshal([]byte(saved.Settings), &stored); err != nil {
		t.Fatalf("stored settings: %v", err)
	}
	for _, c := range stored.Clients {
		if _, isString := c["tgId"].(string); isString {
			t.Fatalf("stored settings keep a string tgId for %v: %v", c["email"], c["tgId"])
		}
	}
}
