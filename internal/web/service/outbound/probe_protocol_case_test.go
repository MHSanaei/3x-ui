package outbound

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The core lowercases a protocol id and a transport name before it resolves
// either, so every reader here has to accept the spelling the core accepts.

func TestTestOutboundsTCPModeForcesCoreSpelledUDPToHTTPProbe(t *testing.T) {
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

	batch := mustJSON(t, []any{map[string]any{"tag": "wg", "protocol": "WireGuard"}})
	results, err := (&OutboundService{}).TestOutbounds(batch, srv.URL, "", "tcp")
	if err != nil {
		t.Fatalf("TestOutbounds: %v", err)
	}
	r := results[0]
	if !r.Success || r.Mode != "http" {
		t.Errorf(`"WireGuard" outbound in tcp mode = %+v, want success with mode %q`, r, "http")
	}
	if r.Egress == nil || r.Egress.IPv4 != "198.51.100.2" {
		t.Errorf(`"WireGuard" outbound egress = %+v`, r.Egress)
	}
}

func TestOutboundTransportIsUDPMatchesTheCore(t *testing.T) {
	tests := []struct {
		name string
		ob   map[string]any
		want bool
	}{
		{"canonical wireguard", map[string]any{"protocol": "wireguard"}, true},
		{"capitalised wireguard", map[string]any{"protocol": "WireGuard"}, true},
		{"upper hysteria", map[string]any{"protocol": "HYSTERIA"}, true},
		{"amneziawg", map[string]any{"protocol": "amneziawg"}, true},
		{"kcp transport", map[string]any{"streamSettings": map[string]any{"network": "kcp"}}, true},
		{"kcp transport capitalised", map[string]any{"streamSettings": map[string]any{"network": "KCP"}}, true},
		{"mkcp alias", map[string]any{"streamSettings": map[string]any{"network": "mkcp"}}, true},
		{"mkcp alias capitalised", map[string]any{"streamSettings": map[string]any{"network": "MKCP"}}, true},
		{"tcp transport", map[string]any{"streamSettings": map[string]any{"network": "tcp"}}, false},
		{"plain vless", map[string]any{"protocol": "vless"}, false},
		{"matched but tcp", map[string]any{"protocol": "vless", "streamSettings": map[string]any{"network": "ws"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := outboundTransportIsUDP(tt.ob); got != tt.want {
				t.Errorf("outboundTransportIsUDP(%v) = %v, want %v", tt.ob, got, tt.want)
			}
		})
	}
}

func TestBuildBatchTestConfigReadsTheProtocolIDLikeTheCore(t *testing.T) {
	items := []*httpBatchItem{
		{tag: "wg", outbound: map[string]any{"tag": "wg", "protocol": "WireGuard"}},
		{tag: "awg", outbound: map[string]any{"tag": "awg", "protocol": "AmneziaWG"}},
	}

	cfg := buildBatchTestConfig(items, nil, []int{61011, 61012})
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	outbounds, _ := m["outbounds"].([]any)
	byTag := make(map[string]map[string]any, len(outbounds))
	for _, entry := range outbounds {
		ob, _ := entry.(map[string]any)
		tag, _ := ob["tag"].(string)
		byTag[tag] = ob
	}

	wg := byTag["wg"]
	if wg == nil {
		t.Fatalf("wg outbound missing from the temp config: %v", outbounds)
	}
	if settings, _ := wg["settings"].(map[string]any); settings == nil || settings["noKernelTun"] != true {
		t.Errorf(`"WireGuard" settings = %v, want noKernelTun: the probe instance must not create a kernel device`, wg["settings"])
	}

	awg := byTag["awg"]
	if awg == nil {
		t.Fatalf("awg outbound missing from the temp config: %v", outbounds)
	}
	if protocol, _ := awg["protocol"].(string); protocol != "socks" {
		t.Errorf(`"AmneziaWG" protocol = %q, want %q: a raw amneziawg entry fails the whole temp config`, protocol, "socks")
	}
}

func TestTestOutboundsTCPLaneReadsProtocolIDCaseInsensitively(t *testing.T) {
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
		"protocol": "SOCKS",
		"settings": map[string]any{"servers": []any{map[string]any{"address": "127.0.0.1", "port": port}}},
	}})
	results, err := (&OutboundService{}).TestOutbounds(batch, "", "", "tcp")
	if err != nil {
		t.Fatalf("TestOutbounds: %v", err)
	}
	r := results[0]
	if !r.Success || r.Mode != "tcp" || len(r.Endpoints) != 1 {
		t.Errorf(`"SOCKS" outbound in tcp mode = %+v, want a successful tcp probe with one endpoint`, r)
	}
}
