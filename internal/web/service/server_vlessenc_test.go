package service

import "testing"

func TestParseVlessEncAuthsAddsStableIDs(t *testing.T) {
	output := `
Authentication: X25519, not Post-Quantum
{
  "decryption": "mlkem768x25519plus.native.600s.server-x25519",
  "encryption": "mlkem768x25519plus.native.0rtt.client-x25519"
}

Authentication: ML-KEM-768, Post-Quantum
{
  "decryption": "mlkem768x25519plus.native.600s.server-mlkem",
  "encryption": "mlkem768x25519plus.native.0rtt.client-mlkem"
}
`

	auths := parseVlessEncAuths(output)
	if len(auths) != 2 {
		t.Fatalf("expected 2 auth blocks, got %d", len(auths))
	}

	tests := []struct {
		index      int
		id         string
		label      string
		decryption string
		encryption string
	}{
		{
			index:      0,
			id:         "x25519",
			label:      "X25519, not Post-Quantum",
			decryption: "mlkem768x25519plus.native.600s.server-x25519",
			encryption: "mlkem768x25519plus.native.0rtt.client-x25519",
		},
		{
			index:      1,
			id:         "mlkem768",
			label:      "ML-KEM-768, Post-Quantum",
			decryption: "mlkem768x25519plus.native.600s.server-mlkem",
			encryption: "mlkem768x25519plus.native.0rtt.client-mlkem",
		},
	}

	for _, test := range tests {
		auth := auths[test.index]
		if auth["id"] != test.id {
			t.Errorf("auth[%d] id = %q, want %q", test.index, auth["id"], test.id)
		}
		if auth["label"] != test.label {
			t.Errorf("auth[%d] label = %q, want %q", test.index, auth["label"], test.label)
		}
		if auth["decryption"] != test.decryption {
			t.Errorf("auth[%d] decryption = %q, want %q", test.index, auth["decryption"], test.decryption)
		}
		if auth["encryption"] != test.encryption {
			t.Errorf("auth[%d] encryption = %q, want %q", test.index, auth["encryption"], test.encryption)
		}
	}
}

func TestParseVlessEncAuthsHandlesMissingTrailingComma(t *testing.T) {
	output := `
Authentication: X25519, not Post-Quantum
"decryption": "server"
"encryption": "client"
`

	auths := parseVlessEncAuths(output)
	if len(auths) != 1 {
		t.Fatalf("expected 1 auth block, got %d", len(auths))
	}
	if auths[0]["decryption"] != "server" {
		t.Fatalf("decryption = %q, want server", auths[0]["decryption"])
	}
	if auths[0]["encryption"] != "client" {
		t.Fatalf("encryption = %q, want client", auths[0]["encryption"])
	}
}
