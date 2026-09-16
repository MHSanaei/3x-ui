package service

import (
	"encoding/json"
	"testing"
)

// The embedded template is what every fresh install starts from, so it must not
// carry the freedom strategy keys the core warns about on every config load.
func TestDefaultXrayTemplateKeepsFreedomStrategyOutOfTheLegacyKeys(t *testing.T) {
	var cfg struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(xrayTemplateConfig), &cfg); err != nil {
		t.Fatalf("embedded config.json is not JSON: %v", err)
	}
	freedom := 0
	for _, ob := range cfg.Outbounds {
		if ob["protocol"] != "freedom" {
			continue
		}
		freedom++
		tag := ob["tag"]
		if _, ok := ob["targetStrategy"]; ok {
			t.Errorf("freedom outbound %v carries the outbound-root targetStrategy", tag)
		}
		settings, _ := ob["settings"].(map[string]any)
		for _, key := range []string{"domainStrategy", "targetStrategy"} {
			if _, ok := settings[key]; ok {
				t.Errorf("freedom outbound %v carries the deprecated settings.%s", tag, key)
			}
		}
	}
	if freedom == 0 {
		t.Fatal("the default template has no freedom outbound to check")
	}
}
