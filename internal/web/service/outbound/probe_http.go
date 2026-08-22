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
	"strings"
	"sync"
	"sync/atomic"
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
// handshakes of connection establishment. Mode "real" instead reports the
// cold request's full elapsed time and skips the warm request.

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
	// egressTraceTimeout keeps diagnostic trace metadata from extending a
	// successful HTTP probe by the full reachability timeout.
	egressTraceTimeout = 3 * time.Second

	defaultTestURL  = "https://www.google.com/generate_204"
	egressTraceHost = "cloudflare.com"
	egressTracePath = "/cdn-cgi/trace"

	// speedUploadByteCap is a safety ceiling on the synthetic POST body, not a
	// tuned value -- the reader's own deadline is what ends the transfer.
	speedUploadByteCap = 2_000_000_000 // ~2 GB
	// speedTransferChunkSize sizes each Read call in the download loop.
	speedTransferChunkSize = 64 << 10

	defaultSpeedUploadURL = "https://speed.cloudflare.com/__up"
	// defaultSpeedDownloadURL is a plain static file, not Cloudflare's own
	// __down: that endpoint TLS-fingerprints Go's client and 403s it.
	defaultSpeedDownloadURL = "https://proof.ovh.net/files/1Gb.dat"

	// speedProbeUserAgent is defense-in-depth for any custom test URL behind
	// simpler, header-based bot checks (the default targets above need none).
	speedProbeUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	// cfMetaUploadBytesHeader is Cloudflare's own count of bytes actually
	// received -- more trustworthy than our local reader's count.
	cfMetaUploadBytesHeader = "Cf-Meta-Upload-Bytes"
)

// Vars, not consts, solely so tests can shorten these (save/restore) to
// exercise deadline-cutoff behavior without an 8+-second test.
var (
	// speedProbeConnectTimeout bounds only reaching a usable connection --
	// generous so a slow-to-establish outbound (Tor) loses no transfer budget.
	speedProbeConnectTimeout = 15 * time.Second
	// speedProbeTransferDuration is the measurement window, per direction.
	// A deadline hit mid-transfer is a valid partial measurement, not a failure.
	speedProbeTransferDuration = 8 * time.Second
)

// httpTestSemaphore serialises HTTP-mode batches (each spawns a temp xray
// instance, which is too expensive to run in parallel). TCP-mode probes are
// dial-only and don't need the semaphore.
var httpTestSemaphore sync.Mutex

// speedTestSemaphore is separate from httpTestSemaphore so a long "Test All
// Speeds" pass doesn't block an ordinary quick latency check.
var speedTestSemaphore sync.Mutex

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

var egressTraceProbe = probeEgressTrace

// httpBatchItem is one outbound inside an HTTP-mode batch. result is the
// pre-allocated entry in the caller's result slice, filled in place.
type httpBatchItem struct {
	index    int
	tag      string
	outbound map[string]any
	result   *TestOutboundResult
}

func probeModeLabel(mode string) string {
	switch mode {
	case "tcp", "real", "speed":
		return mode
	default:
		return "http"
	}
}

// probeOptions bundles per-mode knobs threaded down to the per-item probe
// call, so a new setting doesn't need another positional param everywhere.
type probeOptions struct {
	testURL          string
	realDelay        bool
	mode             string
	speedDownloadURL string
	speedUploadURL   string
}

// TestOutbound probes a single outbound; legacy single-test API kept for the
// /testOutbound endpoint. Mode dispatch matches TestOutbounds.
func (s *OutboundService) TestOutbound(outboundJSON string, testURL string, allOutboundsJSON string, mode string, speedDownloadURL string, speedUploadURL string) (*TestOutboundResult, error) {
	var ob map[string]any
	if err := json.Unmarshal([]byte(outboundJSON), &ob); err != nil {
		return &TestOutboundResult{Mode: probeModeLabel(mode), Success: false, Error: fmt.Sprintf("Invalid outbound JSON: %v", err)}, nil
	}
	results := s.testOutboundsParsed([]map[string]any{ob}, testURL, allOutboundsJSON, mode, speedDownloadURL, speedUploadURL)
	return results[0], nil
}

