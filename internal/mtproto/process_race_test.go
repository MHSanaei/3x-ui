package mtproto

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProcessLifecycleFieldsRaceSafe(t *testing.T) {
	proc := newProcess("", "test")
	stop := make(chan struct{})
	var workers sync.WaitGroup
	defer func() {
		close(stop)
		workers.Wait()
	}()

	workers.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			proc.mu.Lock()
			proc.cmd = &exec.Cmd{}
			proc.done = make(chan struct{})
			proc.mu.Unlock()
			proc.setExitErr(errors.New("exit"))
		}
	})
	for range 4 {
		workers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = proc.IsRunning()
				_ = proc.GetResult()
			}
		})
	}

	time.Sleep(50 * time.Millisecond)
}

func TestProcessStatusDuringExit(t *testing.T) {
	pidFile := installFakeMtg(t)
	exitFile := filepath.Join(t.TempDir(), "exit")
	t.Setenv("MTG_FAKE_EXIT_FILE", exitFile)
	configPath := filepath.Join(t.TempDir(), "mtg.toml")
	if err := os.WriteFile(configPath, nil, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	proc := newProcess(configPath, "test")
	if err := proc.Start(); err != nil {
		t.Fatalf("start process: %v", err)
	}
	t.Cleanup(func() {
		_ = proc.Stop()
	})
	waitSpawnCount(t, pidFile, 1)

	stopReads := make(chan struct{})
	var readers sync.WaitGroup
	defer func() {
		close(stopReads)
		readers.Wait()
	}()
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stopReads:
					return
				default:
					_ = proc.IsRunning()
					_ = proc.GetResult()
				}
			}
		})
	}

	proc.mu.RLock()
	done := proc.done
	proc.mu.RUnlock()
	if err := os.WriteFile(exitFile, nil, 0o600); err != nil {
		t.Fatalf("trigger exit: %v", err)
	}
	if err := waitForExit(done, time.Second); err != nil {
		t.Fatalf("wait for process exit: %v", err)
	}
	if proc.IsRunning() {
		t.Fatal("process must not be running after exit")
	}
	if got := proc.GetResult(); !strings.Contains(got, "exit status 1") {
		t.Fatalf("GetResult after an unexpected exit = %q, want exit status", got)
	}
}
