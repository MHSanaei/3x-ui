package outbound

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// HTTP-mode probing works by spinning up ONE temporary xray instance per
// batch: every outbound under test gets its own loopback SOCKS inbound plus
// an inboundTag→outboundTag routing rule, and the panel then issues a real,
// individually-timed HTTP request through each inbound. Measuring the request
// client-side (instead of polling xray's observatory) returns the moment the
// response lands, yields the actual HTTP status, and allows an httptrace
// timing breakdown — while the shared process keeps "Test All" at one xray
// spawn per batch instead of one per outbound. The reported delay comes from
// a second request on the kept-alive connection, so it reflects the tunnel's
// real per-request round-trip rather than the stacked SOCKS/proxy/TLS
// handshakes of connection establishment.

const (
	// httpProbeTimeout bounds each probe request end-to-end (a probe makes
	// two: a cold one for the breakdown, a warm one for the delay).
	httpProbeTimeout = 10 * time.Second
	// probeDrainLimit caps how much response body a probe reads back to keep
	// the connection reusable for the warm request.
	probeDrainLimit = 256 << 10
	// httpProbeConcurrency caps parallel probe requests within a batch —
	// enough to keep a batch fast, low enough not to spike CPU with TLS
	// handshakes on small VPSes.
	httpProbeConcurrency = 16
	// batchPortsReadyTimeout bounds the wait for the temp instance to open
	// its test inbounds.
	batchPortsReadyTimeout = 10 * time.Second
	// maxBatchItems caps one batch request; the frontend chunks below this.
	maxBatchItems = 50
	// tcpBatchConcurrency caps parallel TCP-mode items in a batch (each item
	// already dials its endpoints concurrently).
	tcpBatchConcurrency = 8

	defaultTestURL = "https://www.google.com/generate_204"
)

// httpTestSemaphore serialises HTTP-mode batches (each spawns a temp xray
// instance, which is too expensive to run in parallel). TCP-mode probes are
// dial-only and don't need the semaphore.
var httpTestSemaphore sync.Mutex

// batchProcess is the slice of xray.Process the batch engine needs; a seam
// so unit tests can stub the process without an xray binary.
type batchProcess interface {
	Start() error
	Stop() error
	IsRunning() bool
	GetResult() string
}

var newBatchProcess = func(cfg *xray.Config, configPath string) batchProcess {
	return xray.NewTestProcess(cfg, configPath)
}

// httpBatchItem is one outbound inside an HTTP-mode batch. result is the
// pre-allocated entry in the caller's result slice, filled in place.
type httpBatchItem struct {
	index    int
	tag      string
	outbound map[string]any
	result   *TestOutboundResult
}

// TestOutbound probes a single outbound; legacy single-test API kept for the
// /testOutbound endpoint. Dispatch matches TestOutbounds: mode "tcp" dials
// the outbound's endpoints directly, anything else routes a real HTTP request
// through a temp xray instance (UDP-transport outbounds are always forced to
// the HTTP probe — a raw dial can't measure them).
func (s *OutboundService) TestOutbound(outboundJSON string, testURL string, allOutboundsJSON string, mode string) (*TestOutboundResult, error) {
	var ob map[string]any
	if err := json.Unmarshal([]byte(outboundJSON), &ob); err != nil {
		m := "http"
		if mode == "tcp" {
			m = "tcp"
		}
		return &TestOutboundResult{Mode: m, Success: false, Error: fmt.Sprintf("Invalid outbound JSON: %v", err)}, nil
	}
	results := s.testOutboundsParsed([]map[string]any{ob}, testURL, allOutboundsJSON, mode)
	return results[0], nil
}

