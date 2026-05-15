package runtime

import (
	"encoding/json"
	"testing"
)

func TestSanitizeStreamSettingsForRemote(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		// wantCertFile / wantKeyFile: expected presence after sanitize
		wantCertFile bool
		wantKeyFile  bool
	}{
		{
			name: "file paths only — kept intact (remote node paths)",
			input: `{
				"tlsSettings": {
					"certificates": [{
						"certificateFile": "/etc/ssl/cert.crt",
						"keyFile": "/etc/ssl/key.key"
					}]
				}
			}`,
			wantCertFile: true,
			wantKeyFile:  true,
		},
		{
			name: "inline content only — unchanged",
			input: `{
				"tlsSettings": {
					"certificates": [{
						"certificate": ["-----BEGIN CERTIFICATE-----"],
						"key": ["-----BEGIN PRIVATE KEY-----"]
					}]
				}
			}`,
			wantCertFile: false,
			wantKeyFile:  false,
		},
		{
			name: "both file paths and inline content — file paths stripped (redundant)",
			input: `{
				"tlsSettings": {
					"certificates": [{
						"certificateFile": "/etc/ssl/cert.crt",
						"keyFile": "/etc/ssl/key.key",
						"certificate": ["-----BEGIN CERTIFICATE-----"],
						"key": ["-----BEGIN PRIVATE KEY-----"]
					}]
				}
			}`,
			wantCertFile: false,
			wantKeyFile:  false,
		},
		{
			name: "empty stream settings",
			input: "",
			// empty input returns empty, nothing to check
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.input == "" {
				if got := sanitizeStreamSettingsForRemote(tc.input); got != "" {
					t.Errorf("expected empty string, got %q", got)
				}
				return
			}
			got := sanitizeStreamSettingsForRemote(tc.input)
			var out map[string]any
			if err := json.Unmarshal([]byte(got), &out); err != nil {
				t.Fatalf("output is not valid JSON: %v\noutput: %s", err, got)
			}

			tls, _ := out["tlsSettings"].(map[string]any)
			certs, _ := tls["certificates"].([]any)
			if len(certs) == 0 {
				t.Fatal("certificates array missing in output")
			}
			cert, _ := certs[0].(map[string]any)

			_, hasCertFile := cert["certificateFile"]
			_, hasKeyFile := cert["keyFile"]

			if hasCertFile != tc.wantCertFile {
				t.Errorf("certificateFile present=%v, want %v", hasCertFile, tc.wantCertFile)
			}
			if hasKeyFile != tc.wantKeyFile {
				t.Errorf("keyFile present=%v, want %v", hasKeyFile, tc.wantKeyFile)
			}
		})
	}
}
