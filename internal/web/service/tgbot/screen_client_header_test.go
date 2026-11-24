package tgbot

import (
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

// Every link page opens with what is left of the config: a dated expiry, the
// bytes remaining, and unlimited where there is no limit rather than zero.
func TestClientHeaderSaysWhatIsLeft(t *testing.T) {
	localizeWithRealBundle(t)
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedClientForTgUser(t, 4242, "left@x")
	const gb = int64(1 << 30)
	expiry := time.Date(2031, 3, 14, 12, 0, 0, 0, time.Local).UnixMilli()

	tests := []struct {
		name        string
		up, total   int64
		expiry      int64
		want, avoid []string
	}{
		{"limited and dated", 30 * gb, 100 * gb, expiry, []string{"2031-03-14", "70.00GB"}, nil},
		{"overspent never goes negative", 150 * gb, 100 * gb, expiry, []string{"📊 0.00B"}, []string{"📊 -"}},
		{"no limits", 5 * gb, 0, 0, []string{tb.I18nBot("tgbot.unlimited")}, []string{"0.00B", "1970"}},
		{"counts from first use", 0, 0, -30 * 86400000, []string{"30 " + tb.I18nBot("tgbot.days")}, []string{"1969"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := database.GetDB().Exec("UPDATE client_traffics SET up = ?, down = 0, total = ?, expiry_time = ? WHERE email = ?",
				tc.up, tc.total, tc.expiry, "left@x").Error; err != nil {
				t.Fatal(err)
			}
			header := tb.clientHeader("left@x")
			for _, want := range tc.want {
				if !strings.Contains(header, want) {
					t.Errorf("header %q lacks %q", header, want)
				}
			}
			for _, avoid := range tc.avoid {
				if strings.Contains(header, avoid) {
					t.Errorf("header %q shows %q", header, avoid)
				}
			}
		})
	}
}

// The pages a customer copies links from say what is left: the individual
// links through the header, the subscription through its owner's usage block.
func TestLinkPagesCarryTheHeader(t *testing.T) {
	localizeWithRealBundle(t)
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedLinkedClient(t, 4242, "pages@x", "Alpha")
	if err := database.GetDB().Exec("UPDATE client_traffics SET total = ? WHERE email = ?", int64(10<<30), "pages@x").Error; err != nil {
		t.Fatal(err)
	}

	for _, data := range []string{"client_sub_links pages@x", "client_individual_links pages@x"} {
		tapClientLinks(t, tb, 4242, data)
		sc, ok := tb.screens().get(4242)
		if !ok || !strings.Contains(sc.body, "10.00GB") {
			t.Errorf("%q drew no header with the quota left: %+v", data, sc)
		}
	}
}

// I18nBot escapes its parameters, so the header hands it the raw email: an
// email with "&" must read as one &amp;, not the &amp;amp; of a second pass.
func TestClientHeaderEscapesTheEmailOnce(t *testing.T) {
	localizeWithRealBundle(t)
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedClientForTgUser(t, 4242, "a&b@x")

	header := tb.clientHeader("a&b@x")
	if !strings.Contains(header, "a&amp;b@x") || strings.Contains(header, "&amp;amp;") {
		t.Errorf("header %q does not escape the email exactly once", header)
	}
}
