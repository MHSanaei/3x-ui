package wattunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := ln.Addr().(*net.TCPAddr).Port
	if err = ln.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func echoTargets(t *testing.T) (int, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, e := ln.Accept()
			if e != nil {
				return
			}
			go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
		}
	}()
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = udp.Close() })
	go func() {
		b := make([]byte, 65535)
		for {
			n, a, e := udp.ReadFrom(b)
			if e != nil {
				return
			}
			_, _ = udp.WriteTo(b[:n], a)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, udp.LocalAddr().(*net.UDPAddr).Port
}

func waitConnected(t *testing.T, m *Manager, s Spec) {
	t.Helper()
	// Reverse dialers back off up to 30 seconds while another carrier is selected.
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if err := m.Apply(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if m.Status(s.GUID).State == "connected" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("tunnel did not connect: %+v", m.Status(s.GUID))
}

func assertEcho(t *testing.T, network string, port int) {
	t.Helper()
	conn, err := net.DialTimeout(network, fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err = conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	payload := "real WatTunnel forwarding payload"
	if _, err = io.WriteString(conn, payload); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, len(payload))
	if _, err = io.ReadFull(conn, reply); err != nil {
		t.Fatal(err)
	}
	if string(reply) != payload {
		t.Fatalf("corrupt %s echo: %q", network, reply)
	}
}

func TestManagedProcessE2E(t *testing.T) {
	binary := os.Getenv("WATTUN_E2E_BINARY")
	if binary == "" {
		t.Skip("WATTUN_E2E_BINARY required")
	}
	names := []string{"tcp", "udp", "tls", "wss"}
	if os.Getenv("WATTUN_E2E_ICMP") == "1" {
		if !canRawSocket() {
			t.Fatal("ICMP requested without Linux CAP_NET_RAW")
		}
		names = append(names, "icmp")
	}
	for _, carrier := range names {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reverse=%t", carrier, reverse), func(t *testing.T) {
				tcpTarget, udpTarget := echoTargets(t)
				tcpPort, udpPort := freePort(t), freePort(t)
				mgr := testManager(t, binary)
				mgr.verify = func(string) error { return nil }
				t.Cleanup(mgr.StopAll)
				key, err := NewKey()
				if err != nil {
					t.Fatal(err)
				}
				cfg := Config{Role: "server", Carrier: carrier, Endpoint: "127.0.0.1", ReversePeer: "127.0.0.1", Reverse: reverse, Port: freePort(t), Bind: "127.0.0.1", Listen: "127.0.0.1", Ports: []Port{{tcpPort, tcpTarget, "tcp"}, {udpPort, udpTarget, "udp"}}, Sessions: 0, Conns: 1}
				server := Spec{GUID: uuid.NewString(), Owner: "test", Revision: 1, Enabled: true, Config: cfg, Secret: key}
				client := server
				client.GUID = uuid.NewString()
				client.Config.Role = "client"
				if err = mgr.Apply(context.Background(), server); err != nil {
					t.Fatal(err)
				}
				if err = mgr.Apply(context.Background(), client); err != nil {
					t.Fatal(err)
				}
				waitConnected(t, mgr, client)
				assertEcho(t, "tcp", tcpPort)
				assertEcho(t, "udp", udpPort)
				client.Enabled = false
				client.Revision++
				if err = mgr.Apply(context.Background(), client); err != nil {
					t.Fatal(err)
				}
				ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", tcpPort))
				if err != nil {
					t.Fatalf("stopped child kept port: %v", err)
				}
				ln.Close()
				client.Enabled = true
				client.Revision++
				if err = mgr.Apply(context.Background(), client); err != nil {
					t.Fatal(err)
				}
				waitConnected(t, mgr, client)
				assertEcho(t, "tcp", tcpPort)
			})
		}
	}
}

func TestPrivateKeyringRejectsWrongBinding(t *testing.T) {
	dir := t.TempDir()
	codec, err := SecretCodec(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	ct, err := codec.EncryptBound([]byte("tunnel/first"), "secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = codec.DecryptBound([]byte("tunnel/second"), ct); err == nil {
		t.Fatal("ciphertext usable in a different tunnel")
	}
	codec, err = SecretCodec(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := codec.DecryptBound([]byte("tunnel/first"), ct)
	if err != nil || plain != "secret-value" {
		t.Fatal("persisted keyring cannot decrypt")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, "keyring.json"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("insecure keyring: %v", info.Mode())
		}
	}
	if strings.Contains(ct, "secret-value") {
		t.Fatal("ciphertext leaked plaintext")
	}
}

func TestManagedAutoFailoverAndCrashRecovery(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reverse=%t", reverse), func(t *testing.T) {
			testManagedAutoFailover(t, reverse)
		})
	}
}