// TestOutbounds probes a JSON array of outbounds and returns one result per
// input, in input order, each carrying the outbound's tag. allOutboundsJSON
// supplies the config context (sockopt.dialerProxy chains); testURL falls
// back to the default probe URL when empty.
func (s *OutboundService) TestOutbounds(outboundsJSON string, testURL string, allOutboundsJSON string, mode string) ([]*TestOutboundResult, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(outboundsJSON), &raw); err != nil {
		return nil, fmt.Errorf("invalid outbounds JSON: %w", err)
	}
	if len(raw) > maxBatchItems {
		return nil, fmt.Errorf("too many outbounds in one request (max %d)", maxBatchItems)
	}
	items := make([]map[string]any, len(raw))
	for i, r := range raw {
		var ob map[string]any
		if err := json.Unmarshal(r, &ob); err == nil {
			items[i] = ob
		}
	}
	return s.testOutboundsParsed(items, testURL, allOutboundsJSON, mode), nil
}

// testOutboundsParsed splits items into the TCP lane (direct dials, bounded
// worker pool) and the HTTP lane (one shared temp xray instance), runs both,
// and returns results aligned with items. A nil item marks unparseable input.
func (s *OutboundService) testOutboundsParsed(items []map[string]any, testURL string, allOutboundsJSON string, mode string) []*TestOutboundResult {
	results := make([]*TestOutboundResult, len(items))

	modeLabel := "http"
	if mode == "tcp" {
		modeLabel = "tcp"
	}

	type tcpEntry struct {
		idx int
		ob  map[string]any
	}
	var tcpLane []tcpEntry
	var httpItems []*httpBatchItem
	seenTags := make(map[string]bool)

	for i, ob := range items {
		if ob == nil {
			results[i] = &TestOutboundResult{Mode: modeLabel, Success: false, Error: "Invalid outbound JSON"}
			continue
		}
		// A bare TCP dial only proves reachability for TCP-based proxies.
		// UDP protocols (wireguard, hysteria, kcp/quic transports) ignore
		// unauthenticated packets, so a raw dial can't tell "reachable" from
		// "dead" — route them through the real xray probe.
		if mode == "tcp" && !outboundTransportIsUDP(ob) {
			tcpLane = append(tcpLane, tcpEntry{idx: i, ob: ob})
			continue
		}

		tag, _ := ob["tag"].(string)
		r := &TestOutboundResult{Tag: tag, Mode: "http"}
		results[i] = r
		protocol, _ := ob["protocol"].(string)
		switch {
		case tag == "":
			r.Error = "Outbound has no tag"
		case protocol == "blackhole" || tag == "blocked":
			r.Error = "Blocked/blackhole outbound cannot be tested"
		case protocol == "loopback":
			r.Error = "Loopback outbound cannot be tested"
		case protocol == "freedom" || protocol == "dns":
			// Direct/DNS outbounds aren't proxies — an HTTP probe through them
			// would only measure the host's own reachability, not a tunnel.
			r.Error = "Direct/DNS outbound cannot be tested"
		case seenTags[tag]:
			r.Error = fmt.Sprintf("Duplicate outbound tag in batch: %s", tag)
		default:
			seenTags[tag] = true
			httpItems = append(httpItems, &httpBatchItem{index: i, tag: tag, outbound: ob, result: r})
		}
	}

	if len(tcpLane) > 0 {
		var wg sync.WaitGroup
		sem := make(chan struct{}, tcpBatchConcurrency)
		for _, e := range tcpLane {
			wg.Add(1)
			go func(e tcpEntry) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				obJSON, err := json.Marshal(e.ob)
				if err != nil {
					tag, _ := e.ob["tag"].(string)
					results[e.idx] = &TestOutboundResult{Tag: tag, Mode: "tcp", Success: false, Error: fmt.Sprintf("Invalid outbound JSON: %v", err)}
					return
				}
				r, _ := s.testOutboundTCP(string(obJSON))
				results[e.idx] = r
			}(e)
		}
		wg.Wait()
	}

	if len(httpItems) == 0 {
		return results
	}

	failAll := func(msg string) {
		for _, it := range httpItems {
			it.result.Success = false
			it.result.Error = msg
		}
	}

	var allOutbounds []any
	if allOutboundsJSON != "" {
		if err := json.Unmarshal([]byte(allOutboundsJSON), &allOutbounds); err != nil {
			failAll(fmt.Sprintf("Invalid allOutbounds JSON: %v", err))
			return results
		}
	}

	if testURL == "" {
		testURL = defaultTestURL
	}

	if !httpTestSemaphore.TryLock() {
		failAll("Another outbound test is already running, please wait")
		return results
	}
	defer httpTestSemaphore.Unlock()

	retryPerItem, err := runHTTPProbeBatch(httpItems, allOutbounds, testURL)
	if err == nil {
		return results
	}
	if !retryPerItem || len(httpItems) == 1 {
		failAll(err.Error())
		return results
	}
	// The shared process never came up — one structurally-bad outbound can
	// poison the whole batch config. Retry each item in its own isolated
	// instance so the broken outbound reports xray's real error and the
	// rest still get tested. Serial: the poisoned case fails fast (~1s).
	for _, it := range httpItems {
		if _, ferr := runHTTPProbeBatch([]*httpBatchItem{it}, allOutbounds, testURL); ferr != nil {
			it.result.Success = false
			it.result.Error = ferr.Error()
		}
	}
	return results
}

