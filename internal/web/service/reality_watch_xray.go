package service

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const realityRealTestTimeout = 25 * time.Second

var realityRealTestBinary = xray.GetBinaryPath

// xrayRealityRealTest connects through a throwaway REALITY server (target = target) and client,
// both on loopback, and fetches https://sni/ through them. TLS 1.3 + h2 is not enough: some
// sites pass that check but break REALITY's handshake, which would cut every client.
func xrayRealityRealTest(ctx context.Context, target, sni string) error {
	bin := realityRealTestBinary()
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("xray binary: %w", err)
	}
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	privKey := base64.RawURLEncoding.EncodeToString(priv.Bytes())
	pubKey := base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes())
	id := uuid.NewString()
	serverPort, err := realityTestPort()
	if err != nil {
		return err
	}
	socksPort, err := realityTestPort()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "xui-reality-test-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	server := map[string]any{
		"log": map[string]any{"loglevel": "none"},
		"inbounds": []any{map[string]any{
			"listen": "127.0.0.1", "port": serverPort, "protocol": "vless",
			"settings": map[string]any{"clients": []any{map[string]any{"id": id}}, "decryption": "none"},
			"streamSettings": map[string]any{"network": "tcp", "security": "reality", "realitySettings": map[string]any{
				"target": target, "serverNames": []string{sni}, "privateKey": privKey, "shortIds": []string{"a1"},
			}},
		}},
		"outbounds": []any{map[string]any{"protocol": "freedom"}},
	}
	client := map[string]any{
		"log":      map[string]any{"loglevel": "none"},
		"inbounds": []any{map[string]any{"listen": "127.0.0.1", "port": socksPort, "protocol": "socks", "settings": map[string]any{"udp": false}}},
		"outbounds": []any{map[string]any{
			"protocol": "vless",
			"settings": map[string]any{"vnext": []any{map[string]any{
				"address": "127.0.0.1", "port": serverPort,
				"users": []any{map[string]any{"id": id, "encryption": "none"}},
			}}},
			"streamSettings": map[string]any{"network": "tcp", "security": "reality", "realitySettings": map[string]any{
				"serverName": sni, "fingerprint": "chrome", "publicKey": pubKey, "shortId": "a1",
			}},
		}},
	}

	ctx, cancel := context.WithTimeout(ctx, realityRealTestTimeout)
	defer cancel()
	for name, cfg := range map[string]any{"server.json": server, "client.json": client} {
		b, _ := json.Marshal(cfg)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, b, 0o600); err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, bin, "run", "-c", path)
		if err := cmd.Start(); err != nil {
			return err
		}
		defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	}
	socks := "127.0.0.1:" + strconv.Itoa(socksPort)
	for !portOpen(socks) {
		select {
		case <-ctx.Done():
			return errors.New("throwaway xray did not start")
		case <-time.After(200 * time.Millisecond):
		}
	}
	httpClient := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(&url.URL{Scheme: "socks5", Host: socks})}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+net.JoinHostPort(sni, "443")+"/", nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("no reply through REALITY: %w", err)
	}
	resp.Body.Close()
	return nil
}

func realityTestPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func portOpen(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}