func testManagedAutoFailover(t *testing.T, reverse bool) {
	t.Helper()
	binary := os.Getenv("WATTUN_E2E_BINARY")
	if binary == "" {
		t.Skip("WATTUN_E2E_BINARY required")
	}
	target, _ := echoTargets(t)
	port := freePort(t)
	mgr := testManager(t, binary)
	mgr.verify = func(string) error { return nil }
	t.Cleanup(mgr.StopAll)
	offset := time.Duration(0)
	mgr.now = func() time.Time { return time.Now().Add(offset) }
	cfg := Config{Role: "server", Carrier: "auto", Endpoint: "127.0.0.1", ReversePeer: "127.0.0.1", Reverse: reverse, Bind: "127.0.0.1", Listen: "127.0.0.1", Ports: []Port{{port, target, "tcp"}}, Candidates: []CarrierEndpoint{{"udp", freePort(t)}, {"tcp", freePort(t)}}, Conns: 1}
	server := Spec{GUID: uuid.NewString(), Owner: "test", Revision: 1, Enabled: true, Config: cfg, Secret: "auto-test-key"}
	client := server
	client.GUID = uuid.NewString()
	client.Config.Role = "client"
	if err := mgr.Apply(context.Background(), server); err != nil {
		t.Fatal(err)
	}
	t.Log("connect initial Auto transport")
	waitConnected(t, mgr, client)
	assertEcho(t, "tcp", port)
	if got := mgr.Status(client.GUID).Carrier; got != "udp" {
		t.Fatalf("wrong initial carrier: %s", got)
	}
	server.Config.Candidates = []CarrierEndpoint{cfg.Candidates[1]}
	server.Revision++
	if err := mgr.Apply(context.Background(), server); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1200 * time.Millisecond)
	// The engine's last-authenticated-packet health window is ten seconds.
	time.Sleep(10 * time.Second)
	if err := mgr.Apply(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	offset += 46 * time.Second
	if err := mgr.Apply(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	offset += 2 * time.Second
	t.Log("connect after transport failover")
	waitConnected(t, mgr, client)
	if got := mgr.Status(client.GUID).Carrier; got != "tcp" {
		t.Fatalf("did not fail over to TCP: %s", got)
	}
	assertEcho(t, "tcp", port)
	mgr.mu.Lock()
	cmd, done := mgr.instances[client.GUID].cmd, mgr.instances[client.GUID].done
	mgr.mu.Unlock()
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("crashed process not reaped")
	}
	if err := mgr.Apply(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	offset += 70 * time.Second
	// Crash retry cycles Auto; force another health timeout if UDP remains unavailable.
	if err := mgr.Apply(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	offset += 46 * time.Second
	if err := mgr.Apply(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	offset += 2 * time.Second
	t.Log("connect after client crash")
	waitConnected(t, mgr, client)
	assertEcho(t, "tcp", port)
	mgr.StopAll()
	if err := mgr.Apply(context.Background(), server); err != nil {
		t.Fatal(err)
	}
	t.Log("connect after supervisor restart")
	waitConnected(t, mgr, client)
	if mgr.Status(client.GUID).Carrier != "tcp" {
		t.Fatal("supervisor restart forgot the last working carrier")
	}
	assertEcho(t, "tcp", port)
}

func testManager(t *testing.T, binary string) *Manager {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "linux" && os.Geteuid() == 0 {
		if err := os.Chmod(filepath.Dir(dir), 0o711); err != nil {
			t.Fatal(err)
		}
	}
	return NewManager(binary, dir)
}

func TestManagedRejectsWrongKeyAndUnavailablePeer(t *testing.T) {
	binary := os.Getenv("WATTUN_E2E_BINARY")
	if binary == "" {
		t.Skip("WATTUN_E2E_BINARY required")
	}
	target, _ := echoTargets(t)
	mgr := testManager(t, binary)
	mgr.verify = func(string) error { return nil }
	t.Cleanup(mgr.StopAll)
	cfg := Config{Role: "server", Carrier: "tcp", Endpoint: "127.0.0.1", Port: freePort(t), Bind: "127.0.0.1", Listen: "127.0.0.1", Ports: []Port{{freePort(t), target, "tcp"}}, Conns: 1}
	server := Spec{GUID: uuid.NewString(), Owner: "test", Revision: 1, Enabled: true, Config: cfg, Secret: "correct-key"}
	client := server
	client.GUID = uuid.NewString()
	client.Config.Role = "client"
	client.Secret = "wrong-key"
	if err := mgr.Apply(context.Background(), server); err != nil {
		t.Fatal(err)
	}
	for _, unreachable := range []bool{false, true} {
		if unreachable {
			client.Config.Port = freePort(t)
			client.Revision++
		}
		if err := mgr.Apply(context.Background(), client); err != nil {
			t.Fatal(err)
		}
		time.Sleep(300 * time.Millisecond)
		if err := mgr.Apply(context.Background(), client); err != nil {
			t.Fatal(err)
		}
		status := mgr.Status(client.GUID)
		if status.State == "connected" || status.Metrics != nil && status.Metrics.Alive {
			t.Fatalf("unauthenticated peer reported connected: %+v", status)
		}
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.Ports[0].Listen), time.Second)
		if err == nil {
			_ = conn.SetDeadline(time.Now().Add(300 * time.Millisecond))
			_, _ = conn.Write([]byte("must-not-forward"))
			b := make([]byte, 16)
			n, _ := conn.Read(b)
			_ = conn.Close()
			if n > 0 {
				t.Fatal("wrong-key or absent peer forwarded traffic")
			}
		}
	}
}

func TestManagedReportsRealBindConflict(t *testing.T) {
	binary := os.Getenv("WATTUN_E2E_BINARY")
	if binary == "" {
		t.Skip("WATTUN_E2E_BINARY required")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	mgr := testManager(t, binary)
	mgr.verify = func(string) error { return nil }
	t.Cleanup(mgr.StopAll)
	cfg := Config{Role: "server", Carrier: "tcp", Endpoint: "127.0.0.1", Listen: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Ports: []Port{{443, 443, "tcp"}}}
	spec := Spec{GUID: uuid.NewString(), Owner: "test", Revision: 1, Enabled: true, Config: cfg, Secret: "bind-secret"}
	if err = mgr.Apply(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_ = mgr.Apply(context.Background(), spec)
		status := mgr.Status(spec.GUID)
		if status.State == "error" {
			if !strings.Contains(status.Reason, "address already in use") || strings.Contains(status.Reason, spec.Secret) {
				t.Fatalf("unsafe or missing bind diagnostic: %+v", status)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("real bind conflict did not become a runtime error")
}
