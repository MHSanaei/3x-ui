package service

import (
	"reflect"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/reflect_util"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func allSettingJSONTags(t *testing.T) map[string]bool {
	t.Helper()
	tags := make(map[string]bool)
	for _, field := range reflect_util.GetFields(reflect.TypeFor[entity.AllSetting]()) {
		if tag := field.Tag.Get("json"); tag != "" {
			tags[tag] = true
		}
	}
	return tags
}

func TestGetFactoryDefaultsExposesBrowserSafeKeys(t *testing.T) {
	defaults := (&SettingService{}).GetFactoryDefaults()

	tests := []struct {
		key  string
		want string
	}{
		{key: "webPort", want: "2053"},
		{key: "subPort", want: "2096"},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			got, ok := defaults[tc.key]
			if !ok {
				t.Fatalf("expected key %q in factory defaults", tc.key)
			}
			if got != tc.want {
				t.Errorf("factory default for %q = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestGetFactoryDefaultsOmitsSensitiveMaterial(t *testing.T) {
	defaults := (&SettingService{}).GetFactoryDefaults()

	for _, key := range []string{
		"secret",
		"panelGuid",
		"nodeMtlsCaCertPem",
		"nodeMtlsCaKeyPem",
		"nodeMtlsClientCertPem",
		"nodeMtlsClientKeyPem",
		"xrayTemplateConfig",
		"tgBotToken",
		"twoFactorToken",
		"ldapPassword",
		"smtpPassword",
	} {
		t.Run(key, func(t *testing.T) {
			if _, ok := defaults[key]; ok {
				t.Errorf("factory defaults must not expose %q", key)
			}
		})
	}
}

func TestGetFactoryDefaultsInvariant(t *testing.T) {
	defaults := (&SettingService{}).GetFactoryDefaults()
	tags := allSettingJSONTags(t)

	for key := range defaults {
		t.Run(key, func(t *testing.T) {
			if !tags[key] {
				t.Errorf("key %q is not an entity.AllSetting json tag", key)
			}
			if factoryDefaultSecretKeys[key] {
				t.Errorf("key %q is in the credential deny-list and must not be returned", key)
			}
		})
	}
}
