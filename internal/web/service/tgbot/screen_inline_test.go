package tgbot

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mymmrac/telego"
)

// inlineCapturingServer records answerInlineQuery results so a test can assert
// on the list Telegram would show, and on the marker an item carries.
func inlineCapturingServer(t *testing.T, supportsInline bool) (*httptest.Server, func() []map[string]any) {
	t.Helper()
	var mu sync.Mutex
	var queries []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		w.Header().Set("Content-Type", "application/json")
		switch method {
		case "answerInlineQuery":
			mu.Lock()
			queries = append(queries, payload)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": true})
		case "getMe":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"id": 42, "is_bot": true, "first_name": "bot", "username": "bot",
				"supports_inline_queries": supportsInline,
			}})
		case "getChat":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"id": 42, "type": "private", "photo": map[string]any{"big_file_id": "big"},
			}})
		case "sendMessage", "sendPhoto", "sendDocument":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"message_id": 5, "date": 0, "chat": map[string]any{"id": 1, "type": "private"},
			}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": true})
		}
	}))
	return srv, func() []map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return append([]map[string]any(nil), queries...)
	}
}

// maybeAnswerInline waits briefly for the asynchronous query answer.
func maybeAnswerInline(get func() []map[string]any) []map[string]any {
	for range 20 {
		if got := get(); len(got) > 0 {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
	return get()
}

func TestInlineQueryListsInboundsWithMarkers(t *testing.T) {
	srv, queries := inlineCapturingServer(t, true)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	resetScreenArt()

	tb := initReportDB(t)
	setTestAdmins(t, 1)
	seedReportClients(t, "nl-fast", []string{"a@x"})

	tb.handleInlineQuery(&telego.InlineQuery{
		ID:    "iq1",
		From:  telego.User{ID: 1},
		Query: "",
	})

	got := maybeAnswerInline(queries)
	if len(got) != 1 {
		t.Fatalf("answerInlineQuery calls = %d, want 1", len(got))
	}
	results, _ := got[0]["results"].([]any)
	if len(results) == 0 {
		t.Fatal("inline results are empty, want the seeded inbound")
	}
	item, _ := results[0].(map[string]any)
	id, _ := item["id"].(string)
	if !strings.HasPrefix(id, "inb:") {
		t.Errorf("result id = %q, want an inb: marker", id)
	}
	content, _ := item["input_message_content"].(map[string]any)
	if text, _ := content["message_text"].(string); text != id {
		t.Errorf("marker text = %q, want the marker itself (%q), never the real content", text, id)
	}
}

// /start is the only advertised command; it must draw the menu screen. It also
// reaches the route the colon predicate guards, so this pins that a command is
// never mistaken for an inline marker.
func TestStartCommandDrawsTheMenu(t *testing.T) {
	srv, calls := recordingServer(t, false)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	resetScreenArt()
	setTestAdmins(t, 1)

	tb := initReportDB(t)

	tb.answerCommand(&telego.Message{
		MessageID: 1,
		Chat:      telego.Chat{ID: 500, Type: "private"},
		From:      &telego.User{ID: 1},
		Text:      "/start",
	}, 500, true)

	got := calls()
	if len(got) == 0 {
		t.Fatal("an admin's /start drew nothing")
	}
	if got[0].Method != "sendPhoto" && got[0].Method != "sendMessage" {
		t.Errorf("first call = %q, want the menu message", got[0].Method)
	}
}

// A stranger's /start reports its own id, which is what an admin needs to bind
// it; it must never render the panel menu.
func TestStartCommandForAStrangerRevealsOnlyItsID(t *testing.T) {
	srv, calls := recordingServer(t, false)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	resetScreenArt()
	setTestAdmins(t, 1)

	tb := initReportDB(t)

	tb.answerCommand(&telego.Message{
		MessageID: 1,
		Chat:      telego.Chat{ID: 777, Type: "private"},
		From:      &telego.User{ID: 777777},
		Text:      "/start",
	}, 777, false)

	got := calls()
	if len(got) == 0 {
		t.Fatal("a stranger's /start got no answer at all")
	}
	// The locale key is what the test environment resolves to; the panel's own
	// bundle renders it as the stranger's id, which an admin needs to bind it.
	if !strings.Contains(got[0].text(), "askToAddUserId") {
		t.Errorf("answer = %q, want the bind-your-id notice", got[0].text())
	}
}

// A client list must be scoped to the inbound the screen it was opened from
// names, and the scope has to survive the trip through the inline query: the
// query carries the user's id, and in a group that is not the chat's id.
func TestInlineQueryListsClientsOfTheChosenInbound(t *testing.T) {
	srv, queries := inlineCapturingServer(t, true)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	resetScreenArt()
	inlineCapability.recordForTest(true)

	tb := initReportDB(t)
	setTestAdmins(t, 1)
	seedReportClients(t, "nl-fast", []string{"a@x"})
	seedReportClients(t, "de-slow", []string{"b@x"})

	inbounds, err := tb.inboundService.GetAllInbounds()
	if err != nil || len(inbounds) != 2 {
		t.Fatalf("seeded inbounds = %d (err %v), want 2", len(inbounds), err)
	}
	chosen := inbounds[0]

	// The screen is drawn in a group: chat id 500, the admin's own id 1.
	tb.screenInboundClients(500, 1, chosen.Id)

	tb.handleInlineQuery(&telego.InlineQuery{ID: "iq4", From: telego.User{ID: 1}, Query: ""})

	got := maybeAnswerInline(queries)
	if len(got) != 1 {
		t.Fatalf("answerInlineQuery calls = %d, want 1", len(got))
	}
	results, _ := got[0]["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results = %d, want exactly the chosen inbound's single client", len(results))
	}
	item, _ := results[0].(map[string]any)
	if id, _ := item["id"].(string); !strings.HasPrefix(id, "cl:") {
		t.Errorf("result id = %q, want a cl: marker, never an inbound", id)
	}
	if title, _ := item["title"].(string); title != "a@x" {
		t.Errorf("result title = %q, want the client of the chosen inbound", title)
	}
}

// The client screen is browsed in inline mode only: a callback-button grid of
// every client is what the screen layer exists to remove.
func TestClientScreenCarriesNoClientButtons(t *testing.T) {
	srv, _ := inlineCapturingServer(t, true)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	resetScreenArt()
	inlineCapability.recordForTest(true)

	tb := initReportDB(t)
	setTestAdmins(t, 1)
	seedReportClients(t, "nl-fast", []string{"a@x", "b@x"})

	inbounds, err := tb.inboundService.GetAllInbounds()
	if err != nil || len(inbounds) == 0 {
		t.Fatalf("seeded inbounds = %d (err %v), want at least 1", len(inbounds), err)
	}
	tb.screenInboundClients(500, 1, inbounds[0].Id)

	sc, ok := tb.screens().get(500)
	if !ok || sc.markup == nil {
		t.Fatal("the client screen was not rendered")
	}
	var buttons []string
	for _, row := range sc.markup.InlineKeyboard {
		for _, btn := range row {
			buttons = append(buttons, btn.CallbackData)
		}
	}
	for _, data := range buttons {
		if strings.Contains(data, "client_get_usage") || strings.Contains(data, "@x") {
			t.Errorf("the client screen carries the client button %q; clients are inline-only", data)
		}
	}
	if len(buttons) == 0 {
		t.Error("the client screen lost its buttons entirely")
	}
}

// A query with no list screen behind it must fall back to inbounds: listing
// every client would hand the panel's client names to anyone who can type.
func TestInlineQueryWithoutScopeListsInboundsNotClients(t *testing.T) {
	srv, queries := inlineCapturingServer(t, true)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	resetScreenArt()
	inlineCapability.recordForTest(true)

	tb := initReportDB(t)
	setTestAdmins(t, 1)
	seedReportClients(t, "nl-fast", []string{"a@x"})
	inlineScopes.reset()

	tb.handleInlineQuery(&telego.InlineQuery{ID: "iq5", From: telego.User{ID: 1}, Query: ""})

	got := maybeAnswerInline(queries)
	results, _ := got[0]["results"].([]any)
	for _, raw := range results {
		item, _ := raw.(map[string]any)
		if id, _ := item["id"].(string); strings.HasPrefix(id, "cl:") {
			t.Fatalf("an unscoped query leaked the client %q", id)
		}
	}
}

func TestInlineQueryAnswersEmptyResult(t *testing.T) {
	srv, queries := inlineCapturingServer(t, true)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	resetScreenArt()

	tb := initReportDB(t)
	setTestAdmins(t, 1)

	tb.handleInlineQuery(&telego.InlineQuery{ID: "iq2", From: telego.User{ID: 1}, Query: "nothing-matches-this"})

	got := maybeAnswerInline(queries)
	if len(got) != 1 {
		t.Fatalf("answerInlineQuery calls = %d, want 1: an empty list must still answer", len(got))
	}
	if results, _ := got[0]["results"].([]any); len(results) == 0 {
		t.Error("results are empty, want the none-found card")
	}
}

func TestInlineQueryRefusesAStranger(t *testing.T) {
	srv, queries := inlineCapturingServer(t, true)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	resetScreenArt()

	tb := initReportDB(t)
	setTestAdmins(t, 1)
	seedReportClients(t, "secret", []string{"a@x"})

	// 777777 owns no client and is not an admin.
	tb.handleInlineQuery(&telego.InlineQuery{ID: "iq3", From: telego.User{ID: 777777}})

	got := maybeAnswerInline(queries)
	if len(got) != 1 {
		t.Fatalf("answerInlineQuery calls = %d, want 1", len(got))
	}
	results, _ := got[0]["results"].([]any)
	for _, raw := range results {
		item, _ := raw.(map[string]any)
		if id, _ := item["id"].(string); strings.HasPrefix(id, "inb:") || strings.HasPrefix(id, "cl:") {
			t.Fatalf("a stranger received the results item %q", id)
		}
	}
}

func TestInlineLauncherOnlyWhenInlineIsOn(t *testing.T) {
	srv, _ := inlineCapturingServer(t, true)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	resetScreenArt()
	inlineCapability.recordForTest(true)

	tb := &Tgbot{}
	row := tb.launchRow("inb", "tgbot.buttons.searchInbounds")
	if len(row) != 1 || row[0].SwitchInlineQueryCurrentChat == nil {
		t.Fatalf("launcher row = %+v, want a switch-inline button", row)
	}

	inlineCapability.recordForTest(false)
	row = tb.launchRow("inb", "tgbot.buttons.searchInbounds")
	if len(row) != 1 || row[0].SwitchInlineQueryCurrentChat != nil {
		t.Fatalf("row = %+v, want the inline-off hint instead of a dead button", row)
	}
	if row[0].CallbackData != "inline_help" {
		t.Errorf("hint callback = %q, want inline_help", row[0].CallbackData)
	}
}

// setTestAdmins registers admins for the duration of a test.
func setTestAdmins(t *testing.T, ids ...int64) {
	t.Helper()
	orig := adminSnapshot()
	tgBotMutex.Lock()
	adminIds = ids
	tgBotMutex.Unlock()
	t.Cleanup(func() {
		tgBotMutex.Lock()
		adminIds = orig
		tgBotMutex.Unlock()
	})
}

// recordForTest pins the capability cache without a round trip.
func (s *inlineSupport) recordForTest(supported bool) {
	s.mu.Lock()
	s.ok = supported
	s.known = true
	s.at = time.Now()
	s.mu.Unlock()
}

func TestListMarkerParsesAndRejectsForeignPrefixes(t *testing.T) {
	for _, tt := range []struct {
		text   string
		action string
		value  string
		ok     bool
	}{
		{"inb:3", "inb", "3", true},
		{"add:12", "add", "12", true},
		{"cl:user@x", "cl", "user@x", true},
		{"hello world", "", "", false},
		{"inb:", "", "", false},
		{"http://x", "", "", false},
	} {
		action, value, ok := splitListMarker(tt.text)
		if ok != tt.ok || action != tt.action || value != tt.value {
			t.Errorf("splitListMarker(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.text, action, value, ok, tt.action, tt.value, tt.ok)
		}
	}
}
