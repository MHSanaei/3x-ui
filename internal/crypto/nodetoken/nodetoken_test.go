package nodetoken

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRing(t *testing.T, activeID string, ids ...string) *Keyring {
	t.Helper()
	kr := &Keyring{ActiveID: activeID, Keys: map[string][keyLen]byte{}}
	for _, id := range ids {
		var k [keyLen]byte
		for i := range k {
			k[i] = byte(i) + id[len(id)-1] // deterministic and distinct for k1/k2
		}
		kr.Keys[id] = k
	}
	return kr
}

func TestRoundTrip(t *testing.T) {
	c, err := NewCodec(ModeRequired, testRing(t, "k1", "k1"))
	if err != nil {
		t.Fatal(err)
	}
	enc, err := c.Encrypt(7, "s3cret-token")
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(enc) || !strings.HasPrefix(enc, "enc:v1:k1:") {
		t.Fatalf("unexpected ciphertext form: %q", enc)
	}
	pt, err := c.Decrypt(7, enc)
	if err != nil {
		t.Fatal(err)
	}
	if pt != "s3cret-token" {
		t.Fatalf("round-trip mismatch: %q", pt)
	}
}

func TestAADBindsToNode(t *testing.T) {
	c, _ := NewCodec(ModeRequired, testRing(t, "k1", "k1"))
	enc, _ := c.Encrypt(7, "tok")
	// Decrypting under a different node id must fail (ciphertext bound to row).
	if _, err := c.Decrypt(8, enc); err == nil {
		t.Fatal("expected AAD mismatch error decrypting under wrong node id")
	}
}

func TestAADBindsSettingsApartFromNodes(t *testing.T) {
	c, _ := NewCodec(ModeRequired, testRing(t, "k1", "k1"))
	enc, err := c.EncryptBound([]byte("settings/pia_token"), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Decrypt(1, enc); err == nil {
		t.Fatal("settings/pia_token ciphertext must not decrypt under nodes/api_token/1")
	}
	pt, err := c.DecryptBound([]byte("settings/pia_token"), enc)
	if err != nil || pt != "tok" {
		t.Fatalf("pia AAD round-trip: %q err=%v", pt, err)
	}
}

func TestNonceIsRandom(t *testing.T) {
	c, _ := NewCodec(ModeRequired, testRing(t, "k1", "k1"))
	a, _ := c.Encrypt(1, "same")
	b, _ := c.Encrypt(1, "same")
	if a == b {
		t.Fatal("two encryptions of the same value produced identical ciphertext (nonce reuse)")
	}
}

func TestPlaintextPassThrough(t *testing.T) {
	// ModeOff: encrypt is a no-op, decrypt returns plaintext.
	c, _ := NewCodec(ModeOff, nil)
	enc, err := c.Encrypt(1, "plain")
	if err != nil || enc != "plain" {
		t.Fatalf("off-mode encrypt should be no-op, got %q err=%v", enc, err)
	}
	// A legacy plaintext row decrypts (passes through) in any mode.
	c2, _ := NewCodec(ModeRequired, testRing(t, "k1", "k1"))
	if pt, err := c2.Decrypt(1, "legacy-plain"); err != nil || pt != "legacy-plain" {
		t.Fatalf("legacy plaintext should pass through, got %q err=%v", pt, err)
	}
}

func TestEncryptedNeverFallsBackToPlaintext(t *testing.T) {
	c, _ := NewCodec(ModeRequired, testRing(t, "k1", "k1"))
	enc, _ := c.Encrypt(1, "tok")
	// Corrupt the ciphertext body — must error, never return raw bytes.
	bad := enc[:len(enc)-2] + "AA"
	if _, err := c.Decrypt(1, bad); err == nil {
		t.Fatal("corrupted ciphertext must fail, not fall back to plaintext")
	}
	// Unknown key id must error.
	c2, _ := NewCodec(ModeRequired, testRing(t, "k1", "k1"))
	other := strings.Replace(enc, "enc:v1:k1:", "enc:v1:zz:", 1)
	if _, err := c2.Decrypt(1, other); err == nil {
		t.Fatal("unknown key id must fail")
	}
}

func TestEncryptionMarkerPassesThroughWhenDisabled(t *testing.T) {
	c, _ := NewCodec(ModeOff, nil)
	stored := "enc:v1:not-ciphertext"
	if got, err := c.Decrypt(1, stored); err != nil || got != stored {
		t.Fatalf("off-mode changed a legacy token: got %q err=%v", got, err)
	}
}

func TestParseKeyringRejectsDelimiterInKeyID(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, keyLen))
	for _, tc := range []struct {
		name, active string
		keys         map[string]string
	}{
		{"active delimiter", "region:k1", map[string]string{"region:k1": key}},
		{"key delimiter", "k1", map[string]string{"k1": key, "old:k0": key}},
		{"empty key", "k1", map[string]string{"k1": key, "": key}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseKeyring(tc.active, tc.keys); err == nil {
				t.Fatal("invalid key id was accepted")
			}
		})
	}
}

