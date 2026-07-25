package amneziawg

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// configDir is where awg-quick expects to find <interface>.conf, matching
// the AmneziaWG DKMS package's own layout.
const configDir = "/etc/amnezia/amneziawg"

// onlineWindow is how recent a peer's last handshake must be to count it as
// online, matching the typical WireGuard rekey interval (every 120s) plus
// margin.
const onlineWindow = 180 * time.Second

// InstanceFromInbound derives a desired Instance from an AmneziaWG inbound,
// building one peer per active client. Returns false when the inbound is not
// a usable AmneziaWG inbound (wrong protocol, unparseable settings, or no
// server block) or has no enabled peer to serve — mirroring
// mtproto.InstanceFromInbound, which skips the sidecar entirely rather than
// run it with nothing to serve.
func InstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	if ib == nil || ib.Protocol != model.AmneziaWG {
		return Instance{}, false
	}
	var parsed InboundSettings
	if err := json.Unmarshal([]byte(ib.Settings), &parsed); err != nil || parsed.Server == nil {
		return Instance{}, false
	}
	server := parsed.Server

	peers := make([]Peer, 0, len(parsed.Clients))
	for _, c := range parsed.Clients {
		if !c.Enable || c.PublicKey == "" || len(c.AllowedIPs) == 0 {
			continue
		}
		peers = append(peers, Peer{
			Email:        c.Email,
			PublicKey:    c.PublicKey,
			PresharedKey: c.PreSharedKey,
			AllowedIPs:   c.AllowedIPs,
		})
	}
	if len(peers) == 0 {
		return Instance{}, false
	}

	return Instance{
		Id:                ib.Id,
		Tag:               ib.Tag,
		InterfaceName:     interfaceNameForID(ib.Id),
		ListenPort:        ib.Port,
		PrivateKey:        server.PrivateKey,
		PublicKey:         server.PublicKey,
		Address:           []string{serverAddress(server.SubnetIP, server.SubnetCIDR)},
		MTU:               server.MTU,
		Obfuscation:       server.Obfuscation(),
		Peers:             peers,
		ExternalInterface: server.ExternalInterface,
	}, true
}

// interfaceNameForID derives the OS-level interface name for an inbound, e.g.
// "awg42".
func interfaceNameForID(id int) string {
	return fmt.Sprintf("awg%d", id)
}

// serverAddress returns the server's own tunnel address for a subnet base,
// e.g. "10.8.1.1/24" for base "10.8.1.0". The server holds the first usable
// host; a base that isn't a bare network address is used as-is.
func serverAddress(subnetIP string, cidr int) string {
	if cidr <= 0 {
		cidr = 24
	}
	if strings.HasSuffix(subnetIP, ".0") {
		return strings.TrimSuffix(subnetIP, "0") + "1/" + strconv.Itoa(cidr)
	}
	return fmt.Sprintf("%s/%d", subnetIP, cidr)
}

// structuralFingerprint changes whenever a value that requires a full
// interface bounce (awg-quick down + up) changes.
func (inst Instance) structuralFingerprint() string {
	o := inst.Obfuscation
	parts := []string{
		inst.InterfaceName,
		strconv.Itoa(inst.ListenPort),
		inst.PrivateKey,
		strings.Join(inst.Address, ","),
		strconv.Itoa(inst.MTU),
		strconv.Itoa(o.Jc), strconv.Itoa(o.Jmin), strconv.Itoa(o.Jmax),
		strconv.Itoa(o.S1), strconv.Itoa(o.S2), strconv.Itoa(o.S3), strconv.Itoa(o.S4),
		o.H1, o.H2, o.H3, o.H4, o.I1,
		inst.ExternalInterface,
	}
	return strings.Join(parts, "|")
}

// peersFingerprint identifies the reloadable peer set regardless of order, so
// a reordered clients array in the stored settings does not read as a
// change. It moves whenever a peer is added, removed, disabled, re-keyed, or
// re-addressed — all of which `awg syncconf` applies in place.
func (inst Instance) peersFingerprint() string {
	pairs := make([]string, 0, len(inst.Peers))
	for _, p := range inst.Peers {
		pairs = append(pairs, fmt.Sprintf("%s=%s;psk=%s;ips=%s", p.Email, p.PublicKey, p.PresharedKey, strings.Join(p.AllowedIPs, ",")))
	}
	slices.Sort(pairs)
	return strings.Join(pairs, "|")
}

