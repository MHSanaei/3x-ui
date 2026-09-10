package job

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// AmneziaWGJob converges embedded AmneziaWG interfaces (inbounds AND the
// template's "amneziawg" outbounds) every 10s; stats stay with Xray.
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

	outboundDesired, err := j.desiredOutboundInstances()
	if err != nil {
		logger.Warning("amneziawg job: get desired outbound instances failed:", err)
		return
	}
	amneziawgnet.GetOutboundManager().Reconcile(outboundDesired)
}

// desiredOutboundInstances derives client instances per template "amneziawg" outbound.
func (j *AmneziaWGJob) desiredOutboundInstances() ([]amneziawgnet.OutboundDesired, error) {
	template, err := j.settingService.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}
	if template == "" {
		return nil, nil
	}
	cfg := &xray.Config{}
	if err := json.Unmarshal([]byte(template), cfg); err != nil {
		return nil, err
	}
	if len(cfg.OutboundConfigs) == 0 {
		return nil, nil
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(cfg.OutboundConfigs, &raws); err != nil {
		return nil, err
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
	return out, nil
}
