package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func TestValidateSubJsonDnsSetting(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		want      string
		wantError string
	}{
		{name: "blank is trimmed", value: "   ", want: ""},
		{name: "object passes through", value: ` {"servers": ["1.1.1.1"]} `, want: `{"servers": ["1.1.1.1"]}`},
		{name: "array passes through", value: `["1.1.1.1", "tls://1.0.0.1"]`, want: `["1.1.1.1", "tls://1.0.0.1"]`},
		{name: "object with hosts and strategy", value: `{"hosts":{"a":"b"},"queryStrategy":"UseIPv4","servers":["1.1.1.1"]}`, want: `{"hosts":{"a":"b"},"queryStrategy":"UseIPv4","servers":["1.1.1.1"]}`},
		{name: "malformed JSON is rejected", value: `{"servers": [`, wantError: "JSON subscription DNS is invalid"},
		{name: "broken field type is rejected", value: `{"servers": ["1.1.1.1"], "hosts": 5}`, wantError: "JSON subscription DNS is invalid"},
		{name: "empty server list is rejected", value: `[]`, wantError: "JSON subscription DNS is invalid"},
		{name: "server without address is rejected", value: `[{"skipFallback": true}]`, wantError: "JSON subscription DNS is invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &entity.AllSetting{SubJsonDns: tt.value}
			err := validateSubJsonDnsSetting(settings)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("err=%v, want %q", err, tt.wantError)
				}
				return
			}
			if err != nil || settings.SubJsonDns != tt.want {
				t.Fatalf("value=%q err=%v", settings.SubJsonDns, err)
			}
		})
	}
}

func TestSubJsonDnsSettingDefaultsAndPersists(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}

	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if settings.SubJsonDns != "" {
		t.Fatalf("expected empty default, got %q", settings.SubJsonDns)
	}

	settings.SubJsonDns = `["https://dns.google/dns-query"]`
	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetSubJsonDns()
	if err != nil || got != `["https://dns.google/dns-query"]` {
		t.Fatalf("expected the stored DNS list, got %q, err %v", got, err)
	}
}
