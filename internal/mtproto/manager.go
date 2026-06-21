package mtproto

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// Instance is the desired runtime configuration of one mtproto inbound.
type Instance struct {
	Id     int
	Tag    string
	Listen string
	Port   int
	Secret string

	// Optional mtg tuning; each is omitted from the generated TOML when
	// zero-valued so mtg falls back to its own defaults.
	Debug                 bool
	ProxyProtocolListener bool
	PreferIP              string
	FrontingIP            string
	FrontingPort          int
	FrontingProxyProtocol bool

	// When RouteThroughXray is set, mtg dials Telegram through the loopback
	// SOCKS bridge the panel injects into the Xray config at XrayRoutePort, so
	// the egress obeys the core's routing rules instead of going out directly.
	RouteThroughXray bool
	XrayRoutePort    int
}

func (inst Instance) bindTo() string {
	listen := inst.Listen
	if listen == "" {
		listen = "0.0.0.0"
	}
	return fmt.Sprintf("%s:%d", listen, inst.Port)
}

// fingerprint changes whenever any value that ends up in the generated TOML
// changes, so ensureLocked restarts mtg when the operator edits a setting.
func (inst Instance) fingerprint() string {
	return strings.Join([]string{
		inst.bindTo(),
		inst.Secret,
		strconv.FormatBool(inst.Debug),
		strconv.FormatBool(inst.ProxyProtocolListener),
		inst.PreferIP,
		inst.FrontingIP,
		strconv.Itoa(inst.FrontingPort),
		strconv.FormatBool(inst.FrontingProxyProtocol),
		strconv.FormatBool(inst.RouteThroughXray),
		strconv.Itoa(inst.XrayRoutePort),
	}, "|")
}

// Traffic is a per-inbound traffic delta scraped from an mtg metrics endpoint.
type Traffic struct {
	Tag  string
	Up   int64
	Down int64
}

type managed struct {
	proc        *Process
	tag         string
	fingerprint string
	metricsPort int
	lastUp      int64
	lastDown    int64
	haveLast    bool
}

// Manager owns the set of running mtg processes keyed by inbound id.
type Manager struct {
	mu    sync.Mutex
	procs map[int]*managed
	// swept records that the one-time startup cleanup of orphaned mtg
	// processes (survivors of a previous x-ui run) has already run.
	swept bool
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process-wide mtg manager singleton.
func GetManager() *Manager {
	managerOnce.Do(func() {
		manager = &Manager{procs: map[int]*managed{}}
	})
	return manager
}

// InstanceFromInbound derives a desired Instance from an mtproto inbound,
// healing the FakeTLS secret so it always matches the configured domain.
// Returns false when the inbound is not a usable mtproto inbound.
func InstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	if ib == nil || ib.Protocol != model.MTProto {
		return Instance{}, false
	}
	settings := ib.Settings
	if healed, ok := model.HealMtprotoSecret(settings); ok {
		settings = healed
	}
	var parsed struct {
		Secret                string `json:"secret"`
		Debug                 bool   `json:"debug"`
		ProxyProtocolListener bool   `json:"proxyProtocolListener"`
		PreferIP              string `json:"preferIp"`
		DomainFronting        struct {
			IP            string `json:"ip"`
			Port          int    `json:"port"`
			ProxyProtocol bool   `json:"proxyProtocol"`
		} `json:"domainFronting"`
		RouteThroughXray bool `json:"routeThroughXray"`
		RouteXrayPort    int  `json:"routeXrayPort"`
	}
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return Instance{}, false
	}
	if parsed.Secret == "" {
		return Instance{}, false
	}
	return Instance{
		Id:                    ib.Id,
		Tag:                   ib.Tag,
		Listen:                ib.Listen,
		Port:                  ib.Port,
		Secret:                parsed.Secret,
		Debug:                 parsed.Debug,
		ProxyProtocolListener: parsed.ProxyProtocolListener,
		PreferIP:              parsed.PreferIP,
		FrontingIP:            parsed.DomainFronting.IP,
		FrontingPort:          parsed.DomainFronting.Port,
		FrontingProxyProtocol: parsed.DomainFronting.ProxyProtocol,
		RouteThroughXray:      parsed.RouteThroughXray,
		XrayRoutePort:         parsed.RouteXrayPort,
	}, true
}

// Ensure starts the mtg process for an instance, or restarts it when its
// configuration changed. A no-op when the desired process is already running.
func (m *Manager) Ensure(inst Instance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepOrphansLocked()
	return m.ensureLocked(inst)
}

