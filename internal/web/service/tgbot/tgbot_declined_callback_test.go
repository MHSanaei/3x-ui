package tgbot

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/web/global"

	"github.com/mymmrac/telego"
)

func tapCallback(t *testing.T, tb *Tgbot, isAdmin bool, data string) {
	t.Helper()
	tb.answerCallback(&telego.CallbackQuery{
		ID:      "q1",
		From:    telego.User{ID: 1},
		Data:    data,
		Message: &telego.Message{Chat: telego.Chat{ID: 1}},
	}, isAdmin)
}

// decliningServer answers both methods a declined callback can reach, and
// leaves isRunning set so a notice would go out if the code decided to send one.
func decliningServer(t *testing.T) func(string) int {
	t.Helper()
	mock, calls := staleButtonServer(t, map[string]any{
		"answerCallbackQuery": map[string]any{"ok": true, "result": true},
		"sendMessage": map[string]any{"ok": true, "result": map[string]any{
			"message_id": 1,
			"date":       0,
			"chat":       map[string]any{"id": 1, "type": "private"},
		}},
	})
	swapTestBot(t, mock.URL)
	t.Cleanup(mock.Close)

	origRunning := isRunning
	t.Cleanup(func() { isRunning = origRunning })
	isRunning = true

	return calls
}

// Regression test: a payload the bot could not route was dropped without an
// answer, so Telegram kept the button spinning until the callback timed out.
func TestUnroutableCallbackIsAnswered(t *testing.T) {
	for _, data := range []string{"no_such_callback", "get_backup_legacy", "add_client_legacy_step 5"} {
		t.Run(data, func(t *testing.T) {
			calls := decliningServer(t)

			tapCallback(t, &Tgbot{}, true, data)

			if n := calls("answerCallbackQuery"); n != 1 {
				t.Errorf("answerCallbackQuery calls = %d, want 1: an unroutable tap must be answered", n)
			}
			if n := calls("sendMessage"); n != 0 {
				t.Errorf("sendMessage calls = %d, want 0: an answer is not a posted message", n)
			}
		})
	}
}

// Regression test: a button whose hash aged out was answered with nothing at all;
// the answer clears it and the chat notice survives a failed send.
func TestExpiredCallbackHashIsAnsweredAndReported(t *testing.T) {
	calls := decliningServer(t)

	origHash := hashStorage
	hashStorage = global.NewHashStorage(time.Minute)
	t.Cleanup(func() { hashStorage = origHash })

	// 32 hex characters — the shape decodeQuery looks up, and never stored.
	tapCallback(t, &Tgbot{}, true, "0123456789abcdef0123456789abcdef")

	if n := calls("answerCallbackQuery"); n != 1 {
		t.Errorf("answerCallbackQuery calls = %d, want 1: an expired button must be answered", n)
	}
	if n := calls("sendMessage"); n != 1 {
		t.Errorf("sendMessage calls = %d, want 1: the notice is the durable half of the reply", n)
	}
}
