//go:build !windows

package xray

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/op/go-logging"
)

func TestWriteFileAtomicModeAndRenameFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := writeFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("content = %q, want new", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}

	originalRename := renameFile
	renameFile = func(_, _ string) error { return errors.New("injected rename failure") }
	t.Cleanup(func() { renameFile = originalRename })
	if err := writeFileAtomic(path, []byte("partial"), 0o600); err == nil {
		t.Fatal("rename failure = nil")
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read preserved file: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("content after failed rename = %q, want committed content", data)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".config-*.tmp"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files leaked: %v", matches)
	}
}

func TestStopWaitsForGracefulExit(t *testing.T) {
	initProcessTestLogger(t)

	p := startProcessHelper(t, "delayed-term")

	start := time.Now()
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
		t.Fatalf("Stop returned before child exited; elapsed=%s", elapsed)
	}
	if p.IsRunning() {
		t.Fatal("process still reports running after Stop")
	}
}

func TestIntentionalStopDoesNotRecordExitError(t *testing.T) {
	initProcessTestLogger(t)

	p := startProcessHelper(t, "default-term")

	if err := p.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := p.GetErr(); err != nil {
		t.Fatalf("GetErr after intentional stop = %v, want nil", err)
	}
	if result := p.GetResult(); result != "" {
		t.Fatalf("GetResult after intentional stop = %q, want empty", result)
	}
}

func TestStopKillsProcessThatIgnoresSIGTERM(t *testing.T) {
	initProcessTestLogger(t)

	oldGraceful := xrayGracefulStopTimeout
	oldForce := xrayForceStopTimeout
	xrayGracefulStopTimeout = 100 * time.Millisecond
	xrayForceStopTimeout = 2 * time.Second
	t.Cleanup(func() {
		xrayGracefulStopTimeout = oldGraceful
		xrayForceStopTimeout = oldForce
	})

	p := startProcessHelper(t, "ignore-term")

	if err := p.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if p.IsRunning() {
		t.Fatal("process still reports running after forced stop")
	}
}

func initProcessTestLogger(t *testing.T) {
	t.Helper()
	t.Setenv("XUI_LOG_FOLDER", t.TempDir())
	xuilogger.InitLogger(logging.ERROR)
}

func startProcessHelper(t *testing.T, mode string) *process {
	t.Helper()

	readyPath := filepath.Join(t.TempDir(), "ready")
	cmd := exec.Command(os.Args[0], "-test.run=TestXrayProcessHelper", "--", mode)
	cmd.Env = append(os.Environ(),
		"XRAY_PROCESS_HELPER=1",
		"XRAY_PROCESS_READY="+readyPath,
	)

	p := newProcess(nil)
	if err := p.startCommand(cmd); err != nil {
		t.Fatalf("start helper process: %v", err)
	}
	waitForProcessHelperReady(t, readyPath)

	t.Cleanup(func() {
		if p.IsRunning() {
			p.intentionalStop.Store(true)
			_ = p.cmd.Process.Kill()
			_ = p.waitForExit(2 * time.Second)
		}
	})

	return p
}

func waitForProcessHelperReady(t *testing.T, readyPath string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(readyPath); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("helper process did not become ready")
}

func TestXrayProcessHelper(t *testing.T) {
	if os.Getenv("XRAY_PROCESS_HELPER") != "1" {
		return
	}

	mode := ""
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
			break
		}
	}

	switch mode {
	case "delayed-term":
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM)
		markProcessHelperReady(t)
		<-sigCh
		time.Sleep(200 * time.Millisecond)
		os.Exit(0)
	case "default-term":
		markProcessHelperReady(t)
		select {}
	case "ignore-term":
		signal.Ignore(syscall.SIGTERM)
		markProcessHelperReady(t)
		select {}
	default:
		t.Fatalf("unknown helper mode %q", mode)
	}
}

func markProcessHelperReady(t *testing.T) {
	t.Helper()

	readyPath := os.Getenv("XRAY_PROCESS_READY")
	if readyPath == "" {
		t.Fatal("XRAY_PROCESS_READY is not set")
	}
	if err := os.WriteFile(readyPath, []byte("ready"), 0644); err != nil {
		t.Fatalf("write helper ready file: %v", err)
	}
}
