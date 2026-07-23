package service

import "testing"

func TestSubShowIdentityOnAllLinksDefaultsAndPersists(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}

	if got, err := s.GetSubShowIdentityOnAllLinks(); err != nil || got {
		t.Fatalf("missing setting = %t, %v; want false, nil", got, err)
	}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if settings.SubShowIdentityOnAllLinks {
		t.Fatal("GetAllSetting returned true for a missing setting")
	}

	settings.SubShowIdentityOnAllLinks = true
	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.GetSubShowIdentityOnAllLinks(); err != nil || !got {
		t.Fatalf("persisted setting = %t, %v; want true, nil", got, err)
	}

	settings, err = s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	settings.SubShowIdentityOnAllLinks = false
	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.GetSubShowIdentityOnAllLinks(); err != nil || got {
		t.Fatalf("persisted setting = %t, %v; want false, nil", got, err)
	}
}
