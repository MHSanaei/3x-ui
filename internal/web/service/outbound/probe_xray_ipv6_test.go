package outbound

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const probeClientID = "a17e367c-2074-4d3e-aaeb-fbef5dfde7e7"

func TestOutboundProbesIPv6ThroughXray(t *testing.T) {
	installProbeXray(t)
	withEgressTraceProbe(t, func(*url.URL) *TestEgressResult { return nil })
	var requests atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	var inbounds []any
	var items []any
	var readyPorts []int
	cert, key := probeCertificate(t)
	for _, network := range []string{"raw", "ws", "kcp"} {
		listenNetwork := "tcp6"
		if network == "kcp" {
			listenNetwork = "udp6"
		}
		port := probeServerPort(t, listenNetwork, "[::1]:0")
		serverStream := map[string]any{"network": network}
		clientStream := map[string]any{"network": network}
		if network == "ws" {
			serverStream["security"] = "tls"
			serverStream["tlsSettings"] = map[string]any{"certificates": []any{map[string]any{"certificate": cert, "key": key}}, "rejectUnknownSni": true}
			serverStream["wsSettings"] = map[string]any{"path": "/probe/[keep]"}
			clientStream["security"] = "tls"
			clientStream["tlsSettings"] = map[string]any{
				"serverName": "proxy.test", "disableSystemRoot": true,
				"certificates": []any{map[string]any{"certificate": cert, "usage": "verify"}},
			}
			clientStream["wsSettings"] = map[string]any{"path": "/probe/[keep]", "host": "proxy.test"}
		}
		inbounds = append(inbounds, map[string]any{
			"tag": "server-" + network, "listen": "::1", "port": port, "protocol": "vless",
			"settings":       map[string]any{"clients": []any{map[string]any{"id": probeClientID}}, "decryption": "none"},
			"streamSettings": serverStream,
		})
		for _, host := range []string{"::1", "[::1]"} {
			ob := probeVLESSOutbound(network+"-"+host, host, port)
			ob["streamSettings"] = clientStream
			items = append(items, ob)
		}
	}
	vlessPort := inbounds[0].(map[string]any)["port"].(int)
	socksPort := probeServerPort(t, "tcp6", "[::1]:0")
	inbounds = append(inbounds, map[string]any{
		"tag": "server-socks", "listen": "::1", "port": socksPort, "protocol": "socks",
		"settings": map[string]any{"auth": "noauth"},
	})
	for _, host := range []string{"::1", "[::1]"} {
		items = append(items, endpointOutbound("socks-"+host, "socks", host, socksPort, false))
	}
	ipv4Port := probeServerPort(t, "tcp4", "127.0.0.1:0")
	inbounds = append(inbounds, map[string]any{
		"tag": "server-ipv4", "listen": "127.0.0.1", "port": ipv4Port, "protocol": "vless",
		"settings": map[string]any{"clients": []any{map[string]any{"id": probeClientID}}, "decryption": "none"},
	})
	readyPorts = append(readyPorts, ipv4Port)
	items = append(items, probeVLESSOutbound("ipv4", "127.0.0.1", ipv4Port), probeVLESSOutbound("dns", "localhost", ipv4Port))
	hop := endpointOutbound("hop", "socks", "[::1]", socksPort, false)
	chain := probeVLESSOutbound("chain", "::1", vlessPort)
	chain["streamSettings"] = map[string]any{"sockopt": map[string]any{"dialerProxy": "hop"}}
	items = append(items, chain)
	startProbeServer(t, inbounds, readyPorts)
	contextJSON := mustJSON(t, []any{hop})

	for _, mode := range []string{"http", "real"} {
		t.Run(mode, func(t *testing.T) {
			wantRequests := int64(1)
			if mode == "http" {
				wantRequests = 2
			}
			for _, host := range []string{"::1", "[::1]"} {
				t.Run(host, func(t *testing.T) {
					before := requests.Load()
					r, err := (&OutboundService{}).TestOutbound(mustJSON(t, probeVLESSOutbound("single", host, vlessPort)), target.URL, "", mode)
					if err != nil {
						t.Fatal(err)
					}
					assertHTTPProbeResult(t, r, mode)
					if got := requests.Load() - before; got != wantRequests {
						t.Fatalf("single %s sent %d requests, want %d", host, got, wantRequests)
					}
				})
			}
			before := requests.Load()
			results, err := (&OutboundService{}).TestOutbounds(mustJSON(t, items), target.URL, contextJSON, mode)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != len(items) {
				t.Fatalf("batch returned %d results, want %d", len(results), len(items))
			}
			for i, r := range results {
				if r.Tag != items[i].(map[string]any)["tag"] {
					t.Fatalf("batch result %d lost its outbound: %+v", i, r)
				}
				assertHTTPProbeResult(t, r, mode)
			}
			if got := requests.Load() - before; got != int64(len(items))*wantRequests {
				t.Fatalf("batch sent %d requests, want %d", got, int64(len(items))*wantRequests)
			}
		})
	}

	t.Run("tcp mode uses real UDP handshake", func(t *testing.T) {
		r, err := (&OutboundService{}).TestOutbound(mustJSON(t, items[4]), target.URL, "", "tcp")
		if err != nil {
			t.Fatal(err)
		}
		assertHTTPProbeResult(t, r, "http")
	})
	t.Run("unreachable and malformed remain failed", func(t *testing.T) {
		closedPort := probeServerPort(t, "tcp6", "[::1]:0")
		for _, mode := range []string{"http", "real"} {
			bad := []any{
				probeVLESSOutbound("closed-raw", "::1", closedPort),
				probeVLESSOutbound("closed-bracketed", "[::1]", closedPort),
				probeVLESSOutbound("malformed", "[[::1]]", vlessPort),
			}
			before := requests.Load()
			results, err := (&OutboundService{}).TestOutbounds(mustJSON(t, bad), target.URL, "", mode)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range results {
				wantError := "Get \"" + target.URL + "\": "
				if r.Tag == "malformed" {
					wantError = "Xray process exited: Failed to start:"
				}
				if r.Success || r.HTTPStatus != 0 || r.Mode != mode || !strings.HasPrefix(r.Error, wantError) {
					t.Errorf("unreachable outbound reported incorrectly: %+v", r)
				}
			}
			if requests.Load() != before {
				t.Fatal("failed outbounds unexpectedly reached the HTTP target")
			}
		}
	})
}

