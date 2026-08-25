package job

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
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
//
// The same Run also reconciles the embedded AmneziaWG OUTBOUND devices
// against the "amneziawg" outbounds declared in the Xray template: same 10s
// cadence, same embedded engine (client mode instead of server mode), so one
// job keeps both directions of the protocol converged.
type AmneziaWGJob struct {
	inboundService service.InboundService
	settingService service.SettingService
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
	amneziawgnet.GetOutboundManager().Reconcile(j.desiredOutboundInstances())
}

// desiredOutboundInstances parses the stored Xray template and derives a
// client-mode OutboundInstance for every "amneziawg" outbound in it.
func (j *AmneziaWGJob) desiredOutboundInstances() []amneziawgnet.OutboundDesired {
	template, err := j.settingService.GetXrayConfigTemplate()
	if err != nil || template == "" {
		return nil
	}
	cfg := &xray.Config{}
	if err := json.Unmarshal([]byte(template), cfg); err != nil || len(cfg.OutboundConfigs) == 0 {
		return nil
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(cfg.OutboundConfigs, &raws); err != nil {
		return nil
	}
	out := make([]amneziawgnet.OutboundDesired, 0, len(raws))
	for _, raw := range raws {
		if !amneziawg.IsAmneziaWGOutbound(raw) {
			continue
		}
		var probe struct {
			Tag string `json:"tag"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil || probe.Tag == "" {
			continue
		}
		inst, ok := amneziawg.InstanceFromOutbound(probe.Tag, raw)
		if !ok {
			continue
		}
		out = append(out, amneziawgnet.OutboundDesired{
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
	return out
}
