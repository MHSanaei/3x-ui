package outbound

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"
)

func TestExtractOutboundEndpointsVLESS(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		want     []string
	}{
		{
			name: "vnext endpoints",
			settings: map[string]any{
				"vnext": []any{
					map[string]any{"address": "first.example.com", "port": float64(443)},
					map[string]any{"address": "second.example.com", "port": float64(8443)},
				},
			},
			want: []string{"first.example.com:443", "second.example.com:8443"},
		},
		{
			name: "flat endpoint",
			settings: map[string]any{
				"address": "legacy.example.com",
				"port":    float64(443),
			},
			want: []string{"legacy.example.com:443"},
		},
		{
			name: "invalid vnext falls back to flat endpoint",
			settings: map[string]any{
				"vnext":   []any{map[string]any{"address": "missing-port.example.com"}},
				"address": "fallback.example.com",
				"port":    float64(2053),
			},
			want: []string{"fallback.example.com:2053"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOutboundEndpoints(map[string]any{
				"protocol": "vless",
				"settings": tt.settings,
			})
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("extractOutboundEndpoints() = %v, want %v", got, tt.want)
			}
		})
	}
}

// A MASQUE outbound names its server flat in settings, like hysteria; without it the
// outbound test has no endpoint to resolve and reports the outbound unreachable.
func TestExtractOutboundEndpointsMasque(t *testing.T) {
	got := extractOutboundEndpoints(map[string]any{
		"protocol": "masque",
		"settings": map[string]any{"address": "masque.example.com", "port": float64(8443)},
	})
	if want := []string{"masque.example.com:8443"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("extractOutboundEndpoints() = %v, want %v", got, want)
	}
}

func endpointOutbound(tag, protocol, host string, port int, flat bool) map[string]any {
	server := map[string]any{"address": host, "port": port}
	var settings map[string]any
	switch {
	case flat:
		settings = server
	case protocol == "vless" || protocol == "vmess":
		settings = map[string]any{"vnext": []any{server}}
	default:
		settings = map[string]any{"servers": []any{server}}
	}
	return map[string]any{"tag": tag, "protocol": protocol, "settings": settings}
}

func TestOutboundEndpointsIPv6DialAddresses(t *testing.T) {
	tests := []struct {
		protocol string
		host     string
		wantHost string
		flat     bool
	}{
		{"vless", "2001:3040:1:100::301:2020", "2001:3040:1:100::301:2020", false},
		{"vless", "[2001:3040:1:100::301:2020]", "2001:3040:1:100::301:2020", false},
		{"vmess", "::1", "::1", false},
		{"trojan", "[::1]", "::1", false},
		{"shadowsocks", "192.0.2.1", "192.0.2.1", false},
		{"socks", "proxy.example.com", "proxy.example.com", false},
		{"http", "::1", "::1", false},
		{"hysteria", "::1", "::1", true},
		{"masque", "[::1]", "::1", true},
		{"vless", "fe80::1%eth0", "fe80::1%eth0", true},
		{"vless", "[fe80::1%eth0]", "fe80::1%eth0", true},
	}
	for _, tt := range tests {
		t.Run(tt.protocol+"/"+tt.host, func(t *testing.T) {
			ob := endpointOutbound("probe", tt.protocol, tt.host, 8443, tt.flat)
			endpoints := extractOutboundEndpoints(ob)
			if len(endpoints) != 1 {
				t.Fatalf("endpoints = %v, want one dial address", endpoints)
			}
			gotHost, gotPort, err := net.SplitHostPort(endpoints[0])
			if err != nil {
				t.Fatalf("endpoint %q cannot be dialed: %v", endpoints[0], err)
			}
			if gotHost != tt.wantHost || gotPort != "8443" {
				t.Fatalf("dial target = (%q, %q), want (%q, 8443)", gotHost, gotPort, tt.wantHost)
			}
		})
	}
}

func TestOutboundTCPIPv6ReachesListener(t *testing.T) {
	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	for _, tc := range []struct {
		protocol string
		flat     bool
	}{
		{protocol: "vless"},
		{protocol: "vless", flat: true},
		{protocol: "vmess"},
		{protocol: "socks"},
	} {
		t.Run(fmt.Sprintf("%s/flat=%t", tc.protocol, tc.flat), func(t *testing.T) {
			items := []any{
				endpointOutbound("raw", tc.protocol, "::1", port, tc.flat),
				endpointOutbound("bracketed", tc.protocol, "[::1]", port, tc.flat),
			}
			svc := &OutboundService{}
			for _, ob := range items {
				t.Run(ob.(map[string]any)["tag"].(string), func(t *testing.T) {
					r, err := svc.TestOutbound(mustJSON(t, ob), "", "", "tcp")
					if err != nil {
						t.Fatal(err)
					}
					assertTCPListenerResult(t, r, listener.Addr().String())
				})
			}
			t.Run("batch", func(t *testing.T) {
				results, err := svc.TestOutbounds(mustJSON(t, items), "", "", "tcp")
				if err != nil {
					t.Fatal(err)
				}
				if len(results) != 2 || results[0].Tag != "raw" || results[1].Tag != "bracketed" {
					t.Fatalf("batch results lost input ordering: %+v", results)
				}
				for _, r := range results {
					assertTCPListenerResult(t, r, listener.Addr().String())
				}
			})
		})
	}
}

func assertTCPListenerResult(t *testing.T, r *TestOutboundResult, address string) {
	t.Helper()
	if !r.Success || r.Error != "" || r.Mode != "tcp" || len(r.Endpoints) != 1 {
		t.Fatalf("TCP probe failed to reach %s: %+v", address, r)
	}
	endpoint := r.Endpoints[0]
	if !endpoint.Success || endpoint.Error != "" || endpoint.Address != address {
		t.Fatalf("endpoint = %+v, want successful dial to %s", endpoint, address)
	}
}

func TestOutboundTCPReportsRefusedIPv6Connection(t *testing.T) {
	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	address := listener.Addr().String()
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	for _, host := range []string{"::1", "[::1]"} {
		t.Run(host, func(t *testing.T) {
			ob := endpointOutbound("closed", "vless", host, port, false)
			r, err := (&OutboundService{}).TestOutbound(mustJSON(t, ob), "", "", "tcp")
			if err != nil {
				t.Fatal(err)
			}
			if r.Success || len(r.Endpoints) != 1 || r.Endpoints[0].Success || r.Endpoints[0].Address != address || !strings.Contains(r.Error, "connectex:") && !strings.Contains(r.Error, "connection refused") {
				t.Fatalf("closed IPv6 listener did not produce a connection error: %+v", r)
			}
		})
	}
}
