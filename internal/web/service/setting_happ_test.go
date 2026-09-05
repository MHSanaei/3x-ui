package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestHappLinkEnableDefaultsOffWithoutPersistingRow(t *testing.T) {
	initHappTestDB(t)
	if err := database.GetDB().Where("key = ?", "happLinkEnable").Delete(&model.Setting{}).Error; err != nil {
		t.Fatal(err)
	}

	s := &SettingService{}
	if enabled, err := s.GetHappLinkEnable(); err != nil || enabled {
		t.Fatalf("missing GetHappLinkEnable = %t, %v; want false, nil", enabled, err)
	}
	if got := happLinkEnableFromDefaults(t, s); got {
		t.Fatal("missing happLinkEnable = true, want false")
	}
	allSetting, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(allSetting)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := json.Unmarshal(encoded, &values); err != nil {
		t.Fatal(err)
	}
	if got, ok := values["happLinkEnable"].(bool); !ok || got {
		t.Fatalf("AllSetting happLinkEnable = %#v, want false", values["happLinkEnable"])
	}

	var count int64
	if err := database.GetDB().Model(&model.Setting{}).Where("key = ?", "happLinkEnable").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("default lookup unexpectedly persisted %d happLinkEnable rows", count)
	}
}

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
