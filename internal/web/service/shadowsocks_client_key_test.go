package service

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

// A method switch between SS-2022 ciphers of different key sizes must regenerate
// client PSKs whose length no longer matches; otherwise xray rejects the user.
func TestNormalizeShadowsocksClientKeys_RegeneratesOnMethodResize(t *testing.T) {
	// 32-byte (aes-256-sized) client key under an aes-128 (16-byte) method.
	oversized := base64.StdEncoding.EncodeToString(make([]byte, 32))
	settings := `{"method":"2022-blake3-aes-128-gcm","password":"` +
		base64.StdEncoding.EncodeToString(make([]byte, 16)) +
		`","clients":[{"email":"a","password":"` + oversized + `"}]}`

	out, changed := normalizeShadowsocksClientKeys(settings)
	if !changed {
		t.Fatalf("expected mismatched client key to be regenerated")
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	clients := m["clients"].([]any)
	pw := clients[0].(map[string]any)["password"].(string)
	if pw == oversized {
		t.Fatalf("client key was not regenerated")
	}
	if decoded, err := base64.StdEncoding.DecodeString(pw); err != nil || len(decoded) != 16 {
		t.Fatalf("regenerated key must be 16 bytes for aes-128, got len=%d err=%v", len(decoded), err)
	}
}

// A correctly-sized key (and non-2022 / legacy settings) must pass through untouched.
func TestNormalizeShadowsocksClientKeys_NoChangeWhenValid(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString(make([]byte, 32))
	settings := `{"method":"2022-blake3-aes-256-gcm","clients":[{"email":"a","password":"` + valid + `"}]}`
	if out, changed := normalizeShadowsocksClientKeys(settings); changed || out != settings {
		t.Fatalf("valid aes-256 key must be left unchanged")
	}

	legacy := `{"method":"aes-256-gcm","clients":[{"email":"a","password":"anything"}]}`
	if out, changed := normalizeShadowsocksClientKeys(legacy); changed || out != legacy {
		t.Fatalf("legacy (non-2022) SS settings must be left unchanged")
	}
}
