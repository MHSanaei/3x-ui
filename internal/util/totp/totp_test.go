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
