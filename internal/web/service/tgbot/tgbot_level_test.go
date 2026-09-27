package tgbot

import (
	"testing"

	"github.com/mymmrac/telego"
)

func withAdmins(t *testing.T, ids ...int64) {
	t.Helper()
	tgBotMutex.Lock()
	orig := adminIds
	adminIds = ids
	tgBotMutex.Unlock()
	t.Cleanup(func() {
		tgBotMutex.Lock()
		adminIds = orig
		tgBotMutex.Unlock()
	})
}

// newLevelTgbot binds ownerMail to ownerTgID in both the inbound settings and
// the clients table, and makes account 1 the only admin.
func newLevelTgbot(t *testing.T) (*Tgbot, func(string) int) {
	t.Helper()
	tb, calls := newLinksCallbackTgbot(t, ownerMail)
	seedClientRecord(t, ownerMail, "sub-owned", ownerTgID)
	withAdmins(t, 1)
	return tb, calls
}

func commandFrom(tgUserID int64, text string) *telego.Message {
	return &telego.Message{
		From: &telego.User{ID: tgUserID},
		Chat: telego.Chat{ID: tgUserID, Type: telego.ChatTypePrivate},
		Text: text,
	}
}

func TestLevelOfFollowsTheClientBinding(t *testing.T) {
	tb, _ := newLevelTgbot(t)

	for id, want := range map[int64]userLevel{1: levelAdmin, ownerTgID: levelClient, 777: levelStranger, 0: levelStranger} {
		if got := tb.levelOf(id); got != want {
			t.Errorf("levelOf(%d) = %d, want %d", id, got, want)
		}
	}
}

// A stranger's refused command must get no reply at all, while a bound client
// is still told the command is unknown, as before the gate existed.
func TestGateCommand(t *testing.T) {
	cases := []struct {
		name      string
		from      int64
		text      string
		wantOK    bool
		wantAdmin bool
		wantSends int
	}{
		{"stranger start", 777, "/start", true, false, 0},
		{"stranger help", 777, "/help", false, false, 0},
		{"stranger admin command", 777, "/restart", false, false, 0},
		{"client usage", ownerTgID, "/usage", true, false, 0},
		{"client admin command", ownerTgID, "/clearall", false, false, 1},
		{"admin", 1, "/clearall", true, true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tb, calls := newLevelTgbot(t)

			isAdmin, ok := tb.gateCommand(commandFrom(c.from, c.text))
			if ok != c.wantOK || isAdmin != c.wantAdmin {
				t.Errorf("gateCommand = (%v, %v), want (%v, %v)", isAdmin, ok, c.wantAdmin, c.wantOK)
			}
			if n := calls("sendMessage"); n != c.wantSends {
				t.Errorf("sendMessage calls = %d, want %d", n, c.wantSends)
			}
		})
	}
}

// Regression test: a stranger's forged callback must be answered, so the button
// stops spinning, and must never reach answerCallback.
func TestGateCallbackStopsStrangers(t *testing.T) {
	tb, calls := newLevelTgbot(t)

	query := &telego.CallbackQuery{ID: "q1", From: telego.User{ID: 777}, Data: "client_sub_links " + ownerMail}
	if _, ok := tb.gateCallback(query); ok {
		t.Fatal("gateCallback admitted a stranger")
	}
	if n := calls("answerCallbackQuery"); n != 1 {
		t.Errorf("answerCallbackQuery calls = %d, want 1", n)
	}

	query.From.ID = ownerTgID
	if isAdmin, ok := tb.gateCallback(query); !ok || isAdmin {
		t.Errorf("gateCallback(client) = (%v, %v), want (false, true)", isAdmin, ok)
	}
}
