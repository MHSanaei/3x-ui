package tor

import (
	"net"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRenderTorrc(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", "testbin")
	content := renderTorrc()
	for _, want := range []string{
		"SocksPort 127.0.0.1:19050",
		"ClientOnly 1",
		"RunAsDaemon 0",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("renderTorrc() missing %q, got:\n%s", want, content)
		}
	}
	if strings.Contains(strings.ToLower(content), "orport") {
		t.Fatal("renderTorrc() must never set an ORPort -- this daemon must stay client-only")
	}
}

func TestWriteTorrcDataDirPermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)

	if err := writeTorrc(); err != nil {
		t.Fatalf("writeTorrc: %v", err)
	}
	if _, err := os.Stat(torrcPath()); err != nil {
		t.Fatalf("torrc not written: %v", err)
	}
	info, err := os.Stat(dataDir())
	if err != nil {
		t.Fatalf("data dir not created: %v", err)
	}
	if runtime.GOOS != "windows" {
		// Tor refuses to start against a group/world-readable DataDirectory.
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Fatalf("data dir mode = %o, want 0700", perm)
		}
	}
}

func TestManagerStartWithoutBinary(t *testing.T) {
	if IsAvailable() {
		t.Skip("a real tor binary is on PATH -- covered by TestManagerRealProcessLifecycle instead")
	}
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	m := &Manager{}

	if err := m.Start(); err == nil {
		t.Fatal("Start() with no tor binary on PATH: want error, got nil")
	}
	if m.IsRunning() {
		t.Fatal("IsRunning() after a failed Start(): want false")
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() when never started: want nil (idempotent no-op), got %v", err)
	}
}

// TestManagerRealProcessLifecycle exercises a real tor daemon end to end --
// only runs on a machine that actually has one on PATH (a real Linux box,
// not this project's usual Windows dev environment), so it is a genuine
// regression test wherever it can run rather than a permanently-skipped one.
func TestManagerRealProcessLifecycle(t *testing.T) {
	if !IsAvailable() {
		t.Skip("tor binary not found on PATH")
	}
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	m := &Manager{}
	t.Cleanup(func() { _ = m.Stop() })

	if err := m.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := m.Start(); err != nil {
		t.Fatalf("Start while already running should be a no-op, got: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	var dialErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:19050", time.Second)
		if err == nil {
			_ = conn.Close()
			dialErr = nil
			break
		}
		dialErr = err
		time.Sleep(200 * time.Millisecond)
	}
	if dialErr != nil {
		t.Fatalf("SOCKS port never came up: %v (last log: %s)", dialErr, m.LastResult())
	}
	if !m.IsRunning() {
		t.Fatal("IsRunning() while the SOCKS port is accepting connections: want true")
	}

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if m.IsRunning() {
		t.Fatal("IsRunning() after Stop(): want false")
	}
}
