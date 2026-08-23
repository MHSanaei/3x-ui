package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// stampLinkCreatedAt marks every link of an inbound with a sentinel timestamp.
// A row that still carries it afterwards was not deleted and re-inserted.
func stampLinkCreatedAt(t *testing.T, inboundId int) {
	t.Helper()
	err := database.GetDB().Model(&model.ClientInbound{}).
		Where("inbound_id = ?", inboundId).
		UpdateColumn("created_at", 1).Error
	if err != nil {
		t.Fatalf("stamp created_at: %v", err)
	}
}

func linksOf(t *testing.T, inboundId int) map[int]model.ClientInbound {
	t.Helper()
	var rows []model.ClientInbound
	if err := database.GetDB().Where("inbound_id = ?", inboundId).Find(&rows).Error; err != nil {
		t.Fatalf("load links: %v", err)
	}
	out := make(map[int]model.ClientInbound, len(rows))
	for _, r := range rows {
		out[r.ClientId] = r
	}
	return out
}

func recordID(t *testing.T, email string) int {
	t.Helper()
	var rec model.ClientRecord
	if err := database.GetDB().Where("email = ?", email).First(&rec).Error; err != nil {
		t.Fatalf("record %q: %v", email, err)
	}
	return rec.Id
}

// A re-sync that changes one client's flow must leave the other links in place
// and UPDATE the changed one, not rebuild the whole membership set (#6252).
func TestSyncInboundReusesUnchangedLinkRows(t *testing.T) {
	setupBulkDB(t)
	cs := &ClientService{}

	seed := []model.Client{
		{ID: "id-a", Email: "a@x", Enable: true, SubID: "s-a"},
		{ID: "id-b", Email: "b@x", Enable: true, SubID: "s-b", Flow: "xtls-rprx-vision"},
		{ID: "id-c", Email: "c@x", Enable: true, SubID: "s-c"},
	}
	ib := mkInbound(t, 21001, model.VLESS, clientsSettings(t, seed))
	if err := cs.SyncInbound(nil, ib.Id, seed); err != nil {
		t.Fatalf("seed SyncInbound: %v", err)
	}
	stampLinkCreatedAt(t, ib.Id)

	changed := make([]model.Client, len(seed))
	copy(changed, seed)
	changed[1].Flow = ""
	if err := cs.SyncInbound(nil, ib.Id, changed); err != nil {
		t.Fatalf("re-sync: %v", err)
	}

	links := linksOf(t, ib.Id)
	if len(links) != 3 {
		t.Fatalf("link count = %d, want 3", len(links))
	}
	for _, email := range []string{"a@x", "b@x", "c@x"} {
		link, ok := links[recordID(t, email)]
		if !ok {
			t.Fatalf("%s lost its link", email)
		}
		if link.CreatedAt != 1 {
			t.Errorf("%s link created_at = %d, want the 1 sentinel: the row was deleted and re-inserted", email, link.CreatedAt)
		}
	}
	if got := links[recordID(t, "b@x")].FlowOverride; got != "" {
		t.Errorf("b@x flow_override = %q, want \"\" (cleared in place)", got)
	}

	// Dropping a client must still remove exactly that one link.
	if err := cs.SyncInbound(nil, ib.Id, []model.Client{seed[0], seed[2]}); err != nil {
		t.Fatalf("prune sync: %v", err)
	}
	links = linksOf(t, ib.Id)
	if len(links) != 2 {
		t.Fatalf("after prune link count = %d, want 2", len(links))
	}
	if _, still := links[recordID(t, "b@x")]; still {
		t.Error("b@x link survived a full sync that dropped it")
	}
	for _, email := range []string{"a@x", "c@x"} {
		if links[recordID(t, email)].CreatedAt != 1 {
			t.Errorf("%s link was rebuilt by the prune sync", email)
		}
	}
}

