package wattunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/net/proxy"

	"github.com/mhsanaei/3x-ui/v3/internal/childproc"
)

func startXray(t *testing.T, binary string, config any, port int) {
	t.Helper()
	config.(map[string]any)["log"] = map[string]any{"loglevel": "debug"}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "xray.json")
	if err = os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "run", "-config", path)
	log, err := os.Create(filepath.Join(t.TempDir(), "xray.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() {
		_ = childproc.Stop(cmd, done, time.Second, time.Second)
		_ = log.Close()
		if t.Failed() {
			b, _ := os.ReadFile(log.Name())
			t.Logf("Xray log: %s", b)
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if e == nil {
			_ = c.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("Xray listener did not start")
}

func TestXrayCascadeE2E(t *testing.T) {
	binary, xray := os.Getenv("WATTUN_E2E_BINARY"), os.Getenv("XRAY_E2E_BINARY")
	if binary == "" || xray == "" {
		t.Skip("WATTUN_E2E_BINARY and XRAY_E2E_BINARY required")
	}
	target, _ := echoTargets(t)
	xrayPort, entrancePort, socksPort := freePort(t), freePort(t), freePort(t)
	id := uuid.NewString()
	startXray(t, xray, map[string]any{
		"inbounds":  []any{map[string]any{"listen": "127.0.0.1", "port": xrayPort, "protocol": "vless", "settings": map[string]any{"decryption": "none", "clients": []any{map[string]any{"id": id}}}}},
		"outbounds": []any{map[string]any{"protocol": "freedom", "settings": map[string]any{"finalRules": []any{map[string]any{"action": "allow", "ip": []string{"127.0.0.1"}, "port": fmt.Sprint(target)}}}}},
	}, xrayPort)
	mgr := testManager(t, binary)
	mgr.verify = func(string) error { return nil }
	t.Cleanup(mgr.StopAll)
	cfg := Config{Role: "server", Carrier: "tcp", Endpoint: "127.0.0.1", Port: freePort(t), Bind: "127.0.0.1", Listen: "127.0.0.1", Ports: []Port{{entrancePort, xrayPort, "tcp"}}, Conns: 1}
	server := Spec{GUID: uuid.NewString(), Owner: "test", Revision: 1, Enabled: true, Config: cfg, Secret: "xray-cascade-test-key"}
	client := server
	client.GUID = uuid.NewString()
	client.Config.Role = "client"
	if err := mgr.Apply(context.Background(), server); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, mgr, client)
	startXray(t, xray, map[string]any{
		"inbounds":  []any{map[string]any{"listen": "127.0.0.1", "port": socksPort, "protocol": "socks", "settings": map[string]any{"auth": "noauth"}}},
		"outbounds": []any{map[string]any{"protocol": "vless", "settings": map[string]any{"vnext": []any{map[string]any{"address": "127.0.0.1", "port": entrancePort, "users": []any{map[string]any{"id": id, "encryption": "none"}}}}}}},
	}, socksPort)
	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", socksPort), nil, &net.Dialer{Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dialer.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", target))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err = conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	payload := "ordinary Xray client through WatTunnel to existing VLESS inbound"
	if _, err = io.WriteString(conn, payload); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, len(payload))
	if _, err = io.ReadFull(conn, reply); err != nil {
		t.Fatal(err)
	}
	if string(reply) != payload {
		t.Fatalf("corrupt cascade payload: %q", reply)
	}
}
