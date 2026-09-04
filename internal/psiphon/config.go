package psiphon

import (
	"encoding/json"
	"fmt"
	"os"
)

// applyForcedFields overwrites the psiphon.config keys the admin's own paste
// must never control, mirroring tor.renderTorrc's forced ClientOnly=1.
func applyForcedFields(raw map[string]any) {
	raw["LocalSocksProxyPort"] = SocksPort
	raw["DisableLocalSocksProxy"] = false
	// Only a socks outbound is ever used -- same minimal surface as internal/tor.
	raw["DisableLocalHTTPProxy"] = true
	// Loopback-only: a config copied from the Docker deployment commonly
	// carries "any", which would bind 0.0.0.0 here instead.
	raw["ListenInterface"] = ""
}

// SaveConfig validates and patches the admin-supplied psiphon.config, writing
// it to ConfigPath. Does not restart a running process -- call Manager.Restart for that.
func SaveConfig(raw []byte) error {
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("not a valid psiphon.config: %w", err)
	}
	if parsed == nil {
		return fmt.Errorf("not a valid psiphon.config: expected a JSON object")
	}
	applyForcedFields(parsed)
	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return fmt.Errorf("re-encoding psiphon.config: %w", err)
	}
	if err := os.MkdirAll(configDir(), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", configDir(), err)
	}
	// 0600: identifies one specific credential to Psiphon Inc., same treatment as the Tor control cookie.
	return os.WriteFile(ConfigPath(), out, 0o600)
}

// EgressRegion reads back the ISO 3166-1 alpha-2 code the config requests, or "" for auto.
func EgressRegion() (string, error) {
	raw, err := os.ReadFile(ConfigPath())
	if err != nil {
		return "", err
	}
	var parsed struct {
		EgressRegion string
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("cannot parse %s: %w", ConfigPath(), err)
	}
	return parsed.EgressRegion, nil
}

// SetEgressRegion patches just the EgressRegion field. Psiphon does not
// hot-reload it, so the caller restarts the process afterward.
func SetEgressRegion(region string) error {
	raw, err := os.ReadFile(ConfigPath())
	if err != nil {
		return fmt.Errorf("no config to update -- upload a psiphon.config first: %w", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("cannot parse %s: %w", ConfigPath(), err)
	}
	parsed["EgressRegion"] = region
	applyForcedFields(parsed)
	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return fmt.Errorf("re-encoding psiphon.config: %w", err)
	}
	return os.WriteFile(ConfigPath(), out, 0o600)
}
