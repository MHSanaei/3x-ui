package wattunnel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/childproc"
	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

type Spec struct {
	GUID     string `json:"guid"`
	Owner    string `json:"owner"`
	Revision int64  `json:"revision"`
	Restart  int64  `json:"restart"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Deleted  bool   `json:"deleted"`
	Config   Config `json:"config"`
	Secret   string `json:"secret"`
}

type Metrics struct {
	carriers   []string `json:"-"`
	Alive      bool     `json:"alive"`
	RTT        *float64 `json:"srtt_ms"`
	Loss       *float64 `json:"loss_pct"`
	BytesIn    uint64   `json:"bytes_in_total"`
	BytesOut   uint64   `json:"bytes_out_total"`
	SendErrors uint64   `json:"send_errors"`
	LastRX     int64    `json:"last_rx_age_ms"`
}

type Status struct {
	State    string   `json:"state" example:"connected"`
	Carrier  string   `json:"carrier" example:"tcp"`
	Reason   string   `json:"reason"`
	Revision int64    `json:"revision"`
	Metrics  *Metrics `json:"metrics"`
}

type Capabilities struct {
	Supported bool     `json:"supported"`
	OS        string   `json:"os" example:"linux"`
	Arch      string   `json:"arch" example:"amd64"`
	Version   string   `json:"version" example:"3.3.0"`
	Carriers  []string `json:"carriers"`
	RawSocket bool     `json:"rawSocket"`
	Error     string   `json:"error"`
}

func BinaryPath() string {
	name := "wattun-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(config.GetBinFolderPath(), name)
}
func RuntimeDir() string { return filepath.Join(config.GetBinFolderPath(), "wattunnel-runtime") }
func KeyDir() string     { return filepath.Join(config.GetDBFolderPath(), "wattunnel-secrets") }

func GetCapabilities() Capabilities {
	c := Capabilities{OS: runtime.GOOS, Arch: runtime.GOARCH, Version: Version, Carriers: []string{}}
	c.Supported = runtime.GOOS == "linux" && (runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64") || runtime.GOOS == "windows" && runtime.GOARCH == "amd64"
	if !c.Supported {
		c.Error = "WatTunnel is not distributed for this platform"
		return c
	}
	c.Carriers = []string{"udp", "tcp", "tls", "wss"}
	c.RawSocket = canRawSocket()
	if c.RawSocket {
		c.Carriers = append(c.Carriers, "icmp")
	}
	if err := VerifyBinary(BinaryPath()); err != nil {
		c.Error = err.Error()
	}
	return c
}

type running struct {
	spec        Spec
	fingerprint string
	cmd         *exec.Cmd
	done        chan struct{}
	selected    string
	api         string
	started     time.Time
	unhealthy   time.Time
	retry       time.Time
	failures    int
	diagnostics *processDiagnostics
	status      Status
}

type Manager struct {
	mu        sync.Mutex
	instances map[string]*running
	binary    string
	dir       string
	verify    func(string) error
	client    *http.Client
	now       func() time.Time
}

func NewManager(binary, dir string) *Manager {
	return &Manager{instances: map[string]*running{}, binary: binary, dir: dir, verify: VerifyBinary, client: &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil, MaxConnsPerHost: 1, MaxIdleConns: 16, IdleConnTimeout: 30 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("metrics redirect refused") }}, now: time.Now}
}

var defaultManager = NewManager("", "")

func GetManager() *Manager { return defaultManager }

func validGUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
		} else if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func (m *Manager) Apply(ctx context.Context, s Spec) error {
	if !validGUID(s.GUID) {
		return errors.New("invalid tunnel GUID")
	}
	if !s.Deleted && s.Enabled {
		if err := s.Config.Validate(); err != nil {
			return err
		}
		if err := ValidateKey(s.Secret); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	fp := hex.EncodeToString(hash[:])
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.instances[s.GUID]
	if r == nil {
		r = &running{}
		m.instances[s.GUID] = r
	}
	if s.Revision < r.spec.Revision {
		return errors.New("stale tunnel revision")
	}
	if s.Deleted || !s.Enabled {
		if err := m.stop(r); err != nil {
			return err
		}
		r.spec = s
		r.status = Status{State: "stopped", Revision: s.Revision}
		if s.Deleted {
			dir := m.instanceDir(s.GUID)
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			delete(m.instances, s.GUID)
		}
		return nil
	}
	if fp != r.fingerprint {
		if err := m.stop(r); err != nil {
			return err
		}
		r.spec = s
		r.fingerprint = fp
		r.retry = time.Time{}
		r.failures = 0
		r.unhealthy = time.Time{}
		r.selected = ""
	}
	return m.ensure(ctx, r)
}

func (m *Manager) instanceDir(guid string) string {
	dir := m.dir
	if dir == "" {
		dir = RuntimeDir()
	}
	return filepath.Join(dir, guid)
}

func isRunning(r *running) bool {
	if r.cmd == nil || r.done == nil {
		return false
	}
	select {
	case <-r.done:
		return false
	default:
		return true
	}
}

func (m *Manager) stop(r *running) error {
	if !isRunning(r) {
		r.cmd = nil
		return nil
	}
	if err := childproc.Stop(r.cmd, r.done, 5*time.Second, 2*time.Second); err != nil {
		return err
	}
	r.cmd = nil
	r.status.Metrics = nil
	return nil
}

func (m *Manager) ensure(ctx context.Context, r *running) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := m.now()
	if isRunning(r) {
		stats, err := m.stats(ctx, r.api)
		if err == nil {
			r.status.Metrics = stats
			if r.spec.Config.Carrier == "auto" && r.spec.Config.Role == "server" {
				r.status.Carrier = strings.Join(stats.carriers, "+")
			}
			if stats.Alive {
				if r.status.State != "connected" && r.spec.Config.Carrier == "auto" && r.spec.Config.Role == "client" {
					if err := WritePrivate(filepath.Join(m.instanceDir(r.spec.GUID), "last-carrier"), []byte(r.selected)); err != nil {
						r.status.Reason = "connected; cannot persist last working transport"
					}
				}
				r.status.State = "connected"
				r.unhealthy = time.Time{}
				r.failures = 0
				return nil
			}
			r.status.State = "connecting"
		} else {
			r.status.Metrics = nil
			r.status.State = "degraded"
		}
		if r.unhealthy.IsZero() {
			r.unhealthy = now
		}
		if now.Sub(r.unhealthy) < 45*time.Second {
			return nil
		}
		if r.spec.Config.Role == "server" && err == nil {
			r.status.State = "waiting"
			return nil
		}
		if err := m.stop(r); err != nil {
			return err
		}
		r.status.Reason = "authenticated peer unavailable; retrying transport"
		if err != nil {
			r.status.Reason = "loopback metrics unavailable; restarting transport"
		}
		r.failures++
		r.retry = now.Add(time.Second * time.Duration(1<<min(r.failures, 6)))
		if r.spec.Config.Carrier == "auto" {
			r.selected = m.nextCarrier(r)
		}
		return nil
	}
	if r.cmd != nil {
		r.cmd = nil
		r.failures++
		r.status.State = "error"
		r.status.Metrics = nil
		r.status.Reason = "WatTunnel exited; retrying with backoff"
		if reason := r.diagnostics.Reason(); reason != "" {
			r.status.Reason = reason + "; retrying with backoff"
		}
		r.retry = now.Add(time.Second * time.Duration(1<<min(r.failures, 6)))
		if r.spec.Config.Carrier == "auto" {
			r.selected = m.nextCarrier(r)
		}
	}
	if now.Before(r.retry) {
		return nil
	}
	if r.selected == "" {
		last := readPreferredCarrier(filepath.Join(m.instanceDir(r.spec.GUID), "last-carrier"))
		if last != "" && r.spec.Config.Carrier == "auto" {
			for _, candidate := range r.spec.Config.Normalized().Candidates {
				if candidate.Carrier == last && (candidate.Carrier != "icmp" || canRawSocket()) {
					r.selected = candidate.Carrier
					break
				}
			}
		}
		if r.selected == "" {
			r.selected = m.nextCarrier(r)
		}
	}
	if err := m.start(ctx, r); err != nil {
		r.failures++
		r.status = Status{State: "error", Carrier: r.selected, Reason: err.Error(), Revision: r.spec.Revision}
		r.retry = now.Add(time.Second * time.Duration(1<<min(r.failures, 6)))
		if r.spec.Config.Carrier == "auto" {
			r.selected = m.nextCarrier(r)
		}
		return err
	}
	r.started = now
	r.unhealthy = now
	r.status = Status{State: "connecting", Carrier: r.selected, Revision: r.spec.Revision, Reason: r.status.Reason}
	return nil
}

func (m *Manager) nextCarrier(r *running) string {
	c := r.spec.Config.Normalized()
	if c.Carrier != "auto" {
		return c.Carrier
	}
	names := []string{}
	for _, name := range carrierOrder {
		for _, v := range c.Candidates {
			if v.Carrier == name && (name != "icmp" || canRawSocket()) {
				names = append(names, name)
			}
		}
	}
	if len(names) == 0 {
		return ""
	}
	idx := slices.Index(names, r.selected)
	return names[(idx+1)%len(names)]
}

func (m *Manager) start(ctx context.Context, r *running) error {
	binary := m.binary
	if binary == "" {
		binary = BinaryPath()
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		return err
	}
	if err = m.verify(binary); err != nil {
		return err
	}
	c := r.spec.Config.Normalized()
	if c.Carrier == "auto" {
		c.Candidates = slices.DeleteFunc(slices.Clone(c.Candidates), func(p CarrierEndpoint) bool { return p.Carrier == "icmp" && !canRawSocket() })
		if len(c.Candidates) == 0 {
			return errors.New("no supported Auto transport remains on this node")
		}
	}
	if r.selected == "icmp" && !canRawSocket() {
		return errors.New("ICMP requires Linux and CAP_NET_RAW")
	}
	dir, err := filepath.Abs(m.instanceDir(r.spec.GUID))
	if err != nil {
		return err
	}
	keyPath := filepath.Join(dir, "key")
	cfgPath := filepath.Join(dir, "config.yml")
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	addr := ln.Addr().String()
	if err = ln.Close(); err != nil {
		return err
	}
	icmpPeer := c.Endpoint
	needsICMP := r.selected == "icmp" || c.Carrier == "auto" && c.Role == "server" && slices.ContainsFunc(c.Candidates, func(p CarrierEndpoint) bool { return p.Carrier == "icmp" })
	if needsICMP && (c.Role == "client" || c.Reverse) {
		if c.Role == "server" {
			icmpPeer = c.ReversePeer
		}
		lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		ips, lookupErr := net.DefaultResolver.LookupNetIP(lookupCtx, "ip4", icmpPeer)
		cancel()
		if lookupErr != nil || len(ips) == 0 {
			return errors.New("ICMP peer requires a resolvable IPv4 address")
		}
		icmpPeer = ips[0].String()
	}
	rendered, err := Render(c, keyPath, addr, r.selected, icmpPeer)
	if err != nil {
		return err
	}
	if err = WritePrivate(keyPath, []byte(r.spec.Secret)); err != nil {
		return err
	}
	if err = WritePrivate(cfgPath, []byte(rendered)); err != nil {
		return err
	}
	// The managed process outlives the HTTP request or reconciliation deadline.
	cmd := exec.CommandContext(context.Background(), binary, "-config", cfgPath)
	cmd.Stdout = io.Discard
	r.diagnostics = &processDiagnostics{}
	cmd.Stderr = r.diagnostics
	if err = prepareChild(cmd, c, r.selected, dir); err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("start WatTunnel: %w", err)
	}
	if err = childproc.Attach(cmd); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("attach WatTunnel child lifetime: %w", err)
	}
	r.cmd = cmd
	r.done = make(chan struct{})
	r.api = "http://" + addr + "/stats"
	done := r.done
	go func() { _ = cmd.Wait(); close(done) }()
	return nil
}

func (m *Manager) stats(ctx context.Context, url string) (*Metrics, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("metrics unavailable")
	}
	var raw struct {
		Clients []struct {
			Carrier string `json:"carrier"`
			Age     int64  `json:"age_ms"`
		} `json:"clients"`
		Alive  json.RawMessage `json:"alive"`
		RTT    *float64        `json:"srtt_ms"`
		Loss   *float64        `json:"loss_pct"`
		In     uint64          `json:"bytes_in_total"`
		Out    uint64          `json:"bytes_out_total"`
		Errors uint64          `json:"send_errors"`
		LastRX int64           `json:"last_rx_age_ms"`
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 1<<20 {
		return nil, errors.New("metrics too large")
	}
	if err = json.Unmarshal(b, &raw); err != nil {
		return nil, errors.New("invalid metrics")
	}
	alive := string(raw.Alive) == "true" || string(raw.Alive) == "1"
	if !alive {
		raw.RTT, raw.Loss = nil, nil
	}
	names := []string{}
	for _, c := range raw.Clients {
		if slices.Contains(carrierOrder, c.Carrier) && c.Age < 10000 && !slices.Contains(names, c.Carrier) {
			names = append(names, c.Carrier)
		}
	}
	slices.Sort(names)
	return &Metrics{carriers: names, Alive: alive, RTT: raw.RTT, Loss: raw.Loss, BytesIn: raw.In, BytesOut: raw.Out, SendErrors: raw.Errors, LastRX: raw.LastRX}, nil
}

func (m *Manager) Status(guid string) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.instances[guid]
	if r == nil {
		return Status{State: "stopped"}
	}
	s := r.status
	if !isRunning(r) && r.spec.Enabled && s.State != "error" {
		s.State = "starting"
		s.Metrics = nil
	}
	return s
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.instances {
		_ = m.stop(r)
	}
	m.instances = map[string]*running{}
	m.client.CloseIdleConnections()
}

type Binding struct {
	Address  string
	Port     int
	Protocol string
}

func (c Config) Bindings() []Binding {
	c = c.Normalized()
	out := []Binding{}
	if c.Role == "client" {
		for _, p := range c.Ports {
			out = append(out, Binding{c.Bind, p.Listen, p.Protocol})
		}
	}
	if c.Role == "server" && !c.Reverse || c.Role == "client" && c.Reverse {
		carriers := c.Candidates
		if c.Carrier != "auto" {
			carriers = []CarrierEndpoint{{c.Carrier, c.Port}}
		}
		for _, p := range carriers {
			if p.Carrier == "icmp" {
				continue
			}
			proto := "tcp"
			if p.Carrier == "udp" {
				proto = "udp"
			}
			out = append(out, Binding{c.Listen, p.Port, proto})
		}
	}
	return out
}

func readPreferredCarrier(path string) string {
	f, err := openPreferredCarrier(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16 {
		return ""
	}
	raw, err := io.ReadAll(io.LimitReader(f, 16))
	if err != nil {
		return ""
	}
	return string(raw)
}
