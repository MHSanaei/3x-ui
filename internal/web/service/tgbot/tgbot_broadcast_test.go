package tgbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"

	telegoapi "github.com/mymmrac/telego/telegoapi"

	"github.com/mymmrac/telego"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// newBroadcastMock serves ok:true for every Telegram method and records the
// per-method call counts and request bodies, so tests can assert what the
// broadcast actually put on the wire. copyMessages answers with an array of
// message ids, mirroring the real API.
func newBroadcastMock(t *testing.T) (url string, calls func(string) int, bodies func(string) []map[string]any) {
	t.Helper()
	var mu sync.Mutex
	counts := map[string]int{}
	sent := map[string][]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		payload := map[string]any{}
		_ = json.Unmarshal(raw, &payload)
		method := strings.TrimPrefix(r.URL.Path, "/bot"+testBotToken+"/")
		message := map[string]any{"message_id": 7, "date": 0, "chat": map[string]any{"id": 1, "type": "private"}}
		result := any(message)
		if method == "copyMessages" {
			result = []any{message, message}
		}
		mu.Lock()
		counts[method]++
		sent[method] = append(sent[method], payload)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv.URL,
		func(method string) int {
			mu.Lock()
			defer mu.Unlock()
			return counts[method]
		},
		func(method string) []map[string]any {
			mu.Lock()
			defer mu.Unlock()
			return append([]map[string]any(nil), sent[method]...)
		}
}

func setBroadcastAdmins(t *testing.T, ids []int64) {
	t.Helper()
	tgBotMutex.Lock()
	orig := adminIds
	adminIds = ids
	tgBotMutex.Unlock()
	t.Cleanup(func() {
		tgBotMutex.Lock()
		adminIds = orig
		tgBotMutex.Unlock()
	})
}

func setBroadcastRunning(t *testing.T, running bool) {
	t.Helper()
	tgBotMutex.Lock()
	orig := isRunning
	isRunning = running
	tgBotMutex.Unlock()
	t.Cleanup(func() {
		tgBotMutex.Lock()
		isRunning = orig
		tgBotMutex.Unlock()
	})
}

func swapBroadcastSender(t *testing.T, sender func(int64, broadcastDraft) error, pause func(time.Duration)) {
	t.Helper()
	origSend, origPause := broadcastSender, broadcastPause
	t.Cleanup(func() {
		broadcastSender, broadcastPause = origSend, origPause
	})
	broadcastSender = sender
	if pause != nil {
		broadcastPause = pause
	}
}

// broadcastLocalizer renders the broadcast keys a test asserts on; without it
// I18n returns the bare key instead of the template output.
func broadcastLocalizer(t *testing.T) {
	t.Helper()
	bundle := i18n.NewBundle(language.MustParse("en-US"))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	_ = bundle.AddMessages(language.MustParse("en-US"),
		&i18n.Message{ID: "tgbot.messages.broadcastPreview", Other: "📤 This message will go to {{ .Count }} recipients. Send it?"},
		&i18n.Message{ID: "tgbot.messages.broadcastNotCopyable", Other: "❗ This message can't be copied for broadcast."},
		&i18n.Message{ID: "tgbot.messages.broadcastAskText", Other: "send the message"},
		&i18n.Message{ID: "tgbot.messages.broadcastAlreadyRunning", Other: "already running"},
		&i18n.Message{ID: "tgbot.messages.broadcastProgress", Other: "progress {{ .Sent }}/{{ .Total }} failed {{ .Failed }}"},
		&i18n.Message{ID: "tgbot.messages.broadcastFinished", Other: "finished"},
		&i18n.Message{ID: "tgbot.messages.broadcastCanceled", Other: "canceled"},
	)
	orig := locale.LocalizerBot
	t.Cleanup(func() { locale.LocalizerBot = orig })
	locale.LocalizerBot = i18n.NewLocalizer(bundle, "en-US")
}

