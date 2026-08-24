package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestUpdateInbound_RejectsHysteriaClientWithoutAuth(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "in-45001-tcp", "0.0.0.0", 45001, model.VLESS,
		`{"network":"tcp"}`, `{"clients":[]}`)

	var existing model.Inbound
	if err := database.GetDB().Where("tag = ?", "in-45001-tcp").First(&existing).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}

	update := existing
	update.Protocol = model.Hysteria
	update.Settings = `{"clients":[{"email":"hysteria@x","enable":true,"password":"not-hysteria-auth"}]}`

	svc := &InboundService{}
	if _, _, err := svc.UpdateInbound(&update); err == nil || !strings.Contains(err.Error(), "empty client ID") {
		t.Fatalf("UpdateInbound error = %v, want empty client ID", err)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, existing.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Protocol != existing.Protocol {
		t.Fatalf("persisted protocol = %q, want unchanged %q", reloaded.Protocol, existing.Protocol)
	}
	if reloaded.Settings != existing.Settings {
		t.Fatalf("rejected settings were persisted\ngot:  %s\nwant: %s", reloaded.Settings, existing.Settings)
	}
}

func TestUpdateInbound_PreservesHysteriaClientAuth(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "in-45002-udp", "0.0.0.0", 45002, model.Hysteria,
		`{"network":"hysteria"}`, `{"clients":[]}`)

	var existing model.Inbound
	if err := database.GetDB().Where("tag = ?", "in-45002-udp").First(&existing).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}

	const wantAuth = "hysteria-auth"
	const password = "not-hysteria-auth"
	update := existing
	update.Settings = `{"clients":[{"email":"hysteria@x","enable":true,"password":"` + password + `","auth":"` + wantAuth + `"}]}`

	svc := &InboundService{}
	if _, _, err := svc.UpdateInbound(&update); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, existing.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	clients, err := ParseInboundSettingsClients(reloaded.Settings)
	if err != nil {
		t.Fatalf("parse persisted clients: %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("persisted clients = %d, want 1", len(clients))
	}
	if clients[0].Auth != wantAuth {
		t.Fatalf("persisted auth = %q, want %q", clients[0].Auth, wantAuth)
	}
	if clients[0].Password != password {
		t.Fatalf("persisted password = %q, want %q", clients[0].Password, password)
	}
}

func TestUpdateInbound_AllowsHysteriaWithoutClients(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "in-45003-udp", "0.0.0.0", 45003, model.Hysteria,
		`{"network":"hysteria"}`, `{"clients":[]}`)

	var existing model.Inbound
	if err := database.GetDB().Where("tag = ?", "in-45003-udp").First(&existing).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}

	update := existing
	update.Remark = "updated without clients"

	svc := &InboundService{}
	if _, _, err := svc.UpdateInbound(&update); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, existing.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Remark != update.Remark {
		t.Fatalf("persisted remark = %q, want %q", reloaded.Remark, update.Remark)
	}
}
