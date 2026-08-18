package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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