func createBroadcastInbound(t *testing.T, tag, settings string) {
	t.Helper()
	inbound := &model.Inbound{Tag: tag, Settings: settings, Enable: true}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("create inbound %s: %v", tag, err)
	}
}

// broadcastClientsJSON renders an inbound settings blob with the given tgIds.
func broadcastClientsJSON(t *testing.T, tgIDs ...int64) string {
	t.Helper()
	clients := make([]string, 0, len(tgIDs))
	for i, tgID := range tgIDs {
		clients = append(clients, fmt.Sprintf(`{"email":"user%d@x","tgId":%d}`, i, tgID))
	}
	return `{"clients":[` + strings.Join(clients, ",") + `]}`
}

// resetBroadcastState clears the shared broadcast globals before a test
// asserts on them: shuffled tests may inherit state from an earlier test.
func resetBroadcastState(t *testing.T, chatID int64) {
	t.Helper()
	if runner := broadcastCurrentRunner(); runner != nil {
		broadcastUnregisterRunner(runner)
	}
	broadcastResetDraft()
	broadcastDropBuffer()
	userStateMgr.clear(chatID)
}

func swapAlbumDebounce(t *testing.T, d time.Duration) {
	t.Helper()
	orig := broadcastAlbumDebounce
	t.Cleanup(func() { broadcastAlbumDebounce = orig })
	broadcastAlbumDebounce = d
}

// waitBroadcastDraft polls until the debounce finalizer has stored a draft.
func waitBroadcastDraft(t *testing.T, chatID int64) broadcastDraft {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if draft, ok := broadcastTakeDraft(chatID); ok {
			return draft
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("album draft was never finalized in time")
	return broadcastDraft{}
}

func TestCollectBroadcastRecipients(t *testing.T) {
	tb := newStaleButtonTgbot(t)
	setBroadcastAdmins(t, []int64{222})

	// 111 appears on both inbounds, 222 is an admin, 0 has no Telegram ID.
	createBroadcastInbound(t, "in-1", broadcastClientsJSON(t, 111, 111, 222))
	createBroadcastInbound(t, "in-2", broadcastClientsJSON(t, 111, 333, 0, 444))

	got := tb.collectBroadcastRecipients()
	slices.Sort(got)
	if !slices.Equal(got, []int64{111, 333, 444}) {
		t.Fatalf("collectBroadcastRecipients() = %v, want [111 333 444]", got)
	}
}

func assertBroadcastResult(t *testing.T, got, want broadcastResult) {
	t.Helper()
	got.Elapsed, want.Elapsed = 0, 0
	if got != want {
		t.Errorf("result = %+v, want %+v", got, want)
	}
}

func TestRunBroadcastCounters(t *testing.T) {
	url, _, _ := newBroadcastMock(t)
	swapTestBot(t, url)
	setBroadcastRunning(t, true)
	blocked := &telegoapi.Error{ErrorCode: 403, Description: "Forbidden: bot was blocked by the user"}

	tests := []struct {
		name       string
		recipients []int64
		outcomes   map[int64]error
		delivered  int
		failed     int
	}{
		{"all delivered", []int64{1, 2, 3}, map[int64]error{1: nil, 2: nil, 3: nil}, 3, 0},
		{
			"blocked and transient errors count as failed",
			[]int64{1, 2, 3, 4},
			map[int64]error{1: nil, 2: blocked, 3: nil, 4: errors.New("connection reset")},
			2, 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			swapBroadcastSender(t, func(chatID int64, _ broadcastDraft) error {
				return tt.outcomes[chatID]
			}, func(time.Duration) {})

			runner := &broadcastRunner{chatID: 100, messageID: 5}
			(&Tgbot{}).runBroadcast(runner, broadcastDraft{FromChatID: 100, MessageIDs: []int{7}}, tt.recipients)

			assertBroadcastResult(t, runner.getResult(), broadcastResult{
				Total:     len(tt.recipients),
				Delivered: tt.delivered,
				Failed:    tt.failed,
				Skipped:   0,
			})
		})
	}
}

