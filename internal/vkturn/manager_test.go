package vkturn

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestReconcilePreservesRestartsAndRemovesProcesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX helper executable")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "server")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexec sleep 1000\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	manager := newManager(binary)
	t.Cleanup(manager.StopAll)
	initial := Instance{ID: 1, Listen: "0.0.0.0:56000", Connect: "127.0.0.1:51820"}
	if err := manager.Ensure(initial); err != nil {
		t.Fatal(err)
	}
	first := manager.processes[1]
	if err := manager.Ensure(initial); err != nil {
		t.Fatal(err)
	}
	if manager.processes[1] != first {
		t.Fatal("unchanged tunnel restarted")
	}
	changed := initial
	changed.Listen = "0.0.0.0:56001"
	manager.Reconcile([]Instance{changed})
	second := manager.processes[1]
	if second == first {
		t.Fatal("port edit retained old process")
	}
	select {
	case <-first.done:
	default:
		t.Fatal("old listener still running after replacement")
	}
	if err := second.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-second.done:
	case <-time.After(2 * time.Second):
		t.Fatal("crashed child did not exit")
	}
	manager.Reconcile([]Instance{changed})
	if manager.processes[1] == second {
		t.Fatal("crashed tunnel was not restarted")
	}
	running := manager.processes[1]
	manager.Reconcile(nil)
	if len(manager.processes) != 0 {
		t.Fatal("deleted tunnel retained")
	}
	select {
	case <-running.done:
	default:
		t.Fatal("deleted tunnel still running")
	}
}

func TestMissingBinaryReturnsActionableFailure(t *testing.T) {
	manager := newManager(filepath.Join(t.TempDir(), "missing-server"))
	err := manager.Ensure(Instance{ID: 2, Listen: "0.0.0.0:56000", Connect: "127.0.0.1:51820"})
	if !errors.Is(err, os.ErrNotExist) || len(manager.processes) != 0 {
		t.Fatalf("Ensure = %v, processes = %d", err, len(manager.processes))
	}
}
