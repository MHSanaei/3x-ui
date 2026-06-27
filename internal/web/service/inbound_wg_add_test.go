package service

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestAddInboundSyncsWireGuardPeersToClients(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	inbound := &model.Inbound{
		Remark:   "wg-import",
		Enable:   false,
		Port:     32123,
		Protocol: model.WireGuard,
		Settings: `{
  "secretKey": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
  "peers": [
    {
      "privateKey": "peer-private",
      "publicKey": "peer-public",
      "allowedIPs": ["10.0.0.2/32"]
    }
  ]
}`,
	}

	created, _, err := (&InboundService{}).AddInbound(inbound)
	if err != nil {
		t.Fatalf("AddInbound: %v", err)
	}

	var rec model.ClientRecord
	if err := database.GetDB().Where("email = ?", "wg-1-peer-1").First(&rec).Error; err != nil {
		t.Fatalf("load synced WG client: %v", err)
	}
	if rec.Password != "peer-private" {
		t.Fatalf("password = %q, want peer-private", rec.Password)
	}
	if rec.WgSettings == "" {
		t.Fatalf("wg_settings was not populated")
	}

	var link model.ClientInbound
	if err := database.GetDB().
		Where("client_id = ? AND inbound_id = ?", rec.Id, created.Id).
		First(&link).Error; err != nil {
		t.Fatalf("load client_inbounds link: %v", err)
	}
}

func TestWireGuardClientCannotAttachToAnotherWireGuardInbound(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	svc := &InboundService{}
	first, _, err := svc.AddInbound(&model.Inbound{
		Remark:   "wg-one",
		Enable:   false,
		Port:     32124,
		Protocol: model.WireGuard,
		Settings: `{"secretKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","peers":[{"privateKey":"peer-private","publicKey":"peer-public","allowedIPs":["10.0.0.2/32"]}]}`,
	})
	if err != nil {
		t.Fatalf("AddInbound first: %v", err)
	}
	second, _, err := svc.AddInbound(&model.Inbound{
		Remark:   "wg-two",
		Enable:   false,
		Port:     32125,
		Protocol: model.WireGuard,
		Settings: `{"secretKey":"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=","peers":[]}`,
	})
	if err != nil {
		t.Fatalf("AddInbound second: %v", err)
	}

	var rec model.ClientRecord
	if err := database.GetDB().Where("email = ?", "wg-1-peer-1").First(&rec).Error; err != nil {
		t.Fatalf("load synced WG client: %v", err)
	}
	if rec.Id == 0 || first.Id == 0 || second.Id == 0 {
		t.Fatalf("expected persisted ids")
	}

	_, err = (&ClientService{}).Attach(svc, rec.Id, []int{second.Id})
	if err == nil || !strings.Contains(err.Error(), "only one WireGuard inbound") {
		t.Fatalf("Attach error = %v, want only-one-WG guard", err)
	}
}

func TestWireGuardClientCannotGenericDetachFromWireGuardInbound(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	inboundSvc := &InboundService{}
	created, _, err := inboundSvc.AddInbound(&model.Inbound{
		Remark:   "wg-one",
		Enable:   false,
		Port:     32126,
		Protocol: model.WireGuard,
		Settings: `{"secretKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","peers":[{"privateKey":"peer-private","publicKey":"peer-public","allowedIPs":["10.0.0.2/32"]}]}`,
	})
	if err != nil {
		t.Fatalf("AddInbound: %v", err)
	}
	var rec model.ClientRecord
	if err := database.GetDB().Where("email = ?", "wg-1-peer-1").First(&rec).Error; err != nil {
		t.Fatalf("load synced WG client: %v", err)
	}

	_, err = (&ClientService{}).Detach(inboundSvc, rec.Id, []int{created.Id})
	if err == nil || !strings.Contains(err.Error(), "cannot be detached") {
		t.Fatalf("Detach error = %v, want WG detach guard", err)
	}
}

func TestOrphanWireGuardClientCannotAttachToWireGuardInbound(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	inboundSvc := &InboundService{}
	created, _, err := inboundSvc.AddInbound(&model.Inbound{
		Remark:   "wg-new",
		Enable:   false,
		Port:     32127,
		Protocol: model.WireGuard,
		Settings: `{"secretKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","peers":[]}`,
	})
	if err != nil {
		t.Fatalf("AddInbound: %v", err)
	}
	rec := &model.ClientRecord{
		Email:      "orphan-wg",
		Password:   "peer-private",
		Enable:     true,
		WgSettings: `{"publicKey":"peer-public","allowedIPs":["10.0.0.2/32"]}`,
	}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("create orphan WG client: %v", err)
	}

	_, err = (&ClientService{}).Attach(inboundSvc, rec.Id, []int{created.Id})
	if err == nil || !strings.Contains(err.Error(), "cannot be reassigned") {
		t.Fatalf("Attach error = %v, want orphan-WG guard", err)
	}
}
