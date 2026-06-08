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

func TestHealMtprotoSecret(t *testing.T) {
	domain := "example.com"
	suffix := hex.EncodeToString([]byte(domain))

	in := `{"fakeTlsDomain":"example.com","secret":""}`
	out, changed := HealMtprotoSecret(in)
	if !changed {
		t.Fatal("expected heal to populate an empty secret")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("healed settings not valid json: %v", err)
	}
	got, _ := parsed["secret"].(string)
	if !strings.HasPrefix(got, "ee") || !strings.HasSuffix(got, suffix) {
		t.Fatalf("healed secret malformed: %q", got)
	}

	if _, changed2 := HealMtprotoSecret(out); changed2 {
		t.Fatal("expected no change for an already-valid secret")
	}

	mid := got[2:34]
	newDomain := "telegram.org"
	in3 := `{"fakeTlsDomain":"telegram.org","secret":"` + got + `"}`
	out3, changed3 := HealMtprotoSecret(in3)
	if !changed3 {
		t.Fatal("expected heal to rewrite the domain suffix")
	}
	if err := json.Unmarshal([]byte(out3), &parsed); err != nil {
		t.Fatalf("healed settings not valid json: %v", err)
	}
	got3, _ := parsed["secret"].(string)
	if got3[2:34] != mid {
		t.Fatalf("random middle should be preserved on domain change: %q vs %q", got3[2:34], mid)
	}
	if !strings.HasSuffix(got3, hex.EncodeToString([]byte(newDomain))) {
		t.Fatalf("suffix not updated for new domain: %q", got3)
	}

	if _, changed4 := HealMtprotoSecret(`{"secret":"ee"}`); changed4 {
		t.Fatal("expected no change when fakeTlsDomain is missing")
	}
}
