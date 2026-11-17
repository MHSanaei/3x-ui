package service

import "testing"

func TestConntrackThresholdDefaultsAndPersistence(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if settings.TgConntrack != 80 {
		t.Fatalf("default threshold = %d, want 80", settings.TgConntrack)
	}
	for _, threshold := range []int{75, 0, 100} {
		settings.TgConntrack = threshold
		if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
			t.Fatal(err)
		}
		got, err := s.GetTgConntrack()
		if err != nil || got != threshold {
			t.Fatalf("threshold = %d, error %v, want %d", got, err, threshold)
		}
	}
	for _, threshold := range []int{-1, 101} {
		settings.TgConntrack = threshold
		if err := s.UpdateAllSetting(settings, SecretClears{}); err == nil {
			t.Fatalf("invalid threshold %d accepted", threshold)
		}
		if got, err := s.GetTgConntrack(); err != nil || got != 100 {
			t.Fatalf("invalid save changed threshold to %d, error %v", got, err)
		}
	}
}
