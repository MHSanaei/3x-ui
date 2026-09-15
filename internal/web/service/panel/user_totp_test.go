package panel

import (
	"testing"
	"time"

	"github.com/xlzd/gotp"
)

func TestVerifyTOTPWithSkew(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	totp := gotp.NewDefaultTOTP(secret)
	now := time.Now()

	if !verifyTOTPWithSkew(secret, totp.AtTime(now)) {
		t.Fatal("current window code should verify")
	}
	if !verifyTOTPWithSkew(secret, totp.AtTime(now.Add(-30*time.Second))) {
		t.Fatal("previous window code should verify (clock skew)")
	}
	if !verifyTOTPWithSkew(secret, totp.AtTime(now.Add(30*time.Second))) {
		t.Fatal("next window code should verify (clock skew)")
	}
	if verifyTOTPWithSkew(secret, totp.AtTime(now.Add(-60*time.Second))) {
		t.Fatal("code two windows old should not verify")
	}
	if verifyTOTPWithSkew(secret, totp.AtTime(now.Add(60*time.Second))) {
		t.Fatal("code two windows ahead should not verify")
	}
	if verifyTOTPWithSkew(secret, "000000") {
		t.Fatal("wrong code should not verify")
	}
}
