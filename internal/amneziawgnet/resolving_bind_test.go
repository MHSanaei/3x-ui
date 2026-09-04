package amneziawgnet

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"
)

func endpointAddrPort(ep awgconn.Endpoint) netip.AddrPort {
	std, ok := ep.(*awgconn.StdNetEndpoint)
	if !ok {
		panic("unexpected endpoint type")
	}
	return std.AddrPort
}

func TestResolvingBind_ParseEndpointIPLiteral(t *testing.T) {
	b := newResolvingBind()
	ep, err := b.ParseEndpoint("203.0.113.7:51820")
	if err != nil {
		t.Fatalf("IP endpoint rejected: %v", err)
	}
	got := endpointAddrPort(ep)
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

	b := newResolvingBind()
	ep, err := b.ParseEndpoint("peer.example.test:443")
	if err != nil {
		t.Fatalf("hostname endpoint rejected: %v", err)
	}
	if got := endpointAddrPort(ep); got.Addr().String() != "198.51.100.9" || got.Port() != 443 {
		t.Fatalf("endpoint = %v, want 198.51.100.9:443", got)
	}
}

func TestResolvingBind_ParseEndpointResolveFailureIsAnError(t *testing.T) {
	orig := lookupEndpointHost
	lookupEndpointHost = func(ctx context.Context, host string) ([]netip.Addr, error) {
		return nil, errors.New("no such host")
	}
	defer func() { lookupEndpointHost = orig }()

	b := newResolvingBind()
	if _, err := b.ParseEndpoint("missing.example.test:80"); err == nil {
		t.Fatal("expected resolve failure to surface as an error")
	}
}

func TestResolvingBind_ParseEndpointBadPortRejected(t *testing.T) {
	b := newResolvingBind()
	if _, err := b.ParseEndpoint("203.0.113.7:none"); err == nil {
		t.Fatal("expected bad port to be rejected")
	}
}
