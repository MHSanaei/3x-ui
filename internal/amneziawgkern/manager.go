package amneziawgkern

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// commandTimeout bounds every awg/ip/nft/iptables invocation.
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

// managed is one live interface and the spec it was last brought to.
type managed struct {
	spec                   spec
	trafficGeneration      uint64
	trafficBaselinePending bool
}

// TrafficInterfaceState identifies one managed interface generation.
type TrafficInterfaceState struct {
	Name       string
	Generation uint64
	Adopted    bool
}

// host is the part of the OS the manager drives; tests substitute a fake.
type host interface {
	linkExists(name string) bool
	linkMatches(name string, addresses []string, mtu int) bool
	run(stdin, name string, args ...string) (string, error)
	hasCommand(name string) bool
	moduleLoaded() bool
	enableIPv4Forwarding() error
}

// Manager owns the running kernel interfaces keyed by inbound id, mirroring
// internal/amneziawgnet.Manager's shape so callers can hold either behind the same call sites.
type Manager struct {
	mu         sync.Mutex
	ifaces     map[int]*managed
	generation uint64
	host       host
	confDir    string
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process-wide kernel-AmneziaWG manager singleton.
func GetManager() *Manager {
	managerOnce.Do(func() {
		manager = newManager(osHost{}, defaultConfDir())
	})
	return manager
}

func newManager(h host, confDir string) *Manager {
	return &Manager{ifaces: map[int]*managed{}, host: h, confDir: confDir}
}

// Ensure brings the inbound's kernel interface to the state d describes.
func (m *Manager) Ensure(d Desired) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureLocked(d)
}

func (m *Manager) ensureLocked(d Desired) error {
	s, err := buildSpec(d)
	if err != nil {
		return err
	}
	id := d.Instance.Id
	cur, isManaged := m.ifaces[id]
	linkUp := m.host.linkExists(s.name)

	owned, found, err := readOwnedConfig(m.confDir, id)
	if err != nil {
		return err
	}
	if linkUp && !isManaged {
		if !found {
			return fmt.Errorf("amneziawgkern: refusing occupied interface %s without an owned config", s.name)
		}
		if err := m.verifyLiveInterface(s.name, owned); err != nil {
			return err
		}
	}

	if linkUp && isManaged && cur.spec.structFP == s.structFP {
		changed := cur.spec.conf != s.conf || cur.spec.nat != s.nat || cur.spec.forward != s.forward
		if err := m.reconfigure(cur.spec, s); err != nil {
			return err
		}
		cur.spec = s
		if changed {
			logger.Debug("amneziawgkern: synced ", s.name)
		}
		return nil
	}
	if linkUp && !isManaged && interfaceSection(owned) == interfaceSection(s.conf) &&
		m.host.linkMatches(s.name, s.addresses, s.mtu) {
		err := m.adopt(s)
		if err == nil {
			m.ifaces[id] = m.newManaged(s, true)
			logger.Info("amneziawgkern: adopted running interface ", s.name, " for inbound ", id)
			return nil
		}
		logger.Warning("amneziawgkern: could not adopt ", s.name, ", rebuilding: ", err)
	}

	// The old config stays on disk until its link is gone, so an interrupted
	// rebuild still has the key that proves the leftover link is ours.
	if linkUp {
		if err := m.deleteLink(s.name); err != nil {
			return err
		}
	}
	if err := writeConf(m.confDir, s.name, s.conf); err != nil {
		return err
	}
	if err := m.createLink(s); err != nil {
		if m.host.linkExists(s.name) {
			if out, delErr := m.host.run("", "ip", "link", "del", "dev", s.name); delErr != nil {
				logger.Warning("amneziawgkern: cleaning up half-built ", s.name, ": ", delErr, " ", out)
			}
		}
		return err
	}
	// Without a managed entry the rules a previous process left are unknown,
	// so the sentinel forces a resync.
	prevForward := forwardRules{tunnel: s.name}
	if isManaged {
		prevForward = cur.spec.forward
	}
	if err := m.applyEgress(s, prevForward, true); err != nil {
		return err
	}
	m.ifaces[id] = m.newManaged(s, false)
	logger.Info("amneziawgkern: started kernel interface ", s.name, " for inbound ", id)
	return nil
}

