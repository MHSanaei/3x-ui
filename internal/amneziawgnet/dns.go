package amneziawgnet

import (
	"context"
	"fmt"
	"math/rand"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// DefaultTunnelDNSServer resolves domain targets through outbound netstack.
const (
	DefaultTunnelDNSServer   = "1.1.1.1:53"
	DefaultTunnelDNSServerV6 = "[2606:4700:4700::1111]:53"
)

func deviceHasV4(addrs []netip.Addr) bool {
	for _, a := range addrs {
		if a.Is4() {
			return true
		}
	}
	return false
}

func deviceHasV6(addrs []netip.Addr) bool {
	for _, a := range addrs {
		if a.Is6() && !a.Is4In6() {
			return true
		}
	}
	return false
}

// defaultDNSFor picks a default resolver matching the tunnel address family:
// IPv4 default when the tunnel carries IPv4 (or empty), IPv6 default when
// the tunnel is IPv6-only.
func defaultDNSFor(addrs []netip.Addr) string {
	if deviceHasV4(addrs) || len(addrs) == 0 {
		return DefaultTunnelDNSServer
	}
	return DefaultTunnelDNSServerV6
}

const (
	// tunnelResolveTimeout bounds one lookup inside a live connection handler.
	tunnelResolveTimeout   = 4 * time.Second
	tunnelDNSPacketTimeout = 1200 * time.Millisecond
	tunnelDNSAttempts      = 3
)

type tunnelDNSCacheEntry struct {
	addr netip.Addr
	exp  time.Time
}

var tunnelDNSCache = struct {
	mu sync.Mutex
	m  map[string]tunnelDNSCacheEntry
}{m: map[string]tunnelDNSCacheEntry{}}

const (
	tunnelDNSCacheTTL     = 60 * time.Second
	tunnelDNSCacheMaxSize = 1024
)

// dnsCacheKey computes cache key scoped by outbound tag, server, and host.
func dnsCacheKey(tag, dnsServer, host string) string {
	return tag + "|" + dnsServer + "|" + host
}

func resolveTunnelVia(ctx context.Context, dev *Device, tag string, dnsServer string, host string) (netip.Addr, error) {
	normDNS := normalizeDNSServer(dnsServer)
	if normDNS == "" {
		normDNS = defaultDNSFor(dev.LocalAddresses())
	}
	key := dnsCacheKey(tag, normDNS, host)
	now := time.Now()
	tunnelDNSCache.mu.Lock()
	if e, ok := tunnelDNSCache.m[key]; ok && now.Before(e.exp) {
		tunnelDNSCache.mu.Unlock()
		return e.addr, nil
	}
	tunnelDNSCache.mu.Unlock()

	server, err := netip.ParseAddrPort(normDNS)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("bad tunnel DNS server %q: %w", normDNS, err)
	}
	raddr := tcpip.FullAddress{
		NIC:  1,
		Addr: tcpip.AddrFromSlice(server.Addr().AsSlice()),
		Port: server.Port(),
	}
	conn, derr := gonet.DialUDP(dev.Stack, nil, &raddr, tunnelNetwork(server.Addr()))
	if derr != nil {
		logger.Warningf("amneziawgnet: resolveTunnel tag=%q host=%q server=%s localAddrs=%v err=%v", tag, host, server, dev.LocalAddresses(), derr)
		return netip.Addr{}, fmt.Errorf("dns dial %s: %w", server, derr)
	}
	defer conn.Close()

	addr, rerr := exchangeTunnelDNSWithFallback(ctx, conn, dev.LocalAddresses(), host)
	if rerr != nil {
		return netip.Addr{}, rerr
	}

	tunnelDNSCache.mu.Lock()
	if len(tunnelDNSCache.m) >= tunnelDNSCacheMaxSize {
		tunnelDNSCache.m = map[string]tunnelDNSCacheEntry{}
	}
	tunnelDNSCache.m[key] = tunnelDNSCacheEntry{addr: addr, exp: now.Add(tunnelDNSCacheTTL)}
	tunnelDNSCache.mu.Unlock()
	logger.Debugf("amneziawgnet: resolved tag=%q %q -> %s via tunnel", tag, host, addr)
	return addr, nil
}

