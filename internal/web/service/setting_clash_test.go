package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestValidateRegex(t *testing.T) {
	for _, pattern := range []string{"", `(?i)^jsonclient([ /]|$)`, `(?m)^general-purpose$`} {
		if err := ValidateRegex(pattern); err != nil {
			t.Errorf("ValidateRegex(%q) returned %v", pattern, err)
		}
	}
	for _, pattern := range []string{"[", strings.Repeat("a", 2049)} {
		if err := ValidateRegex(pattern); err == nil {
			t.Errorf("ValidateRegex(%q) accepted an invalid pattern", pattern)
		}
	}
}

func TestSubscriptionAutoDetectDefaultsWithoutStoredRows(t *testing.T) {
	setupSettingTestDB(t)
	keys := []string{"subClashAutoDetect", "subClashUserAgentRegex", "subJsonAutoDetect", "subJsonAlwaysArray", "subJsonUserAgentRegex"}
	if err := database.GetDB().Where("key IN ?", keys).Delete(&model.Setting{}).Error; err != nil {
		t.Fatal(err)
	}

	s := &SettingService{}
	clashEnabled, err := s.GetSubClashAutoDetect()
	if err != nil {
		t.Fatal(err)
	}
	jsonEnabled, err := s.GetSubJsonAutoDetect()
	if err != nil {
		t.Fatal(err)
	}
	jsonAlwaysArray, err := s.GetSubJsonAlwaysArray()
	if err != nil {
		t.Fatal(err)
	}
	clashRegex, err := s.GetSubClashUserAgentRegex()
	if err != nil {
		t.Fatal(err)
	}
	jsonRegex, err := s.GetSubJsonUserAgentRegex()
	if err != nil {
		t.Fatal(err)
	}

	if clashEnabled || jsonEnabled || jsonAlwaysArray {
		t.Fatalf("missing subscription flags must default off: clashAuto=%v jsonAuto=%v jsonAlwaysArray=%v", clashEnabled, jsonEnabled, jsonAlwaysArray)
	}
	if clashRegex != "" {
		t.Fatalf("missing Clash regex = %q, want empty inherited value", clashRegex)
	}
	if jsonRegex != "" {
		t.Fatalf("missing JSON regex = %q, want empty inherited value", jsonRegex)
	}

	var count int64
	if err := database.GetDB().Model(&model.Setting{}).Where("key IN ?", keys).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("default lookup unexpectedly persisted %d setting rows", count)
	}
}

func TestUpdateAllSettingPreservesEmptyUserAgentRegexes(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	settings.SubJsonUserAgentRegex = "   "
	settings.SubClashUserAgentRegex = ""

	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"subJsonUserAgentRegex", "subClashUserAgentRegex"} {
		var stored model.Setting
		if err := database.GetDB().Where("key = ?", key).First(&stored).Error; err != nil {
			t.Fatal(err)
		}
		if stored.Value != "" {
			t.Fatalf("%s stored value = %q, want empty inherited value", key, stored.Value)
		}
	}
}

func TestUpdateAllSettingPersistsClashSubscriptionSettings(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}

	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if settings.SubClashAutoDetect {
		t.Fatal("subClashAutoDetect default = true, want false")
	}
	if settings.SubJsonAutoDetect {
		t.Fatal("subJsonAutoDetect default = true, want false")
	}
	if settings.SubJsonAlwaysArray {
		t.Fatal("subJsonAlwaysArray default = true, want false")
	}
	if settings.SubJsonUserAgentRegex != "" {
		t.Fatalf("subJsonUserAgentRegex = %q, want empty inherited value", settings.SubJsonUserAgentRegex)
	}
	if settings.SubClashUserAgentRegex != "" {
		t.Fatalf("subClashUserAgentRegex = %q, want empty inherited value", settings.SubClashUserAgentRegex)
	}
	settings.SubClashAutoDetect = true
	settings.SubClashUserAgentRegex = `(?i)^custom-clash/`
	settings.SubJsonAutoDetect = true
	settings.SubJsonAlwaysArray = true
	settings.SubJsonUserAgentRegex = `(?i)^custom-json/`
	settings.SubJsonEnable = true
	settings.SubJsonPath = "/json-custom/"
	settings.SubJsonURI = "https://subscriptions.example.com/json-custom/"
	settings.SubClashEnable = true
	settings.SubClashPath = "/clash-custom/"
	settings.SubClashURI = "https://subscriptions.example.com/clash-custom/"
	settings.SubClashEnableRouting = true
	settings.SubClashRules = "GEOIP,private,DIRECT"

	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if !got.SubClashEnable {
		t.Fatal("subClashEnable = false, want true")
	}
	if !got.SubClashAutoDetect {
		t.Fatal("subClashAutoDetect = false, want true")
	}
	if !got.SubJsonAutoDetect {
		t.Fatal("subJsonAutoDetect = false, want true")
	}
	if !got.SubJsonAlwaysArray {
		t.Fatal("subJsonAlwaysArray = false, want true")
	}
	if !got.SubJsonEnable {
		t.Fatal("subJsonEnable = false, want true")
	}
	if got.SubJsonPath != "/json-custom/" {
		t.Fatalf("subJsonPath = %q, want %q", got.SubJsonPath, "/json-custom/")
	}
	if got.SubJsonURI != "https://subscriptions.example.com/json-custom/" {
		t.Fatalf("subJsonURI = %q, want %q", got.SubJsonURI, "https://subscriptions.example.com/json-custom/")
	}
	if got.SubJsonUserAgentRegex != `(?i)^custom-json/` {
		t.Fatalf("subJsonUserAgentRegex = %q, want %q", got.SubJsonUserAgentRegex, `(?i)^custom-json/`)
	}
	if got.SubClashUserAgentRegex != `(?i)^custom-clash/` {
		t.Fatalf("subClashUserAgentRegex = %q, want %q", got.SubClashUserAgentRegex, `(?i)^custom-clash/`)
	}
	if got.SubClashPath != "/clash-custom/" {
		t.Fatalf("subClashPath = %q, want %q", got.SubClashPath, "/clash-custom/")
	}
	if got.SubClashURI != "https://subscriptions.example.com/clash-custom/" {
		t.Fatalf("subClashURI = %q, want %q", got.SubClashURI, "https://subscriptions.example.com/clash-custom/")
	}
	if !got.SubClashEnableRouting {
		t.Fatal("subClashEnableRouting = false, want true")
	}
	if got.SubClashRules != "GEOIP,private,DIRECT" {
		t.Fatalf("subClashRules = %q, want %q", got.SubClashRules, "GEOIP,private,DIRECT")
	}
}

func TestUpdateAllSettingRejectsInvalidClashUserAgentRegex(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	settings.SubClashUserAgentRegex = "["
	if err := s.UpdateAllSetting(settings, SecretClears{}); err == nil {
		t.Fatal("UpdateAllSetting accepted an invalid Clash/Mihomo User-Agent regex")
	}
}

func TestUpdateAllSettingRejectsInvalidJsonUserAgentRegex(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	settings.SubJsonUserAgentRegex = "["
	if err := s.UpdateAllSetting(settings, SecretClears{}); err == nil {
		t.Fatal("UpdateAllSetting accepted an invalid Xray JSON User-Agent regex")
	}
}
