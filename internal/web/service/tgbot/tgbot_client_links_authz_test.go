package tgbot

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/global"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/mymmrac/telego"
)

const (
	ownerTgID = int64(4242)
	ownerMail = "owner@x"
)

// newLinksCallbackTgbot seeds one inbound whose settings bind email to
// ownerTgID, the traffic row the ownership lookup joins on, and a mocked API.
func newLinksCallbackTgbot(t *testing.T, email string) (*Tgbot, func(string) int) {
	t.Helper()
	mock, calls := staleButtonServer(t, map[string]any{
		"answerCallbackQuery": map[string]any{"ok": true, "result": true},
		"sendMessage": map[string]any{"ok": true, "result": map[string]any{
			"message_id": 1,
			"date":       0,
			"chat":       map[string]any{"id": ownerTgID, "type": "private"},
		}},
	})
	swapTestBot(t, mock.URL)
	t.Cleanup(mock.Close)

	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	inbound := &model.Inbound{
		UserId:   1,
		Remark:   "in",
		Port:     443,
		Protocol: model.VLESS,
		Enable:   true,
		Settings: `{"clients":[{"email":"` + email + `","tgId":4242,"subId":"sub-owned"}]}`,
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId: inbound.Id,
		Email:     email,
		Enable:    true,
	}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	origRunning := isRunning
	t.Cleanup(func() { isRunning = origRunning })
	isRunning = true

	return &Tgbot{}, calls
}

func tapClientLinks(t *testing.T, tb *Tgbot, tgUserID int64, data string) {
	t.Helper()
	tb.answerCallback(&telego.CallbackQuery{
		ID:      "q1",
		From:    telego.User{ID: tgUserID},
		Data:    data,
		Message: &telego.Message{Chat: telego.Chat{ID: tgUserID}},
	}, false)
}

// Regression test: a non-admin tapping a link callback carrying another
// client's email must be refused; without the ownership check it is served.
func TestClientLinkCallbackRefusesForeignClient(t *testing.T) {
	tb, calls := newLinksCallbackTgbot(t, ownerMail)

	tapClientLinks(t, tb, ownerTgID, "client_sub_links someone-else@x")

	if n := calls("sendMessage"); n != 0 {
		t.Errorf("sendMessage calls = %d, want 0: a non-admin received a foreign client's links", n)
	}
	if n := calls("answerCallbackQuery"); n != 1 {
		t.Errorf("answerCallbackQuery calls = %d, want 1: the refused tap must be answered", n)
	}
}

// The same guard must not lock the owner out of their own links.
func TestClientLinkCallbackServesOwnClient(t *testing.T) {
	tb, calls := newLinksCallbackTgbot(t, ownerMail)

	tapClientLinks(t, tb, ownerTgID, "client_sub_links "+ownerMail)

	if n := calls("sendMessage"); n != 1 {
		t.Errorf("sendMessage calls = %d, want 1: the owner must still get its links", n)
	}
	if n := calls("answerCallbackQuery"); n != 0 {
		t.Errorf("answerCallbackQuery calls = %d, want 0: an allowed tap is not refused", n)
	}
}

// Regression test: a payload past 64 chars arrives as its hash, so an email long
// enough to be hashed must still be decoded and served to its owner.
func TestHashedLinkCallbackServesOwnClient(t *testing.T) {
	const longMail = "very-long-owner-address-for-hashed-buttons@example.com"
	tb, calls := newLinksCallbackTgbot(t, longMail)

	origStorage := hashStorage
	hashStorage = global.NewHashStorage(20 * time.Minute)
	t.Cleanup(func() { hashStorage = origStorage })

	data := tb.encodeQuery("client_sub_links " + longMail)
	if data == "client_sub_links "+longMail {
		t.Fatalf("encodeQuery left %q unhashed; the test needs a hashed payload", data)
	}
	tapClientLinks(t, tb, ownerTgID, data)

	if n := calls("sendMessage"); n != 1 {
		t.Errorf("sendMessage calls = %d, want 1: the owner's hashed button must still be served", n)
	}
}
