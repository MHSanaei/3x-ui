package sub

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// A single getSubs entry can hold several links (one per host of an inbound)
// joined by newlines. BuildPageData must split them into one entry per link, with
// the email replicated, so the subpage renders one row per host instead of
// collapsing them onto a single mangled line.
func TestBuildPageData_SplitsMultiHostLinks(t *testing.T) {
	s := &SubService{}
	subs := []string{
		"vless://a@h1:443?type=tcp#DE-john@x\nvless://a@h2:443?type=tcp#DE-john@x\nvless://a@h3:443?type=tcp#DE-john@x",
		"vless://b@h:443?type=tcp#FR-alice@x",
	}
	emails := []string{"john@x", "alice@x"}

	page := s.BuildPageData("s1", "", xray.ClientTraffic{}, 0, subs, emails, "", "", "", "/", "", "")

	if len(page.Result) != 4 {
		t.Fatalf("Result len = %d, want 4 (3 host links + 1 single link)", len(page.Result))
	}
	for i, link := range page.Result {
		if strings.Contains(link, "\n") {
			t.Fatalf("Result[%d] still multi-line: %q", i, link)
		}
	}
	wantEmails := []string{"john@x", "john@x", "john@x", "alice@x"}
	if !reflect.DeepEqual(page.Emails, wantEmails) {
		t.Fatalf("Emails = %v, want %v", page.Emails, wantEmails)
	}
}
