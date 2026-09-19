package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestSetExternalLinksPersistsEnableState(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	svc := &ClientService{}

	rec := model.ClientRecord{Email: "links@example.com", SubID: "sub-links", UUID: "uuid", Enable: true}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	if err := svc.SetExternalLinksForRecord(rec.Id, []ExternalLinkInput{
		{Kind: model.ExternalLinkKindLink, Value: "trojan://pw@example.com:443#on", Remark: "Primary", Enable: new(true), ExpiryTime: 1767225600000},
		{Kind: model.ExternalLinkKindSubscription, Value: "https://provider.example/sub", Remark: "Provider", Enable: new(false), NamePrefix: "[zjh] "},
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
	row := model.ExternalLink{
		Kind:           model.ExternalLinkKindSubscription,
		Value:          "https://provider.example/sub",
		Remark:         "old",
		LastFetchAt:    1767220000000,
		LastFetchError: "timeout",
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create library link: %v", err)
	}
	assignLibraryLink(t, row.Id, model.ExternalLinkTargetClient, rec.Id, "old")

	if err := svc.SetExternalLinksForRecord(rec.Id, []ExternalLinkInput{
		{Kind: row.Kind, Value: row.Value, Remark: "new", Enable: new(true)},
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

// TestSetExternalLinksRefusesInvalidRowsAndDropsBlankOnes pins the bar the client
// form saves through: a blank row is nothing to store, a repeated pair is one
// link, and a row the panel cannot serve is refused whole - the client keeps the
// links it already had instead of a half-applied save.
func TestSetExternalLinksRefusesInvalidRowsAndDropsBlankOnes(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	svc := &ClientService{}

	rec := model.ClientRecord{Email: "invalid@example.com", SubID: "sub-invalid", UUID: "uuid", Enable: true}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := svc.SetExternalLinksForRecord(rec.Id, []ExternalLinkInput{
		{Kind: model.ExternalLinkKindLink, Value: "trojan://kept@example.com:443#kept", Remark: "kept"},
	}); err != nil {
		t.Fatalf("seed the client's own link: %v", err)
	}

	if err := svc.SetExternalLinksForRecord(rec.Id, []ExternalLinkInput{
		{Kind: model.ExternalLinkKindLink, Value: "   "},
		{Kind: model.ExternalLinkKindLink, Value: "trojan://dup@example.com:443#dup"},
		{Kind: "", Value: "trojan://dup@example.com:443#dup"},
		{Kind: model.ExternalLinkKindLink, Value: "trojan://blank@example.com:443#blank", Remark: "  spaced  "},
	}); err != nil {
		t.Fatalf("save with a blank and a duplicate: %v", err)
	}
	rows, err := svc.GetExternalLinksForRecord(rec.Id)
	if err != nil {
		t.Fatalf("get external links: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2: the blank is dropped and the repeated (kind, value) is one link", len(rows))
	}
	if rows[1].Remark != "spaced" {
		t.Fatalf("remark = %q, want the trimmed one", rows[1].Remark)
	}

	refusals := []struct {
		name  string
		input ExternalLinkInput
		want  string
	}{
		{
			name:  "link that does not parse",
			input: ExternalLinkInput{Kind: model.ExternalLinkKindLink, Value: "not-a-link"},
			want:  "unsupported or invalid share link: not-a-link",
		},
		{
			name:  "subscription over ftp",
			input: ExternalLinkInput{Kind: model.ExternalLinkKindSubscription, Value: "ftp://provider.example/sub"},
			want:  "external subscription must be an http(s) URL: ftp://provider.example/sub",
		},
		{
			name:  "unknown kind",
			input: ExternalLinkInput{Kind: "bogus", Value: "trojan://pw@example.com:443#x"},
			want:  "unknown external link kind: bogus",
		},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			// The refused row rides beside a valid one: the save must land neither.
			err := svc.SetExternalLinksForRecord(rec.Id, []ExternalLinkInput{
				{Kind: model.ExternalLinkKindLink, Value: "trojan://new@example.com:443#new"},
				tc.input,
			})
			if err == nil || strings.TrimSpace(err.Error()) != tc.want {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			rows, err := svc.GetExternalLinksForRecord(rec.Id)
			if err != nil {
				t.Fatalf("get external links: %v", err)
			}
			if len(rows) != 2 {
				t.Fatalf("rows after the refused save = %d, want the client's two previous links", len(rows))
			}
			for _, row := range rows {
				if strings.Contains(row.Value, "new@example.com") {
					t.Fatalf("the refused save stored %q: the valid row beside it must not land either", row.Value)
				}
			}
		})
	}
}

func TestSetExternalLinksRejectsNegativeExpiry(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	svc := &ClientService{}

	rec := model.ClientRecord{Email: "negative@example.com", SubID: "sub-negative", UUID: "uuid", Enable: true}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	err := svc.SetExternalLinksForRecord(rec.Id, []ExternalLinkInput{
		{Kind: model.ExternalLinkKindLink, Value: "trojan://pw@example.com:443#neg", ExpiryTime: -86400000},
	})
	want := "external link expiryTime must be 0 (never) or a future unix millisecond timestamp: trojan://pw@example.com:443#neg\n"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}

	rows, err := svc.GetExternalLinksForRecord(rec.Id)
	if err != nil {
		t.Fatalf("get external links: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %d, want the rejected save to persist nothing", len(rows))
	}
}
