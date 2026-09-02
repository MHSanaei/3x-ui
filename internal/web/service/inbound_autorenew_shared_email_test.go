package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestAutoRenewClients_UpdatesEveryInboundForSharedEmail(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	past := time.Now().Add(-48 * time.Hour).UnixMilli()
	shared := model.Client{
		Email: "shared@x", ID: "11111111-1111-1111-1111-111111111111",
		Enable: false, Reset: 30, ExpiryTime: past,
	}
	ib1 := mkInbound(t, 30201, model.VLESS, clientsSettings(t, []model.Client{shared}))
	ib2 := mkInbound(t, 30202, model.VLESS, clientsSettings(t, []model.Client{shared}))
	for _, ib := range []*model.Inbound{ib1, ib2} {
		if err := svc.clientService.SyncInbound(nil, ib.Id, []model.Client{shared}); err != nil {
			t.Fatalf("SyncInbound %d: %v", ib.Id, err)
		}
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib1.Id, Email: shared.Email, Enable: false,
		Up: 100, Down: 200, Reset: 30, ExpiryTime: past,
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	batch := newTrafficMutationBatch()
	if _, count, err := svc.autoRenewClients(db, batch); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	} else if count != 1 {
		t.Fatalf("renewed count = %d, want 1 shared client", count)
	}

	var traffic xray.ClientTraffic
	if err := db.Where("email = ?", shared.Email).First(&traffic).Error; err != nil {
		t.Fatalf("read client_traffics: %v", err)
	}
	if !traffic.Enable || traffic.ExpiryTime <= time.Now().UnixMilli() {
		t.Fatalf("traffic state not renewed: enable=%v expiry=%d", traffic.Enable, traffic.ExpiryTime)
	}
	for _, ib := range []*model.Inbound{ib1, ib2} {
		reloaded, err := svc.GetInbound(ib.Id)
		if err != nil {
			t.Fatalf("GetInbound %d: %v", ib.Id, err)
		}
		clients, err := svc.GetClients(reloaded)
		if err != nil {
			t.Fatalf("GetClients %d: %v", ib.Id, err)
		}
		if len(clients) != 1 {
			t.Fatalf("inbound %d clients = %d, want 1", ib.Id, len(clients))
		}
		if !clients[0].Enable || clients[0].ExpiryTime != traffic.ExpiryTime {
			t.Errorf("inbound %d state = enable %v expiry %d, want true/%d", ib.Id, clients[0].Enable, clients[0].ExpiryTime, traffic.ExpiryTime)
		}
	}

	record, err := svc.clientService.GetRecordByEmail(nil, shared.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if !record.Enable || record.ExpiryTime != traffic.ExpiryTime {
		t.Errorf("clients row = enable %v expiry %d, want true/%d", record.Enable, record.ExpiryTime, traffic.ExpiryTime)
	}
	if len(batch.localPlans) != 2 {
		t.Errorf("runtime add plans = %d, want one for each inbound", len(batch.localPlans))
	}
	planCountByInbound := make(map[int]int, len(batch.localPlans))
	for _, plan := range batch.localPlans {
		planCountByInbound[plan.inbound.Id]++
	}
	for _, ib := range []*model.Inbound{ib1, ib2} {
		if planCountByInbound[ib.Id] != 1 {
			t.Errorf("inbound %d runtime add plans = %d, want 1", ib.Id, planCountByInbound[ib.Id])
		}
	}
}

func TestAutoRenewClients_PreservesOperatorDisabledClient(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	past := time.Now().Add(-48 * time.Hour).UnixMilli()
	disabled := model.Client{
		Email: "disabled@x", ID: "22222222-2222-2222-2222-222222222222",
		Enable: false, Reset: 30, ExpiryTime: past,
	}
	ib := mkInbound(t, 30203, model.VLESS, clientsSettings(t, []model.Client{disabled}))
	if err := svc.clientService.SyncInbound(nil, ib.Id, []model.Client{disabled}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: disabled.Email, Enable: true,
		Up: 100, Down: 200, Reset: 30, ExpiryTime: past,
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	batch := newTrafficMutationBatch()
	if _, count, err := svc.autoRenewClients(db, batch); err != nil {
		t.Fatalf("autoRenewClients: %v", err)
	} else if count != 1 {
		t.Fatalf("renewed count = %d, want 1", count)
	}
	var traffic xray.ClientTraffic
	if err := db.Where("email = ?", disabled.Email).First(&traffic).Error; err != nil {
		t.Fatalf("read client_traffics: %v", err)
	}
	if !traffic.Enable || traffic.ExpiryTime <= time.Now().UnixMilli() {
		t.Fatalf("traffic state not renewed: enable=%v expiry=%d", traffic.Enable, traffic.ExpiryTime)
	}

	reloaded, err := svc.GetInbound(ib.Id)
	if err != nil {
		t.Fatalf("GetInbound: %v", err)
	}
	clients, err := svc.GetClients(reloaded)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("clients = %d, want 1", len(clients))
	}
	if clients[0].Enable {
		t.Error("operator-disabled client was enabled in inbound settings")
	}
	if clients[0].ExpiryTime != traffic.ExpiryTime {
		t.Errorf("settings expiry = %d, want %d", clients[0].ExpiryTime, traffic.ExpiryTime)
	}

	record, err := svc.clientService.GetRecordByEmail(nil, disabled.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if record.Enable {
		t.Error("operator-disabled client was enabled in clients table")
	}
	if record.ExpiryTime != traffic.ExpiryTime {
		t.Errorf("clients row expiry = %d, want %d", record.ExpiryTime, traffic.ExpiryTime)
	}
	if len(batch.localPlans) != 0 {
		t.Errorf("runtime add plans = %d, want 0", len(batch.localPlans))
	}
}
