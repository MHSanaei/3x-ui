package tgbot

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
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

// reportTestBody is long enough that pageMessage splits it into several messages.
func reportTestBody() string {
	return strings.Repeat("📈 line of the scheduled report\r\n", 260)
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
