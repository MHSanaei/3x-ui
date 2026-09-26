package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// The legacy table holds one row per client, so the same URL can carry its own
// enable, expiry and order per client — the migration must not lose any of it.
func TestMigrateClientExternalLinksToLibraryPreservesPerClientState(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	// A fresh install never creates the legacy table, so the migration is a
	// no-op there — the table a real upgrade carries is built by hand here.
	if err := db.AutoMigrate(&model.ClientExternalLink{}); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	yes, no := true, false
	legacy := []model.ClientExternalLink{
		{ClientId: 1, Kind: model.ExternalLinkKindLink, Value: "trojan://a", Remark: "A", Enable: &yes, ExpiryTime: 1767225600000, SortIndex: 0, LastFetchAt: 111},
		{ClientId: 2, Kind: model.ExternalLinkKindLink, Value: "trojan://a", NamePrefix: "[x] ", Enable: &no, ExpiryTime: 0, SortIndex: 3},
		{ClientId: 3, Kind: model.ExternalLinkKindSubscription, Value: "https://provider.example/sub", Remark: "Provider", Enable: &yes, SortIndex: 1},
	}
	for i := range legacy {
		if err := db.Create(&legacy[i]).Error; err != nil {
			t.Fatalf("seed legacy row %d: %v", i, err)
		}
	}

	if err := migrateClientExternalLinksToLibrary(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var links []model.ExternalLink
	if err := db.Order("id ASC").Find(&links).Error; err != nil {
		t.Fatalf("read library: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("library rows = %d, want 2 (the shared URL must collapse)", len(links))
	}
	shared := links[0]
	// The shared row must carry no client's expiry: the first legacy row's
	// timestamp here would silently expire every other client's link.
	if shared.Kind != model.ExternalLinkKindLink || shared.Value != "trojan://a" ||
		shared.Remark != "A" || shared.NamePrefix != "" || shared.SortIndex != 0 ||
		shared.LastFetchAt != 111 || shared.ExpiryTime != 0 {
		t.Errorf("library row = %+v, want the first legacy row's library state and no per-client expiry", shared)
	}
	if shared.Enable == nil || !*shared.Enable {
		t.Errorf("library enable = %v, want true", shared.Enable)
	}
	if sub := links[1]; sub.Kind != model.ExternalLinkKindSubscription || sub.Value != "https://provider.example/sub" || sub.Remark != "Provider" {
		t.Errorf("library row = %+v, want the provider subscription", sub)
	}

	var assignments []model.ExternalLinkAssignment
	if err := db.Order("link_id ASC, target_id ASC").Find(&assignments).Error; err != nil {
		t.Fatalf("read assignments: %v", err)
	}
	if len(assignments) != 3 {
		t.Fatalf("assignments = %d, want one per legacy row", len(assignments))
	}
	for _, a := range assignments {
		if a.TargetType != model.ExternalLinkTargetClient || a.Origin != model.ExternalLinkOriginPanel {
			t.Errorf("assignment %+v is not a panel client binding", a)
		}
	}
	first := assignments[0]
	if first.TargetId != 1 || first.Remark != "A" || first.SortIndex != 0 || first.ExpiryTime != 1767225600000 {
		t.Errorf("client 1 assignment = %+v, want its own naming, order and expiry", first)
	}
	second := assignments[1]
	// Client 2 had expiry_time = 0, which has always meant "never": the
	// sentinel keeps that distinct from inheriting client 1's timestamp.
	if second.TargetId != 2 || second.NamePrefix != "[x] " || second.SortIndex != 3 ||
		second.ExpiryTime != model.ExternalLinkExpiryNever {
		t.Errorf("client 2 assignment = %+v, want its own prefix, order and a never-expiring expiry", second)
	}
	if second.Enable == nil || *second.Enable {
		t.Errorf("client 2 enable = %v, want the disabled state it carried", second.Enable)
	}
	if second.Remark != "" {
		t.Errorf("client 2 remark = %q, want empty so it inherits the library name", second.Remark)
	}
	if third := assignments[2]; third.TargetId != 3 || third.Remark != "Provider" {
		t.Errorf("client 3 assignment = %+v, want the provider row", third)
	}

	if !db.Migrator().HasTable(clientExternalLinkLegacyTable) {
		t.Fatal("legacy table was not renamed, so the migration will run again on every boot")
	}
	if db.Migrator().HasTable("client_external_links") {
		t.Fatal("legacy table still answers to its old name")
	}
}

// Re-running the row pass must not duplicate anything: the library row is
// reused by (kind, value) and the assignment insert is a no-op conflict.
func TestMigrateClientExternalLinksToLibraryIsIdempotent(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	if err := db.AutoMigrate(&model.ClientExternalLink{}); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	yes := true
	if err := db.Create(&model.ClientExternalLink{
		ClientId: 7, Kind: model.ExternalLinkKindLink, Value: "ss://b", Enable: &yes, SortIndex: 2,
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := migrateClientExternalLinkRows(); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	var links int64
	if err := db.Model(&model.ExternalLink{}).Count(&links).Error; err != nil {
		t.Fatalf("count library: %v", err)
	}
	var assignments int64
	if err := db.Model(&model.ExternalLinkAssignment{}).Count(&assignments).Error; err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	if links != 1 || assignments != 1 {
		t.Fatalf("library/assignments = %d/%d after three runs, want 1/1", links, assignments)
	}
}

// An install that only ever ran the library build has no legacy table at all,
// so the migration must not create one or invent rows.
func TestMigrateClientExternalLinksToLibrarySkipsAFreshInstall(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	if err := migrateClientExternalLinksToLibrary(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var links int64
	if err := db.Model(&model.ExternalLink{}).Count(&links).Error; err != nil {
		t.Fatalf("count library: %v", err)
	}
	if links != 0 {
		t.Fatalf("library rows = %d, want 0 on a fresh install", links)
	}
	if db.Migrator().HasTable(clientExternalLinkLegacyTable) {
		t.Fatal("a fresh install must not end up with a legacy table")
	}
}
