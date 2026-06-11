package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// changing an inbound's port must re-derive an auto-generated tag, both in
// the persisted row and in the value returned to the caller (the API
// response the UI renders). The UI round-trips the old tag in a hidden
// field, so the update arrives carrying the stale tag.
func TestUpdateInbound_RegeneratesAutoTagOnPortChange(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "in-22435-tcp", "0.0.0.0", 22435, model.VLESS, `{"network":"tcp"}`, `{"clients":[]}`)

	var existing model.Inbound
	if err := database.GetDB().Where("tag = ?", "in-22435-tcp").First(&existing).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}

	svc := &InboundService{}
	update := existing
	update.Port = 33000
	update.Tag = "in-22435-tcp"
	got, _, err := svc.UpdateInbound(&update)
	if err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, existing.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Tag != "in-33000-tcp" {
		t.Fatalf("persisted tag = %q, want in-33000-tcp", reloaded.Tag)
	}
	if got.Tag != "in-33000-tcp" {
		t.Fatalf("returned tag = %q, want in-33000-tcp", got.Tag)
	}
}

// a node-scoped inbound (tag carries the "n1-" prefix) must keep that prefix
// when its port changes, even if the caller omits nodeId in the update body —
// the node can't be migrated, so the stored NodeID drives the tag. The runtime
// manager isn't wired in unit tests, so UpdateInbound returns a runtime error
// for node inbounds before persisting; we assert on the tag it computed (set on
// the returned object) which is what the save would use.
func TestUpdateInbound_NodeTagKeepsPrefixWhenNodeIdOmitted(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflictNode(t, "n1-in-443-tcp", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{"clients":[]}`, intPtr(1))

	var existing model.Inbound
	if err := database.GetDB().Where("tag = ?", "n1-in-443-tcp").First(&existing).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}

	svc := &InboundService{}
	update := existing
	update.Port = 8443
	update.Tag = "n1-in-443-tcp"
	update.NodeID = nil
	got, _, _ := svc.UpdateInbound(&update)
	if got.Tag != "n1-in-8443-tcp" {
		t.Fatalf("node prefix must survive a port change, got %q", got.Tag)
	}
}

// a tag the user set by hand (doesn't match the canonical shape) survives a
// port change untouched.
func TestUpdateInbound_KeepsCustomTagOnPortChange(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "my-custom-tag", "0.0.0.0", 22435, model.VLESS, `{"network":"tcp"}`, `{"clients":[]}`)

	var existing model.Inbound
	if err := database.GetDB().Where("tag = ?", "my-custom-tag").First(&existing).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}

	svc := &InboundService{}
	update := existing
	update.Port = 33000
	update.Tag = "my-custom-tag"
	got, _, err := svc.UpdateInbound(&update)
	if err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, existing.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Tag != "my-custom-tag" {
		t.Fatalf("persisted tag = %q, want my-custom-tag", reloaded.Tag)
	}
	if got.Tag != "my-custom-tag" {
		t.Fatalf("returned tag = %q, want my-custom-tag", got.Tag)
	}
}

func TestUpdateInbound_PreservesShareAddressFieldsWhenOmitted(t *testing.T) {
	setupConflictDB(t)

	existing := model.Inbound{
		Tag:               "in-443-tcp",
		Enable:            true,
		Listen:            "0.0.0.0",
		Port:              443,
		Protocol:          model.VLESS,
		StreamSettings:    `{"network":"tcp"}`,
		Settings:          `{"clients":[]}`,
		ShareAddrStrategy: "custom",
		ShareAddr:         "  edge.example.com  ",
	}
	if err := database.GetDB().Create(&existing).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	update := existing
	update.Remark = "updated"
	update.ShareAddrStrategy = ""
	update.ShareAddr = ""

	svc := &InboundService{}
	got, _, err := svc.UpdateInbound(&update)
	if err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, existing.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.ShareAddrStrategy != "custom" || reloaded.ShareAddr != "edge.example.com" {
		t.Fatalf("persisted share fields = (%q, %q), want (custom, edge.example.com)", reloaded.ShareAddrStrategy, reloaded.ShareAddr)
	}
	if got.ShareAddrStrategy != "custom" || got.ShareAddr != "edge.example.com" {
		t.Fatalf("returned share fields = (%q, %q), want (custom, edge.example.com)", got.ShareAddrStrategy, got.ShareAddr)
	}
}

func TestNormalizeInboundShareAddressStrict_RequiresHostOnly(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		want    string
		wantErr bool
	}{
		{name: "hostname", addr: " edge.example.com ", want: "edge.example.com"},
		{name: "ipv4", addr: "203.0.113.10", want: "203.0.113.10"},
		{name: "bare ipv6", addr: "2001:db8::1", want: "[2001:db8::1]"},
		{name: "bracketed ipv6", addr: "[2001:db8::2]", want: "[2001:db8::2]"},
		{name: "scheme rejected", addr: "https://edge.example.com", wantErr: true},
		{name: "port rejected", addr: "edge.example.com:8443", wantErr: true},
		{name: "bracketed ipv6 port rejected", addr: "[2001:db8::1]:8443", wantErr: true},
		{name: "path rejected", addr: "edge.example.com/path", wantErr: true},
		{name: "space rejected", addr: "bad host", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inbound := &model.Inbound{
				ShareAddrStrategy: "custom",
				ShareAddr:         tt.addr,
			}
			err := normalizeInboundShareAddressStrict(inbound)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeInboundShareAddressStrict(%q) expected error", tt.addr)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeInboundShareAddressStrict(%q): %v", tt.addr, err)
			}
			if inbound.ShareAddr != tt.want {
				t.Fatalf("ShareAddr = %q, want %q", inbound.ShareAddr, tt.want)
			}
		})
	}
}
