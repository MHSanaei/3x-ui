package service

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestSubProfileModeReadsLegacyAndExplicitSettings(t *testing.T) {
	tests := []struct {
		name       string
		storedMode string
		storedURL  string
		modeExists bool
		want       string
	}{
		{name: "fresh installation", want: "none"},
		{name: "legacy URL", storedURL: " https://profile.example/account ", want: "custom"},
		{name: "legacy blank URL", storedURL: " \t ", want: "none"},
		{name: "legacy empty mode with URL", modeExists: true, storedURL: "https://profile.example/account", want: "custom"},
		{name: "legacy empty mode without URL", modeExists: true, want: "none"},
		{name: "explicit none preserves saved URL", modeExists: true, storedMode: "none", storedURL: "https://profile.example/account", want: "none"},
		{name: "explicit builtin", modeExists: true, storedMode: "builtin", storedURL: "https://profile.example/account", want: "builtin"},
		{name: "explicit custom", modeExists: true, storedMode: "custom", storedURL: "https://profile.example/account", want: "custom"},
		{name: "custom with empty URL", modeExists: true, storedMode: "custom", want: "custom"},
		{name: "invalid stored mode fails closed", modeExists: true, storedMode: "automatic", storedURL: "https://profile.example/account", want: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupSettingTestDB(t)
			s := &SettingService{}
			if tt.modeExists {
				if err := s.saveSetting("subProfileMode", tt.storedMode); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.saveSetting("subProfileUrl", tt.storedURL); err != nil {
				t.Fatal(err)
			}
			assertSubProfileSettings(t, s, tt.want, tt.storedURL)
		})
	}
}

func TestSubProfileModeUpdatesPreserveURLAndLegacyPayloads(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	if got := s.GetFactoryDefaults()["subProfileMode"]; got != "none" {
		t.Errorf("factory profile mode = %q, want none", got)
	}
	tests := []struct {
		name string
		mode string
		url  string
		want string
	}{
		{name: "custom", mode: "custom", url: "https://profile.example/account", want: "custom"},
		{name: "none retains custom URL", mode: "none", url: "https://profile.example/account", want: "none"},
		{name: "builtin retains custom URL", mode: "builtin", url: "https://profile.example/account", want: "builtin"},
		{name: "custom restores saved URL", mode: "custom", url: "https://profile.example/account", want: "custom"},
		{name: "legacy URL submission", url: "https://legacy.example/account", want: "custom"},
		{name: "legacy empty URL submission", want: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings, err := s.GetAllSetting()
			if err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(map[string]string{"subProfileMode": tt.mode, "subProfileUrl": tt.url})
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(payload, settings); err != nil {
				t.Fatal(err)
			}
			if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
				t.Fatal(err)
			}
			assertSubProfileSettings(t, s, tt.want, tt.url)
			stored, err := s.getSetting("subProfileMode")
			if err != nil {
				t.Fatal(err)
			}
			if stored.Value != tt.want {
				t.Fatalf("persisted mode = %q, want %q", stored.Value, tt.want)
			}
		})
	}
}

func TestSubProfileModeRejectsInvalidUpdateBeforeWrites(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	if err := s.saveSetting("subTitle", "Original title"); err != nil {
		t.Fatal(err)
	}
	var before []model.Setting
	if err := database.GetDB().Order("id").Find(&before).Error; err != nil {
		t.Fatal(err)
	}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"subProfileMode":"automatic","subTitle":"Changed title","subProfileUrl":"https://profile.example/account"}`), settings); err != nil {
		t.Fatal(err)
	}
	err = s.UpdateAllSetting(settings, SecretClears{})
	if err == nil || err.Error() != "subscription profile mode must be none, builtin, or custom" {
		t.Errorf("UpdateAllSetting error = %v, want invalid profile mode error", err)
	}
	var after []model.Setting
	if err := database.GetDB().Order("id").Find(&after).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("invalid profile mode update modified stored settings")
	}
}

func assertSubProfileSettings(t *testing.T, s *SettingService, wantMode, wantURL string) {
	t.Helper()
	if mode, err := s.GetSubProfileMode(); err != nil || mode != wantMode {
		t.Fatalf("GetSubProfileMode = %q, %v; want %q, nil", mode, err, wantMode)
	}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	var profile struct {
		Mode string `json:"subProfileMode"`
		URL  string `json:"subProfileUrl"`
	}
	if err := json.Unmarshal(payload, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.Mode != wantMode || profile.URL != wantURL {
		t.Fatalf("profile settings = (%q, %q), want (%q, %q)", profile.Mode, profile.URL, wantMode, wantURL)
	}
}
