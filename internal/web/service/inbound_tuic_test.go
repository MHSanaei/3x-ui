package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestInjectTuicSocks(t *testing.T) {
	cfg := &xray.Config{}
	inbounds := []*model.Inbound{
		{
			Id:       5,
			Tag:      "tuic-in-5",
			Protocol: model.TUIC,
			Enable:   true,
			Settings: `{
				"certificate": "dummy-cert",
				"private_key": "dummy-key",
				"clients": [
					{"uuid": "a0000000-0000-0000-0000-000000000001", "password": "pass1", "email": "user1@example.com", "enable": true}
				]
			}`,
		},
	}

	injectTuicSocks(cfg, inbounds)

	if len(cfg.InboundConfigs) != 1 {
		t.Fatalf("expected 1 injected SOCKS inbound, got %d", len(cfg.InboundConfigs))
	}

	sc := cfg.InboundConfigs[0]
	if sc.Tag != "tuic-in-5" {
		t.Fatalf("expected tag tuic-in-5, got %s", sc.Tag)
	}
	if sc.Protocol != "socks" {
		t.Fatalf("expected protocol socks, got %s", sc.Protocol)
	}
	expectedPort := tuic.SOCKSPortForInbound(5)
	if sc.Port != expectedPort {
		t.Fatalf("expected port %d, got %d", expectedPort, sc.Port)
	}
	if string(sc.Listen) != `"127.0.0.1"` {
		t.Fatalf("expected listen 127.0.0.1, got %s", sc.Listen)
	}
	if string(sc.Sniffing) != tuicEgressSniffingSettings {
		t.Fatalf("expected sniffing settings %s, got %s", tuicEgressSniffingSettings, sc.Sniffing)
	}

	var parsedSettings struct {
		Auth     string `json:"auth"`
		UDP      bool   `json:"udp"`
		Accounts []struct {
			User string `json:"user"`
			Pass string `json:"pass"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(sc.Settings, &parsedSettings); err != nil {
		t.Fatalf("failed to unmarshal settings: %v", err)
	}
	if parsedSettings.Auth != "password" || !parsedSettings.UDP {
		t.Fatalf("expected auth=password, udp=true, got %+v", parsedSettings)
	}
	if len(parsedSettings.Accounts) != 1 || parsedSettings.Accounts[0].User != "user1@example.com" {
		t.Fatalf("unexpected accounts: %+v", parsedSettings.Accounts)
	}
}

func TestCheckTuicSocksConflict(t *testing.T) {
	setupConflictDB(t)

	// Seed TUIC inbound with ID 10
	tuicIb := &model.Inbound{
		Id:       10,
		Tag:      "tuic-10",
		Protocol: model.TUIC,
		Enable:   true,
		Listen:   "0.0.0.0",
		Port:     8443,
		Settings: `{"clients":[{"uuid":"a0000000-0000-0000-0000-000000000001","password":"p","email":"u@test.com"}]}`,
	}
	if err := database.GetDB().Create(tuicIb).Error; err != nil {
		t.Fatalf("failed to seed TUIC inbound: %v", err)
	}

	relayPort := tuic.SOCKSPortForInbound(10)

	// Try to create a new TCP inbound on that relayPort on 127.0.0.1
	newIb := &model.Inbound{
		Tag:      "colliding-inbound",
		Protocol: model.Mixed,
		Enable:   true,
		Listen:   "127.0.0.1",
		Port:     relayPort,
	}

	detail, err := checkTuicSocksConflict(database.GetDB(), newIb, 0, transportTCP)
	if err != nil {
		t.Fatalf("checkTuicSocksConflict error: %v", err)
	}
	if detail == nil {
		t.Fatalf("expected conflict on port %d, got none", relayPort)
	}
	if detail.Tag != "tuic-10" {
		t.Fatalf("expected conflict tag tuic-10, got %s", detail.Tag)
	}
}

func TestCheckTuicSocksReverseConflict(t *testing.T) {
	setupConflictDB(t)

	targetPort := tuic.SOCKSPortForInbound(20)

	// Seed existing inbound on targetPort on 127.0.0.1
	existing := &model.Inbound{
		Id:       99,
		Tag:      "existing-on-relay-port",
		Protocol: model.Mixed,
		Enable:   true,
		Listen:   "127.0.0.1",
		Port:     targetPort,
	}
	if err := database.GetDB().Create(existing).Error; err != nil {
		t.Fatalf("failed to seed existing inbound: %v", err)
	}

	detail, err := checkTuicSocksReverseConflict(database.GetDB(), 20)
	if err != nil {
		t.Fatalf("checkTuicSocksReverseConflict error: %v", err)
	}
	if detail == nil {
		t.Fatalf("expected reverse conflict for id 20 on port %d, got none", targetPort)
	}
	if detail.Tag != "existing-on-relay-port" {
		t.Fatalf("expected tag existing-on-relay-port, got %s", detail.Tag)
	}
}

func TestDesiredTuicInstances(t *testing.T) {
	setupConflictDB(t)

	ib := &model.Inbound{
		Id:       30,
		Tag:      "tuic-desired-test",
		Protocol: model.TUIC,
		Enable:   true,
		Listen:   "0.0.0.0",
		Port:     9443,
		Settings: `{
			"certificate": "cert",
			"private_key": "key",
			"clients": [
				{"uuid": "a0000000-0000-0000-0000-000000000001", "password": "p1", "email": "active@test.com", "enable": true},
				{"uuid": "a0000000-0000-0000-0000-000000000002", "password": "p2", "email": "disabled@test.com", "enable": true}
			]
		}`,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("failed to seed inbound: %v", err)
	}

	// Add client traffic entry disabling disabled@test.com
	ct := &xray.ClientTraffic{
		InboundId: 30,
		Email:     "disabled@test.com",
		Enable:    false,
	}
	if err := database.GetDB().Create(ct).Error; err != nil {
		t.Fatalf("failed to seed client traffic: %v", err)
	}

	svc := &InboundService{}
	instances, err := svc.DesiredTuicInstances()
	if err != nil {
		t.Fatalf("DesiredTuicInstances failed: %v", err)
	}

	found := false
	for _, inst := range instances {
		if inst.Id == 30 {
			found = true
			if len(inst.Clients) != 1 || inst.Clients[0].Email != "active@test.com" {
				t.Fatalf("expected only active@test.com, got %+v", inst.Clients)
			}
		}
	}
	if !found {
		t.Fatal("expected to find instance for inbound 30")
	}
}

func TestCheckForwardedPortsConflict_CollidesWithTuicSocksPort(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "tuic-1", "0.0.0.0", 8443, model.TUIC, ``, `{"clients":[{"uuid":"u","password":"p","email":"e"}]}`)

	var tuicInbound model.Inbound
	if err := database.GetDB().Where("tag = ?", "tuic-1").First(&tuicInbound).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}
	relayPort := tuic.SOCKSPortForInbound(tuicInbound.Id)

	svc := &InboundService{}
	ctx, err := svc.loadPortConflictContext(database.GetDB())
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	hit := svc.checkForwardedPortsConflict(ctx, fmt.Sprintf("%d", relayPort))
	if !strings.Contains(hit, "SOCKS5") {
		t.Fatalf("expected a collision naming the TUIC inbound's SOCKS5 relay port, got %q", hit)
	}
}
