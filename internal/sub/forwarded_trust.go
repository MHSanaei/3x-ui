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

var shippedTrustedProxyCIDRs = sync.OnceValue(func() string {
	return (&service.SettingService{}).GetFactoryDefaults()["trustedProxyCIDRs"]
})

var warnSuppressedForwardedOnce sync.Once

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

func warnSuppressedForwardedHeaders(c *gin.Context) {
	present := make([]string, 0, 3)
	for _, name := range []string{"X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-IP"} {
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

func remoteAddrInCIDRs(remoteAddr, cidrs string) bool {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return false
	}
	addr = addr.Unmap()
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
