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

	embedded, kernel := splitAmneziaWGDatapaths(desired)
	// Counters first: reconcile drops disabled peers and removed interfaces,
	// and their counters with them.
	j.collectKernelTraffic(kernel)
	reconcileAmneziaWGDatapaths(embedded, kernel, amneziawgnet.GetManager(), amneziawgkern.GetManager())

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

// FlushKernelTraffic charges what kernel interfaces counted since the last
// tick; shutdown runs it before tearing the interfaces and counters down.
func (j *AmneziaWGJob) FlushKernelTraffic() {
	// With no kernel interface there is nothing to charge, so a panel that
	// never used kernel mode shuts down without touching the database.
	if !amneziawgkern.GetManager().HasRunning() {
		return
	}
	desired, err := j.inboundService.DesiredAmneziaWGInstances()
	if err != nil {
		logger.Warning("amneziawg job: get desired instances for the final traffic flush failed:", err)
	}
	_, kernel := splitAmneziaWGDatapaths(desired)
	j.collectKernelTraffic(kernel)
}

func splitAmneziaWGDatapaths(desired []amneziawg.Instance) ([]amneziawgnet.Desired, []amneziawgkern.Desired) {
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
	return embedded, kernel
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
	ManagedIDs() []int
	TrafficStateFor(id int) (amneziawgkern.TrafficInterfaceState, bool)
	PrimeAdoptedTrafficBaseline(id int, generation uint64, stats []amneziawgkern.PeerStat, collector *amneziawgkern.Collector) bool
}

type kernelTrafficDumper func(string) ([]amneziawgkern.PeerStat, error)

// collectKernelTrafficWith reads every managed interface, not only desired
// ones, so a peer or inbound that is about to be dropped is still charged.
func collectKernelTrafficWith(inboundService *service.InboundService, kernel []amneziawgkern.Desired, manager kernelTrafficManager, collector *amneziawgkern.Collector, dump kernelTrafficDumper) {
	emailsByID := make(map[int]map[string]string, len(kernel))
	for _, d := range kernel {
		emails := make(map[string]string, len(d.Instance.Peers))
		for _, p := range d.Instance.Peers {
			emails[p.PublicKey] = p.Email
		}
		emailsByID[d.Instance.Id] = emails
	}

	now := time.Now()
	var traffics []*xray.ClientTraffic
	for _, id := range manager.ManagedIDs() {
		state, ok := manager.TrafficStateFor(id)
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
		current, ok := manager.TrafficStateFor(id)
		if !ok || current.Name != state.Name || current.Generation != state.Generation {
			continue
		}
		if state.Adopted && !manager.PrimeAdoptedTrafficBaseline(id, state.Generation, stats, collector) {
			continue
		}

		emails, desired := emailsByID[id]
		// Adopted: downtime traffic is unknowable without durable accounting state.
		// Otherwise deltas, never totals: counters restart at zero on every rebuild.
		if !state.Adopted {
			for _, delta := range collector.Delta(state.Name, stats) {
				email := emails[delta.PublicKey]
				if email == "" {
					email = collector.EmailFor(state.Name, delta.PublicKey)
				}
				if email == "" {
					continue
				}
				traffics = append(traffics, &xray.ClientTraffic{
					InboundId: id,
					Email:     email,
					Up:        delta.Up,
					Down:      delta.Down,
				})
			}
		}
		if desired {
			collector.SetEmails(state.Name, emails)
		}
		collector.ObserveEndpoints(state.Name, stats, now, endpointWindow)
		collector.ObserveHandshakes(state.Name, stats)
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