// TestOutbounds probes a JSON array of outbounds, returning one result per
// input in order. Empty URL params fall back to their own defaults.
func (s *OutboundService) TestOutbounds(outboundsJSON string, testURL string, allOutboundsJSON string, mode string, speedDownloadURL string, speedUploadURL string) ([]*TestOutboundResult, error) {
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
	return s.testOutboundsParsed(items, testURL, allOutboundsJSON, mode, speedDownloadURL, speedUploadURL), nil
}

// testOutboundsParsed splits items into the TCP lane and the HTTP lane (also
// used for "speed" mode, forced to concurrency 1), then runs both.
func (s *OutboundService) testOutboundsParsed(items []map[string]any, testURL string, allOutboundsJSON string, mode string, speedDownloadURL string, speedUploadURL string) []*TestOutboundResult {
	results := make([]*TestOutboundResult, len(items))

	modeLabel := probeModeLabel(mode)
	probeLabel := modeLabel
	if probeLabel == "tcp" {
		probeLabel = "http"
	}
	realDelay := mode == "real"

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
		r := &TestOutboundResult{Tag: tag, Mode: probeLabel}
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
	if speedDownloadURL == "" {
		speedDownloadURL = defaultSpeedDownloadURL
	}
	if speedUploadURL == "" {
		speedUploadURL = defaultSpeedUploadURL
	}
	opts := probeOptions{
		testURL:          testURL,
		realDelay:        realDelay,
		mode:             mode,
		speedDownloadURL: speedDownloadURL,
		speedUploadURL:   speedUploadURL,
	}

	sem := &httpTestSemaphore
	busyMsg := "Another outbound test is already running, please wait"
	if mode == "speed" {
		sem = &speedTestSemaphore
		busyMsg = "Another speed test is already running, please wait"
	}
	if !sem.TryLock() {
		failAll(busyMsg)
		return results
	}
	defer sem.Unlock()

	retryPerItem, err := runHTTPProbeBatch(httpItems, allOutbounds, opts)
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
		if _, ferr := runHTTPProbeBatch([]*httpBatchItem{it}, allOutbounds, opts); ferr != nil {
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
func runHTTPProbeBatch(items []*httpBatchItem, allOutbounds []any, opts probeOptions) (retryPerItem bool, err error) {
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

	// Speed mode is hard-forced to concurrency 1 here too (defense in depth):
	// concurrent throughput tests would contend for the same uplink/downlink.
	concurrency := httpProbeConcurrency
	if opts.mode == "speed" {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i := range items {
		wg.Add(1)
		go func(it *httpBatchItem, port int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if opts.mode == "speed" {
				probeSpeedThroughSocks(port, opts.speedDownloadURL, opts.speedUploadURL, speedProbeConnectTimeout, speedProbeTransferDuration, it.result)
			} else {
				probeThroughSocks(port, opts.testURL, httpProbeTimeout, opts.realDelay, it.result)
			}
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
func probeThroughSocks(port int, testURL string, timeout time.Duration, realDelay bool, result *TestOutboundResult) {
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
	if !realDelay {
		if warmDelay, ok := timedWarmGet(client, testURL); ok {
			delay = warmDelay
		}
	}
	result.Delay = max(delay, 1)
	if !realDelay {
		result.Egress = egressTraceProbe(proxyURL)
	}
}

// probeSpeedThroughSocks measures download then upload, strictly
// sequentially -- concurrent directions would contend for the same uplink.
func probeSpeedThroughSocks(port int, downloadURL, uploadURL string, connectTimeout, transferDuration time.Duration, result *TestOutboundResult) {
	proxyURL := &url.URL{Scheme: "socks5", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port))}
	tr := &http.Transport{
		Proxy:               http.ProxyURL(proxyURL),
		MaxIdleConns:        1,
		MaxIdleConnsPerHost: 1,
	}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr}

	var errs []string

	downBytes, downDur, downErr := speedProbeDownload(client, downloadURL, connectTimeout, transferDuration)
	if downErr != nil {
		errs = append(errs, "download: "+downErr.Error())
	} else {
		result.Success = true
		result.DownloadMbps = max(mbps(downBytes, downDur), 0.01)
	}

	upBytes, upDur, upErr := speedProbeUpload(client, uploadURL, connectTimeout, transferDuration)
	if upErr != nil {
		errs = append(errs, "upload: "+upErr.Error())
	} else {
		result.Success = true
		result.UploadMbps = max(mbps(upBytes, upDur), 0.01)
	}

	if len(errs) > 0 {
		result.Error = strings.Join(errs, "; ")
	}
	if !result.Success && result.Error == "" {
		result.Error = "Speed test failed"
	}
}

// speedProbeDownload swaps a connect-phase timer for a transfer-phase timer
// once headers arrive, so a slow connect never eats into the transfer window.
func speedProbeDownload(client *http.Client, downloadURL string, connectTimeout, transferDuration time.Duration) (bytesRead int64, elapsed time.Duration, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connectTimer := time.AfterFunc(connectTimeout, cancel)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		connectTimer.Stop()
		return 0, 0, err
	}
	req.Header.Set("User-Agent", speedProbeUserAgent)
	resp, err := client.Do(req)
	connectTimer.Stop()
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	transferTimer := time.AfterFunc(transferDuration, cancel)
	defer transferTimer.Stop()

	start := time.Now()
	buf := make([]byte, speedTransferChunkSize)
	var total int64
	for {
		n, rerr := resp.Body.Read(buf)
		total += int64(n)
		if rerr != nil || time.Since(start) >= transferDuration {
			break
		}
	}
	return total, time.Since(start), nil
}

// uploadStart crosses from the transport's goroutine to speedProbeUpload's
// via a channel (startCh), not a shared variable, so reads are race-free.
type uploadStart struct {
	at    time.Time
	timer *time.Timer
}

// speedProbeUpload ends the body at EOF (not by aborting) once transferDuration
// elapses, so the server's own received-byte count, when reported, wins over ours.
func speedProbeUpload(client *http.Client, uploadURL string, connectTimeout, transferDuration time.Duration) (bytesWritten int64, elapsed time.Duration, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connectTimer := time.AfterFunc(connectTimeout, cancel)

	body := &countingZeroReader{cap: speedUploadByteCap}
	startCh := make(chan uploadStart, 1)
	var startOnce sync.Once
	trace := &httptrace.ClientTrace{
		WroteHeaders: func() {
			startOnce.Do(func() {
				connectTimer.Stop()
				now := time.Now()
				body.setDeadline(now.Add(transferDuration))
				timer := time.AfterFunc(2*transferDuration, cancel)
				startCh <- uploadStart{at: now, timer: timer}
			})
		},
	}

	req, reqErr := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodPost, uploadURL, body)
	if reqErr != nil {
		connectTimer.Stop()
		return 0, 0, reqErr
	}
	req.Header.Set("User-Agent", speedProbeUserAgent)
	resp, doErr := client.Do(req)
	connectTimer.Stop()

	var s uploadStart
	select {
	case s = <-startCh:
	default:
	}
	if s.timer != nil {
		s.timer.Stop()
	}

	if doErr != nil {
		if s.at.IsZero() {
			return atomic.LoadInt64(&body.read), 0, doErr
		}
		return atomic.LoadInt64(&body.read), time.Since(s.at), nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, probeDrainLimit))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	written := atomic.LoadInt64(&body.read)
	if serverBytes, ok := parseCfMetaUploadBytes(resp.Header); ok {
		written = serverBytes
	}
	if s.at.IsZero() {
		return written, 0, nil
	}
	return written, time.Since(s.at), nil
}

