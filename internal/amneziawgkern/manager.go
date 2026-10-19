package amneziawgkern

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// commandTimeout bounds every awg-quick/awg/ip invocation.
const commandTimeout = 20 * time.Second

// Desired is one inbound's wanted kernel-interface state.
type Desired struct {
	Instance amneziawg.Instance
	// EgressInterface is the host NIC masquerade attaches to; blank means
	// detect it (DetectDefaultInterface).
	EgressInterface string
	// RouteThroughXray reflects a persisted setting rejected by kernel preflight.
	RouteThroughXray bool
}

// managed is one live interface plus enough state to decide, on the next
// reconcile tick, whether anything actually has to be done.
type managed struct {
	name string
	// conf is the full rendered config last written, so an unchanged tick is
	// a pure string compare with no exec at all.
	conf string
	// structFP covers everything awg syncconf cannot change on a live interface -- address, MTU, listen
	// port, key, obfuscation, NAT block.
	structFP               string
	trafficGeneration      uint64
	trafficBaselinePending bool
}

// TrafficInterfaceState identifies one managed interface generation.
type TrafficInterfaceState struct {
	Name       string
	Generation uint64
	Adopted    bool
}

// Manager owns the running kernel interfaces keyed by inbound id, mirroring
// internal/amneziawgnet.Manager's shape so callers can hold either behind the same call sites.
type Manager struct {
	mu         sync.Mutex
	ifaces     map[int]*managed
	generation uint64
	ops        managerOps
	// confDir is where awg-quick configs are written; a field rather than a
	// constant so the write path is testable off /etc.
	confDir string
}

type managerOps struct {
	linkExists           func(string) bool
	liveInterfaceMatches func(string, amneziawg.Instance) bool
	writeConf            func(string, string, string) error
	syncPeers            func(string) error
	run                  func(string, ...string) (string, error)
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process-wide kernel-AmneziaWG manager singleton.
func GetManager() *Manager {
	managerOnce.Do(func() {
		manager = &Manager{ifaces: map[int]*managed{}, confDir: ConfDir}
	})
	return manager
}

// Ensure brings the inbound's kernel interface to the state d describes.
func (m *Manager) Ensure(d Desired) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureLocked(d)
}

func (m *Manager) ensureLocked(d Desired) error {
	inst := d.Instance
	name := interfaceName(inst)

	conf, err := Render(inst, d.EgressInterface, d.RouteThroughXray)
	if err != nil {
		return err
	}
	fp := structuralFingerprint(conf)

	cur, exists := m.ifaces[inst.Id]
	if exists && cur.conf == conf && m.linkExists(name) {
		return nil
	}

	if err := m.writeConf(m.confDir, name, conf); err != nil {
		return err
	}

	// Adopt an interface that outlived the panel.
	if !exists && m.liveInterfaceMatches(name, inst) {
		if err := m.syncPeers(name); err == nil {
			m.ifaces[inst.Id] = m.newManaged(name, conf, fp, true)
			logger.Info("amneziawgkern: adopted running interface ", name, " for inbound ", inst.Id)
			return nil
		} else {
			logger.Warning("amneziawgkern: could not adopt ", name, ", rebuilding: ", err)
		}
	}

	// Reconfigure in place only when we know the interface is genuinely ours and structurally unchanged.
	if exists && cur.structFP == fp && m.linkExists(name) {
		if err := m.syncPeers(name); err != nil {
			return err
		}
		cur.conf = conf
		cur.structFP = fp
		m.ifaces[inst.Id] = cur
		logger.Debug("amneziawgkern: synced peers for ", name)
		return nil
	}

	if m.linkExists(name) {
		if out, err := m.run("awg-quick", "down", name); err != nil {
			// A stale link with no matching config makes awg-quick down fail;
			// drop the netdev directly so `up` below starts from nothing.
			logger.Warning("amneziawgkern: awg-quick down ", name, ": ", err, " ", out)
			if out, err := m.run("ip", "link", "del", name); err != nil {
				return fmt.Errorf("amneziawgkern: removing stale link %s: %w (%s)", name, err, out)
			}
		}
	}
	if out, err := m.run("awg-quick", "up", name); err != nil {
		return fmt.Errorf("amneziawgkern: awg-quick up %s: %w (%s)", name, err, out)
	}

	m.ifaces[inst.Id] = m.newManaged(name, conf, fp, false)
	logger.Info("amneziawgkern: started kernel interface ", name, " for inbound ", inst.Id)
	return nil
}

func (m *Manager) newManaged(name, conf, fp string, adopted bool) *managed {
	m.generation++
	return &managed{
		name: name, conf: conf, structFP: fp,
		trafficGeneration: m.generation, trafficBaselinePending: adopted,
	}
}

func (m *Manager) linkExists(name string) bool {
	if m.ops.linkExists != nil {
		return m.ops.linkExists(name)
	}
	return linkExists(name)
}

func (m *Manager) liveInterfaceMatches(name string, inst amneziawg.Instance) bool {
	if m.ops.liveInterfaceMatches != nil {
		return m.ops.liveInterfaceMatches(name, inst)
	}
	return liveInterfaceMatches(name, inst)
}

