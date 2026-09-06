package service

import "testing"

// An existing install has no row for these keys, so the default decides whether
// upgrading silently switches the daily reports off.
func TestTgBotNotifyDefaultsToEnabled(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}

	tests := []struct {
		name string
		get  func() (bool, error)
	}{
		{"server usage", s.GetTgBotNotifyServerUsage},
		{"deplete soon", s.GetTgBotNotifyDepleteSoon},
		{"new client", s.GetTgBotNotifyNewClient},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.get()
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if !got {
				t.Fatal("default is off, so upgrading would silently stop the daily report")
			}
		})
	}
}

func TestTgBotNotifyRoundTrip(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}

	tests := []struct {
		name string
		get  func() (bool, error)
		set  func(bool) error
	}{
		{"server usage", s.GetTgBotNotifyServerUsage, s.SetTgBotNotifyServerUsage},
		{"deplete soon", s.GetTgBotNotifyDepleteSoon, s.SetTgBotNotifyDepleteSoon},
		{"new client", s.GetTgBotNotifyNewClient, s.SetTgBotNotifyNewClient},
		{"backup", s.GetTgBotBackup, s.SetTgBotBackup},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, want := range []bool{false, true} {
				if err := tc.set(want); err != nil {
					t.Fatalf("set(%v): %v", want, err)
				}
				got, err := tc.get()
				if err != nil {
					t.Fatalf("get after set(%v): %v", want, err)
				}
				if got != want {
					t.Fatalf("got %v after set(%v)", got, want)
				}
			}
		})
	}
}
