package outbound

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// stubProcess implements batchProcess without an xray binary. When serveSocks
// is set, Start opens a minimal SOCKS5 server on every inbound port from the
// config, so probes run against a real tunnel.
type stubProcess struct {
	cfg        *xray.Config
	startErr   error
	result     string
	serveSocks bool

	running   bool
	listeners []net.Listener
}

func (p *stubProcess) Start() error {
	if p.startErr != nil {
		return p.startErr
	}
	for _, in := range p.cfg.InboundConfigs {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", in.Port))
		if err != nil {
			return err
		}
		p.listeners = append(p.listeners, l)
		if p.serveSocks {
			go serveStubSocks(l)
		}
	}
	p.running = true
	return nil
}

func (p *stubProcess) Stop() error {
	for _, l := range p.listeners {
		l.Close()
	}
	p.running = false
	return nil
}

func (p *stubProcess) IsRunning() bool { return p.running }
func (p *stubProcess) GetResult() string {
	if p.result != "" {
		return p.result
	}
	return "stub exited"
}

// serveStubSocks answers SOCKS5 no-auth CONNECTs and pipes to the requested
// target — just enough protocol for net/http's socks5 client.
func serveStubSocks(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			hello := make([]byte, 2)
			if _, err := io.ReadFull(c, hello); err != nil {
				return
			}
			methods := make([]byte, hello[1])
			if _, err := io.ReadFull(c, methods); err != nil {
				return
			}
			c.Write([]byte{0x05, 0x00})
			hdr := make([]byte, 4)
			if _, err := io.ReadFull(c, hdr); err != nil {
				return
			}
			var host string
			switch hdr[3] {
			case 0x01:
				b := make([]byte, 4)
				io.ReadFull(c, b)
				host = net.IP(b).String()
			case 0x03:
				lb := make([]byte, 1)
				io.ReadFull(c, lb)
				b := make([]byte, lb[0])
				io.ReadFull(c, b)
				host = string(b)
			case 0x04:
				b := make([]byte, 16)
				io.ReadFull(c, b)
				host = net.IP(b).String()
			default:
				return
			}
			pb := make([]byte, 2)
			if _, err := io.ReadFull(c, pb); err != nil {
				return
			}
			port := int(pb[0])<<8 | int(pb[1])
			upstream, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
			if err != nil {
				c.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
				return
			}
			defer upstream.Close()
			c.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			go io.Copy(upstream, c)
			io.Copy(c, upstream)
		}(conn)
	}
}

func withStubProcess(t *testing.T, factory func(cfg *xray.Config, configPath string) batchProcess) {
	t.Helper()
	// createTestConfigPath writes into the bin folder, which doesn't exist
	// when running tests from the package directory.
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	orig := newBatchProcess
	newBatchProcess = factory
	t.Cleanup(func() { newBatchProcess = orig })
}

