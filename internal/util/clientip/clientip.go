// Package clientip resolves the address a request came from behind trusted reverse proxies.
package clientip

import (
	"net"
	"net/netip"
	"strings"
)

// Resolve returns the address rate limiters key on: forwarded headers count only
// when remoteAddr is inside trustedCIDRs, and never when they contradict each other.
func Resolve(remoteAddr, xRealIP, xForwardedFor, trustedCIDRs string) string {
	remoteIP, ok := Parse(remoteAddr)
	if !ok {
		return "unknown"
	}
	if !Trusted(remoteIP, trustedCIDRs) {
		return remoteIP.String()
	}

	realIP, hasRealIP := Parse(xRealIP)
	forwardedIP, hasForwarded := forwardedClient(xForwardedFor, trustedCIDRs)
	switch {
	case hasRealIP && hasForwarded && realIP != forwardedIP:
		// A proxy rewrites at most one of the two, so the other is the client's
		// own claim; neither may pick the identity a limiter keys on.
		return remoteIP.String()
	case hasForwarded:
		return forwardedIP.String()
	case hasRealIP:
		return realIP.String()
	}
	return remoteIP.String()
}

// forwardedClient walks X-Forwarded-For from the proxy end: entries a client sent
// come first, so the first hop that is not a trusted proxy is the address one saw.
func forwardedClient(xForwardedFor, trustedCIDRs string) (netip.Addr, bool) {
	parts := strings.Split(xForwardedFor, ",")
	var client netip.Addr
	found := false
	for i := len(parts) - 1; i >= 0; i-- {
		ip, ok := Parse(parts[i])
		if !ok {
			break
		}
		client, found = ip, true
		if !Trusted(ip, trustedCIDRs) {
			break
		}
	}
	return client, found
}

// Trusted reports whether addr is inside the comma-separated CIDR or address list;
// an entry that parses as neither is skipped.
func Trusted(addr netip.Addr, trustedCIDRs string) bool {
	addr = addr.Unmap()
	for value := range strings.SplitSeq(trustedCIDRs, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(value); err == nil {
			if prefix.Contains(addr) {
				return true
			}
			continue
		}
		if proxyIP, err := netip.ParseAddr(value); err == nil && proxyIP.Unmap() == addr {
			return true
		}
	}
	return false
}

// Parse reads an address from a header value or a RemoteAddr, with or without a port.
func Parse(value string) (netip.Addr, bool) {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return netip.Addr{}, false
	}
	if ip, err := netip.ParseAddr(candidate); err == nil {
		return ip.Unmap(), true
	}
	if host, _, err := net.SplitHostPort(candidate); err == nil {
		if ip, err := netip.ParseAddr(strings.TrimSpace(host)); err == nil {
			return ip.Unmap(), true
		}
	}
	if strings.Count(candidate, ":") == 1 {
		if host, _, err := net.SplitHostPort("[" + candidate + "]"); err == nil {
			if ip, err := netip.ParseAddr(strings.TrimSpace(host)); err == nil {
				return ip.Unmap(), true
			}
		}
	}
	return netip.Addr{}, false
}
