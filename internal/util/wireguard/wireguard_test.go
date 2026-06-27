package wireguard

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestGenerateWireguardKeypairRoundTrip(t *testing.T) {
	priv, pub, err := GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("GenerateWireguardKeypair: %v", err)
	}
	for name, key := range map[string]string{"private": priv, "public": pub} {
		raw, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			t.Fatalf("%s key not base64: %v", name, err)
		}
		if len(raw) != 32 {
			t.Fatalf("%s key decodes to %d bytes, want 32", name, len(raw))
		}
	}

	derived, err := PublicKeyFromPrivate(priv)
	if err != nil {
		t.Fatalf("PublicKeyFromPrivate: %v", err)
	}
	if derived != pub {
		t.Fatalf("PublicKeyFromPrivate(priv) = %q, want %q", derived, pub)
	}
}

func TestPublicKeyFromPrivateKnownVector(t *testing.T) {
	privHex := "77076d0a7318a57d3c16c17251b26645df4c2f87ebc0992ab177fba51db92c2a"
	wantPubHex := "8520f0098930a754748b7ddcb43ef75a0dbf3a0d26381af4eba4a98eaa9b4e6a"

	privBytes, err := hex.DecodeString(privHex)
	if err != nil {
		t.Fatalf("decode priv vector: %v", err)
	}
	pubB64, err := PublicKeyFromPrivate(base64.StdEncoding.EncodeToString(privBytes))
	if err != nil {
		t.Fatalf("PublicKeyFromPrivate: %v", err)
	}
	gotPubHex, err := KeyToHex(pubB64)
	if err != nil {
		t.Fatalf("KeyToHex: %v", err)
	}
	if gotPubHex != wantPubHex {
		t.Fatalf("derived public key hex = %q, want %q", gotPubHex, wantPubHex)
	}
}

func TestKeyToHex(t *testing.T) {
	low := make([]byte, 32)
	for i := range low {
		low[i] = byte(i)
	}
	high := make([]byte, 32)
	for i := range high {
		high[i] = 0xff
	}

	for _, raw := range [][]byte{low, high} {
		wantHex := hex.EncodeToString(raw)
		std := base64.StdEncoding.EncodeToString(raw)
		url := base64.URLEncoding.EncodeToString(raw)
		padless := strings.TrimRight(std, "=")
		for label, in := range map[string]string{"std": std, "url": url, "padless": padless, "hex": wantHex} {
			got, err := KeyToHex(in)
			if err != nil {
				t.Fatalf("KeyToHex(%s=%q): %v", label, in, err)
			}
			if got != wantHex {
				t.Fatalf("KeyToHex(%s) = %q, want %q", label, got, wantHex)
			}
			if back, err := hex.DecodeString(got); err != nil || len(back) != 32 {
				t.Fatalf("KeyToHex output not a 32-byte hex key: err=%v len=%d", err, len(back))
			}
		}
	}
}

func TestKeyToHexEmpty(t *testing.T) {
	got, err := KeyToHex("")
	if err != nil {
		t.Fatalf("KeyToHex(\"\"): %v", err)
	}
	if got != "" {
		t.Fatalf("KeyToHex(\"\") = %q, want empty", got)
	}
}

func TestKeyToHexRejectsBadInput(t *testing.T) {
	cases := map[string]string{
		"not base64":   "this is not base64 @@@@",
		"wrong length": base64.StdEncoding.EncodeToString(make([]byte, 16)),
	}
	for name, in := range cases {
		if _, err := KeyToHex(in); err == nil {
			t.Fatalf("KeyToHex(%s=%q) expected error, got nil", name, in)
		}
	}
}

func TestGenerateWireguardPSK(t *testing.T) {
	a, err := GenerateWireguardPSK()
	if err != nil {
		t.Fatalf("GenerateWireguardPSK: %v", err)
	}
	b, err := GenerateWireguardPSK()
	if err != nil {
		t.Fatalf("GenerateWireguardPSK: %v", err)
	}
	if a == b {
		t.Fatalf("two PSKs are identical: %q", a)
	}
	raw, err := base64.StdEncoding.DecodeString(a)
	if err != nil || len(raw) != 32 {
		t.Fatalf("PSK not a 32-byte base64 key: err=%v len=%d", err, len(raw))
	}
}
