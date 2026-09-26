package tgbot

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"

	"github.com/mymmrac/telego"
)

// draftTexts serves the methods the add-client wizard touches and records the
// text of every sendMessage and editMessageText per chat.
func draftTexts(t *testing.T) (string, func(int64) []string) {
	t.Helper()
	var mu sync.Mutex
	texts := map[int64][]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		result := any(true)
		if r.URL.Path == "/bot"+testBotToken+"/sendMessage" || r.URL.Path == "/bot"+testBotToken+"/editMessageText" {
			var payload struct {
				ChatID any    `json:"chat_id"`
				Text   string `json:"text"`
			}
			_ = json.Unmarshal(body, &payload)
			chatID := int64(0)
			switch v := payload.ChatID.(type) {
			case float64:
				chatID = int64(v)
			}
			mu.Lock()
			texts[chatID] = append(texts[chatID], payload.Text)
			mu.Unlock()
			result = map[string]any{"message_id": 1, "date": 0, "chat": map[string]any{"id": chatID, "type": "private"}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
	}))
	t.Cleanup(srv.Close)

	return srv.URL, func(chatID int64) []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), texts[chatID]...)
	}
}

// cardEmail reads the email off a rendered draft card, which is the field the
// wizard assigns when the flow starts.
func cardEmail(t *testing.T, card string) string {
	t.Helper()
	const marker = "Email: <code>"
	start := strings.Index(card, marker)
	if start < 0 {
		t.Fatalf("not a draft card: %q", card)
	}
	rest := card[start+len(marker):]
	end := strings.Index(rest, "</code>")
	if end < 0 {
		t.Fatalf("card has an unterminated email: %q", card)
	}
	return rest[:end]
}

func lastDraftCard(t *testing.T, texts []string) string {
	t.Helper()
	for i := len(texts) - 1; i >= 0; i-- {
		if strings.Contains(texts[i], "Email: <code>") {
			return texts[i]
		}
	}
	t.Fatal("no draft card reached the chat")
	return ""
}

// Regression test: one package-level draft per bot meant an admin's new client
// was filled in by another chat's steps.
func TestAddClientDraftIsPerChat(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const (
		chatA = int64(7101)
		chatB = int64(7202)
	)
	url, textsFor := draftTexts(t)
	swapTestBot(t, url)
	origRunning := isRunning
	t.Cleanup(func() { isRunning = origRunning })
	isRunning = true

	callback := func(chatID int64, data string) {
		t.Helper()
		(&Tgbot{}).answerCallback(&telego.CallbackQuery{
			ID:      "q1",
			From:    telego.User{ID: 1},
			Data:    data,
			Message: &telego.Message{MessageID: 7, Chat: telego.Chat{ID: chatID}},
		}, true)
	}

	// Both admins start a client; each card carries the email the wizard just
	// generated for that chat.
	callback(chatA, "add_client_to 1")
	callback(chatB, "add_client_to 2")
	emailA := cardEmail(t, lastDraftCard(t, textsFor(chatA)))
	emailB := cardEmail(t, lastDraftCard(t, textsFor(chatB)))
	if emailA == "" || emailA == emailB {
		t.Fatalf("drafts start with the same email %q, want one per chat", emailA)
	}

	// Chat A renders its card again, with chat B's wizard already past its start.
	callback(chatA, "add_client_default_traffic_exp")

	if got := cardEmail(t, lastDraftCard(t, textsFor(chatA))); got != emailA {
		t.Errorf("chat A's card shows email %q, want its own %q from chat B's draft", got, emailA)
	}
	if got := cardEmail(t, lastDraftCard(t, textsFor(chatB))); got != emailB {
		t.Errorf("chat B's card shows email %q, want %q", got, emailB)
	}
}

// Regression test: the draft's lock and map were reached before the admin gate, so
// a report tap queued behind a wizard and any chat a tap came from got stored.
func TestNonWizardCallbackTakesNoDraftLock(t *testing.T) {
	const (
		heldChat  = int64(7303)
		spareChat = int64(7404)
	)
	decliningServer(t)

	held := addClientDrafts.forActor(chatUser{chatID: heldChat, userID: 1})
	held.Lock()
	defer held.Unlock()

	tap := func(chatID int64, isAdmin bool, data string) {
		(&Tgbot{}).answerCallback(&telego.CallbackQuery{
			ID:      "q1",
			From:    telego.User{ID: 1},
			Data:    data,
			Message: &telego.Message{Chat: telego.Chat{ID: chatID}},
		}, isAdmin)
	}
	returns := func(what string, tap func()) {
		t.Helper()
		done := make(chan struct{})
		go func() {
			defer close(done)
			tap()
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("%s waited on the draft lock it never reads", what)
		}
	}

	returns("an admin report tap", func() { tap(heldChat, true, "no_such_admin_action 5") })
	returns("a non-admin wizard tap", func() { tap(heldChat, false, "add_client_to 1") })
	tap(spareChat, false, "add_client_to 1")

	addClientDrafts.mu.Lock()
	_, stored := addClientDrafts.drafts[chatUser{chatID: spareChat, userID: 1}]
	addClientDrafts.mu.Unlock()
	if stored {
		t.Errorf("draft stored for chat %d, want none until its wizard starts", spareChat)
	}
}
