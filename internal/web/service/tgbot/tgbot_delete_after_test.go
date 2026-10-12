package tgbot

import "testing"

func TestDeleteMessageAfterDelayKeepsUserState(t *testing.T) {
	userStateMgr.reset()
	t.Cleanup(userStateMgr.reset)

	const chatID = int64(4242)
	userStateMgr.set(chatUser{chatID: chatID, userID: 1}, "awaiting_comment")

	tg := &Tgbot{}
	tg.deleteMessageAfterDelay(chatID, 1, 0)

	if st, ok := userStateMgr.get(chatUser{chatID: chatID, userID: 1}); !ok || st != "awaiting_comment" {
		t.Fatalf("delayed message deletion cleared the conversation state: got (%q, %v), want (%q, true)", st, ok, "awaiting_comment")
	}
}
