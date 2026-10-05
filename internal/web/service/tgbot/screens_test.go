package tgbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
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
		case "copyMessage":
			// Telegram answers with a MessageID object, not a boolean.
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"message_id": id,
			}})
		case "copyMessages":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []any{
				map[string]any{"message_id": id},
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
		case "getUserProfilePhotos":
			// The avatar has to arrive as a Photo file_id: a ChatPhoto id from
			// getChat ist rejected by sendPhoto with "can't use file of type
			// ChatPhoto as Photo".
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"total_count": 1,
				"photos": []any{[]any{
					map[string]any{"file_id": "small-photo", "file_unique_id": "u1", "width": 160, "height": 160},
					map[string]any{"file_id": "big-photo", "file_unique_id": "u2", "width": 640, "height": 640},
				}},
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
	// The menu opens with categories only: each subject is one tap deeper, and
	// the entry screen must not grow back into the wall of buttons.
	for _, want := range []string{cbCatServer, cbCatClients, cbCatTraffic, cbCatMaintenance, cbHome} {
		if !data[want] {
			t.Errorf("menu is missing the %q button", want)
		}
	}
	for _, old := range []string{cbServer, cbInbounds, cbClients, cbOnlines, cbDeplete, cbReport, cbBackup, cbBanLogs, cbAddClient} {
		if data[old] {
			t.Errorf("menu still carries %q directly; it belongs in a category", old)
		}
	}

	// Every category the menu offers must be drawn, and carry its own entries.
	for _, category := range []string{"server", "clients", "traffic", "maintenance"} {
		rows := tb.categoryRows(category)
		if len(rows) < 2 {
			t.Errorf("category %q has %d rows, want its entries plus a way back", category, len(rows))
			continue
		}
		flat := map[string]bool{}
		for _, row := range rows {
			for _, btn := range row {
				flat[btn.CallbackData] = true
			}
		}
		if !flat[cbHome] {
			t.Errorf("category %q has no way back to the menu", category)
		}
	}
}

// hasHomeRow reports whether a screen carries the way back to the menu.
func hasHomeRow(sc *screen) bool {
	for _, row := range sc.rows {
		for _, btn := range row {
			if btn.CallbackData == cbHome {
				return true
			}
		}
	}
	return false
}

// A screen whose only buttons are its own actions is a trap once the action
// runs — that is exactly what the reset confirmation did. Every screen must
// offer a way out, even one that forgot to ask for it.
func TestEveryScreenOffersAWayOut(t *testing.T) {
	tb := &Tgbot{}

	ownActionOnly := tb.newScreen("main", "body",
		tu.InlineKeyboardRow(tb.btn("tgbot.buttons.confirmResetTraffic", "reset_all_traffics_c")))
	if !hasHomeRow(ownActionOnly) {
		t.Error("a screen with only its own action got no way back")
	}

	// A screen that already ends with Back must not grow a second copy.
	withBack := tb.newScreen("main", "body", tb.backRow())
	backs := 0
	for _, row := range withBack.rows {
		for _, btn := range row {
			if btn.CallbackData == cbHome {
				backs++
			}
		}
	}
	if backs != 1 {
		t.Errorf("back rows = %d, want exactly 1", backs)
	}

	// And the screens the menu actually draws.
	for name, sc := range map[string]*screen{
		"reset confirmation": tb.resetAllConfirm(),
		"home":               tb.newScreen("main", "x", tb.homeRows()...),
		"category":           tb.newScreen("main", "x", tb.categoryRows("maintenance")...),
	} {
		if !hasHomeRow(sc) {
			t.Errorf("%s has no way back to the menu", name)
		}
	}
}

// The reset confirmation must not stay on screen after the reset runs: its
// Cancel was the way back and its only other button was the one just pressed.
func TestResetConfirmIsReplacedByTheMenu(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	setTestAdmins(t, 1)
	initReportDB(t)
	seedReportClients(t, "nl-fast", []string{"a@x"})

	tb.answerCallback(&telego.CallbackQuery{
		ID:      "q1",
		From:    telego.User{ID: 1},
		Data:    "reset_all_traffics_c",
		Message: &telego.Message{Chat: telego.Chat{ID: 1}},
	}, true)

	if countMethod(calls(), "sendPhoto")+countMethod(calls(), "sendMessage") == 0 {
		t.Fatal("the reset drew no message at all")
	}
	// The chat's tracked screen has to be the menu now, not the confirmation.
	sc, ok := tb.screens().get(1)
	if !ok {
		t.Fatal("no screen is tracked after the reset")
	}
	if !hasHomeRow(sc) {
		t.Error("the screen left behind after the reset has no way back")
	}
	if sc.kind != "main" {
		t.Errorf("tracked screen kind = %q, want the menu", sc.kind)
	}
}

