package tgbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mymmrac/telego"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
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
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))

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

func TestNotifyExhaustedLooksUpEachIDOnce(t *testing.T) {
	for _, outcome := range []string{"empty", "failure", "exhausted", "expiry", "admin", "zero", "disabled-inbound"} {
		t.Run(outcome, func(t *testing.T) {
			tb := initReportDB(t)
			clientInfoLocalizer(t)
			mock, sent := captureReportServer(t)
			swapTestBot(t, mock.URL)
			t.Cleanup(mock.Close)
			clients := []model.Client{
				{Email: "notify-a", ID: "uuid-a", TgID: 6886, Enable: true},
				{Email: "notify-b", ID: "uuid-b", TgID: 6886, Enable: true},
			}
			if outcome == "zero" {
				for i := range clients {
					clients[i].TgID = 0
				}
			}
			if outcome == "admin" {
				tgBotMutex.Lock()
				previous := adminIds
				adminIds = append(append([]int64(nil), adminIds...), 6886)
				tgBotMutex.Unlock()
				t.Cleanup(func() {
					tgBotMutex.Lock()
					adminIds = previous
					tgBotMutex.Unlock()
				})
			}
			db := database.GetDB()
			for i := range 2 {
				inbound := &model.Inbound{UserId: 1, Tag: fmt.Sprintf("notify-%d", i), Port: 41890 + i, Enable: true, Protocol: model.VLESS, Settings: `{"clients":[{"email":"notify-a","id":"uuid-a","enable":true,"tgId":6886},{"email":"notify-b","id":"uuid-b","enable":true,"tgId":6886}]}`}
				if outcome == "disabled-inbound" {
					inbound.Enable = false
				}
				if err := db.Create(inbound).Error; err != nil {
					t.Fatal(err)
				}
				if err := (&service.ClientService{}).SyncInbound(nil, inbound.Id, clients); err != nil {
					t.Fatal(err)
				}
				// Keep the inbound eligible even when the Telegram account has no traffic.
				traffic := &xray.ClientTraffic{Email: fmt.Sprintf("unrelated-%d", i), InboundId: inbound.Id}
				if outcome == "exhausted" || outcome == "expiry" {
					traffic.Email = clients[i].Email
					traffic.Enable = true
					traffic.Total = 1
					traffic.Up = 2
					if outcome == "expiry" {
						traffic.Total = 0
						traffic.ExpiryTime = time.Now().Add(-time.Hour).UnixMilli()
					}
				}
				if err := db.Create(traffic).Error; err != nil {
					t.Fatal(err)
				}
			}
			lookups := 0
			cb := "test:notify_lookup"
			if err := db.Callback().Query().Before("gorm:query").Register(cb, func(tx *gorm.DB) {
				if tx.Statement.Table == "clients" && strings.Contains(fmt.Sprint(tx.Statement.Clauses["WHERE"]), "tg_id") {
					lookups++
					if outcome == "failure" {
						tx.AddError(errors.New("injected Telegram lookup failure"))
					}
				}
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Callback().Query().Remove(cb) })
			tb.notifyExhausted()
			wantLookups := 1
			if outcome == "admin" || outcome == "zero" || outcome == "disabled-inbound" {
				wantLookups = 0
			}
			if lookups != wantLookups {
				t.Fatalf("lookups = %d, want %d after %s", lookups, wantLookups, outcome)
			}
			got := sent()
			if outcome == "exhausted" || outcome == "expiry" {
				if len(got) != 1 || !strings.Contains(got[0].Text, "notify-a") || !strings.Contains(got[0].Text, "notify-b") {
					t.Fatalf("want one notification containing both clients, got %+v", got)
				}
			} else if len(got) != 0 {
				t.Fatalf("unexpected notifications: %+v", got)
			}
		})
	}
}
