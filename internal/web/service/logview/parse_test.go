package logview

import (
	"testing"
	"time"
)

func localMillis(t *testing.T, layout, value string) int64 {
	t.Helper()
	ts, err := time.ParseInLocation(layout, value, time.Local)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return ts.UnixMilli()
}

func TestParseLine(t *testing.T) {
	const stamp = "2006/01/02 15:04:05"
	at := localMillis(t, stamp, "2026/03/04 05:06:07")
	cases := []struct {
		name string
		raw  string
		want LogLine
	}{
		{
			name: "xray access accepted",
			raw:  "2026/03/04 05:06:07.123456 from 203.0.113.9:5555 accepted tcp:example.com:443 [inbound-443 >> direct] email: alice@example.com",
			want: LogLine{
				Time: at + 123, Level: LevelInfo, IP: "203.0.113.9", User: "alice@example.com", Host: "example.com",
				Protocol: "tcp", Inbound: "inbound-443", Outbound: "direct", Status: "accepted",
				Message: "from 203.0.113.9:5555 accepted tcp:example.com:443 [inbound-443 >> direct] email: alice@example.com",
			},
		},
		{
			name: "xray access from an ipv6 client to an ipv6 target over the older arrow",
			raw:  "2026/03/04 05:06:07 from tcp:[2001:db8::1]:5555 accepted udp:[2606:4700::1111]:53 [in-v6 -> blocked]",
			want: LogLine{
				Time: at, Level: LevelInfo, IP: "2001:db8::1", Host: "2606:4700::1111", Protocol: "udp",
				Inbound: "in-v6", Outbound: "blocked", Status: "accepted",
				Message: "from tcp:[2001:db8::1]:5555 accepted udp:[2606:4700::1111]:53 [in-v6 -> blocked]",
			},
		},
		{
			name: "xray access from a masked address keeps the mask as the client identity",
			raw:  "2026/03/04 05:06:07 from 203.0.*.*:5555 accepted tcp:example.com:443 [in >> out]",
			want: LogLine{
				Time: at, Level: LevelInfo, IP: "203.0.*.*", Host: "example.com", Protocol: "tcp",
				Inbound: "in", Outbound: "out", Status: "accepted",
				Message: "from 203.0.*.*:5555 accepted tcp:example.com:443 [in >> out]",
			},
		},
		{
			name: "from followed by a word is prose, not a connection",
			raw:  "2026/03/04 05:06:07 INFO - request from panel accepted",
			want: LogLine{Time: at, Level: LevelInfo, Message: "request from panel accepted"},
		},
		{
			name: "xray access rejected reads as a warning",
			raw:  "2026/03/04 05:06:07 from 203.0.113.9:5555 rejected  proxy/vless/encoding: invalid request user id",
			want: LogLine{
				Time: at, Level: LevelWarning, IP: "203.0.113.9", Protocol: "vless", Status: "rejected",
				Message: "from 203.0.113.9:5555 rejected  proxy/vless/encoding: invalid request user id",
			},
		},
		{
			name: "xray error log carries level, session id and component",
			raw:  "2026/03/04 05:06:07.000000 [Warning] [1234567] app/proxyman/inbound: connection ends > proxy/vless/encoding: failed from 198.51.100.7:1000",
			want: LogLine{
				Time: at, Level: LevelWarning, IP: "198.51.100.7", Protocol: "vless", Category: "app/proxyman/inbound",
				Message: "app/proxyman/inbound: connection ends > proxy/vless/encoding: failed from 198.51.100.7:1000",
			},
		},
		{
			name: "panel log",
			raw:  "2026/03/04 05:06:07 ERROR - [LIMIT_IP] Client bob@example.com: Kept 2 live IPs",
			want: LogLine{
				Time: at, Level: LevelError, User: "bob@example.com", Category: "LIMIT_IP",
				Message: "[LIMIT_IP] Client bob@example.com: Kept 2 live IPs",
			},
		},
		{
			name: "ip limit log written by the fail2ban job",
			raw:  "2026/03/04 05:06:07 [LIMIT_IP] Email = carol@example.com || Disconnecting OLD IP = 192.0.2.44 || Timestamp = 1767225600",
			want: LogLine{
				Time: at, IP: "192.0.2.44", User: "carol@example.com", Category: "LIMIT_IP",
				Message: "[LIMIT_IP] Email = carol@example.com || Disconnecting OLD IP = 192.0.2.44 || Timestamp = 1767225600",
			},
		},
		{
			name: "combined web access format",
			raw:  `203.0.113.9 - dave [04/Mar/2026:05:06:07 +0000] "GET /sub/abc HTTP/1.1" 502 123 "-" "curl/8"`,
			want: LogLine{
				Time: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC).UnixMilli(), Level: LevelError,
				IP: "203.0.113.9", User: "dave", Protocol: "http", Status: "502", Message: "GET /sub/abc HTTP/1.1",
			},
		},
		{
			name: "json line with an epoch-seconds timestamp",
			raw:  `{"ts":1767225600,"level":"warn","msg":"quota hit","client_ip":"192.0.2.1:99","email":"erin","sni":"a.example","proto":"VLESS","tag":"in-1","status":"blocked"}`,
			want: LogLine{
				Time: 1767225600000, Level: LevelWarning, IP: "192.0.2.1", User: "erin", Host: "a.example",
				Protocol: "vless", Inbound: "in-1", Status: "blocked", Message: "quota hit",
			},
		},
		{
			name: "logfmt line",
			raw:  `time=2026-03-04T05:06:07Z level=error msg=boom remote_addr=[2001:db8::2]:80 host=b.example status=500`,
			want: LogLine{
				Time: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC).UnixMilli(), Level: LevelError,
				IP: "2001:db8::2", Host: "b.example", Status: "500",
				Message: `time=2026-03-04T05:06:07Z level=error msg=boom remote_addr=[2001:db8::2]:80 host=b.example status=500`,
			},
		},
		{
			name: "unstructured line keeps its text and invents nothing",
			raw:  "goroutine 1 [running]:",
			want: LogLine{Message: "goroutine 1 [running]:"},
		},
		{
			name: "broken json falls back to plain text",
			raw:  `{"ts":`,
			want: LogLine{Message: `{"ts":`},
		},
		{
			name: "out-of-range date is not a timestamp",
			raw:  "2026/13/45 05:06:07 INFO - hello",
			want: LogLine{Message: "2026/13/45 05:06:07 INFO - hello"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseLine(tc.raw)
			tc.want.Raw = tc.raw
			if got != tc.want {
				t.Errorf("parseLine(%q)\n got %+v\nwant %+v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestParseTimestampLayouts(t *testing.T) {
	utc := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC).UnixMilli()
	local := localMillis(t, "2006-01-02 15:04:05", "2026-03-04 05:06:07")
	cases := []struct {
		raw  string
		want int64
	}{
		{"2026-03-04T05:06:07Z INFO started", utc},
		{"2026-03-04T05:06:07+00:00 INFO started", utc},
		{"2026-03-04 05:06:07 INFO started", local},
		{"2026-03-04 05:06:07,250 INFO started", local + 250},
		{"[2026-03-04 05:06:07] INFO started", local},
	}
	for _, tc := range cases {
		got := parseLine(tc.raw)
		if got.Time != tc.want || got.Level != LevelInfo || got.Message != "started" {
			t.Errorf("parseLine(%q) = time %d level %q message %q, want time %d level info message started", tc.raw, got.Time, got.Level, got.Message, tc.want)
		}
	}
}

// Syslog stamps carry no year, so a December line read in January must land in the previous year.
func TestSyslogTimestampNeverLandsInTheFuture(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.Local)
	got := syslogYear(time.Date(0, 12, 31, 23, 0, 0, 0, time.Local), now)
	if want := time.Date(2025, 12, 31, 23, 0, 0, 0, time.Local); !got.Equal(want) {
		t.Errorf("syslogYear = %v, want %v", got, want)
	}
	got = syslogYear(time.Date(0, 1, 2, 3, 0, 0, 0, time.Local), now)
	if want := time.Date(2026, 1, 2, 3, 0, 0, 0, time.Local); !got.Equal(want) {
		t.Errorf("syslogYear = %v, want %v", got, want)
	}
}

// Lines are attacker-influenced: a client's requested destination reaches the access log verbatim.
func FuzzParseLine(f *testing.F) {
	for _, seed := range []string{
		"", " ", "{", "[", "[]", "from", "from ", "2026/03/04", "2026/03/04 05:06:07 from",
		"x from y accepted", "accepted", "[a >>", "[a >> b]", "email:", "Email =", "Client", "tcp:", "udp:[",
		`{"ts":1e400}`, `{"level":["x"]}`, `a - b [c] "d" 200 1`, "proxy/", "Jan", "Jan 2", "a=", "=b",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if got := parseLine(raw); got.Raw != raw {
			t.Errorf("parseLine lost the raw text: got %q, want %q", got.Raw, raw)
		}
	})
}
