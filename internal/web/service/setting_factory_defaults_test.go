package service

import "testing"

func TestGetFactoryDefaultsExposesBrowserSafeKeys(t *testing.T) {
	defaults := (&SettingService{}).GetFactoryDefaults()

	expected := map[string]string{
		"webPort": "2053",
		"subPort": "2096",
	}
	for key, want := range expected {
		got, ok := defaults[key]
		if !ok {
			t.Fatalf("expected key %q in factory defaults", key)
		}
		if got != want {
			t.Fatalf("factory default for %q = %q, want %q", key, got, want)
		}
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
		if _, ok := defaults[key]; ok {
			t.Fatalf("factory defaults must not expose %q", key)
		}
	}
}
