package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestUpdate_PersistsRecordEnable_True(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "u-true@x"
	c := model.Client{Email: email, ID: "11111111-1111-1111-1111-111111111111", SubID: email, Enable: false}
	ib := mkInbound(t, 53001, model.VLESS, clientsSettings(t, []model.Client{c}))
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{c}); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, email, 0, 0, 0, 0, false)

	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	updated := rec.ToClient()
	updated.Enable = true
	if _, err := svc.Update(inboundSvc, rec.Id, *updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got := recordEnableOf(t, svc, email); !got {
		t.Fatalf("%s: client_records.enable = false, want true", email)
	}
	if got := trafficOf(t, email).Enable; !got {
		t.Fatalf("%s: client_traffics.enable = false, want true", email)
	}
	if got := jsonClientEnable(t, inboundSvc, ib.Id, email); !got {
		t.Fatalf("%s: inbound JSON enable = false, want true", email)
	}
}

func TestUpdate_PersistsRecordEnable_False(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "u-false@x"
	c := model.Client{Email: email, ID: "11111111-1111-1111-1111-111111111111", SubID: email, Enable: true}
	ib := mkInbound(t, 53002, model.VLESS, clientsSettings(t, []model.Client{c}))
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{c}); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, email, 0, 0, 0, 0, true)

	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	updated := rec.ToClient()
	updated.Enable = false
	if _, err := svc.Update(inboundSvc, rec.Id, *updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got := recordEnableOf(t, svc, email); got {
		t.Fatalf("%s: client_records.enable = true, want false", email)
	}
}

func TestUpdate_PersistsRecordEnable_NoInbound(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "u-noib@x"
	rec := &model.ClientRecord{
		Email:  email,
		UUID:   "11111111-1111-1111-1111-111111111111",
		SubID:  email,
		Enable: false,
	}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}
	forceRecordDisabled(t, svc, email)

	updated := rec.ToClient()
	updated.Enable = true
	if _, err := svc.Update(inboundSvc, rec.Id, *updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got := recordEnableOf(t, svc, email); !got {
		t.Fatalf("%s: client_records.enable = false, want true (no-inbound persistence gap)", email)
	}
}

func TestResetTrafficByEmail_LeavesRecordEnableTrue(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "r-attached@x"
	c := model.Client{Email: email, ID: "11111111-1111-1111-1111-111111111111", SubID: email, Enable: false}
	ib := mkInbound(t, 53003, model.VLESS, clientsSettings(t, []model.Client{c}))
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{c}); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	mkTraffic(t, ib.Id, email, 10, 20, 0, 0, false)

	if _, err := svc.ResetTrafficByEmail(inboundSvc, email); err != nil {
		t.Fatalf("ResetTrafficByEmail: %v", err)
	}

	if got := recordEnableOf(t, svc, email); !got {
		t.Fatalf("%s: client_records.enable = false, want true", email)
	}
	tr := trafficOf(t, email)
	if !tr.Enable {
		t.Fatalf("%s: client_traffics.enable = false, want true", email)
	}
	if tr.Up != 0 || tr.Down != 0 {
		t.Fatalf("%s: expected up/down 0, got up=%d down=%d", email, tr.Up, tr.Down)
	}
}

func TestResetTrafficByEmail_NoInbound_LeavesRecordEnableTrue(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "r-noib@x"
	rec := &model.ClientRecord{
		Email:  email,
		UUID:   "11111111-1111-1111-1111-111111111111",
		SubID:  email,
		Enable: false,
	}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}
	forceRecordDisabled(t, svc, email)

	if _, err := svc.ResetTrafficByEmail(inboundSvc, email); err != nil {
		t.Fatalf("ResetTrafficByEmail: %v", err)
	}

	if got := recordEnableOf(t, svc, email); !got {
		t.Fatalf("%s: client_records.enable = false, want true (no-inbound reset re-enable gap)", email)
	}
}
