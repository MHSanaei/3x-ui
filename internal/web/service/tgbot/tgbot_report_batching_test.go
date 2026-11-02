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

	"github.com/mymmrac/telego"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
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
		UserId: 1,
		// The tag is unique per inbound, so a test may seed more than one.
		Tag:      "tag-" + remark,
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
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))

	origRunning := isRunning
	t.Cleanup(func() { isRunning = origRunning })
	isRunning = true
	return &Tgbot{}
}

type sentMessage struct {
	Text        string          `json:"text"`
	Caption     string          `json:"caption"`
	ReplyMarkup json.RawMessage `json:"reply_markup"`
}

// body is what Telegram would show, whichever call carried it.
func (m sentMessage) body() string {
	if m.Text != "" {
		return m.Text
	}
	return m.Caption
}

// captureReportServer records every message-shaped call (sendMessage and the screen's sendPhoto), so a
// test asserts on what Telegram would receive, not just how many calls were made.
func captureReportServer(t *testing.T) (*httptest.Server, func() []sentMessage) {
	t.Helper()
	var mu sync.Mutex
	var sent []sentMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		result := any(true)
		path := r.URL.Path
		if path == "/bot"+testBotToken+"/sendMessage" || path == "/bot"+testBotToken+"/sendPhoto" {
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
	mock, sent := captureReportServer(t)
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

	if n := len(sent()); n != 1 {
		t.Errorf("messages = %d, want 1: one report per tap, not one per client", n)
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
	// Two messages: the report and the menu that replaces the confirmation, which used to stay on screen
	// with only the button just pressed.
	if len(got) != 2 {
		t.Fatalf("messages = %d, want 2: the report and the menu that replaces the confirmation", len(got))
	}
	if got[0].body() == "" {
		t.Error("reset report text is empty, want the finish-process message")
	}
	// The bot keeps no reply keyboards any more: every answer carries the hide
	// button instead, so the user can clear it with one tap.
	if !strings.Contains(string(got[0].ReplyMarkup), `"hide"`) {
		t.Errorf("reply_markup = %s, want the hide button", got[0].ReplyMarkup)
	}
}

// Every bot message is HTML-parsed, and a node chooses the remark and e-mails the master
// stores for its inbound: filled into a template unescaped, they became markup.
func TestClientInfoEscapesNodeValues(t *testing.T) {
	tb := initReportDB(t)
	draftLocalizer(t, &i18n.Message{ID: "tgbot.messages.email", Other: "📧 Email: {{ .Email }}\r\n"})
	remark := `<a href="https://evil.example/">Panel session expired</a>`
	email := "<b>zz</b>"
	seedReportClients(t, remark, []string{email})
	var traffic xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&traffic).Error; err != nil {
		t.Fatalf("read traffic: %v", err)
	}

	out := tb.clientInfoMsg(&traffic, true, false, false, true, true, false)

	for _, raw := range []string{"<a href=", "<b>zz</b>"} {
		if strings.Contains(out, raw) {
			t.Errorf("client info = %q, want %q escaped", out, raw)
		}
	}
	for _, want := range []string{"📧 Email: &lt;b&gt;zz&lt;/b&gt;", "&lt;a href="} {
		if !strings.Contains(out, want) {
			t.Errorf("client info = %q, want it to contain %q", out, want)
		}
	}
}
