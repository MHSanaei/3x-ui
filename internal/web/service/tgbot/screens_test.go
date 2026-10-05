package tgbot

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mymmrac/telego"
)

// recordingServer answers the Bot API calls the screen layer makes and keeps
// them in order, so a test can assert on the sequence, not just the outcome.
type apiCall struct {
	Method  string
	Payload map[string]any
}

func (c apiCall) text() string {
	if v, ok := c.Payload["text"].(string); ok {
		return v
	}
	if v, ok := c.Payload["caption"].(string); ok {
		return v
	}
	return ""
}

func recordingServer(t *testing.T, failEdit bool) (*httptest.Server, func() []apiCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []apiCall
	nextID := 100

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		if strings.HasPrefix(string(body), "{") {
			// JSON payload: keep it as decoded.
		} else {
			payload = map[string]any{}
		}
		mu.Lock()
		calls = append(calls, apiCall{Method: method, Payload: payload})
		nextID++
		id := nextID
		mu.Unlock()

		fail := func(desc string) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 400, "description": desc})
		}
		switch method {
		case "editMessageMedia", "editMessageText", "editMessageCaption":
			if failEdit {
				fail("Bad Request: message to edit not found")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"message_id": id, "date": 0, "chat": map[string]any{"id": 1, "type": "private"},
				"photo": []any{map[string]any{"file_id": "ph", "file_unique_id": "u", "width": 1, "height": 1}},
			}})
		case "sendPhoto", "sendMessage", "sendDocument":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"message_id": id, "date": 0, "chat": map[string]any{"id": 1, "type": "private"},
			}})
		case "getMe":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"id": 42, "is_bot": true, "first_name": "bot", "username": "bot",
			}})
		case "getChat":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"id": 42, "type": "private", "photo": map[string]any{"small_file_id": "s", "big_file_id": "big"},
			}})
		default:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": true})
		}
	}))
	return srv, func() []apiCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]apiCall(nil), calls...)
	}
}

// newScreenTgbot points the package bot at a recording server and clears the
// process-wide screen state the previous test may have left behind.
func newScreenTgbot(t *testing.T, failEdit bool) (*Tgbot, func() []apiCall) {
	t.Helper()
	srv, calls := recordingServer(t, failEdit)
	swapTestBot(t, srv.URL)
	t.Cleanup(srv.Close)
	resetScreenArt()
	return &Tgbot{}, calls
}

func methods(calls []apiCall) []string {
	out := make([]string, 0, len(calls))
	for _, c := range calls {
		out = append(out, c.Method)
	}
	return out
}

func indexOf(calls []apiCall, method string) int {
	for i, c := range calls {
		if c.Method == method {
			return i
		}
	}
	return -1
}

func TestScreenRendersOnceAndEditsAfterwards(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)

	tb.renderScreen(1, tb.newScreen("main", "first", tb.homeRows()...))
	tb.renderScreen(1, tb.newScreen("main", "second", tb.homeRows()...))

	got := calls()
	if n := countMethod(got, "sendPhoto") + countMethod(got, "sendMessage"); n != 1 {
		t.Fatalf("screen messages = %d, want 1: one screen per chat (calls: %v)", n, methods(got))
	}
	if n := countMethod(got, "editMessageMedia"); n != 1 {
		t.Fatalf("editMessageMedia = %d, want 1: the second render must edit (calls: %v)", n, methods(got))
	}
	if n := countMethod(got, "deleteMessage"); n != 0 {
		t.Errorf("deleteMessage = %d, want 0: an edit needs no replacement", n)
	}
}

func TestScreenReplacesInSendThenDeleteOrder(t *testing.T) {
	tb, calls := newScreenTgbot(t, true)

	tb.renderScreen(1, tb.newScreen("main", "live", tb.homeRows()...))
	before := len(calls())
	tb.renderScreen(1, tb.newScreen("main", "refreshed", tb.homeRows()...))

	got := calls()[before:]
	send := indexOf(got, "sendPhoto")
	if send < 0 {
		send = indexOf(got, "sendMessage")
	}
	del := indexOf(got, "deleteMessage")
	if send < 0 || del < 0 {
		t.Fatalf("calls = %v, want a re-send and a delete", methods(got))
	}
	if del < send {
		t.Errorf("calls = %v, want the new screen sent BEFORE the old one is deleted", methods(got))
	}
}

func TestScreenTreatsNotModifiedAsSuccess(t *testing.T) {
	var mu sync.Mutex
	var calls []apiCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		mu.Lock()
		calls = append(calls, apiCall{Method: method})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch method {
		case "editMessageMedia", "editMessageText":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 400,
				"description": "Bad Request: message is not modified"})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"message_id": 7, "date": 0, "chat": map[string]any{"id": 1, "type": "private"},
			}})
		}
	}))
	swapTestBot(t, srv.URL)
	defer srv.Close()
	resetScreenArt()

	tb := &Tgbot{}
	tb.renderScreen(1, tb.newScreen("main", "one", tb.homeRows()...))
	mu.Lock()
	before := len(calls)
	mu.Unlock()
	tb.renderScreen(1, tb.newScreen("main", "one", tb.homeRows()...))

	mu.Lock()
	after := append([]apiCall(nil), calls[before:]...)
	mu.Unlock()
	for _, c := range after {
		if c.Method == "deleteMessage" || c.Method == "sendPhoto" || c.Method == "sendMessage" {
			t.Fatalf("calls after the no-op edit = %v, want no replacement", methods(after))
		}
	}
}

