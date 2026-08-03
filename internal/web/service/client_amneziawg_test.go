package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// AmneziaWG's own kernel interface Address is exactly the configured
// subnet (unlike WireGuard's Xray-native inbound), so allocation for it must
// never widen past that subnet -- an address from outside it would be
// silently unroutable. See PR #6105 Finding 12.
func TestAllocateWireguardAddress_AmneziaWGNeverWidens(t *testing.T) {
	used := make([]string, 0, 254)
	for i := 2; i <= 255; i++ {
		used = append(used, fmt.Sprintf("10.8.1.%d/32", i))
	}
	if _, err := allocateWireguardAddress(used, "10.8.1.0/24", false); err == nil {
		t.Fatal("a full AmneziaWG /24 must fail loudly instead of allocating an address outside the interface's own subnet")
	}
}

func TestAllocateWireguardAddress_AmneziaWGFillsItsOwnSubnetNormally(t *testing.T) {
	got, err := allocateWireguardAddress([]string{"10.8.1.2/32"}, "10.8.1.0/24", false)
	if err != nil {
		t.Fatalf("allocateWireguardAddress: %v", err)
	}
	if got != "10.8.1.3/32" {
		t.Fatalf("address = %q, want 10.8.1.3/32", got)
	}
}

const amneziawgClientTestSettings = `{"server":{"subnetIp":"10.8.1.0","subnetCidr":24}}`

func TestDefaultAmneziaWGSubnetBases(t *testing.T) {
	v4, v6, err := defaultAmneziaWGSubnetBases(amneziawgClientTestSettings)
	if err != nil {
		t.Fatalf("defaultAmneziaWGSubnetBases: %v", err)
	}
	if v4 != "10.8.1.0/24" {
		t.Fatalf("v4Base = %q, want 10.8.1.0/24", v4)
	}
	if v6 != "" {
		t.Fatalf("v6Base = %q, want empty when IPv6 is not enabled", v6)
	}
}

func TestDefaultAmneziaWGSubnetBasesIncludesIPv6WhenEnabled(t *testing.T) {
	settings := `{"server":{"subnetIp":"10.8.1.0","subnetCidr":24,"ipv6Enabled":true,"ipv6Subnet":"fd00::/64"}}`
	v4, v6, err := defaultAmneziaWGSubnetBases(settings)
	if err != nil {
		t.Fatalf("defaultAmneziaWGSubnetBases: %v", err)
	}
	if v4 != "10.8.1.0/24" || v6 != "fd00::/64" {
		t.Fatalf("got v4=%q v6=%q", v4, v6)
	}
}

func TestDefaultAmneziaWGSubnetBasesRejectsMissingServer(t *testing.T) {
	if _, _, err := defaultAmneziaWGSubnetBases(`{}`); err == nil {
		t.Fatal("expected an error when the settings have no server block")
	}
}

func TestDefaultAmneziaWGClientsGeneratesKeypairAndAllocatesFromOwnSubnet(t *testing.T) {
	clients := []model.Client{{Email: "a@awg"}}
	ifaces := []any{map[string]any{"email": "a@awg"}}
	if err := defaultAmneziaWGClients(amneziawgClientTestSettings, nil, clients, ifaces, nil); err != nil {
		t.Fatalf("defaultAmneziaWGClients: %v", err)
	}
	c := clients[0]
	if c.PrivateKey == "" || c.PublicKey == "" {
		t.Fatalf("keypair not generated: priv=%q pub=%q", c.PrivateKey, c.PublicKey)
	}
	if len(c.AllowedIPs) != 1 || c.AllowedIPs[0] != "10.8.1.2/32" {
		t.Fatalf("allowedIPs not allocated from the inbound's own subnet: %v", c.AllowedIPs)
	}
}

