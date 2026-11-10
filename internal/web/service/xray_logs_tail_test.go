package service

import (
	"strings"
	"testing"
)

func xrayAccessLogFixture() string {
	return strings.Join([]string{
		"2026/01/01 00:00:01 from 1.1.1.1:1 accepted tcp:a.example:443 [in-1 -> direct] email: a@x",
		"2026/01/01 00:00:02 from 1.1.1.2:1 accepted tcp:b.example:443 [in-1 -> blocked] email: b@x",
		"2026/01/01 00:00:03 from 1.1.1.3:1 accepted tcp:c.example:443 [in-1 -> proxy] email: c@x",
		"2026/01/01 00:00:04 from 1.1.1.4:1 accepted tcp:d.example:443 [api -> api]",
		"2026/01/01 00:00:05 from 1.1.1.5:1 accepted tcp:e.example:443 [in-1 -> direct] email: e@x",
		"",
	}, "\n")
}

func tailFixture(t *testing.T, limit int, showDirect string) []LogEntry {
	t.Helper()
	entries, err := tailXrayLogEntries(
		strings.NewReader(xrayAccessLogFixture()), limit, "", showDirect, "true", "true",
		[]string{"direct"}, []string{"blocked"},
	)
	if err != nil {
		t.Fatalf("tailXrayLogEntries: %v", err)
	}
	return entries
}

func logEntryEmails(entries []LogEntry) string {
	emails := make([]string, len(entries))
	for i, e := range entries {
		emails[i] = e.Email
	}
	return strings.Join(emails, ",")
}

// A negative count used to slice entries[len+1:] and panic the request.
func TestTailXrayLogEntriesNonPositiveLimitReturnsNothing(t *testing.T) {
	for _, limit := range []int{-1, 0} {
		if got := tailFixture(t, limit, "true"); len(got) != 0 {
			t.Fatalf("limit %d returned %d entries, want 0", limit, len(got))
		}
	}
}

func TestTailXrayLogEntriesKeepsNewestInFileOrder(t *testing.T) {
	tests := []struct {
		name       string
		limit      int
		showDirect string
		want       string
	}{
		{"limit below matches keeps the newest", 2, "true", "c@x,e@x"},
		{"limit wraps the ring more than once", 1, "true", "e@x"},
		{"limit above matches keeps all", 10, "true", "a@x,b@x,c@x,e@x"},
		{"filtered events do not consume the limit", 2, "false", "b@x,c@x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := logEntryEmails(tailFixture(t, tt.limit, tt.showDirect)); got != tt.want {
				t.Fatalf("emails = %q, want %q", got, tt.want)
			}
		})
	}
}