func TestScreenCaptionFitsTelegramLimitAndPages(t *testing.T) {
	tb, _ := newScreenTgbot(t, false)

	short := tb.newScreen("main", "hello", tb.homeRows()...)
	if short.pageMax != 1 {
		t.Fatalf("pageMax = %d, want 1 for a short body", short.pageMax)
	}
	tb.renderScreen(1, short)
	if len(short.text) > botCaptionLimit {
		t.Errorf("caption is %d chars, want at most %d", len(short.text), botCaptionLimit)
	}

	long := tb.newScreen("main", strings.Repeat("line of text\n", 300), tb.homeRows()...)
	if long.pageMax < 2 {
		t.Fatalf("pageMax = %d, want the long body paged", long.pageMax)
	}
	tb.renderScreen(1, long)
	if len([]rune(long.text)) > botCaptionLimit {
		t.Errorf("paged caption is %d runes, want at most %d", len([]rune(long.text)), botCaptionLimit)
	}
	// The pager must be reachable: a body nobody can page through is a dead end.
	found := false
	for _, row := range long.markup.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == cbPageNext {
				found = true
			}
		}
	}
	if !found {
		t.Error("paged screen has no next-page button")
	}
}

func TestHideButtonDeletesItsOwnMessage(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)

	tb.hideMessage(&telego.CallbackQuery{
		ID:      "q1",
		Message: &telego.Message{MessageID: 55, Chat: telego.Chat{ID: 1}},
	})

	got := calls()
	for _, c := range got {
		if c.Method != "deleteMessage" {
			continue
		}
		if id, ok := c.Payload["message_id"].(float64); ok && int(id) == 55 {
			return
		}
	}
	t.Fatalf("calls = %v, want deleteMessage of the tapped message 55", methods(got))
}

func TestHomeScreenCarriesTheMenu(t *testing.T) {
	tb, _ := newScreenTgbot(t, false)
	sc := tb.newScreen("main", "summary", tb.homeRows()...)
	data := map[string]bool{}
	for _, row := range sc.rows {
		for _, btn := range row {
			data[btn.CallbackData] = true
		}
	}
	for _, want := range []string{cbServer, cbInbounds, cbClients, cbOnlines, cbDeplete, cbReport, cbBackup, cbBanLogs, cbAddClient, cbActions, cbHome} {
		if !data[want] {
			t.Errorf("menu is missing the %q button", want)
		}
	}
}

func TestScreenKeepsArtFromTheBotAvatar(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	tb.resolveScreenArt()

	if _, ok := artFileID("main"); !ok {
		t.Fatal("resolveScreenArt did not cache the bot avatar")
	}
	tb.renderScreen(1, tb.newScreen("main", "with art", tb.homeRows()...))

	if countMethod(calls(), "getChat") == 0 {
		t.Error("getChat was not called: the avatar cannot have been read")
	}
	if countMethod(calls(), "sendPhoto") == 0 {
		t.Errorf("calls = %v, want the screen sent as a photo", methods(calls()))
	}
}

// sentText is what a Telegram call would display, plus the chat it went to.
// A screen travels as a photo (multipart, caption inside the media field), so a
// test helper that only reads JSON `text` sees nothing at all.
type sentText struct {
	ChatID int64
	Text   string
}

// extractSentBody reads the displayable text and the target chat off a Bot API
// request, whichever shape the call used.
func extractSentBody(t *testing.T, r *http.Request) sentText {
	t.Helper()
	out := sentText{}
	ctype := r.Header.Get("Content-Type")
	if strings.HasPrefix(ctype, "multipart/form-data") {
		if err := r.ParseMultipartForm(1 << 22); err != nil {
			return out
		}
		out.Text = r.FormValue("caption")
		if out.Text == "" {
			out.Text = r.FormValue("text")
		}
		if media := r.FormValue("media"); media != "" {
			var payload struct {
				Caption string `json:"caption"`
			}
			if json.Unmarshal([]byte(media), &payload) == nil && payload.Caption != "" {
				out.Text = payload.Caption
			}
		}
		out.ChatID = parseInt64(r.FormValue("chat_id"))
		return out
	}
	body, _ := io.ReadAll(r.Body)
	var payload struct {
		ChatID  json.RawMessage `json:"chat_id"`
		Text    string          `json:"text"`
		Caption string          `json:"caption"`
		Media   json.RawMessage `json:"media"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return out
	}
	out.Text = payload.Text
	if out.Text == "" {
		out.Text = payload.Caption
	}
	if len(payload.Media) > 0 {
		var media struct {
			Caption string `json:"caption"`
		}
		if json.Unmarshal(payload.Media, &media) == nil && media.Caption != "" {
			out.Text = media.Caption
		}
	}
	var chat any
	if json.Unmarshal(payload.ChatID, &chat) == nil {
		if f, ok := chat.(float64); ok {
			out.ChatID = int64(f)
		}
	}
	return out
}

func parseInt64(v string) int64 {
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func countMethod(calls []apiCall, method string) int {
	n := 0
	for _, c := range calls {
		if c.Method == method {
			n++
		}
	}
	return n
}
