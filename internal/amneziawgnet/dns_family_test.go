package amneziawgnet

import (
	"net/netip"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

func TestDNSQueryTypesFor(t *testing.T) {
	v4 := netip.MustParseAddr("10.8.0.2")
	v6 := netip.MustParseAddr("2001:db8::2")
	mapped := netip.MustParseAddr("::ffff:10.8.0.2")

	cases := []struct {
		name  string
		addrs []netip.Addr
		want  []dnsmessage.Type
	}{
		{
			name:  "v4-only",
			addrs: []netip.Addr{v4},
			want:  []dnsmessage.Type{dnsmessage.TypeA},
		},
		{
			name:  "v6-only",
			addrs: []netip.Addr{v6},
			want:  []dnsmessage.Type{dnsmessage.TypeAAAA},
		},
		{
			name:  "dual-stack prefers A then AAAA",
			addrs: []netip.Addr{v4, v6},
			want:  []dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA},
		},
		{
			name:  "empty falls back to A then AAAA",
			addrs: nil,
			want:  []dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA},
		},
		{
			name:  "v4-mapped alone is not dual-stack",
			addrs: []netip.Addr{mapped},
			want:  []dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dnsQueryTypesFor(tc.addrs)
			if len(got) != len(tc.want) {
				t.Fatalf("dnsQueryTypesFor(%v) = %v, want %v", tc.addrs, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("dnsQueryTypesFor(%v) = %v, want %v", tc.addrs, got, tc.want)
				}
			}
		})
	}
}

func TestTunnelSupportsAddr(t *testing.T) {
	v4 := netip.MustParseAddr("10.8.0.2")
	v6 := netip.MustParseAddr("2001:db8::2")
	dest4 := netip.MustParseAddr("8.8.8.8")
	dest6 := netip.MustParseAddr("2001:4860:4860::8888")
	mappedDest := netip.MustParseAddr("::ffff:8.8.8.8")

	if !tunnelSupportsAddr([]netip.Addr{v4}, dest4) {
		t.Error("v4 tunnel should dial IPv4")
	}
	if tunnelSupportsAddr([]netip.Addr{v4}, dest6) {
		t.Error("v4-only tunnel must not dial IPv6")
	}
	if !tunnelSupportsAddr([]netip.Addr{v6}, dest6) {
		t.Error("v6 tunnel should dial IPv6")
	}
	if tunnelSupportsAddr([]netip.Addr{v6}, dest4) {
		t.Error("v6-only tunnel must not dial IPv4")
	}
	if !tunnelSupportsAddr([]netip.Addr{v4, v6}, dest4) || !tunnelSupportsAddr([]netip.Addr{v4, v6}, dest6) {
		t.Error("dual-stack tunnel should dial both families")
	}
	if !tunnelSupportsAddr([]netip.Addr{v4}, mappedDest) {
		t.Error("v4 tunnel should treat IPv4-mapped destinations as IPv4")
	}
	if tunnelSupportsAddr(nil, dest4) {
		t.Error("empty address list should not claim support")
	}
}

func TestSocksTargetResolveTunnelVia_RejectsWrongFamilyLiteral(t *testing.T) {
	dev := &Device{localAddrs: []netip.Addr{netip.MustParseAddr("10.8.0.2")}}
	target := socksTarget{ip: netip.MustParseAddr("2001:4860:4860::8888"), port: 443}
	_, err := target.resolveTunnelVia("", "awg", dev)
	if err == nil {
		t.Fatal("expected error dialing IPv6 literal on v4-only tunnel")
	}

	okTarget := socksTarget{ip: netip.MustParseAddr("8.8.8.8"), port: 443}
	got, err := okTarget.resolveTunnelVia("", "awg", dev)
	if err != nil {
		t.Fatalf("v4 literal on v4 tunnel: %v", err)
	}
	if got.String() != "8.8.8.8:443" {
		t.Fatalf("got %s", got)
	}
}
