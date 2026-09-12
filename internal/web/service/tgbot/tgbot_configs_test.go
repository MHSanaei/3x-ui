package tgbot

import "testing"

// My Configs is the only door to a config now, so anything it renders must pass
// the same level gate the old top-level buttons did.
func TestConfigsKeyboardIsCustomerReachable(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)
	data := callbackData(tg.configsKeyboard())

	for _, want := range []string{"client_individual_links", "client_one_link", "client_sub_links", "client_menu"} {
		if !contains(data, want) {
			t.Fatalf("configs keyboard is missing %q: %v", want, data)
		}
	}
	for _, entry := range data {
		if !isClientSelfCallback(entry) {
			t.Fatalf("configs keyboard renders %q, which a customer may not fire", entry)
		}
	}
}

// Fails closed like the gate behind it: an operator who never opted in must not
// see a destructive button that would only refuse them.
func TestConfigsKeyboardHidesResetUntilEnabled(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)

	if data := callbackData(tg.configsKeyboard()); contains(data, "client_reset_self") {
		t.Fatalf("reset is offered on a stock panel: %v", data)
	}
	if err := tg.settingService.SetTgBotAllowSelfReset(true); err != nil {
		t.Fatalf("SetTgBotAllowSelfReset: %v", err)
	}
	if data := callbackData(tg.configsKeyboard()); !contains(data, "client_reset_self") {
		t.Fatalf("reset stayed hidden after an admin enabled it: %v", data)
	}
}

// The whole point of Get one is that the QR under the delivered link needs no
// second question, so the button must carry that link's own index.
func TestOneLinkKeyboardCarriesTheChosenIndex(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)
	data := callbackData(tg.oneLinkKeyboard("amy@x", 2))

	if !contains(data, "qr_one amy@x 2") {
		t.Fatalf("QR button does not name the chosen link: %v", data)
	}
	verb, arg, ok := clientSelfAction("qr_one amy@x 2")
	if !ok {
		t.Fatal("the QR button under a delivered link is not ownership-checked")
	}
	if target := clientSelfTarget(verb, arg); target != "amy@x" {
		t.Fatalf("ownership check would read %q, not the owning email", target)
	}
}

// Help is a hub now: the guide and the command sheet hang off it, and both must
// stay reachable for a customer.
func TestHelpKeyboardCarriesGuideAndCommands(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)
	data := callbackData(tg.helpKeyboard())

	for _, want := range []string{"client_commands", "client_guide", "client_menu"} {
		if !contains(data, want) {
			t.Fatalf("help hub is missing %q: %v", want, data)
		}
	}
	for _, entry := range data {
		if !isClientSelfCallback(entry) {
			t.Fatalf("help hub renders %q, which a customer may not fire", entry)
		}
	}
}