func TestRunBroadcastCancelsMidway(t *testing.T) {
	url, _, _ := newBroadcastMock(t)
	swapTestBot(t, url)
	setBroadcastRunning(t, true)

	runner := &broadcastRunner{chatID: 100, messageID: 5}
	swapBroadcastSender(t, func(chatID int64, _ broadcastDraft) error {
		if chatID == 1 {
			runner.cancel.Store(true)
		}
		return nil
	}, func(time.Duration) {})

	(&Tgbot{}).runBroadcast(runner, broadcastDraft{FromChatID: 100, MessageIDs: []int{7}}, []int64{1, 2, 3, 4, 5})

	assertBroadcastResult(t, runner.getResult(), broadcastResult{
		Total:     5,
		Delivered: 1,
		Failed:    0,
		Skipped:   4,
		Canceled:  true,
	})
	if broadcastCurrentRunner() != nil {
		t.Errorf("broadcast slot still registered after the run finished")
	}
}

func TestBroadcastDeliverOneRetries429(t *testing.T) {
	flood := func(after int) error {
		return &telegoapi.Error{
			ErrorCode:   429,
			Description: "Too Many Requests: retry after " + fmt.Sprint(after),
			Parameters:  &telegoapi.ResponseParameters{RetryAfter: after},
		}
	}

	tests := []struct {
		name       string
		responses  []error
		wantErr    string
		wantCalls  int
		wantPauses []time.Duration
	}{
		{
			name:       "flood control waits and retries the same recipient",
			responses:  []error{flood(2), nil},
			wantCalls:  2,
			wantPauses: []time.Duration{2 * time.Second},
		},
		{
			name:       "gives up after the retry budget",
			responses:  []error{flood(1), flood(1), flood(1), flood(1), flood(1), flood(1), nil},
			wantErr:    `429 "Too Many Requests: retry after 1", migrate to chat ID: 0, retry after: 1`,
			wantCalls:  6,
			wantPauses: []time.Duration{time.Second, time.Second, time.Second, time.Second, time.Second},
		},
		{
			name:      "non-429 errors are returned without retrying",
			responses: []error{errors.New("connection reset")},
			wantErr:   "connection reset",
			wantCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			callNum := 0
			var pauses []time.Duration
			swapBroadcastSender(t, func(int64, broadcastDraft) error {
				mu.Lock()
				defer mu.Unlock()
				callNum++
				if callNum > len(tt.responses) {
					return nil
				}
				return tt.responses[callNum-1]
			}, func(d time.Duration) {
				mu.Lock()
				defer mu.Unlock()
				pauses = append(pauses, d)
			})

			err := broadcastDeliverOne(9, broadcastDraft{FromChatID: 100, MessageIDs: []int{7}})

			mu.Lock()
			defer mu.Unlock()
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Errorf("broadcastDeliverOne() error = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("broadcastDeliverOne() error = %v, want nil", err)
			}
			if callNum != tt.wantCalls {
				t.Errorf("sender calls = %d, want %d", callNum, tt.wantCalls)
			}
			if len(pauses) != len(tt.wantPauses) {
				t.Fatalf("pauses = %v, want %v", pauses, tt.wantPauses)
			}
			for i, p := range tt.wantPauses {
				if pauses[i] != p {
					t.Errorf("pauses[%d] = %v, want %v", i, pauses[i], p)
				}
			}
		})
	}
}

func TestBroadcastRegisterRunnerSingleSlot(t *testing.T) {
	first := broadcastRegisterRunner(1)
	if first == nil {
		t.Fatal("broadcastRegisterRunner() = nil for an idle bot")
	}
	t.Cleanup(func() { broadcastUnregisterRunner(first) })

	if second := broadcastRegisterRunner(2); second != nil {
		t.Fatalf("broadcastRegisterRunner() = %v while a broadcast is running, want nil", second)
	}

	broadcastUnregisterRunner(first)
	if broadcastCurrentRunner() != nil {
		t.Fatalf("slot still registered after unregister")
	}
}

