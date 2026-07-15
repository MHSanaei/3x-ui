package tgbot

import "testing"

// A transient "delete after N seconds" message must not reset the conversation
// state when its timer fires: the user may have advanced to the next wizard step
// (setting a fresh state) within that window, and clearing it would silently
// drop their next input.
func TestDeleteMessageAfterDelayKeepsUserState(t *testing.T) {
	userStateMgr.reset()
	t.Cleanup(userStateMgr.reset)

	const chatID = int64(4242)
	userStateMgr.set(chatID, "awaiting_comment")

	tg := &Tgbot{}
	tg.deleteMessageAfterDelay(chatID, 1, 0)

	if st, ok := userStateMgr.get(chatID); !ok || st != "awaiting_comment" {
		t.Fatalf("delayed message deletion cleared the conversation state: got (%q, %v), want (%q, true)", st, ok, "awaiting_comment")
	}
}
