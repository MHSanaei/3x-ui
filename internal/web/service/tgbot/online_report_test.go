package tgbot

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestOnlineReportPreservesLocalAndRemoteIdentityWithoutAttribution(t *testing.T) {
	localizeWithRealBundle(t)
	process := xray.NewProcess(&xray.Config{})
	process.RefreshLocalOnline([]string{"local<&>", "shared", "shared"}, nil, 1000, 100)
	process.SetNodeOnlineTree(17, map[string][]string{"remote-node": {"remote", "shared"}})
	tb := &Tgbot{}
	body := tb.onlineReport(process.GetOnlineClients())
	for _, email := range []string{"local&lt;&amp;&gt;", "shared", "remote"} {
		if got := strings.Count(body, "• "+email+"\n"); got != 1 {
			t.Errorf("online identity %q occurs %d times: %s", email, got, body)
		}
	}
	if strings.Contains(body, " · ") || !strings.Contains(body, "3") {
		t.Fatalf("report invents inbound attribution or counts duplicates: %s", body)
	}
	process.SetNodeOnlineTree(17, nil)
	body = tb.onlineReport(process.GetOnlineClients())
	if strings.Contains(body, "• remote\n") || strings.Count(body, "• shared\n") != 1 {
		t.Fatalf("stale remote identity or duplicate local identity: %s", body)
	}
}