func TestStartBroadcastRefusesWhileRunning(t *testing.T) {
	broadcastLocalizer(t)
	const chatID = int64(9102)
	resetBroadcastState(t, chatID)

	runner := broadcastRegisterRunner(chatID)
	t.Cleanup(func() {
		broadcastUnregisterRunner(runner)
		userStateMgr.clear(chatID)
	})

	(&Tgbot{}).startBroadcast(chatID)

	if state, ok := userStateMgr.get(chatID); ok {
		t.Fatalf("state = %q while a broadcast is running, want none", state)
	}
}

func TestBroadcastCommandRequiresAdmin(t *testing.T) {
	const chatID = int64(9101)
	resetBroadcastState(t, chatID)

	message := &telego.Message{Chat: telego.Chat{ID: chatID}, Text: "/broadcast"}
	(&Tgbot{}).answerCommand(message, chatID, false)

	if state, ok := userStateMgr.get(chatID); ok {
		t.Fatalf("non-admin /broadcast set state %q", state)
	}
	if draft, ok := broadcastTakeDraft(chatID); ok || !draft.empty() {
		t.Fatalf("non-admin /broadcast produced a draft %v", draft)
	}
	if broadcastCurrentRunner() != nil {
		t.Fatalf("non-admin /broadcast started a runner")
	}
}

func TestBroadcastStartCommandSetsState(t *testing.T) {
	broadcastLocalizer(t)
	const chatID = int64(9103)
	resetBroadcastState(t, chatID)
	defer func() {
		userStateMgr.clear(chatID)
		broadcastResetDraft()
	}()

	(&Tgbot{}).answerCommand(&telego.Message{Chat: telego.Chat{ID: chatID}, Text: "/broadcast"}, chatID, true)

	state, ok := userStateMgr.get(chatID)
	if !ok || state != broadcastAwaitingText {
		t.Fatalf("state = %q (ok=%v), want %q", state, ok, broadcastAwaitingText)
	}
}

