package tgbot

import "testing"

// The rungs are the whole feature: a customer must be told once as they cross
// each threshold, and never for a config that has no quota to run out of.
func TestQuotaRungOf(t *testing.T) {
	const gb = int64(1073741824)

	tests := []struct {
		name  string
		used  int64
		total int64
		want  quotaRung
	}{
		{name: "unlimited", used: 900 * gb, total: 0, want: quotaNone},
		{name: "negative total", used: gb, total: -1, want: quotaNone},
		{name: "unused", used: 0, total: 100 * gb, want: quotaNone},
		{name: "just under 80", used: 79 * gb, total: 100 * gb, want: quotaNone},
		{name: "exactly 80", used: 80 * gb, total: 100 * gb, want: quotaEighty},
		{name: "between the rungs", used: 90 * gb, total: 100 * gb, want: quotaEighty},
		{name: "just under 95", used: 94 * gb, total: 100 * gb, want: quotaEighty},
		{name: "exactly 95", used: 95 * gb, total: 100 * gb, want: quotaNinetyFive},
		{name: "spent", used: 100 * gb, total: 100 * gb, want: quotaNinetyFive},
		{name: "over quota", used: 150 * gb, total: 100 * gb, want: quotaNinetyFive},
		{name: "negative usage", used: -5, total: 100 * gb, want: quotaNone},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := quotaRungOf(tc.used, tc.total); got != tc.want {
				t.Fatalf("quotaRungOf(%d, %d) = %d, want %d", tc.used, tc.total, got, tc.want)
			}
		})
	}
}

// A customer must hear about each threshold once. Crossing the next one speaks
// again; a traffic reset drops them below and re-arms both.
func TestQuotaNoticeIsDue(t *testing.T) {
	tests := []struct {
		name    string
		rung    quotaRung
		warned  int64
		hasMark bool
		want    bool
	}{
		{name: "first time at 80", rung: quotaEighty, hasMark: false, want: true},
		{name: "still at 80", rung: quotaEighty, warned: int64(quotaEighty), hasMark: true, want: false},
		{name: "climbed to 95", rung: quotaNinetyFive, warned: int64(quotaEighty), hasMark: true, want: true},
		{name: "still at 95", rung: quotaNinetyFive, warned: int64(quotaNinetyFive), hasMark: true, want: false},
		{name: "dropped back below", rung: quotaNone, warned: int64(quotaNinetyFive), hasMark: true, want: false},
		{name: "no quota at all", rung: quotaNone, hasMark: false, want: false},
		{name: "stored value is nonsense", rung: quotaEighty, warned: 99, hasMark: true, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := quotaNoticeIsDue(tc.rung, tc.warned, tc.hasMark); got != tc.want {
				t.Fatalf("quotaNoticeIsDue(%d, %d, %v) = %v, want %v", tc.rung, tc.warned, tc.hasMark, got, tc.want)
			}
		})
	}
}

// A client whose traffic was reset must be forgotten, or their next approach to
// 80% is met with silence because the old mark still stands.
func TestQuotaMarkIsClearedOnReset(t *testing.T) {
	initLangDB(t)
	bot := new(Tgbot)
	const email = "amy@example.com"

	if err := quotaWarned.put(bot, email, int64(quotaNinetyFive)); err != nil {
		t.Fatalf("put: %v", err)
	}
	bot.syncQuotaMark(email, quotaNone)
	if _, ok := quotaWarned.get(bot, email); ok {
		t.Fatal("a client back under the first rung kept their mark")
	}

	bot.syncQuotaMark(email, quotaEighty)
	got, ok := quotaWarned.get(bot, email)
	if !ok || got != int64(quotaEighty) {
		t.Fatalf("mark after crossing 80%% = (%d, %v), want (%d, true)", got, ok, quotaEighty)
	}
}
