package job

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// The protocol gate must let hysteria through: XrayAPI supports it, and the
// skip left over-limit Hysteria2 sessions alive until the fail2ban ban caught up.
func TestDisconnectClientTemporarilyAllowsHysteria(t *testing.T) {
	setupIntegrationDB(t)

	const email = "hy2-limit-probe"
	inbound := &model.Inbound{
		Id:       1,
		Protocol: model.Hysteria,
		Tag:      "hy2-limit-probe-tag",
		Settings: `{"clients":[]}`,
	}
	clients := []model.Client{{Email: email, Auth: "secret"}}

	(&CheckClientIpJob{}).disconnectClientTemporarily(inbound, email, clients)

	var unsupported, attempted bool
	for _, line := range logger.GetLogs(500, "warning") {
		if strings.Contains(line, "Temporary disconnect is not supported for protocol hysteria") {
			unsupported = true
		}
		if strings.Contains(line, "Failed to remove user "+email) {
			attempted = true
		}
	}
	if unsupported {
		t.Fatal("hysteria was rejected by the protocol gate")
	}
	if !attempted {
		t.Fatal("expected a remove attempt against the Xray API for hysteria")
	}
}