// peerCounters is the last-seen cumulative transfer counters for one peer,
// used to compute per-poll deltas the same way mtproto tracks per-secret
// counters.
type peerCounters struct {
	rx int64
	tx int64
}

type managed struct {
	inst         Instance
	structuralFP string
	peersFP      string
	last         map[string]peerCounters // keyed by peer public key
}

// Manager owns the set of running AmneziaWG interfaces keyed by inbound id.
type Manager struct {
	mu     sync.Mutex
	ifaces map[int]*managed
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process-wide AmneziaWG manager singleton.
func GetManager() *Manager {
	managerOnce.Do(func() {
		manager = &Manager{ifaces: map[int]*managed{}}
	})
	return manager
}

// ensureAction is what ensureLocked must do to move a running interface to a
// desired instance: leave it alone, hot-reload just its peers, or fully
// bounce it.
type ensureAction int

const (
	ensureNoop ensureAction = iota
	ensureReload
	ensureRestart
)

// ensureActionFor decides how to apply a desired instance to the currently
// managed interface. A structural change (or a down interface) forces a
// restart; a peers-only change is a candidate for an in-place `syncconf`;
// identical fingerprints on an up interface need nothing.
func ensureActionFor(up bool, curStructFP, curPeersFP, newStructFP, newPeersFP string) ensureAction {
	if !up || curStructFP != newStructFP {
		return ensureRestart
	}
	if curPeersFP != newPeersFP {
		return ensureReload
	}
	return ensureNoop
}

// Ensure brings one interface to its desired state, or restarts/reloads it
// when its configuration changed. A no-op when it already matches.
func (m *Manager) Ensure(inst Instance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureLocked(inst)
}

func (m *Manager) ensureLocked(inst Instance) error {
	structFP := inst.structuralFingerprint()
	peersFP := inst.peersFingerprint()

	cur, exists := m.ifaces[inst.Id]
	action := ensureRestart
	if exists {
		action = ensureActionFor(isInterfaceUp(cur.inst.InterfaceName), cur.structuralFP, cur.peersFP, structFP, peersFP)
	}

	switch action {
	case ensureNoop:
		cur.inst = inst
		return nil
	case ensureReload:
		if err := writeConfigFile(inst); err != nil {
			return err
		}
		if err := syncConfig(inst); err != nil {
			return err
		}
	case ensureRestart:
		if exists {
			_ = interfaceDown(cur.inst.InterfaceName)
		}
		if err := writeConfigFile(inst); err != nil {
			return err
		}
		if err := interfaceUp(inst.InterfaceName); err != nil {
			return err
		}
		logger.Infof("amneziawg: started interface %s for inbound %d", inst.InterfaceName, inst.Id)
	}

	last := map[string]peerCounters{}
	if exists {
		last = cur.last
	}
	m.ifaces[inst.Id] = &managed{inst: inst, structuralFP: structFP, peersFP: peersFP, last: last}
	return nil
}

// Remove tears down and forgets the interface for an inbound id.
func (m *Manager) Remove(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.ifaces[id]; ok {
		_ = interfaceDown(cur.inst.InterfaceName)
		removeConfigFile(cur.inst.InterfaceName)
		delete(m.ifaces, id)
		logger.Infof("amneziawg: stopped interface %s for inbound %d", cur.inst.InterfaceName, id)
	}
}

// Reconcile drives the running set toward the desired instances: it tears
// down interfaces that are no longer wanted and ensures the rest. Used at
// boot and periodically to recover from crashes or an out-of-band `awg-quick
// down`.
func (m *Manager) Reconcile(desired []Instance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := make(map[int]struct{}, len(desired))
	for _, inst := range desired {
		want[inst.Id] = struct{}{}
	}
	for id, cur := range m.ifaces {
		if _, ok := want[id]; !ok {
			_ = interfaceDown(cur.inst.InterfaceName)
			removeConfigFile(cur.inst.InterfaceName)
			delete(m.ifaces, id)
			logger.Infof("amneziawg: stopped interface %s for removed inbound %d", cur.inst.InterfaceName, id)
		}
	}
	for _, inst := range desired {
		if err := m.ensureLocked(inst); err != nil {
			logger.Warningf("amneziawg: reconcile failed for inbound %d: %v", inst.Id, err)
		}
	}
}

// StopAll tears down every managed interface. Called on panel shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cur := range m.ifaces {
		_ = interfaceDown(cur.inst.InterfaceName)
		delete(m.ifaces, id)
	}
}

