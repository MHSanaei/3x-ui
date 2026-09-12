package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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
