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

func TestIsMtprotoBridgeLog(t *testing.T) {
	t.Parallel()

	routed := map[string]struct{}{"mtproto-in": {}}
	bridge := parseAccessLogFields("2026/07/22 09:41:09.000000 from tcp:127.0.0.1:60242 accepted tcp:149.154.167.50:443 [mtproto-in >> proxy-a]")
	if !isMtprotoBridgeLog(bridge, routed) {
		t.Fatal("loopback MTProto bridge row must be hidden")
	}

	attributed := parseAccessLogFields("2026/07/22 09:41:09.000000 from 203.0.113.7:54321 accepted tcp:149.154.167.50:443 [mtproto-in >> proxy-a] email: alice")
	if isMtprotoBridgeLog(attributed, routed) {
		t.Fatal("attributed MTProto access row must remain visible")
	}
	localAttributed := parseAccessLogFields("2026/07/22 09:41:09.000000 from [::1]:54321 accepted tcp:[2001:67c:4e8:f002::a]:443 [mtproto-in >> proxy-a] email: alice")
	if isMtprotoBridgeLog(localAttributed, routed) {
		t.Fatal("attributed local MTProto access row must remain visible")
	}
	other := parseAccessLogFields("2026/07/22 09:41:09.000000 from tcp:127.0.0.1:60242 accepted tcp:example.com:443 [other-in >> proxy-a]")
	if isMtprotoBridgeLog(other, routed) {
		t.Fatal("loopback rows from unrelated inbounds must remain visible")
	}
}
