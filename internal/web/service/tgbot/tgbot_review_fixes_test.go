package tgbot

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"

	"github.com/mymmrac/telego"
)

// A login success must not fold into the failure card of the same IP: the
// sequence is the alert. The live-card key therefore carries the outcome.
func TestLoginSuccessAndFailureGetSeparateCards(t *testing.T) {
	tb := &Tgbot{}
	fail := eventbus.Event{
		Type:   eventbus.EventLoginAttempt,
		Source: "10.0.0.1",
		Data:   &eventbus.LoginEventData{IP: "10.0.0.1", Username: "root", Status: "fail"},
	}
	ok := eventbus.Event{
		Type:   eventbus.EventLoginAttempt,
		Source: "10.0.0.1",
		Data:   &eventbus.LoginEventData{IP: "10.0.0.1", Username: "root", Status: "success"},
	}

	if tb.eventNoticeKind(fail) == tb.eventNoticeKind(ok) {
		t.Fatalf("a login success and a failure from the same IP share the key %q, so the "+
			"failure card is overwritten", tb.eventNoticeKind(fail))
	}
	// Anything that is not a login keeps the old behaviour: one card per source.
	out := eventbus.Event{Type: eventbus.EventOutboundDown, Source: "proxy-a"}
	other := eventbus.Event{Type: eventbus.EventOutboundDown, Source: "proxy-b"}
	if tb.eventNoticeKind(out) == tb.eventNoticeKind(other) {
		t.Fatal("two different outbound sources share one card")
	}
}

// The scheduled report posts its own message, so a cron landing on the
// add-client wizard cannot replace the draft the admin is editing.
func TestScheduledDepletionReportDoesNotEditTheLiveScreen(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	// A wizard the admin has open: the tracked screen of this chat.
	tb.renderScreen(555, tb.newScreen("wizard", "draft in progress", tb.backRow()))

	tb.sendReport(555, "🚨 Exhausted Clients count:\r\n🛑 Disabled: 0\r\n🔜 Deplete Soon: 0")

	for _, c := range calls() {
		if c.Method == "editMessageText" || c.Method == "editMessageMedia" {
			t.Fatalf("the scheduled report edited the admin's live screen (%s)", c.Method)
		}
	}
	tracked, ok := tb.screens().get(555)
	if !ok || tracked.kind != "wizard" {
		t.Fatalf("the wizard screen was replaced (tracked=%v ok=%v)", tracked, ok)
	}
}

// Every message a report is split into carries its own dismiss button; the old
// behaviour left all but the last one undismissable.
func TestEveryReportMessageCanBeDismissed(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)

	tb.sendReport(777, reportTestBody())

	sent := 0
	for _, c := range calls() {
		if c.Method != "sendMessage" {
			continue
		}
		sent++
		markup, _ := json.Marshal(c.Payload["reply_markup"])
		if !strings.Contains(string(markup), "hide") {
			t.Fatalf("report message %d offers no dismiss button: %s", sent, markup)
		}
	}
	if sent < 2 {
		t.Fatalf("the report went out as %d message(s); the test needs a split body", sent)
	}
}

// One report per run replaces the previous one instead of stacking a new copy
// every day.
func TestNewReportReplacesThePreviousOne(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	reportPrevSet(999, 0)
	t.Cleanup(func() { reportPrevSet(999, 0) })

	tb.sendReport(999, "first report")
	firstID, ok := reportPrevGet(999)
	if !ok || firstID == 0 {
		t.Fatalf("the first report was not tracked (id=%d ok=%v)", firstID, ok)
	}

	tb.sendReport(999, "second report")

	deleted := false
	for _, c := range calls() {
		if c.Method != "deleteMessage" {
			continue
		}
		if id, ok := c.Payload["message_id"].(float64); ok && int(id) == firstID {
			deleted = true
		}
	}
	if !deleted {
		t.Fatalf("the previous report (message %d) was not removed", firstID)
	}
	if prev, _ := reportPrevGet(999); prev == firstID {
		t.Fatal("the tracked report id still points at the deleted message")
	}
}

// A client's daily alert about its own traffic is routine too: it must carry the
// same dismiss the rest of the bot uses, not arrive as an unclearable message.
func TestClientTrafficAlertIsDismissable(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)

	tb.sendNotice(4242, "🚨 Your traffic is running out")

	for _, c := range calls() {
		if c.Method != "sendMessage" {
			continue
		}
		markup, _ := json.Marshal(c.Payload["reply_markup"])
		if !strings.Contains(string(markup), "hide") {
			t.Fatalf("the client alert carries no dismiss button: %s", markup)
		}
		return
	}
	t.Fatal("no alert message was sent")
}

// reportTestBody is long enough that pageMessage splits it into several messages.
func reportTestBody() string {
	return strings.Repeat("📈 line of the scheduled report\r\n", 260)
}

