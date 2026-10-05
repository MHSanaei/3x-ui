package tgbot

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
)

// noticeServer records sendMessage/sendDocument/editMessageText calls, with the
// body a user would see, so a test can assert on how many messages an event
// burst produced.
func noticeServer(t *testing.T) (*httptest.Server, func() []apiCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []apiCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		sent := extractSentBody(t, r)
		mu.Lock()
		calls = append(calls, apiCall{Method: method, Payload: map[string]any{
			"text": sent.Text, "chat_id": float64(sent.ChatID),
		}})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch method {
		case "sendMessage", "sendDocument", "sendPhoto":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"message_id": 77, "date": 0, "chat": map[string]any{"id": sent.ChatID, "type": "private"},
			}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"message_id": 77, "date": 0, "chat": map[string]any{"id": sent.ChatID, "type": "private"},
			}})
		}
	}))
	return srv, func() []apiCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]apiCall(nil), calls...)
	}
}

// Regression test: the same event twice must edit its live card, not post a
// second message — a flapping outbound used to bury the chat.
func TestRepeatedEventEditsItsLiveCard(t *testing.T) {
	srv, calls := noticeServer(t)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	setTestAdmins(t, 1)
	notices.reset()

	tb := initReportDB(t)
	event := eventbus.Event{Type: eventbus.EventOutboundDown, Source: "nl-node"}

	tb.liveNotice(1, "outbound_down:nl-node", "Outbound nl-node is DOWN")
	tb.liveNotice(1, "outbound_down:nl-node", "Outbound nl-node is DOWN")
	tb.liveNotice(1, "outbound_down:nl-node", "Outbound nl-node is DOWN")

	got := calls()
	if n := countMethod(got, "sendMessage"); n != 1 {
		t.Errorf("sendMessage = %d, want 1: repeats must edit, not post (calls: %v)", n, methods(got))
	}
	if n := countMethod(got, "editMessageText"); n != 2 {
		t.Errorf("editMessageText = %d, want 2: each repeat redraws the card", n)
	}
	_ = event
}

// A different kind of event gets its own card: they tell unrelated stories.
func TestDifferentEventKindsKeepSeparateCards(t *testing.T) {
	srv, calls := noticeServer(t)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	setTestAdmins(t, 1)
	notices.reset()

	tb := initReportDB(t)
	tb.liveNotice(1, "outbound_down:a", "A is DOWN")
	tb.liveNotice(1, "xray_crash:x", "Xray crashed")

	if n := countMethod(calls(), "sendMessage"); n != 2 {
		t.Errorf("sendMessage = %d, want 2: unrelated events need their own cards", n)
	}
}

// The backup must arrive as one message: the file carries the summary.
func TestBackupArrivesAsOneMessageWithACaption(t *testing.T) {
	srv, calls := noticeServer(t)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	notices.reset()

	tb := initReportDB(t)
	tb.sendDocumentWithCaption(1, []byte("db-bytes"), "x-ui.db", "host\nbackup time")

	got := calls()
	sends := 0
	for _, c := range got {
		if c.Method == "sendMessage" || c.Method == "sendDocument" {
			sends++
		}
	}
	if sends != 1 {
		t.Fatalf("messages = %d, want 1 (calls: %v)", sends, methods(got))
	}
	if countMethod(got, "sendDocument") != 1 {
		t.Error("the backup did not travel as a document")
	}
}

// A caption longer than Telegram's limit is cut on a line break, never mid-line.
func TestTrimCaptionCutsOnALineBreak(t *testing.T) {
	body := strings.Repeat("line of a long report\n", 200)
	got := trimCaption(body, botCaptionLimit)
	if len([]rune(got)) > botCaptionLimit {
		t.Fatalf("caption is %d runes, want at most %d", len([]rune(got)), botCaptionLimit)
	}
	if strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\r") {
		t.Errorf("caption ends on a dangling line break: %q", got[max(0, len(got)-20):])
	}
}

// Hiding a notice forgets its card, so the next event does not edit a message
// the user just deleted.
func TestHiddenNoticeIsForgotten(t *testing.T) {
	srv, _ := noticeServer(t)
	swapTestBot(t, srv.URL)
	defer srv.Close()
	setTestAdmins(t, 1)
	notices.reset()

	tb := initReportDB(t)
	tb.liveNotice(1, "outbound_down:a", "A is DOWN")
	key := noticeKey{chatID: 1, kind: "outbound_down:a"}
	if _, ok := notices.get(key); !ok {
		t.Fatal("the live card was not remembered")
	}
	tb.dropNoticeFor(1, 77)
	if _, ok := notices.get(key); ok {
		t.Error("a hidden notice is still remembered; the next event would edit a deleted message")
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var _ = io.Discard