// runHTTPProbeBatch makes one shared-process attempt for the given items,
// writing per-request outcomes into the items' results. It returns a non-nil
// error only when the process never became usable; retryPerItem reports
// whether splitting the batch into per-item instances could help (true for
// start failures / early exits that a poisoned config would explain, false
// for environmental failures like a missing binary or no free ports).
func runHTTPProbeBatch(items []*httpBatchItem, allOutbounds []any, testURL string) (retryPerItem bool, err error) {
	ports, release, err := reserveLoopbackPorts(len(items))
	if err != nil {
		return false, fmt.Errorf("Failed to reserve test ports: %w", err)
	}
	defer release()

	cfg := buildBatchTestConfig(items, allOutbounds, ports)

	configPath, err := createTestConfigPath()
	if err != nil {
		return false, fmt.Errorf("Failed to create test config path: %w", err)
	}
	defer os.Remove(configPath)

	proc := newBatchProcess(cfg, configPath)
	defer func() {
		if proc.IsRunning() {
			_ = proc.Stop()
		}
	}()

	// Free the reserved ports just before xray binds them; the window is
	// milliseconds, and a lost race makes xray exit fast, which surfaces
	// below and triggers the per-item retry with fresh ports.
	release()
	if err := proc.Start(); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Binary missing — per-item retries would all fail the same way.
			return false, fmt.Errorf("Failed to start test xray instance: %w", err)
		}
		return true, fmt.Errorf("Failed to start test xray instance: %w", err)
	}

	if err := waitForPortsReady(proc, ports, batchPortsReadyTimeout); err != nil {
		return err.exited, err
	}

	sem := make(chan struct{}, httpProbeConcurrency)
	var wg sync.WaitGroup
	for i := range items {
		wg.Add(1)
		go func(it *httpBatchItem, port int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			probeThroughSocks(port, testURL, httpProbeTimeout, it.result)
		}(items[i], ports[i])
	}
	wg.Wait()

	if !proc.IsRunning() {
		detail := proc.GetResult()
		for _, it := range items {
			if !it.result.Success {
				it.result.Error = "Xray process exited: " + detail
			}
		}
	}
	return false, nil
}

// portsReadyError distinguishes "process died" (a poisoned config — worth a
// per-item retry) from "ports never opened while alive" (environmental).
type portsReadyError struct {
	msg    string
	exited bool
}

func (e *portsReadyError) Error() string { return e.msg }

