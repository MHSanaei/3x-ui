package service

import "testing"

func TestHappLinkEnableReadsExplicitValues(t *testing.T) {
	initHappTestDB(t)
	s := &SettingService{}

	for _, want := range []bool{false, true} {
		settings, err := s.GetAllSetting()
		if err != nil {
			t.Fatal(err)
		}
		settings.HappLinkEnable = want
		if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
			t.Fatal(err)
		}
		gotDirect, err := s.GetHappLinkEnable()
		if err != nil || gotDirect != want {
			t.Fatalf("GetHappLinkEnable = %t, %v; want %t, nil", gotDirect, err, want)
		}
		if got := happLinkEnableFromDefaults(t, s); got != want {
			t.Fatalf("stored happLinkEnable = %t, want %t", got, want)
		}
	}
}

func happLinkEnableFromDefaults(t *testing.T, s *SettingService) bool {
	t.Helper()
	defaults, err := s.GetDefaultSettings("panel.example")
	if err != nil {
		t.Fatal(err)
	}
	values, ok := defaults.(map[string]any)
	if !ok {
		t.Fatalf("GetDefaultSettings type = %T, want map[string]any", defaults)
	}
	enabled, ok := values["happLinkEnable"].(bool)
	if !ok {
		t.Fatalf("happLinkEnable = %#v, want bool", values["happLinkEnable"])
	}
	return enabled
}
