package service

import (
	"testing"
)

func TestXrayPeriodicRestartSettings(t *testing.T) {
	setupSettingTestDB(t)

	service := &SettingService{}

	// Verify default values
	enabled, err := service.GetXrayPeriodicRestartEnable()
	if err != nil {
		t.Fatalf("GetXrayPeriodicRestartEnable failed: %v", err)
	}
	if enabled != false {
		t.Fatalf("expected default XrayPeriodicRestartEnable to be false, got %v", enabled)
	}

	cron, err := service.GetXrayPeriodicRestartCron()
	if err != nil {
		t.Fatalf("GetXrayPeriodicRestartCron failed: %v", err)
	}
	if cron != "@daily" {
		t.Fatalf("expected default XrayPeriodicRestartCron to be '@daily', got %q", cron)
	}

	// Update and verify
	if err := service.SetXrayPeriodicRestartEnable(true); err != nil {
		t.Fatalf("SetXrayPeriodicRestartEnable failed: %v", err)
	}
	enabled, err = service.GetXrayPeriodicRestartEnable()
	if err != nil || !enabled {
		t.Fatalf("expected XrayPeriodicRestartEnable to be true, got %v (err: %v)", enabled, err)
	}

	testCron := "0 4 * * *"
	if err := service.SetXrayPeriodicRestartCron(testCron); err != nil {
		t.Fatalf("SetXrayPeriodicRestartCron failed: %v", err)
	}
	cron, err = service.GetXrayPeriodicRestartCron()
	if err != nil || cron != testCron {
		t.Fatalf("expected XrayPeriodicRestartCron to be %q, got %q (err: %v)", testCron, cron, err)
	}
}