// waitForPortsReady polls until every test inbound accepts connections,
// aborting as soon as the process exits.
func waitForPortsReady(proc batchProcess, ports []int, timeout time.Duration) *portsReadyError {
	deadline := time.Now().Add(timeout)
	for _, port := range ports {
		for {
			if !proc.IsRunning() {
				return &portsReadyError{msg: "Xray process exited: " + proc.GetResult(), exited: true}
			}
			conn, err := (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
			if err == nil {
				conn.Close()
				break
			}
			if time.Now().After(deadline) {
				return &portsReadyError{msg: fmt.Sprintf("Xray failed to open test inbounds: port %d not ready after %v", port, timeout)}
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	return nil
}

// buildBatchTestConfig assembles the temp instance config: one loopback SOCKS
// inbound per tested outbound, a routing rule binding each inbound to its
// outbound tag, and the full outbound context so dialerProxy chains resolve.
func buildBatchTestConfig(items []*httpBatchItem, allOutbounds []any, ports []int) *xray.Config {
	// allOutbounds is the template's outbound list; subscription outbounds
	// are injected at runtime and aren't part of it, so append any tested
	// outbound whose tag is missing. When a tested outbound's tag collides
	// with a template outbound, the template version wins — same semantics
	// as the pre-batch tester.
	outbounds := make([]any, 0, len(allOutbounds)+len(items))
	outbounds = append(outbounds, allOutbounds...)
	for _, it := range items {
		if !outboundsContainTag(outbounds, it.tag) {
			outbounds = append(outbounds, it.outbound)
		}
	}
	for _, ob := range outbounds {
		outbound, ok := ob.(map[string]any)
		if !ok {
			continue
		}
		// The temp instance must not touch kernel WireGuard devices.
		if protocol, ok := outbound["protocol"].(string); ok && protocol == "wireguard" {
			if settings, ok := outbound["settings"].(map[string]any); ok {
				settings["noKernelTun"] = true
			} else {
				outbound["settings"] = map[string]any{"noKernelTun": true}
			}
		}
	}
	outboundsJSON, _ := json.Marshal(outbounds)

	inbounds := make([]xray.InboundConfig, len(items))
	rules := make([]any, len(items))
	for i, it := range items {
		inTag := fmt.Sprintf("test-in-%d", i)
		inbounds[i] = xray.InboundConfig{
			Listen:   json_util.RawMessage(`"127.0.0.1"`),
			Port:     ports[i],
			Protocol: "socks",
			Settings: json_util.RawMessage(`{"auth":"noauth","udp":false}`),
			Tag:      inTag,
		}
		rules[i] = map[string]any{
			"type":        "field",
			"inboundTag":  []string{inTag},
			"outboundTag": it.tag,
		}
	}
	routingJSON, _ := json.Marshal(map[string]any{
		"domainStrategy": "AsIs",
		"rules":          rules,
	})

	logJSON, _ := json.Marshal(map[string]any{
		"loglevel": "warning",
		"access":   "none",
		"error":    "",
		"dnsLog":   false,
	})

	return &xray.Config{
		LogConfig:       json_util.RawMessage(logJSON),
		InboundConfigs:  inbounds,
		OutboundConfigs: json_util.RawMessage(outboundsJSON),
		RouterConfig:    json_util.RawMessage(routingJSON),
		Policy:          json_util.RawMessage(`{}`),
		Stats:           json_util.RawMessage(`{}`),
	}
}

// outboundsContainTag reports whether any outbound in the slice has the given tag.
func outboundsContainTag(outbounds []any, tag string) bool {
	for _, ob := range outbounds {
		if m, ok := ob.(map[string]any); ok {
			if t, _ := m["tag"].(string); t == tag {
				return true
			}
		}
	}
	return false
}

// probeThroughSocks probes the local SOCKS inbound at the given port and
// fills result. A first, cold GET proves reachability and carries the
// httptrace breakdown: any HTTP response — including 4xx/5xx and unfollowed
// redirects — counts as reachable; only transport-level failures (refused,
// reset, timeout, proxy errors) are failures. Delay is then re-measured on a
// warm request over the kept-alive connection — the real round-trip through
// the established tunnel — falling back to the cold total if the warm request
// fails. The test URL's hostname is resolved by xray (Go's SOCKS5 client
// sends the domain to the proxy), so DNS goes through the outbound too.
func probeThroughSocks(port int, testURL string, timeout time.Duration, result *TestOutboundResult) {
	proxyURL := &url.URL{Scheme: "socks5", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port))}
	tr := &http.Transport{
		Proxy:               http.ProxyURL(proxyURL),
		MaxIdleConns:        1,
		MaxIdleConnsPerHost: 1,
		IdleConnTimeout:     timeout,
	}
	defer tr.CloseIdleConnections()
	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
		// A redirect would re-dial through the proxy and skew the timing;
		// the 3xx itself already proves the outbound works.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	// Timing breakdown. ConnectStart/Done wrap the TCP dial to the local
	// inbound (the SOCKS handshake isn't traced, and xray ACKs CONNECT
	// before dialing upstream — so the real outbound establishment lands in
	// the TLS phase for https URLs, or inside TTFB for plain http).
	var (
		connStart, tlsStart           time.Time
		connDur, tlsDur, ttfbDur      time.Duration
		connDone, tlsDone, gotFirstRB bool
	)
	start := time.Now()
	trace := &httptrace.ClientTrace{
		ConnectStart: func(network, addr string) {
			if connStart.IsZero() {
				connStart = time.Now()
			}
		},
		ConnectDone: func(network, addr string, err error) {
			if err == nil && !connDone && !connStart.IsZero() {
				connDone = true
				connDur = time.Since(connStart)
			}
		},
		TLSHandshakeStart: func() {
			if tlsStart.IsZero() {
				tlsStart = time.Now()
			}
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			if err == nil && !tlsDone && !tlsStart.IsZero() {
				tlsDone = true
				tlsDur = time.Since(tlsStart)
			}
		},
		GotFirstResponseByte: func() {
			if !gotFirstRB {
				gotFirstRB = true
				ttfbDur = time.Since(start)
			}
		},
	}

	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(context.Background(), trace), http.MethodGet, testURL, nil)
	if err != nil {
		result.Error = err.Error()
		return
	}
	resp, err := client.Do(req)
	coldDelay := time.Since(start).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return
	}
	drainAndClose(resp)

	result.Success = true
	result.HTTPStatus = resp.StatusCode
	if connDone {
		result.ConnectMs = max(connDur.Milliseconds(), 1)
	}
	if tlsDone {
		result.TLSMs = max(tlsDur.Milliseconds(), 1)
	}
	if gotFirstRB {
		result.TTFBMs = max(ttfbDur.Milliseconds(), 1)
	}

	delay := coldDelay
	if warmDelay, ok := timedWarmGet(client, testURL); ok {
		delay = warmDelay
	}
	result.Delay = max(delay, 1)
}

