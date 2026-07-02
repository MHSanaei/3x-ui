package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestGetInboundOptions_NodeAddress verifies that a node-managed inbound carries
// its hosting node's externally reachable address, while this panel's own
// inbounds report an empty NodeAddress. The clients page uses it as the
// WireGuard endpoint host so a copied config points at the node, not the master.
func TestGetInboundOptions_NodeAddress(t *testing.T) {
	setupConflictDB(t)

	node := &model.Node{Name: "de-fra-1", Address: "node.example.net", Port: 2053, Enable: true}
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	nodeInbound := &model.Inbound{
		UserId:   1,
		Tag:      "in-51820-udp",
		Enable:   true,
		Listen:   "0.0.0.0",
		Port:     51820,
		Protocol: model.WireGuard,
		Settings: `{"clients":[],"secretKey":"QGVlb2dXc1ZTWGw0ZXBzZndsWmtMaUM5MUlNYjBHWFdYbz0="}`,
		NodeID:   &node.Id,
	}
	localInbound := &model.Inbound{
		UserId:            1,
		Tag:               "in-443-tcp",
		Enable:            true,
		Listen:            "0.0.0.0",
		Port:              443,
		Protocol:          model.VLESS,
		StreamSettings:    `{"network":"tcp"}`,
		Settings:          `{"clients":[]}`,
		ShareAddrStrategy: "custom",
		ShareAddr:         "vpn.example.com",
	}
	if err := database.GetDB().Create(nodeInbound).Error; err != nil {
		t.Fatalf("create node inbound: %v", err)
	}
	if err := database.GetDB().Create(localInbound).Error; err != nil {
		t.Fatalf("create local inbound: %v", err)
	}

	svc := &InboundService{}
	options, err := svc.GetInboundOptions(1)
	if err != nil {
		t.Fatalf("GetInboundOptions: %v", err)
	}

	byID := make(map[int]InboundOption, len(options))
	for _, o := range options {
		byID[o.Id] = o
	}

	got, ok := byID[nodeInbound.Id]
	if !ok {
		t.Fatalf("node inbound %d missing from options", nodeInbound.Id)
	}
	if got.NodeAddress != "node.example.net" {
		t.Fatalf("node inbound NodeAddress = %q, want node.example.net", got.NodeAddress)
	}
	if got.Listen != "0.0.0.0" {
		t.Fatalf("node inbound Listen = %q, want 0.0.0.0", got.Listen)
	}
	if got.ShareAddrStrategy != "" {
		t.Fatalf("node inbound ShareAddrStrategy = %q, want empty (the default node strategy is elided so omitempty drops it)", got.ShareAddrStrategy)
	}

	local, ok := byID[localInbound.Id]
	if !ok {
		t.Fatalf("local inbound %d missing from options", localInbound.Id)
	}
	if local.NodeAddress != "" {
		t.Fatalf("local inbound NodeAddress = %q, want empty", local.NodeAddress)
	}
	if local.ShareAddrStrategy != "custom" {
		t.Fatalf("local inbound ShareAddrStrategy = %q, want custom", local.ShareAddrStrategy)
	}
	if local.ShareAddr != "vpn.example.com" {
		t.Fatalf("local inbound ShareAddr = %q, want vpn.example.com", local.ShareAddr)
	}
}
