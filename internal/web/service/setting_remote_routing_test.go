package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func TestValidateRemoteRoutingURLSettings(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		want      string
		wantError string
	}{
		{name: "valid HTTPS", value: " https://example.com/rules#fragment ", want: "https://example.com/rules"},
		{name: "credentials", value: "https://user:pass@example.com/rules", wantError: "must not contain URL credentials"},
		{name: "missing host", value: "https:///rules", wantError: "absolute HTTPS URL"},
		{name: "legacy HTTP stays inline", value: "http://example.com/rules", want: "http://example.com/rules"},
		{name: "multiline Clash stays inline", value: "https://example.com/rules\nMATCH,PROXY", want: "https://example.com/rules\nMATCH,PROXY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &entity.AllSetting{SubRoutingRules: tt.value}
			err := validateSettingsURLs(settings)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("err=%v, want %q", err, tt.wantError)
				}
				return
			}
			if err != nil || settings.SubRoutingRules != tt.want {
				t.Fatalf("value=%q err=%v", settings.SubRoutingRules, err)
			}
		})
	}
}

func TestValidateSubJsonRoutingRulesSetting(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		want      string
		wantError string
	}{
		{name: "remote URL is canonicalised", value: " https://example.com/DEFAULT.JSON#frag ", want: "https://example.com/DEFAULT.JSON"},
		{name: "remote URL with credentials is rejected", value: "https://user:pw@example.com/DEFAULT.JSON", wantError: "JSON subscription routing source"},
		{name: "inline JSON passes through", value: "{\"DirectSites\":[\"geosite:cat\"]}", want: "{\"DirectSites\":[\"geosite:cat\"]}"},
		{name: "blank stays blank", value: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &entity.AllSetting{SubJsonRoutingRules: tt.value}
			err := validateSettingsURLs(settings)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("err=%v, want %q", err, tt.wantError)
				}
				return
			}
			if err != nil || settings.SubJsonRoutingRules != tt.want {
				t.Fatalf("value=%q err=%v", settings.SubJsonRoutingRules, err)
			}
		})
	}
}
