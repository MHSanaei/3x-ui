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
		{Kind: model.ExternalLinkKindLink, Value: "trojan://pw@example.com:443#on", Enable: externalLinkBool(true)},
		{Kind: model.ExternalLinkKindSubscription, Value: "https://provider.example/sub", Enable: externalLinkBool(false)},
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
}
