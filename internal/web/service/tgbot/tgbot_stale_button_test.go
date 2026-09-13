package tgbot

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"github.com/mymmrac/telego"
)

// staleButtonServer serves canned Telegram API responses so tests can drive
// bot-dependent paths; the returned func reports per-method call counts.
func staleButtonServer(t *testing.T, responses map[string]any) (*httptest.Server, func(string) int) {
	t.Helper()
	var mu sync.Mutex
	counts := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for method, body := range responses {
			if r.URL.Path == "/bot"+testBotToken+"/"+method {
				mu.Lock()
				counts[method]++
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(body)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	return srv, func(method string) int {
		mu.Lock()
		defer mu.Unlock()
		return counts[method]
	}
}

func swapTestBot(t *testing.T, url string) {
	t.Helper()
	origBot := bot
	origPool := messageWorkerPool
	t.Cleanup(func() {
		bot = origBot
		messageWorkerPool = origPool
	})
	var err error
	bot, err = telego.NewBot(testBotToken, telego.WithAPIServer(url))
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}
	messageWorkerPool = make(chan struct{}, 10)
}

func newStaleButtonTgbot(t *testing.T) *Tgbot {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	return &Tgbot{}
}

// Regression test: a stale get_clients_for_* tap on a deleted inbound must
// answer an error, not panic on inbound.Remark; removing the guard fails here.
func TestChooseInboundClientStaleInbound(t *testing.T) {
	mock, calls := staleButtonServer(t, map[string]any{
		"answerCallbackQuery": map[string]any{"ok": true, "result": true},
	})
	swapTestBot(t, mock.URL)
	defer mock.Close()

	tb := newStaleButtonTgbot(t)

	q := &telego.CallbackQuery{
		ID:      "q1",
		From:    telego.User{ID: 999999},
		Message: &telego.Message{Chat: telego.Chat{ID: 1}},
	}
	tb.chooseInboundClient(q, 1, 42, "client_sub_links")
	if n := calls("answerCallbackQuery"); n != 1 {
		t.Errorf("answerCallbackQuery calls = %d, want 1: a stale tap must be answered with an error", n)
	}
}

// The keyboard builder must consume the caller's inbound row; a second DB read
// reintroduces the stale-row window the guard closed.
func TestGetInboundClientsForUsesProvidedInbound(t *testing.T) {
	mock, _ := staleButtonServer(t, map[string]any{
		"answerCallbackQuery": map[string]any{"ok": true, "result": true},
	})
	swapTestBot(t, mock.URL)
	defer mock.Close()

	tb := newStaleButtonTgbot(t)

	inbound := &model.Inbound{Id: 7, Remark: "in-7", Settings: `{"clients":[{"email":"a@b.c"}]}`}
	kb, err := tb.getInboundClientsFor(inbound, "client_sub_links")
	if err != nil {
		t.Fatalf("getInboundClientsFor: %v", err)
	}
	if kb == nil || len(kb.InlineKeyboard) == 0 || len(kb.InlineKeyboard[0]) == 0 {
		t.Fatalf("getInboundClientsFor returned no keyboard")
	}
	if email := kb.InlineKeyboard[0][0].Text; email != "a@b.c" {
		t.Errorf("keyboard button text = %q, want %q", email, "a@b.c")
	}
}

// A panicking handler must be contained by runBotHandler; without the
// recover() the panic escapes and fails this test.
func TestRunBotHandlerRecoversPanic(t *testing.T) {
	origPool := messageWorkerPool
	t.Cleanup(func() { messageWorkerPool = origPool })
	messageWorkerPool = make(chan struct{}, 10)

	ran := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic escaped runBotHandler: %v", r)
			}
		}()
		runBotHandler(func() {
			ran = true
			panic("boom")
		})
	}()
	if !ran {
		t.Errorf("handler body did not run")
	}
}
