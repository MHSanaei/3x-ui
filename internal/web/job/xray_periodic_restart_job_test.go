package job

import (
	"testing"
)

func TestNewXrayPeriodicRestartJob(t *testing.T) {
	job := NewXrayPeriodicRestartJob()
	if job == nil {
		t.Fatal("expected NewXrayPeriodicRestartJob to return non-nil instance")
	}
}

func TestXrayPeriodicRestartJobRun(t *testing.T) {
	setupIntegrationDB(t)

	job := NewXrayPeriodicRestartJob()
	// Run should execute cleanly without panic even if Xray core process is not running in test context
	job.Run()
}