func TestDeliverBroadcastCopy(t *testing.T) {
	tests := []struct {
		name        string
		draft       broadcastDraft
		wantMethods map[string]int
		check       func(t *testing.T, bodies func(string) []map[string]any)
	}{
		{
			name:        "a single message rides copyMessage",
			draft:       broadcastDraft{FromChatID: 55, MessageIDs: []int{7}},
			wantMethods: map[string]int{"copyMessage": 1},
			check: func(t *testing.T, bodies func(string) []map[string]any) {
				body := bodies("copyMessage")[0]
				if fmt.Sprint(body["from_chat_id"]) != "55" || fmt.Sprint(body["message_id"]) != "7" {
					t.Errorf("copy body = %v, want from 55 message 7", body)
				}
			},
		},
		{
			name:        "an album rides one copyMessages call",
			draft:       broadcastDraft{FromChatID: 55, MessageIDs: []int{1, 2, 3}},
			wantMethods: map[string]int{"copyMessages": 1, "copyMessage": 0},
			check: func(t *testing.T, bodies func(string) []map[string]any) {
				if fmt.Sprint(bodies("copyMessages")[0]["message_ids"]) != "[1 2 3]" {
					t.Errorf("message_ids = %v, want [1 2 3]", bodies("copyMessages")[0]["message_ids"])
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A fresh mock per case keeps the per-method counts independent.
			url, calls, bodies := newBroadcastMock(t)
			swapTestBot(t, url)

			if err := deliverBroadcastCopy(66, tt.draft); err != nil {
				t.Fatalf("deliverBroadcastCopy() error = %v", err)
			}
			for method, want := range tt.wantMethods {
				if got := calls(method); got != want {
					t.Errorf("%s calls = %d, want %d", method, got, want)
				}
			}
			if tt.check != nil {
				tt.check(t, bodies)
			}
		})
	}
}

// Regression: a media group used to produce one draft per photo, so three
// photos meant three previews and only the last tapped one was delivered.
func TestHandleBroadcastInputMediaGroup(t *testing.T) {
	broadcastLocalizer(t)
	url, calls, bodies := newBroadcastMock(t)
	swapTestBot(t, url)
	setBroadcastRunning(t, true)
	swapAlbumDebounce(t, 20*time.Millisecond)
	tb := newStaleButtonTgbot(t)
	setBroadcastAdmins(t, nil)

	const chatID = int64(9109)
	resetBroadcastState(t, chatID)
	createBroadcastInbound(t, "in-1", broadcastClientsJSON(t, 601))

	userStateMgr.set(chatID, broadcastAwaitingText)
	// Updates of one album arrive out of order and copyMessages demands
	// strictly increasing ids, so the draft must sort them.
	for _, id := range []int{103, 101, 102} {
		tb.handleBroadcastInput(&telego.Message{
			Chat:         telego.Chat{ID: chatID},
			MessageID:    id,
			MediaGroupID: "grp9",
			Photo:        []telego.PhotoSize{{FileID: "unused"}},
		})
	}

	draft := waitBroadcastDraft(t, chatID)
	if !slices.Equal(draft.MessageIDs, []int{101, 102, 103}) || draft.FromChatID != chatID {
		t.Fatalf("album draft = %+v, want the three album message ids", draft)
	}
	if state, ok := userStateMgr.get(chatID); ok {
		t.Errorf("state = %q after the album was accepted, want cleared", state)
	}
	if got := calls("copyMessages"); got != 1 {
		t.Errorf("copyMessages calls = %d, want 1 self-copy of the whole album", got)
	}
	// copyMessages rejects ids that are not strictly increasing.
	if got := fmt.Sprint(bodies("copyMessages")[0]["message_ids"]); got != "[101 102 103]" {
		t.Errorf("self-copy message_ids = %v, want [101 102 103]", got)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && calls("sendMessage") == 0 {
		time.Sleep(2 * time.Millisecond)
	}
	if calls("sendMessage") != 1 {
		t.Errorf("sendMessage calls = %d, want 1 confirmation card", calls("sendMessage"))
	}
}

func TestHandleBroadcastInputSingleMessage(t *testing.T) {
	broadcastLocalizer(t)
	url, calls, _ := newBroadcastMock(t)
	swapTestBot(t, url)
	setBroadcastRunning(t, true)
	tb := newStaleButtonTgbot(t)
	setBroadcastAdmins(t, nil)

	const chatID = int64(9104)
	resetBroadcastState(t, chatID)
	createBroadcastInbound(t, "in-1", broadcastClientsJSON(t, 602))

	userStateMgr.set(chatID, broadcastAwaitingText)
	tb.handleBroadcastInput(&telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 42, Text: "hello all"})

	draft, ok := broadcastTakeDraft(chatID)
	if !ok || draft.FromChatID != chatID || !slices.Equal(draft.MessageIDs, []int{42}) {
		t.Fatalf("draft = %+v (ok=%v), want a reference to message 42", draft, ok)
	}
	if got := calls("copyMessage"); got != 1 {
		t.Errorf("copyMessage calls = %d, want 1 self-copy preview", got)
	}
}

func waitBroadcastFinished(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if broadcastCurrentRunner() == nil {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("broadcast did not finish in time")
}

func TestConfirmBroadcastEndToEnd(t *testing.T) {
	broadcastLocalizer(t)
	url, calls, _ := newBroadcastMock(t)
	swapTestBot(t, url)
	setBroadcastRunning(t, true)
	tb := newStaleButtonTgbot(t)
	setBroadcastAdmins(t, nil)

	const chatID = int64(9105)
	resetBroadcastState(t, chatID)

	createBroadcastInbound(t, "in-1", broadcastClientsJSON(t, 501, 502))
	broadcastSetDraft(chatID, broadcastDraft{FromChatID: chatID, MessageIDs: []int{9}})

	tb.answerCallback(&telego.CallbackQuery{
		ID:      "q1",
		From:    telego.User{ID: chatID},
		Data:    "broadcast_confirm",
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 5},
	}, true)

	waitBroadcastFinished(t)

	// Two copies to recipients; the preview card is edited into the progress
	// card and then into the final summary, so the summary is never sent twice.
	if got := calls("copyMessage"); got != 2 {
		t.Errorf("copyMessage calls = %d, want 2 deliveries", got)
	}
	if got := calls("sendMessage"); got != 0 {
		t.Errorf("sendMessage calls = %d, want 0: the card is edited, not re-sent", got)
	}
	if got := calls("editMessageText"); got != 2 {
		t.Errorf("editMessageText calls = %d, want 2 (progress + summary)", got)
	}
	if got := calls("answerCallbackQuery"); got != 1 {
		t.Errorf("answerCallbackQuery calls = %d, want 1", got)
	}
}

func TestConfirmBroadcastWithoutDraftAnswersError(t *testing.T) {
	url, calls, _ := newBroadcastMock(t)
	swapTestBot(t, url)
	tb := newStaleButtonTgbot(t)
	const chatID = int64(9106)
	resetBroadcastState(t, chatID)

	tb.answerCallback(&telego.CallbackQuery{
		ID:      "q1",
		From:    telego.User{ID: chatID},
		Data:    "broadcast_confirm",
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 5},
	}, true)

	if calls("answerCallbackQuery") != 1 {
		t.Errorf("answerCallbackQuery calls = %d, want 1 error answer", calls("answerCallbackQuery"))
	}
	if broadcastCurrentRunner() != nil {
		t.Errorf("a confirm without a draft must not start a broadcast")
	}
}

