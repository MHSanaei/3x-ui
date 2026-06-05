package controller

import "time"

// Registration shares the loginLimiter implementation but with its own
// instance and tuning. Registration is keyed by client IP only (there is no
// trusted username yet), so abusive sign-up bursts from a single host are
// throttled without affecting other clients.
const (
	registrationLimitMaxAttempts = 5
	registrationLimitWindow      = 10 * time.Minute
	registrationLimitCooldown    = 30 * time.Minute

	// registrationLimitBucket is the constant "username" component of the
	// limiter key so every request from an IP shares one counter.
	registrationLimitBucket = "register"
)

var defaultRegisterLimiter = newLoginLimiter(registrationLimitMaxAttempts, registrationLimitWindow, registrationLimitCooldown)