func (m *Manager) writeConf(dir, name, conf string) error {
	if m.ops.writeConf != nil {
		return m.ops.writeConf(dir, name, conf)
	}
	return writeConf(dir, name, conf)
}

func (m *Manager) run(name string, args ...string) (string, error) {
	if m.ops.run != nil {
		return m.ops.run(name, args...)
	}
	return run(name, args...)
}

// syncPeers applies the on-disk config's peer list to the live interface.
func (m *Manager) syncPeers(name string) error {
	if m.ops.syncPeers != nil {
		return m.ops.syncPeers(name)
	}
	stripped, err := m.run("awg-quick", "strip", ConfPathIn(m.confDir, name))
	if err != nil {
		return fmt.Errorf("amneziawgkern: awg-quick strip %s: %w (%s)", name, err, stripped)
	}
	tmp, err := os.CreateTemp("", "awgsync-*.conf")
	if err != nil {
		return fmt.Errorf("amneziawgkern: temp file for syncconf: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(stripped); err != nil {
		tmp.Close()
		return fmt.Errorf("amneziawgkern: writing syncconf input: %w", err)
	}
	tmp.Close()

	if out, err := m.run("awg", "syncconf", name, tmp.Name()); err != nil {
		return fmt.Errorf("amneziawgkern: awg syncconf %s: %w (%s)", name, err, out)
	}
	return nil
}

// Reconcile brings the whole set of kernel interfaces to what desired describes, tearing down any that
// are no longer wanted.
func (m *Manager) Reconcile(desired []Desired) {
	m.mu.Lock()
	defer m.mu.Unlock()

	want := make(map[int]struct{}, len(desired))
	for _, d := range desired {
		want[d.Instance.Id] = struct{}{}
	}
	attempted := make(map[int]struct{})
	for id, cur := range m.ifaces {
		if _, ok := want[id]; ok {
			continue
		}
		attempted[id] = struct{}{}
		if err := m.teardown(id, cur.name); err != nil {
			logger.Warning("amneziawgkern: stopping removed inbound ", id, ": ", err)
			continue
		}
		GetCollector().Forget(cur.name)
		delete(m.ifaces, id)
		logger.Info("amneziawgkern: stopped kernel interface for removed inbound ", id)
	}
	ownedIDs, err := ownedConfigIDs(m.confDir)
	if err != nil {
		logger.Warning("amneziawgkern: listing owned configs: ", err)
	} else {
		for _, id := range ownedIDs {
			if _, ok := want[id]; ok {
				continue
			}
			if _, ok := attempted[id]; ok {
				continue
			}
			name := ownedInterfaceName(id)
			if err := m.teardown(id, name); err != nil {
				logger.Warning("amneziawgkern: stopping recovered inbound ", id, ": ", err)
				continue
			}
			GetCollector().Forget(name)
			logger.Info("amneziawgkern: stopped recovered kernel interface for inbound ", id)
		}
	}
	for _, d := range desired {
		if err := m.ensureLocked(d); err != nil {
			logger.Warning("amneziawgkern: ensure inbound ", d.Instance.Id, ": ", err)
		}
	}
}

// Remove tears down one inbound's interface and forgets it.
func (m *Manager) Remove(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cur, ok := m.ifaces[id]
	name := ownedInterfaceName(id)
	if ok {
		name = cur.name
	}
	if err := m.teardown(id, name); err != nil {
		return err
	}
	if ok {
		delete(m.ifaces, id)
	}
	GetCollector().Forget(name)
	return nil
}

// StopAll drops process bookkeeping while leaving kernel interfaces running.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, cur := range m.ifaces {
		logger.Info("amneziawgkern: leaving kernel interface ", cur.name, " up across shutdown")
		delete(m.ifaces, id)
	}
}

// HasRunning reports whether any kernel interface is currently managed.
func (m *Manager) HasRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.ifaces) > 0
}

// Running reports whether this inbound has a live kernel interface.
func (m *Manager) Running(id int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.ifaces[id]
	return ok && m.linkExists(cur.name)
}

// InterfaceNameFor exposes the interface an inbound would use, for callers
// that need to read counters or report state without holding a Desired.
func (m *Manager) InterfaceNameFor(id int) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.ifaces[id]
	if !ok {
		return "", false
	}
	return cur.name, true
}

// TrafficStateFor returns the current generation and whether it was adopted.
func (m *Manager) TrafficStateFor(id int) (TrafficInterfaceState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.ifaces[id]
	if !ok || !m.linkExists(cur.name) {
		return TrafficInterfaceState{}, false
	}
	return TrafficInterfaceState{
		Name: cur.name, Generation: cur.trafficGeneration,
		Adopted: cur.trafficBaselinePending,
	}, true
}