// The server screen carries the host's addresses and versions, so /status must
// not hand it to a client-level account.
func TestStatusCommandGivesAClientLivenessOnly(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	setTestAdmins(t, 1001)

	tb.answerCommand(&telego.Message{
		Chat: telego.Chat{ID: 2002, Type: "private"},
		From: &telego.User{ID: 2002},
		Text: "/status",
	}, 2002, false)

	if methods := methods(calls()); len(methods) == 0 {
		t.Fatal("a client got no answer to /status")
	}
	for _, c := range calls() {
		text := c.text()
		if strings.Contains(text, "Xray") || strings.Contains(text, "Uptime") {
			t.Fatalf("a client received the server screen: %q", text)
		}
	}
}

// The inline browser listed every inbound to any client-level account, which
// leaked remarks, ports and client counts.
func TestInlineBrowserRefusesInboundsForAClient(t *testing.T) {
	tb := &Tgbot{}
	for _, action := range []string{"", "inb", "add"} {
		if got := tb.inlineResults(action, "", "", false); len(got) != 0 {
			t.Fatalf("client-level inline scope %q returned %d results", action, len(got))
		}
	}
}

// Past ~80 clients the per-client reset report exceeded Telegram's message limit
// whole, so the admin saw no outcome at all.
func TestResetAllReportIsPaged(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)

	// Roughly what 120 clients produce.
	body := strings.Repeat("✅ Traffic reset for client@example.com\r\n\r\n", 120)
	tb.sendPagedNotice(31337, body)

	sent := 0
	for _, c := range calls() {
		if c.Method != "sendMessage" {
			continue
		}
		sent++
		if len(c.text()) > telegramPageLimit {
			t.Fatalf("message %d is %d bytes, over the %d limit", sent, len(c.text()), telegramPageLimit)
		}
	}
	if sent < 2 {
		t.Fatalf("the report went out as %d message(s); it must be split", sent)
	}
}

// A single line longer than a caption used to become a page over the limit, and
// Telegram rejected it whole: the screen never appeared.
func TestOverlongLineIsSplitInsteadOfRejected(t *testing.T) {
	long := "vless://" + strings.Repeat("a", 4000)
	for i, page := range splitMessageLines(long, botCaptionLimit) {
		if len(page) > botCaptionLimit {
			t.Fatalf("page %d is %d bytes, over the %d caption limit", i, len(page), botCaptionLimit)
		}
	}
	if joined := strings.Join(splitMessageLines(long, botCaptionLimit), ""); joined != long {
		t.Fatal("splitting an over-long line lost or changed bytes")
	}
}

// /usage with no argument is how a client asks about its own traffic; the rows
// were built from email[0] and panicked on the empty slice.
func TestUsageWithoutAnArgumentDoesNotPanic(t *testing.T) {
	initReportDB(t)
	tb, _ := newScreenTgbot(t, false)
	seedClientForTgUser(t, 3131, "usage-client@example.com")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("getClientUsage panicked: %v", r)
		}
	}()
	tb.getClientUsage(3131, 3131)

	if sc, ok := tb.screens().get(3131); !ok || sc.msgID == 0 {
		t.Fatalf("the usage path never rendered a screen (ok=%v): the test does not exercise the row-building line", ok)
	}
}

// The usage card drew the admin keyboard for every caller, offering a client reset-traffic, IP-limit and
// enable buttons that act on an account.
func TestUsageGivesAClientNoAdminButtons(t *testing.T) {
	initReportDB(t)
	tb, _ := newScreenTgbot(t, false)
	seedClientForTgUser(t, 4141, "usage-self@example.com")

	tb.getClientUsage(4141, 4141)

	sc, ok := tb.screens().get(4141)
	if !ok {
		t.Fatal("no screen was tracked")
	}
	markup, _ := json.Marshal(sc.markup)
	for _, action := range []string{
		"reset_traffic", "limit_traffic", "ip_limit", "ip_log",
		"toggle_enable", "tg_user", "reset_exp", "client_invite_link",
	} {
		if strings.Contains(string(markup), action) {
			t.Fatalf("a client's usage card offered the admin control %q: %s", action, markup)
		}
	}
}

// A non-admin email argument must not open an admin card either: /usage <email>
// for a client resolves to its own card, never to the named one.
func TestUsageEmailArgumentDoesNotLeakAnotherClient(t *testing.T) {
	initReportDB(t)
	tb, _ := newScreenTgbot(t, false)
	seedClientForTgUser(t, 5151, "mine@example.com")
	seedClientForTgUser(t, 9999, "other@example.com")

	tb.getClientUsage(5151, 5151, "other@example.com")

	sc, ok := tb.screens().get(5151)
	if !ok {
		t.Fatal("no screen was tracked")
	}
	body := sc.body + sc.text
	markup, _ := json.Marshal(sc.markup)
	if strings.Contains(body, "other@example.com") || strings.Contains(string(markup), "other@example.com") {
		t.Fatalf("a client's /usage <email> leaked another client:\n%s\n%s", body, markup)
	}
}