// parseCfMetaUploadBytes reads Cloudflare's own received-byte count, when
// present, so upload throughput reflects real delivery, not local buffering.
func parseCfMetaUploadBytes(h http.Header) (int64, bool) {
	v := h.Get(cfMetaUploadBytesHeader)
	if v == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// countingZeroReader hands out up-to-cap zero bytes for a speed-probe POST body;
// setDeadline makes Read EOF at a wall-clock cutoff instead of the caller aborting.
type countingZeroReader struct {
	cap          int64
	read         int64
	deadlineNano int64
}

func (r *countingZeroReader) setDeadline(t time.Time) {
	atomic.StoreInt64(&r.deadlineNano, t.UnixNano())
}

func (r *countingZeroReader) Read(p []byte) (int, error) {
	if dl := atomic.LoadInt64(&r.deadlineNano); dl != 0 && time.Now().UnixNano() >= dl {
		return 0, io.EOF
	}
	remaining := r.cap - atomic.LoadInt64(&r.read)
	if remaining <= 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > remaining {
		n = remaining
	}
	atomic.AddInt64(&r.read, n)
	return int(n), nil
}

// mbps returns 0 (not a tiny positive number) when there's nothing to
// measure, so callers can tell "no data" apart from "measured, and slow".
func mbps(bytesTransferred int64, elapsed time.Duration) float64 {
	seconds := elapsed.Seconds()
	if seconds <= 0 || bytesTransferred <= 0 {
		return 0
	}
	return float64(bytesTransferred) * 8 / 1_000_000 / seconds
}

// probeEgressTrace fetches Cloudflare's plain-text trace endpoint through the
// same SOCKS route used by the HTTP probe. It asks one IPv4 and one IPv6
// Cloudflare address directly, when available, while keeping the TLS SNI as
// cloudflare.com. Failures are intentionally ignored by the caller: egress
// metadata is diagnostic, not reachability.
func probeEgressTrace(proxyURL *url.URL) *TestEgressResult {
	ipv4, ipv6 := cloudflareTraceTargets()
	if ipv4 == nil && ipv6 == nil {
		return nil
	}

	tr := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{
			ServerName: egressTraceHost,
		},
	}
	defer tr.CloseIdleConnections()

	client := &http.Client{
		Transport: tr,
		Timeout:   egressTraceTimeout,
	}

	egress := &TestEgressResult{}
	targets := make([]net.IP, 0, 2)
	if ipv4 != nil {
		targets = append(targets, ipv4)
	}
	if ipv6 != nil {
		targets = append(targets, ipv6)
	}
	results := make(chan map[string]string, len(targets))
	for _, target := range targets {
		go func(ip net.IP) {
			results <- fetchCloudflareTrace(client, ip)
		}(target)
	}
	for range targets {
		applyEgressTrace(egress, <-results)
	}
	if egress.IPv4 == "" && egress.IPv6 == "" && egress.Country == "" && egress.Warp == "" {
		return nil
	}

	return egress
}

