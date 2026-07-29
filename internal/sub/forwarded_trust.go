package sub

import (
	"net"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"
)

// defaultTrustedProxyCIDRs mirrors the shipped default of the trustedProxyCIDRs
// setting. An operator who never touched that setting still carries this value,
// which is how forwardedHeadersTrusted tells "unconfigured" from "declared".
const defaultTrustedProxyCIDRs = "127.0.0.1/32,::1/128"

// forwardedHeadersTrusted reports whether X-Forwarded-* headers on this request
// may drive the generated subscription URLs.
//
// The subscription server has always taken X-Forwarded-Host and
// X-Forwarded-Proto at face value, so a client can point its own links at any
// host by sending the header. Enforcing the panel's trustedProxyCIDRs
// unconditionally would break every deployment whose reverse proxy is not on
// the loopback default, so the check only engages once an operator has
// declared a trust boundary by changing the setting away from its default.
// Unconfigured installs keep the legacy behavior byte for byte.
func (s *SubService) forwardedHeadersTrusted(c *gin.Context) bool {
	configured, err := s.settingService.GetTrustedProxyCIDRs()
	if err != nil {
		return true
	}
	configured = strings.TrimSpace(configured)
	if configured == "" || configured == defaultTrustedProxyCIDRs {
		return true
	}
	return remoteAddrInCIDRs(c.Request.RemoteAddr, configured)
}

// remoteAddrInCIDRs reports whether remoteAddr falls inside any CIDR or bare
// address in the comma-separated list, mirroring the panel-side check in
// internal/web/controller/util.go.
func remoteAddrInCIDRs(remoteAddr, cidrs string) bool {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return false
	}
	for value := range strings.SplitSeq(cidrs, ",") {
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
		if proxyIP, err := netip.ParseAddr(value); err == nil && proxyIP.Unmap() == addr.Unmap() {
			return true
		}
	}
	return false
}
