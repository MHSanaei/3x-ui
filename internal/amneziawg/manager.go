package amneziawg

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// onlineWindow is how recent a handshake must be to count as online: the
// WireGuard rekey interval (120s) plus margin.
const onlineWindow = 180 * time.Second

// InstanceFromInbound derives a desired Instance, one peer per active client.
// ok is false for an unusable inbound or one with no peer to serve, mirroring
// mtproto.InstanceFromInbound rather than running with nothing to serve.
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
			Email:          c.Email,
			PublicKey:      c.PublicKey,
			PresharedKey:   c.PreSharedKey,
			AllowedIPs:     c.AllowedIPs,
			ForwardedPorts: c.ForwardedPorts,
		})
	}
	if len(peers) == 0 {
		return Instance{}, false
	}

	addresses := []string{serverAddress(server.SubnetIP, server.SubnetCIDR)}
	if server.IPv6Enabled {
		if v6, ok := serverAddressV6(server.IPv6Subnet); ok {
			addresses = append(addresses, v6)
		}
	}

	return Instance{
		Id:                    ib.Id,
		Tag:                   ib.Tag,
		InterfaceName:         interfaceNameForID(ib.Id),
		ListenPort:            ib.Port,
		PrivateKey:            server.PrivateKey,
		PublicKey:             server.PublicKey,
		Address:               addresses,
		MTU:                   server.MTU,
		Obfuscation:           server.Obfuscation(),
		Peers:                 peers,
		ExternalInterface:     server.ExternalInterface,
		IPv6Enabled:           server.IPv6Enabled,
		IPv6ExternalInterface: server.IPv6ExternalInterface,
		RouteThroughXray:      server.RouteThroughXray,
	}, true
}

// interfaceNameForID derives the OS-level interface name for an inbound, e.g.
// "awg42".
func interfaceNameForID(id int) string {
	return fmt.Sprintf("awg%d", id)
}

// serverAddress returns the server's own tunnel address: always the network's
// first usable host, derived via netip rather than assuming subnetIP ends in
// ".0", so a typo'd base can never collide with peer addresses (which start at
// the second host, see allocateWireguardAddress).
func serverAddress(subnetIP string, cidr int) string {
	if cidr <= 0 {
		cidr = 24
	}
	// A /32 has no host bits, so Next() would step outside the block: use a
	// single-host base exactly as given.
	prefix, err := netip.ParsePrefix(fmt.Sprintf("%s/%d", subnetIP, cidr))
	if err != nil || !prefix.Addr().Is4() || cidr >= 32 {
		return fmt.Sprintf("%s/%d", subnetIP, cidr)
	}
	host := prefix.Masked().Addr().Next()
	return fmt.Sprintf("%s/%d", host, cidr)
}

// serverAddressV6 returns the first usable host of an IPv6 prefix (e.g.
// "fd86:ea04:1115::1/64"); ok is false when subnetCIDR isn't one.
func serverAddressV6(subnetCIDR string) (addr string, ok bool) {
	prefix, err := netip.ParsePrefix(subnetCIDR)
	if err != nil || !prefix.Addr().Is6() {
		return "", false
	}
	host := prefix.Masked().Addr().Next()
	return fmt.Sprintf("%s/%d", host, prefix.Bits()), true
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
		o.H1, o.H2, o.H3, o.H4, o.I1, o.I2, o.I3, o.I4, o.I5,
		o.HeaderProtectionKey, o.ContentPaddingAddition,
		o.RekeyAfterTime, o.RekeyTimeout, o.RejectAfterTime,
		o.KeepaliveTimeout, o.MaxHandshakeAttempts,
		strconv.FormatBool(o.RandomTrailers), strconv.FormatBool(o.DisableCookies),
		inst.ExternalInterface,
		strconv.FormatBool(inst.IPv6Enabled),
		inst.IPv6ExternalInterface,
		strconv.FormatBool(inst.RouteThroughXray),
	}
	// "\n" cannot appear in any fingerprinted field (ValidateConfigValue),
	// unlike "|", which is legal in I1-I5 and would make the join ambiguous.
	return strings.Join(parts, "\n")
}

// peersFingerprint identifies the order-independent peer set `awg syncconf`
// can apply in place. Excludes ForwardedPorts on purpose: those live in
// PostUp/PostDown, so they need hostRulesFingerprint's full bounce instead.
func (inst Instance) peersFingerprint() string {
	pairs := make([]string, 0, len(inst.Peers))
	for _, p := range inst.Peers {
		pairs = append(pairs, fmt.Sprintf("%s=%s;psk=%s;ips=%s", p.Email, p.PublicKey, p.PresharedKey, strings.Join(p.AllowedIPs, ",")))
	}
	slices.Sort(pairs)
	return strings.Join(pairs, "|")
}

