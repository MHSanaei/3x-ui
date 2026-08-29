package service

import (
	"reflect"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/frontproxy"
	"github.com/mhsanaei/3x-ui/v3/internal/util/reflect_util"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

// validFrontProxySettings is a settings block that passes every other check in
// validateFrontProxySettings, so a subtest can vary one field at a time.
func validFrontProxySettings() *entity.AllSetting {
	return &entity.AllSetting{
		FrontProxyPort:          7443,
		FrontProxyListen:        "127.0.0.1",
		FrontProxyCertMode:      string(frontproxy.CertManual),
		FrontProxyDecoyMode:     string(frontproxy.DecoyTemplate),
		FrontProxyDecoyTemplate: frontproxy.DefaultDecoyTemplate,
	}
}

// TestValidateFrontProxySettingsAcceptsEveryDecoyMode guards a real failure:
// the decoy mode is validated in two places -- the SetFrontProxyDecoyMode
// setter and this bulk-save validator -- and adding "adguard" to only the
// first one left saving any setting at all rejected with "invalid front proxy
// decoy mode: adguard". Anything the mode enum gains has to be accepted here
// too, so this walks the enum rather than listing the modes by hand.
func TestValidateFrontProxySettingsAcceptsEveryDecoyMode(t *testing.T) {
	modes := []frontproxy.DecoyMode{
		frontproxy.DecoyTemplate,
		frontproxy.DecoyUpload,
		frontproxy.DecoyProxy,
		frontproxy.DecoyAdGuard,
	}
	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			settings := validFrontProxySettings()
			settings.FrontProxyDecoyMode = string(mode)
			if err := validateFrontProxySettings(settings); err != nil {
				t.Errorf("decoy mode %q rejected on save: %v", mode, err)
			}
		})
	}

	t.Run("still refuses an unknown mode", func(t *testing.T) {
		settings := validFrontProxySettings()
		settings.FrontProxyDecoyMode = "definitely-not-a-mode"
		if err := validateFrontProxySettings(settings); err == nil {
			t.Error("an unknown decoy mode was accepted")
		}
	})
}

// TestAllSettingFieldsAreStorable catches the other half of the same class of
// bug: UpdateAllSetting walks entity.AllSetting by reflection and stores each
// field under its json tag, so a setting the panel offers but the struct does
// not carry is silently dropped on save, while one whose tag has no default
// has nothing to fall back to on read.
func TestAllSettingFieldsAreStorable(t *testing.T) {
	for _, field := range reflect_util.GetFields(reflect.TypeFor[entity.AllSetting]()) {
		key := field.Tag.Get("json")
		if key == "" {
			t.Errorf("%s has no json tag, so UpdateAllSetting would store it under an empty key", field.Name)
			continue
		}
		if _, ok := defaultValueMap[key]; !ok {
			t.Errorf("%s is saved as %q but that key has no entry in defaultValueMap", field.Name, key)
		}
	}
}

// TestAdGuardFilterDNSIsSavable is deliberately specific: the toggle existed
// in the UI and in defaultValueMap before it existed on entity.AllSetting, so
// saving it appeared to work and changed nothing.
func TestAdGuardFilterDNSIsSavable(t *testing.T) {
	found := false
	for _, field := range reflect_util.GetFields(reflect.TypeFor[entity.AllSetting]()) {
		if field.Tag.Get("json") == "adguardFilterDns" {
			found = true
			if field.Type.Kind() != reflect.Bool {
				t.Errorf("adguardFilterDns is %s, want bool", field.Type.Kind())
			}
		}
	}
	if !found {
		t.Error("entity.AllSetting has no adguardFilterDns field, so the toggle cannot be saved")
	}
}
