package cronspec

import (
	"strings"
	"testing"
	"time"
)

// #1648 upstream: the panel's scheduler runs WithSeconds(), whose parser refused
// a standard five-field crontab, so an admin's "30 8 * * *" never fired.
func TestParseNextFire(t *testing.T) {
	from := time.Date(2026, 10, 11, 7, 0, 0, 0, time.UTC)
	cases := []struct {
		spec string
		want time.Time
	}{
		{"30 8 * * *", time.Date(2026, 10, 11, 8, 30, 0, 0, time.UTC)},
		{" 15 30 8 * * * ", time.Date(2026, 10, 11, 8, 30, 15, 0, time.UTC)},
		{"@daily", time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)},
		{"@every 90m", from.Add(90 * time.Minute)},
	}
	for _, c := range cases {
		t.Run(c.spec, func(t *testing.T) {
			schedule, err := Parse(c.spec)
			if err != nil {
				t.Fatalf("Parse(%q): %v", c.spec, err)
			}
			if got := schedule.Next(from); !got.Equal(c.want) {
				t.Errorf("Parse(%q).Next = %v, want %v", c.spec, got, c.want)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	for _, spec := range []string{"", "   ", "every day", "61 * * * *", "* * * *"} {
		t.Run(spec, func(t *testing.T) {
			_, err := Parse(spec)
			if err == nil {
				t.Fatalf("Parse(%q): want error, got nil", spec)
			}
			if !strings.Contains(err.Error(), "cron schedule") {
				t.Errorf("Parse(%q) error = %q, want it to name the cron schedule", spec, err)
			}
		})
	}
}
