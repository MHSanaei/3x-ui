package xray

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// e2eCore is a real xray-core process plus a connected panel API client.
type e2eCore struct {
	api  *XrayAPI
	cmd  *exec.Cmd
	port int
}

// alive reports whether the core is still serving its API port. A gRPC call
// that hands the core an account type its inbound does not expect takes the
// whole process down, so every user probe checks this.
func (c *e2eCore) alive() bool {
	if c.cmd.ProcessState != nil {
		return false
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", c.port), time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// startE2ECore boots xray-core with the given inbounds plus the api inbound the
// panel talks through, and returns a connected client. Skips unless
// XRAY_E2E_BINARY points at an xray built from the version go.mod pins.
func startE2ECore(t *testing.T, inbounds []any) *e2eCore {
	t.Helper()
	bin := os.Getenv("XRAY_E2E_BINARY")
	if bin == "" {
		t.Skip("set XRAY_E2E_BINARY to an xray binary to run this test")
	}

	apiPort := freePort(t)
	all := []any{
		map[string]any{
			"listen":   "127.0.0.1",
			"port":     apiPort,
			"protocol": "tunnel",
			"settings": map[string]any{"rewriteAddress": "127.0.0.1"},
			"tag":      "api",
		},
	}
	all = append(all, inbounds...)

	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"api": map[string]any{
			"services": []string{"HandlerService", "StatsService", "RoutingService"},
			"tag":      "api",
		},
		"inbounds": all,
		"outbounds": []any{
			map[string]any{"protocol": "freedom", "settings": map[string]any{}, "tag": "direct"},
			map[string]any{"protocol": "blackhole", "settings": map[string]any{}, "tag": "blocked"},
		},
		"routing": map[string]any{
			"domainStrategy": "AsIs",
			"rules": []any{
				map[string]any{"type": "field", "inboundTag": []string{"api"}, "outboundTag": "api"},
			},
		},
		"policy": map[string]any{
			"levels": map[string]any{
				"0": map[string]any{"statsUserUplink": true, "statsUserDownlink": true, "statsUserOnline": true},
			},
			"system": map[string]any{"statsInboundUplink": true, "statsInboundDownlink": true},
		},
		"stats": map[string]any{},
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "-c", cfgPath)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start xray: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	waitForPort(t, apiPort)

	api := &XrayAPI{}
	if err := api.Init(apiPort); err != nil {
		t.Fatalf("api init: %v", err)
	}
	t.Cleanup(api.Close)
	return &e2eCore{api: api, cmd: cmd, port: apiPort}
}

// ssKey builds a base64 shadowsocks-2022 key of the given byte length.
func ssKey(n int, seed byte) string {
	raw := make([]byte, n)
	for i := range raw {
		raw[i] = seed + byte(i)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// panelUser mirrors the map shape the panel's client-apply paths hand to
// AddUser: every field is always present, unused ones are empty.
func panelUser(email string, fields map[string]any) map[string]any {
	user := map[string]any{
		"email":        email,
		"id":           "",
		"auth":         "",
		"security":     "",
		"flow":         "",
		"password":     "",
		"cipher":       "",
		"publicKey":    "",
		"allowedIPs":   nil,
		"preSharedKey": "",
		"keepAlive":    "",
	}
	maps.Copy(user, fields)
	return user
}

// TestXrayAPI_E2E_Users runs the panel's add/remove user surface against a live
// core for every protocol it builds accounts for, pinning the error texts
// IsUserExistsErr and IsMissingHandlerErr match on.
func TestXrayAPI_E2E_Users(t *testing.T) {
	c := startE2ECore(t, []any{
		map[string]any{
			"listen": "127.0.0.1", "port": freePort(t), "protocol": "vmess", "tag": "vmess-in",
			"settings": map[string]any{"clients": []any{
				map[string]any{"id": "b831381d-6324-4d53-ad4f-8cda48b30811", "email": "seed-vmess"},
			}},
		},
		map[string]any{
			"listen": "127.0.0.1", "port": freePort(t), "protocol": "vless", "tag": "vless-in",
			"settings": map[string]any{"clients": []any{
				map[string]any{"id": "b831381d-6324-4d53-ad4f-8cda48b30812", "email": "seed-vless"},
			}, "decryption": "none"},
		},
		map[string]any{
			"listen": "127.0.0.1", "port": freePort(t), "protocol": "trojan", "tag": "trojan-in",
			"settings": map[string]any{"clients": []any{
				map[string]any{"password": "seed-pw", "email": "seed-trojan"},
			}},
		},
		map[string]any{
			"listen": "127.0.0.1", "port": freePort(t), "protocol": "shadowsocks", "tag": "ss-in",
			"settings": map[string]any{
				"method": "aes-256-gcm", "network": "tcp,udp",
				"clients": []any{map[string]any{"method": "aes-256-gcm", "password": "seed-pw", "email": "seed-ss"}},
			},
		},
		map[string]any{
			"listen": "127.0.0.1", "port": freePort(t), "protocol": "shadowsocks", "tag": "ss2022-in",
			"settings": map[string]any{
				"method": "2022-blake3-aes-256-gcm", "password": ssKey(32, 1), "network": "tcp,udp",
				"clients": []any{map[string]any{"password": ssKey(32, 9), "email": "seed-ss2022"}},
			},
		},
	})

	tests := []struct {
		name     string
		protocol string
		tag      string
		user     map[string]any
		// rejectsDuplicateEmail is false for the legacy shadowsocks inbound,
		// whose validator does not dedupe emails at all.
		rejectsDuplicateEmail bool
	}{
		{
			name: "vmess", protocol: "vmess", tag: "vmess-in",
			user:                  panelUser("e2e-vmess", map[string]any{"id": "b831381d-6324-4d53-ad4f-8cda48b30821", "security": "auto"}),
			rejectsDuplicateEmail: true,
		},
		{
			name: "vless", protocol: "vless", tag: "vless-in",
			user:                  panelUser("e2e-vless", map[string]any{"id": "b831381d-6324-4d53-ad4f-8cda48b30822", "flow": "xtls-rprx-vision"}),
			rejectsDuplicateEmail: true,
		},
		{
			name: "trojan", protocol: "trojan", tag: "trojan-in",
			user:                  panelUser("e2e-trojan", map[string]any{"password": "e2e-pw"}),
			rejectsDuplicateEmail: true,
		},
		{
			name: "shadowsocks legacy", protocol: "shadowsocks", tag: "ss-in",
			user:                  panelUser("e2e-ss", map[string]any{"password": "e2e-pw", "cipher": "aes-256-gcm"}),
			rejectsDuplicateEmail: false,
		},
		{
			name: "shadowsocks 2022", protocol: "shadowsocks", tag: "ss2022-in",
			user:                  panelUser("e2e-ss2022", map[string]any{"password": ssKey(32, 40), "cipher": "2022-blake3-aes-256-gcm"}),
			rejectsDuplicateEmail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email := tt.user["email"].(string)
			if err := c.api.AddUser(tt.protocol, tt.tag, tt.user); err != nil {
				t.Fatalf("AddUser: %v", err)
			}
			if !c.alive() {
				t.Fatal("core died on AddUser: the account type does not match the running inbound")
			}

			if tt.rejectsDuplicateEmail {
				err := c.api.AddUser(tt.protocol, tt.tag, tt.user)
				if err == nil {
					t.Fatal("adding a user whose email is taken must fail")
				}
				if !IsUserExistsErr(err) {
					t.Fatalf("duplicate-email error not matched by IsUserExistsErr: %q", err)
				}
			}

			if err := c.api.RemoveUser(tt.tag, email); err != nil {
				t.Fatalf("RemoveUser: %v", err)
			}
			err := c.api.RemoveUser(tt.tag, email)
			if err == nil {
				t.Fatal("the user is still registered after RemoveUser")
			}
			if !IsMissingHandlerErr(err) {
				t.Fatalf("missing-user error not matched by IsMissingHandlerErr: %q", err)
			}
		})
	}

	t.Run("unknown inbound tag", func(t *testing.T) {
		err := c.api.AddUser("vmess", "no-such-tag", panelUser("e2e-ghost", map[string]any{"id": "b831381d-6324-4d53-ad4f-8cda48b30899"}))
		if err == nil {
			t.Fatal("AddUser on an unknown tag must fail")
		}
		if !IsMissingHandlerErr(err) {
			t.Fatalf("unknown-tag error not matched by IsMissingHandlerErr: %q", err)
		}
	})
}

// TestXrayAPI_E2E_ShadowsocksAccountTypeGuess is the regression for the crash
// that took the whole core down: a shadowsocks user whose cipher the panel
// could not resolve used to be sent as a shadowsocks-2022 account, which the
// legacy inbound casts without checking. Every case here must leave the core
// running.
func TestXrayAPI_E2E_ShadowsocksAccountTypeGuess(t *testing.T) {
	c := startE2ECore(t, []any{
		map[string]any{
			"listen": "127.0.0.1", "port": freePort(t), "protocol": "shadowsocks", "tag": "ss-in",
			"settings": map[string]any{
				"method": "aes-256-gcm", "network": "tcp,udp",
				"clients": []any{map[string]any{"method": "aes-256-gcm", "password": "seed-pw", "email": "seed-ss"}},
			},
		},
		map[string]any{
			"listen": "127.0.0.1", "port": freePort(t), "protocol": "shadowsocks", "tag": "ss2022-in",
			"settings": map[string]any{
				"method": "2022-blake3-aes-256-gcm", "password": ssKey(32, 1), "network": "tcp,udp",
				"clients": []any{map[string]any{"password": ssKey(32, 9), "email": "seed-ss2022"}},
			},
		},
	})

	// The auto-renew path hands over the client object straight out of the
	// inbound's settings, which carries the cipher under "method".
	t.Run("client object from settings", func(t *testing.T) {
		user := map[string]any{"email": "renew-ss", "password": "pw", "method": "aes-256-gcm", "enable": true}
		if err := c.api.AddUser("shadowsocks", "ss-in", user); err != nil {
			t.Fatalf("AddUser with the settings-shaped client: %v", err)
		}
		if !c.alive() {
			t.Fatal("core died: a legacy shadowsocks client was sent as a 2022 account")
		}
		if err := c.api.RemoveUser("ss-in", "renew-ss"); err != nil {
			t.Fatalf("RemoveUser: %v", err)
		}
	})

	t.Run("cipher missing entirely", func(t *testing.T) {
		err := c.api.AddUser("shadowsocks", "ss-in", map[string]any{"email": "nocipher", "password": "pw"})
		if err == nil {
			t.Fatal("a shadowsocks user with no resolvable cipher must be refused, not guessed")
		}
		if !c.alive() {
			t.Fatal("core died on a shadowsocks user with no cipher")
		}
	})

	t.Run("legacy cipher against a 2022 inbound", func(t *testing.T) {
		// The reverse mismatch is only reachable when the panel's view of the
		// inbound has drifted from the running one; the account type is still
		// the one the cipher names, so the panel never guesses here.
		user := map[string]any{"email": "drift", "password": ssKey(32, 50), "cipher": "2022-blake3-aes-256-gcm"}
		if err := c.api.AddUser("shadowsocks", "ss2022-in", user); err != nil {
			t.Fatalf("AddUser: %v", err)
		}
		if !c.alive() {
			t.Fatal("core died adding a 2022 user to a 2022 inbound")
		}
		if err := c.api.RemoveUser("ss2022-in", "drift"); err != nil {
			t.Fatalf("RemoveUser: %v", err)
		}
	})
}

// TestXrayAPI_E2E_ShadowsocksAddIsIdempotent covers the legacy shadowsocks
// inbound accepting a second user under an email it already holds: one removal
// then left the client connectable. Adding twice must leave exactly one
// registration, so a single removal fully revokes the client.
func TestXrayAPI_E2E_ShadowsocksAddIsIdempotent(t *testing.T) {
	c := startE2ECore(t, []any{
		map[string]any{
			"listen": "127.0.0.1", "port": freePort(t), "protocol": "shadowsocks", "tag": "ss-in",
			"settings": map[string]any{
				"method": "aes-256-gcm", "network": "tcp,udp",
				"clients": []any{map[string]any{"method": "aes-256-gcm", "password": "seed-pw", "email": "seed-ss"}},
			},
		},
	})

	user := map[string]any{"email": "dup-ss", "password": "pw", "cipher": "aes-256-gcm"}
	for i := range 2 {
		if err := c.api.AddUser("shadowsocks", "ss-in", user); err != nil {
			t.Fatalf("AddUser #%d: %v", i+1, err)
		}
	}

	if err := c.api.RemoveUser("ss-in", "dup-ss"); err != nil {
		t.Fatalf("RemoveUser: %v", err)
	}
	err := c.api.RemoveUser("ss-in", "dup-ss")
	if err == nil {
		t.Fatal("a second registration survived removal: a disabled client would keep connecting")
	}
	if !IsMissingHandlerErr(err) {
		t.Fatalf("missing-user error not matched by IsMissingHandlerErr: %q", err)
	}
}

// TestXrayAPI_E2E_NewClientTrafficIsCounted proves against a live core that a
// counter appearing after the baseline poll is reported in full: xray creates a
// user's counter on first use, and dropping that first sample lost a new
// client's traffic for a whole polling interval.
func TestXrayAPI_E2E_NewClientTrafficIsCounted(t *testing.T) {
	const payload = 64 * 1024

	backendLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	backend := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, payload))
	})}
	go func() { _ = backend.Serve(backendLn) }()
	t.Cleanup(func() { _ = backend.Close() })

	proxyPort := freePort(t)
	c := startE2ECore(t, []any{
		map[string]any{
			"listen": "127.0.0.1", "port": proxyPort, "protocol": "http", "tag": "http-in",
			"settings": map[string]any{
				"accounts": []any{map[string]any{"user": "e2e-fresh", "pass": "e2e-pass"}},
			},
		},
	})

	// Baseline poll: the client's counter does not exist yet.
	if _, _, err := c.api.GetTraffic(); err != nil {
		t.Fatalf("GetTraffic (baseline): %v", err)
	}

	proxyURL, err := url.Parse(fmt.Sprintf("http://e2e-fresh:e2e-pass@127.0.0.1:%d", proxyPort))
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	resp, err := client.Get(fmt.Sprintf("http://%s/", backendLn.Addr().String()))
	if err != nil {
		t.Fatalf("proxied request: %v", err)
	}
	n, err := io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("reading the proxied response: %v", err)
	}
	if n != payload {
		t.Fatalf("proxied %d bytes, want %d", n, payload)
	}
	time.Sleep(500 * time.Millisecond)

	_, clients, err := c.api.GetTraffic()
	if err != nil {
		t.Fatalf("GetTraffic: %v", err)
	}
	var got *ClientTraffic
	for _, ct := range clients {
		if ct.Email == "e2e-fresh" {
			got = ct
		}
	}
	if got == nil {
		t.Fatalf("the new client's traffic was dropped; reported clients: %+v", clients)
	}
	if got.Down < payload {
		t.Fatalf("downlink = %d, want at least the %d bytes that were proxied", got.Down, payload)
	}
}

// TestXrayAPI_E2E_TestRoutePortRange keeps TestRoute from wrapping an
// out-of-range port into the uint32 the core is asked about.
func TestXrayAPI_E2E_TestRoutePortRange(t *testing.T) {
	c := startE2ECore(t, nil)

	for _, port := range []int{-1, 65536, 70000} {
		if _, err := c.api.TestRoute(RouteTestRequest{Domain: "example.com", Port: port}); err == nil {
			t.Fatalf("TestRoute accepted the out-of-range port %d", port)
		}
	}
	if _, err := c.api.TestRoute(RouteTestRequest{Domain: "example.com", Port: 65535}); err != nil {
		t.Fatalf("TestRoute rejected the valid port 65535: %v", err)
	}
}
