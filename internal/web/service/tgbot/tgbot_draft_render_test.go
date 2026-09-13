package tgbot

import (
	"encoding/json"
	"html"
	"io"
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

// Regression test: the draft is sent with ParseMode HTML, so Markdown markers
// were rendered literally and an unescaped value could break the whole message.
func TestClientDraftMessageRendersHTML(t *testing.T) {
	origEmail, origComment, origTgID := client_Email, client_Comment, client_TgID
	origTotalGB, origLimitIP, origExpiry := client_TotalGB, client_LimitIP, client_ExpiryTime
	origInboundIDs := receiver_inbound_IDs
	t.Cleanup(func() {
		client_Email, client_Comment, client_TgID = origEmail, origComment, origTgID
		client_TotalGB, client_LimitIP, client_ExpiryTime = origTotalGB, origLimitIP, origExpiry
		receiver_inbound_IDs = origInboundIDs
	})

	client_Email = "a@b.c"
	client_Comment = "<b>promo</b> & <10 GB>"
	client_TgID = "42"
	client_TotalGB, client_LimitIP, client_ExpiryTime = 0, 0, 0
	receiver_inbound_IDs = nil

	out := (&Tgbot{}).BuildClientDraftMessage()

	if !strings.Contains(out, "<b>New client draft</b>") {
		t.Errorf("draft title is not HTML markup: %q", out)
	}
	if strings.Contains(out, "*New client draft*") || strings.Contains(out, "`") {
		t.Errorf("draft still carries Markdown markers: %q", out)
	}
	if strings.Contains(out, "<b>promo</b>") {
		t.Errorf("raw comment markup reached the message: %q", out)
	}
	if !strings.Contains(out, html.EscapeString(client_Comment)) {
		t.Errorf("comment is not HTML-escaped: %q", out)
	}
}

// botPromptLocalizer renders the two prompts the callback tests drive, with the
// templates the translation files carry; without it I18n returns the bare key.
func botPromptLocalizer(t *testing.T) {
	t.Helper()
	bundle := i18n.NewBundle(language.MustParse("en-US"))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	_ = bundle.AddMessages(language.MustParse("en-US"),
		&i18n.Message{ID: "tgbot.messages.email_prompt", Other: "📧 Default Email: {{ .ClientEmail }}\n\nEnter your email."},
		&i18n.Message{ID: "tgbot.messages.comment_prompt", Other: "💬 Default Comment: {{ .ClientComment }}\n\nEnter your comment."},
	)
	orig := locale.LocalizerBot
	t.Cleanup(func() { locale.LocalizerBot = orig })
	locale.LocalizerBot = i18n.NewLocalizer(bundle, "en-US")
}

// promptTexts serves the methods these prompts touch and returns the text of
// every sendMessage, so a test can check what Telegram would actually parse.
func promptTexts(t *testing.T) (string, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var texts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		result := any(true)
		if r.URL.Path == "/bot"+testBotToken+"/sendMessage" {
			var payload struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(body, &payload)
			mu.Lock()
			texts = append(texts, payload.Text)
			mu.Unlock()
			result = map[string]any{"message_id": 1, "date": 0, "chat": map[string]any{"id": 1, "type": "private"}}
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

// Regression test: the wizard's own prompts are HTML-parsed as well, so the
// draft value they echo has to be escaped exactly like the draft card.
func TestAddClientPromptsEscapeDraftValues(t *testing.T) {
	botPromptLocalizer(t)
	url, texts := promptTexts(t)
	swapTestBot(t, url)

	origEmail, origComment := client_Email, client_Comment
	origRunning := isRunning
	t.Cleanup(func() {
		client_Email, client_Comment = origEmail, origComment
		isRunning = origRunning
	})
	isRunning = true

	cases := []struct {
		name  string
		data  string
		value string
	}{
		{"email prompt", "add_client_ch_default_email", "long<name>@example.com"},
		{"comment prompt", "add_client_ch_default_comment", "promo <b>tag</b>"},
	}
	tb := &Tgbot{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client_Email, client_Comment = tc.value, tc.value

			tb.answerCallback(&telego.CallbackQuery{
				ID:      "q1",
				From:    telego.User{ID: 1},
				Data:    tc.data,
				Message: &telego.Message{Chat: telego.Chat{ID: 1}},
			}, true) // admin

			sent := texts()
			if len(sent) == 0 {
				t.Fatalf("no prompt was sent for %s", tc.data)
			}
			got := sent[len(sent)-1]
			if strings.Contains(got, tc.value) {
				t.Errorf("prompt = %q, want the draft value escaped", got)
			}
			if !strings.Contains(got, html.EscapeString(tc.value)) {
				t.Errorf("prompt = %q, want it to contain %q", got, html.EscapeString(tc.value))
			}
		})
	}
}
