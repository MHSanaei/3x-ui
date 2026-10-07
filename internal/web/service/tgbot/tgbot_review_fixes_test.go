package tgbot

import (
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

// The scheduled report must not edit a screen the admin is working on. The
// depletion report is posted as its own dismissible message.
func TestScheduledDepletionReportDoesNotEditTheLiveScreen(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	// A wizard the admin has open: the tracked screen of this chat.
	tb.renderScreen(555, tb.newScreen("wizard", "draft in progress", tb.backRow()))

	tb.sendExhaustedToAdmins()

	edited := false
	for _, c := range calls() {
		if c.Method == "editMessageText" || c.Method == "editMessageMedia" {
			edited = true
		}
	}
	if edited {
		t.Fatal("the scheduled report edited the admin's live screen")
	}
	tracked, ok := tb.screens().get(555)
	if !ok || tracked.kind != "wizard" {
		t.Fatalf("the wizard screen was replaced (tracked=%v ok=%v)", tracked, ok)
	}
}
