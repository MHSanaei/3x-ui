package tgbot

import (
	"testing"

	"github.com/mymmrac/telego"
)

// runningBot marks the bot live: SendMsgToTgbot returns early otherwise, so a
// test would assert on a message that was never sent.
func runningBot(t *testing.T) {
	t.Helper()
	orig := isRunning
	t.Cleanup(func() { isRunning = orig })
	isRunning = true
}

// Every menu button must resolve to a verb the router actually serves. A button
// that falls through to the catch-all answers "error", so a customer taps it
// and nothing happens.
func TestClientMenuButtonsAllRoute(t *testing.T) {
	buttons := [][2]string{
		{"client_individual_links", "client_individual_links " + ownerMail},
		{"client_one_link", "client_one_link " + ownerMail},
		{"client_sub_links", "client_sub_links " + ownerMail},
		{"client_qr_links", "client_qr_links " + ownerMail},
		{"guide_menu", "guide_menu"},
	}
	for _, b := range buttons {
		if !isClientSelfCallback(b[1]) {
			t.Errorf("%q (sent as %q) does not pass the customer gate", b[0], b[1])
		}
	}
}

// A menu entry that names no client must still be recognised, or the tap is
// dropped before the router ever sees it.
func TestClientMenuEntryPointsPassTheGate(t *testing.T) {
	for _, data := range []string{"client_menu", "guide_menu", "guide_ios", "guide_linux"} {
		if !isClientSelfCallback(data) {
			t.Errorf("isClientSelfCallback(%q) = false, want true", data)
		}
	}
}

// A customer callback data naming a client must be gated, and one that does not
// must not be, so an admin-only button cannot be reached by a customer.
func TestAdminOnlyCallbacksStayOutOfTheCustomerGate(t *testing.T) {
	for _, data := range []string{
		"client_cancel " + ownerMail,
		"client_edit " + ownerMail,
		"set_bindmax 5",
		"admin_settings",
		"broadcast_confirm abc",
	} {
		if isClientSelfCallback(data) {
			t.Errorf("isClientSelfCallback(%q) = true: an admin-only callback is reachable by a customer", data)
		}
	}
}

// A customer with nothing bound must be told how to bind. Without the check the
// menu would open onto buttons that each refuse them, which reads as a bug.
func TestClientMenuRefusesAnUnboundAccount(t *testing.T) {
	tb, calls := newLinksCallbackTgbot(t, "someone-else@x")
	customerLocalizer(t)
	runningBot(t)

	tb.clientMenu(7001, 9999, false)

	if n := calls("sendMessage"); n != 1 {
		t.Fatalf("sendMessage calls = %d, want 1: an unbound account must be told how to bind", n)
	}
}

// A bound customer gets the menu. The keyboard carries the buttons rather than
// prose, so the tap is what the routing test above pins.
func TestClientMenuServesABoundAccount(t *testing.T) {
	tb, calls := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)
	runningBot(t)

	tb.clientMenu(7001, ownerTgID, false)

	if n := calls("sendMessage"); n != 1 {
		t.Fatalf("sendMessage calls = %d, want 1: a bound customer must get their menu", n)
	}
}

// An admin who opens the customer menu with nothing bound is told where the
// panel's own invite links are, not to ask another admin for a link.
func TestNoBoundClientAdviceIsDifferentForAdmins(t *testing.T) {
	tb, _ := newLinksCallbackTgbot(t, "someone-else@x")
	customerLocalizer(t)

	if tb.noBoundClientMsg(true) == tb.noBoundClientMsg(false) {
		t.Fatal("admin and customer were given the same advice")
	}
}

// The guide menu is reachable without any client bound, since it names no
// client: setup instructions are useful to someone who has not connected yet.
func TestGuideMenuNeedsNoClient(t *testing.T) {
	tb, calls := newLinksCallbackTgbot(t, "someone-else@x")
	customerLocalizer(t)
	runningBot(t)

	tb.guideMenu(7001)

	if n := calls("sendMessage"); n != 1 {
		t.Fatalf("sendMessage calls = %d, want 1: the guide must open without a bound client", n)
	}
}

// selfCallback builds a callback tap from a customer's own Telegram id, which
// is what the router's non-admin path sees.
func selfCallback(data string) *telego.CallbackQuery {
	return &telego.CallbackQuery{
		ID:      "q1",
		From:    telego.User{ID: 9999},
		Data:    data,
		Message: &telego.Message{Chat: telego.Chat{ID: 9999}},
	}
}
