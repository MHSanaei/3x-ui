package tgbot

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const testGB = int64(1073741824)

func rosterFixture(now time.Time) []service.ClientWithAttachments {
	day := func(n int) int64 { return now.AddDate(0, 0, n).UnixMilli() }
	client := func(email string, expiry int64, total, used int64, enable bool, tgID int64, comment string) service.ClientWithAttachments {
		return service.ClientWithAttachments{
			ClientRecord: model.ClientRecord{
				Email: email, ExpiryTime: expiry, TotalGB: total,
				Enable: enable, TgID: tgID, Comment: comment,
			},
			Traffic: &xray.ClientTraffic{Up: used, Down: 0},
		}
	}

	return []service.ClientWithAttachments{
		client("soon@x", day(2), 100*testGB, 10*testGB, true, 111, "Pays monthly"),
		client("overdue@x", day(-3), 100*testGB, 10*testGB, true, 222, ""),
		client("spent@x", day(60), 100*testGB, 99*testGB, true, 333, ""),
		client("off@x", day(60), 100*testGB, 1*testGB, false, 444, ""),
		client("unbound@x", day(60), 100*testGB, 1*testGB, true, 0, "Friend of BOB"),
		client("fine@x", 0, 0, 500*testGB, true, 555, ""),
	}
}

func emailsOf(clients []service.ClientWithAttachments) []string {
	out := make([]string, 0, len(clients))
	for i := range clients {
		out = append(out, clients[i].Email)
	}
	return out
}

func assertEmails(t *testing.T, got []service.ClientWithAttachments, want ...string) {
	t.Helper()
	gotEmails := emailsOf(got)
	if len(gotEmails) != len(want) {
		t.Fatalf("got %v, want %v", gotEmails, want)
	}
	for _, email := range want {
		if !contains(gotEmails, email) {
			t.Fatalf("got %v, want %v", gotEmails, want)
		}
	}
}

// A roster capped at 30 rows and sorted by expiry cannot answer "who is about
// to lapse" or "who never bound an account" on a panel with real customers.
func TestFilterClients(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clients := rosterFixture(now)

	t.Run("all", func(t *testing.T) {
		assertEmails(t, filterClients(clients, rosterFilterAll, now),
			"soon@x", "overdue@x", "spent@x", "off@x", "unbound@x", "fine@x")
	})

	t.Run("expiring covers the ladder's window and the overdue", func(t *testing.T) {
		assertEmails(t, filterClients(clients, rosterFilterExpiring, now), "soon@x", "overdue@x")
	})

	t.Run("exhausted is by quota, not by expiry", func(t *testing.T) {
		assertEmails(t, filterClients(clients, rosterFilterExhausted, now), "spent@x")
	})

	t.Run("disabled", func(t *testing.T) {
		assertEmails(t, filterClients(clients, rosterFilterDisabled, now), "off@x")
	})

	t.Run("unbound", func(t *testing.T) {
		assertEmails(t, filterClients(clients, rosterFilterUnbound, now), "unbound@x")
	})

	t.Run("an unknown filter shows everything rather than nothing", func(t *testing.T) {
		if got := filterClients(clients, "../../etc/passwd", now); len(got) != len(clients) {
			t.Fatalf("unknown filter returned %v", emailsOf(got))
		}
	})
}

func TestSearchClients(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	clients := rosterFixture(now)

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "email substring", query: "soon", want: []string{"soon@x"}},
		{name: "case insensitive", query: "SOON", want: []string{"soon@x"}},
		{name: "comment substring", query: "monthly", want: []string{"soon@x"}},
		{name: "comment is case insensitive too", query: "bob", want: []string{"unbound@x"}},
		{name: "telegram id is exact", query: "222", want: []string{"overdue@x"}},
		{name: "telegram id does not match a substring", query: "22", want: nil},
		{name: "surrounding space is ignored", query: "  spent  ", want: []string{"spent@x"}},
		{name: "no match", query: "nobody", want: nil},
		{name: "empty query matches nothing", query: "", want: nil},
		{name: "an unbound client is not matched by id zero", query: "0", want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertEmails(t, searchClients(clients, tc.query), tc.want...)
		})
	}
}
