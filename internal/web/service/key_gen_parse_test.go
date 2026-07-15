package service

import "testing"

func TestParseXrayKeyPairOutput(t *testing.T) {
	a, b, err := parseXrayKeyPairOutput("Private key: abc123\nPublic key: def456\n")
	if err != nil {
		t.Fatalf("well-formed output errored: %v", err)
	}
	if a != "abc123" || b != "def456" {
		t.Fatalf("got (%q, %q), want (abc123, def456)", a, b)
	}

	malformed := []string{
		"",
		"only one line: value",
		"Private key: abc\n",
		"no colon here\nno colon two",
		"Private key\nPublic key",
	}
	for _, out := range malformed {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("parseXrayKeyPairOutput panicked on %q: %v", out, r)
				}
			}()
			if _, _, err := parseXrayKeyPairOutput(out); err == nil {
				t.Errorf("expected error for malformed output %q, got nil", out)
			}
		}()
	}
}
