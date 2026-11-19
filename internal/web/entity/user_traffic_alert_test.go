package entity

import "testing"

func TestUserTrafficAlertValidation(t *testing.T) {
	s := AllSetting{SmtpEnabledEvents: "user.rate.high,user.traffic.high", SmtpUserRateMB: 7, SmtpUserRateSeconds: 60, SmtpUserWindowGB: 5, SmtpUserWindowMinutes: 30, SmtpUserCooldownMinutes: 60}
	if err := s.checkUserTrafficAlerts(); err != nil {
		t.Fatal(err)
	}
	s.SmtpUserRateSeconds = 0
	if err := s.checkUserTrafficAlerts(); err == nil {
		t.Fatal("enabled rule accepted zero duration")
	}
	s.SmtpEnabledEvents = "cpu.high"
	if err := s.checkUserTrafficAlerts(); err != nil {
		t.Fatal("legacy settings without new fields rejected", err)
	}
	s.SmtpUserWindowMinutes = 1441
	if err := s.checkUserTrafficAlerts(); err == nil {
		t.Fatal("unbounded window accepted")
	}
}
