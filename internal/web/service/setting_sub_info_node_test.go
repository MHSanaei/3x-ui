package service

import (
	"testing"
)

func TestSubHostSortModePersistsAndRejectsInvalidValues(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if settings.SubHostSortMode != "inbound-first" {
		t.Fatalf("default = %q", settings.SubHostSortMode)
	}
	settings.SubHostSortMode = "hosts-first"
	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := s.GetAllSetting()
	if err != nil || reloaded.SubHostSortMode != "hosts-first" {
		t.Fatalf("reloaded mode = %+v, err=%v", reloaded, err)
	}
	settings.SubHostSortMode = "invalid"
	if err := s.UpdateAllSetting(settings, SecretClears{}); err == nil || err.Error() != "subscription host sort mode must be inbound-first or hosts-first" {
		t.Fatalf("invalid mode error = %v", err)
	}
	mode, err := s.GetSubHostSortMode()
	if err != nil || mode != "hosts-first" {
		t.Fatalf("invalid save changed mode: %q, %v", mode, err)
	}
}

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