// timedWarmGet re-issues the probe request over the transport's kept-alive
// connection and returns its duration — the tunnel's per-request round-trip.
func timedWarmGet(client *http.Client, testURL string) (int64, bool) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testURL, nil)
	if err != nil {
		return 0, false
	}
	start := time.Now()
	resp, err := client.Do(req)
	delay := time.Since(start).Milliseconds()
	if err != nil {
		return 0, false
	}
	drainAndClose(resp)
	return delay, true
}

// drainAndClose consumes the body (bounded by probeDrainLimit) so the
// connection returns to the keep-alive pool for the warm request.
func drainAndClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, probeDrainLimit))
	resp.Body.Close()
}

// reserveLoopbackPorts grabs n free loopback ports and keeps the listeners
// open so nothing else claims them; release() frees them (idempotent — the
// caller releases right before starting xray and again via defer).
func reserveLoopbackPorts(n int) ([]int, func(), error) {
	listeners := make([]net.Listener, 0, n)
	release := func() {
		for _, l := range listeners {
			l.Close()
		}
	}
	ports := make([]int, 0, n)
	for range n {
		l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			release()
			return nil, nil, err
		}
		listeners = append(listeners, l)
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	return ports, release, nil
}

// createTestConfigPath returns a unique path for a temporary xray config file in the bin folder.
// The temp file is created and closed so the path is reserved; Start() will overwrite it.
func createTestConfigPath() (string, error) {
	tmpFile, err := os.CreateTemp(config.GetBinFolderPath(), "xray_test_*.json")
	if err != nil {
		return "", err
	}
	path := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}
