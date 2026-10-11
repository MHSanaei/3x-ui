package web

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

// A five-field crontab used to be refused by the WithSeconds() scheduler, so the
// Telegram/Discord report and LDAP sync were never scheduled at all.
func TestScheduleSettingJob(t *testing.T) {
	from := time.Date(2026, 10, 11, 7, 0, 0, 0, time.UTC)
	cases := []struct {
		name, spec string
		want       time.Time
	}{
		{"five-field crontab", "30 8 * * *", time.Date(2026, 10, 11, 8, 30, 0, 0, time.UTC)},
		{"six-field crontab", "15 30 8 * * *", time.Date(2026, 10, 11, 8, 30, 15, 0, time.UTC)},
		{"empty falls back", "  ", time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)},
		{"invalid falls back", "every morning", time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sched := cron.New(cron.WithSeconds(), cron.WithLocation(time.UTC))
			id := scheduleSettingJob(sched, "test job", c.spec, "@daily", cron.FuncJob(func() {}))
			entry := sched.Entry(id)
			if entry.Schedule == nil {
				t.Fatalf("spec %q: job was not scheduled", c.spec)
			}
			if got := entry.Schedule.Next(from); !got.Equal(c.want) {
				t.Errorf("spec %q: next run %v, want %v", c.spec, got, c.want)
			}
		})
	}
}
