package sub

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray/dnsconf"
)

// The subJsonDns setting holds either a full xray dns block or a bare array of
// servers; dnsconf validates it against the parser the client runs.

// SetDnsConfig overrides the dns block of every emitted document; an unusable
// value is logged and ignored so a typo cannot take subscriptions down.
func (s *SubJsonService) SetDnsConfig(raw string) {
	block, err := dnsconf.Parse(raw)
	if err != nil {
		logger.Warningf("subJsonDns: %v; keeping the template DNS", err)
		return
	}
	s.dnsBlock = block
}