// hostRulesFingerprint identifies per-peer state that only takes effect through
// PostUp/PostDown (forwarded ports, TPROXY and NDP-proxy rules, all keyed on a
// peer address). `awg syncconf` never re-runs those hooks, so a change here must
// force a full bounce; each component is gated on the feature that reads it, so
// a plain instance keeps the syncconf fast path for peer add/remove/re-IP.
func (inst Instance) hostRulesFingerprint() string {
	pairs := make([]string, 0, len(inst.Peers))
	for _, p := range inst.Peers {
		v := fmt.Sprintf("%s=fwd:%s", p.Email, p.ForwardedPorts)
		if inst.RouteThroughXray || p.ForwardedPorts != "" {
			v += ";ip:" + FirstIPv4(p.AllowedIPs)
		}
		if inst.IPv6Enabled {
			v += ";ip6:" + firstIPv6(p.AllowedIPs)
		}
		pairs = append(pairs, v)
	}
	slices.Sort(pairs)
	return strings.Join(pairs, "|")
}

// peerCounters is one peer's last-seen cumulative transfer, for computing
// per-poll deltas the way mtproto tracks per-secret counters.
type peerCounters struct {
	rx int64
	tx int64
}

type managed struct {
	inst         Instance
	structuralFP string
	peersFP      string
	hostRulesFP  string
	last         map[string]peerCounters // keyed by peer public key
}

