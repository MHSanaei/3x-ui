package job

import (
	"encoding/json"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgkern"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// endpointWindow is how long a kernel-mode peer's connect location stays
// counted toward its IP limit -- Xray's access log never sees these peers.
const endpointWindow = 30 * time.Minute

// AmneziaWGJob converges AmneziaWG interfaces (inbounds on either datapath AND
// the template's "amneziawg" outbounds) every 10s; only kernel mode needs stats.
type AmneziaWGJob struct {
	inboundService service.InboundService
	settingService service.SettingService
	xrayService    service.XrayService
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

	embedded := make([]amneziawgnet.Desired, 0, len(desired))
	kernel := make([]amneziawgkern.Desired, 0, len(desired))
	for _, inst := range desired {
		if inst.KernelMode {
			kernel = append(kernel, amneziawgkern.Desired{
				Instance:         inst,
				EgressInterface:  inst.ExternalInterface,
				RouteThroughXray: inst.RouteThroughXray,
			})
			continue
		}
		embedded = append(embedded, amneziawgnet.Desired{
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
	reconcileAmneziaWGDatapaths(embedded, kernel, amneziawgnet.GetManager(), amneziawgkern.GetManager())
	j.collectKernelTraffic(kernel)

	outboundDesired, err := j.desiredOutboundInstances()
	if err != nil {
		logger.Warning("amneziawg job: get desired outbound instances failed:", err)
		return
	}
	amneziawgnet.GetOutboundManager().Reconcile(outboundDesired)
	// Xray's bridges are generated apart from the listener; one that moved needs them regenerated.
	if amneziawgnet.BridgesStale() {
		j.xrayService.SetToNeedRestart()
	}
}

type embeddedInboundManager interface {
	Remove(int)
	Reconcile([]amneziawgnet.Desired)
}

type kernelInboundManager interface {
	Reconcile([]amneziawgkern.Desired)
	Running(int) bool
}

func reconcileAmneziaWGDatapaths(embedded []amneziawgnet.Desired, kernel []amneziawgkern.Desired, embeddedManager embeddedInboundManager, kernelManager kernelInboundManager) {
	for _, d := range kernel {
		embeddedManager.Remove(d.Instance.Id)
	}
	kernelManager.Reconcile(kernel)
	readyEmbedded := embedded[:0]
	for _, d := range embedded {
		if kernelManager.Running(d.Instance.Id) {
			continue
		}
		readyEmbedded = append(readyEmbedded, d)
	}
	embeddedManager.Reconcile(readyEmbedded)
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

// collectKernelTraffic feeds each kernel interface's per-peer counter
// increments into the same accounting the rest of the panel uses.
func (j *AmneziaWGJob) collectKernelTraffic(kernel []amneziawgkern.Desired) {
	collectKernelTrafficWith(
		&j.inboundService, kernel, amneziawgkern.GetManager(),
		amneziawgkern.GetCollector(), amneziawgkern.Dump,
	)
}

type kernelTrafficManager interface {
	TrafficStateFor(id int) (amneziawgkern.TrafficInterfaceState, bool)
	PrimeAdoptedTrafficBaseline(id int, generation uint64, stats []amneziawgkern.PeerStat, collector *amneziawgkern.Collector) bool
}

type kernelTrafficDumper func(string) ([]amneziawgkern.PeerStat, error)

func collectKernelTrafficWith(inboundService *service.InboundService, kernel []amneziawgkern.Desired, manager kernelTrafficManager, collector *amneziawgkern.Collector, dump kernelTrafficDumper) {
	if len(kernel) == 0 {
		return
	}

	now := time.Now()

	var traffics []*xray.ClientTraffic
	for _, d := range kernel {
		state, ok := manager.TrafficStateFor(d.Instance.Id)
		if !ok {
			continue
		}
		stats, err := dump(state.Name)
		if err != nil {
			logger.Warning("amneziawg job: reading counters for ", state.Name, ": ", err)
			continue
		}
		if len(stats) == 0 {
			continue
		}
		current, ok := manager.TrafficStateFor(d.Instance.Id)
		if !ok || current.Name != state.Name || current.Generation != state.Generation {
			continue
		}
		if state.Adopted && !manager.PrimeAdoptedTrafficBaseline(d.Instance.Id, state.Generation, stats, collector) {
			continue
		}

		emails := make(map[string]string, len(d.Instance.Peers))
		for _, p := range d.Instance.Peers {
			emails[p.PublicKey] = p.Email
		}
		collector.SetEmails(state.Name, emails)
		collector.ObserveEndpoints(state.Name, stats, now, endpointWindow)
		collector.ObserveHandshakes(state.Name, stats)
		if state.Adopted {
			// Downtime traffic is unknowable without durable accounting state.
			continue
		}

		// Deltas, never totals: kernel counters restart at zero on every
		// interface rebuild, and AddTraffic adds whatever it is given.
		for _, delta := range collector.Delta(state.Name, stats) {
			email := emails[delta.PublicKey]
			if email == "" {
				continue
			}
			traffics = append(traffics, &xray.ClientTraffic{
				InboundId: d.Instance.Id,
				Email:     email,
				Up:        delta.Up,
				Down:      delta.Down,
			})
		}
	}

	if len(traffics) == 0 {
		return
	}
	// A client that just crossed its limit is disabled here; the next reconcile
	// tick drops its peer from the interface, which is what cuts it off.
	if _, _, err := inboundService.AddTraffic(nil, traffics); err != nil {
		logger.Warning("amneziawg job: recording kernel-mode traffic:", err)
	}
}