// HasRunning reports whether any managed interface is currently up.
func (m *Manager) HasRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, cur := range m.ifaces {
		if isInterfaceUp(cur.inst.InterfaceName) {
			return true
		}
	}
	return false
}

// Traffic is a per-peer traffic delta scraped from `awg show <iface> dump`.
// Tag is the owning inbound's tag and Email is the client the bytes belong
// to.
type Traffic struct {
	Tag   string
	Email string
	Up    int64
	Down  int64
}

// CollectTraffic polls `awg show <iface> dump` for every running interface
// and returns the per-peer byte deltas since the previous poll, plus the
// emails of peers with a handshake inside onlineWindow.
func (m *Manager) CollectTraffic() ([]Traffic, []string) {
	type snap struct {
		id   int
		inst Instance
		last map[string]peerCounters
	}
	m.mu.Lock()
	snaps := make([]snap, 0, len(m.ifaces))
	for id, cur := range m.ifaces {
		lastCopy := make(map[string]peerCounters, len(cur.last))
		maps.Copy(lastCopy, cur.last)
		snaps = append(snaps, snap{id: id, inst: cur.inst, last: lastCopy})
	}
	m.mu.Unlock()

	var out []Traffic
	var online []string
	now := time.Now()

	for _, s := range snaps {
		stats, err := getPeerStats(s.inst.InterfaceName)
		if err != nil {
			continue
		}
		emailByKey := make(map[string]string, len(s.inst.Peers))
		for _, p := range s.inst.Peers {
			emailByKey[p.PublicKey] = p.Email
		}

		newLast := make(map[string]peerCounters, len(stats))
		for _, st := range stats {
			email, ok := emailByKey[st.publicKey]
			if !ok || email == "" {
				continue
			}
			newLast[st.publicKey] = peerCounters{rx: st.rx, tx: st.tx}
			if st.latestHandshake > 0 && now.Sub(time.Unix(st.latestHandshake, 0)) < onlineWindow {
				online = append(online, email)
			}
			prev, had := s.last[st.publicKey]
			if !had {
				continue
			}
			du := st.rx - prev.rx // client upload = bytes the server received
			dd := st.tx - prev.tx // client download = bytes the server sent
			if du < 0 {
				du = 0
			}
			if dd < 0 {
				dd = 0
			}
			if du > 0 || dd > 0 {
				out = append(out, Traffic{Tag: s.inst.Tag, Email: email, Up: du, Down: dd})
			}
		}

		m.mu.Lock()
		if cur, ok := m.ifaces[s.id]; ok {
			cur.last = newLast
		}
		m.mu.Unlock()
	}
	return out, online
}

// --- config rendering ---

// generateServerConfig builds the awg-quick .conf content for an interface:
// its own [Interface] block (keys, address, obfuscation, NAT PostUp/PostDown)
// followed by one [Peer] block per client.
func generateServerConfig(inst Instance) string {
	var b strings.Builder

	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", inst.PrivateKey)
	if len(inst.Address) > 0 {
		fmt.Fprintf(&b, "Address = %s\n", strings.Join(inst.Address, ", "))
	}
	fmt.Fprintf(&b, "ListenPort = %d\n", inst.ListenPort)
	if inst.MTU > 0 {
		fmt.Fprintf(&b, "MTU = %d\n", inst.MTU)
	}
	writeObfuscation(&b, inst.Obfuscation)

	ext := inst.ExternalInterface
	if ext == "" {
		ext = detectDefaultInterface()
	}
	postUp, postDown := defaultPostUpDown(inst.InterfaceName, ext, inst.Address)
	fmt.Fprintf(&b, "PostUp = %s\n", postUp)
	fmt.Fprintf(&b, "PostDown = %s\n", postDown)

	for _, p := range inst.Peers {
		b.WriteString("\n[Peer]\n")
		if p.Email != "" {
			fmt.Fprintf(&b, "# %s\n", p.Email)
		}
		fmt.Fprintf(&b, "PublicKey = %s\n", p.PublicKey)
		if p.PresharedKey != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", p.PresharedKey)
		}
		fmt.Fprintf(&b, "AllowedIPs = %s\n", strings.Join(p.AllowedIPs, ", "))
	}

	return b.String()
}

