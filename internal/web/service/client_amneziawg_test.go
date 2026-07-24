package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestDefaultAmneziaWGClientsGeneratesKeypair(t *testing.T) {
	settings := amneziawg.SettingsInbound{
		Server: &amneziawg.ServerConfig{
			SubnetIP:   "10.8.1.0",
			SubnetCIDR: 24,
		},
	}
	bs, _ := json.Marshal(settings)
	clients := []model.Client{{Email: "a@awg"}}
	ifaces := []any{map[string]any{"email": "a@awg"}}

	if err := defaultAmneziaWGClients(string(bs), clients, ifaces); err != nil {
		t.Fatalf("defaultAmneziaWGClients: %v", err)
	}

	c := clients[0]
	if c.PrivateKey == "" {
		t.Fatal("private key not generated")
	}
	if c.PublicKey == "" {
		t.Fatal("public key not generated")
	}
	if c.PreSharedKey == "" {
		t.Fatal("preshared key not generated")
	}
	if len(c.AllowedIPs) != 1 || c.AllowedIPs[0] != "10.8.1.2/32" {
		t.Fatalf("allowedIPs not allocated: %v", c.AllowedIPs)
	}

	m := ifaces[0].(map[string]any)
	if m["privateKey"] != c.PrivateKey {
		t.Fatal("interface map privateKey mismatch")
	}
	if m["publicKey"] != c.PublicKey {
		t.Fatal("interface map publicKey mismatch")
	}
	if m["assignedIp"] != "10.8.1.2" {
		t.Fatalf("interface map assignedIp = %q, want %q", m["assignedIp"], "10.8.1.2")
	}
}

func TestDefaultAmneziaWGClientsSkipsExistingKeys(t *testing.T) {
	priv, pub, psk, err := amneziawg.GenerateWireGuardKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	settings := amneziawg.SettingsInbound{
		Server: &amneziawg.ServerConfig{
			SubnetIP:   "10.8.1.0",
			SubnetCIDR: 24,
		},
	}
	bs, _ := json.Marshal(settings)
	clients := []model.Client{{
		Email:        "b@awg",
		PrivateKey:   priv,
		PublicKey:    pub,
		PreSharedKey: psk,
	}}
	ifaces := []any{map[string]any{"email": "b@awg"}}

	if err := defaultAmneziaWGClients(string(bs), clients, ifaces); err != nil {
		t.Fatalf("defaultAmneziaWGClients: %v", err)
	}

	if clients[0].PrivateKey != priv {
		t.Fatal("existing private key was overwritten")
	}
	if clients[0].PublicKey != pub {
		t.Fatal("existing public key was overwritten")
	}
}

func TestDefaultAmneziaWGClientsAllocatesSequentialIPs(t *testing.T) {
	settings := amneziawg.SettingsInbound{
		Server: &amneziawg.ServerConfig{
			SubnetIP:   "10.8.1.0",
			SubnetCIDR: 24,
		},
	}
	bs, _ := json.Marshal(settings)
	clients := []model.Client{
		{Email: "c1@awg"},
		{Email: "c2@awg"},
	}
	ifaces := []any{
		map[string]any{"email": "c1@awg"},
		map[string]any{"email": "c2@awg"},
	}

	if err := defaultAmneziaWGClients(string(bs), clients, ifaces); err != nil {
		t.Fatalf("defaultAmneziaWGClients: %v", err)
	}

	if clients[0].AllowedIPs[0] != "10.8.1.2/32" {
		t.Fatalf("client 0 IP = %q, want %q", clients[0].AllowedIPs[0], "10.8.1.2/32")
	}
	if clients[1].AllowedIPs[0] != "10.8.1.3/32" {
		t.Fatalf("client 1 IP = %q, want %q", clients[1].AllowedIPs[0], "10.8.1.3/32")
	}
}

func TestDefaultAmneziaWGClientsMissingServer(t *testing.T) {
	err := defaultAmneziaWGClients(`{"clients":[]}`, nil, nil)
	if err == nil {
		t.Fatal("expected error for missing server config")
	}
}

func TestDefaultAmneziaWGClientsInvalidSettings(t *testing.T) {
	err := defaultAmneziaWGClients(`not-json`, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid settings JSON")
	}
}

func TestDefaultAmneziaWGClientsUpdatesInterfaceMap(t *testing.T) {
	settings := amneziawg.SettingsInbound{
		Server: &amneziawg.ServerConfig{
			SubnetIP:   "10.8.1.0",
			SubnetCIDR: 24,
		},
	}
	bs, _ := json.Marshal(settings)
	clients := []model.Client{{Email: "d@awg"}}
	ifaces := []any{map[string]any{"email": "d@awg"}}

	if err := defaultAmneziaWGClients(string(bs), clients, ifaces); err != nil {
		t.Fatalf("defaultAmneziaWGClients: %v", err)
	}

	m := ifaces[0].(map[string]any)
	if _, ok := m["privateKey"]; !ok {
		t.Fatal("interface map missing privateKey")
	}
	if _, ok := m["publicKey"]; !ok {
		t.Fatal("interface map missing publicKey")
	}
	if _, ok := m["presharedKey"]; !ok {
		t.Fatal("interface map missing presharedKey")
	}
	if _, ok := m["assignedIp"]; !ok {
		t.Fatal("interface map missing assignedIp")
	}
	if _, ok := m["allowedIPs"]; !ok {
		t.Fatal("interface map missing allowedIPs")
	}
}