func (m *Manager) newManaged(s spec, adopted bool) *managed {
	m.generation++
	return &managed{spec: s, trafficGeneration: m.generation, trafficBaselinePending: adopted}
}

// reconfigure applies peer and egress changes to a live, structurally
// unchanged interface without dropping its sessions.
func (m *Manager) reconfigure(prev, s spec) error {
	if prev.conf != s.conf {
		if err := writeConf(m.confDir, s.name, s.conf); err != nil {
			return err
		}
		if out, err := m.host.run("", "awg", "syncconf", s.name, ConfPathIn(m.confDir, s.name)); err != nil {
			return fmt.Errorf("amneziawgkern: awg syncconf %s: %w (%s)", s.name, err, out)
		}
	}
	return m.applyEgress(s, prev.forward, prev.nat != s.nat)
}

// adopt takes over an interface that outlived the panel; its egress state is
// unknown, so NAT and forwarding are re-applied unconditionally.
func (m *Manager) adopt(s spec) error {
	if err := writeConf(m.confDir, s.name, s.conf); err != nil {
		return err
	}
	if out, err := m.host.run("", "awg", "syncconf", s.name, ConfPathIn(m.confDir, s.name)); err != nil {
		return fmt.Errorf("amneziawgkern: awg syncconf %s: %w (%s)", s.name, err, out)
	}
	return m.applyEgress(s, forwardRules{tunnel: s.name}, true)
}

func (m *Manager) createLink(s spec) error {
	if !m.host.moduleLoaded() {
		if out, err := m.host.run("", "modprobe", "amneziawg"); err != nil {
			return fmt.Errorf("amneziawgkern: modprobe amneziawg: %w (%s)", err, out)
		}
	}
	steps := [][]string{
		{"ip", "link", "add", "dev", s.name, "type", "amneziawg"},
		{"awg", "setconf", s.name, ConfPathIn(m.confDir, s.name)},
	}
	for _, a := range s.addresses {
		steps = append(steps, []string{"ip", "-4", "address", "add", a, "dev", s.name})
	}
	steps = append(steps, []string{"ip", "link", "set", "dev", s.name, "mtu", fmt.Sprint(s.mtu), "up"})
	for _, step := range steps {
		if out, err := m.host.run("", step[0], step[1:]...); err != nil {
			return fmt.Errorf("amneziawgkern: %s: %w (%s)", strings.Join(step, " "), err, out)
		}
	}
	return nil
}

func (m *Manager) deleteLink(name string) error {
	out, err := m.host.run("", "ip", "link", "del", "dev", name)
	if m.host.linkExists(name) {
		if err != nil {
			return fmt.Errorf("amneziawgkern: ip link del %s: %w (%s)", name, err, out)
		}
		return fmt.Errorf("amneziawgkern: ip link del %s left the interface running", name)
	}
	return nil
}

// applyEgress converges NAT (when natChanged) and the FORWARD accepts.
func (m *Manager) applyEgress(s spec, prevForward forwardRules, natChanged bool) error {
	if natChanged {
		if out, err := m.host.run(s.nat, "nft", "-f", "-"); err != nil {
			return fmt.Errorf("amneziawgkern: applying NAT for %s: %w (%s)", s.name, err, out)
		}
	}
	if s.forward.egress != "" {
		if err := m.host.enableIPv4Forwarding(); err != nil {
			return fmt.Errorf("amneziawgkern: enabling IPv4 forwarding: %w", err)
		}
	}
	if prevForward != s.forward {
		m.syncForwardRules(s.name, s.forward)
	}
	return nil
}

