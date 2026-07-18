package service

import (
	"testing"
)

func TestRestartXrayRespectsManualStop(t *testing.T) {
	setupSettingTestDB(t)
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", "{ not valid json"); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	t.Cleanup(func() { isManuallyStopped.Store(false) })

	isManuallyStopped.Store(true)
	_ = (&XrayService{}).RestartXray(false)

	if !isManuallyStopped.Load() {
		t.Fatal("a non-forced restart cleared a deliberate manual stop and would revive xray")
	}
}

func TestApplyPendingRestartReArmsFlagOnFailure(t *testing.T) {
	setupSettingTestDB(t)
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", "{ not valid json"); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	t.Cleanup(func() {
		isManuallyStopped.Store(false)
		isNeedXrayRestart.Store(false)
	})
	isManuallyStopped.Store(false)

	svc := &XrayService{}
	svc.SetToNeedRestart()
	svc.ApplyPendingRestart()

	if !isNeedXrayRestart.Load() {
		t.Fatal("a failed restart must re-arm the need-restart flag so the pending config change is retried")
	}
}
