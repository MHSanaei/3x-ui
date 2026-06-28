package xray

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	wireguard "github.com/xtls/xray-core/proxy/wireguard"
	"google.golang.org/protobuf/proto"
)

func b64Key(seed byte) string {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = seed + byte(i)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func decodeWgAccount(t *testing.T, user map[string]any) *wireguard.PeerConfig {
	t.Helper()
	tm, err := buildUserAccount("wireguard", user)
	if err != nil {
		t.Fatalf("buildUserAccount: %v", err)
	}
	if tm == nil {
		t.Fatal("buildUserAccount returned nil account for wireguard")
	}
	var pc wireguard.PeerConfig
	if err := proto.Unmarshal(tm.Value, &pc); err != nil {
		t.Fatalf("unmarshal PeerConfig: %v", err)
	}
	return &pc
}

func assertHexKey(t *testing.T, label, value string) {
	t.Helper()
	if len(value) != 64 {
		t.Fatalf("%s = %q, want 64-char hex", label, value)
	}
	if raw, err := hex.DecodeString(value); err != nil || len(raw) != 32 {
		t.Fatalf("%s is not a 32-byte hex key: err=%v len=%d", label, err, len(raw))
	}
}

func TestBuildUserAccountWireGuardHexConversion(t *testing.T) {
	pub := b64Key(1)
	psk := b64Key(100)
	user := map[string]any{
		"email":        "alice@example.test",
		"publicKey":    pub,
		"preSharedKey": psk,
		"allowedIPs":   []any{"10.0.0.2/32", "fd00::2/128"},
		"keepAlive":    "25",
	}

	pc := decodeWgAccount(t, user)
	assertHexKey(t, "PublicKey", pc.PublicKey)
	assertHexKey(t, "PreSharedKey", pc.PreSharedKey)

	wantPubHex, _ := hex.DecodeString(pc.PublicKey)
	gotPub, _ := base64.StdEncoding.DecodeString(pub)
	if string(wantPubHex) != string(gotPub) {
		t.Fatal("PublicKey hex does not match the base64 input bytes")
	}

	if len(pc.AllowedIps) != 2 || pc.AllowedIps[0] != "10.0.0.2/32" || pc.AllowedIps[1] != "fd00::2/128" {
		t.Fatalf("AllowedIps = %v, want [10.0.0.2/32 fd00::2/128]", pc.AllowedIps)
	}
	if pc.KeepAlive != "25" {
		t.Fatalf("KeepAlive = %q, want %q", pc.KeepAlive, "25")
	}
}

func TestBuildUserAccountWireGuardNoPSK(t *testing.T) {
	user := map[string]any{
		"email":      "bob@example.test",
		"publicKey":  b64Key(2),
		"allowedIPs": []string{"10.0.0.3/32"},
	}
	pc := decodeWgAccount(t, user)
	if pc.PreSharedKey != "" {
		t.Fatalf("PreSharedKey = %q, want empty", pc.PreSharedKey)
	}
	if pc.KeepAlive != "" {
		t.Fatalf("KeepAlive = %q, want empty", pc.KeepAlive)
	}
}

func TestBuildUserAccountWireGuardMissingPublicKey(t *testing.T) {
	user := map[string]any{
		"email":      "c@example.test",
		"allowedIPs": []any{"10.0.0.4/32"},
	}
	if _, err := buildUserAccount("wireguard", user); err == nil {
		t.Fatal("expected error for missing publicKey")
	}
}

func TestBuildUserAccountWireGuardMissingAllowedIPs(t *testing.T) {
	user := map[string]any{
		"email":     "d@example.test",
		"publicKey": b64Key(3),
	}
	if _, err := buildUserAccount("wireguard", user); err == nil {
		t.Fatal("expected error for missing allowedIPs")
	}
}

func TestBuildUserAccountWireGuardBadKey(t *testing.T) {
	user := map[string]any{
		"email":      "e@example.test",
		"publicKey":  "not-a-valid-key",
		"allowedIPs": []any{"10.0.0.5/32"},
	}
	if _, err := buildUserAccount("wireguard", user); err == nil {
		t.Fatal("expected error for invalid publicKey")
	}
}

func TestBuildUserAccountUnknownProtocolReturnsNil(t *testing.T) {
	tm, err := buildUserAccount("mtproto", map[string]any{"email": "x@example.test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm != nil {
		t.Fatal("expected nil account for unsupported protocol")
	}
}
