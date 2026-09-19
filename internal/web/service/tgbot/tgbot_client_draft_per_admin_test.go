package tgbot

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"

	"github.com/mymmrac/telego"
)

// Regression test: keying the add-client wizard by chat alone left the two
// admins of a group chat filling in one client between them.
func TestAddClientDraftIsPerAdminInGroupChat(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const (
		groupChat = int64(-1001234567890)
		adminA    = int64(8101)
		adminB    = int64(8202)
	)
	url, textsFor := draftTexts(t)
	swapTestBot(t, url)
	origRunning := isRunning
	t.Cleanup(func() {
		isRunning = origRunning
		userStateMgr.reset()
	})
	isRunning = true

	// Both admins tap in the same chat; only the sender tells them apart.
	tap := func(userID int64, data string) {
		t.Helper()
		(&Tgbot{}).answerCallback(&telego.CallbackQuery{
			ID:      "q1",
			From:    telego.User{ID: userID},
			Data:    data,
			Message: &telego.Message{MessageID: 7, Chat: telego.Chat{ID: groupChat}},
		}, true)
	}

	tap(adminA, "add_client_to 1")
	emailA := cardEmail(t, lastDraftCard(t, textsFor(groupChat)))
	tap(adminB, "add_client_to 2")
	emailB := cardEmail(t, lastDraftCard(t, textsFor(groupChat)))
	if emailA == "" || emailA == emailB {
		t.Fatalf("both admins start with email %q, want one draft per admin", emailA)
	}

	// Admin A renders again, with admin B's wizard already past its start.
	tap(adminA, "add_client_default_traffic_exp")
	if got := cardEmail(t, lastDraftCard(t, textsFor(groupChat))); got != emailA {
		t.Errorf("admin A's card shows email %q, want its own %q from admin B's draft", got, emailA)
	}

	// The step they are each on is their own too: A's prompt must not put B into
	// the same step, or B's next message lands in A's wizard.
	tap(adminA, "add_client_ch_default_email")
	if st, ok := userStateMgr.get(chatUser{chatID: groupChat, userID: adminA}); !ok || st != "awaiting_email" {
		t.Errorf("admin A's step = %q (set: %v), want awaiting_email", st, ok)
	}
	if st, ok := userStateMgr.get(chatUser{chatID: groupChat, userID: adminB}); ok {
		t.Errorf("admin B's step = %q, want none: only the tapper's step may change", st)
	}
}
