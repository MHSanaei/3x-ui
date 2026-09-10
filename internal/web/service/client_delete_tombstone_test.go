package service

import (
	"fmt"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// Delete tombstones up front and keeps the record when an inbound fails. A
// surviving tombstone lets the next node merge finish the refused deletion.
func TestFailedDeleteWithdrawsTombstone(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	db := database.GetDB()

	const email = "retry@x"
	broken := mkInbound(t, 30401, model.VLESS, `{"clients": [ THIS IS NOT JSON`)
	rec := &model.ClientRecord{Email: email, Enable: true, UUID: "33333333-3333-3333-3333-333333333333"}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client record: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: broken.Id}).Error; err != nil {
		t.Fatalf("attach client: %v", err)
	}

	t.Cleanup(func() { withdrawClientTombstones(email) })

	if _, err := svc.Delete(inboundSvc, rec.Id, false); err == nil {
		t.Fatal("setup: delete was expected to fail on the unparseable inbound settings")
	}

	var surviving int64
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", email).Count(&surviving).Error; err != nil {
		t.Fatalf("count clients: %v", err)
	}
	if surviving != 1 {
		t.Fatalf("failed delete must keep the record for a retry, got %d rows", surviving)
	}
	if isClientEmailTombstoned(email) {
		t.Fatal("delete kept the record but left a live tombstone: the next node sync would finish the deletion it refused")
	}
}

// A client re-created under a just-deleted email is a live identity again. If
// the tombstone outlives it, the node merge prunes the new client's link.
func TestRecreatedClientSurvivesNodeMerge(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	const email = "reborn@x"
	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	createNodeInboundWithClient(t, db, 1, "n1-in", 41501, email)
	nodeSettings := fmt.Sprintf(`{"clients": [{"email": %q, "enable": true}]}`, email)
	syncNodeWithSettings(t, inboundSvc, 1, "n1-in", nodeSettings, xray.ClientTraffic{Email: email, Enable: true})

	var ib model.Inbound
	if err := db.Where("tag = ?", "n1-in").First(&ib).Error; err != nil {
		t.Fatalf("load inbound: %v", err)
	}

	rec := &model.ClientRecord{}
	if err := db.Where("email = ?", email).First(rec).Error; err != nil {
		t.Fatalf("load adopted client record: %v", err)
	}
	t.Cleanup(func() { withdrawClientTombstones(email) })
	if _, err := svc.Delete(inboundSvc, rec.Id, false); err != nil {
		t.Fatalf("delete client: %v", err)
	}

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: email, Enable: true, ID: "44444444-4444-4444-4444-444444444444"},
		InboundIds: []int{ib.Id},
	}); err != nil {
		t.Fatalf("re-create client: %v", err)
	}

	// The create marks the node config-dirty, which parks the client merge; the
	// reconcile clears it a tick later, well inside the 90s tombstone window.
	if err := db.Model(&model.Node{}).Where("id = ?", 1).Update("config_dirty", false).Error; err != nil {
		t.Fatalf("clear config_dirty: %v", err)
	}
	snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{{
		Tag: "n1-in", Protocol: model.VLESS, Settings: nodeSettings,
		ClientStats: []xray.ClientTraffic{{Email: email, Enable: true}},
	}}}
	if _, err := inboundSvc.setRemoteTrafficLocked(1, snap, false, false); err != nil {
		t.Fatalf("node merge: %v", err)
	}

	var links int64
	if err := db.Model(&model.ClientInbound{}).
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("clients.email = ? AND client_inbounds.inbound_id = ?", email, ib.Id).
		Count(&links).Error; err != nil {
		t.Fatalf("count client links: %v", err)
	}
	if links != 1 {
		t.Fatalf("re-created client has %d inbound links after the node merge, want 1: the delete tombstone outlived the email and the merge pruned it", links)
	}
}
