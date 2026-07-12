package outbound

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

// OutboundService provides business logic for managing Xray outbound configurations.
// It handles outbound traffic monitoring and statistics.
type OutboundService struct{}

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

// saturatingAdd caps counters at database.TrafficMax: unlike the SQL paths,
// this read-modify-write add happens in Go, where an int64 overflow silently
// wraps negative instead of erroring (#5762).
func saturatingAdd(a, b int64) int64 {
	if b > database.TrafficMax-a {
		return database.TrafficMax
	}
	return a + b
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
			outbound.Up = saturatingAdd(outbound.Up, traffic.Up)
			outbound.Down = saturatingAdd(outbound.Down, traffic.Down)
			outbound.Total = saturatingAdd(outbound.Up, outbound.Down)

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
// probes; HTTP mode reports the round-trip of a real HTTP request on an
// established connection through the outbound (the cold first request
// supplies the timing breakdown).
type TestOutboundResult struct {
	Tag     string `json:"tag,omitempty"`
	Success bool   `json:"success"`
	Delay   int64  `json:"delay"`
	Error   string `json:"error,omitempty"`
	Mode    string `json:"mode,omitempty"`

	// HTTP-mode extras. Any HTTP response counts as reachable; HTTPStatus
	// records what the test URL answered. ConnectMs is the dial to the local
	// test inbound; TLSMs covers outbound-chain establishment + target TLS
	// (https URLs only, since xray ACKs the SOCKS CONNECT before dialing
	// upstream); TTFBMs is request start → first response byte.
	HTTPStatus int   `json:"httpStatus,omitempty"`
	ConnectMs  int64 `json:"connectMs,omitempty"`
	TLSMs      int64 `json:"tlsMs,omitempty"`
	TTFBMs     int64 `json:"ttfbMs,omitempty"`

	Endpoints []TestEndpointResult `json:"endpoints,omitempty"`
	Egress    *TestEgressResult    `json:"egress,omitempty"`
}

// TestEndpointResult is one entry in a TCP-mode probe — the per-endpoint
// dial outcome for outbounds that expose multiple servers/peers.
type TestEndpointResult struct {
	Address string `json:"address"`
	Success bool   `json:"success"`
	Delay   int64  `json:"delay"`
	Error   string `json:"error,omitempty"`
}

// TestEgressResult is populated by HTTP-mode probes from Cloudflare's trace
// endpoint. It reports what an external service sees after the outbound chain.
type TestEgressResult struct {
	IPv4    string `json:"ipv4,omitempty"`
	IPv6    string `json:"ipv6,omitempty"`
	Country string `json:"country,omitempty"`
	Warp    string `json:"warp,omitempty"`
}

func (s *OutboundService) testOutboundTCP(outboundJSON string) (*TestOutboundResult, error) {
	var ob map[string]any
	if err := json.Unmarshal([]byte(outboundJSON), &ob); err != nil {
		return &TestOutboundResult{Mode: "tcp", Success: false, Error: fmt.Sprintf("Invalid outbound JSON: %v", err)}, nil
	}
	tag, _ := ob["tag"].(string)
	protocol, _ := ob["protocol"].(string)
	if protocol == "blackhole" || protocol == "freedom" || tag == "blocked" {
		return &TestOutboundResult{Tag: tag, Mode: "tcp", Success: false, Error: "Outbound has no testable endpoint"}, nil
	}

	endpoints := extractOutboundEndpoints(ob)
	if len(endpoints) == 0 {
		return &TestOutboundResult{Tag: tag, Mode: "tcp", Success: false, Error: "No testable endpoint"}, nil
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

	out := &TestOutboundResult{Tag: tag, Mode: "tcp", Endpoints: results}
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
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(context.Background(), "tcp", endpoint)
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
