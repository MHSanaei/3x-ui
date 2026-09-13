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
	draftLocalizer(t,
		&i18n.Message{ID: "tgbot.days", Other: "Days"},
		&i18n.Message{ID: "tgbot.unlimited", Other: "Unlimited"},
	)
	url, texts := draftTexts(t)
	swapTestBot(t, url)

	// The card lists attached inbounds by remark, which would reach the database;
	// no attach step runs here, so keep that list empty whichever test ran before.
	origInbounds := receiver_inbound_IDs
	origExpiry := client_ExpiryTime
	origRunning := isRunning
	t.Cleanup(func() {
		receiver_inbound_IDs = origInbounds
		client_ExpiryTime = origExpiry
		isRunning = origRunning
	})
	receiver_inbound_IDs = nil
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
			client_ExpiryTime = -7 * 86400000

			tb.answerCallback(&telego.CallbackQuery{
				ID:      "q1",
				From:    telego.User{ID: 1},
				Data:    "add_client_reset_exp_c " + tc.days,
				Message: &telego.Message{MessageID: 7, Chat: telego.Chat{ID: 1}},
			}, true) // admin

			sent := texts()
			if len(sent) == 0 {
				t.Fatalf("add_client_reset_exp_c %s rendered no card", tc.days)
			}
			if got := sent[len(sent)-1]; !strings.Contains(got, tc.want) {
				t.Errorf("card after add_client_reset_exp_c %s = %q, want it to contain %q", tc.days, got, tc.want)
			}
		})
	}
}
