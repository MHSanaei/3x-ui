package sub

import (
	"net"
	"net/netip"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// shippedTrustedProxyCIDRs is the factory default of the trustedProxyCIDRs
// setting, read from the setting service rather than restated here so the gate
// cannot drift from the value an unconfigured install actually carries.
var shippedTrustedProxyCIDRs = sync.OnceValue(func() string {
	return (&service.SettingService{}).GetFactoryDefaults()["trustedProxyCIDRs"]
})

var forwardedHeaderNames = []string{"X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-IP"}

var warnSuppressedForwardedOnce sync.Once

// forwardedHeadersTrusted reports whether X-Forwarded-* headers on this request
// may steer the generated subscription URLs. The gate engages only once the
// operator has moved trustedProxyCIDRs off its shipped default; an install that
// never touched the setting keeps the legacy behavior. The recover mirrors
// internal/web/controller/util.go, where a setting read before InitDB panics
// rather than returning an error.
func (s *SubService) forwardedHeadersTrusted(c *gin.Context) (trusted bool) {
	trusted = true
	defer func() {
		_ = recover()
	}()

	configured, err := s.settingService.GetTrustedProxyCIDRs()
	if err != nil {
		return true
	}
	configured = strings.TrimSpace(configured)
	if configured == "" || configured == shippedTrustedProxyCIDRs() {
		return true
	}
	return remoteAddrInCIDRs(c.Request.RemoteAddr, configured)
}

// warnSuppressedForwardedHeaders makes the gate diagnosable: an operator whose
// subscription links start carrying the raw request host otherwise has nothing
// connecting that to the setting they changed. The warning fires once per
// process so a polling client cannot flood the log; every occurrence is logged
// at debug level.
func warnSuppressedForwardedHeaders(c *gin.Context) {
	present := make([]string, 0, len(forwardedHeaderNames))
	for _, name := range forwardedHeaderNames {
		if c.GetHeader(name) != "" {
			present = append(present, name)
		}
	}
	if len(present) == 0 {
		return
	}
	headers := strings.Join(present, ", ")
	logger.Debugf("sub: ignoring %s from %s, which is outside trustedProxyCIDRs", headers, c.Request.RemoteAddr)
	warnSuppressedForwardedOnce.Do(func() {
		logger.Warningf("sub: ignoring %s from %s because it is outside trustedProxyCIDRs; subscription URLs will use the request host. Add the proxy to that setting, or set subURI, if the generated links look wrong.", headers, c.Request.RemoteAddr)
	})
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
