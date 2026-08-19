package netsafe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strings"
	"time"
)

// ErrPrivateAddressBlocked marks a failed dial where the guard refused at least
// one resolved address, so a caller offering an opt-in can tell it apart from an
// ordinary connection failure.
var ErrPrivateAddressBlocked = errors.New("blocked private/internal address")

// Ranges Go's net.IP predicates do not treat as internal. The transition
// mechanisms here are deprecated (RFC 7526) or local-use, so none carry public traffic.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),  // CGNAT (RFC 6598)
	netip.MustParsePrefix("2002::/16"),      // 6to4 (RFC 3056)
	netip.MustParsePrefix("2001::/32"),      // Teredo (RFC 4380)
	netip.MustParsePrefix("64:ff9b:1::/48"), // NAT64 local-use (RFC 8215)
	netip.MustParsePrefix("fec0::/10"),      // site-local (RFC 3879)
}

// Judged by the IPv4 it embeds rather than blocked outright: on a DNS64 network
// every public IPv4 host resolves into this prefix (RFC 6052 mandates /96 here).
var nat64WellKnown = netip.MustParsePrefix("64:ff9b::/96")

func IsBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	if nat64WellKnown.Contains(addr) {
		embedded := addr.As16()
		return IsBlockedIP(net.IP(embedded[12:16]))
	}
	return false
}

type allowPrivateCtxKey struct{}

func ContextWithAllowPrivate(ctx context.Context, allow bool) context.Context {
	return context.WithValue(ctx, allowPrivateCtxKey{}, allow)
}

func AllowPrivateFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(allowPrivateCtxKey{}).(bool)
	return v
}

var defaultDialer = &net.Dialer{Timeout: 10 * time.Second}

func SSRFGuardedDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	allowPrivate := AllowPrivateFromContext(ctx)
	var ips []net.IPAddr
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IPAddr{{IP: ip}}
	} else {
		ips, err = net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
	}
	var lastErr, blockedErr error
	for _, ipAddr := range ips {
		if !allowPrivate && IsBlockedIP(ipAddr.IP) {
			blockedErr = fmt.Errorf("%w %s", ErrPrivateAddressBlocked, ipAddr.IP)
			continue
		}
		conn, derr := defaultDialer.DialContext(ctx, network, net.JoinHostPort(ipAddr.IP.String(), port))
		if derr == nil {
			return conn, nil
		}
		lastErr = derr
	}
	// A dual-stack name can mix refused and merely unreachable addresses, so the
	// refusal is reported alongside instead of being lost to the last failure.
	if blockedErr != nil {
		if lastErr != nil {
			return nil, fmt.Errorf("%w; %w", blockedErr, lastErr)
		}
		return nil, blockedErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no usable address for %s", host)
	}
	return nil, lastErr
}

var hostnamePattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)*$`)

func NormalizeHost(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", fmt.Errorf("address is required")
	}
	if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
		addr = addr[1 : len(addr)-1]
	}
	if ip := net.ParseIP(addr); ip != nil {
		return ip.String(), nil
	}
	if len(addr) > 253 || !hostnamePattern.MatchString(addr) {
		return "", fmt.Errorf("invalid host %q", addr)
	}
	return addr, nil
}
