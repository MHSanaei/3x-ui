package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func mkTraffic(t *testing.T, inboundId int, email string, up, down, total, expiry int64, enable bool) {
	t.Helper()
	row := xray.ClientTraffic{
		InboundId:  inboundId,
		Email:      email,
		Up:         up,
		Down:       down,
		Total:      total,
		ExpiryTime: expiry,
		Enable:     enable,
	}
	if err := database.GetDB().Create(&row).Error; err != nil {
		t.Fatalf("create traffic %s: %v", email, err)
	}
}

func trafficOf(t *testing.T, email string) xray.ClientTraffic {
	t.Helper()
	var row xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("load traffic %s: %v", email, err)
	}
	return row
}

func TestBulkResetTrafficZeroesUsageAndReenables(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "alice@x", ID: "11111111-1111-1111-1111-111111111111", SubID: "sa", Enable: true},
		{Email: "bob@x", ID: "22222222-2222-2222-2222-222222222222", SubID: "sb", Enable: true},
		{Email: "carol@x", ID: "33333333-3333-3333-3333-333333333333", SubID: "sc", Enable: true},
	}
	ib := mkInbound(t, 21001, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, "alice@x", 10, 20, 0, 0, false)
	mkTraffic(t, ib.Id, "bob@x", 5, 5, 0, 0, true)
	mkTraffic(t, ib.Id, "carol@x", 7, 0, 0, 0, true)

	affected, err := svc.BulkResetTraffic(inboundSvc, []string{"alice@x", "bob@x"})
	if err != nil {
		t.Fatalf("BulkResetTraffic: %v", err)
	}
	if affected != 2 {
		t.Fatalf("expected 2 affected, got %d", affected)
	}

	for _, e := range []string{"alice@x", "bob@x"} {
		tr := trafficOf(t, e)
		if tr.Up != 0 || tr.Down != 0 {
			t.Fatalf("%s: expected up/down 0, got up=%d down=%d", e, tr.Up, tr.Down)
		}
		if !tr.Enable {
			t.Fatalf("%s: expected re-enabled", e)
		}
	}

	carol := trafficOf(t, "carol@x")
	if carol.Up != 7 {
		t.Fatalf("carol not in list should be untouched, got up=%d", carol.Up)
	}
}

func TestDelDepletedRemovesOnlyDepleted(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "alice@x", ID: "11111111-1111-1111-1111-111111111111", SubID: "sa", Enable: true},
		{Email: "bob@x", ID: "22222222-2222-2222-2222-222222222222", SubID: "sb", Enable: true},
		{Email: "carol@x", ID: "33333333-3333-3333-3333-333333333333", SubID: "sc", Enable: true},
	}
	ib := mkInbound(t, 21002, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	past := time.Now().Add(-time.Hour).UnixMilli()
	mkTraffic(t, ib.Id, "alice@x", 60, 60, 100, 0, true)
	mkTraffic(t, ib.Id, "bob@x", 10, 10, 100, 0, true)
	mkTraffic(t, ib.Id, "carol@x", 0, 0, 0, past, true)

	deleted, _, err := svc.DelDepleted(inboundSvc)
	if err != nil {
		t.Fatalf("DelDepleted: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deleted (alice traffic-depleted, carol expired), got %d", deleted)
	}

	if _, err := svc.GetRecordByEmail(nil, "bob@x"); err != nil {
		t.Fatalf("bob should survive: %v", err)
	}
	for _, e := range []string{"alice@x", "carol@x"} {
		if _, err := svc.GetRecordByEmail(nil, e); err == nil {
			t.Fatalf("%s should be deleted", e)
		}
	}

	reloaded, _ := inboundSvc.GetInbound(ib.Id)
	jsonClients, _ := inboundSvc.GetClients(reloaded)
	if len(jsonClients) != 1 || jsonClients[0].Email != "bob@x" {
		t.Fatalf("settings JSON should contain only bob, got %d clients", len(jsonClients))
	}
}

func TestClientDeleteKeepTrafficPreservesRowForAttachedClient(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "keepme@x"
	c := model.Client{Email: email, ID: "11111111-1111-1111-1111-111111111111", SubID: email, Enable: true}
	ib := mkInbound(t, 52030, model.VLESS, clientsSettings(t, []model.Client{c}))
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{c}); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, email, 111, 222, 0, 0, true)

	rec := lookupClientRecord(t, email)
	if _, err := svc.Delete(inboundSvc, rec.Id, true); err != nil {
		t.Fatalf("Delete(keepTraffic): %v", err)
	}

	var cnt int64
	if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", email).Count(&cnt).Error; err != nil {
		t.Fatalf("count traffic: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("keepTraffic delete of an inbound-attached client must preserve its client_traffics row, found %d", cnt)
	}
}

func TestBulkDeleteRemovesClientExternalLinks(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "extlink@x"
	c := model.Client{Email: email, ID: "11111111-1111-1111-1111-111111111111", SubID: email, Enable: true}
	ib := mkInbound(t, 52040, model.VLESS, clientsSettings(t, []model.Client{c}))
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{c}); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	rec := lookupClientRecord(t, email)
	if err := database.GetDB().Create(&model.ClientExternalLink{ClientId: rec.Id, Kind: "sub", Value: "https://example.com/x"}).Error; err != nil {
		t.Fatalf("seed external link: %v", err)
	}

	if _, _, err := svc.BulkDelete(inboundSvc, []string{email}, false); err != nil {
		t.Fatalf("BulkDelete: %v", err)
	}

	var cnt int64
	if err := database.GetDB().Model(&model.ClientExternalLink{}).Where("client_id = ?", rec.Id).Count(&cnt).Error; err != nil {
		t.Fatalf("count external links: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("BulkDelete left %d orphan external-link row(s) behind", cnt)
	}
}

func TestGetClientTrafficByEmailReadsClientsTable(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "alice@x", ID: "11111111-1111-1111-1111-111111111111", SubID: "sa", Enable: true},
	}
	ib := mkInbound(t, 21003, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, "alice@x", 1, 2, 0, 0, true)

	tr, err := inboundSvc.GetClientTrafficByEmail("alice@x")
	if err != nil {
		t.Fatalf("GetClientTrafficByEmail: %v", err)
	}
	if tr == nil {
		t.Fatalf("expected traffic, got nil")
		return
	}
	if tr.UUID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("UUID not enriched from clients table, got %q", tr.UUID)
	}
	if tr.SubId != "sa" {
		t.Fatalf("SubId not enriched from clients table, got %q", tr.SubId)
	}
}
