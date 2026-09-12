package tgbot

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func rosterClient(email string, expiryTime int64) service.ClientWithAttachments {
	return service.ClientWithAttachments{
		ClientRecord: model.ClientRecord{Email: email, ExpiryTime: expiryTime},
	}
}

func rosterEmails(clients []service.ClientWithAttachments) []string {
	out := make([]string, 0, len(clients))
	for i := range clients {
		out = append(out, clients[i].Email)
	}
	return out
}

// The whole point of the report is that whoever lapses next is at the top, so
// the sentinel expiries must not be allowed to jump the queue.
func TestSortClientsByExpiry(t *testing.T) {
	tests := []struct {
		name    string
		clients []service.ClientWithAttachments
		want    []string
	}{
		{
			name: "soonest deadline first",
			clients: []service.ClientWithAttachments{
				rosterClient("later@x", 3000),
				rosterClient("sooner@x", 1000),
				rosterClient("middle@x", 2000),
			},
			want: []string{"sooner@x", "middle@x", "later@x"},
		},
		{
			name: "unlimited sorts after every deadline",
			clients: []service.ClientWithAttachments{
				rosterClient("forever@x", 0),
				rosterClient("dated@x", 5000),
			},
			want: []string{"dated@x", "forever@x"},
		},
		{
			name: "an unstarted window is not a deadline",
			clients: []service.ClientWithAttachments{
				rosterClient("unstarted@x", -2592000000),
				rosterClient("dated@x", 5000),
			},
			want: []string{"dated@x", "unstarted@x"},
		},
		{
			name: "equal expiries fall back to email",
			clients: []service.ClientWithAttachments{
				rosterClient("bravo@x", 1000),
				rosterClient("alpha@x", 1000),
			},
			want: []string{"alpha@x", "bravo@x"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sortClientsByExpiry(tc.clients)
			got := rosterEmails(tc.clients)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("order = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestRosterPage(t *testing.T) {
	clients := []service.ClientWithAttachments{
		rosterClient("a@x", 1),
		rosterClient("b@x", 2),
		rosterClient("c@x", 3),
	}

	tests := []struct {
		name        string
		limit       int
		wantEmails  []string
		wantOmitted int
	}{
		{"under the cap", 10, []string{"a@x", "b@x", "c@x"}, 0},
		{"exactly the cap", 3, []string{"a@x", "b@x", "c@x"}, 0},
		{"over the cap keeps the head", 2, []string{"a@x", "b@x"}, 1},
		{"no cap", 0, []string{"a@x", "b@x", "c@x"}, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, omitted := rosterPage(clients, tc.limit)
			got := rosterEmails(page)
			if omitted != tc.wantOmitted {
				t.Fatalf("omitted = %d, want %d", omitted, tc.wantOmitted)
			}
			if len(got) != len(tc.wantEmails) {
				t.Fatalf("page = %v, want %v", got, tc.wantEmails)
			}
			for i := range got {
				if got[i] != tc.wantEmails[i] {
					t.Fatalf("page = %v, want %v", got, tc.wantEmails)
				}
			}
		})
	}
}
