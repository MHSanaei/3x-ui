package xray

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestXrayAPI_E2E_ActiveProfiles exercises actual VLESS and QUIC/Hysteria2
// tunnels, not an IP/traffic heuristic. All listeners and traffic stay local.
func TestXrayAPI_E2E_ActiveProfiles(t *testing.T) {
	if os.Getenv("XRAY_E2E_BINARY") == "" {
		t.Skip("set XRAY_E2E_BINARY")
	}
	const aliceID = "b831381d-6324-4d53-ad4f-8cda48b30831"
	const bobID = "b831381d-6324-4d53-ad4f-8cda48b30832"
	const auth = "local-hysteria-test-only"
	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { backend.Close() })
	go func() {
		for {
			conn, err := backend.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				buffer := make([]byte, 4096)
				for {
					n, err := conn.Read(buffer)
					if n > 0 {
						if _, writeErr := conn.Write(buffer[:n]); writeErr != nil {
							return
						}
					}
					if err != nil {
						return
					}
				}
			}()
		}
	}()
	_, portText, _ := net.SplitHostPort(backend.Addr().String())
	var backendPort int
	fmt.Sscan(portText, &backendPort)
	firstPort, secondPort, hyPort, proxyPort := freePort(t), freePort(t), freePort(t), freePort(t)
	cert, key := activeProfileTestCert(t)
	vless := func(port int, tag string) any {
		return map[string]any{
			"listen": "127.0.0.1", "port": port, "protocol": "vless", "tag": tag,
			"settings": map[string]any{"decryption": "none", "clients": []any{map[string]any{"id": aliceID, "email": "alice"}, map[string]any{"id": bobID, "email": "bob"}}},
		}
	}
	c := startE2ECore(t, []any{
		vless(firstPort, "vless-one"), vless(secondPort, "vless-two"),
		map[string]any{"listen": "127.0.0.2", "port": hyPort, "protocol": "hysteria", "tag": "hy-profile",
			"settings":       map[string]any{"version": 2, "clients": []any{map[string]any{"auth": auth, "email": "alice"}}},
			"streamSettings": map[string]any{"network": "hysteria", "security": "tls", "tlsSettings": map[string]any{"alpn": []string{"h3"}, "certificates": []any{map[string]any{"certificateFile": cert, "keyFile": key}}}},
		},
		map[string]any{"listen": "127.0.0.1", "port": proxyPort, "protocol": "http", "tag": "hy-test-client", "settings": map[string]any{}},
	})
	outbound, _ := json.Marshal(map[string]any{
		"tag": "hy-test-out", "protocol": "hysteria", "sendThrough": "127.0.0.2",
		"settings": map[string]any{"version": 2, "address": "127.0.0.2", "port": hyPort},
		"streamSettings": map[string]any{"network": "hysteria", "security": "tls", "hysteriaSettings": map[string]any{"version": 2, "auth": auth},
			"tlsSettings": map[string]any{"serverName": "xui.test", "alpn": []string{"h3"}, "certificates": []any{map[string]any{"usage": "verify", "certificateFile": cert}}}},
	})
	if err := c.api.AddOutbound(outbound); err != nil {
		t.Fatal(err)
	}
	// New cores block private destinations for VPN protocols by default. Allow
	// only the local fixture explicitly, without changing production defaults.
	if err := c.api.AddOutbound([]byte(`{"tag":"local-test-out","protocol":"freedom","settings":{"finalRules":[{"action":"allow","ip":["127.0.0.0/8"]}]}}`)); err != nil {
		t.Fatal(err)
	}
	if err := c.api.ApplyRoutingConfig([]byte(`{"domainStrategy":"AsIs","rules":[{"type":"field","inboundTag":["api"],"outboundTag":"api"},{"type":"field","inboundTag":["hy-test-client"],"outboundTag":"hy-test-out"},{"type":"field","inboundTag":["vless-one","vless-two","hy-profile"],"outboundTag":"local-test-out"}]}`)); err != nil {
		t.Fatal(err)
	}
	dial := func(port int) net.Conn {
		t.Helper()
		d := net.Dialer{Timeout: 5 * time.Second, LocalAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.2")}}
		conn, err := d.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		conn.SetDeadline(time.Now().Add(8 * time.Second))
		return conn
	}
	vlessDial := func(port int, id string) net.Conn {
		t.Helper()
		conn := dial(port)
		uuid, _ := hex.DecodeString(strings.ReplaceAll(id, "-", ""))
		header := append([]byte{0}, uuid...)
		header = append(header, 0, 1, byte(backendPort>>8), byte(backendPort), 1, 127, 0, 0, 1)
		if _, err := conn.Write(append(header, []byte("alive")...)); err != nil {
			t.Fatal(err)
		}
		response := make([]byte, 7)
		if _, err := io.ReadFull(conn, response); err != nil {
			t.Fatal(err)
		}
		if string(response[2:]) != "alive" {
			t.Fatalf("VLESS echo: %q", response)
		}
		conn.SetDeadline(time.Time{})
		return conn
	}
	first := vlessDial(firstPort, aliceID)
	bob := vlessDial(secondPort, bobID)
	hy := dial(proxyPort)
	if _, err := fmt.Fprintf(hy, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", backend.Addr(), backend.Addr()); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(hy)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "200") {
		t.Fatalf("Hysteria CONNECT failed: %q", status)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	if _, err := hy.Write([]byte("alive")); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, 5)
	if _, err := io.ReadFull(reader, echo); err != nil {
		t.Fatal(err)
	}
	if string(echo) != "alive" {
		t.Fatalf("Hysteria echo: %q", echo)
	}
	hy.SetDeadline(time.Time{})
	wait := func(email string, want []string) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for {
			users, err := c.api.GetOnlineUsers()
			if err != nil {
				t.Fatal(err)
			}
			if !c.api.OnlineInboundTracking {
				t.Fatal("built core does not advertise exact inbound tracking")
			}
			var got []string
			for _, u := range users {
				if u.Email == email {
					got = u.InboundTags
				}
			}
			if slices.Equal(got, want) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s active profiles = %v, want %v", email, got, want)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	wait("alice", []string{"hy-profile", "vless-one"})
	wait("bob", []string{"vless-two"})
	// No bytes move during this interval: an idle open tunnel must stay visible.
	time.Sleep(200 * time.Millisecond)
	wait("alice", []string{"hy-profile", "vless-one"})
	second := vlessDial(secondPort, aliceID)
	wait("alice", []string{"hy-profile", "vless-one", "vless-two"})
	first.Close()
	wait("alice", []string{"hy-profile", "vless-two"})
	second.Close()
	wait("alice", []string{"hy-profile"})
	hy.Close()
	wait("alice", nil)
	wait("bob", []string{"vless-two"})
	bob.Close()
	wait("bob", nil)
}

func activeProfileTestCert(t *testing.T) (string, string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "xui.test"}, DNSNames: []string{"xui.test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}