// A client's /start must never render the admin menu: it used to, which both
// advertised panel operations a client cannot run and listed every client's
// email through the All-clients browser.
func TestClientStartNeverShowsTheAdminMenu(t *testing.T) {
	tb, _ := newScreenTgbot(t, false)
	setTestAdmins(t, 1)
	initReportDB(t)
	seedReportClients(t, "nl-fast", []string{"mine@x"})

	// A client: owns a record, is not an admin.
	screenArtMu.Lock()
	artFromAvatar, artProbedAt = false, time.Now()
	screenArtMu.Unlock()

	tb.screenHome(500, 777777)

	sc, ok := tb.screens().get(500)
	if !ok || sc.markup == nil {
		// No record at all: the client is told its id instead. That is fine.
		return
	}
	for _, row := range sc.rows {
		for _, btn := range row {
			switch btn.CallbackData {
			case cbCatServer, cbCatClients, cbCatTraffic, cbCatMaintenance, cbBroadcast, cbAddClient:
				t.Errorf("a client's menu carries the admin-only %q button", btn.CallbackData)
			}
		}
	}
}

// The broadcast the /broadcast command already drove must be reachable as a
// button, and only for an admin.
func TestBroadcastButtonIsAdminOnly(t *testing.T) {
	tb := &Tgbot{}
	found := false
	for _, category := range []string{"server", "clients", "traffic", "maintenance"} {
		for _, row := range tb.categoryRows(category) {
			for _, btn := range row {
				if btn.CallbackData == cbBroadcast {
					found = true
					if category != "clients" {
						t.Errorf("broadcast sits in %q, want the clients category", category)
					}
				}
			}
		}
	}
	if !found {
		t.Error("no category carries the broadcast button")
	}
}

