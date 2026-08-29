package amneziawgnet

import (
	"fmt"
	"net/netip"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// OutboundDesired pairs an instance with inbound-path DeviceOptions; AWG
// parameters must be identical on both ends of a tunnel.
type OutboundDesired struct {
	Instance amneziawg.OutboundInstance
	Options  DeviceOptions
}

// managedOutbound is one running interface plus its rendered UAPI config
// (no-op/reconfigure decision) and an address/MTU fingerprint.
type managedOutbound struct {
	dev        *Device
	uapiConfig string
	structFP   string
}

// OutboundManager owns the running AmneziaWG client interfaces keyed by tag,
// keeping the egress registry and listener current; callers just Reconcile.
type OutboundManager struct {
	mu    sync.Mutex
	iface map[string]*managedOutbound
}

var (
	outboundManagerOnce sync.Once
	outboundManager     *OutboundManager
)

// GetOutboundManager returns the process-wide outbound manager singleton.
func GetOutboundManager() *OutboundManager {
	outboundManagerOnce.Do(func() {
		outboundManager = &OutboundManager{iface: map[string]*managedOutbound{}}
	})
	return outboundManager
}

func outboundFingerprint(inst amneziawg.OutboundInstance) string {
	return fmt.Sprintf("%d|%s", inst.MTU, strings.Join(inst.Address, ","))
}

// normalizeDNSServer normalizes a configured DNS server to host:port.
func normalizeDNSServer(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if addr, err := netip.ParseAddr(s); err == nil {
		return netip.AddrPortFrom(addr, 53).String()
	}
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.String()
	}
	return s
}

// Reconcile converges devices to desired and stops removed tags; per-tick
// contract of Manager.Reconcile -- errors log, never abort the batch.
func (m *OutboundManager) Reconcile(desired []OutboundDesired) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Empty desired converges to "no tunnels": close the egress listener too,
	// so 127.0.0.1:64900 stays free on installs without AWG outbounds instead
	// of being bound lazily by every reconcile tick.
	if len(desired) == 0 {
		for tag, cur := range m.iface {
			cur.dev.Close()
			GetEgressServer().DeleteStack(tag)
			delete(m.iface, tag)
			logger.Infof("amneziawgnet: stopped embedded outbound %q", tag)
		}
		GetEgressServer().Close()
		return
	}

	if err := GetEgressServer().Listen(); err != nil {
		logger.Warningf("amneziawgnet: egress listener unavailable: %v", err)
	}

	want := make(map[string]struct{}, len(desired))
	for _, d := range desired {
		want[d.Instance.Tag] = struct{}{}
	}
	for tag, cur := range m.iface {
		if _, ok := want[tag]; ok {
			continue
		}
		cur.dev.Close()
		GetEgressServer().DeleteStack(tag)
		delete(m.iface, tag)
		logger.Infof("amneziawgnet: stopped embedded outbound %q", tag)
	}

	for _, d := range desired {
		if err := m.ensureLocked(d); err != nil {
			logger.Warningf("amneziawgnet: reconcile failed for outbound %q: %v", d.Instance.Tag, err)
		}
	}
}

// ensureLocked picks no-op / reconfigure-in-place / rebuild for one desired
// outbound (address/MTU are fixed at netstack build time).
func (m *OutboundManager) ensureLocked(d OutboundDesired) error {
	inst, opts := d.Instance, d.Options
	if opts.Logger == nil {
		opts.Logger = verboseLoggerIfEnabled(0)
	}

	fp := outboundFingerprint(inst)
	conf, err := buildClientUAPIConfig(inst, opts)
	if err != nil {
		return fmt.Errorf("render UAPI config: %w", err)
	}

	cur, exists := m.iface[inst.Tag]
	if exists && cur.structFP == fp {
		if conf == cur.uapiConfig {
			GetEgressServer().SetStack(inst.Tag, cur.dev, inst.DNS)
			return nil
		}
		if err := cur.dev.IpcSet(conf); err != nil {
			return fmt.Errorf("reconfigure outbound %q: %w", inst.Tag, err)
		}
		cur.uapiConfig = conf
		GetEgressServer().SetStack(inst.Tag, cur.dev, inst.DNS)
		return nil
	}

	if exists {
		cur.dev.Close()
		// A failed rebuild must not leave stackFor handing out a closed device.
		GetEgressServer().DeleteStack(inst.Tag)
		delete(m.iface, inst.Tag)
	}
	dev, err := newUnconfiguredClientDevice(inst, opts)
	if err != nil {
		return err
	}
	if err := dev.ConfigureClient(inst, opts); err != nil {
		return err
	}
	m.iface[inst.Tag] = &managedOutbound{dev: dev, uapiConfig: conf, structFP: fp}
	GetEgressServer().SetStack(inst.Tag, dev, inst.DNS)
	logger.Infof("amneziawgnet: started embedded outbound %s (%d peers)", inst.Tag, len(inst.Peers))
	return nil
}

// Remove tears down one outbound's device by tag.
func (m *OutboundManager) Remove(tag string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, exists := m.iface[tag]
	if !exists {
		return
	}
	cur.dev.Close()
	GetEgressServer().DeleteStack(tag)
	delete(m.iface, tag)
	logger.Infof("amneziawgnet: stopped embedded outbound %q", tag)
}

// StopAll tears down every managed outbound device and the egress listener;
// m.mu stays held across Close so a cron tick cannot re-bind mid-teardown.
func (m *OutboundManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for tag, cur := range m.iface {
		cur.dev.Close()
		GetEgressServer().DeleteStack(tag)
		delete(m.iface, tag)
	}
	GetEgressServer().Close()
}

// HasRunning reports whether any outbound device is currently managed.
func (m *OutboundManager) HasRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.iface) > 0
}
