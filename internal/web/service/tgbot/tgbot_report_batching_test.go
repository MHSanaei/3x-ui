package tgbot

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/mymmrac/telego"
)

// seedReportClients writes one inbound holding every email plus the traffic row
// each of them needs to appear in the sorted usage report.
func seedReportClients(t *testing.T, remark string, emails []string) {
	t.Helper()
	settings := make([]string, 0, len(emails))
	for _, email := range emails {
		settings = append(settings, fmt.Sprintf(`{"email":%q,"subId":"sub-%s"}`, email, email))
	}
	inbound := &model.Inbound{
		UserId:   1,
		Remark:   remark,
		Port:     8443,
		Protocol: model.VLESS,
		Enable:   true,
		Settings: `{"clients":[` + strings.Join(settings, ",") + `]}`,
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	for _, email := range emails {
		if err := database.GetDB().Create(&xray.ClientTraffic{
			InboundId: inbound.Id,
			Email:     email,
			Enable:    true,
			Up:        1,
			Down:      1,
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
}

func initReportDB(t *testing.T) *Tgbot {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	origRunning := isRunning
	t.Cleanup(func() { isRunning = origRunning })
	isRunning = true
	return &Tgbot{}
}

type sentMessage struct {
	Text        string          `json:"text"`
	ReplyMarkup json.RawMessage `json:"reply_markup"`
}

// captureReportServer records every sendMessage call so a test can assert on
// what Telegram would have received, not merely how many calls were made.
func captureReportServer(t *testing.T) (*httptest.Server, func() []sentMessage) {
	t.Helper()
	var mu sync.Mutex
	var sent []sentMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		result := any(true)
		if r.URL.Path == "/bot"+testBotToken+"/sendMessage" {
			var payload sentMessage
			_ = json.Unmarshal(body, &payload)
			mu.Lock()
			sent = append(sent, payload)
			mu.Unlock()
			result = map[string]any{"message_id": 1, "date": 0, "chat": map[string]any{"id": 1, "type": "private"}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
	}))
	return srv, func() []sentMessage {
		mu.Lock()
		defer mu.Unlock()
		return append([]sentMessage(nil), sent...)
	}
}

// Regression test: the sorted usage report must reach Telegram as one message
// whatever the client count; per-client sends burst past the rate limit.
func TestTrafficUsageReportIsOneMessage(t *testing.T) {
	mock, calls := staleButtonServer(t, map[string]any{
		"sendMessage": map[string]any{"ok": true, "result": map[string]any{
			"message_id": 1,
			"date":       0,
			"chat":       map[string]any{"id": 1, "type": "private"},
		}},
		"deleteMessage": map[string]any{"ok": true, "result": true},
	})
	swapTestBot(t, mock.URL)
	defer mock.Close()

	tb := initReportDB(t)
	seedReportClients(t, "report", []string{"a@x", "b@x", "c@x"})

	tb.answerCallback(&telego.CallbackQuery{
		ID:      "q1",
		From:    telego.User{ID: 1},
		Data:    "get_sorted_traffic_usage_report",
		Message: &telego.Message{Chat: telego.Chat{ID: 1}},
	}, true) // admin

	if n := calls("sendMessage"); n != 1 {
		t.Errorf("sendMessage calls = %d, want 1: one report per tap, not one per client", n)
	}
}

// Regression test: batching must not swallow the reply on a panel with no
// clients, where the old code still answered FinishProcess.
func TestResetAllTrafficsAnswersWithNoClients(t *testing.T) {
	mock, sent := captureReportServer(t)
	swapTestBot(t, mock.URL)
	defer mock.Close()

	tb := initReportDB(t)

	tb.answerCallback(&telego.CallbackQuery{
		ID:      "q1",
		From:    telego.User{ID: 1},
		Data:    "reset_all_traffics_c",
		Message: &telego.Message{Chat: telego.Chat{ID: 1}},
	}, true) // admin

	got := sent()
	if len(got) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1: an empty panel must still answer the tap", len(got))
	}
	if got[0].Text == "" {
		t.Error("reset report text is empty, want the finish-process message")
	}
	if !strings.Contains(string(got[0].ReplyMarkup), `"remove_keyboard":true`) {
		t.Errorf("reply_markup = %s, want the reply keyboard removed", got[0].ReplyMarkup)
	}
}

// Regression test: the report leaves as one HTML-parsed message, so a remark
// holding "<" must reach Telegram escaped instead of dropping the whole page.
func TestTrafficUsageReportEscapesHtml(t *testing.T) {
	mock, sent := captureReportServer(t)
	swapTestBot(t, mock.URL)
	defer mock.Close()

	tb := initReportDB(t)
	seedReportClients(t, "DE <fast>", []string{"a@x"})

	tb.answerCallback(&telego.CallbackQuery{
		ID:      "q1",
		From:    telego.User{ID: 1},
		Data:    "get_sorted_traffic_usage_report",
		Message: &telego.Message{Chat: telego.Chat{ID: 1}},
	}, true) // admin

	got := sent()
	if len(got) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(got))
	}
	if strings.Contains(got[0].Text, "<fast>") {
		t.Errorf("report text = %q, want the remark escaped", got[0].Text)
	}
	if !strings.Contains(got[0].Text, "&lt;fast&gt;") {
		t.Errorf("report text = %q, want the remark escaped as &lt;fast&gt;", got[0].Text)
	}
}
