package tgbot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	// The menu is drawn as a photo when art is cached and as text otherwise;
	// either way the call has to be there.
	drew := false
	for _, c := range got {
		if c.Method == "sendPhoto" || c.Method == "sendMessage" {
			drew = true
		}
	}
	if !drew {
		t.Errorf("calls = %v, want the menu message", methods(got))
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
	tb.screenInboundClients(500, chosen.Id)

	// The launcher prefills the query with the browser's intent; that text is
	// what Telegram sends back, so the test sends exactly it.
	sc, ok := tb.screens().get(500)
	if !ok {
		t.Fatal("the client screen was not rendered")
	}
	var queryText string
	for _, row := range sc.markup.InlineKeyboard {
		for _, btn := range row {
			if btn.SwitchInlineQueryCurrentChat != nil {
				queryText = *btn.SwitchInlineQueryCurrentChat
			}
		}
	}
	if queryText == "" {
		t.Fatal("the client screen carries no inline launcher")
	}
	tb.handleInlineQuery(&telego.InlineQuery{ID: "iq4", From: telego.User{ID: 1}, Query: queryText})

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
	tb.screenInboundClients(500, inbounds[0].Id)

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

// An unknown capability cache must not answer "inline is off": right after a
// restart that showed the warning for a bot that has inline mode on, until the
// background refresh happened to land.
func TestInlineSupportProbesWhenUnknown(t *testing.T) {
	srv, _ := inlineCapturingServer(t, true)
	swapTestBot(t, srv.URL)
	defer srv.Close()

	// Fresh process: nothing was recorded yet.
	inlineCapability.mu.Lock()
	inlineCapability.ok, inlineCapability.known = false, false
	inlineCapability.mu.Unlock()

	tb := &Tgbot{}
	if !tb.inlineSupported() {
		t.Error("an unknown cache answered inline-off; the probe must run first")
	}
	if !inlineCapability.known {
		t.Error("the probe did not record the capability")
	}
}

// An inline client card is what a user reads before tapping: the email as the
// title, and three lines of facts under it.
func TestInlineClientCardCarriesTrafficAndExpiry(t *testing.T) {
	srv, queries := inlineCapturingServer(t, true)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	resetScreenArt()
	inlineCapability.recordForTest(true)

	tb := initReportDB(t)
	setTestAdmins(t, 1)
	seedReportClients(t, "nl-fast", []string{"a@x"})

	tb.handleInlineQuery(&telego.InlineQuery{ID: "iq7", From: telego.User{ID: 1}, Query: inlineScopeClients})

	got := maybeAnswerInline(queries)
	results, _ := got[0]["results"].([]any)
	if len(results) == 0 {
		t.Fatal("the client browser returned nothing")
	}
	item, _ := results[0].(map[string]any)
	if title, _ := item["title"].(string); title != "a@x" {
		t.Errorf("title = %q, want the client's email", title)
	}
	desc, _ := item["description"].(string)
	if strings.Count(desc, "\n") != 2 {
		t.Errorf("description = %q, want three lines (two newlines)", desc)
	}
	for _, want := range []string{"🚦", "📅"} {
		if !strings.Contains(desc, want) {
			t.Errorf("description = %q, want the %s line", desc, want)
		}
	}
}

// All clients is its own browser: it lists clients straight away and never asks
// for an inbound first.
func TestAllClientsScreenListsClientsWithoutAnInboundStep(t *testing.T) {
	srv, queries := inlineCapturingServer(t, true)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	resetScreenArt()
	inlineCapability.recordForTest(true)

	tb := initReportDB(t)
	setTestAdmins(t, 1)
	seedReportClients(t, "nl-fast", []string{"a@x", "b@x"})

	tb.screenAllClients(500)

	sc, ok := tb.screens().get(500)
	if !ok || sc.markup == nil {
		t.Fatal("All clients drew no screen")
	}
	var queryText string
	for _, row := range sc.markup.InlineKeyboard {
		for _, btn := range row {
			if btn.SwitchInlineQueryCurrentChat != nil {
				queryText = *btn.SwitchInlineQueryCurrentChat
			}
		}
	}
	if queryText != inlineScopeClients {
		t.Fatalf("launcher query = %q, want the all-clients marker %q", queryText, inlineScopeClients)
	}

	tb.handleInlineQuery(&telego.InlineQuery{ID: "iq6", From: telego.User{ID: 1}, Query: queryText})

	got := maybeAnswerInline(queries)
	results, _ := got[0]["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("results = %d, want both seeded clients", len(results))
	}
	for _, raw := range results {
		item, _ := raw.(map[string]any)
		if id, _ := item["id"].(string); !strings.HasPrefix(id, "cl:") {
			t.Errorf("result id = %q, want a cl: marker, never an inbound", id)
		}
	}
}

// The query text is the whole intent, so its parsing is pinned: a server-side
// scope is what kept sending the client step back to the inbound list.
func TestParseInlineQuery(t *testing.T) {
	cases := []struct {
		text, action, data, search string
	}{
		{"", inlineScopeInbounds, "", ""},
		{"nl", inlineScopeInbounds, "", "nl"},
		{inlineScopeClients, inlineScopeClients, "", ""},
		{inlineScopeClients + " 3", inlineScopeClients, "3", ""},
		{inlineScopeClients + " 3 vas", inlineScopeClients, "3", "vas"},
		{inlineScopeClients + " vas", inlineScopeClients, "", "vas"},
		{"add", "add", "", ""},
		{"add nl", "add", "", "nl"},
	}
	for _, c := range cases {
		action, data, search := parseInlineQuery(c.text)
		if action != c.action || data != c.data || search != c.search {
			t.Errorf("parseInlineQuery(%q) = (%q, %q, %q), want (%q, %q, %q)",
				c.text, action, data, search, c.action, c.data, c.search)
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

// A picked item is the only thing the marker route may match. The predicate used
// to be "text contains a colon", which ate broadcast text with a URL or a time.
func TestOnlyAPickedItemMatchesTheMarkerRoute(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"inb:3", true},
		{"add:12", true},
		{"cl:user@x", true},
		{"https://example.com/a", false},
		{"backup at 03:00", false},
		{"Paid: 2026-11", false},
		{"note: see panel", false},
		{"inb:", false},
		{"", false},
	}
	for _, c := range cases {
		update := telego.Update{Message: &telego.Message{Text: c.text, From: &telego.User{ID: 1}}}
		if got := messageIsListMarker(context.Background(), update); got != c.want {
			t.Errorf("messageIsListMarker(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

// Telegram rejects an answer with more than 50 results, so a big panel has to
// page: an un-paged answer is rejected whole and the list looks empty forever.
func TestBigListIsPagedInsteadOfRejected(t *testing.T) {
	all := make([]telego.InlineQueryResult, 0, 120)
	for i := range 120 {
		all = append(all, &telego.InlineQueryResultArticle{
			Type: "article", ID: strconv.Itoa(i), Title: strconv.Itoa(i),
			InputMessageContent: &telego.InputTextMessageContent{MessageText: "cl:x" + strconv.Itoa(i)},
		})
	}

	first, next := pageInlineResults(all, "")
	if len(first) != maxInlineResults {
		t.Fatalf("first page = %d results, want %d", len(first), maxInlineResults)
	}
	if next != "50" {
		t.Fatalf("first page cursor = %q, want %q", next, "50")
	}
	second, next2 := pageInlineResults(all, next)
	if len(second) != 50 {
		t.Fatalf("second page = %d results, want 50", len(second))
	}
	if next2 != "100" {
		t.Fatalf("second page cursor = %q, want %q", next2, "100")
	}
	last, next3 := pageInlineResults(all, next2)
	if len(last) != 20 {
		t.Fatalf("last page = %d results, want 20", len(last))
	}
	if next3 != "" {
		t.Fatalf("last page cursor = %q, want empty", next3)
	}
	if _, beyond := pageInlineResults(all, "500"); beyond != "" {
		t.Fatalf("past-the-end cursor = %q, want empty", beyond)
	}
	if got, _ := pageInlineResults(all, "not-a-number"); len(got) != maxInlineResults {
		t.Fatalf("garbage cursor returned %d results, want a first page", len(got))
	}
}

// A short list must not grow a cursor, or Telegram keeps paging an exhausted list.
func TestShortListHasNoCursor(t *testing.T) {
	list := []telego.InlineQueryResult{&telego.InlineQueryResultArticle{Type: "article", ID: "a"}}
	page, next := pageInlineResults(list, "")
	if len(page) != 1 || next != "" {
		t.Fatalf("page = %d results, cursor = %q; want 1 and empty", len(page), next)
	}
}

// Telegram caps a result id at 64 bytes; one long email would sink the answer.
func TestLongMarkerStaysInsideTheResultIDLimit(t *testing.T) {
	long := "cl:" + strings.Repeat("a", 200) + "@example.com"
	id := markerResultID(long)
	if len(id) > 64 {
		t.Fatalf("result id is %d bytes, want at most 64", len(id))
	}
	if id == markerResultID("cl:"+strings.Repeat("a", 201)+"@example.com") {
		t.Fatal("two different markers hashed to the same id")
	}
	if got := markerResultID("cl:user@x"); got != "cl:user@x" {
		t.Fatalf("short marker = %q, want it unchanged", got)
	}
}