// A missing avatar is retried sooner than a present one: an operator who
// uploads a picture checks right after, and tiles until the next restart would
// read as the feature not working.
func TestScreenArtReprobeWindow(t *testing.T) {
	srv, calls := recordingServer(t, false)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	resetScreenArt()

	tb := &Tgbot{}

	// A miss two minutes ago is past the retry window: a probe must fire.
	screenArtMu.Lock()
	artFromAvatar = false
	artProbedAt = time.Now().Add(-2 * time.Minute)
	screenArtMu.Unlock()

	tb.ensureScreenArt()
	for range 50 {
		if countMethod(calls(), "getUserProfilePhotos") > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if countMethod(calls(), "getUserProfilePhotos") == 0 {
		t.Error("a stale missing avatar was not re-probed")
	}

	// A known avatar is inside its long window: no probe.
	screenArtMu.Lock()
	artFromAvatar = true
	artProbedAt = time.Now().Add(-2 * time.Minute)
	screenArtMu.Unlock()

	before := countMethod(calls(), "getUserProfilePhotos")
	tb.ensureScreenArt()
	time.Sleep(80 * time.Millisecond)
	if countMethod(calls(), "getUserProfilePhotos") != before {
		t.Error("a freshly read avatar was re-probed too eagerly")
	}
}

func TestScreenKeepsArtFromTheBotAvatar(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	tb.resolveScreenArt()

	if _, ok := artFileID("main"); !ok {
		t.Fatal("resolveScreenArt did not cache the bot avatar")
	}
	tb.renderScreen(1, tb.newScreen("main", "with art", tb.homeRows()...))

	if countMethod(calls(), "getUserProfilePhotos") == 0 {
		t.Error("the avatar probe never ran, so no picture can have been cached")
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

// sheetBytesForTest reads the bytes back out of an uploadable file, so a test can
// decode the picture the bot would send.
func sheetBytesForTest(t *testing.T, file *telego.InputFile) []byte {
	t.Helper()
	// A byte-backed InputFile carries a NamedReader; reading it is what the
	// upload would do.
	if file.File == nil {
		t.Fatal("the sheet carries no data")
	}
	data, err := io.ReadAll(file.File)
	if err != nil {
		t.Fatalf("read the sheet: %v", err)
	}
	return data
}

// ord is rune to int, for the emoji range checks in the menu tests.
func ord(ch rune) int { return int(ch) }

// localizeWithRealBundle points the bot localizer at the shipped en-US file, so
// a test sees the text a user would, not the bare key.
func localizeWithRealBundle(t *testing.T) {
	t.Helper()
	path := filepath.Join(repoRootForTest(t), "internal/web/translation/en-US.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	bundle := i18n.NewBundle(language.MustParse("en-US"))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	if _, err := bundle.ParseMessageFileBytes(data, path); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	orig := locale.LocalizerBot
	locale.LocalizerBot = i18n.NewLocalizer(bundle, "en-US")
	t.Cleanup(func() { locale.LocalizerBot = orig })
}

// seeding a client bound to a Telegram user, which the client screens key on
func seedClientForTgUser(t *testing.T, tgUserID int64, email string) {
	t.Helper()
	// A client's Telegram binding lives in the inbound's settings blob, not in a
	// column: that is where GetClientTrafficTgBot reads it from.
	settings := fmt.Sprintf(`{"clients":[{"email":%q,"tgId":%d,"subId":"sub-%s"}]}`, email, tgUserID, email)
	inbound := &model.Inbound{
		UserId: 1, Tag: "tg-" + email, Remark: "tg-" + email, Port: 8443,
		Protocol: model.VLESS, Enable: true, Settings: settings,
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound for %s: %v", email, err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId: inbound.Id, Email: email, Enable: true, Up: 1, Down: 1,
	}).Error; err != nil {
		t.Fatalf("seed traffic for %s: %v", email, err)
	}
	record := (&model.Client{Email: email, Enable: true, SubID: "sub-" + email}).ToRecord()
	if err := database.GetDB().Create(record).Error; err != nil {
		t.Fatalf("seed client %s: %v", email, err)
	}
	if err := database.GetDB().Create(&model.ClientInbound{ClientId: record.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatalf("seed client_inbounds for %s: %v", email, err)
	}
}

// 1. The back button did nothing for a client: "home" was not one of the
// callbacks a non-admin was allowed to reach.
func TestClientBackButtonIsAllowed(t *testing.T) {
	if !isClientSelfCallback("home") {
		t.Fatal("a client's back button is still refused by the callback gate")
	}
}

// A client with one subscription has nothing to choose: the links must open
// directly instead of asking the client to tap its own name.
func TestClientMenuWithOneSubscriptionOpensLinksDirectly(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedClientForTgUser(t, 4242, "solo@x")

	tb.screenClientHome(777, 4242)

	if countMethod(calls(), "sendPhoto") == 0 {
		t.Fatal("nothing was rendered")
	}
	sc, ok := tb.screens().get(777)
	if !ok {
		t.Fatal("no screen was tracked")
	}
	markup, _ := json.Marshal(sc.markup)
	// The client's own top level: its subscription's real actions, right there,
	// and none of the controls that used to do nothing.
	for _, want := range []string{"client_sub_links", "client_qr_links"} {
		if !strings.Contains(string(markup), want) {
			t.Errorf("the client's menu lacks %s: %s", want, markup)
		}
	}
	for _, gone := range []string{"client_commands", "client_traffic", "pleaseChoose", "\"home\""} {
		if strings.Contains(string(markup), gone) {
			t.Errorf("the dead or pointless %s control is still offered: %s", gone, markup)
		}
	}
}

// Several subscriptions still need a choice, but one button per subscription
// rather than a "choose yourself" step.
func TestClientMenuWithSeveralSubscriptionsListsThem(t *testing.T) {
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedClientForTgUser(t, 4343, "a@x")
	seedClientForTgUser(t, 4343, "b@x")

	tb.screenClientHome(777, 4343)

	sc, ok := tb.screens().get(777)
	if !ok {
		t.Fatal("no screen was tracked")
	}
	markup, _ := json.Marshal(sc.markup)
	for _, email := range []string{"a@x", "b@x"} {
		if !strings.Contains(string(markup), email) {
			t.Errorf("the client's own subscription %q is not offered: %s", email, markup)
		}
	}
}

// A QR screen shows the code as its own picture; it is not a file beside it.
func TestQRScreenCarriesTheCodeAsItsPicture(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedClientForTgUser(t, 4444, "qr@x")

	tb.screenClientQR(777, "qr@x")

	if got := countMethod(calls(), "sendDocument"); got != 0 {
		t.Errorf("sendDocument calls = %d, want the QR on the screen instead of as a file", got)
	}
	if got := countMethod(calls(), "sendPhoto"); got == 0 {
		t.Fatal("the QR screen sent no picture")
	}
}

// /start is a reset: leaving the broadcast flow must not strand its prompt.
func TestStartClearsTheBroadcastPrompt(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t, 5555)
	broadcastLocalizer(t)
	t.Cleanup(func() { resetBroadcastState(t) })
	setBroadcastRunning(t, true)

	admin := chatUser{chatID: 5555, userID: 5555}
	tb.startBroadcast(admin)
	broadcastSetPrompt(admin, 4321)

	before := countMethod(calls(), "deleteMessage")
	tb.answerCommand(&telego.Message{
		MessageID: 9, Chat: telego.Chat{ID: 5555}, From: &telego.User{ID: 5555},
		Text: "/start",
	}, 5555, true)

	if got := countMethod(calls(), "deleteMessage"); got <= before {
		t.Error("the broadcast prompt survived /start")
	}
}

// /start is a command like any other: once it has opened the menu the command
// itself is removed, and a second /start edits the live screen instead of
// leaving the chat with two of them.
func TestStartCommandIsRemovedAndEditsTheLiveScreen(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t, 6666)

	// The handler deletes the command after answering, exactly as the router
	// wires it; calling both keeps the test on the real path.
	start := func(messageID int) {
		msg := &telego.Message{
			MessageID: messageID,
			Chat:      telego.Chat{ID: 6666},
			From:      &telego.User{ID: 6666},
			Text:      "/start",
		}
		tb.answerCommand(msg, 6666, true)
		tb.deleteIncoming(msg)
	}

	start(12)
	if countMethod(calls(), "sendPhoto") != 1 {
		t.Fatalf("sendPhoto calls = %d, want 1 for the first /start", countMethod(calls(), "sendPhoto"))
	}
	if countMethod(calls(), "deleteMessage") < 1 {
		t.Error("the /start command message was left in the chat")
	}

	// A second /start must redraw the tracked screen, not send another one.
	start(13)
	if got := countMethod(calls(), "sendPhoto"); got != 1 {
		t.Errorf("sendPhoto calls = %d, want 1: a second /start duplicated the menu", got)
	}
	if countMethod(calls(), "editMessageMedia") == 0 {
		t.Error("the second /start did not edit the live screen")
	}
}

// The screen tests compare callback data, so a button whose translation key does
// not exist still passes them: the label is the empty string. /start broke that
// way. This one loads the real en-US file and insists the client's menu has
// visible labels.
func TestClientMenuButtonsHaveRealLabels(t *testing.T) {
	localizeWithRealBundle(t)
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedClientForTgUser(t, 4545, "labelled@x")

	tb.screenClientHome(777, 4545)

	sc, ok := tb.screens().get(777)
	if !ok {
		t.Fatal("the client's /start produced no screen")
	}
	if len(sc.rows) == 0 {
		t.Fatal("the client's screen has no buttons")
	}
	for _, row := range sc.rows {
		for _, button := range row {
			if strings.TrimSpace(button.Text) == "" {
				t.Errorf("a button on the client's menu has an empty label (callback %q)", button.CallbackData)
			}
			if strings.HasPrefix(button.Text, "tgbot.") {
				t.Errorf("button %q shows its raw key instead of text: the key is missing", button.CallbackData)
			}
		}
	}
	if strings.TrimSpace(sc.pages[0]) == "" {
		t.Error("the client's screen body is empty")
	}
}

// 1. A finished broadcast is a message like any other: its run controls go, and
// what stays is the hide button.
func TestFinishedBroadcastCardKeepsOnlyHide(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	initReportDB(t)
	broadcastLocalizer(t)

	runner := &broadcastRunner{actor: chatUser{chatID: 100}, chatID: 100, messageID: 5}
	if !tb.finalizeBroadcastCard(runner, "done") {
		t.Fatal("the summary card could not be edited")
	}

	edits := 0
	for _, call := range calls() {
		if call.Method != "editMessageText" {
			continue
		}
		edits++
		markup, _ := json.Marshal(call.Payload["reply_markup"])
		if !strings.Contains(string(markup), "\"hide\"") {
			t.Errorf("the finished card offers no hide button: %s", markup)
		}
		if strings.Contains(string(markup), "\"broadcast_cancel\"") {
			t.Errorf("the finished card still offers the run controls: %s", markup)
		}
	}
	if edits != 1 {
		t.Errorf("editMessageText calls = %d, want 1", edits)
	}
}

// 3. "Client card" on the links screen only repeats the client's own usage, so a
// client must not be offered it; an admin still gets it.
func TestLinksScreenHidesTheClientCardFromItsOwner(t *testing.T) {
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedClientForTgUser(t, 4646, "mine@x")

	tb.screenClientLinks(4646, "mine@x")
	sc, ok := tb.screens().get(4646)
	if !ok {
		t.Fatal("no screen was tracked")
	}
	markup, _ := json.Marshal(sc.markup)
	if strings.Contains(string(markup), "client_get_usage") {
		t.Errorf("a client is still offered the admin's client card: %s", markup)
	}
	// The other links are what the client came for.
	for _, want := range []string{"client_individual_links", "client_qr_links"} {
		if !strings.Contains(string(markup), want) {
			t.Errorf("the links screen lacks %s: %s", want, markup)
		}
	}
}

// 4. The QR screen carries every code the subscription has, as one picture.
func TestQRScreenCarriesEveryCodeOnOnePicture(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedClientForTgUser(t, 4747, "codes@x")

	tb.screenClientQR(777, "codes@x")

	if got := countMethod(calls(), "sendDocument"); got != 0 {
		t.Errorf("sendDocument calls = %d, want the codes on the screen's picture", got)
	}
	if countMethod(calls(), "sendPhoto") == 0 {
		t.Fatal("the QR screen sent no picture")
	}
	sc, ok := tb.screens().get(777)
	if !ok {
		t.Fatal("no screen was tracked")
	}
	if sc.qrPicture == nil {
		t.Fatal("the QR screen carries no sheet of codes")
	}
}

// The sheet itself, independent of any service: every code given is drawn, and
// the legend numbers exactly that many.
func TestQRSheetDrawsEveryCodeAndNumbersIt(t *testing.T) {
	codes := []subscriptionQRCode{
		{content: "https://panel.example/sub/abc", kind: "sub"},
		{content: "https://panel.example/json/abc", kind: "json"},
		{content: "vless://uuid@host:443?type=tcp#app1", kind: "link", app: 1},
		{content: "vless://uuid@host:8443?type=ws#app2", kind: "link", app: 2},
	}

	picture := qrSheetPicture(codes)
	if picture == nil {
		t.Fatal("the sheet could not be rendered")
	}

	// Decode what the encoder produced, so a broken picture is caught here and not
	// in the chat. Four codes fill three columns, so the sheet is 3x2 tiles with
	// padding between and around them.
	img, err := png.Decode(bytes.NewReader(sheetBytesForTest(t, picture)))
	if err != nil {
		t.Fatalf("the sheet is not a valid PNG: %v", err)
	}
	wantW := 3*qrTileSize + 4*qrSheetPad
	wantH := 2*qrTileSize + 3*qrSheetPad
	if img.Bounds().Dx() != wantW || img.Bounds().Dy() != wantH {
		t.Errorf("sheet is %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), wantW, wantH)
	}
}

// ru-Locale labels for the menu, so these tests are independent of the localizer.
func TestAdminCategoriesHaveEmojiOnEveryButton(t *testing.T) {
	localizeWithRealBundle(t)
	tb := &Tgbot{}

	hasEmoji := func(s string) bool {
		for _, ch := range s {
			o := ord(ch)
			if (o >= 0x1F000 && o <= 0x1FAFF) || (o >= 0x2600 && o <= 0x27BF) ||
				(o >= 0x2B00 && o <= 0x2BFF) || o == 0xFE0F {
				return true
			}
		}
		return false
	}

	for _, category := range []string{"server", "clients", "traffic", "maintenance"} {
		rows := tb.categoryRows(category)
		if len(rows) == 0 {
			t.Fatalf("category %q has no buttons", category)
		}
		for _, row := range rows {
			for _, button := range row {
				if button.CallbackData == cbHome {
					continue // the back row is a plain arrow by design
				}
				if !hasEmoji(button.Text) {
					t.Errorf("category %q: button %q has no emoji: %q",
						category, button.CallbackData, button.Text)
				}
			}
		}
	}
}

// A screen must not print "Refreshed on" twice: the card carries one and the
// caller used to append a second.
func TestClientScreensPrintRefreshedOnOnce(t *testing.T) {
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedClientForTgUser(t, 4848, "once@x")

	// The admin's client card, which is where the duplicate was visible.
	tb.searchClientScreen(777, "once@x")
	sc, ok := tb.screens().get(777)
	if !ok {
		t.Fatal("no screen was tracked")
	}
	needle := tb.I18nBot("tgbot.messages.refreshedOn", "Time==x")
	if needle == "" {
		t.Skip("no localizer in this test environment")
	}
	// Compare on the key itself, which survives an empty localizer.
	if got := strings.Count(sc.pages[0], "refreshedOn"); got > 1 {
		t.Errorf("refreshedOn appears %d times on the client card", got)
	}
	if got := strings.Count(sc.body, "Refreshed"); got > 1 && needle != "" {
		t.Errorf("the refreshed line is printed %d times", got)
	}
}

// Inbounds belongs with the server, not under traffic: it says what the server
// listens on, and the traffic category is about quota.
func TestInboundsSitsInTheServerCategory(t *testing.T) {
	tb := &Tgbot{}
	in := func(category, want string) bool {
		for _, row := range tb.categoryRows(category) {
			for _, button := range row {
				if button.CallbackData == want {
					return true
				}
			}
		}
		return false
	}
	if !in("server", cbInbounds) {
		t.Error("the Server category does not offer Inbounds")
	}
	if in("traffic", cbInbounds) {
		t.Error("the Traffic category still offers Inbounds")
	}
}

// A delivered broadcast copy carries the hide button when the recipient is one of
// the bot's admins: that recipient has no panel menu to clear it from.
func TestBroadcastCopyToAnotherAdminCarriesHide(t *testing.T) {
	localizeWithRealBundle(t)
	srv, calls := recordingServer(t, false)
	swapTestBot(t, srv.URL)
	t.Cleanup(srv.Close)
	setTestAdmins(t, 111, 222)

	if _, err := deliverBroadcastCopy(222, broadcastDraft{FromChatID: 111, MessageIDs: []int{5}}); err != nil {
		t.Fatalf("deliverBroadcastCopy: %v", err)
	}

	got := false
	for _, call := range calls() {
		if call.Method != "editMessageReplyMarkup" {
			continue
		}
		markup, _ := json.Marshal(call.Payload["reply_markup"])
		if strings.Contains(string(markup), "\"hide\"") {
			got = true
		}
	}
	if !got {
		t.Error("the copy delivered to an admin carries no hide button")
	}
}

// 1. EVERY recipient gets the hide button, not just another admin: a client has
// no panel menu, so without it the copy cannot be dismissed at all.
func TestBroadcastCopyToAClientCarriesHide(t *testing.T) {
	localizeWithRealBundle(t)
	srv, calls := recordingServer(t, false)
	swapTestBot(t, srv.URL)
	t.Cleanup(srv.Close)
	setTestAdmins(t, 111) // 222 is an ordinary client, not an admin

	if _, err := deliverBroadcastCopy(222, broadcastDraft{FromChatID: 111, MessageIDs: []int{5}}); err != nil {
		t.Fatalf("deliverBroadcastCopy: %v", err)
	}

	got := ""
	for _, call := range calls() {
		if call.Method != "editMessageReplyMarkup" {
			continue
		}
		markup, _ := json.Marshal(call.Payload["reply_markup"])
		got = string(markup)
	}
	if !strings.Contains(got, "\"hide\"") {
		t.Errorf("the copy delivered to a client carries no hide button: %s", got)
	}
	// A blank label would be worse than English: a client's language need not be
	// one the panel ships.
	if !strings.Contains(got, "Hide") {
		t.Errorf("the hide button has no visible label: %s", got)
	}
}

// The hide button must reach a client through the callback gate, or it would be
// drawn and then silently ignored.
func TestClientCanReachTheHideCallback(t *testing.T) {
	if !isClientSelfCallback("hide") {
		t.Fatal("a client's hide button is refused by the callback gate")
	}
}

// 2. The individual-links screen offered "Links", which did what the back row
// already does.
func TestIndividualLinksScreenHasNoDuplicateLinksButton(t *testing.T) {
	// The same harness the ownership tests use: it seeds a real client whose
	// subscription actually resolves, which the individual links need.
	// The screen's own keyboard, without going through the subscription service:
	// drawing the full screen needs a subscription the panel can resolve, which
	// the DB-only harness cannot produce.
	rows := (&Tgbot{}).individualLinksRows()
	if len(rows) != 1 || len(rows[0]) != 1 {
		t.Fatalf("the individual-links screen has %d row(s), want exactly the back row", len(rows))
	}
	if got := rows[0][0].CallbackData; got != cbHome {
		t.Errorf("the only control is %q, want the back row %q", got, cbHome)
	}
}
