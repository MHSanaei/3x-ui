package outbound

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The core lowercases a protocol id (infra/conf/loader.go) and a transport name
// (TransportProtocol.Build) before it resolves either, and resolves both "kcp"
// and "mkcp" to mKCP. Both readers below have to agree, or a template the core
// is running gets probed as if it were a different protocol.

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
