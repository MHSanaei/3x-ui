package model

import "testing"

func TestIsHysteria(t *testing.T) {
	cases := []struct {
		in   Protocol
		want bool
	}{
		{Hysteria, true},
		{Hysteria2, true},
		{VLESS, false},
		{Shadowsocks, false},
		{Protocol(""), false},
		{Protocol("hysteria3"), false},
	}
	for _, c := range cases {
		if got := IsHysteria(c.in); got != c.want {
			t.Errorf("IsHysteria(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestSocksProtocolConstant pins the wire value of the SOCKS5 protocol
// constant. It must stay "socks" because that's the literal Xray expects
// in inbound.protocol JSON (see https://xtls.github.io/config/inbounds/socks.html);
// changing it would silently break every stored inbound row.
func TestSocksProtocolConstant(t *testing.T) {
	if got, want := string(Socks), "socks"; got != want {
		t.Errorf("Socks protocol constant = %q, want %q", got, want)
	}
	if Socks == Mixed {
		t.Error("Socks and Mixed must be distinct protocols")
	}
}

func TestIsSocksLike(t *testing.T) {
	cases := []struct {
		in   Protocol
		want bool
	}{
		{Socks, true},
		{Mixed, true},
		{HTTP, false},
		{VLESS, false},
		{VMESS, false},
		{Trojan, false},
		{Shadowsocks, false},
		{WireGuard, false},
		{Hysteria, false},
		{Hysteria2, false},
		{Tunnel, false},
		{Protocol(""), false},
		{Protocol("SOCKS"), false}, // case-sensitive: must match the stored lowercase value
	}
	for _, c := range cases {
		if got := IsSocksLike(c.in); got != c.want {
			t.Errorf("IsSocksLike(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestIsAccountBased pins the set of inbounds that route credentials
// through settings.accounts[] rather than settings.clients[]. Anything
// that returns true here is OFF-LIMITS for the client-lifecycle code
// paths (AddInboundClient / UpdateInboundClient / DelInboundClient,
// depletion, traffic reset, telegram 'add client' keyboard).
//
// HTTP is intentionally included even though the panel UI doesn't
// currently surface it as a standalone protocol — the runtime API
// AddUser branch in xray/api.go handles it symmetrically with socks,
// and any future UI work plugging HTTP back in inherits the same
// safety net for free.
//
// WireGuard is intentionally EXCLUDED: its peers live under
// settings.peers[] and are managed through a separate path; treating
// it as account-based would lock out the existing wireguard UI.
func TestIsAccountBased(t *testing.T) {
	cases := []struct {
		in   Protocol
		want bool
	}{
		{Socks, true},
		{Mixed, true},
		{HTTP, true},

		{VLESS, false},
		{VMESS, false},
		{Trojan, false},
		{Shadowsocks, false},
		{WireGuard, false}, // peers, not accounts; managed separately
		{Hysteria, false},
		{Hysteria2, false},
		{Tunnel, false},
		{Protocol(""), false},
		{Protocol("SOCKS"), false}, // case-sensitive lowercase invariant
	}
	for _, c := range cases {
		if got := IsAccountBased(c.in); got != c.want {
			t.Errorf("IsAccountBased(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
