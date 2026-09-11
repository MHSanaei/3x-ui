package tuic

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnsureFrontsSidecarWithRelayAndRemoveReleasesPort(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as the sidecar binary")
	}
	bin := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", bin)
	if err := os.WriteFile(filepath.Join(bin, GetBinaryName()), []byte("#!/bin/sh\nexec sleep 300\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	port, err := freeLoopbackUDPPort()
	if err != nil {
		t.Fatal(err)
	}
	inst := Instance{
		Id: 7, Tag: "tuic-7", Listen: "127.0.0.1", Port: port,
		Clients: []TuicClientSettings{{UUID: "u", Password: "p", Email: "e"}},
	}
	m := &Manager{procs: map[int]*managed{}, lastStartErr: map[int]string{}}
	t.Cleanup(m.StopAll)

	if err := m.Ensure(inst); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}); err == nil {
		_ = c.Close()
		t.Fatal("the relay must own the inbound's public port while the sidecar runs")
	}
	raw, err := os.ReadFile(ConfigPathForID(7))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Server string `json:"server"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	host, sidecarPort, err := net.SplitHostPort(cfg.Server)
	if err != nil || host != "127.0.0.1" || sidecarPort == "" || cfg.Server == inst.BindTo() {
		t.Fatalf("sidecar bound to %q, want a loopback port other than the public %q", cfg.Server, inst.BindTo())
	}

	m.Remove(7)
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		t.Fatalf("public port still held after Remove: %v", err)
	}
	_ = c.Close()
}
