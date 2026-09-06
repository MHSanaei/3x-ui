package tgbot

import (
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const (
	oneDay  = int64(86400000)
	oneGiB  = int64(1073741824)
	tenDays = 10 * oneDay
)

func nowMillis() int64 {
	return time.Now().Unix() * 1000
}

func TestPartitionExpiringClients(t *testing.T) {
	now := nowMillis()
	traffics := []*xray.ClientTraffic{
		{Email: "lapsed@x", Enable: false, ExpiryTime: now - tenDays},
		{Email: "expiring@x", Enable: true, ExpiryTime: now + oneDay},
		{Email: "nearly-full@x", Enable: true, Total: 10 * oneGiB, Up: 9 * oneGiB, Down: oneGiB / 2},
		{Email: "healthy@x", Enable: true, ExpiryTime: now + tenDays, Total: 100 * oneGiB},
		{Email: "unlimited@x", Enable: true},
	}

	exhausted, disabled := partitionExpiringClients(traffics, now, 3*oneDay, oneGiB)

	wantExhausted := []string{"expiring@x", "nearly-full@x"}
	if len(exhausted) != len(wantExhausted) {
		t.Fatalf("exhausted = %v, want %v", emails(exhausted), wantExhausted)
	}
	for i, email := range wantExhausted {
		if exhausted[i].Email != email {
			t.Fatalf("exhausted = %v, want %v", emails(exhausted), wantExhausted)
		}
	}
	if len(disabled) != 1 || disabled[0].Email != "lapsed@x" {
		t.Fatalf("disabled = %v, want [lapsed@x]", emails(disabled))
	}
}

// A customer whose only subscription already lapsed still has to be told, even
// though nothing of theirs is merely approaching a threshold.
func TestBuildExhaustedNoticeReportsLapsedOnlyCustomers(t *testing.T) {
	initInviteDB(t)
	now := nowMillis()
	tg := &Tgbot{}

	tests := []struct {
		name             string
		exhausted        []xray.ClientTraffic
		disabled         []xray.ClientTraffic
		wantNotice       bool
		wantListsLapsed  bool
		wantListsExpirng bool
	}{
		{
			name:       "nothing to report",
			wantNotice: false,
		},
		{
			name:            "only lapsed",
			disabled:        []xray.ClientTraffic{{Email: "lapsed@x", ExpiryTime: now - tenDays}},
			wantNotice:      true,
			wantListsLapsed: true,
		},
		{
			name:             "only expiring",
			exhausted:        []xray.ClientTraffic{{Email: "expiring@x", Enable: true, ExpiryTime: now + oneDay}},
			wantNotice:       true,
			wantListsExpirng: true,
		},
		{
			name:             "both",
			exhausted:        []xray.ClientTraffic{{Email: "expiring@x", Enable: true, ExpiryTime: now + oneDay}},
			disabled:         []xray.ClientTraffic{{Email: "lapsed@x", ExpiryTime: now - tenDays}},
			wantNotice:       true,
			wantListsLapsed:  true,
			wantListsExpirng: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tg.buildExhaustedNotice(tc.exhausted, tc.disabled)
			if !tc.wantNotice {
				if got != "" {
					t.Fatalf("notice = %q, want empty", got)
				}
				return
			}
			if got == "" {
				t.Fatal("notice is empty, want a report")
			}
			if gotLapsed := strings.Contains(got, lapsedSectionMarker); gotLapsed != tc.wantListsLapsed {
				t.Fatalf("lapsed section present = %v, want %v in %q", gotLapsed, tc.wantListsLapsed, got)
			}
			expiringDetails := strings.Count(got, clientDetailMarker) - boolToInt(tc.wantListsLapsed)
			if (expiringDetails > 0) != tc.wantListsExpirng {
				t.Fatalf("expiring entries = %d, want listed = %v in %q", expiringDetails, tc.wantListsExpirng, got)
			}
		})
	}
}

// Without a localizer the bot helpers echo their keys back, so the section
// headers are the stable way to assert which parts of the notice were rendered.
const (
	lapsedSectionMarker = "tgbot.clients:"
	clientDetailMarker  = "tgbot.messages.email"
)

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func emails(traffics []xray.ClientTraffic) []string {
	out := make([]string, 0, len(traffics))
	for _, traffic := range traffics {
		out = append(out, traffic.Email)
	}
	return out
}
