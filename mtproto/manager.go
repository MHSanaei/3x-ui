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

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/logger"
)

// Instance is the desired runtime configuration of one mtproto inbound.
type Instance struct {
	Id     int
	Tag    string
	Listen string
	Port   int
	Secret string
}

func (inst Instance) bindTo() string {
	listen := inst.Listen
	if listen == "" {
		listen = "0.0.0.0"
	}
	return fmt.Sprintf("%s:%d", listen, inst.Port)
}

func (inst Instance) fingerprint() string {
	return fmt.Sprintf("%s|%s", inst.bindTo(), inst.Secret)
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
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return Instance{}, false
	}
	if parsed.Secret == "" {
		return Instance{}, false
	}
	return Instance{
		Id:     ib.Id,
		Tag:    ib.Tag,
		Listen: ib.Listen,
		Port:   ib.Port,
		Secret: parsed.Secret,
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
	metricsPort, err := freeLocalPort()
	if err != nil {
		return err
	}
	cfgPath := configPathForID(inst.Id)
	if err := writeConfig(cfgPath, inst.Secret, inst.bindTo(), metricsPort); err != nil {
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

func freeLocalPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func writeConfig(path, secret, bindTo string, metricsPort int) error {
	if err := os.MkdirAll(configDir(), 0o750); err != nil {
		return err
	}
	content := fmt.Sprintf("secret = %q\nbind-to = %q\n\n[stats.prometheus]\nenabled = true\nbind-to = \"127.0.0.1:%d\"\nhttp-path = \"/metrics\"\nmetric-prefix = \"mtg\"\n",
		secret, bindTo, metricsPort)
	return os.WriteFile(path, []byte(content), 0o640)
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
		for _, kv := range strings.Split(line[brace+1:end], ",") {
			eq := strings.IndexByte(kv, '=')
			if eq < 0 {
				continue
			}
			labels[strings.TrimSpace(kv[:eq])] = strings.Trim(strings.TrimSpace(kv[eq+1:]), `"`)
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
