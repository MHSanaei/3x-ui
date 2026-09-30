package tgbot

import "testing"

// A guide has to open without a bound client: setup instructions are exactly
// what someone needs before they have connected anything.
func TestGuideMenuSendsThePicker(t *testing.T) {
	tb, calls := newLinksCallbackTgbot(t, "someone-else@x")
	customerLocalizer(t)
	runningBot(t)

	tb.guideMenu(7001)

	if n := calls("sendMessage"); n != 1 {
		t.Fatalf("sendMessage calls = %d, want 1: the guide must open without a bound client", n)
	}
}

// Every platform button must reach the router's guide branch, or the picker
// shows a platform whose tap does nothing.
func TestEveryGuideButtonPassesTheCustomerGate(t *testing.T) {
	for _, platform := range guidePlatforms {
		data := guideCallbackPrefix + platform.tag
		if !isClientSelfCallback(data) {
			t.Errorf("isClientSelfCallback(%q) = false, want true", data)
		}
		if _, ok := parseGuideCallback(data); !ok {
			t.Errorf("%q does not resolve to a message key", data)
		}
	}
}

// A customer tapping a guide must be answered, or the button spins until
// Telegram times the callback out.
func TestGuideCallbackAnswersTheTap(t *testing.T) {
	tb, calls := newLinksCallbackTgbot(t, "someone-else@x")
	customerLocalizer(t)
	runningBot(t)

	tb.answerCallback(selfCallback("guide_ios"), false)

	if n := calls("answerCallbackQuery"); n != 1 {
		t.Errorf("answerCallbackQuery calls = %d, want 1: a guide tap must be answered", n)
	}
	if n := calls("sendMessage"); n != 1 {
		t.Errorf("sendMessage calls = %d, want 1: the guide text must be sent", n)
	}
}

// An unknown guide tag is attacker-supplied data; it must be refused rather
// than sent to the localizer, which would resolve it to an empty message.
func TestUnknownGuideTagIsRefused(t *testing.T) {
	tb, calls := newLinksCallbackTgbot(t, "someone-else@x")
	customerLocalizer(t)
	runningBot(t)

	tb.answerCallback(selfCallback("guide_solaris"), false)

	if n := calls("sendMessage"); n != 0 {
		t.Errorf("sendMessage calls = %d, want 0: an unknown platform sent a message", n)
	}
}
