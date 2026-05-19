//go:build !windows

package xray

import (
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	xuilogger "github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/op/go-logging"
)

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
