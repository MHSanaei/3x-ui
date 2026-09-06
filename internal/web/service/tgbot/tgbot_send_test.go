package tgbot

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

func TestIsTelegramNotModifiedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"not modified", errors.New("Bad Request: message is not modified"), true},
		{"No fields to modify", errors.New("Bad Request: No fields to modify"), true},
		{"unrelated error", errors.New("Bad Request: message to edit not found"), false},
		{"network error", errors.New("connection reset"), false},
		{"empty string", errors.New(""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTelegramNotModifiedError(tt.err)
			if got != tt.want {
				t.Errorf("isTelegramNotModifiedError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestEditMessageTgBotSkipsNotModified(t *testing.T) {
	// Mock Telegram API that always returns "message is not modified".
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":          false,
			"error_code":  400,
			"description": "Bad Request: message is not modified: specified new message content and reply markup are exactly the same as a current content and reply markup of the message.",
		})
	}))
	defer mock.Close()

	// Point the package-level bot at the mock.
	origBot := bot
	t.Cleanup(func() { bot = origBot })
	var err error
	bot, err = telego.NewBot("test-token", telego.WithAPIServer(mock.URL))
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}

	// Snapshot warning count before the edit call.
	before := logger.GetLogs(100, "warning")

	tb := &Tgbot{}
	tb.editMessageTgBot(123, 456, "<b>hello</b>")

	after := logger.GetLogs(100, "warning")
	if len(after) > len(before) {
		t.Errorf("editMessageTgBot logged %d new warnings, want 0; new entries: %v",
			len(after)-len(before), after[len(before):])
	}
}

func TestEditMessageCallbackTgBotSkipsNotModified(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":          false,
			"error_code":  400,
			"description": "Bad Request: message is not modified",
		})
	}))
	defer mock.Close()

	origBot := bot
	t.Cleanup(func() { bot = origBot })
	var err error
	bot, err = telego.NewBot("test-token", telego.WithAPIServer(mock.URL))
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}

	before := logger.GetLogs(100, "warning")

	tb := &Tgbot{}
	kb := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("btn").WithCallbackData("test"),
	))
	tb.editMessageCallbackTgBot(123, 456, kb)

	after := logger.GetLogs(100, "warning")
	if len(after) > len(before) {
		t.Errorf("editMessageCallbackTgBot logged %d new warnings, want 0; new entries: %v",
			len(after)-len(before), after[len(before):])
	}
}

func TestPageMessageSplitsLinkListWithoutBlankLines(t *testing.T) {
	var message strings.Builder
	message.WriteString("Individual links:\r\n")
	for range 50 {
		message.WriteString("<code>vless://" + strings.Repeat("a", 300) + "</code>\r\n")
	}

	pages := pageMessage(message.String(), telegramPageLimit)
	if len(pages) < 2 {
		t.Fatalf("pageMessage() returned %d page, want multiple", len(pages))
	}

	links := 0
	for index, page := range pages {
		if len(page) > telegramPageLimit {
			t.Errorf("page %d has %d bytes, want at most %d", index, len(page), telegramPageLimit)
		}
		openingTags := strings.Count(page, "<code>")
		closingTags := strings.Count(page, "</code>")
		if openingTags != closingTags {
			t.Errorf("page %d has %d opening tags and %d closing tags", index, openingTags, closingTags)
		}
		links += openingTags
	}
	if links != 50 {
		t.Errorf("pages contain %d links, want 50", links)
	}
}