// syncForwardRules is best effort: with a FORWARD policy of ACCEPT the rules
// are redundant, so a host without iptables still forwards.
func (m *Manager) syncForwardRules(name string, f forwardRules) {
	if !m.host.hasCommand("iptables") {
		return
	}
	m.removeForwardRules(name)
	for _, rule := range f.rules() {
		if out, err := m.host.run("", "iptables", append([]string{"-w", "5", "-I", "FORWARD", "1"}, rule...)...); err != nil {
			logger.Warning("amneziawgkern: adding FORWARD accept for ", name, ": ", err, " ", out)
		}
	}
}

// removeForwardRules deletes every FORWARD rule tagged with name's comment,
// whatever egress it was written for.
func (m *Manager) removeForwardRules(name string) {
	if !m.host.hasCommand("iptables") {
		return
	}
	listing, err := m.host.run("", "iptables", "-w", "5", "-S", "FORWARD")
	if err != nil {
		logger.Warning("amneziawgkern: listing FORWARD rules: ", err, " ", listing)
		return
	}
	tag := forwardComment(name)
	for _, line := range strings.Split(listing, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "-A" {
			continue
		}
		// -S quotes the comment; passed back verbatim the quotes would be part of it.
		for i := range fields {
			fields[i] = strings.Trim(fields[i], `"`)
		}
		if !hasCommentTag(fields, tag) {
			continue
		}
		fields[0] = "-D"
		if out, err := m.host.run("", "iptables", append([]string{"-w", "5"}, fields...)...); err != nil {
			logger.Warning("amneziawgkern: removing FORWARD accept for ", name, ": ", err, " ", out)
		}
	}
}

func forwardComment(name string) string { return "3x-ui:" + name }

func hasCommentTag(fields []string, tag string) bool {
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "--comment" && fields[i+1] == tag {
			return true
		}
	}
	return false
}

func (f forwardRules) rules() [][]string {
	if f.egress == "" {
		return nil
	}
	comment := []string{"-m", "comment", "--comment", forwardComment(f.tunnel), "-j", "ACCEPT"}
	return [][]string{
		append([]string{"-i", f.tunnel, "-o", f.egress}, comment...),
		append([]string{"-i", f.egress, "-o", f.tunnel, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED"}, comment...),
	}
}

// verifyLiveInterface refuses a live link whose key is not the owned config's.
// A keyless link is one a crashed create left behind.
func (m *Manager) verifyLiveInterface(name, config string) error {
	want, err := configInterfacePublicKey(config)
	if err != nil {
		return fmt.Errorf("amneziawgkern: refusing to manage %s: %w", name, err)
	}
	got, err := m.host.run("", "awg", "show", name, "public-key")
	if err != nil {
		return fmt.Errorf("amneziawgkern: refusing to manage %s: cannot verify live AmneziaWG interface: %w", name, err)
	}
	if got = strings.TrimSpace(got); got != want && got != "(none)" && got != "" {
		return fmt.Errorf("amneziawgkern: refusing to manage %s: live interface key does not match its owned config", name)
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
	for id := range m.ifaces {
		if _, ok := want[id]; ok {
			continue
		}
		attempted[id] = struct{}{}
		if err := m.teardown(id); err != nil {
			logger.Warning("amneziawgkern: stopping removed inbound ", id, ": ", err)
			continue
		}
		logger.Info("amneziawgkern: stopped kernel interface for removed inbound ", id)
	}
	ownedIDs, err := ownedConfigIDs(m.confDir)
	if err != nil {
		logger.Warning("amneziawgkern: listing owned configs: ", err)
	}
	for _, id := range ownedIDs {
		if _, ok := want[id]; ok {
			continue
		}
		if _, ok := attempted[id]; ok {
			continue
		}
		if err := m.teardown(id); err != nil {
			logger.Warning("amneziawgkern: stopping recovered inbound ", id, ": ", err)
			continue
		}
		logger.Info("amneziawgkern: stopped recovered kernel interface for inbound ", id)
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
	return m.teardown(id)
}

// StopAll tears every managed interface down, like the embedded datapath does
// on shutdown: a stopped panel enforces no quota, so nothing may keep forwarding.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id := range m.ifaces {
		if err := m.teardown(id); err != nil {
			logger.Warning("amneziawgkern: stopping inbound ", id, " on shutdown: ", err)
		}
	}
}

// ManagedIDs lists the inbounds with a managed interface, in id order.
func (m *Manager) ManagedIDs() []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]int, 0, len(m.ifaces))
	for id := range m.ifaces {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
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
	name := ownedInterfaceName(id)
	if !m.host.linkExists(name) {
		return false
	}
	if _, ok := m.ifaces[id]; ok {
		return true
	}
	_, found, err := readOwnedConfig(m.confDir, id)
	return err == nil && found
}

