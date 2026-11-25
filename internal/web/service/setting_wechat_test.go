package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWechatSecretSettingsSurviveUnrelatedSaveAndClearExplicitly(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	const endpoint = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=private-key"
	for key, value := range map[string]string{"wechatWebhookURL": endpoint, "wechatToken": "private-token"} {
		if err := s.saveSetting(key, value); err != nil {
			t.Fatal(err)
		}
	}
	view, err := s.GetAllSettingView()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private-key") || strings.Contains(string(encoded), "private-token") {
		t.Fatal("settings response exposed a WeChat credential")
	}
	if !view.HasWechatWebhookURL || !view.HasWechatToken {
		t.Fatal("redacted response lost the configured credential indicators")
	}
	view.PageSize = 50
	if err := s.UpdateAllSetting(&view.AllSetting, SecretClears{}); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if stored.WechatWebhookURL != endpoint || stored.WechatToken != "private-token" || stored.PageSize != 50 {
		t.Fatal("unrelated settings save replaced the redacted WeChat credentials")
	}
	stored.WechatWebhookURL, stored.WechatToken = "", ""
	if err := s.UpdateAllSetting(stored, SecretClears{WechatWebhookURL: true, WechatToken: true}); err != nil {
		t.Fatal(err)
	}
	cleared, err := s.GetAllSettingView()
	if err != nil {
		t.Fatal(err)
	}
	if cleared.HasWechatWebhookURL || cleared.HasWechatToken {
		t.Fatal("explicit credential clearing retained the saved WeChat credentials")
	}
}

func TestInvalidWechatDestinationDoesNotReplaceSavedSettings(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	for _, endpoint := range []string{
		"http://example.com/notify", "https://user:password@example.com/notify",
		"https://example.com/notify#secret", "https://127.0.0.1/notify",
		"https://qyapi.weixin.qq.com.evil.example/cgi-bin/webhook/send?key=secret",
	} {
		t.Run(endpoint, func(t *testing.T) {
			settings, err := s.GetAllSetting()
			if err != nil {
				t.Fatal(err)
			}
			settings.WechatEnable = true
			settings.WechatMode = "wecom"
			settings.WechatWebhookURL = endpoint
			settings.PageSize = 99
			err = s.UpdateAllSetting(settings, SecretClears{})
			if err == nil || err.Error() != "WeChat webhook URL is invalid" {
				t.Fatalf("invalid destination error = %v, want classified validation error", err)
			}
			stored, err := s.GetAllSetting()
			if err != nil {
				t.Fatal(err)
			}
			if stored.PageSize == 99 || stored.WechatEnable {
				t.Fatal("invalid WeChat destination partially persisted unrelated settings")
			}
		})
	}
}

func TestWechatCronAndEventSelectionRejectInvalidConfiguration(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	for _, tc := range []struct{ field, value string }{
		{"wechatMode", "telegram"},
		{"wechatRunTime", "not a schedule"},
		{"wechatEnabledEvents", "login.attempt,unknown.event"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			settings, err := s.GetAllSetting()
			if err != nil {
				t.Fatal(err)
			}
			switch tc.field {
			case "wechatMode":
				settings.WechatMode = tc.value
			case "wechatRunTime":
				settings.WechatRunTime = tc.value
			case "wechatEnabledEvents":
				settings.WechatEnabledEvents = tc.value
			}
			if err := s.UpdateAllSetting(settings, SecretClears{}); err == nil || !strings.Contains(err.Error(), "WeChat") {
				t.Fatalf("invalid %s was not rejected with a WeChat validation error: %v", tc.field, err)
			}
		})
	}
}
