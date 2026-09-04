package service

import (
	"testing"
)

func TestSubInfoNodeSettingsDefaultsAndPersists(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}

	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if settings.SubInfoNodeEnable {
		t.Fatal("expected default SubInfoNodeEnable false")
	}
	if settings.SubExpiredTemplate != DefaultSubExpiredTemplate {
		t.Fatalf("expected default SubExpiredTemplate %q, got %q", DefaultSubExpiredTemplate, settings.SubExpiredTemplate)
	}
	if settings.SubTrafficDepletedTemplate != DefaultSubTrafficDepletedTemplate {
		t.Fatalf("expected default SubTrafficDepletedTemplate %q, got %q", DefaultSubTrafficDepletedTemplate, settings.SubTrafficDepletedTemplate)
	}

	settings.SubInfoNodeEnable = true
	settings.SubExpiredTemplate = "custom expired"
	settings.SubTrafficDepletedTemplate = "custom depleted"
	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatal(err)
	}

	gotEnabled, err := s.GetSubInfoNodeEnable()
	if err != nil || !gotEnabled {
		t.Fatalf("expected true, got %v, err %v", gotEnabled, err)
	}
	gotExp, err := s.GetSubExpiredTemplate()
	if err != nil || gotExp != "custom expired" {
		t.Fatalf("expected 'custom expired', got %q, err %v", gotExp, err)
	}
	gotDep, err := s.GetSubTrafficDepletedTemplate()
	if err != nil || gotDep != "custom depleted" {
		t.Fatalf("expected 'custom depleted', got %q, err %v", gotDep, err)
	}
}
