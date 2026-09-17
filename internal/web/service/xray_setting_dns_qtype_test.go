package service

import (
	"encoding/json"
	"testing"
)

// The template editor saves raw JSON that never passes the outbound form's
// adapter, and the core reads a numeric qType 0 as every query.
func TestSaveXraySettingSpellsDNSQTypeZeroAsString(t *testing.T) {
	setupSettingTestDB(t)
	svc := &XraySettingService{}
	template := `{"outbounds":[{"tag":"dns-out","protocol":"dns","settings":{"rules":[{"action":"drop","qType":0},{"action":"hijack","qType":28}]}}]}`

	if err := svc.SaveXraySetting(template); err != nil {
		t.Fatalf("SaveXraySetting: %v", err)
	}

	stored, err := svc.GetXrayConfigTemplate()
	if err != nil {
		t.Fatalf("GetXrayConfigTemplate: %v", err)
	}
	var cfg struct {
		Outbounds []struct {
			Settings struct {
				Rules []map[string]any `json:"rules"`
			} `json:"settings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(stored), &cfg); err != nil || len(cfg.Outbounds) != 1 {
		t.Fatalf("stored template unreadable (%v): %s", err, stored)
	}
	rules := cfg.Outbounds[0].Settings.Rules
	if len(rules) != 2 || rules[0]["qType"] != "0" || rules[1]["qType"] != float64(28) {
		t.Fatalf("stored dns rules = %v, want qType \"0\" then the untouched 28", rules)
	}
}