// sweepOrphansLocked kills mtg processes left running by a previous x-ui run,
// exactly once per process lifetime and before any of our own mtg are started.
// Because x-ui owns every mtg process, anything alive at this point is an orphan
// that would otherwise keep holding an inbound port with a stale secret.
func (m *Manager) sweepOrphansLocked() {
	if m.swept {
		return
	}
	m.swept = true
	if n := killStrayMtgProcesses(GetBinaryPath()); n > 0 {
		logger.Warningf("mtproto: terminated %d orphaned mtg process(es) from a previous run", n)
	}
}

func (m *Manager) ensureLocked(inst Instance) error {
	fp := inst.fingerprint()
	if cur, ok := m.procs[inst.Id]; ok {
		if cur.fingerprint == fp && cur.proc.IsRunning() {
			cur.tag = inst.Tag
			return nil
		}
		cur.proc.Stop()
		delete(m.procs, inst.Id)
	}
	metricsPort, err := FreeLocalPort()
	if err != nil {
		return err
	}
	cfgPath := configPathForID(inst.Id)
	if err := writeConfig(cfgPath, inst, metricsPort); err != nil {
		return err
	}
	proc := newProcess(cfgPath, fmt.Sprintf("inbound %d", inst.Id))
	if err := proc.Start(); err != nil {
		return err
	}
	m.procs[inst.Id] = &managed{
		proc:        proc,
		tag:         inst.Tag,
		fingerprint: fp,
		metricsPort: metricsPort,
	}
	logger.Infof("mtproto: started mtg for inbound %d on %s", inst.Id, inst.bindTo())
	return nil
}

// Remove stops and forgets the mtg process for an inbound id.
func (m *Manager) Remove(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.procs[id]; ok {
		cur.proc.Stop()
		delete(m.procs, id)
		_ = os.Remove(configPathForID(id))
		logger.Infof("mtproto: stopped mtg for inbound %d", id)
	}
}

// Reconcile drives the running set toward the desired instances: it stops
// processes that are no longer wanted and (re)starts the rest. Used at boot
// and periodically to recover from crashes.
func (m *Manager) Reconcile(desired []Instance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepOrphansLocked()
	want := make(map[int]struct{}, len(desired))
	for _, inst := range desired {
		want[inst.Id] = struct{}{}
	}
	for id, cur := range m.procs {
		if _, ok := want[id]; !ok {
			cur.proc.Stop()
			delete(m.procs, id)
			_ = os.Remove(configPathForID(id))
		}
	}
	for _, inst := range desired {
		if err := m.ensureLocked(inst); err != nil {
			logger.Warningf("mtproto: reconcile failed for inbound %d: %v", inst.Id, err)
		}
	}
}

// StopAll stops every managed mtg process. Called on panel shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cur := range m.procs {
		_ = cur.proc.Stop()
		_ = os.Remove(configPathForID(id))
		delete(m.procs, id)
	}
}

// CollectTraffic scrapes each running mtg metrics endpoint and returns the
// per-inbound byte deltas since the previous scrape.
func (m *Manager) CollectTraffic() []Traffic {
	// Snapshot the state we need under the lock, then release before doing
	// network I/O so that Ensure/Reconcile/Remove are not blocked.
	type snap struct {
		id          int
		metricsPort int
		tag         string
		haveLast    bool
		lastUp      int64
		lastDown    int64
	}
	m.mu.Lock()
	snaps := make([]snap, 0, len(m.procs))
	for id, cur := range m.procs {
		if cur.proc == nil || !cur.proc.IsRunning() {
			continue
		}
		snaps = append(snaps, snap{
			id:          id,
			metricsPort: cur.metricsPort,
			tag:         cur.tag,
			haveLast:    cur.haveLast,
			lastUp:      cur.lastUp,
			lastDown:    cur.lastDown,
		})
	}
	m.mu.Unlock()

	out := make([]Traffic, 0, len(snaps))
	for _, s := range snaps {
		up, down, ok := scrapeTraffic(s.metricsPort)
		if !ok {
			continue
		}
		var du, dd int64
		if s.haveLast {
			du = up - s.lastUp
			dd = down - s.lastDown
			if du < 0 {
				du = 0
			}
			if dd < 0 {
				dd = 0
			}
		}

		// Re-acquire lock to persist the new baseline, but only if the entry
		// still exists (it may have been removed during the scrape).
		m.mu.Lock()
		if cur, ok := m.procs[s.id]; ok {
			cur.lastUp = up
			cur.lastDown = down
			cur.haveLast = true
		}
		m.mu.Unlock()

		if s.haveLast && (du > 0 || dd > 0) {
			out = append(out, Traffic{Tag: s.tag, Up: du, Down: dd})
		}
	}
	return out
}

