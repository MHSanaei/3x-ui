package tgbot

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func neverMuted(string, int64) bool { return false }

// The manual button promises a count before it chases anyone, so the count must agree
// with the pass; the rungs compare calendar days in the panel's own zone.
func TestPlanReminders(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.Local)
	plan := planReminders(rosterFixture(now), now, true, neverMuted, map[string]int64{})

	// overdue@x is on the ladder, spent@x is at 99%, and the two-day client is
	// summarised for admins without a customer reminder.
	if plan.renew != 1 || plan.quota != 1 || plan.total != 2 {
		t.Fatalf("planReminders = %+v, want renew 1, quota 1, total 2", plan)
	}
}

func TestPlanRemindersCounts(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.Local)
	day := func(n int) int64 { return now.AddDate(0, 0, n).UnixMilli() }
	client := func(email string, expiry int64, total, used, tgID int64) service.ClientWithAttachments {
		return service.ClientWithAttachments{
			ClientRecord: model.ClientRecord{Email: email, ExpiryTime: expiry, TotalGB: total, Enable: true, TgID: tgID},
			Traffic:      &xray.ClientTraffic{Up: used},
		}
	}
	records := []service.ClientWithAttachments{
		client("both@x", day(-1), 100*testGB, 96*testGB, 111),
		client("expiring@x", day(1), 0, 0, 222),
		client("spent@x", day(90), 100*testGB, 81*testGB, 333),
		client("unbound@x", day(-1), 100*testGB, 99*testGB, 0),
		client("fine@x", day(90), 100*testGB, 1*testGB, 444),
	}

	tests := []struct {
		name     string
		quotaOn  bool
		muted    func(string, int64) bool
		marks    map[string]int64
		wantPlan reminderPlan
	}{
		{
			name:     "a client due both notices is one recipient",
			quotaOn:  true,
			muted:    neverMuted,
			marks:    map[string]int64{},
			wantPlan: reminderPlan{renew: 2, quota: 2, total: 3},
		},
		{
			name:     "a muted client is not chased",
			quotaOn:  true,
			muted:    func(email string, _ int64) bool { return email == "expiring@x" },
			marks:    map[string]int64{},
			wantPlan: reminderPlan{renew: 1, quota: 2, total: 2},
		},
		{
			name:     "the traffic warning switch takes the quota half out",
			quotaOn:  false,
			muted:    neverMuted,
			marks:    map[string]int64{},
			wantPlan: reminderPlan{renew: 2, quota: 0, total: 2},
		},
		{
			name:     "a rung already announced is not counted again",
			quotaOn:  true,
			muted:    neverMuted,
			marks:    map[string]int64{"both@x": int64(quotaNinetyFive), "spent@x": int64(quotaEighty)},
			wantPlan: reminderPlan{renew: 2, quota: 0, total: 2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := planReminders(records, now, tc.quotaOn, tc.muted, tc.marks); got != tc.wantPlan {
				t.Fatalf("planReminders = %+v, want %+v", got, tc.wantPlan)
			}
		})
	}
}

// Nothing to send has to read as nothing to send: an empty roster must not
// offer a confirm button that would fire a pass at no one.
func TestPlanRemindersEmpty(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.Local)
	if plan := planReminders(nil, now, true, neverMuted, map[string]int64{}); !plan.empty() {
		t.Fatalf("planReminders on an empty roster = %+v, want empty", plan)
	}
}
