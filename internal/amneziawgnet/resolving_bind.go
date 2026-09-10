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

// resolvingBind lets peer endpoints be hostnames: StdNetBind has no DNS and
// an unresolved name kills the whole IpcSet. Resolved once at configure.
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

func newResolvingBind() *resolvingBind {
	return &resolvingBind{Bind: awgconn.NewDefaultBind()}
}

// ParseEndpoint resolves hostnames before handing the address to amneziawg-go
// (whose own implementation accepts literal IPs only).
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
	return &awgconn.StdNetEndpoint{AddrPort: netip.AddrPortFrom(addr.Unmap(), uint16(port64))}, nil
}
