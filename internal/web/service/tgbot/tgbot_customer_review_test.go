package tgbot

import "testing"

// The customer keyboard is the only surface every linked account already has,
// so the menu has to be reachable from it or nothing below it can be opened.
func TestCustomerMenuIsReachableFromTheCustomerKeyboard(t *testing.T) {
	tb, _ := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)

	markup := tb.clientKeyboard()

	var found bool
	for _, row := range markup.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == "client_menu_open" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("the customer keyboard has no button that opens the customer menu")
	}
	if !isClientSelfCallback("client_menu_open") {
		t.Error("client_menu_open does not pass the customer gate")
	}
}

// Regression test: collapsing the three client pickers routed their verbs
// through runClientSelfAction, which handled only this branch's own two verbs.
// A customer with exactly one config tapping Subscription URL, Individual links
// or QR codes was therefore answered with silence. On main the same tap showed a
// picker.
func TestSingleConfigCustomerGetsMainVerbsServed(t *testing.T) {
	for _, verb := range []string{
		"client_sub_links",
		"client_individual_links",
		"client_qr_links",
		"client_one_link",
	} {
		t.Run(verb, func(t *testing.T) {
			tb, calls := newCustomerTgbot(t, ownerMail)
			customerLocalizer(t)
			runningBot(t)

			before := calls("sendMessage")
			beforeDoc := calls("sendDocument")
			tb.clientEmailPicker(7001, ownerTgID, verb, false)

			// The QR verb answers with documents rather than text, so it counts
			// the send that carries them instead of only sendMessage.
			if got := calls("sendMessage") - before + calls("sendDocument") - beforeDoc; got == 0 {
				t.Fatalf("%s was answered with silence for a customer holding one config", verb)
			}
		})
	}
}

// A verb no handler claims would leave the button spinning, so it is answered.
func TestUnhandledSelfVerbIsAnsweredNotSwallowed(t *testing.T) {
	tb, calls := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)
	runningBot(t)

	tb.runClientSelfAction(7001, "client_not_a_verb", ownerMail)

	if n := calls("sendMessage"); n == 0 {
		t.Fatal("an unhandled verb produced no reply at all")
	}
}

// Every button on the menu must map to a verb the router serves, and each must
// pass the customer gate. A button that falls through answers "error", so the
// customer taps it and nothing happens.
func TestEveryMenuButtonIsServedAndGated(t *testing.T) {
	tb, _ := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)

	markup := tb.clientMenuKeyboard()
	seen := 0
	for _, row := range markup.InlineKeyboard {
		for _, btn := range row {
			seen++
			if btn.CallbackData == "" {
				t.Error("a menu button carries no callback data")
				continue
			}
			data := btn.CallbackData
			if data == "client_menu" {
				continue
			}
			if !isClientSelfCallback(data) {
				t.Errorf("menu button %q does not pass the customer gate", data)
			}
		}
	}
	if seen == 0 {
		t.Fatal("the menu keyboard has no buttons")
	}
}

// The menu and the docs both promise a Subscription URL, so the button has to
// exist and carry the verb that serves it.
func TestMenuOffersSubscriptionUrl(t *testing.T) {
	tb, _ := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)

	var found bool
	for _, row := range tb.clientMenuKeyboard().InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == "client_sub_links" {
				found = true
			}
		}
	}
	if !found {
		t.Error("the menu offers no Subscription URL, which the PR description and docs both list")
	}
}

// The menu's "All links" button must send the all-links verb. It once carried
// the one-link verb instead, so the label did not describe the action.
func TestMenuAllLinksButtonCarriesTheRightVerb(t *testing.T) {
	tb, _ := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)

	verbs := map[string]string{}
	for _, row := range tb.clientMenuKeyboard().InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData != "" {
				verbs[btn.Text] = btn.CallbackData
			}
		}
	}
	allLinks, ok := verbs[tb.I18nBot("tgbot.buttons.getAllLinks")]
	if !ok {
		t.Fatal("the menu has no All links button")
	}
	if allLinks != "client_individual_links" {
		t.Errorf("All links sends %q, want client_individual_links", allLinks)
	}
}
