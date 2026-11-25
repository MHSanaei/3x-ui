package totp

import (
	"testing"
	"time"

	"github.com/xlzd/gotp"
)

func TestVerifyWithSkew(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	totp := gotp.NewDefaultTOTP(secret)
	// Anchor mid-window so a step boundary can't fall between sampling and verify.
	now := time.Unix((time.Now().Unix()/30)*30+15, 0).UTC()

	if !VerifyWithSkew(secret, totp.AtTime(now), now) {
		t.Fatal("current window code should verify")
	}
	if !VerifyWithSkew(secret, totp.AtTime(now.Add(-30*time.Second)), now) {
		t.Fatal("previous window code should verify (clock skew)")
	}
	if !VerifyWithSkew(secret, totp.AtTime(now.Add(30*time.Second)), now) {
		t.Fatal("next window code should verify (clock skew)")
	}
	if VerifyWithSkew(secret, totp.AtTime(now.Add(-60*time.Second)), now) {
		t.Fatal("code two windows old should not verify")
	}
	if VerifyWithSkew(secret, totp.AtTime(now.Add(60*time.Second)), now) {
		t.Fatal("code two windows ahead should not verify")
	}
	if VerifyWithSkew(secret, "000000", now) {
		t.Fatal("wrong code should not verify")
	}
}

func TestVerifyWithSkewSecretForms(t *testing.T) {
	const canonical = "JBSWY3DPEHPK3PXP"
	now := time.Unix((time.Now().Unix()/30)*30+15, 0).UTC()
	code := gotp.NewDefaultTOTP(canonical).AtTime(now)

	cases := []struct {
		name   string
		secret string
		want   bool
	}{
		{"canonical", canonical, true},
		{"lower case", "jbswy3dpehpk3pxp", true},
		{"grouped and padded", " jbsw y3dp ehpk 3pxp== ", true},
		{"empty", "", false},
		{"blank", "   ", false},
		{"not base32", "totp-secret", false},
		{"base32 digits out of range", "JBSWY3DPEHPK3PX1", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := VerifyWithSkew(c.secret, code, now); got != c.want {
				t.Fatalf("VerifyWithSkew(%q) = %v, want %v", c.secret, got, c.want)
			}
		})
	}
}
