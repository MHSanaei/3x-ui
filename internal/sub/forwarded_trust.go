package sub

import (
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/clientip"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

var warnSuppressedForwardedOnce sync.Once

var forwardedHeaderNames = [...]string{"X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-IP"}

func (s *SubService) forwardedHeadersTrusted(c *gin.Context) (trusted bool) {
	if !hasForwardedHeaders(c) {
		return true
	}

	trusted = true
	defer func() {
		_ = recover()
	}()

	configured, err := s.settingService.GetTrustedProxyCIDRs()
	if err != nil {
		return true
	}
	configured = strings.TrimSpace(configured)
	if configured == "" || configured == service.DefaultTrustedProxyCIDRs {
		return true
	}
	return remoteAddrInCIDRs(c.Request.RemoteAddr, configured)
}

func hasForwardedHeaders(c *gin.Context) bool {
	for _, name := range forwardedHeaderNames {
		if c.GetHeader(name) != "" {
			return true
		}
	}
	return false
}

func warnSuppressedForwardedHeaders(c *gin.Context) {
	present := make([]string, 0, 3)
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

func remoteAddrInCIDRs(remoteAddr, cidrs string) bool {
	addr, ok := clientip.Parse(remoteAddr)
	return ok && clientip.Trusted(addr, cidrs)
}
