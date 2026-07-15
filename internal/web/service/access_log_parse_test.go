package service

import "testing"

func TestParseAccessLogFields(t *testing.T) {
	malformed := []string{
		"",
		"singletoken",
		"2024/01/02",
		"2024/01/02 15:04:05.000000 from",
		"2024/01/02 15:04:05.000000 accepted",
		"2024/01/02 15:04:05.000000 email:",
	}
	for _, line := range malformed {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("parseAccessLogFields panicked on %q: %v", line, r)
				}
			}()
			_ = parseAccessLogFields(line)
		}()
	}

	line := "2024/01/02 15:04:05.123456 from 1.2.3.4:555 accepted tcp:example.com:443 [inbound-tag >> outbound-tag] email: alice@example.com"
	entry := parseAccessLogFields(line)
	if entry.FromAddress != "1.2.3.4:555" {
		t.Errorf("FromAddress = %q, want %q", entry.FromAddress, "1.2.3.4:555")
	}
	if entry.ToAddress != "tcp:example.com:443" {
		t.Errorf("ToAddress = %q, want %q", entry.ToAddress, "tcp:example.com:443")
	}
	if entry.Inbound != "inbound-tag" {
		t.Errorf("Inbound = %q, want %q", entry.Inbound, "inbound-tag")
	}
	if entry.Outbound != "outbound-tag" {
		t.Errorf("Outbound = %q, want %q", entry.Outbound, "outbound-tag")
	}
	if entry.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", entry.Email, "alice@example.com")
	}
	if entry.DateTime.IsZero() {
		t.Error("DateTime was not parsed from a well-formed line")
	}
}
