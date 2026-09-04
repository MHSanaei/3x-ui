package psiphon

import (
	"encoding/json"
	"os"
	"testing"
)

func TestApplyForcedFields(t *testing.T) {
	// Simulates a config copied from the Docker deployment: "any" binds
	// 0.0.0.0, exactly the mistake applyForcedFields exists to correct.
	raw := map[string]any{
		"ListenInterface":        "any",
		"LocalSocksProxyPort":    float64(1080),
		"DisableLocalSocksProxy": true,
		"DisableLocalHTTPProxy":  false,
		"PropagationChannelId":   "kept-untouched",
	}
	applyForcedFields(raw)

	want := map[string]any{
		"LocalSocksProxyPort":    SocksPort,
		"DisableLocalSocksProxy": false,
		"DisableLocalHTTPProxy":  true,
		"ListenInterface":        "",
	}
	for k, v := range want {
		if raw[k] != v {
			t.Errorf("applyForcedFields: raw[%q] = %v, want %v", k, raw[k], v)
		}
	}
	if raw["PropagationChannelId"] != "kept-untouched" {
		t.Errorf("applyForcedFields touched an admin field it should leave alone: %v", raw["PropagationChannelId"])
	}
}

// A literal "null" unmarshals into a nil map with no error; applyForcedFields
// then panics assigning into it, so SaveConfig must reject it first.
func TestSaveConfigRejectsNonObjectJSON(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	for _, tc := range []struct {
		name string
		body string
	}{
		{"null", "null"},
		{"array", "[1,2,3]"},
		{"number", "42"},
		{"string", `"just a string"`},
		{"not json at all", "definitely not json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := SaveConfig([]byte(tc.body)); err == nil {
				t.Fatalf("SaveConfig(%q) returned nil error, want a rejection", tc.body)
			}
		})
	}
}

func TestSaveConfigAndEgressRegionRoundTrip(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())

	cfg := map[string]any{
		"PropagationChannelId": "abc123",
		"SponsorId":            "def456",
		"EgressRegion":         "BE",
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshaling test config: %v", err)
	}
	if err := SaveConfig(raw); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if !IsConfigured() {
		t.Fatal("IsConfigured() = false after a successful SaveConfig")
	}

	region, err := EgressRegion()
	if err != nil {
		t.Fatalf("EgressRegion: %v", err)
	}
	if region != "BE" {
		t.Errorf("EgressRegion() = %q, want %q", region, "BE")
	}

	if err := SetEgressRegion("JP"); err != nil {
		t.Fatalf("SetEgressRegion: %v", err)
	}
	region, err = EgressRegion()
	if err != nil {
		t.Fatalf("EgressRegion after SetEgressRegion: %v", err)
	}
	if region != "JP" {
		t.Errorf("EgressRegion() after SetEgressRegion(%q) = %q, want %q", "JP", region, "JP")
	}

	// SetEgressRegion re-applies the forced fields too, not just the region.
	raw, err = os.ReadFile(ConfigPath())
	if err != nil {
		t.Fatalf("reading config back: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parsing config back: %v", err)
	}
	if parsed["ListenInterface"] != "" {
		t.Errorf("ListenInterface after SetEgressRegion = %v, want empty (loopback)", parsed["ListenInterface"])
	}
}
