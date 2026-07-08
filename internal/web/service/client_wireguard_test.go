package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func TestAllocateWireguardAddress(t *testing.T) {
	tests := []struct {
		name string
		used []string
		base string
		want string
		err  bool
	}{
		{name: "empty starts at .2", used: nil, base: "10.0.0.0/24", want: "10.0.0.2/32"},
		{name: "skips used", used: []string{"10.0.0.2/32"}, base: "10.0.0.0/24", want: "10.0.0.3/32"},
		{name: "fills gap", used: []string{"10.0.0.3/32", "10.0.0.4/32"}, base: "10.0.0.0/24", want: "10.0.0.2/32"},
		{name: "ignores catch-all", used: []string{"0.0.0.0/0", "::/0"}, base: "10.0.0.0/24", want: "10.0.0.2/32"},
		{name: "default base when empty", used: nil, base: "", want: "10.0.0.2/32"},
		{name: "exhausted /30", used: []string{"10.9.0.2/32", "10.9.0.3/32"}, base: "10.9.0.0/30", err: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := allocateWireguardAddress(tt.used, tt.base)
			if tt.err {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultWireguardClientsGeneratesKeypair(t *testing.T) {
	clients := []model.Client{{Email: "a@wg"}}
	ifaces := []any{map[string]any{"email": "a@wg"}}
	if err := defaultWireguardClients(nil, clients, ifaces); err != nil {
		t.Fatalf("defaultWireguardClients: %v", err)
	}
	c := clients[0]
	if c.PrivateKey == "" || c.PublicKey == "" {
		t.Fatalf("keypair not generated: priv=%q pub=%q", c.PrivateKey, c.PublicKey)
	}
	if len(c.AllowedIPs) != 1 || c.AllowedIPs[0] != "10.0.0.2/32" {
		t.Fatalf("allowedIPs not allocated: %v", c.AllowedIPs)
	}
	m := ifaces[0].(map[string]any)
	if m["privateKey"] != c.PrivateKey || m["publicKey"] != c.PublicKey {
		t.Fatalf("interface map not updated: %v", m)
	}
}

func TestDefaultWireguardClientsDerivesPublicKey(t *testing.T) {
	priv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatal(err)
	}
	wantPub, err := wgutil.PublicKeyFromPrivate(priv)
	if err != nil {
		t.Fatal(err)
	}
	clients := []model.Client{{Email: "b@wg", PrivateKey: priv}}
	ifaces := []any{map[string]any{"email": "b@wg"}}
	if err := defaultWireguardClients(nil, clients, ifaces); err != nil {
		t.Fatalf("defaultWireguardClients: %v", err)
	}
	if clients[0].PublicKey != wantPub {
		t.Fatalf("derived public key = %q, want %q", clients[0].PublicKey, wantPub)
	}
}

func TestDefaultWireguardClientsPreservesProvided(t *testing.T) {
	clients := []model.Client{{
		Email:      "c@wg",
		PrivateKey: "keep-priv",
		PublicKey:  "keep-pub",
		AllowedIPs: []string{"10.0.0.50/32"},
	}}
	ifaces := []any{map[string]any{"email": "c@wg"}}
	if err := defaultWireguardClients(nil, clients, ifaces); err != nil {
		t.Fatalf("defaultWireguardClients: %v", err)
	}
	if clients[0].PrivateKey != "keep-priv" || clients[0].PublicKey != "keep-pub" {
		t.Fatalf("provided keys were rotated: %+v", clients[0])
	}
	if clients[0].AllowedIPs[0] != "10.0.0.50/32" {
		t.Fatalf("provided allowedIPs changed: %v", clients[0].AllowedIPs)
	}
}

func TestWireguardAllocationBase(t *testing.T) {
	tests := []struct {
		name     string
		used     []string
		fallback string
		want     string
	}{
		{name: "no peers uses fallback", used: nil, fallback: "10.0.0.0/24", want: "10.0.0.0/24"},
		{name: "derives subnet from existing peer", used: []string{"172.16.0.2/32"}, fallback: "10.0.0.0/24", want: "172.16.0.0/24"},
		{name: "skips catch-all and ipv6", used: []string{"0.0.0.0/0", "::/0", "fd00::2/128", "192.168.5.7/32"}, fallback: "10.0.0.0/24", want: "192.168.5.0/24"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wireguardAllocationBase(tt.used, tt.fallback); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultWireguardClientsHonorsExistingSubnet(t *testing.T) {
	existing := []model.Client{{Email: "old@wg", AllowedIPs: []string{"172.16.0.2/32"}}}
	clients := []model.Client{{Email: "new@wg"}}
	ifaces := []any{map[string]any{"email": "new@wg"}}
	if err := defaultWireguardClients(existing, clients, ifaces); err != nil {
		t.Fatalf("defaultWireguardClients: %v", err)
	}
	if got := clients[0].AllowedIPs[0]; got != "172.16.0.3/32" {
		t.Fatalf("new client address = %q, want 172.16.0.3/32 in existing subnet", got)
	}
}

func TestDefaultWireguardClientsAllocatesDistinctIPs(t *testing.T) {
	clients := []model.Client{{Email: "x@wg"}, {Email: "y@wg"}}
	ifaces := []any{map[string]any{"email": "x@wg"}, map[string]any{"email": "y@wg"}}
	if err := defaultWireguardClients(nil, clients, ifaces); err != nil {
		t.Fatalf("defaultWireguardClients: %v", err)
	}
	if clients[0].AllowedIPs[0] == clients[1].AllowedIPs[0] {
		t.Fatalf("two clients got the same address: %v", clients[0].AllowedIPs)
	}
}

func TestNormalizeWireguardAllowedIPs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
		err  bool
	}{
		{name: "cidr passes through", in: []string{"10.0.0.5/32"}, want: []string{"10.0.0.5/32"}},
		{name: "bare ipv4 becomes /32", in: []string{"10.0.0.5"}, want: []string{"10.0.0.5/32"}},
		{name: "bare ipv6 becomes /128", in: []string{"fd00::5"}, want: []string{"fd00::5/128"}},
		{name: "trims and drops empties", in: []string{" 10.0.0.5/32 ", "", "  "}, want: []string{"10.0.0.5/32"}},
		{name: "dedupes", in: []string{"10.0.0.5/32", "10.0.0.5/32"}, want: []string{"10.0.0.5/32"}},
		{name: "routed subnet allowed", in: []string{"10.0.0.5/32", "192.168.1.0/24"}, want: []string{"10.0.0.5/32", "192.168.1.0/24"}},
		{name: "garbage rejected", in: []string{"not-an-ip"}, err: true},
		{name: "bad prefix rejected", in: []string{"10.0.0.5/99"}, err: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeWireguardAllowedIPs(tt.in)
			if tt.err {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestDefaultWireguardClientsHonorsAndValidatesSuppliedAllowedIPs(t *testing.T) {
	existing := []model.Client{{Email: "old@wg", AllowedIPs: []string{"10.0.0.2/32"}}}

	clients := []model.Client{{Email: "c@wg", AllowedIPs: []string{"10.0.0.9"}}}
	ifaces := []any{map[string]any{"email": "c@wg"}}
	if err := defaultWireguardClients(existing, clients, ifaces); err != nil {
		t.Fatalf("defaultWireguardClients: %v", err)
	}
	if len(clients[0].AllowedIPs) != 1 || clients[0].AllowedIPs[0] != "10.0.0.9/32" {
		t.Fatalf("supplied allowedIPs not normalized: %v", clients[0].AllowedIPs)
	}

	dup := []model.Client{{Email: "d@wg", AllowedIPs: []string{"10.0.0.2/32"}}}
	err := defaultWireguardClients(existing, dup, []any{map[string]any{"email": "d@wg"}})
	if err == nil {
		t.Fatal("duplicate allowedIPs across clients must be rejected")
	}

	bad := []model.Client{{Email: "e@wg", AllowedIPs: []string{"not-an-ip"}}}
	if err := defaultWireguardClients(existing, bad, []any{map[string]any{"email": "e@wg"}}); err == nil {
		t.Fatal("invalid allowedIPs entry must be rejected")
	}
}