func probeVLESSOutbound(tag, host string, port int) map[string]any {
	return map[string]any{
		"tag": tag, "protocol": "vless",
		"settings": map[string]any{"address": host, "port": port, "id": probeClientID, "encryption": "none"},
	}
}

func assertHTTPProbeResult(t *testing.T, r *TestOutboundResult, mode string) {
	t.Helper()
	if !r.Success || r.Error != "" || r.HTTPStatus != http.StatusNoContent || r.Mode != mode || r.Delay < 1 {
		t.Fatalf("%s probe failed: %+v", mode, r)
	}
}

func installProbeXray(t *testing.T) {
	t.Helper()
	binary := os.Getenv("XRAY_E2E_BINARY")
	if binary == "" {
		t.Skip("set XRAY_E2E_BINARY to an xray binary built from the version in go.mod")
	}
	folder := t.TempDir()
	name := xray.GetBinaryName()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	in, err := os.Open(binary)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(filepath.Join(folder, name), os.O_CREATE|os.O_WRONLY, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("copy xray binary: %v; close: %v", copyErr, closeErr)
	}
	t.Setenv("XUI_BIN_FOLDER", folder)
	t.Setenv("XUI_LOG_FOLDER", folder)
}

func probeServerPort(t *testing.T, network, address string) int {
	t.Helper()
	if network == "udp6" {
		listener, err := net.ListenPacket(network, address)
		if err != nil {
			t.Skipf("IPv6 UDP loopback unavailable: %v", err)
		}
		defer listener.Close()
		return listener.LocalAddr().(*net.UDPAddr).Port
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		if network == "tcp6" {
			t.Skipf("IPv6 loopback unavailable: %v", err)
		}
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func startProbeServer(t *testing.T, inbounds []any, readyPorts []int) {
	t.Helper()
	raw := mustJSON(t, map[string]any{
		"log": map[string]any{"loglevel": "warning"}, "inbounds": inbounds,
		"outbounds": []any{map[string]any{
			"protocol": "freedom", "tag": "direct",
			"settings": map[string]any{"finalRules": []any{map[string]any{"action": "allow", "ip": []string{"127.0.0.1/32", "::1/128"}}}},
		}},
	})
	var cfg xray.Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	path, err := createTestConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	process := xray.NewTestProcess(&cfg, path)
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = process.Stop()
		runtime.KeepAlive(process)
	})
	if err := waitForPortsReady(process, readyPorts, batchPortsReadyTimeout); err != nil {
		t.Fatal(err)
	}
}

func probeCertificate(t *testing.T) ([]string, []string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "proxy.test"}, DNSNames: []string{"proxy.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))), "\n"),
		strings.Split(strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))), "\n")
}