func TestDefaultAmneziaWGClientsPreservesProvided(t *testing.T) {
	clients := []model.Client{{
		Email:      "b@awg",
		PrivateKey: "keep-priv",
		PublicKey:  "keep-pub",
		AllowedIPs: []string{"10.8.1.50/32"},
	}}
	ifaces := []any{map[string]any{"email": "b@awg"}}
	if err := defaultAmneziaWGClients(amneziawgClientTestSettings, nil, clients, ifaces, nil); err != nil {
		t.Fatalf("defaultAmneziaWGClients: %v", err)
	}
	if clients[0].PrivateKey != "keep-priv" || clients[0].PublicKey != "keep-pub" {
		t.Fatalf("provided keys were rotated: %+v", clients[0])
	}
	if clients[0].AllowedIPs[0] != "10.8.1.50/32" {
		t.Fatalf("provided allowedIPs changed: %v", clients[0].AllowedIPs)
	}
}

func TestDefaultAmneziaWGClientsRejectsSameInboundDuplicate(t *testing.T) {
	existing := []model.Client{{Email: "old@awg", AllowedIPs: []string{"10.8.1.9/32"}}}
	dup := []model.Client{{Email: "new@awg", AllowedIPs: []string{"10.8.1.9/32"}}}
	err := defaultAmneziaWGClients(amneziawgClientTestSettings, existing, dup, []any{map[string]any{"email": "new@awg"}}, nil)
	if err == nil {
		t.Fatal("duplicate allowedIPs on the same inbound must be rejected")
	}
}

// The exact real-world scenario that motivated crossInboundUsed: a WireGuard
// client and an AmneziaWG peer given the same address by habit. The
// collision must be caught even though the two live on different inbounds
// and neither appears in the other's own "existing" client list, and the
// error should name the other inbound so an admin isn't left guessing.
func TestDefaultAmneziaWGClientsRejectsCrossInboundDuplicate(t *testing.T) {
	crossUsed := map[string]string{"10.8.1.21/32": "inbound 'wg' (#12)"}
	dup := []model.Client{{Email: "c@awg", AllowedIPs: []string{"10.8.1.21/32"}}}
	err := defaultAmneziaWGClients(amneziawgClientTestSettings, nil, dup, []any{map[string]any{"email": "c@awg"}}, crossUsed)
	if err == nil {
		t.Fatal("allowedIPs already used on another inbound must be rejected")
	}
	if !strings.Contains(err.Error(), "inbound 'wg' (#12)") {
		t.Fatalf("error should name the other inbound holding the address, got: %v", err)
	}
}

func TestDefaultAmneziaWGClientsAutoAllocateSkipsCrossInboundUsed(t *testing.T) {
	crossUsed := map[string]string{"10.8.1.2/32": "inbound 'other-awg' (#3)"}
	clients := []model.Client{{Email: "d@awg"}}
	ifaces := []any{map[string]any{"email": "d@awg"}}
	if err := defaultAmneziaWGClients(amneziawgClientTestSettings, nil, clients, ifaces, crossUsed); err != nil {
		t.Fatalf("defaultAmneziaWGClients: %v", err)
	}
	if clients[0].AllowedIPs[0] != "10.8.1.3/32" {
		t.Fatalf("auto-allocation should skip the cross-inbound-used .2 and pick .3, got %v", clients[0].AllowedIPs)
	}
}

// Unlike WireGuard's allocation base (inferred from existing peers with a
// fallback), AmneziaWG's base always comes from the inbound's own configured
// subnet -- so this is really confirming crossInboundUsed can never change
// which subnet is used, only which addresses within it are free.
func TestDefaultAmneziaWGClientsCrossInboundUsedDoesNotChangeBase(t *testing.T) {
	crossUsed := map[string]string{"192.168.99.5/32": "inbound 'unrelated' (#99)"}
	clients := []model.Client{{Email: "e@awg"}}
	ifaces := []any{map[string]any{"email": "e@awg"}}
	if err := defaultAmneziaWGClients(amneziawgClientTestSettings, nil, clients, ifaces, crossUsed); err != nil {
		t.Fatalf("defaultAmneziaWGClients: %v", err)
	}
	if got := clients[0].AllowedIPs[0]; got != "10.8.1.2/32" {
		t.Fatalf("base subnet must stay the inbound's own 10.8.1.0/24; got %v", got)
	}
}
