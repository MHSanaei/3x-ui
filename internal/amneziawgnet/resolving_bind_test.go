package amneziawgnet

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"
)

func mustResolvingBind(t *testing.T) *resolvingBind {
	t.Helper()
	return newResolvingBind("")
}

// endpointAddrPort reads any bind's endpoint; Windows' default bind has its own type.
func endpointAddrPort(t *testing.T, ep awgconn.Endpoint) netip.AddrPort {
	t.Helper()
	ap, err := netip.ParseAddrPort(ep.DstToString())
	if err != nil {
		t.Fatalf("endpoint %q: %v", ep.DstToString(), err)
	}
	return ap
}

// ownEndpointBind accepts only endpoints it parsed itself, as WinRingBind does.
type ownEndpointBind struct{ awgconn.Bind }

type ownEndpoint struct{ awgconn.StdNetEndpoint }

func (ownEndpointBind) ParseEndpoint(s string) (awgconn.Endpoint, error) {
	ap, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
	}
	return &ownEndpoint{awgconn.StdNetEndpoint{AddrPort: ap}}, nil
}

// WinRingBind, the default bind on Windows, refuses to send to an endpoint of any
// other type, so a hand-built StdNetEndpoint killed every handshake there.
func TestResolvingBind_ParseEndpointComesFromTheWrappedBind(t *testing.T) {
	ep, err := (&resolvingBind{Bind: ownEndpointBind{}}).ParseEndpoint("203.0.113.7:51820")
	if err != nil {
		t.Fatalf("ParseEndpoint: %v", err)
	}
	if _, ok := ep.(*ownEndpoint); !ok {
		t.Fatalf("endpoint is %T, not the wrapped bind's own type", ep)
	}
}

func TestResolvingBind_ParseEndpointIPLiteral(t *testing.T) {
	b := mustResolvingBind(t)
	ep, err := b.ParseEndpoint("203.0.113.7:51820")
	if err != nil {
		t.Fatalf("IP endpoint rejected: %v", err)
	}
	got := endpointAddrPort(t, ep)
	if got.Addr().String() != "203.0.113.7" || got.Port() != 51820 {
		t.Fatalf("endpoint = %v, want 203.0.113.7:51820", got)
	}
}

func TestResolvingBind_ParseEndpointHostnameResolves(t *testing.T) {
	orig := lookupEndpointHost
	lookupEndpointHost = func(ctx context.Context, host string) ([]netip.Addr, error) {
		if host != "peer.example.test" {
			t.Errorf("unexpected lookup host %q", host)
		}
		return []netip.Addr{netip.MustParseAddr("198.51.100.9")}, nil
	}
	defer func() { lookupEndpointHost = orig }()

	b := mustResolvingBind(t)
	ep, err := b.ParseEndpoint("peer.example.test:443")
	if err != nil {
		t.Fatalf("hostname endpoint rejected: %v", err)
	}
	if got := endpointAddrPort(t, ep); got.Addr().String() != "198.51.100.9" || got.Port() != 443 {
		t.Fatalf("endpoint = %v, want 198.51.100.9:443", got)
	}
}

func TestResolvingBind_ParseEndpointResolveFailureIsAnError(t *testing.T) {
	orig := lookupEndpointHost
	lookupEndpointHost = func(ctx context.Context, host string) ([]netip.Addr, error) {
		return nil, errors.New("no such host")
	}
	defer func() { lookupEndpointHost = orig }()

	b := mustResolvingBind(t)
	if _, err := b.ParseEndpoint("missing.example.test:80"); err == nil {
		t.Fatal("expected resolve failure to surface as an error")
	}
}

func TestResolvingBind_ParseEndpointBadPortRejected(t *testing.T) {
	b := mustResolvingBind(t)
	if _, err := b.ParseEndpoint("203.0.113.7:none"); err == nil {
		t.Fatal("expected bad port to be rejected")
	}
}