// Adding a client must not re-merge its bystanders' records from the settings
// JSON; comment lives only in the clients table, so a full sync erases it.
func TestAddInboundClientLeavesBystanderRecordsUntouched(t *testing.T) {
	setupBulkDB(t)
	cs := &ClientService{}
	is := &InboundService{}

	seed := []model.Client{
		{ID: "id-a", Email: "a@x", Enable: true, SubID: "s-a"},
		{ID: "id-b", Email: "b@x", Enable: true, SubID: "s-b"},
	}
	ib := mkInbound(t, 21002, model.VLESS, clientsSettings(t, seed))
	if err := cs.SyncInbound(nil, ib.Id, seed); err != nil {
		t.Fatalf("seed SyncInbound: %v", err)
	}
	db := database.GetDB()
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "a@x").
		UpdateColumn("comment", "operator note").Error; err != nil {
		t.Fatalf("set comment: %v", err)
	}
	stampLinkCreatedAt(t, ib.Id)

	add := []model.Client{{ID: "id-c", Email: "c@x", Enable: true, SubID: "s-c"}}
	if _, err := cs.AddInboundClient(is, &model.Inbound{Id: ib.Id, Settings: clientsSettings(t, add)}); err != nil {
		t.Fatalf("AddInboundClient: %v", err)
	}

	var bystander model.ClientRecord
	if err := db.Where("email = ?", "a@x").First(&bystander).Error; err != nil {
		t.Fatalf("reload a@x: %v", err)
	}
	if bystander.Comment != "operator note" {
		t.Errorf("bystander comment = %q, want %q: the add re-merged an unrelated record from settings JSON",
			bystander.Comment, "operator note")
	}

	links := linksOf(t, ib.Id)
	if len(links) != 3 {
		t.Fatalf("link count = %d, want 3", len(links))
	}
	for _, email := range []string{"a@x", "b@x"} {
		if links[recordID(t, email)].CreatedAt != 1 {
			t.Errorf("%s link was rebuilt by an unrelated add", email)
		}
	}
	newLink, ok := links[recordID(t, "c@x")]
	if !ok {
		t.Fatal("c@x got no link")
	}
	if newLink.CreatedAt == 1 {
		t.Error("c@x link carries the sentinel; it should be freshly inserted")
	}
}

// Deleting one client detaches only that client; the others keep both their
// link rows and the record fields that live only in the clients table.
func TestDelInboundClientDetachesOnlyTheRemovedClient(t *testing.T) {
	setupBulkDB(t)
	cs := &ClientService{}
	is := &InboundService{}

	seed := []model.Client{
		{ID: "id-a", Email: "a@x", Enable: true, SubID: "s-a"},
		{ID: "id-b", Email: "b@x", Enable: true, SubID: "s-b"},
		{ID: "id-c", Email: "c@x", Enable: true, SubID: "s-c"},
	}
	ib := mkInbound(t, 21003, model.VLESS, clientsSettings(t, seed))
	if err := cs.SyncInbound(nil, ib.Id, seed); err != nil {
		t.Fatalf("seed SyncInbound: %v", err)
	}
	db := database.GetDB()
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "c@x").
		UpdateColumn("comment", "keep me").Error; err != nil {
		t.Fatalf("set comment: %v", err)
	}
	removedID := recordID(t, "b@x")
	stampLinkCreatedAt(t, ib.Id)

	if _, err := cs.DelInboundClientByEmail(is, ib.Id, "b@x", true, false); err != nil {
		t.Fatalf("DelInboundClientByEmail: %v", err)
	}

	links := linksOf(t, ib.Id)
	if _, still := links[removedID]; still {
		t.Error("b@x link survived the delete")
	}
	if len(links) != 2 {
		t.Fatalf("link count = %d, want 2", len(links))
	}
	for _, email := range []string{"a@x", "c@x"} {
		if links[recordID(t, email)].CreatedAt != 1 {
			t.Errorf("%s link was rebuilt by an unrelated delete", email)
		}
	}
	// Detach must not delete the record itself.
	var removed model.ClientRecord
	if err := db.Where("email = ?", "b@x").First(&removed).Error; err != nil {
		t.Fatalf("b@x record should survive a detach: %v", err)
	}
	var kept model.ClientRecord
	if err := db.Where("email = ?", "c@x").First(&kept).Error; err != nil {
		t.Fatalf("reload c@x: %v", err)
	}
	if kept.Comment != "keep me" {
		t.Errorf("bystander comment = %q, want %q", kept.Comment, "keep me")
	}
}
