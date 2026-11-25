package tgbot

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mymmrac/telego"
)

const testXrayConfig = `{"log":{}}`

// backupFixture gives GetDb and the config read real files in a throwaway folder.
func backupFixture(t *testing.T) (*Tgbot, func() []apiCall) {
	t.Helper()
	_, calls := newScreenTgbot(t, false)
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	binFolder := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", binFolder)
	if err := os.WriteFile(filepath.Join(binFolder, "config.json"), []byte(testXrayConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	tb := initReportDB(t)
	setTestAdmins(t, 1)
	tb.resolveScreenArt()
	return tb, calls
}

func documentsOf(calls []apiCall) []apiCall {
	var documents []apiCall
	for _, call := range calls {
		if call.Method == "sendDocument" {
			documents = append(documents, call)
		}
	}
	return documents
}

// A Hide button on a backup deleted the admin's only copy of the database with
// one tap, so neither delivery path may offer a way to delete the files.
func TestBackupFilesCarryNoHideButton(t *testing.T) {
	for _, tc := range []struct {
		name    string
		deliver func(tb *Tgbot)
	}{
		{"backup button", func(tb *Tgbot) {
			tb.answerCallback(&telego.CallbackQuery{
				ID: "q1", From: telego.User{ID: 1}, Data: cbBackup,
				Message: &telego.Message{MessageID: 50, Chat: telego.Chat{ID: 1, Type: "private"}},
			}, true)
		}},
		{"scheduled backup", func(tb *Tgbot) { tb.SendBackupToAdmins() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tb, calls := backupFixture(t)
			tc.deliver(tb)

			documents := documentsOf(calls())
			if len(documents) != 2 {
				t.Fatalf("documents = %d, want the database and config.json (calls: %v)", len(documents), methods(calls()))
			}
			dbName, _ := documents[0].Payload["filename"].(string)
			if !strings.HasSuffix(dbName, ".db") || documents[1].Payload["filename"] != "config.json" {
				t.Errorf("files = %q, %v; want a .db then config.json", dbName, documents[1].Payload["filename"])
			}
			for _, document := range documents {
				if markup, ok := document.Payload["reply_markup"]; ok {
					t.Errorf("%v carries a keyboard that can delete it: %v", document.Payload["filename"], markup)
				}
			}
		})
	}
}

// The files used to land under the menu and bury it, so the bot looked dead to
// an admin who stayed at the bottom of the chat.
func TestBackupButtonMovesTheMenuBelowTheFiles(t *testing.T) {
	tb, calls := backupFixture(t)
	const chatID int64 = 1
	tb.screenCategory(chatID, "maintenance")
	menu, ok := tb.screens().get(chatID)
	if !ok {
		t.Fatal("menu not tracked")
	}
	before := len(calls())

	tb.answerCallback(&telego.CallbackQuery{
		ID: "q1", From: telego.User{ID: 1}, Data: cbBackup,
		Message: &telego.Message{MessageID: menu.msgID, Chat: telego.Chat{ID: chatID, Type: "private"}},
	}, true)

	got := calls()[before:]
	want := []string{"answerCallbackQuery", "sendDocument", "sendDocument", "sendPhoto", "deleteMessage"}
	if !reflect.DeepEqual(methods(got), want) {
		t.Fatalf("calls = %v, want %v", methods(got), want)
	}
	if deleted := got[4].Payload["message_id"]; deleted != float64(menu.msgID) {
		t.Errorf("deleted message %v, want the old menu %d", deleted, menu.msgID)
	}
	current, ok := tb.screens().get(chatID)
	if !ok || current.msgID == menu.msgID || !hasButton(current.rows, cbBackup) {
		t.Errorf("tracked screen = %+v, want a new maintenance menu", current)
	}
}

// A scheduled backup arrives unprompted: it must not move a menu the admin may
// be using at that moment.
func TestScheduledBackupLeavesTheMenuInPlace(t *testing.T) {
	tb, calls := backupFixture(t)
	tb.screenCategory(1, "maintenance")
	menu, _ := tb.screens().get(1)
	before := len(calls())

	tb.SendBackupToAdmins()

	if got := methods(calls()[before:]); !reflect.DeepEqual(got, []string{"sendDocument", "sendDocument"}) {
		t.Errorf("calls = %v, want only the two files", got)
	}
	if current, _ := tb.screens().get(1); current.msgID != menu.msgID {
		t.Errorf("tracked menu moved from %d to %d", menu.msgID, current.msgID)
	}
}

func hasButton(rows [][]telego.InlineKeyboardButton, data string) bool {
	for _, row := range rows {
		for _, button := range row {
			if button.CallbackData == data {
				return true
			}
		}
	}
	return false
}
