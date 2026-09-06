package tgbot

import (
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// The dump goes out with ParseMode HTML, so a log line that happens to contain
// angle brackets must not be able to close or open a tag.
func TestLogTailEscapesMarkup(t *testing.T) {
	got := logTail([]string{"dial tcp <nil>: refused & retried"}, 200)
	want := "dial tcp &lt;nil&gt;: refused &amp; retried"
	if got != want {
		t.Fatalf("logTail = %q, want %q", got, want)
	}
}

// Telegram rejects an oversized message outright, so the trim has to drop the
// oldest lines and keep the newest ones an admin is actually looking for.
func TestLogTailKeepsNewestWithinLimit(t *testing.T) {
	lines := []string{"oldest", "middle", "newest"}

	tests := []struct {
		name  string
		limit int
		want  []string
	}{
		{"everything fits", 200, []string{"oldest", "middle", "newest"}},
		{"only the newest fits", 10, []string{"newest"}},
		{"nothing fits", 3, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := logTail(lines, tc.limit)
			if len(got) > tc.limit {
				t.Fatalf("logTail returned %d chars, over the %d limit", len(got), tc.limit)
			}
			want := strings.Join(tc.want, "\r\n")
			if got != want {
				t.Fatalf("logTail = %q, want %q", got, want)
			}
		})
	}
}

func TestLogTailEmptyInput(t *testing.T) {
	if got := logTail(nil, 100); got != "" {
		t.Fatalf("logTail(nil) = %q, want empty so the caller can report no logs", got)
	}
}

// A truncated access-log line yields an entry with blank fields, and those must
// render as an incomplete line rather than as stray separators.
func TestFormatXrayLogEntries(t *testing.T) {
	stamp := time.Date(2026, 8, 19, 10, 33, 12, 0, time.UTC)

	tests := []struct {
		name  string
		entry service.LogEntry
		want  []string
		skip  []string
	}{
		{
			name:  "proxied with email",
			entry: service.LogEntry{DateTime: stamp, FromAddress: "1.2.3.4:5", ToAddress: "example.com:443", Email: "alice@x", Event: 2},
			want:  []string{"🔀", "1.2.3.4:5 → example.com:443", "(alice@x)", stamp.Local().Format("01-02 15:04:05")},
		},
		{
			name:  "blocked",
			entry: service.LogEntry{DateTime: stamp, FromAddress: "1.2.3.4:5", ToAddress: "ads.example:80", Event: 1},
			want:  []string{"⛔"},
			skip:  []string{"("},
		},
		{
			name:  "direct",
			entry: service.LogEntry{DateTime: stamp, FromAddress: "1.2.3.4:5", ToAddress: "cdn.example:443", Event: 0},
			want:  []string{"🌐"},
		},
		{
			name:  "unparsed timestamp",
			entry: service.LogEntry{FromAddress: "1.2.3.4:5", ToAddress: "example.com:443", Event: 2},
			want:  []string{"🔀 1.2.3.4:5 → example.com:443"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lines := formatXrayLogEntries([]service.LogEntry{tc.entry})
			if len(lines) != 1 {
				t.Fatalf("formatXrayLogEntries returned %d lines, want 1", len(lines))
			}
			for _, part := range tc.want {
				if !strings.Contains(lines[0], part) {
					t.Fatalf("line %q is missing %q", lines[0], part)
				}
			}
			for _, part := range tc.skip {
				if strings.Contains(lines[0], part) {
					t.Fatalf("line %q should not contain %q", lines[0], part)
				}
			}
		})
	}
}

func TestFormatXrayLogEntriesEmpty(t *testing.T) {
	if lines := formatXrayLogEntries(nil); len(lines) != 0 {
		t.Fatalf("formatXrayLogEntries(nil) = %v, want no lines", lines)
	}
}