// PrimeAdoptedTrafficBaseline consumes a matching adoption after its first dump.
func (m *Manager) PrimeAdoptedTrafficBaseline(id int, generation uint64, stats []PeerStat, collector *Collector) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.ifaces[id]
	if !ok || cur.trafficGeneration != generation || !cur.trafficBaselinePending || !m.linkExists(cur.name) {
		return false
	}
	collector.Prime(cur.name, stats)
	cur.trafficBaselinePending = false
	return true
}

// liveInterfaceMatches checks whether a live link already has the requested address and MTU.
func liveInterfaceMatches(name string, inst amneziawg.Instance) bool {
	ifi, err := net.InterfaceByName(name)
	if err != nil {
		return false
	}
	if ifi.MTU != effectiveMTU(inst) {
		return false
	}
	addrs, err := ifi.Addrs()
	if err != nil {
		return false
	}
	live := make([]string, 0, len(addrs))
	for _, a := range addrs {
		live = append(live, a.String())
	}
	return addressesSatisfied(inst.Address, live)
}

// addressesSatisfied reports whether every wanted address is present on the interface.
func addressesSatisfied(wanted, live []string) bool {
	if len(wanted) == 0 {
		return false
	}
	have := make(map[string]struct{}, len(live))
	for _, a := range live {
		have[normalizeCIDR(a)] = struct{}{}
	}
	for _, w := range wanted {
		if _, ok := have[normalizeCIDR(w)]; !ok {
			return false
		}
	}
	return true
}

// normalizeCIDR makes "10.8.1.1/24" comparable however it was spelled.
func normalizeCIDR(s string) string {
	s = strings.TrimSpace(s)
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return s
	}
	return p.String()
}

func (m *Manager) teardown(id int, name string) error {
	if name != ownedInterfaceName(id) {
		return fmt.Errorf("amneziawgkern: refusing non-canonical interface %s for inbound %d", name, id)
	}
	config, found, err := readOwnedConfig(m.confDir, id)
	if err != nil {
		return err
	}
	linkUp := m.linkExists(name)
	if !found {
		if linkUp {
			return fmt.Errorf("amneziawgkern: refusing to stop %s without an owned config", name)
		}
		return nil
	}
	if linkUp {
		out, downErr := m.run("awg-quick", "down", name)
		if m.linkExists(name) {
			if downErr != nil {
				return fmt.Errorf("amneziawgkern: awg-quick down %s: %w (%s)", name, downErr, out)
			}
			return fmt.Errorf("amneziawgkern: awg-quick down %s left the interface running", name)
		}
	}
	if strings.Contains(config, "PostDown = nft delete table ip awgnat"+fmt.Sprint(id)+"\n") {
		tables, listErr := m.run("nft", "list", "tables")
		if listErr != nil {
			return fmt.Errorf("amneziawgkern: listing nftables tables for %s: %w (%s)", name, listErr, tables)
		}
		table := "awgnat" + fmt.Sprint(id)
		for _, line := range strings.Split(tables, "\n") {
			fields := strings.Fields(line)
			if len(fields) != 3 || fields[0] != "table" || fields[1] != "ip" || fields[2] != table {
				continue
			}
			if out, deleteErr := m.run("nft", "delete", "table", "ip", table); deleteErr != nil {
				return fmt.Errorf("amneziawgkern: deleting nftables table %s: %w (%s)", table, deleteErr, out)
			}
			break
		}
	}
	if _, found, err := readOwnedConfig(m.confDir, id); err != nil {
		return err
	} else if found {
		if err := os.Remove(ConfPathIn(m.confDir, name)); err != nil {
			return fmt.Errorf("amneziawgkern: removing private config %s: %w", name, err)
		}
	}
	return nil
}

// interfaceName falls back to awg<id> so two inbounds can never collide, and
// so the name never clashes with whatever the embedded backend calls its own.
func interfaceName(inst amneziawg.Instance) string {
	if n := strings.TrimSpace(inst.InterfaceName); n != "" {
		return n
	}
	return fmt.Sprintf("awg%d", inst.Id)
}

// structuralFingerprint hashes everything above the first [Peer] section --
// exactly the properties awg syncconf cannot change on a live interface.
func structuralFingerprint(conf string) string {
	head := conf
	if i := strings.Index(conf, "\n[Peer]"); i >= 0 {
		head = conf[:i]
	}
	sum := sha256.Sum256([]byte(head))
	return hex.EncodeToString(sum[:])
}

// writeConf replaces the interface's config atomically at 0600: it holds a
// private key, and awg-quick refuses a world-readable config.
func writeConf(dir, name, conf string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("amneziawgkern: creating %s: %w", dir, err)
	}
	path := ConfPathIn(dir, name)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".awgconf-*")
	if err != nil {
		return fmt.Errorf("amneziawgkern: temp config: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(conf); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("amneziawgkern: writing config: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("amneziawgkern: chmod config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("amneziawgkern: closing config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("amneziawgkern: installing config at %s: %w", path, err)
	}
	return nil
}

func linkExists(name string) bool {
	_, err := os.Stat("/sys/class/net/" + name)
	return err == nil
}

// run executes one command with a bounded timeout and returns its combined
// output, which is where awg-quick puts the detail worth logging on failure.
func run(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
