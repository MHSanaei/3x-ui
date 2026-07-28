package sub

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestAmbiguousScalarsAreQuoted(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		mustQuote bool
	}{
		{"reality short-id read as a float", "2351e1", true},
		{"short exponent form", "0e1", true},
		{"long exponent form", "12e34", true},
		{"all digits", "123456", true},
		{"leading zeros read as octal", "0177", true},
		{"hex form", "0x1f", true},
		{"decimal point", "1.5", true},
		{"boolean word", "true", true},
		{"single letter bool", "y", true},
		{"null word", "null", true},
		{"tilde", "~", true},
		{"date", "2026-07-27", true},
		{"sexagesimal", "12:30", true},
		{"plain hex with letters", "6ba7b8", false},
		{"letters only", "abcdef", false},
		{"hostname", "example.com", false},
		{"dotted quad", "1.2.3.4", false},
		{"uuid", "6ba7b810-9dad-11d1-80b4-00c04fd430c8", false},
		{"alpn token", "h2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := marshalClashYAML(map[string]any{
				"reality-opts": map[string]any{"short-id": tt.value},
			})
			if err != nil {
				t.Fatalf("marshalClashYAML: %v", err)
			}
			got := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "reality-opts:"))
			quoted := strings.Contains(got, "'"+tt.value+"'") || strings.Contains(got, `"`+tt.value+`"`)
			if tt.mustQuote && !quoted {
				t.Errorf("%q must be emitted quoted, got %q", tt.value, got)
			}
			if !tt.mustQuote && quoted {
				t.Errorf("%q needs no quoting, got %q", tt.value, got)
			}
			var decoded map[string]any
			if err := yaml.Unmarshal(out, &decoded); err != nil {
				t.Fatalf("output must stay parseable: %v\n%s", err, out)
			}
		})
	}
}

func TestClashShortIDIsQuotedInOutput(t *testing.T) {
	out, err := marshalClashYAML(map[string]any{
		"proxies": []any{map[string]any{
			"name":         "de-1",
			"reality-opts": map[string]any{"short-id": "2351e1"},
		}},
	})
	if err != nil {
		t.Fatalf("marshalClashYAML: %v", err)
	}
	if !strings.Contains(string(out), `'2351e1'`) {
		t.Errorf("short-id must be emitted as a quoted scalar, got:\n%s", out)
	}
}

func TestAmbiguousPasswordIsQuoted(t *testing.T) {
	out, err := marshalClashYAML(map[string]any{
		"proxies": []any{map[string]any{
			"name":           "de-1",
			"password":       "80e12",
			"obfs-password":  "12345",
			"pre-shared-key": "9e9",
		}},
	})
	if err != nil {
		t.Fatalf("marshalClashYAML: %v", err)
	}
	for _, want := range []string{`'80e12'`, `'12345'`, `'9e9'`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("expected %s in output:\n%s", want, out)
		}
	}
}

func TestUnambiguousScalarsAreUnchanged(t *testing.T) {
	config := map[string]any{
		"port":     7890,
		"mode":     "rule",
		"proxies":  []any{map[string]any{"name": "de-1", "udp": true, "port": 443}},
		"rules":    []string{"MATCH,PROXY"},
		"alpn":     []string{"h2", "http/1.1"},
		"nonempty": "example.com",
	}
	quoted, err := marshalClashYAML(config)
	if err != nil {
		t.Fatalf("marshalClashYAML: %v", err)
	}
	plain, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if string(quoted) != string(plain) {
		t.Errorf("unambiguous document changed:\n--- got ---\n%s\n--- want ---\n%s", quoted, plain)
	}
}

func TestQuotedScalarEscapesQuotes(t *testing.T) {
	out, err := marshalClashYAML(map[string]any{"password": "1e2'3"})
	if err != nil {
		t.Fatalf("marshalClashYAML: %v", err)
	}
	var decoded map[string]any
	if err := yaml.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output must stay parseable: %v\n%s", err, out)
	}
	if got := decoded["password"]; got != any("1e2'3") {
		t.Errorf("password round-tripped to %#v\n%s", got, out)
	}
}