// Manager owns the set of running AmneziaWG interfaces keyed by inbound id.
type Manager struct {
	mu     sync.Mutex
	ifaces map[int]*managed
	// swept records that the one-time startup cleanup of interfaces left by a
	// previous x-ui run has already happened.
	swept bool
	// warnedOldTools rate-limits the pre-3.1 awg tools warning to one line per
	// process (see warnIfOldToolsLocked).
	warnedOldTools bool
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

// ensureAction is what ensureLocked must do to reach a desired instance: leave
// it alone, hot-reload just its peers, or fully bounce it.
type ensureAction int

const (
	ensureNoop ensureAction = iota
	ensureReload
	ensureRestart
)

// ensureActionFor decides how to apply a desired instance: a structural or
// host-rules change (or a down interface) forces a restart, a peers-only change
// is a candidate for in-place `syncconf`, identical fingerprints need nothing.
func ensureActionFor(up bool, curStructFP, curHostRulesFP, curPeersFP, newStructFP, newHostRulesFP, newPeersFP string) ensureAction {
	if !up || curStructFP != newStructFP || curHostRulesFP != newHostRulesFP {
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
	hostRulesFP := inst.hostRulesFingerprint()
	peersFP := inst.peersFingerprint()

	cur, exists := m.ifaces[inst.Id]
	action := ensureRestart
	if exists {
		action = ensureActionFor(isInterfaceUp(cur.inst.InterfaceName), cur.structuralFP, cur.hostRulesFP, cur.peersFP, structFP, hostRulesFP, peersFP)
	}
	if action != ensureNoop && inst.Obfuscation.Uses31Features() {
		m.warnIfOldToolsLocked(inst)
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
		// Kernel state, not `exists`: after an ungraceful exit the previous
		// process's interface is still up while a fresh Manager has never seen
		// it, and "ip link add" against an existing name fails forever.
		if isInterfaceUp(inst.InterfaceName) {
			_ = interfaceDown(inst.InterfaceName)
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
		last = nextTrafficBaseline(action, cur.last)
	}
	m.ifaces[inst.Id] = &managed{inst: inst, structuralFP: structFP, hostRulesFP: hostRulesFP, peersFP: peersFP, last: last}
	return nil
}

// nextTrafficBaseline keeps the per-peer counters only across a reload; a full
// down+up zeroes the kernel's own, and carrying the old baseline over that
// would make the next poll compute a negative delta and discard real traffic.
func nextTrafficBaseline(action ensureAction, prev map[string]peerCounters) map[string]peerCounters {
	if action == ensureReload {
		return prev
	}
	return map[string]peerCounters{}
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

// sweepOrphansLocked tears down interfaces left by a previous process whose
// inbound is gone from the DB entirely: m.ifaces starts empty on a fresh
// process, so nothing else would ever discover them. Runs once per process,
// and only from Reconcile -- Ensure's single-instance `want` set would
// misidentify every other still-desired interface as an orphan.
func (m *Manager) sweepOrphansLocked(want map[int]struct{}) {
	if m.swept {
		return
	}
	entries, err := os.ReadDir(configDir)
	if err != nil {
		// swept left false on purpose, so a transient error (no directory yet,
		// a filesystem hiccup) lets the next tick retry instead of giving up.
		return
	}
	m.swept = true
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	for _, ifaceName := range orphanedInterfaces(names, want) {
		if isInterfaceUp(ifaceName) {
			_ = interfaceDown(ifaceName)
			logger.Warningf("amneziawg: tore down orphaned interface %s (its inbound no longer exists)", ifaceName)
		}
		removeConfigFile(ifaceName)
	}
}

// orphanedInterfaces returns the names among confFileNames whose inbound id is
// not in want -- the pure decision sweepOrphansLocked acts on.
func orphanedInterfaces(confFileNames []string, want map[int]struct{}) []string {
	var out []string
	for _, name := range confFileNames {
		if !strings.HasSuffix(name, ".conf") {
			continue
		}
		ifaceName := strings.TrimSuffix(name, ".conf")
		id, ok := inboundIDForInterfaceName(ifaceName)
		if !ok {
			continue
		}
		if _, wanted := want[id]; wanted {
			continue
		}
		out = append(out, ifaceName)
	}
	return out
}

// inboundIDForInterfaceName reverses interfaceNameForID ("awg42" -> 42). The
// suffix must be all digits, so "awg-1.conf" can never resolve to a negative id.
func inboundIDForInterfaceName(name string) (int, bool) {
	suffix, ok := strings.CutPrefix(name, "awg")
	if !ok || suffix == "" {
		return 0, false
	}
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	id, err := strconv.Atoi(suffix)
	if err != nil {
		return 0, false
	}
	return id, true
}

// Reconcile drives the running set toward desired, tearing down what is no
// longer wanted. Recovers from crashes and out-of-band `awg-quick down`.
func (m *Manager) Reconcile(desired []Instance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := make(map[int]struct{}, len(desired))
	for _, inst := range desired {
		want[inst.Id] = struct{}{}
	}
	m.sweepOrphansLocked(want)
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

// Traffic is a per-peer delta scraped from `awg show <iface> dump`; Tag is the
// owning inbound and Email the client the bytes belong to.
type Traffic struct {
	Tag   string
	Email string
	Up    int64
	Down  int64
}

// CollectTraffic returns each running interface's per-peer byte deltas since
// the previous poll, plus the emails of peers handshaked inside onlineWindow.
func (m *Manager) CollectTraffic() ([]Traffic, []string) {
	type snap struct {
		id   int
		inst Instance
		last map[string]peerCounters
		// entry is the exact *managed snapshotted below, so the write-back can
		// detect an ensureLocked that replaced it while getPeerStats ran.
		entry *managed
	}
	m.mu.Lock()
	snaps := make([]snap, 0, len(m.ifaces))
	for id, cur := range m.ifaces {
		lastCopy := make(map[string]peerCounters, len(cur.last))
		maps.Copy(lastCopy, cur.last)
		snaps = append(snaps, snap{id: id, inst: cur.inst, last: lastCopy, entry: cur})
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
		// Only if still the exact entry snapshotted above: a restart in the
		// meantime reset last to empty, and writing newLast over that would
		// resurrect the pre-restart baseline and zero a real poll of traffic.
		if cur, ok := m.ifaces[s.id]; ok && cur == s.entry {
			cur.last = newLast
		}
		m.mu.Unlock()
	}
	return out, online
}

// --- config rendering ---

// generateServerConfig builds an interface's awg-quick .conf: its [Interface]
// block followed by one [Peer] block per client.
func generateServerConfig(inst Instance) string {
	var b strings.Builder

	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", sanitizeConfigValue(inst.PrivateKey))
	if len(inst.Address) > 0 {
		fmt.Fprintf(&b, "Address = %s\n", strings.Join(inst.Address, ", "))
	}
	fmt.Fprintf(&b, "ListenPort = %d\n", inst.ListenPort)
	if inst.MTU > 0 {
		fmt.Fprintf(&b, "MTU = %d\n", inst.MTU)
	}
	writeObfuscation(&b, inst.Obfuscation)

	ext := safeInterfaceName(inst.ExternalInterface)
	if ext == "" {
		ext = detectDefaultInterface()
	}
	postUp, postDown := defaultPostUpDown(inst, ext)
	fmt.Fprintf(&b, "PostUp = %s\n", postUp)
	fmt.Fprintf(&b, "PostDown = %s\n", postDown)

	for _, p := range inst.Peers {
		b.WriteString("\n[Peer]\n")
		if p.Email != "" {
			fmt.Fprintf(&b, "# %s\n", sanitizeConfigValue(p.Email))
		}
		fmt.Fprintf(&b, "PublicKey = %s\n", sanitizeConfigValue(p.PublicKey))
		if p.PresharedKey != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", sanitizeConfigValue(p.PresharedKey))
		}
		fmt.Fprintf(&b, "AllowedIPs = %s\n", sanitizeConfigValue(strings.Join(p.AllowedIPs, ", ")))
	}

	return b.String()
}

// sanitizeConfigValue is the render-time backstop for ValidateConfigValue: a
// row predating it (an upgrade, a restored backup, a direct DB edit) would
// otherwise reach awg-quick's parser, where a newline re-opens a section and
// smuggles in a hook it executes as root. Drops the bytes, never fails.
func sanitizeConfigValue(v string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, v)
}

// writeObfuscation emits the parameters both ends of a tunnel must share.
// Optional ones only when set, so blanking a field turns its feature off.
func writeObfuscation(b *strings.Builder, o Obfuscation31) {
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
	fmt.Fprintf(b, "H1 = %s\n", sanitizeConfigValue(hOrDefault(o.H1, "1")))
	fmt.Fprintf(b, "H2 = %s\n", sanitizeConfigValue(hOrDefault(o.H2, "2")))
	fmt.Fprintf(b, "H3 = %s\n", sanitizeConfigValue(hOrDefault(o.H3, "3")))
	fmt.Fprintf(b, "H4 = %s\n", sanitizeConfigValue(hOrDefault(o.H4, "4")))
	for i, v := range []string{o.I1, o.I2, o.I3, o.I4, o.I5} {
		if v != "" {
			fmt.Fprintf(b, "I%d = %s\n", i+1, sanitizeConfigValue(v))
		}
	}
	optional := []struct{ key, v string }{
		{"HeaderProtectionKey", o.HeaderProtectionKey},
		{"ContentPaddingAddition", o.ContentPaddingAddition},
		{"RekeyAfterTime", o.RekeyAfterTime},
		{"RekeyTimeout", o.RekeyTimeout},
		{"RejectAfterTime", o.RejectAfterTime},
		{"KeepaliveTimeout", o.KeepaliveTimeout},
		{"MaxHandshakeAttempts", o.MaxHandshakeAttempts},
	}
	for _, p := range optional {
		if p.v != "" {
			fmt.Fprintf(b, "%s = %s\n", p.key, sanitizeConfigValue(p.v))
		}
	}
	if o.RandomTrailers {
		b.WriteString("RandomTrailers = on\n")
	}
	if o.DisableCookies {
		b.WriteString("DisableCookies = on\n")
	}
}

// safeInterfaceName is the render-time backstop for ValidateInterfaceName:
// PostUp/PostDown run as root, where stripping control characters alone would
// still let a shell metacharacter through.
func safeInterfaceName(name string) string {
	if ValidateInterfaceName(name) != nil {
		return ""
	}
	return name
}

// hOrDefault returns def when v is blank, so a legacy record cannot emit an
// invalid "H1 = " line.
func hOrDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// defaultPostUpDown returns the interface's NAT and forwarding rules, plus the
// per-peer NDP-proxy entries (IPv6Enabled), DNAT rules (ForwardedPorts) and
// TPROXY rules into its own Xray bridge (RouteThroughXray, off by default).
func defaultPostUpDown(inst Instance, ext string) (postUp, postDown string) {
	iface := inst.InterfaceName
	up := []string{
		fmt.Sprintf("iptables -A FORWARD -i %s -j ACCEPT", iface),
		fmt.Sprintf("iptables -A FORWARD -o %s -j ACCEPT", iface),
	}
	down := []string{
		fmt.Sprintf("iptables -D FORWARD -i %s -j ACCEPT", iface),
		fmt.Sprintf("iptables -D FORWARD -o %s -j ACCEPT", iface),
	}
	if subnet := firstAddress(inst.Address); subnet != "" && ext != "" {
		up = append([]string{fmt.Sprintf("iptables -t nat -A POSTROUTING -s %s -o %s -j MASQUERADE", subnet, ext)}, up...)
		down = append([]string{fmt.Sprintf("iptables -t nat -D POSTROUTING -s %s -o %s -j MASQUERADE", subnet, ext)}, down...)
	}

	if inst.IPv6Enabled {
		ext6 := safeInterfaceName(inst.IPv6ExternalInterface)
		if ext6 == "" {
			ext6 = ext
		}
		up = append(up,
			fmt.Sprintf("ip6tables -A FORWARD -i %s -j ACCEPT", iface),
			fmt.Sprintf("ip6tables -A FORWARD -o %s -j ACCEPT", iface),
			fmt.Sprintf("ip6tables -A FORWARD -i %s -o %s -j ACCEPT", ext6, iface),
			"sysctl -w net.ipv6.conf.all.forwarding=1",
			fmt.Sprintf("sysctl -w net.ipv6.conf.%s.proxy_ndp=1", ext6),
		)
		down = append(down,
			fmt.Sprintf("ip6tables -D FORWARD -i %s -j ACCEPT", iface),
			fmt.Sprintf("ip6tables -D FORWARD -o %s -j ACCEPT", iface),
			fmt.Sprintf("ip6tables -D FORWARD -i %s -o %s -j ACCEPT", ext6, iface),
		)
		for _, p := range inst.Peers {
			ip6 := firstIPv6(p.AllowedIPs)
			if ip6 == "" {
				continue
			}
			up = append(up, fmt.Sprintf("ip -6 neigh add proxy %s dev %s", ip6, ext6))
			down = append(down, fmt.Sprintf("ip -6 neigh del proxy %s dev %s", ip6, ext6))
		}
	}

	for _, p := range inst.Peers {
		if p.ForwardedPorts == "" {
			continue
		}
		clientIP := FirstIPv4(p.AllowedIPs)
		if clientIP == "" {
			continue
		}
		up = append(up, portForwardLines("-A", ext, iface, clientIP, p.Email, p.ForwardedPorts)...)
		down = append(down, portForwardLines("-D", ext, iface, clientIP, p.Email, p.ForwardedPorts)...)
	}

	// A bridge Xray itself will refuse to open (see EgressPortForInbound) must
	// not get TPROXY rules either, or every peer's traffic is redirected into
	// a socket that never exists.
	if egressPort, portOK := EgressPortForInbound(inst.Id); inst.RouteThroughXray && portOK {
		anyPeerTproxied := false
		for _, p := range inst.Peers {
			clientIP := FirstIPv4(p.AllowedIPs)
			if clientIP == "" {
				continue
			}
			up = append(up, routeEgressLines("-A", iface, clientIP, p.Email, egressPort)...)
			down = append(down, routeEgressLines("-D", iface, clientIP, p.Email, egressPort)...)
			anyPeerTproxied = true
		}
		if anyPeerTproxied {
			// Both rules below are system-wide, idempotent and never torn down: a
			// second instance must find them in place, not race to remove what the
			// first still needs. The policy route is what lets TPROXY deliver a
			// packet whose destination is never one of this host's addresses; the
			// INPUT accept is what stops a default-deny firewall's "is this
			// destination local" check (UFW's ufw-not-local) from silently
			// dropping it before Xray's socket ever sees a byte.
			up = append(up,
				// grep -c, not -q: -q exits early, so "ip rule list" takes SIGPIPE
				// and pipefail reports 141 even on a match, adding a duplicate rule.
				fmt.Sprintf("ip rule list | grep -c 'fwmark %#x lookup %d' >/dev/null || ip rule add fwmark %#x lookup %d", EgressFwmark, EgressTable, EgressFwmark, EgressTable),
				fmt.Sprintf("ip route replace local 0.0.0.0/0 dev lo table %d", EgressTable),
				fmt.Sprintf("iptables -C INPUT -m mark --mark %#x -j ACCEPT 2>/dev/null || iptables -I INPUT 1 -m mark --mark %#x -j ACCEPT", EgressFwmark, EgressFwmark),
			)
		}
	}

	up = append(up, "sysctl -w net.ipv4.ip_forward=1")
	return strings.Join(up, "; "), strings.Join(appendOrTrue(down), "; ")
}

// appendOrTrue makes PostDown best-effort: awg-quick runs the joined hooks under
// `set -e`, so one failed "-D" (a ufw reload flushed the table already) would
// skip every later delete and leak a rule set per bounce. PostUp is left strict.
func appendOrTrue(cmds []string) []string {
	out := make([]string, len(cmds))
	for i, c := range cmds {
		out[i] = c + " || true"
	}
	return out
}

// firstAddress returns the NAT source subnet for PostUp/PostDown.
func firstAddress(addresses []string) string {
	if len(addresses) == 0 {
		return ""
	}
	return addresses[0]
}

// firstIPv6 returns the first IPv6 address among allowedIPs, mask stripped --
// one NDP proxy PostUp/PostDown entry per peer is built from it.
func firstIPv6(allowedIPs []string) string {
	for _, a := range allowedIPs {
		if prefix, err := netip.ParsePrefix(a); err == nil {
			if prefix.Addr().Is6() {
				return prefix.Addr().String()
			}
			continue
		}
		if addr, err := netip.ParseAddr(a); err == nil && addr.Is6() {
			return addr.String()
		}
	}
	return ""
}

// FirstIPv4 returns the first IPv4 address among allowedIPs, mask stripped.
// Exported so internal/web/service derives a peer's tunnel address identically.
func FirstIPv4(allowedIPs []string) string {
	for _, a := range allowedIPs {
		if prefix, err := netip.ParsePrefix(a); err == nil {
			if prefix.Addr().Is4() {
				return prefix.Addr().String()
			}
			continue
		}
		if addr, err := netip.ParseAddr(a); err == nil && addr.Is4() {
			return addr.String()
		}
	}
	return ""
}

// detectDefaultInterface returns the first non-loopback, non-tunnel, UP
// interface with a routable IPv4 address, falling back to "eth0".
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

// awgCommandTimeout bounds every awg/awg-quick call, so a stuck kernel module
// operation can't block the reconcile job indefinitely.
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

// syncConfig applies a peers-only change without dropping other peers'
// connections, falling back to a restart when the interface won't take it.
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

// getPeerStats runs `awg show <iface> dump`: line 1 is the interface, each
// later line one peer (key, psk, endpoint, ips, handshake, rx, tx, keepalive).
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

var (
	toolsVersionOnce sync.Once
	toolsMajor       int
	toolsMinor       int
	toolsVersionOK   bool
)

// awgToolsVersion parses the installed amneziawg-tools version once per
// process; ok is false when awg is missing or its output has no version.
func awgToolsVersion() (major, minor int, ok bool) {
	toolsVersionOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), awgCommandTimeout)
		defer cancel()
		out, err := exec.CommandContext(ctx, "awg", "--version").CombinedOutput()
		if err != nil {
			return
		}
		toolsMajor, toolsMinor, toolsVersionOK = parseAwgVersion(string(out))
	})
	return toolsMajor, toolsMinor, toolsVersionOK
}

