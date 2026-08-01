package job

import (
	"slices"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestSplitMovedClientTraffics(t *testing.T) {
	rows := []*xray.ClientTraffic{
		{Email: "idle@x", Up: 0, Down: 0},
		{Email: "up@x", Up: 1024, Down: 0},
		{Email: "down@x", Up: 0, Down: 2048},
		nil,
		{Email: "both@x", Up: 512, Down: 512},
		{Email: "alsoidle@x", Up: 0, Down: 0},
	}

	moved, emails, active := splitMovedClientTraffics(rows)

	t.Run("only the rows that moved bytes are broadcast", func(t *testing.T) {
		got := make([]string, 0, len(moved))
		for _, ct := range moved {
			got = append(got, ct.Email)
		}
		want := []string{"up@x", "down@x", "both@x"}
		if !slices.Equal(got, want) {
			t.Fatalf("moved = %v, want %v", got, want)
		}
	})

	t.Run("the active list and set agree with the broadcast rows", func(t *testing.T) {
		want := []string{"up@x", "down@x", "both@x"}
		if !slices.Equal(emails, want) {
			t.Fatalf("activeEmails = %v, want %v", emails, want)
		}
		if len(active) != len(want) {
			t.Fatalf("deltaActive has %d entries, want %d", len(active), len(want))
		}
		for _, e := range want {
			if !active[e] {
				t.Fatalf("deltaActive missing %q", e)
			}
		}
		if active["idle@x"] {
			t.Fatal("an idle client must not count as active")
		}
	})

	t.Run("an all-idle poll broadcasts nothing", func(t *testing.T) {
		moved, emails, active := splitMovedClientTraffics([]*xray.ClientTraffic{
			{Email: "a@x"}, {Email: "b@x"},
		})
		if len(moved) != 0 || len(emails) != 0 || len(active) != 0 {
			t.Fatalf("expected an empty split, got %d/%d/%d", len(moved), len(emails), len(active))
		}
	})
}
