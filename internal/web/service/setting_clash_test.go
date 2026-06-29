package service

import "testing"

func TestUpdateAllSettingPersistsClashSubscriptionSettings(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}

	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if settings.SubAutoDetect {
		t.Fatal("subAutoDetect default = true, want false")
	}
	if settings.SubJsonAutoDetect {
		t.Fatal("subJsonAutoDetect default = true, want false")
	}
	if settings.SubJsonUserAgentRegex != DefaultSubJsonUserAgentRegex {
		t.Fatalf("subJsonUserAgentRegex = %q, want default %q", settings.SubJsonUserAgentRegex, DefaultSubJsonUserAgentRegex)
	}
	if settings.SubClashUserAgentRegex != DefaultSubClashUserAgentRegex {
		t.Fatalf("subClashUserAgentRegex = %q, want default %q", settings.SubClashUserAgentRegex, DefaultSubClashUserAgentRegex)
	}
	settings.SubAutoDetect = true
	settings.SubClashUserAgentRegex = `(?i)^custom-clash/`
	settings.SubJsonAutoDetect = true
	settings.SubJsonUserAgentRegex = `(?i)^custom-json/`
	settings.SubJsonEnable = true
	settings.SubJsonPath = "/json-custom/"
	settings.SubJsonURI = "https://subscriptions.example.com/json-custom/"
	settings.SubClashEnable = true
	settings.SubClashPath = "/clash-custom/"
	settings.SubClashURI = "https://subscriptions.example.com/clash-custom/"
	settings.SubClashEnableRouting = true
	settings.SubClashRules = "GEOIP,private,DIRECT"

	if err := s.UpdateAllSetting(settings); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if !got.SubClashEnable {
		t.Fatal("subClashEnable = false, want true")
	}
	if !got.SubAutoDetect {
		t.Fatal("subAutoDetect = false, want true")
	}
	if !got.SubJsonAutoDetect {
		t.Fatal("subJsonAutoDetect = false, want true")
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
	if err := s.UpdateAllSetting(settings); err == nil {
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
	if err := s.UpdateAllSetting(settings); err == nil {
		t.Fatal("UpdateAllSetting accepted an invalid Xray JSON User-Agent regex")
	}
}
