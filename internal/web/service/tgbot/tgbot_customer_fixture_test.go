package tgbot

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// newCustomerTgbot seeds an inbound plus the clients-table row that binds it to
// a Telegram id, and answers every Telegram method the customer surface calls.
// The binding check reads the clients table while the link senders read traffic,
// so both have to exist before a customer tap can be exercised end to end.
func newCustomerTgbot(t *testing.T, email string) (*Tgbot, func(string) int) {
	t.Helper()
	mock, calls := staleButtonServer(t, map[string]any{
		"answerCallbackQuery": map[string]any{"ok": true, "result": true},
		"sendMessage": map[string]any{"ok": true, "result": map[string]any{
			"message_id": 1, "date": 0, "chat": map[string]any{"id": ownerTgID, "type": "private"},
		}},
		// The QR verb answers with documents, so it has to be stubbed or the
		// send silently fails and a test would read that as "no reply".
		"sendDocument": map[string]any{"ok": true, "result": map[string]any{
			"message_id": 2, "date": 0, "chat": map[string]any{"id": ownerTgID, "type": "private"},
		}},
		"editMessageReplyMarkup": map[string]any{"ok": true, "result": map[string]any{
			"message_id": 1, "date": 0, "chat": map[string]any{"id": ownerTgID, "type": "private"},
		}},
		"deleteMessage": map[string]any{"ok": true, "result": true},
	})
	swapTestBot(t, mock.URL)
	t.Cleanup(mock.Close)

	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))

	inbound := &model.Inbound{
		UserId: 1, Remark: "in", Port: 443, Protocol: model.VLESS, Enable: true,
		Settings: `{"clients":[{"email":"` + email + `","tgId":4242,"subId":"sub-owned"}]}`,
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId: inbound.Id, Email: email, Enable: true,
		Total: 1024 * 1024 * 1024, Up: 1024 * 1024 * 10, Down: 1024 * 1024 * 20,
	}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}
	seedClientRecord(t, email, "sub-owned", ownerTgID)
	if err := database.GetDB().Model(&model.Inbound{}).Where("remark = ?", "in").
		Update("settings", `{"clients":[{"email":"`+email+`","tgId":4242,"subId":"sub-owned"}]}`).Error; err != nil {
		t.Fatalf("seed inbound settings: %v", err)
	}
	return &Tgbot{}, calls
}
