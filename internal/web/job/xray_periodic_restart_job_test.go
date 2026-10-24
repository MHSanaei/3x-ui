package job

import (
	"errors"
	"testing"
)

type mockXrayRestartRunner struct {
	isRunning       bool
	restartCalled   bool
	lastForcePassed bool
	restartErr      error
}

func (m *mockXrayRestartRunner) IsXrayRunning() bool {
	return m.isRunning
}

func (m *mockXrayRestartRunner) RestartXray(isForce bool) error {
	m.restartCalled = true
	m.lastForcePassed = isForce
	return m.restartErr
}

func TestXrayPeriodicRestartJobForcesRestartWhenRunning(t *testing.T) {
	runner := &mockXrayRestartRunner{isRunning: true}
	job := &XrayPeriodicRestartJob{xrayService: runner}

	job.Run()

	if !runner.restartCalled {
		t.Fatal("expected RestartXray to be called when Xray is running, but it was not called")
	}
	if !runner.lastForcePassed {
		t.Fatal("expected RestartXray(true) with isForce=true to force core recreation, got false")
	}
}

func TestXrayPeriodicRestartJobSkipsWhenNotRunning(t *testing.T) {
	runner := &mockXrayRestartRunner{isRunning: false}
	job := &XrayPeriodicRestartJob{xrayService: runner}

	job.Run()

	if runner.restartCalled {
		t.Fatal("expected RestartXray NOT to be called when Xray is stopped, but it was called")
	}
}

func TestXrayPeriodicRestartJobHandlesRestartFailure(t *testing.T) {
	runner := &mockXrayRestartRunner{
		isRunning:  true,
		restartErr: errors.New("cannot bind port"),
	}
	job := &XrayPeriodicRestartJob{xrayService: runner}

	// Must handle error gracefully without panicking
	job.Run()

	if !runner.restartCalled {
		t.Fatal("expected RestartXray to be called")
	}
}