// writeObfuscation writes the AmneziaWG obfuscation parameters that must be
// identical on both ends of a tunnel. S3/S4 and I1 are emitted only when set,
// so a plain 1.x-equivalent set (S3=S4=0, I1="") produces the classic
// generator's output; a 2.0 set adds the extra padding, header ranges and CPS
// packet.
func writeObfuscation(b *strings.Builder, o Obfuscation20) {
	fmt.Fprintf(b, "Jc = %d\n", o.Jc)
	fmt.Fprintf(b, "Jmin = %d\n", o.Jmin)
	fmt.Fprintf(b, "Jmax = %d\n", o.Jmax)
	fmt.Fprintf(b, "S1 = %d\n", o.S1)
	fmt.Fprintf(b, "S2 = %d\n", o.S2)
	if o.S3 > 0 {
		fmt.Fprintf(b, "S3 = %d\n", o.S3)
	}
	if o.S4 > 0 {
		fmt.Fprintf(b, "S4 = %d\n", o.S4)
	}
	fmt.Fprintf(b, "H1 = %s\n", hOrDefault(o.H1, "1"))
	fmt.Fprintf(b, "H2 = %s\n", hOrDefault(o.H2, "2"))
	fmt.Fprintf(b, "H3 = %s\n", hOrDefault(o.H3, "3"))
	fmt.Fprintf(b, "H4 = %s\n", hOrDefault(o.H4, "4"))
	if o.I1 != "" {
		fmt.Fprintf(b, "I1 = %s\n", o.I1)
	}
}

// hOrDefault returns def when v is blank, guarding against an empty H value
// (which would emit an invalid "H1 = " line) on legacy/partial records.
func hOrDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// defaultPostUpDown returns basic NAT + forwarding rules: MASQUERADE the
// tunnel subnet out the external interface and accept forwarded traffic in
// both directions. Per-peer port-forwarding, IPv6/NDP and RouteViaXray are a
// later phase (see project TODO).
func defaultPostUpDown(iface, ext string, addresses []string) (postUp, postDown string) {
	up := []string{
		fmt.Sprintf("iptables -A FORWARD -i %s -j ACCEPT", iface),
		fmt.Sprintf("iptables -A FORWARD -o %s -j ACCEPT", iface),
	}
	down := []string{
		fmt.Sprintf("iptables -D FORWARD -i %s -j ACCEPT", iface),
		fmt.Sprintf("iptables -D FORWARD -o %s -j ACCEPT", iface),
	}
	if subnet := firstAddress(addresses); subnet != "" && ext != "" {
		up = append([]string{fmt.Sprintf("iptables -t nat -A POSTROUTING -s %s -o %s -j MASQUERADE", subnet, ext)}, up...)
		down = append([]string{fmt.Sprintf("iptables -t nat -D POSTROUTING -s %s -o %s -j MASQUERADE", subnet, ext)}, down...)
	}
	up = append(up, "sysctl -w net.ipv4.ip_forward=1")
	return strings.Join(up, "; "), strings.Join(down, "; ")
}

// firstAddress returns the first configured interface address, used as the
// NAT source subnet for PostUp/PostDown.
func firstAddress(addresses []string) string {
	if len(addresses) == 0 {
		return ""
	}
	return addresses[0]
}

// detectDefaultInterface returns the first non-loopback, non-tunnel, UP
// interface that has a routable IPv4 address. Falls back to "eth0" only if
// nothing is found.
func detectDefaultInterface() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "eth0"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		if strings.HasPrefix(iface.Name, "awg") || strings.HasPrefix(iface.Name, "wg") ||
			strings.HasPrefix(iface.Name, "docker") || strings.HasPrefix(iface.Name, "br-") ||
			strings.HasPrefix(iface.Name, "veth") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLinkLocalUnicast() && ipNet.IP.To4() != nil {
				return iface.Name
			}
		}
	}
	return "eth0"
}

// --- process control ---

func configPath(interfaceName string) string {
	return filepath.Join(configDir, interfaceName+".conf")
}

// writeConfigFile renders and persists the .conf file awg-quick reads.
func writeConfigFile(inst Instance) error {
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("amneziawg: create config dir: %w", err)
	}
	if err := os.WriteFile(configPath(inst.InterfaceName), []byte(generateServerConfig(inst)), 0o600); err != nil {
		return fmt.Errorf("amneziawg: write config for %s: %w", inst.InterfaceName, err)
	}
	return nil
}

