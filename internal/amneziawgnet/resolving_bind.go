package amneziawgnet

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"
)

// endpointResolveTimeout bounds the one-time DNS lookup in ParseEndpoint.
const endpointResolveTimeout = 5 * time.Second

// resolvingBind wraps a Bind so peer endpoints may be hostnames (#6367).
// Concrete listen values use pinnedBind; wildcards keep StdNetBind.
type resolvingBind struct {
	awgconn.Bind
}

var lookupEndpointHost = defaultLookupEndpointHost

func defaultLookupEndpointHost(ctx context.Context, host string) ([]netip.Addr, error) {
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	out := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, a.Unmap())
	}
	return out, nil
}

func newResolvingBind(listen string) *resolvingBind {
	return &resolvingBind{Bind: newListenBind(listen)}
}

// ParseEndpoint resolves hostnames, then lets the wrapped bind build the endpoint:
// its own parser takes literal IPs only, and WinRingBind sends to its own type only.
func (b *resolvingBind) ParseEndpoint(s string) (awgconn.Endpoint, error) {
	host, portStr, err := net.SplitHostPort(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("endpoint %q: %w", s, err)
	}
	port64, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil || port64 == 0 {
		return nil, fmt.Errorf("endpoint %q: bad port", s)
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), endpointResolveTimeout)
		defer cancel()
		addrs, rerr := lookupEndpointHost(ctx, host)
		if rerr != nil {
			return nil, fmt.Errorf("endpoint %q: resolve host: %w", s, rerr)
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("endpoint %q: host resolved to no addresses", s)
		}
		addr = addrs[0]
	}
	return b.Bind.ParseEndpoint(netip.AddrPortFrom(addr.Unmap(), uint16(port64)).String())
}
