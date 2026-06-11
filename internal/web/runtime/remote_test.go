package runtime

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// cacheGetTag must resolve a remote inbound id even when the n<id>- prefix
// sits on only one side: the node may store the bare tag while the central
// panel pushes the prefixed form, or vice versa. Without this a mismatch makes
// the push create a duplicate inbound on the node.
func TestCacheGetTag_PrefixAgnostic(t *testing.T) {
	cases := []struct {
		name      string
		cacheTag  string
		lookup    string
		wantID    int
		wantFound bool
	}{
		{"exact", "n1-in-443-tcp", "n1-in-443-tcp", 7, true},
		{"node bare, lookup prefixed", "in-443-tcp", "n1-in-443-tcp", 7, true},
		{"node prefixed, lookup bare", "n1-in-443-tcp", "in-443-tcp", 7, true},
		{"unrelated tag", "in-443-tcp", "in-999-tcp", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NewRemote(&model.Node{Id: 1, Name: "n1"})
			r.cacheSet(c.cacheTag, 7)
			id, ok := r.cacheGetTag(c.lookup)
			if ok != c.wantFound || id != c.wantID {
				t.Fatalf("cacheGetTag(%q) = (%d, %v), want (%d, %v)", c.lookup, id, ok, c.wantID, c.wantFound)
			}
		})
	}
}

func TestWireInboundIncludesShareAddressFields(t *testing.T) {
	values := wireInbound(&model.Inbound{
		ShareAddrStrategy: "custom",
		ShareAddr:         "edge.example.com",
	})

	if got := values.Get("shareAddrStrategy"); got != "custom" {
		t.Fatalf("shareAddrStrategy = %q, want custom", got)
	}
	if got := values.Get("shareAddr"); got != "edge.example.com" {
		t.Fatalf("shareAddr = %q, want edge.example.com", got)
	}
}

func TestWireInboundDefaultsShareAddressStrategy(t *testing.T) {
	values := wireInbound(&model.Inbound{})

	if got := values.Get("shareAddrStrategy"); got != "node" {
		t.Fatalf("shareAddrStrategy = %q, want node", got)
	}

	values = wireInbound(&model.Inbound{ShareAddrStrategy: "auto"})
	if got := values.Get("shareAddrStrategy"); got != "node" {
		t.Fatalf("invalid shareAddrStrategy = %q, want node", got)
	}
}

func TestSanitizeStreamSettingsForRemote(t *testing.T) {
	tests := []struct {
		name  string
		input string
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
			name:  "empty stream settings",
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