func TestBroadcastCancelCallbackClearsDraft(t *testing.T) {
	url, calls, _ := newBroadcastMock(t)
	swapTestBot(t, url)
	tb := newStaleButtonTgbot(t)

	const chatID = int64(9107)
	resetBroadcastState(t, chatID)

	userStateMgr.set(chatID, broadcastAwaitingText)
	broadcastSetDraft(chatID, broadcastDraft{FromChatID: chatID, MessageIDs: []int{9}})

	tb.answerCallback(&telego.CallbackQuery{
		ID:      "q1",
		From:    telego.User{ID: chatID},
		Data:    "broadcast_cancel",
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 9},
	}, true)

	if _, ok := userStateMgr.get(chatID); ok {
		t.Errorf("state survived the cancel tap")
	}
	if draft, ok := broadcastTakeDraft(chatID); ok || !draft.empty() {
		t.Errorf("draft = %+v (ok=%v) after the cancel tap, want dropped", draft, ok)
	}
	if got := calls("deleteMessage"); got != 1 {
		t.Errorf("deleteMessage calls = %d, want 1", got)
	}
	if got := calls("answerCallbackQuery"); got != 1 {
		t.Errorf("answerCallbackQuery calls = %d, want 1", got)
	}
}

func TestBroadcastCallbacksDeniedToNonAdmin(t *testing.T) {
	url, calls, _ := newBroadcastMock(t)
	swapTestBot(t, url)
	tb := newStaleButtonTgbot(t)

	const chatID = int64(9108)
	resetBroadcastState(t, chatID)

	for _, data := range []string{"broadcast_confirm", "broadcast_cancel"} {
		tb.answerCallback(&telego.CallbackQuery{
			ID:      "q1",
			From:    telego.User{ID: 999999},
			Data:    data,
			Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 5},
		}, false)
		if calls("answerCallbackQuery") != 0 {
			t.Fatalf("%s answered a non-admin callback", data)
		}
		if broadcastCurrentRunner() != nil {
			t.Fatalf("%s started a broadcast for a non-admin", data)
		}
	}
}