// flushTunnelDNSCacheForTag purges all cached DNS entries for an outbound tag.
func flushTunnelDNSCacheForTag(tag string) {
	tunnelDNSCache.mu.Lock()
	defer tunnelDNSCache.mu.Unlock()
	prefix := tag + "|"
	for k := range tunnelDNSCache.m {
		if strings.HasPrefix(k, prefix) {
			delete(tunnelDNSCache.m, k)
		}
	}
}

// exchangeTunnelDNSWithFallback queries A and/or AAAA depending on the local
// address families configured on the device stack.
func exchangeTunnelDNSWithFallback(ctx context.Context, conn *gonet.UDPConn, addrs []netip.Addr, host string) (netip.Addr, error) {
	hasV4 := deviceHasV4(addrs)
	hasV6 := deviceHasV6(addrs)

	// If the tunnel is IPv6-only, query AAAA first; else query A first.
	types := []dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA}
	if hasV6 && !hasV4 {
		types = []dnsmessage.Type{dnsmessage.TypeAAAA, dnsmessage.TypeA}
	}

	var firstErr error
	for _, qType := range types {
		// Skip AAAA if device has no IPv6 capability and has IPv4, unless A failed.
		addr, err := exchangeTunnelDNSQuery(ctx, conn, host, qType)
		if err == nil {
			return addr, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return netip.Addr{}, firstErr
}

func exchangeTunnelDNSQuery(ctx context.Context, conn *gonet.UDPConn, host string, qType dnsmessage.Type) (netip.Addr, error) {
	name, err := dnsmessage.NewName(host + ".")
	if err != nil {
		return netip.Addr{}, fmt.Errorf("dns name %q: %w", host, err)
	}
	id := uint16(rand.Intn(1 << 16))
	query := dnsmessage.Message{
		Header: dnsmessage.Header{ID: id, RecursionDesired: true},
		Questions: []dnsmessage.Question{{
			Name:  name,
			Type:  qType,
			Class: dnsmessage.ClassINET,
		}},
	}
	wire, err := query.Pack()
	if err != nil {
		return netip.Addr{}, fmt.Errorf("dns pack %q: %w", host, err)
	}

	buf := make([]byte, 512)
	for attempt := 0; attempt < tunnelDNSAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return netip.Addr{}, ctx.Err()
		default:
		}
		if _, werr := conn.Write(wire); werr != nil {
			return netip.Addr{}, fmt.Errorf("dns send %q: %w", host, werr)
		}
		if derr := conn.SetReadDeadline(time.Now().Add(tunnelDNSPacketTimeout)); derr != nil {
			return netip.Addr{}, fmt.Errorf("dns deadline %q: %w", host, derr)
		}
		for {
			n, rerr := conn.Read(buf)
			if rerr != nil {
				break // per-attempt timeout -> next attempt
			}
			var resp dnsmessage.Message
			if uerr := resp.Unpack(buf[:n]); uerr != nil || resp.ID != id {
				continue
			}
			for _, ans := range resp.Answers {
				if a, ok := ans.Body.(*dnsmessage.AResource); ok && qType == dnsmessage.TypeA {
					return netip.AddrFrom4(a.A), nil
				}
				if aaaa, ok := ans.Body.(*dnsmessage.AAAAResource); ok && qType == dnsmessage.TypeAAAA {
					return netip.AddrFrom16(aaaa.AAAA), nil
				}
			}
			return netip.Addr{}, fmt.Errorf("dns %q (type %v): rcode=%d answers=%d", host, qType, resp.RCode, len(resp.Answers))
		}
	}
	return netip.Addr{}, fmt.Errorf("dns lookup %q (type %v): no answer after %d attempts", host, qType, tunnelDNSAttempts)
}
