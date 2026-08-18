package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// AmneziaWGJob reconciles the running embedded AmneziaWG interfaces
// (internal/amneziawgnet -- amneziawg-go over a gVisor netstack, no kernel
// module) against the enabled AmneziaWG inbounds in the database,
// rebuilding/reconfiguring any that drifted. Unlike the retired
// kernel-module Manager this job used to drive, there is no traffic/
// online-status accounting here at all: once a peer's decapsulated traffic
// is relayed into Xray's own SOCKS5 inbound (see
// internal/web/service/xray.go's injectAmneziawgnetSocks, and
// internal/amneziawgnet.Manager's automatic forwarder/relay wiring), it's
// an ordinary Xray user, and XrayTrafficJob's existing, protocol-blind
// stats/online-status polling already picks it up for free.
type AmneziaWGJob struct {
	inboundService service.InboundService
}

// NewAmneziaWGJob creates a new AmneziaWG reconcile job instance.
func NewAmneziaWGJob() *AmneziaWGJob {
	return new(AmneziaWGJob)
}

// Run reconciles desired AmneziaWG inbounds with running embedded interfaces.
func (j *AmneziaWGJob) Run() {
	desired, err := j.inboundService.DesiredAmneziaWGInstances()
	if err != nil {
		logger.Warning("amneziawg job: get desired instances failed:", err)
		return
	}

	wanted := make([]amneziawgnet.Desired, 0, len(desired))
	for _, inst := range desired {
		wanted = append(wanted, amneziawgnet.Desired{
			Instance: inst,
			Options: amneziawgnet.DeviceOptions{
				HeaderProtectionKey:    inst.Obfuscation.HeaderProtectionKey,
				ContentPaddingAddition: inst.Obfuscation.ContentPaddingAddition,
				RekeyAfterTime:         inst.Obfuscation.RekeyAfterTime,
				RekeyTimeout:           inst.Obfuscation.RekeyTimeout,
				RejectAfterTime:        inst.Obfuscation.RejectAfterTime,
				KeepaliveTimeout:       inst.Obfuscation.KeepaliveTimeout,
				MaxHandshakeAttempts:   inst.Obfuscation.MaxHandshakeAttempts,
				RandomTrailers:         inst.Obfuscation.RandomTrailers,
				DisableCookies:         inst.Obfuscation.DisableCookies,
			},
		})
	}
	amneziawgnet.GetManager().Reconcile(wanted)
}
