package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func externalLinkBool(v bool) *bool {
	return &v
}

func TestSetExternalLinksPersistsEnableState(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	svc := &ClientService{}

	rec := model.ClientRecord{Email: "links@example.com", SubID: "sub-links", UUID: "uuid", Enable: true}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	if err := svc.SetExternalLinksForRecord(rec.Id, []ExternalLinkInput{
		{Kind: model.ExternalLinkKindLink, Value: "trojan://pw@example.com:443#on", Remark: "Primary", Enable: externalLinkBool(true), ExpiryTime: 1767225600000},
		{Kind: model.ExternalLinkKindSubscription, Value: "https://provider.example/sub", Remark: "Provider", Enable: externalLinkBool(false), NamePrefix: "[zjh] "},
		{Kind: model.ExternalLinkKindLink, Value: "trojan://pw@example.net:443#default"},
	}); err != nil {
		t.Fatalf("set external links: %v", err)
	}

	rows, err := svc.GetExternalLinksForRecord(rec.Id)
	if err != nil {
		t.Fatalf("get external links: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if rows[0].Enable == nil || *rows[0].Enable != true {
		t.Fatalf("first row enable = %#v, want true", rows[0].Enable)
	}
	if rows[1].Enable == nil || *rows[1].Enable != false {
		t.Fatalf("second row enable = %#v, want false", rows[1].Enable)
	}
	if rows[2].Enable == nil || *rows[2].Enable != true {
		t.Fatalf("omitted enable should default true, got %#v", rows[2].Enable)
	}
	if rows[0].Remark != "Primary" || rows[0].ExpiryTime != 1767225600000 {
		t.Fatalf("first row fields not persisted: %#v", rows[0])
	}
	if rows[1].Remark != "Provider" || rows[1].NamePrefix != "[zjh] " {
		t.Fatalf("subscription fields not persisted: %#v", rows[1])
	}
}

func TestSetExternalLinksPreservesFetchStatus(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	svc := &ClientService{}

	rec := model.ClientRecord{Email: "status@example.com", SubID: "sub-status", UUID: "uuid", Enable: true}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	row := model.ClientExternalLink{
		ClientId:       rec.Id,
		Kind:           model.ExternalLinkKindSubscription,
		Value:          "https://provider.example/sub",
		Remark:         "old",
		LastFetchAt:    1767220000000,
		LastFetchError: "timeout",
		SortIndex:      0,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create external link: %v", err)
	}

	if err := svc.SetExternalLinksForRecord(rec.Id, []ExternalLinkInput{
		{Id: row.Id, Kind: row.Kind, Value: row.Value, Remark: "new", Enable: externalLinkBool(true)},
	}); err != nil {
		t.Fatalf("set external links: %v", err)
	}

	rows, err := svc.GetExternalLinksForRecord(rec.Id)
	if err != nil {
		t.Fatalf("get external links: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].LastFetchAt != row.LastFetchAt || rows[0].LastFetchError != row.LastFetchError {
		t.Fatalf("fetch status not preserved: %#v", rows[0])
	}
	if rows[0].Remark != "new" {
		t.Fatalf("editable fields not updated: %#v", rows[0])
	}
}