func TestEncryptRoundTripSafe(t *testing.T) {
	// Re-submitting stored ciphertext (UI round-trip) must not double-encrypt.
	c, _ := NewCodec(ModeRequired, testRing(t, "k1", "k1"))
	enc, _ := c.Encrypt(5, "tok")
	again, err := c.Encrypt(5, enc)
	if err != nil {
		t.Fatal(err)
	}
	if again != enc {
		t.Fatal("re-encrypting stored ciphertext changed it (double-encrypt)")
	}
	if pt, _ := c.Decrypt(5, again); pt != "tok" {
		t.Fatalf("round-trip-safe encrypt corrupted token: %q", pt)
	}
}

func TestEmptyTokenNeverEncrypted(t *testing.T) {
	c, _ := NewCodec(ModeRequired, testRing(t, "k1", "k1"))
	if v, _ := c.Encrypt(1, ""); v != "" {
		t.Fatalf("empty token must stay empty, got %q", v)
	}
}

func TestRotation(t *testing.T) {
	// k2 active, k1 retained. Old-key value still decrypts; new writes use k2.
	ring := testRing(t, "k2", "k1", "k2")
	if ring.Keys["k1"] == ring.Keys["k2"] {
		t.Fatal("rotation fixture keys k1 and k2 are identical")
	}
	c, _ := NewCodec(ModeRequired, ring)
	// produce a k1 value via a codec whose active is k1
	c1, _ := NewCodec(ModeRequired, testRing(t, "k1", "k1", "k2"))
	old, _ := c1.Encrypt(3, "tok")
	if pt, err := c.Decrypt(3, old); err != nil || pt != "tok" {
		t.Fatalf("retained old key must decrypt, got %q err=%v", pt, err)
	}
	if c.EncryptedWithActive(old) {
		t.Fatal("k1 value should not count as encrypted-with-active(k2)")
	}
	neu, _ := c.Encrypt(3, "tok")
	if !c.EncryptedWithActive(neu) {
		t.Fatal("new write should be encrypted with active key")
	}
}

func TestNewCodecRequiresKey(t *testing.T) {
	if _, err := NewCodec(ModeRequired, nil); err == nil {
		t.Fatal("required mode without a key must fail (fail-closed)")
	}
	if _, err := NewCodec(ModeMigration, &Keyring{ActiveID: "x", Keys: nil}); err == nil {
		t.Fatal("migration mode with empty keyring must fail")
	}
}

func TestParseMode(t *testing.T) {
	for in, want := range map[string]Mode{"": ModeOff, "off": ModeOff, "Migration": ModeMigration, "REQUIRED": ModeRequired} {
		if m, err := ParseMode(in); err != nil || m != want {
			t.Fatalf("ParseMode(%q)=%v err=%v, want %v", in, m, err, want)
		}
	}
	if _, err := ParseMode("bogus"); err == nil {
		t.Fatal("unknown mode must error")
	}
}

func TestFileKeySourceRejectsLoosePerms(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "k.json")
	key := make([]byte, keyLen)
	body, _ := json.Marshal(keyFile{Active: "k1", Keys: map[string]string{"k1": base64.StdEncoding.EncodeToString(key)}})
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (FileKeySource{Path: p}).Load(); err == nil {
		t.Fatal("0644 key file must be rejected")
	}
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatal(err)
	}
	kr, err := (FileKeySource{Path: p}).Load()
	if err != nil {
		t.Fatalf("0600 key file should load: %v", err)
	}
	if kr.ActiveID != "k1" || len(kr.Keys) != 1 {
		t.Fatalf("unexpected keyring %+v", kr)
	}
}

func TestEnvKeySource(t *testing.T) {
	key := make([]byte, keyLen)
	for i := range key {
		key[i] = byte(i)
	}
	t.Setenv("XUI_NODE_TOKEN_KEY_TEST", base64.StdEncoding.EncodeToString(key))
	kr, err := (EnvKeySource{Var: "XUI_NODE_TOKEN_KEY_TEST"}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if kr.ActiveID != "env" {
		t.Fatalf("env key id should be 'env', got %q", kr.ActiveID)
	}
	c, _ := NewCodec(ModeRequired, kr)
	enc, _ := c.Encrypt(1, "x")
	if pt, _ := c.Decrypt(1, enc); pt != "x" {
		t.Fatal("env-sourced key failed round trip")
	}
}
