package entity

import (
	"fmt"
	"slices"
	"strings"
)

func (s *AllSetting) checkUserTrafficAlerts() error {
	events := strings.Split(s.SmtpEnabledEvents, ",")
	for i := range events {
		events[i] = strings.TrimSpace(events[i])
	}
	rate := slices.Contains(events, "user.rate.high")
	window := slices.Contains(events, "user.traffic.high")

	// Preserve omitted settings from older clients; enabled rules reject zero.
	if !rate {
		if s.SmtpUserRateMB == 0 {
			s.SmtpUserRateMB = 7
		}
		if s.SmtpUserRateSeconds == 0 {
			s.SmtpUserRateSeconds = 60
		}
	}
	if !window {
		if s.SmtpUserWindowGB == 0 {
			s.SmtpUserWindowGB = 5
		}
		if s.SmtpUserWindowMinutes == 0 {
			s.SmtpUserWindowMinutes = 30
		}
	}
	if !rate && !window && s.SmtpUserCooldownMinutes == 0 {
		s.SmtpUserCooldownMinutes = 60
	}
	checks := []struct {
		name       string
		value, max int
		required   bool
	}{
		{"smtpUserRateMB", s.SmtpUserRateMB, 100000, rate},
		{"smtpUserRateSeconds", s.SmtpUserRateSeconds, 86400, rate},
		{"smtpUserWindowGB", s.SmtpUserWindowGB, 1000000, window},
		{"smtpUserWindowMinutes", s.SmtpUserWindowMinutes, 1440, window},
		{"smtpUserCooldownMinutes", s.SmtpUserCooldownMinutes, 10080, rate || window},
	}
	for _, c := range checks {
		if c.value < 0 || c.value > c.max || (c.required && c.value == 0) {
			return fmt.Errorf("%s must be between 1 and %d when the traffic alert is enabled", c.name, c.max)
		}
	}
	return nil
}