// TrafficStateFor returns the current generation and whether it was adopted.
func (m *Manager) TrafficStateFor(id int) (TrafficInterfaceState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.ifaces[id]
	if !ok || !m.host.linkExists(cur.spec.name) {
		return TrafficInterfaceState{}, false
	}
	return TrafficInterfaceState{
		Name: cur.spec.name, Generation: cur.trafficGeneration,
		Adopted: cur.trafficBaselinePending,
	}, true
}

// PrimeAdoptedTrafficBaseline consumes a matching adoption after its first dump.
func (m *Manager) PrimeAdoptedTrafficBaseline(id int, generation uint64, stats []PeerStat, collector *Collector) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.ifaces[id]
	if !ok || cur.trafficGeneration != generation || !cur.trafficBaselinePending || !m.host.linkExists(cur.spec.name) {
		return false
	}
	collector.Prime(cur.spec.name, stats)
	cur.trafficBaselinePending = false
	return true
}

// teardown removes the link, its NAT table, its FORWARD accepts and its
// private config, refusing a live link the panel cannot prove is its own.
func (m *Manager) teardown(id int) error {
	name := ownedInterfaceName(id)
	owned, found, err := readOwnedConfig(m.confDir, id)
	if err != nil {
		return err
	}
	_, isManaged := m.ifaces[id]
	if !found && !isManaged {
		if m.host.linkExists(name) {
			return fmt.Errorf("amneziawgkern: refusing to stop %s without an owned config", name)
		}
		return nil
	}
	if m.host.linkExists(name) {
		if found {
			if err := m.verifyLiveInterface(name, owned); err != nil {
				return err
			}
		}
		if err := m.deleteLink(name); err != nil {
			return err
		}
	}
	if out, err := m.host.run(natTransaction(id, ""), "nft", "-f", "-"); err != nil {
		return fmt.Errorf("amneziawgkern: removing NAT for %s: %w (%s)", name, err, out)
	}
	m.removeForwardRules(name)
	if found {
		if err := os.Remove(ConfPathIn(m.confDir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("amneziawgkern: removing private config %s: %w", name, err)
		}
	}
	delete(m.ifaces, id)
	GetCollector().Forget(name)
	return nil
}

// interfaceSection is the config above its first [Peer] -- what adoption
// requires to match before a live link's peers may be replaced.
func interfaceSection(conf string) string {
	if i := strings.Index(conf, "\n[Peer]"); i >= 0 {
		return conf[:i]
	}
	return conf
}

// writeConf replaces the interface's config atomically at 0600: it holds the
// inbound's private key.
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

// osHost is the real Linux host.
type osHost struct{}

func (osHost) linkExists(name string) bool {
	_, err := os.Stat("/sys/class/net/" + name)
	return err == nil
}

// linkMatches checks whether a live link already has the requested addresses and MTU.
func (osHost) linkMatches(name string, addresses []string, mtu int) bool {
	ifi, err := net.InterfaceByName(name)
	if err != nil || ifi.MTU != mtu {
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
	return addressesSatisfied(addresses, live)
}

func (osHost) run(stdin, name string, args ...string) (string, error) {
	return run(stdin, name, args...)
}

func (osHost) hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (osHost) moduleLoaded() bool {
	_, err := os.Stat("/sys/module/amneziawg")
	return err == nil
}

func (osHost) enableIPv4Forwarding() error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0o644)
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

// run executes one command, no shell involved, with a bounded timeout and
// returns its combined output for the error message.
func run(stdin, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
