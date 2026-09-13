package tgbot

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"

	"github.com/mymmrac/telego"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// expiryCardLocalizer renders the two labels the draft card prints around its
// expiry; without it I18n returns the bare keys.
func expiryCardLocalizer(t *testing.T) {
	t.Helper()
	bundle := i18n.NewBundle(language.MustParse("en-US"))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	_ = bundle.AddMessages(language.MustParse("en-US"),
		&i18n.Message{ID: "tgbot.days", Other: "Days"},
		&i18n.Message{ID: "tgbot.unlimited", Other: "Unlimited"},
	)
	orig := locale.LocalizerBot
	t.Cleanup(func() { locale.LocalizerBot = orig })
	locale.LocalizerBot = i18n.NewLocalizer(bundle, "en-US")
}

// expiryCardTexts serves the calls one preset tap makes and returns the text of
// every message Telegram would have received.
func expiryCardTexts(t *testing.T) (string, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var texts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := any(true)
		if r.URL.Path == "/bot"+testBotToken+"/editMessageText" || r.URL.Path == "/bot"+testBotToken+"/sendMessage" {
			var payload struct {
				Text string `json:"text"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			mu.Lock()
			texts = append(texts, payload.Text)
			mu.Unlock()
			result = map[string]any{"message_id": 7, "date": 0, "chat": map[string]any{"id": 1, "type": "private"}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
	}))
	t.Cleanup(srv.Close)

	return srv.URL, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), texts...)
	}
}

// Pins the decision the dead accumulate branch hid: a preset tap chooses the term
// instead of adding to it, and 0 is Unlimited. The custom keypad shares this case.
func TestAddClientExpiryPresetReplacesTheTerm(t *testing.T) {
	expiryCardLocalizer(t)
	url, texts := expiryCardTexts(t)
	swapTestBot(t, url)

	// The card lists attached inbounds by remark, which would reach the database;
	// no attach step runs here, so keep that list empty whichever test ran before.
	origInbounds := receiver_inbound_IDs
	t.Cleanup(func() { receiver_inbound_IDs = origInbounds })
	receiver_inbound_IDs = nil

	origRunning := isRunning
	t.Cleanup(func() { isRunning = origRunning })
	isRunning = true

	tb := &Tgbot{}
	for _, tc := range []struct {
		days string
		want string
	}{
		{"30", "Expire: 30 Days"},
		{"90", "Expire: 90 Days"},  // not 120: the second tap replaces the first
		{"0", "Expire: Unlimited"}, // the Unlimited button clears the term
	} {
		t.Run(tc.days, func(t *testing.T) {
			tb.answerCallback(&telego.CallbackQuery{
				ID:      "q1",
				From:    telego.User{ID: 1},
				Data:    "add_client_reset_exp_c " + tc.days,
				Message: &telego.Message{MessageID: 7, Chat: telego.Chat{ID: 1}},
			}, true) // admin

			sent := texts()
			if len(sent) == 0 {
				t.Fatalf("add_client_reset_exp_c %s rendered no card", tc.days)
			}
			if got := sent[len(sent)-1]; !strings.Contains(got, tc.want) {
				t.Errorf("card after add_client_reset_exp_c %s = %q, want it to contain %q", tc.days, got, tc.want)
			}
		})
	}
}