func cloudflareTraceTargets() (net.IP, net.IP) {
	ctx, cancel := context.WithTimeout(context.Background(), egressTraceTimeout)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, egressTraceHost)
	if err != nil {
		return nil, nil
	}
	var ipv4, ipv6 net.IP
	for _, addr := range addrs {
		ip := addr.IP
		if ipv4 == nil {
			if v4 := ip.To4(); v4 != nil {
				ipv4 = v4
				continue
			}
		}
		if ipv6 == nil && ip.To4() == nil && ip.To16() != nil {
			ipv6 = ip
		}
		if ipv4 != nil && ipv6 != nil {
			break
		}
	}
	return ipv4, ipv6
}

func fetchCloudflareTrace(client *http.Client, ip net.IP) map[string]string {
	if ip == nil {
		return nil
	}
	traceURL := (&url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(ip.String(), "443"),
		Path:   egressTracePath,
	}).String()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, traceURL, nil)
	if err != nil {
		return nil
	}
	req.Host = egressTraceHost
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if err != nil {
		return nil
	}
	return parseCloudflareTrace(string(body))
}

func applyEgressTrace(egress *TestEgressResult, values map[string]string) {
	if len(values) == 0 {
		return
	}
	if ip := net.ParseIP(values["ip"]); ip != nil {
		if ip.To4() != nil {
			if egress.IPv4 == "" {
				egress.IPv4 = values["ip"]
			}
		} else if egress.IPv6 == "" {
			egress.IPv6 = values["ip"]
		}
	}
	if egress.Country == "" {
		egress.Country = values["loc"]
	}
	if values["warp"] == "on" || egress.Warp == "" {
		egress.Warp = values["warp"]
	}
}

func parseCloudflareTrace(body string) map[string]string {
	values := make(map[string]string)
	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return values
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