var awgVersionPattern = regexp.MustCompile(`v?(\d+)\.(\d+)`)

// parseAwgVersion extracts major.minor from `awg --version` output, e.g.
// "wireguard-tools v3.1.20260812 - https://...".
func parseAwgVersion(out string) (major, minor int, ok bool) {
	m := awgVersionPattern.FindStringSubmatch(out)
	if m == nil {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(m[1])
	minor, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return major, minor, true
}

// Uses31Features reports whether the set carries an AmneziaWG 3.x-only
// parameter that pre-3.1 awg tools reject as an unknown config key.
func (o Obfuscation31) Uses31Features() bool {
	return o.HeaderProtectionKey != "" || o.ContentPaddingAddition != "" ||
		o.RekeyAfterTime != "" || o.RekeyTimeout != "" || o.RejectAfterTime != "" ||
		o.KeepaliveTimeout != "" || o.MaxHandshakeAttempts != "" ||
		o.RandomTrailers || o.DisableCookies
}

// warnIfOldToolsLocked logs once per process that the installed awg tools
// predate the 3.1 parameters in use; the apply still proceeds, since awg-quick
// itself reports the definitive failure. Caller must hold m.mu.
func (m *Manager) warnIfOldToolsLocked(inst Instance) {
	if m.warnedOldTools {
		return
	}
	major, minor, ok := awgToolsVersion()
	if !ok || major > 3 || (major == 3 && minor >= 1) {
		return
	}
	m.warnedOldTools = true
	logger.Warningf("amneziawg: installed awg tools are v%d.%d but inbound %d uses AmneziaWG 3.1 parameters (HeaderProtectionKey etc.); awg-quick will likely reject the config — upgrade amneziawg-tools to v3.1.20260812+ and the kernel module/amneziawg-go to v3.1.20260814+", major, minor, inst.Id)
}
