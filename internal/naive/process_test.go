package naive

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestWaitDoneClosed(t *testing.T) {
	done := make(chan struct{})
	close(done)
	if !waitDone(done, 10*time.Millisecond) {
		t.Fatal("waitDone returned false for closed channel")
	}
}

func TestWaitDoneTimeout(t *testing.T) {
	done := make(chan struct{})
	if waitDone(done, 10*time.Millisecond) {
		t.Fatal("waitDone returned true for open channel")
	}
}

func TestProcessExitRemovesConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses a POSIX shell script")
	}

	binDir := t.TempDir()
	logDir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", binDir)
	t.Setenv("XUI_LOG_FOLDER", logDir)

	binary := filepath.Join(binDir, binaryName())
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	tag := fmt.Sprintf("cleanup-%d", time.Now().UnixNano())
	configPath := ConfigPath(tag)
	t.Cleanup(func() { _ = os.Remove(configPath) })

	process, err := Start(tag, Config{
		Listen: "socks://127.0.0.1:30000",
		Proxy:  "https://user:pass@example.com:443",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	select {
	case <-process.done:
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit")
	}

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config file remains after process exit: %v", err)
	}
}
