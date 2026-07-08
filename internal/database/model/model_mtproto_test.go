package model

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestGenerateFakeTLSSecret(t *testing.T) {
	domain := "www.cloudflare.com"
	s := GenerateFakeTLSSecret(domain)
	if !strings.HasPrefix(s, "ee") {
		t.Fatalf("secret must start with ee, got %q", s)
	}
	wantSuffix := hex.EncodeToString([]byte(domain))
	if !strings.HasSuffix(s, wantSuffix) {
		t.Fatalf("secret must end with hex(domain) %q, got %q", wantSuffix, s)
	}
	if len(s) != 2+32+len(wantSuffix) {
		t.Fatalf("unexpected secret length %d", len(s))
	}
	if _, err := hex.DecodeString(s[2:34]); err != nil {
		t.Fatalf("middle is not valid hex: %v", err)
	}
}

func TestStripMtprotoInboundAdTag(t *testing.T) {
	in := `{"adTag":"0123456789abcdef0123456789abcdef","clients":[{"email":"a","adTag":"fedcba9876543210fedcba9876543210"}]}`
	out, changed := StripMtprotoInboundAdTag(in)
	if !changed {
		t.Fatal("expected the inbound-level adTag to be stripped")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := parsed["adTag"]; ok {
		t.Fatalf("adTag key must be removed, got %s", out)
	}
	clients := parsed["clients"].([]any)
	if clients[0].(map[string]any)["adTag"] != "fedcba9876543210fedcba9876543210" {
		t.Fatalf("client adTag must be preserved, got %s", out)
	}
	if _, changed2 := StripMtprotoInboundAdTag(out); changed2 {
		t.Fatal("second strip must be a no-op")
	}
}

func TestStripMtprotoInboundSecret(t *testing.T) {
	// A multi-client inbound that still carries a dead inbound-level secret has
	// it removed while the clients (and every other key) survive untouched.
	in := `{"fakeTlsDomain":"a.com","secret":"eedeadbeef","clients":[{"email":"x","secret":"eeaaaa"}]}`
	out, changed := StripMtprotoInboundSecret(in)
	if !changed {
		t.Fatal("expected the inbound-level secret to be stripped")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("stripped settings not valid json: %v", err)
	}
	if _, ok := parsed["secret"]; ok {
		t.Fatalf("inbound-level secret should be gone, got %q", out)
	}
	if parsed["fakeTlsDomain"] != "a.com" {
		t.Fatalf("fakeTlsDomain must survive, got %q", out)
	}
	clients, ok := parsed["clients"].([]any)
	if !ok || len(clients) != 1 {
		t.Fatalf("clients must survive untouched, got %q", out)
	}
	if clients[0].(map[string]any)["secret"] != "eeaaaa" {
		t.Fatalf("client secret must survive untouched, got %q", out)
	}

	// Nothing to strip when there is no inbound-level secret.
	if _, changed2 := StripMtprotoInboundSecret(out); changed2 {
		t.Fatal("expected no change when there is no inbound-level secret")
	}
	if _, changed3 := StripMtprotoInboundSecret(`{"clients":[]}`); changed3 {
		t.Fatal("expected no change for settings without a secret key")
	}
}

func TestHealMtprotoClientSecrets(t *testing.T) {
	// An empty client secret is filled from the inbound-level default domain.
	in := `{"fakeTlsDomain":"a.com","clients":[{"email":"x","secret":""}]}`
	out, changed := HealMtprotoClientSecrets(in)
	if !changed {
		t.Fatal("expected an empty client secret to be filled")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("healed settings not valid json: %v", err)
	}
	clients := parsed["clients"].([]any)
	got := clients[0].(map[string]any)["secret"].(string)
	if !strings.HasPrefix(got, "ee") || !strings.HasSuffix(got, hex.EncodeToString([]byte("a.com"))) {
		t.Fatalf("filled client secret malformed: %q", got)
	}

	// Healing is idempotent once every client secret is valid.
	if _, changed2 := HealMtprotoClientSecrets(out); changed2 {
		t.Fatal("expected no change for already-valid client secrets")
	}

	// A client's own embedded domain is preserved even when it differs from the
	// inbound-level default (per-client domain fronting).
	own := "ee00112233445566778899aabbccddeeff" + hex.EncodeToString([]byte("b.com"))
	in3 := `{"fakeTlsDomain":"a.com","clients":[{"email":"y","secret":"` + own + `"}]}`
	out3, changed3 := HealMtprotoClientSecrets(in3)
	if changed3 {
		t.Fatalf("a valid per-client secret must be left untouched, got %q", out3)
	}

	// No clients array — nothing to heal.
	if _, changed4 := HealMtprotoClientSecrets(`{"fakeTlsDomain":"a.com"}`); changed4 {
		t.Fatal("expected no change when there are no clients")
	}
}