// removeConfigFile deletes the config file for an interface, best-effort.
func removeConfigFile(interfaceName string) {
	if err := os.Remove(configPath(interfaceName)); err != nil && !os.IsNotExist(err) {
		logger.Warningf("amneziawg: failed to remove config file for %s: %v", interfaceName, err)
	}
}

// awgCommandTimeout bounds every short-lived awg/awg-quick invocation so a
// hung command (e.g. a stuck kernel module operation) can't block the
// reconcile job indefinitely.
const awgCommandTimeout = 30 * time.Second

// interfaceUp brings an AmneziaWG interface up via awg-quick.
func interfaceUp(interfaceName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), awgCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "awg-quick", "up", configPath(interfaceName)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("awg-quick up %s failed: %s: %w", interfaceName, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// interfaceDown takes an AmneziaWG interface down via awg-quick.
func interfaceDown(interfaceName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), awgCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "awg-quick", "down", configPath(interfaceName)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("awg-quick down %s failed: %s: %w", interfaceName, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// isInterfaceUp checks whether the named AmneziaWG interface currently
// exists.
func isInterfaceUp(interfaceName string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), awgCommandTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "awg", "show", interfaceName).Run() == nil
}

// syncConfig applies a peers-only config change without dropping existing
// connections on other peers, falling back to a full restart when the live
// interface won't accept the diff (or isn't up yet).
func syncConfig(inst Instance) error {
	if !isInterfaceUp(inst.InterfaceName) {
		return interfaceUp(inst.InterfaceName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), awgCommandTimeout)
	defer cancel()
	stripped, err := exec.CommandContext(ctx, "awg-quick", "strip", configPath(inst.InterfaceName)).Output()
	if err != nil {
		logger.Warningf("amneziawg: awg-quick strip failed for %s, restarting: %v", inst.InterfaceName, err)
		return restartInterface(inst.InterfaceName)
	}

	syncCtx, syncCancel := context.WithTimeout(context.Background(), awgCommandTimeout)
	defer syncCancel()
	sync := exec.CommandContext(syncCtx, "awg", "syncconf", inst.InterfaceName, "/dev/stdin")
	sync.Stdin = bytes.NewReader(stripped)
	if out, err := sync.CombinedOutput(); err != nil {
		logger.Warningf("amneziawg: awg syncconf failed for %s, restarting: %s: %v", inst.InterfaceName, strings.TrimSpace(string(out)), err)
		return restartInterface(inst.InterfaceName)
	}
	return nil
}

// restartInterface performs a full down+up cycle.
func restartInterface(interfaceName string) error {
	_ = interfaceDown(interfaceName)
	return interfaceUp(interfaceName)
}

// peerStat is one peer's runtime stats parsed from `awg show <iface> dump`.
type peerStat struct {
	publicKey       string
	latestHandshake int64 // unix seconds
	rx              int64 // bytes received from the peer (its upload)
	tx              int64 // bytes sent to the peer (its download)
}

// getPeerStats parses `awg show <iface> dump`. The dump format is
// tab-separated: line 1 is the interface (private-key, public-key,
// listen-port, fwmark); each following line is one peer (public-key,
// preshared-key, endpoint, allowed-ips, latest-handshake, transfer-rx,
// transfer-tx, persistent-keepalive).
func getPeerStats(interfaceName string) ([]peerStat, error) {
	ctx, cancel := context.WithTimeout(context.Background(), awgCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "awg", "show", interfaceName, "dump").Output()
	if err != nil {
		return nil, fmt.Errorf("awg show %s dump failed: %w", interfaceName, err)
	}

	var stats []peerStat
	scanner := bufio.NewScanner(bytes.NewReader(out))
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) < 8 {
			continue
		}
		handshake, _ := strconv.ParseInt(fields[4], 10, 64)
		rx, _ := strconv.ParseInt(fields[5], 10, 64)
		tx, _ := strconv.ParseInt(fields[6], 10, 64)
		stats = append(stats, peerStat{publicKey: fields[0], latestHandshake: handshake, rx: rx, tx: tx})
	}
	return stats, nil
}

// IsAwgInstalled reports whether the awg and awg-quick binaries are on PATH.
func IsAwgInstalled() bool {
	_, err1 := exec.LookPath("awg")
	_, err2 := exec.LookPath("awg-quick")
	return err1 == nil && err2 == nil
}
