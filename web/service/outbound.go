package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/config"
	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/xray"

	"gorm.io/gorm"
)

// OutboundService provides business logic for managing Xray outbound configurations.
// It handles outbound traffic monitoring and statistics.
type OutboundService struct{}

// httpTestSemaphore serialises HTTP-mode probes (each one spawns a temp xray
// instance, which is too expensive to run in parallel). TCP-mode probes are
// dial-only and don't need the semaphore.
var httpTestSemaphore sync.Mutex

func (s *OutboundService) AddTraffic(traffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) (error, bool) {
	var err error
	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	err = s.addOutboundTraffic(tx, traffics)
	if err != nil {
		return err, false
	}

	return nil, false
}

func (s *OutboundService) addOutboundTraffic(tx *gorm.DB, traffics []*xray.Traffic) error {
	if len(traffics) == 0 {
		return nil
	}

	var err error

	for _, traffic := range traffics {
		if traffic.IsOutbound {

			var outbound model.OutboundTraffics

			err = tx.Model(&model.OutboundTraffics{}).Where("tag = ?", traffic.Tag).
				FirstOrCreate(&outbound).Error
			if err != nil {
				return err
			}

			outbound.Tag = traffic.Tag
			outbound.Up = outbound.Up + traffic.Up
			outbound.Down = outbound.Down + traffic.Down
			outbound.Total = outbound.Up + outbound.Down

			err = tx.Save(&outbound).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *OutboundService) GetOutboundsTraffic() ([]*model.OutboundTraffics, error) {
	db := database.GetDB()
	var traffics []*model.OutboundTraffics

	err := db.Model(model.OutboundTraffics{}).Find(&traffics).Error
	if err != nil {
		logger.Warning("Error retrieving OutboundTraffics: ", err)
		return nil, err
	}

	return traffics, nil
}

func (s *OutboundService) ResetOutboundTraffic(tag string) error {
	db := database.GetDB()

	whereText := "tag "
	if tag == "-alltags-" {
		whereText += " <> ?"
	} else {
		whereText += " = ?"
	}

	result := db.Model(model.OutboundTraffics{}).
		Where(whereText, tag).
		Updates(map[string]any{"up": 0, "down": 0, "total": 0})

	err := result.Error
	if err != nil {
		return err
	}

	return nil
}

// TestOutboundResult represents the result of testing an outbound.
// Delay/timing fields are in milliseconds. Endpoints is only populated for
// TCP-mode probes; the HTTP-mode timing breakdown lives in DNSMs/ConnectMs/
// TLSMs/TTFBMs (any of these can be 0 if the underlying step was skipped —
// e.g. a non-TLS target leaves TLSMs at 0).
type TestOutboundResult struct {
	Success    bool   `json:"success"`
	Delay      int64  `json:"delay"`
	Error      string `json:"error,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
	Mode       string `json:"mode,omitempty"`

	DNSMs     int64 `json:"dnsMs,omitempty"`
	ConnectMs int64 `json:"connectMs,omitempty"`
	TLSMs     int64 `json:"tlsMs,omitempty"`
	TTFBMs    int64 `json:"ttfbMs,omitempty"`

	Endpoints []TestEndpointResult `json:"endpoints,omitempty"`
}

// TestEndpointResult is one entry in a TCP-mode probe — the per-endpoint
// dial outcome for outbounds that expose multiple servers/peers.
type TestEndpointResult struct {
	Address string `json:"address"`
	Success bool   `json:"success"`
	Delay   int64  `json:"delay"`
	Error   string `json:"error,omitempty"`
}

// TestOutbound dispatches to the chosen probe mode:
//   - mode="tcp": dial the outbound's host:port directly. No xray spin-up,
//     parallel-safe, ~100ms per endpoint. Doesn't validate the proxy
//     protocol — only that the remote is reachable on TCP.
//   - mode="" or "http": spin a temp xray instance, route a real HTTP
//     request through it, return delay + a DNS/Connect/TLS/TTFB breakdown.
//     Authoritative but expensive and serialised by httpTestSemaphore.
//
// allOutboundsJSON is only consulted in HTTP mode (it backs
// sockopt.dialerProxy chains during test).
func (s *OutboundService) TestOutbound(outboundJSON string, testURL string, allOutboundsJSON string, mode string) (*TestOutboundResult, error) {
	if mode == "tcp" {
		return s.testOutboundTCP(outboundJSON)
	}
	return s.testOutboundHTTP(outboundJSON, testURL, allOutboundsJSON)
}

func (s *OutboundService) testOutboundTCP(outboundJSON string) (*TestOutboundResult, error) {
	var ob map[string]any
	if err := json.Unmarshal([]byte(outboundJSON), &ob); err != nil {
		return &TestOutboundResult{Mode: "tcp", Success: false, Error: fmt.Sprintf("Invalid outbound JSON: %v", err)}, nil
	}
	tag, _ := ob["tag"].(string)
	protocol, _ := ob["protocol"].(string)
	if protocol == "blackhole" || protocol == "freedom" || tag == "blocked" {
		return &TestOutboundResult{Mode: "tcp", Success: false, Error: "Outbound has no testable endpoint"}, nil
	}

	endpoints := extractOutboundEndpoints(ob)
	if len(endpoints) == 0 {
		return &TestOutboundResult{Mode: "tcp", Success: false, Error: "No testable endpoint"}, nil
	}

	results := make([]TestEndpointResult, len(endpoints))
	var wg sync.WaitGroup
	for i := range endpoints {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = probeTCPEndpoint(endpoints[i], 5*time.Second)
		}(i)
	}
	wg.Wait()

	var bestDelay int64 = -1
	var firstErr string
	for _, r := range results {
		if r.Success {
			if bestDelay < 0 || r.Delay < bestDelay {
				bestDelay = r.Delay
			}
		} else if firstErr == "" {
			firstErr = r.Error
		}
	}

	out := &TestOutboundResult{Mode: "tcp", Endpoints: results}
	if bestDelay >= 0 {
		out.Success = true
		out.Delay = bestDelay
	} else {
		out.Error = firstErr
		if out.Error == "" {
			out.Error = "All endpoints unreachable"
		}
	}
	return out, nil
}

func probeTCPEndpoint(endpoint string, timeout time.Duration) TestEndpointResult {
	r := TestEndpointResult{Address: endpoint}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", endpoint, timeout)
	r.Delay = time.Since(start).Milliseconds()
	if err != nil {
		r.Error = err.Error()
		return r
	}
	conn.Close()
	r.Success = true
	return r
}

func extractOutboundEndpoints(ob map[string]any) []string {
	protocol, _ := ob["protocol"].(string)
	settings, _ := ob["settings"].(map[string]any)
	if settings == nil {
		return nil
	}
	var out []string
	addServer := func(addr any, port any) {
		host, _ := addr.(string)
		p := numAsInt(port)
		if host != "" && p > 0 {
			out = append(out, fmt.Sprintf("%s:%d", host, p))
		}
	}
	switch protocol {
	case "vmess":
		if vnext, ok := settings["vnext"].([]any); ok {
			for _, v := range vnext {
				if vm, ok := v.(map[string]any); ok {
					addServer(vm["address"], vm["port"])
				}
			}
		}
	case "vless":
		addServer(settings["address"], settings["port"])
	case "trojan", "shadowsocks", "http", "socks":
		if servers, ok := settings["servers"].([]any); ok {
			for _, sv := range servers {
				if sm, ok := sv.(map[string]any); ok {
					addServer(sm["address"], sm["port"])
				}
			}
		}
	case "wireguard":
		if peers, ok := settings["peers"].([]any); ok {
			for _, p := range peers {
				if pm, ok := p.(map[string]any); ok {
					if ep, _ := pm["endpoint"].(string); ep != "" {
						out = append(out, ep)
					}
				}
			}
		}
	}
	return out
}

func numAsInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
	}
	return 0
}

func (s *OutboundService) testOutboundHTTP(outboundJSON string, testURL string, allOutboundsJSON string) (*TestOutboundResult, error) {
	if testURL == "" {
		testURL = "https://www.google.com/generate_204"
	}

	if !httpTestSemaphore.TryLock() {
		return &TestOutboundResult{
			Mode:    "http",
			Success: false,
			Error:   "Another outbound test is already running, please wait",
		}, nil
	}
	defer httpTestSemaphore.Unlock()

	var testOutbound map[string]any
	if err := json.Unmarshal([]byte(outboundJSON), &testOutbound); err != nil {
		return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Invalid outbound JSON: %v", err)}, nil
	}
	outboundTag, _ := testOutbound["tag"].(string)
	if outboundTag == "" {
		return &TestOutboundResult{Mode: "http", Success: false, Error: "Outbound has no tag"}, nil
	}
	if protocol, _ := testOutbound["protocol"].(string); protocol == "blackhole" || outboundTag == "blocked" {
		return &TestOutboundResult{Mode: "http", Success: false, Error: "Blocked/blackhole outbound cannot be tested"}, nil
	}

	var allOutbounds []any
	if allOutboundsJSON != "" {
		if err := json.Unmarshal([]byte(allOutboundsJSON), &allOutbounds); err != nil {
			return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Invalid allOutbounds JSON: %v", err)}, nil
		}
	}
	if len(allOutbounds) == 0 {
		allOutbounds = []any{testOutbound}
	}

	testPort, err := findAvailablePort()
	if err != nil {
		return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Failed to find available port: %v", err)}, nil
	}

	testConfig := s.createTestConfig(outboundTag, allOutbounds, testPort)

	testConfigPath, err := createTestConfigPath()
	if err != nil {
		return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Failed to create test config path: %v", err)}, nil
	}
	defer os.Remove(testConfigPath)

	testProcess := xray.NewTestProcess(testConfig, testConfigPath)
	defer func() {
		if testProcess.IsRunning() {
			testProcess.Stop()
		}
	}()

	if err := testProcess.Start(); err != nil {
		return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Failed to start test xray instance: %v", err)}, nil
	}

	if err := waitForPort(testPort, 3*time.Second); err != nil {
		if !testProcess.IsRunning() {
			result := testProcess.GetResult()
			return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Xray process exited: %s", result)}, nil
		}
		return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Xray failed to start listening: %v", err)}, nil
	}

	if !testProcess.IsRunning() {
		result := testProcess.GetResult()
		return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Xray process exited: %s", result)}, nil
	}

	return s.testConnection(testPort, testURL)
}

// createTestConfig creates a test config by copying all outbounds unchanged and adding
// only the test inbound (SOCKS) and a route rule that sends traffic to the given outbound tag.
func (s *OutboundService) createTestConfig(outboundTag string, allOutbounds []any, testPort int) *xray.Config {
	// Test inbound (SOCKS proxy) - only addition to inbounds
	testInbound := xray.InboundConfig{
		Tag:      "test-inbound",
		Listen:   json_util.RawMessage(`"127.0.0.1"`),
		Port:     testPort,
		Protocol: "socks",
		Settings: json_util.RawMessage(`{"auth":"noauth","udp":true}`),
	}

	// Outbounds: copy all, but set noKernelTun=true for WireGuard outbounds
	processedOutbounds := make([]any, len(allOutbounds))
	for i, ob := range allOutbounds {
		outbound, ok := ob.(map[string]any)
		if !ok {
			processedOutbounds[i] = ob
			continue
		}
		if protocol, ok := outbound["protocol"].(string); ok && protocol == "wireguard" {
			// Set noKernelTun to true for WireGuard outbounds
			if settings, ok := outbound["settings"].(map[string]any); ok {
				settings["noKernelTun"] = true
			} else {
				// Create settings if it doesn't exist
				outbound["settings"] = map[string]any{
					"noKernelTun": true,
				}
			}
		}
		processedOutbounds[i] = outbound
	}
	outboundsJSON, _ := json.Marshal(processedOutbounds)

	// Create routing rule to route all traffic through test outbound
	routingRules := []map[string]any{
		{
			"type":        "field",
			"outboundTag": outboundTag,
			"network":     "tcp,udp",
		},
	}

	routingJSON, _ := json.Marshal(map[string]any{
		"domainStrategy": "AsIs",
		"rules":          routingRules,
	})

	// Disable logging for test process to avoid creating orphaned log files
	logConfig := map[string]any{
		"loglevel": "warning",
		"access":   "none",
		"error":    "none",
		"dnsLog":   false,
	}
	logJSON, _ := json.Marshal(logConfig)

	// Create minimal config
	cfg := &xray.Config{
		LogConfig: json_util.RawMessage(logJSON),
		InboundConfigs: []xray.InboundConfig{
			testInbound,
		},
		OutboundConfigs: json_util.RawMessage(string(outboundsJSON)),
		RouterConfig:    json_util.RawMessage(string(routingJSON)),
		Policy:          json_util.RawMessage(`{}`),
		Stats:           json_util.RawMessage(`{}`),
	}

	return cfg
}

// testConnection runs the actual HTTP probe through the local SOCKS proxy.
// A warmup request seeds xray's DNS cache / handshake; then a fresh
// transport runs the measured request so httptrace sees a real cold
// connection and reports DNS/Connect/TLS/TTFB. Note that DNS and Connect
// reflect *client → SOCKS-on-loopback*, not the remote target — those
// happen inside xray and aren't visible to net/http. TLS and TTFB are
// the meaningful breakdown values for a SOCKS-proxied HTTPS probe.
func (s *OutboundService) testConnection(proxyPort int, testURL string) (*TestOutboundResult, error) {
	proxyURLStr := fmt.Sprintf("socks5://127.0.0.1:%d", proxyPort)
	proxyURLParsed, err := url.Parse(proxyURLStr)
	if err != nil {
		return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Invalid proxy URL: %v", err)}, nil
	}

	mkClient := func() *http.Client {
		return &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURLParsed),
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:       1,
				IdleConnTimeout:    1 * time.Second,
				DisableCompression: true,
			},
		}
	}

	warmup := mkClient()
	warmupResp, err := warmup.Get(testURL)
	if err != nil {
		return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Request failed: %v", err)}, nil
	}
	io.Copy(io.Discard, warmupResp.Body)
	warmupResp.Body.Close()
	warmup.CloseIdleConnections()

	var dnsStart, dnsDone, connectStart, connectDone, tlsStart, tlsDone, firstByte time.Time
	trace := &httptrace.ClientTrace{
		DNSStart:             func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone:              func(_ httptrace.DNSDoneInfo) { dnsDone = time.Now() },
		ConnectStart:         func(_, _ string) { connectStart = time.Now() },
		ConnectDone:          func(_, _ string, _ error) { connectDone = time.Now() },
		TLSHandshakeStart:    func() { tlsStart = time.Now() },
		TLSHandshakeDone:     func(_ tls.ConnectionState, _ error) { tlsDone = time.Now() },
		GotFirstResponseByte: func() { firstByte = time.Now() },
	}

	client := mkClient()
	defer client.CloseIdleConnections()
	ctx := httptrace.WithClientTrace(context.Background(), trace)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
	if err != nil {
		return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Request build failed: %v", err)}, nil
	}

	startTime := time.Now()
	resp, err := client.Do(req)
	delay := time.Since(startTime).Milliseconds()
	if err != nil {
		return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Request failed: %v", err)}, nil
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	out := &TestOutboundResult{
		Mode:       "http",
		Success:    true,
		Delay:      delay,
		StatusCode: resp.StatusCode,
	}
	if !dnsStart.IsZero() && !dnsDone.IsZero() {
		out.DNSMs = dnsDone.Sub(dnsStart).Milliseconds()
	}
	if !connectStart.IsZero() && !connectDone.IsZero() {
		out.ConnectMs = connectDone.Sub(connectStart).Milliseconds()
	}
	if !tlsStart.IsZero() && !tlsDone.IsZero() {
		out.TLSMs = tlsDone.Sub(tlsStart).Milliseconds()
	}
	if !firstByte.IsZero() {
		out.TTFBMs = firstByte.Sub(startTime).Milliseconds()
	}
	return out, nil
}

// waitForPort polls until the given TCP port is accepting connections or the timeout expires.
func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("port %d not ready after %v", port, timeout)
}

// findAvailablePort finds an available port for testing
func findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
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
