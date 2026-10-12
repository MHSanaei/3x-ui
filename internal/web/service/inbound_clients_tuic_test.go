package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func TestBuildTargetClientFromSourceTuic(t *testing.T) {
	s := &InboundService{}
	source := model.Client{
		Email:    "test@example.com",
		ID:       "old-uuid",
		Password: "old-password",
	}
	targetInbound := &model.Inbound{
		Protocol: model.TUIC,
	}

	target, err := s.buildTargetClientFromSource(source, targetInbound, "test@example.com", "")
	if err != nil {
		t.Fatalf("buildTargetClientFromSource failed: %v", err)
	}

	if target.ID == "" || target.ID == "old-uuid" {
		t.Fatalf("expected new UUID for TUIC client, got %q", target.ID)
	}
	if target.Password == "" || target.Password == "old-password" {
		t.Fatalf("expected new password for TUIC client, got %q", target.Password)
	}
}

func TestAddInboundTuicClientValidation(t *testing.T) {
	setupConflictDB(t)
	s := &InboundService{}
	ib := &model.Inbound{
		Tag:      "tuic-test-1",
		Protocol: model.TUIC,
		Settings: `{"clients":[{"id":"uuid-1","password":""}]}`,
	}
	_, _, err := s.AddInbound(ib)
	if err == nil || strings.TrimSpace(err.Error()) != "tuic client requires a password" {
		t.Fatalf("expected 'tuic client requires a password' error, got %v", err)
	}

	ibNoID := &model.Inbound{
		Tag:      "tuic-test-2",
		Protocol: model.TUIC,
		Settings: `{"clients":[{"id":"","password":"pass"}]}`,
	}
	_, _, errNoID := s.AddInbound(ibNoID)
	if errNoID == nil || strings.TrimSpace(errNoID.Error()) != "empty client ID" {
		t.Fatalf("expected 'empty client ID' error, got %v", errNoID)
	}

	ibNoEmail := &model.Inbound{
		Tag:      "tuic-test-3",
		Protocol: model.TUIC,
		Settings: `{"clients":[{"id":"uuid-3","password":"pass","email":""}]}`,
	}
	_, _, errNoEmail := s.AddInbound(ibNoEmail)
	if errNoEmail == nil || strings.TrimSpace(errNoEmail.Error()) != "empty client email" {
		t.Fatalf("expected 'empty client email' error, got %v", errNoEmail)
	}
}

func TestTuicClientMutationsDoNotRequireRestart(t *testing.T) {
	setupConflictDB(t)
	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}})
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}

	ib := &model.Inbound{
		Tag:      "tuic-restart-test",
		Protocol: model.TUIC,
		Port:     54321,
		Enable:   true,
		Settings: `{
			"certificate": "dummy-cert",
			"private_key": "dummy-key",
			"clients":[{"id":"a0000000-0000-0000-0000-000000000001","password":"p1","email":"user1@tuic.com","enable":true}]
		}`,
	}
	created, _, err := inboundSvc.AddInbound(ib)
	if err != nil {
		t.Fatalf("AddInbound failed: %v", err)
	}

	addPayload := &model.Inbound{
		Id:       created.Id,
		Settings: `{"clients":[{"id":"a0000000-0000-0000-0000-000000000002","password":"p2","email":"user2@tuic.com","enable":true}]}`,
	}
	needRestart, err := clientSvc.AddInboundClient(inboundSvc, addPayload)
	if err != nil {
		t.Fatalf("AddInboundClient failed: %v", err)
	}
	if needRestart {
		t.Fatal("expected needRestart = false when adding TUIC client")
	}

	updatePayload := &model.Inbound{
		Id:       created.Id,
		Settings: `{"clients":[{"id":"a0000000-0000-0000-0000-000000000002","password":"new-p2","email":"user2@tuic.com","enable":true}]}`,
	}
	needRestart, err = clientSvc.UpdateInboundClient(inboundSvc, updatePayload, "user2@tuic.com")
	if err != nil {
		t.Fatalf("UpdateInboundClient failed: %v", err)
	}
	if needRestart {
		t.Fatal("expected needRestart = false when updating TUIC client")
	}

	needRestart, err = clientSvc.DelInboundClientByEmail(inboundSvc, created.Id, "user2@tuic.com", false, true)
	if err != nil {
		t.Fatalf("DelInboundClientByEmail failed: %v", err)
	}
	if needRestart {
		t.Fatal("expected needRestart = false when deleting TUIC client")
	}
}
