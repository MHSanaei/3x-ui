package tgbot

import (
	"reflect"
	"testing"

	"github.com/mymmrac/telego"
)

func TestTypedCommandsSendFreshScreensEvenWhenOldEditsSucceed(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		admin         bool
	}{
		{"admin start", "/start", true},
		{"client start", "/start", false},
		{"admin status", "/status", true},
		{"admin client lookup", "/usage admin@e", true},
		{"admin usage", "/usage", true},
		{"client usage", "/usage", false},
		{"client usage with email", "/usage other@e", false},
		{"admin clear confirmation", "/clearall", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, calls := newScreenTgbot(t, false)
			tb := initReportDB(t)
			setTestAdmins(t, 1)
			seedClientForTgUser(t, 1, "admin@e")
			seedClientForTgUser(t, 2, "client@e")
			tb.resolveScreenArt()
			userID := int64(2)
			if tc.admin {
				userID = 1
			}
			const chatID int64 = 500
			tb.renderScreen(chatID, tb.newScreen("main", "previous menu", tb.homeRows()...))
			attempts := 1
			if tc.command == "/start" {
				attempts = 2
			}
			for attempt := range attempts {
				previous, ok := tb.screens().get(chatID)
				if !ok {
					t.Fatal("previous menu missing")
				}
				before := len(calls())
				message := &telego.Message{
					MessageID: 9000 + attempt,
					Chat:      telego.Chat{ID: chatID, Type: "private"},
					From:      &telego.User{ID: userID}, Text: tc.command,
				}
				tb.answerCommand(message, chatID, tc.admin)
				tb.deleteIncoming(message)
				got := calls()[before:]
				if countMethod(got, "sendPhoto") != 1 || countMethod(got, "editMessageMedia")+countMethod(got, "editMessageCaption")+countMethod(got, "editMessageText") != 0 {
					t.Fatalf("command must send a visible new menu even when editing succeeds: %v", methods(got))
				}
				current, ok := tb.screens().get(chatID)
				if !ok || current.msgID == previous.msgID || current.text == "previous menu" {
					t.Fatalf("new menu not tracked: previous=%+v current=%+v", previous, current)
				}
				deletions := []int{}
				for _, call := range got {
					if call.Method == "deleteMessage" {
						deletions = append(deletions, int(call.Payload["message_id"].(float64)))
					}
				}
				if len(deletions) != 2 || deletions[0] != previous.msgID || deletions[1] != message.MessageID || got[0].Method != "sendPhoto" {
					t.Fatalf("new menu must be sent before retiring the old menu and command: methods=%v deletions=%v", methods(got), deletions)
				}
				before = len(calls())
				tb.answerCallback(&telego.CallbackQuery{
					ID: "return-home", From: *message.From, Data: "home",
					Message: &telego.Message{MessageID: current.msgID, Chat: message.Chat},
				}, tc.admin)
				got = calls()[before:]
				if countMethod(got, "sendPhoto")+countMethod(got, "sendMessage") != 0 || countMethod(got, "editMessageMedia")+countMethod(got, "editMessageCaption") != 1 {
					t.Fatalf("callback must still edit the visible menu: %v", methods(got))
				}
			}
		})
	}
}

func TestReplacedCommandMenuLosesKeyboardWhenDeletionFails(t *testing.T) {
	for _, cleanupFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "keyboard retired", true: "cleanup also unavailable"}[cleanupFails], func(t *testing.T) {
			replies := map[string]any{
				"deleteMessage":          map[string]any{"ok": false, "error_code": 400, "description": "Bad Request: message can't be deleted"},
				"editMessageReplyMarkup": map[string]any{"ok": true, "result": map[string]any{"message_id": 333, "date": 0, "chat": map[string]any{"id": 500, "type": "private"}}},
			}
			if cleanupFails {
				replies["editMessageReplyMarkup"] = map[string]any{"ok": false, "error_code": 400, "description": "Bad Request: message to edit not found"}
			}
			server, calls := recordingServer(t, false, replies)
			swapTestBot(t, server.URL)
			t.Cleanup(server.Close)
			tb := initReportDB(t)
			setTestAdmins(t, 1)
			tb.resolveScreenArt()
			tb.renderScreen(500, tb.newScreen("main", "old menu", tb.homeRows()...))
			previous, _ := tb.screens().get(500)
			before := len(calls())
			tb.answerCommand(&telego.Message{MessageID: 9999, Chat: telego.Chat{ID: 500, Type: "private"}, From: &telego.User{ID: 1}, Text: "/start"}, 500, true)
			got := calls()[before:]
			if !reflect.DeepEqual(methods(got), []string{"sendPhoto", "deleteMessage", "editMessageReplyMarkup"}) {
				t.Fatalf("retire undeletable menu after new send: %v", methods(got))
			}
			markup, _ := got[2].Payload["reply_markup"].(map[string]any)
			if got[2].Payload["chat_id"] != float64(500) || got[2].Payload["message_id"] != float64(previous.msgID) || !reflect.DeepEqual(markup["inline_keyboard"], []any{}) {
				t.Fatalf("only old menu's keyboard must be removed: %+v", got[2].Payload)
			}
			current, ok := tb.screens().get(500)
			if !ok || current.msgID == previous.msgID || current.msgID == 0 {
				t.Fatalf("cleanup failure discarded the new menu: %+v", current)
			}
		})
	}
}

func TestFailedCommandScreenSendKeepsPreviousMenu(t *testing.T) {
	photo := map[string]any{"file_id": "avatar", "file_unique_id": "u", "width": 1, "height": 1}
	server, count := staleButtonServer(t, map[string]any{
		"getMe":                  map[string]any{"ok": true, "result": map[string]any{"id": 42, "is_bot": true, "first_name": "bot", "username": "bot"}},
		"getUserProfilePhotos":   map[string]any{"ok": true, "result": map[string]any{"total_count": 1, "photos": [][]any{{photo}}}},
		"sendPhoto":              map[string]any{"ok": false, "error_code": 503, "description": "Service Unavailable"},
		"editMessageMedia":       map[string]any{"ok": true, "result": map[string]any{"message_id": 333, "date": 0, "chat": map[string]any{"id": 500, "type": "private"}}},
		"editMessageReplyMarkup": map[string]any{"ok": true, "result": map[string]any{"message_id": 333, "date": 0, "chat": map[string]any{"id": 500, "type": "private"}}},
	})
	swapTestBot(t, server.URL)
	t.Cleanup(server.Close)
	tb := initReportDB(t)
	setTestAdmins(t, 1)
	tb.resolveScreenArt()
	previous := &screen{msgID: 333, kind: "main", hasPhoto: true, photo: "avatar", text: "previous menu"}
	tb.screens().put(500, previous)
	tb.answerCommand(&telego.Message{MessageID: 444, Chat: telego.Chat{ID: 500, Type: "private"}, From: &telego.User{ID: 1}, Text: "/start"}, 500, true)
	current, ok := tb.screens().get(500)
	if count("sendPhoto") != 1 || count("editMessageMedia") != 0 || count("deleteMessage") != 0 || count("editMessageReplyMarkup") != 0 || !ok || current != previous {
		t.Fatalf("failed send must preserve the previous menu: sends=%d edits=%d deletes=%d keyboardEdits=%d current=%+v", count("sendPhoto"), count("editMessageMedia"), count("deleteMessage"), count("editMessageReplyMarkup"), current)
	}
}