// FreeLocalPort asks the OS for an unused loopback TCP port. It is used both
// for mtg's metrics endpoint and to allocate the per-inbound SOCKS egress
// bridge port persisted into mtproto inbound settings.
func FreeLocalPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// renderConfig builds the mtg TOML for an instance. Top-level keys must precede
// any [section] header in TOML, so the layout is: required keys, then the
// optional scalar tuning, then [domain-fronting], and finally [stats.prometheus]
// — which x-ui always emits and scrapes for traffic (see scrapeTraffic).
func renderConfig(inst Instance, metricsPort int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "secret = %q\n", inst.Secret)
	fmt.Fprintf(&b, "bind-to = %q\n", inst.bindTo())
	if inst.Debug {
		b.WriteString("debug = true\n")
	}
	if inst.ProxyProtocolListener {
		b.WriteString("proxy-protocol-listener = true\n")
	}
	if inst.PreferIP != "" {
		fmt.Fprintf(&b, "prefer-ip = %q\n", inst.PreferIP)
	}
	if inst.FrontingIP != "" || inst.FrontingPort > 0 || inst.FrontingProxyProtocol {
		b.WriteString("\n[domain-fronting]\n")
		if inst.FrontingIP != "" {
			fmt.Fprintf(&b, "ip = %q\n", inst.FrontingIP)
		}
		if inst.FrontingPort > 0 {
			fmt.Fprintf(&b, "port = %d\n", inst.FrontingPort)
		}
		if inst.FrontingProxyProtocol {
			b.WriteString("proxy-protocol = true\n")
		}
	}
	// When the inbound opts into Xray routing, mtg reaches Telegram through the
	// loopback SOCKS bridge the panel injects into the running Xray config. mtg
	// only supports SOCKS5 upstreams, which is exactly what the bridge exposes.
	if inst.RouteThroughXray && inst.XrayRoutePort > 0 {
		fmt.Fprintf(&b, "\n[network]\nproxies = [\"socks5://127.0.0.1:%d\"]\n", inst.XrayRoutePort)
	}
	fmt.Fprintf(&b, "\n[stats.prometheus]\nenabled = true\nbind-to = \"127.0.0.1:%d\"\nhttp-path = \"/metrics\"\nmetric-prefix = \"mtg\"\n", metricsPort)
	return b.String()
}

func writeConfig(path string, inst Instance, metricsPort int) error {
	if err := os.MkdirAll(configDir(), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(renderConfig(inst, metricsPort)), 0o640)
}

// scrapeTraffic reads the mtg Prometheus metrics endpoint and sums byte
// counters by direction. mtg exposes a traffic counter labelled with a
// direction; "to_telegram" is treated as upload and "to_client" as download.
// Best-effort: an unreachable endpoint or unrecognised format yields ok=false.
func scrapeTraffic(port int) (up int64, down int64, ok bool) {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", port))
	if err != nil {
		return 0, 0, false
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	found := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' || !strings.Contains(line, "traffic") {
			continue
		}
		name, labels, value, perr := parseMetricLine(line)
		if perr != nil || !strings.HasPrefix(name, "mtg") {
			continue
		}
		switch labels["direction"] {
		case "to_telegram", "egress", "up":
			up += int64(value)
		case "to_client", "ingress", "down":
			down += int64(value)
		default:
			down += int64(value)
		}
		found = true
	}
	if err := scanner.Err(); err != nil {
		logger.Debug("mtproto: metrics scan error:", err)
	}
	return up, down, found
}

func parseMetricLine(line string) (name string, labels map[string]string, value float64, err error) {
	labels = map[string]string{}
	rest := line
	if brace := strings.IndexByte(line, '{'); brace >= 0 {
		name = line[:brace]
		end := strings.IndexByte(line, '}')
		if end < brace {
			return "", nil, 0, fmt.Errorf("malformed metric line")
		}
		for kv := range strings.SplitSeq(line[brace+1:end], ",") {
			before, after, ok := strings.Cut(kv, "=")
			if !ok {
				continue
			}
			labels[strings.TrimSpace(before)] = strings.Trim(strings.TrimSpace(after), `"`)
		}
		rest = strings.TrimSpace(line[end+1:])
	} else {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return "", nil, 0, fmt.Errorf("malformed metric line")
		}
		name = fields[0]
		rest = fields[1]
	}
	valFields := strings.Fields(rest)
	if len(valFields) == 0 {
		return "", nil, 0, fmt.Errorf("missing metric value")
	}
	value, err = strconv.ParseFloat(valFields[0], 64)
	if err != nil {
		return "", nil, 0, err
	}
	return name, labels, value, nil
}
