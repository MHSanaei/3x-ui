package amneziawgnet

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// OutboundDesired pairs an amneziawg.OutboundInstance with the same
// DeviceOptions the inbound path uses -- one AWG 3.0/3.1 parameter set must
// be identical on both ends of a tunnel, so both directions carry it in the
// same shape.
type OutboundDesired struct {
	Instance amneziawg.OutboundInstance
	Options  DeviceOptions
}

// managedOutbound is one running client-mode interface: its Device, the
// rendered UAPI config used for the no-op/reconfigure decision, and enough
// of the instance to fingerprint address/MTU changes.
type managedOutbound struct {
	dev        *Device
	uapiConfig string
	structFP   string
}

// OutboundManager owns the running embedded AmneziaWG client interfaces,
// keyed by outbound tag -- the outbound mirror of Manager. It also keeps the
// SOCKS5 egress server's tag->device registry current and makes sure that
// server's listener exists, so a caller only calls Reconcile.
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

// Reconcile brings every desired outbound's device up to date and stops any
// whose tag is no longer desired -- the per-tick contract of Manager.Reconcile.
// Never returns errors: one broken outbound logs and leaves the rest alone,
// exactly like the inbound side.
func (m *OutboundManager) Reconcile(desired []OutboundDesired) {
	m.mu.Lock()
	defer m.mu.Unlock()

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

// ensureLocked decides between no-op / reconfigure-in-place / rebuild for one
// desired outbound -- same three-way split and same reasoning as the inbound
// Manager.ensureLocked (IpcSet always resets peers; address/MTU are fixed at
// netstack build time).
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
			GetEgressServer().SetStack(inst.Tag, cur.dev)
			return nil
		}
		if err := cur.dev.IpcSet(conf); err != nil {
			return fmt.Errorf("reconfigure outbound %q: %w", inst.Tag, err)
		}
		cur.uapiConfig = conf
		GetEgressServer().SetStack(inst.Tag, cur.dev)
		return nil
	}

	if exists {
		cur.dev.Close()
		delete(m.iface, inst.Tag)
	}
	dev, err := newUnconfiguredClientDevice(inst, opts)
	if err != nil {
		return err
	}
	// No forwarder to attach on the client side: traffic enters this stack
	// only through our own SOCKS5 egress server, which dials via gonet at
	// use time -- so ConfigureClient can run immediately.
	if err := dev.ConfigureClient(inst, opts); err != nil {
		return err
	}
	m.iface[inst.Tag] = &managedOutbound{dev: dev, uapiConfig: conf, structFP: fp}
	GetEgressServer().SetStack(inst.Tag, dev)
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

// StopAll tears down every managed outbound device and the egress listener.
// Called on panel shutdown alongside the inbound Manager's StopAll.
func (m *OutboundManager) StopAll() {
	m.mu.Lock()
	for tag, cur := range m.iface {
		cur.dev.Close()
		delete(m.iface, tag)
	}
	m.mu.Unlock()
	GetEgressServer().Close()
}

// HasRunning reports whether any outbound device is currently managed.
func (m *OutboundManager) HasRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.iface) > 0
}
