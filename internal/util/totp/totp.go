package totp

import (
	"time"

	"github.com/xlzd/gotp"
)

// SkewWindows is how many 30s steps around now VerifyWithSkew accepts.
// Standard TOTP clock-drift tolerance, see MHSanaei/3x-ui#6535.
const SkewWindows = 1

// VerifyWithSkew accepts the code for the current step plus/minus SkewWindows.
func VerifyWithSkew(secret, code string, now time.Time) bool {
	totp := gotp.NewDefaultTOTP(secret)
	for i := -SkewWindows; i <= SkewWindows; i++ {
		if totp.AtTime(now.Add(time.Duration(i*30)*time.Second)) == code {
			return true
		}
	}
	return false
}
