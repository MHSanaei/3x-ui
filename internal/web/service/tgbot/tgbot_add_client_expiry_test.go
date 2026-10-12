package tgbot

import (
	"strings"
	"testing"

	"github.com/mymmrac/telego"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// Pins the decision the dead accumulate branch hid: a preset tap chooses the term
// instead of adding to it, and 0 is Unlimited. The custom keypad shares this case.
func TestAddClientExpiryPresetReplacesTheTerm(t *testing.T) {
	const chatID = int64(7505)
	draftLocalizer(t,
		&i18n.Message{ID: "tgbot.days", Other: "Days"},
		&i18n.Message{ID: "tgbot.unlimited", Other: "Unlimited"},
	)
	url, textsFor := draftTexts(t)
	swapTestBot(t, url)

	// A fresh draft attaches no inbound, so the card never looks one up by remark.
	draft := addClientDrafts.forActor(chatUser{chatID: chatID, userID: 1})
	origRunning := isRunning
	t.Cleanup(func() {
		addClientDrafts.reset(chatUser{chatID: chatID, userID: 1})
		isRunning = origRunning
	})
	isRunning = true

	tb := &Tgbot{}
	for _, tc := range []struct {
		days string
		want string
	}{
		{"30", "Expire: 30 Days"},  // not 37: a second tap replaces the first
		{"90", "Expire: 90 Days"},  // not 97, which accumulating would show
		{"0", "Expire: Unlimited"}, // the Unlimited button clears the term
	} {
		t.Run(tc.days, func(t *testing.T) {
			// A term left by an earlier preset; every row has to fail on its own
			// under the accumulate semantics this change rejected.
			draft.expiryTime = -7 * 86400000

			tb.answerCallback(&telego.CallbackQuery{
				ID:      "q1",
				From:    telego.User{ID: 1},
				Data:    "add_client_reset_exp_c " + tc.days,
				Message: &telego.Message{MessageID: 7, Chat: telego.Chat{ID: chatID}},
			}, true) // admin

			sent := textsFor(chatID)
			if len(sent) == 0 {
				t.Fatalf("add_client_reset_exp_c %s rendered no card", tc.days)
			}
			if got := sent[len(sent)-1]; !strings.Contains(got, tc.want) {
				t.Errorf("card after add_client_reset_exp_c %s = %q, want it to contain %q", tc.days, got, tc.want)
			}
		})
	}
}
