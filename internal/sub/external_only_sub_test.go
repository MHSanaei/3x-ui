package sub

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// A subscription whose only entries are external links — no enabled standard
// inbound — must still render in the JSON and Clash formats, not just the raw
// one. Regression guard for the premature len(inbounds)==0 early return that
// short-circuited GetJson/GetClash before external links were ever fetched.
func TestJsonAndClashServeExternalLinkOnlySub(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()

	rec := &model.ClientRecord{Email: "ext@x", SubID: "ext-only", UUID: "ext-uuid", Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	link := "vless://11111111-1111-1111-1111-111111111111@example.com:443?type=tcp&security=reality&pbk=abc&sid=12&fp=chrome#orig"
	if err := db.Create(&model.ClientExternalLink{ClientId: rec.Id, Kind: model.ExternalLinkKindLink, Value: link, Remark: "DE-Provider", SortIndex: 1}).Error; err != nil {
		t.Fatalf("seed external link: %v", err)
	}

	base := NewSubService("")

	jsonService := NewSubJsonService("", "", "", base)
	jsonOut, _, err := jsonService.GetJson("ext-only", "sub.example.com", false)
	if err != nil {
		t.Fatalf("GetJson err = %v", err)
	}
	if jsonOut == "" {
		t.Fatal("GetJson returned empty for an external-link-only sub")
	}
	if !strings.Contains(jsonOut, "DE-Provider") {
		t.Fatalf("GetJson missing external remark: %s", jsonOut)
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &config); err != nil {
		t.Fatalf("legacy GetJson must return an object for a single profile: %v; body=%s", err, jsonOut)
	}

	standardOut, _, err := jsonService.GetJson("ext-only", "sub.example.com", true)
	if err != nil {
		t.Fatalf("standards-compliant GetJson err = %v", err)
	}
	var configs []map[string]any
	if err := json.Unmarshal([]byte(standardOut), &configs); err != nil {
		t.Fatalf("standards-compliant GetJson must return an array for a single profile: %v; body=%s", err, standardOut)
	}
	if len(configs) != 1 {
		t.Fatalf("standards-compliant GetJson profile count = %d, want 1", len(configs))
	}

	clashOut, _, err := NewSubClashService(false, "", base).GetClash("ext-only", "sub.example.com")
	if err != nil {
		t.Fatalf("GetClash err = %v", err)
	}
	if clashOut == "" {
		t.Fatal("GetClash returned empty for an external-link-only sub")
	}
	if !strings.Contains(clashOut, "DE-Provider") {
		t.Fatalf("GetClash missing external proxy: %s", clashOut)
	}
}