func withEgressTraceProbe(t *testing.T, probe func(*url.URL) *TestEgressResult) {
	t.Helper()
	orig := egressTraceProbe
	egressTraceProbe = probe
	t.Cleanup(func() { egressTraceProbe = orig })
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestBuildBatchTestConfig(t *testing.T) {
	items := []*httpBatchItem{
		{tag: "wg-sub", outbound: map[string]any{"tag": "wg-sub", "protocol": "wireguard"}},
		{tag: "proxy-a", outbound: map[string]any{"tag": "proxy-a", "protocol": "vless"}},
	}
	allOutbounds := []any{
		map[string]any{"tag": "direct", "protocol": "freedom", "settings": map[string]any{}},
		map[string]any{"tag": "proxy-a", "protocol": "vless", "settings": map[string]any{"address": "a.example.com"}},
	}
	ports := []int{61001, 61002}

	cfg := buildBatchTestConfig(items, allOutbounds, ports)
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	inbounds, _ := m["inbounds"].([]any)
	if len(inbounds) != 2 {
		t.Fatalf("expected 2 inbounds, got %d", len(inbounds))
	}
	for i, raw := range inbounds {
		in := raw.(map[string]any)
		if got := in["tag"]; got != fmt.Sprintf("test-in-%d", i) {
			t.Errorf("inbound %d tag = %v", i, got)
		}
		if got := int(in["port"].(float64)); got != ports[i] {
			t.Errorf("inbound %d port = %d, want %d", i, got, ports[i])
		}
		if got := in["protocol"]; got != "socks" {
			t.Errorf("inbound %d protocol = %v", i, got)
		}
		if got := in["listen"]; got != "127.0.0.1" {
			t.Errorf("inbound %d listen = %v", i, got)
		}
		settings := in["settings"].(map[string]any)
		if settings["auth"] != "noauth" || settings["udp"] != false {
			t.Errorf("inbound %d settings = %v", i, settings)
		}
	}

	routing := m["routing"].(map[string]any)
	rules, _ := routing["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("expected 2 routing rules, got %d", len(rules))
	}
	wantTags := []string{"wg-sub", "proxy-a"}
	for i, raw := range rules {
		rule := raw.(map[string]any)
		inTags := rule["inboundTag"].([]any)
		if len(inTags) != 1 || inTags[0] != fmt.Sprintf("test-in-%d", i) {
			t.Errorf("rule %d inboundTag = %v", i, inTags)
		}
		if rule["outboundTag"] != wantTags[i] {
			t.Errorf("rule %d outboundTag = %v, want %s", i, rule["outboundTag"], wantTags[i])
		}
	}

	outbounds, _ := m["outbounds"].([]any)
	if len(outbounds) != 3 {
		t.Fatalf("expected 3 outbounds (wg-sub appended once, proxy-a deduped), got %d", len(outbounds))
	}
	var wg map[string]any
	for _, raw := range outbounds {
		ob := raw.(map[string]any)
		if ob["tag"] == "wg-sub" {
			wg = ob
		}
	}
	if wg == nil {
		t.Fatal("wg-sub not appended to outbounds")
	}
	if settings, _ := wg["settings"].(map[string]any); settings == nil || settings["noKernelTun"] != true {
		t.Errorf("wireguard settings missing noKernelTun: %v", wg["settings"])
	}

	if m["burstObservatory"] != nil {
		t.Errorf("burstObservatory should not be set, got %v", m["burstObservatory"])
	}
	if m["metrics"] != nil {
		t.Errorf("metrics should not be set, got %v", m["metrics"])
	}
}

func TestTestOutboundsPrevalidationAndOrdering(t *testing.T) {
	calls := 0
	withStubProcess(t, func(cfg *xray.Config, configPath string) batchProcess {
		calls++
		return &stubProcess{cfg: cfg, startErr: errors.New("boom")}
	})

	batch := mustJSON(t, []any{
		map[string]any{"protocol": "vless"},                   // no tag
		map[string]any{"tag": "bh", "protocol": "blackhole"},  // blackhole
		map[string]any{"tag": "loop", "protocol": "loopback"}, // loopback
		map[string]any{"tag": "a", "protocol": "socks"},       // valid
		map[string]any{"tag": "a", "protocol": "vless"},       // duplicate
	})
	results, err := (&OutboundService{}).TestOutbounds(batch, "http://example.invalid/gen", "", "http")
	if err != nil {
		t.Fatalf("TestOutbounds: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	wantErrs := []string{
		"Outbound has no tag",
		"Blocked/blackhole outbound cannot be tested",
		"Loopback outbound cannot be tested",
		"Failed to start test xray instance: boom",
		"Duplicate outbound tag in batch: a",
	}
	for i, want := range wantErrs {
		if results[i].Success {
			t.Errorf("result %d unexpectedly succeeded", i)
		}
		if results[i].Error != want {
			t.Errorf("result %d error = %q, want %q", i, results[i].Error, want)
		}
	}
	if results[3].Tag != "a" || results[4].Tag != "a" || results[1].Tag != "bh" {
		t.Errorf("tags not propagated: %+v", results)
	}
	// Single valid item → no per-item fallback round.
	if calls != 1 {
		t.Errorf("process spawned %d times, want 1", calls)
	}
}

func TestTestOutboundsFallbackOnStartFailure(t *testing.T) {
	calls := 0
	withStubProcess(t, func(cfg *xray.Config, configPath string) batchProcess {
		calls++
		return &stubProcess{cfg: cfg, startErr: errors.New("boom")}
	})

	batch := mustJSON(t, []any{
		map[string]any{"tag": "a", "protocol": "socks"},
		map[string]any{"tag": "b", "protocol": "vless"},
	})
	results, err := (&OutboundService{}).TestOutbounds(batch, "http://example.invalid/gen", "", "http")
	if err != nil {
		t.Fatalf("TestOutbounds: %v", err)
	}
	for i, r := range results {
		if r.Success || r.Error != "Failed to start test xray instance: boom" {
			t.Errorf("result %d = %+v, want start failure", i, r)
		}
	}
	// 1 shared attempt + 2 isolated fallback attempts.
	if calls != 3 {
		t.Errorf("process spawned %d times, want 3", calls)
	}
}

func TestTestOutboundsNoFallbackWhenBinaryMissing(t *testing.T) {
	calls := 0
	withStubProcess(t, func(cfg *xray.Config, configPath string) batchProcess {
		calls++
		return &stubProcess{cfg: cfg, startErr: &fs.PathError{Op: "exec", Path: "xray", Err: fs.ErrNotExist}}
	})

	batch := mustJSON(t, []any{
		map[string]any{"tag": "a", "protocol": "socks"},
		map[string]any{"tag": "b", "protocol": "vless"},
	})
	results, err := (&OutboundService{}).TestOutbounds(batch, "http://example.invalid/gen", "", "http")
	if err != nil {
		t.Fatalf("TestOutbounds: %v", err)
	}
	for i, r := range results {
		if r.Success || !strings.HasPrefix(r.Error, "Failed to start test xray instance:") {
			t.Errorf("result %d = %+v, want start failure", i, r)
		}
	}
	if calls != 1 {
		t.Errorf("process spawned %d times, want 1 (no fallback for missing binary)", calls)
	}
}

func TestTestOutboundsSemaphoreBusy(t *testing.T) {
	withStubProcess(t, func(cfg *xray.Config, configPath string) batchProcess {
		t.Fatal("process must not be spawned while semaphore is held")
		return nil
	})

	httpTestSemaphore.Lock()
	defer httpTestSemaphore.Unlock()

	batch := mustJSON(t, []any{map[string]any{"tag": "a", "protocol": "socks"}})
	results, err := (&OutboundService{}).TestOutbounds(batch, "", "", "http")
	if err != nil {
		t.Fatalf("TestOutbounds: %v", err)
	}
	if results[0].Success || results[0].Error != "Another outbound test is already running, please wait" {
		t.Errorf("result = %+v, want busy error", results[0])
	}
}

func TestTestOutboundsInputValidation(t *testing.T) {
	s := &OutboundService{}
	if _, err := s.TestOutbounds("not json", "", "", "tcp"); err == nil {
		t.Error("expected error for invalid JSON")
	}

	big := make([]any, maxBatchItems+1)
	for i := range big {
		big[i] = map[string]any{"tag": fmt.Sprintf("t%d", i), "protocol": "socks"}
	}
	if _, err := s.TestOutbounds(mustJSON(t, big), "", "", "tcp"); err == nil {
		t.Error("expected error for oversized batch")
	}

	results, err := s.TestOutbounds("[]", "", "", "tcp")
	if err != nil || len(results) != 0 {
		t.Errorf("empty batch: results=%v err=%v", results, err)
	}
}

func TestTestOutboundsTCPLane(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	port := l.Addr().(*net.TCPAddr).Port

	batch := mustJSON(t, []any{map[string]any{
		"tag":      "t1",
		"protocol": "socks",
		"settings": map[string]any{"servers": []any{map[string]any{"address": "127.0.0.1", "port": port}}},
	}})
	results, err := (&OutboundService{}).TestOutbounds(batch, "", "", "tcp")
	if err != nil {
		t.Fatalf("TestOutbounds: %v", err)
	}
	r := results[0]
	if !r.Success || r.Mode != "tcp" || r.Tag != "t1" || len(r.Endpoints) != 1 {
		t.Errorf("unexpected tcp result: %+v", r)
	}
}

func TestTestOutboundsHTTPBatchThroughStubSocks(t *testing.T) {
	var mu sync.Mutex
	requestsPerConn := make(map[string]int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestsPerConn[r.RemoteAddr]++
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var proc *stubProcess
	calls := 0
	withStubProcess(t, func(cfg *xray.Config, configPath string) batchProcess {
		calls++
		proc = &stubProcess{cfg: cfg, serveSocks: true}
		return proc
	})
	withEgressTraceProbe(t, func(*url.URL) *TestEgressResult {
		return &TestEgressResult{IPv4: "198.51.100.1", Country: "ZZ", Warp: "off"}
	})

	batch := mustJSON(t, []any{
		map[string]any{"tag": "a", "protocol": "vless"},
		map[string]any{"tag": "b", "protocol": "trojan"},
	})
	results, err := (&OutboundService{}).TestOutbounds(batch, srv.URL, "", "http")
	if err != nil {
		t.Fatalf("TestOutbounds: %v", err)
	}
	if calls != 1 {
		t.Fatalf("process spawned %d times, want 1", calls)
	}
	for i, r := range results {
		if !r.Success {
			t.Fatalf("result %d failed: %+v", i, r)
		}
		if r.HTTPStatus != http.StatusNoContent {
			t.Errorf("result %d status = %d, want 204", i, r.HTTPStatus)
		}
		if r.Delay < 1 || r.ConnectMs < 1 || r.TTFBMs < 1 {
			t.Errorf("result %d timing not populated: %+v", i, r)
		}
		if r.TLSMs != 0 {
			t.Errorf("result %d TLSMs = %d, want 0 for plain http", i, r.TLSMs)
		}
		if r.Mode != "http" {
			t.Errorf("result %d mode = %q", i, r.Mode)
		}
		if r.Egress == nil || r.Egress.IPv4 != "198.51.100.1" {
			t.Errorf("result %d egress = %+v", i, r.Egress)
		}
	}
	if proc.IsRunning() {
		t.Error("temp process not stopped after batch")
	}

	mu.Lock()
	defer mu.Unlock()
	totalRequests := 0
	for addr, n := range requestsPerConn {
		totalRequests += n
		if n != 2 {
			t.Errorf("connection %s served %d requests, want 2 (warm delay request must reuse the cold request's connection)", addr, n)
		}
	}
	if totalRequests != 4 {
		t.Errorf("test URL served %d requests, want 4 (cold + warm per probe)", totalRequests)
	}
}

func TestTestOutboundsRealDelayBatchThroughStubSocks(t *testing.T) {
	var mu sync.Mutex
	requestsPerConn := make(map[string]int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestsPerConn[r.RemoteAddr]++
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	withStubProcess(t, func(cfg *xray.Config, configPath string) batchProcess {
		return &stubProcess{cfg: cfg, serveSocks: true}
	})

	batch := mustJSON(t, []any{
		map[string]any{"tag": "a", "protocol": "vless"},
		map[string]any{"tag": "wg", "protocol": "wireguard"},
	})
	results, err := (&OutboundService{}).TestOutbounds(batch, srv.URL, "", "real")
	if err != nil {
		t.Fatalf("TestOutbounds: %v", err)
	}
	for i, r := range results {
		if !r.Success {
			t.Fatalf("result %d failed: %+v", i, r)
		}
		if r.Mode != "real" {
			t.Errorf("result %d mode = %q, want %q", i, r.Mode, "real")
		}
		if r.HTTPStatus != http.StatusNoContent {
			t.Errorf("result %d status = %d, want 204", i, r.HTTPStatus)
		}
		if r.Delay < 1 || r.ConnectMs < 1 || r.TTFBMs < 1 {
			t.Errorf("result %d timing not populated: %+v", i, r)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	totalRequests := 0
	for addr, n := range requestsPerConn {
		totalRequests += n
		if n != 1 {
			t.Errorf("connection %s served %d requests, want 1 (real mode must skip the warm request)", addr, n)
		}
	}
	if totalRequests != 2 {
		t.Errorf("test URL served %d requests, want 2 (one cold request per probe)", totalRequests)
	}
}

func TestTestOutboundsTCPModeForcesUDPToHTTPProbe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	withStubProcess(t, func(cfg *xray.Config, configPath string) batchProcess {
		return &stubProcess{cfg: cfg, serveSocks: true}
	})
	withEgressTraceProbe(t, func(*url.URL) *TestEgressResult {
		return &TestEgressResult{IPv4: "198.51.100.2", Country: "ZZ", Warp: "off"}
	})

	batch := mustJSON(t, []any{map[string]any{"tag": "wg", "protocol": "wireguard"}})
	results, err := (&OutboundService{}).TestOutbounds(batch, srv.URL, "", "tcp")
	if err != nil {
		t.Fatalf("TestOutbounds: %v", err)
	}
	r := results[0]
	if !r.Success || r.Mode != "http" {
		t.Errorf("UDP outbound in tcp mode = %+v, want success with mode %q", r, "http")
	}
	if r.Egress == nil || r.Egress.IPv4 != "198.51.100.2" {
		t.Errorf("UDP outbound egress = %+v", r.Egress)
	}
}

func TestProbeModeLabel(t *testing.T) {
	cases := []struct{ mode, want string }{
		{"tcp", "tcp"},
		{"real", "real"},
		{"http", "http"},
		{"", "http"},
		{"bogus", "http"},
	}
	for _, c := range cases {
		if got := probeModeLabel(c.mode); got != c.want {
			t.Errorf("probeModeLabel(%q) = %q, want %q", c.mode, got, c.want)
		}
	}
}

func TestProbeThroughSocksTransportFailure(t *testing.T) {
	// A listener that accepts and immediately closes — SOCKS handshake dies.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	var result TestOutboundResult
	probeThroughSocks(l.Addr().(*net.TCPAddr).Port, "http://127.0.0.1:9/", 2*time.Second, false, &result)
	if result.Success || result.Error == "" {
		t.Errorf("expected transport failure, got %+v", result)
	}
}
