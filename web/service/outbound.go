package service

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
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
// Delay is in milliseconds. Endpoints is only populated for TCP-mode
// probes; HTTP mode reports the round-trip delay measured by xray's
// burstObservatory probe.
type TestOutboundResult struct {
	Success bool   `json:"success"`
	Delay   int64  `json:"delay"`
	Error   string `json:"error,omitempty"`
	Mode    string `json:"mode,omitempty"`

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
		// A bare TCP dial only proves reachability for TCP-based proxies.
		// UDP protocols (wireguard, hysteria, kcp/quic transports) ignore
		// unauthenticated packets, so a raw dial can't tell "reachable" from
		// "dead" — route them through the authoritative xray handshake probe.
		var ob map[string]any
		if json.Unmarshal([]byte(outboundJSON), &ob) == nil && outboundTransportIsUDP(ob) {
			return s.testOutboundHTTP(outboundJSON, testURL, allOutboundsJSON)
		}
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

// outboundTransportIsUDP reports whether the outbound's proxy speaks UDP
// (wireguard, hysteria, or a kcp/quic/hysteria stream transport). A bare
// UDP dial can't probe these — they ignore unauthenticated packets, so a
// dial neither proves reachability nor measures latency. Such outbounds
// must go through the real xray handshake probe instead.
func outboundTransportIsUDP(ob map[string]any) bool {
	if protocol, _ := ob["protocol"].(string); protocol == "hysteria" || protocol == "wireguard" {
		return true
	}
	if stream, ok := ob["streamSettings"].(map[string]any); ok {
		if n, _ := stream["network"].(string); n == "hysteria" || n == "kcp" || n == "quic" {
			return true
		}
	}
	return false
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
	case "hysteria":
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

// testOutboundHTTP spins up a temporary xray instance whose only job is
// to run a burstObservatory probe against the target outbound, then polls
// xray's metrics /debug/vars endpoint until that outbound is reported
// alive (success) or the deadline expires (failure). The probe lives
// inside xray, so the measured delay and any failure reason reflect what
// xray itself sees over the real proxy chain — no SOCKS round-trip on
// the client side.
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

	metricsPort, err := findAvailablePort()
	if err != nil {
		return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Failed to find available port: %v", err)}, nil
	}

	testConfig := s.createTestConfig(outboundTag, allOutbounds, metricsPort, testURL)

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

	if err := waitForPort(metricsPort, 5*time.Second); err != nil {
		if !testProcess.IsRunning() {
			result := testProcess.GetResult()
			return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Xray process exited: %s", result)}, nil
		}
		return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Xray failed to start metrics listener: %v", err)}, nil
	}

	if !testProcess.IsRunning() {
		result := testProcess.GetResult()
		return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Xray process exited: %s", result)}, nil
	}

	return pollObservatoryResult(testProcess, metricsPort, outboundTag, 12*time.Second), nil
}

// createTestConfig builds a probe-only xray config: the original outbounds
// are kept as-is so dialerProxy chains still resolve, a burstObservatory
// is wired to probe the target tag, and a metrics listener exposes the
// observatory snapshot via /debug/vars. No inbound or routing rules are
// needed — burstObservatory issues the probe traffic itself.
func (s *OutboundService) createTestConfig(outboundTag string, allOutbounds []any, metricsPort int, probeURL string) *xray.Config {
	processedOutbounds := make([]any, len(allOutbounds))
	for i, ob := range allOutbounds {
		outbound, ok := ob.(map[string]any)
		if !ok {
			processedOutbounds[i] = ob
			continue
		}
		if protocol, ok := outbound["protocol"].(string); ok && protocol == "wireguard" {
			if settings, ok := outbound["settings"].(map[string]any); ok {
				settings["noKernelTun"] = true
			} else {
				outbound["settings"] = map[string]any{"noKernelTun": true}
			}
		}
		processedOutbounds[i] = outbound
	}
	outboundsJSON, _ := json.Marshal(processedOutbounds)

	routingJSON, _ := json.Marshal(map[string]any{
		"domainStrategy": "AsIs",
		"rules":          []any{},
	})

	burstObservatoryJSON, _ := json.Marshal(map[string]any{
		"subjectSelector": []string{outboundTag},
		"pingConfig": map[string]any{
			"destination":   probeURL,
			"interval":      "1s",
			"connectivity":  "",
			"timeout":       "5s",
			"samplingCount": 1,
		},
	})

	metricsJSON, _ := json.Marshal(map[string]any{
		"tag":    "test-metrics",
		"listen": fmt.Sprintf("127.0.0.1:%d", metricsPort),
	})

	logConfig := map[string]any{
		"loglevel": "warning",
		"access":   "none",
		"error":    "none",
		"dnsLog":   false,
	}
	logJSON, _ := json.Marshal(logConfig)

	cfg := &xray.Config{
		LogConfig:        json_util.RawMessage(logJSON),
		InboundConfigs:   []xray.InboundConfig{},
		OutboundConfigs:  json_util.RawMessage(string(outboundsJSON)),
		RouterConfig:     json_util.RawMessage(string(routingJSON)),
		Policy:           json_util.RawMessage(`{}`),
		Stats:            json_util.RawMessage(`{}`),
		BurstObservatory: json_util.RawMessage(string(burstObservatoryJSON)),
		Metrics:          json_util.RawMessage(string(metricsJSON)),
	}

	return cfg
}

// observatoryEntry mirrors the per-outbound shape published by xray's
// observatory under /debug/vars.
type observatoryEntry struct {
	Alive        bool   `json:"alive"`
	Delay        int64  `json:"delay"`
	LastSeenTime int64  `json:"last_seen_time"`
	LastTryTime  int64  `json:"last_try_time"`
	OutboundTag  string `json:"outbound_tag"`
}

// pollObservatoryResult repeatedly reads /debug/vars and returns as soon
// as the target outbound reports alive=true. burstObservatory updates the
// snapshot after each ping (interval=1s, timeout=5s), so a healthy
// outbound usually surfaces within ~2s and the timeout caps the wait for
// truly dead ones.
func pollObservatoryResult(testProcess *xray.Process, metricsPort int, tag string, timeout time.Duration) *TestOutboundResult {
	url := fmt.Sprintf("http://127.0.0.1:%d/debug/vars", metricsPort)
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)
	var lastEntry observatoryEntry
	var sawEntry bool
	for time.Now().Before(deadline) {
		if !testProcess.IsRunning() {
			result := testProcess.GetResult()
			return &TestOutboundResult{Mode: "http", Success: false, Error: fmt.Sprintf("Xray process exited: %s", result)}
		}
		entry, ok := fetchObservatoryEntry(client, url, tag)
		if ok {
			if entry.Alive {
				delay := entry.Delay
				if delay <= 0 {
					delay = 1
				}
				return &TestOutboundResult{Mode: "http", Success: true, Delay: delay}
			}
			lastEntry = entry
			sawEntry = true
		}
		time.Sleep(400 * time.Millisecond)
	}

	msg := "Probe timed out — outbound did not become reachable"
	if sawEntry && lastEntry.LastTryTime > 0 {
		msg = fmt.Sprintf("All probes failed (last attempt %ds ago)", time.Now().Unix()-lastEntry.LastTryTime)
	}
	return &TestOutboundResult{Mode: "http", Success: false, Error: msg}
}

func fetchObservatoryEntry(client *http.Client, url, tag string) (observatoryEntry, bool) {
	resp, err := client.Get(url)
	if err != nil {
		return observatoryEntry{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return observatoryEntry{}, false
	}
	var payload struct {
		Observatory map[string]observatoryEntry `json:"observatory"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return observatoryEntry{}, false
	}
	if entry, ok := payload.Observatory[tag]; ok {
		return entry, true
	}
	for _, entry := range payload.Observatory {
		if entry.OutboundTag == tag {
			return entry, true
		}
	}
	return observatoryEntry{}, false
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
