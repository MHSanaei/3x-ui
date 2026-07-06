package mtproto

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// SecretEntry is one named FakeTLS secret served by an mtg-multi process. Name is
// the client email, used both as the [secrets] key and as the per-user key in the
// /stats API so traffic can be attributed back to the client.
type SecretEntry struct {
	Name   string
	Secret string
}

// Instance is the desired runtime configuration of one mtproto inbound. A single
// mtg-multi process serves every active client's secret through the [secrets]
// section, so one inbound maps to one process with many named secrets.
type Instance struct {
	Id      int
	Tag     string
	Listen  string
	Port    int
	Secrets []SecretEntry

	// Optional mtg tuning; each is omitted from the generated TOML when
	// zero-valued so mtg falls back to its own defaults.
	Debug                 bool
	ProxyProtocolListener bool
	PreferIP              string
	FrontingIP            string
	FrontingPort          int
	FrontingProxyProtocol bool

	// ThrottleMaxConnections caps concurrent connections across all users with a
	// fair-share algorithm; zero disables throttling.
	ThrottleMaxConnections int

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

// structuralFingerprint changes whenever a value outside the [secrets] section
// of the generated TOML changes. Such a change can only be applied by
// restarting mtg, unlike a secrets-only change, which a reload-capable mtg can
// absorb in place.
func (inst Instance) structuralFingerprint() string {
	parts := []string{
		inst.bindTo(),
		strconv.FormatBool(inst.Debug),
		strconv.FormatBool(inst.ProxyProtocolListener),
		inst.PreferIP,
		inst.FrontingIP,
		strconv.Itoa(inst.FrontingPort),
		strconv.FormatBool(inst.FrontingProxyProtocol),
		strconv.Itoa(inst.ThrottleMaxConnections),
		strconv.FormatBool(inst.RouteThroughXray),
		strconv.Itoa(inst.XrayRoutePort),
	}
	return strings.Join(parts, "|")
}

// secretsFingerprint identifies the served secret set regardless of order, so
// a reordered clients array in the stored settings does not read as a config
// change. It moves whenever a client is added, removed, disabled, or re-keyed.
func (inst Instance) secretsFingerprint() string {
	pairs := make([]string, 0, len(inst.Secrets))
	for _, e := range inst.Secrets {
		pairs = append(pairs, e.Name+"="+e.Secret)
	}
	slices.Sort(pairs)
	return strings.Join(pairs, "|")
}

// Traffic is a per-client traffic delta scraped from an mtg /stats endpoint. Tag
// is the owning inbound's tag and Email is the client the bytes belong to.
type Traffic struct {
	Tag   string
	Email string
	Up    int64
	Down  int64
}

type clientCounters struct {
	up   int64
	down int64
}

type managed struct {
	proc         *Process
	tag          string
	structuralFP string
	secretsFP    string
	apiPort      int
	last         map[string]clientCounters
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
// building one named secret per active client. Secrets are healed on save (see
// normalizeMtprotoSecret) and by the migration, so they are read as-is here to
// keep the fingerprint stable across reconciles. Returns false when the inbound
// is not a usable mtproto inbound or has no active client secret to serve.
func InstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	if ib == nil || ib.Protocol != model.MTProto {
		return Instance{}, false
	}
	settings := ib.Settings
	var parsed struct {
		ProxyProtocolListener bool `json:"proxyProtocolListener"`
		Debug                 bool `json:"debug"`
		DomainFronting        struct {
			IP            string `json:"ip"`
			Port          int    `json:"port"`
			ProxyProtocol bool   `json:"proxyProtocol"`
		} `json:"domainFronting"`
		PreferIP               string `json:"preferIp"`
		ThrottleMaxConnections int    `json:"throttleMaxConnections"`
		RouteThroughXray       bool   `json:"routeThroughXray"`
		RouteXrayPort          int    `json:"routeXrayPort"`
		Clients                []struct {
			Email  string `json:"email"`
			Secret string `json:"secret"`
			Enable bool   `json:"enable"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return Instance{}, false
	}
	secrets := make([]SecretEntry, 0, len(parsed.Clients))
	for _, c := range parsed.Clients {
		if !c.Enable || c.Secret == "" || c.Email == "" {
			continue
		}
		secrets = append(secrets, SecretEntry{Name: c.Email, Secret: c.Secret})
	}
	if len(secrets) == 0 {
		return Instance{}, false
	}
	return Instance{
		Id:                     ib.Id,
		Tag:                    ib.Tag,
		Listen:                 ib.Listen,
		Port:                   ib.Port,
		Secrets:                secrets,
		Debug:                  parsed.Debug,
		ProxyProtocolListener:  parsed.ProxyProtocolListener,
		PreferIP:               parsed.PreferIP,
		FrontingIP:             parsed.DomainFronting.IP,
		FrontingPort:           parsed.DomainFronting.Port,
		FrontingProxyProtocol:  parsed.DomainFronting.ProxyProtocol,
		ThrottleMaxConnections: parsed.ThrottleMaxConnections,
		RouteThroughXray:       parsed.RouteThroughXray,
		XrayRoutePort:          parsed.RouteXrayPort,
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

// ensureAction is what ensureLocked must do to move a running mtg process to a
// desired instance: leave it alone, hot-reload only its secrets, or fully
// restart it.
type ensureAction int

const (
	ensureNoop ensureAction = iota
	ensureReload
	ensureRestart
)

// ensureActionFor decides how to apply a desired instance to the currently
// managed process. A structural change (or a dead process) forces a restart; a
// secrets-only change is a candidate for an in-place reload; identical
// fingerprints on a live process need nothing.
func ensureActionFor(running bool, curStructFP, curSecretsFP, newStructFP, newSecretsFP string) ensureAction {
	if !running || curStructFP != newStructFP {
		return ensureRestart
	}
	if curSecretsFP != newSecretsFP {
		return ensureReload
	}
	return ensureNoop
}

func (m *Manager) ensureLocked(inst Instance) error {
	structFP := inst.structuralFingerprint()
	secFP := inst.secretsFingerprint()
	if cur, ok := m.procs[inst.Id]; ok {
		switch ensureActionFor(cur.proc.IsRunning(), cur.structuralFP, cur.secretsFP, structFP, secFP) {
		case ensureNoop:
			cur.tag = inst.Tag
			return nil
		case ensureReload:
			if err := writeConfig(configPathForID(inst.Id), inst, cur.apiPort); err != nil {
				return err
			}
			if requestReload(cur.apiPort) {
				cur.tag = inst.Tag
				cur.secretsFP = secFP
				logger.Infof("mtproto: hot-reloaded secrets for inbound %d", inst.Id)
				return nil
			}
			logger.Warningf("mtproto: reload unavailable for inbound %d, restarting", inst.Id)
			fallthrough
		case ensureRestart:
			_ = cur.proc.Stop()
			delete(m.procs, inst.Id)
		}
	}
	apiPort, err := FreeLocalPort()
	if err != nil {
		return err
	}
	cfgPath := configPathForID(inst.Id)
	if err := writeConfig(cfgPath, inst, apiPort); err != nil {
		return err
	}
	proc := newProcess(cfgPath, fmt.Sprintf("inbound %d", inst.Id))
	if err := proc.Start(); err != nil {
		return err
	}
	m.procs[inst.Id] = &managed{
		proc:         proc,
		tag:          inst.Tag,
		structuralFP: structFP,
		secretsFP:    secFP,
		apiPort:      apiPort,
		last:         map[string]clientCounters{},
	}
	logger.Infof("mtproto: started mtg for inbound %d on %s", inst.Id, inst.bindTo())
	return nil
}

// Remove stops and forgets the mtg process for an inbound id.
func (m *Manager) Remove(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.procs[id]; ok {
		_ = cur.proc.Stop()
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
			_ = cur.proc.Stop()
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

// CollectTraffic scrapes each running mtg /stats endpoint and returns the
// per-client byte deltas since the previous scrape, plus the emails of clients
// with at least one live connection.
func (m *Manager) CollectTraffic() ([]Traffic, []string) {
	type snap struct {
		id      int
		apiPort int
		tag     string
		last    map[string]clientCounters
	}
	m.mu.Lock()
	snaps := make([]snap, 0, len(m.procs))
	for id, cur := range m.procs {
		if cur.proc == nil || !cur.proc.IsRunning() {
			continue
		}
		lastCopy := make(map[string]clientCounters, len(cur.last))
		for k, v := range cur.last {
			lastCopy[k] = v
		}
		snaps = append(snaps, snap{id: id, apiPort: cur.apiPort, tag: cur.tag, last: lastCopy})
	}
	m.mu.Unlock()

	var out []Traffic
	var online []string
	for _, s := range snaps {
		users, ok := scrapeStats(s.apiPort)
		if !ok {
			continue
		}
		newLast := make(map[string]clientCounters, len(users))
		for email, u := range users {
			up := u.BytesIn
			down := u.BytesOut
			newLast[email] = clientCounters{up: up, down: down}
			if u.Connections > 0 {
				online = append(online, email)
			}
			prev, had := s.last[email]
			if !had {
				continue
			}
			du := up - prev.up
			dd := down - prev.down
			if du < 0 {
				du = 0
			}
			if dd < 0 {
				dd = 0
			}
			if du > 0 || dd > 0 {
				out = append(out, Traffic{Tag: s.tag, Email: email, Up: du, Down: dd})
			}
		}

		m.mu.Lock()
		if cur, ok := m.procs[s.id]; ok {
			cur.last = newLast
		}
		m.mu.Unlock()
	}
	return out, online
}

// FreeLocalPort asks the OS for an unused loopback TCP port. It is used both
// for mtg's /stats API endpoint and to allocate the per-inbound SOCKS egress
// bridge port persisted into mtproto inbound settings.
func FreeLocalPort() (int, error) {
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// renderConfig builds the mtg-multi TOML for an instance. Top-level keys must
// precede any [section] header in TOML, and [secrets] must be the final section
// so trailing keys are not swallowed by another table. The layout is therefore:
// top-level scalars (incl. api-bind-to), then [domain-fronting], [network] and
// [throttle], and finally [secrets] with one named secret per active client.
func renderConfig(inst Instance, apiPort int) string {
	var b strings.Builder
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
	fmt.Fprintf(&b, "api-bind-to = \"127.0.0.1:%d\"\n", apiPort)
	if inst.FrontingIP != "" || inst.FrontingPort > 0 || inst.FrontingProxyProtocol {
		b.WriteString("\n[domain-fronting]\n")
		if inst.FrontingIP != "" {
			fmt.Fprintf(&b, "host = %q\n", inst.FrontingIP)
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
	if inst.ThrottleMaxConnections > 0 {
		fmt.Fprintf(&b, "\n[throttle]\nmax-connections = %d\n", inst.ThrottleMaxConnections)
	}
	b.WriteString("\n[secrets]\n")
	for _, e := range inst.Secrets {
		fmt.Fprintf(&b, "%q = %q\n", e.Name, e.Secret)
	}
	return b.String()
}

func writeConfig(path string, inst Instance, apiPort int) error {
	if err := os.MkdirAll(configDir(), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(renderConfig(inst, apiPort)), 0o640)
}

// statsUser is one entry of the mtg-multi /stats users map. bytes_in is traffic
// the client sent to the proxy (upload) and bytes_out is what the proxy returned
// (download).
type statsUser struct {
	Connections int64 `json:"connections"`
	BytesIn     int64 `json:"bytes_in"`
	BytesOut    int64 `json:"bytes_out"`
}

// requestReload asks a running mtg-multi to re-read its config file and swap the
// [secrets] set in place (POST /reload on the same loopback API port that serves
// /stats). It returns true only on a 200: an older binary without the endpoint
// (404), a refused connection, or any other status yields false, so the caller
// falls back to a full restart.
func requestReload(port int) bool {
	client := http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/reload", port), nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// scrapeStats reads the mtg-multi /stats JSON API and returns the per-user
// cumulative counters. Best-effort: an unreachable endpoint or unparseable body
// yields ok=false.
func scrapeStats(port int) (map[string]statsUser, bool) {
	client := http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/stats", port), nil)
	if err != nil {
		return nil, false
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	var parsed struct {
		Users map[string]statsUser `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, false
	}
	return parsed.Users, true
}
