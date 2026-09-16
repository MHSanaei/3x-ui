package tgbot

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// clientInfoLocalizer renders the lines clientInfoMsg prints with the templates
// the translation files carry; without it I18n returns the bare keys.
func clientInfoLocalizer(t *testing.T) {
	t.Helper()
	bundle := i18n.NewBundle(language.MustParse("en-US"))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	_ = bundle.AddMessages(language.MustParse("en-US"),
		&i18n.Message{ID: "tgbot.messages.email", Other: "Email: {{ .Email }}\r\n"},
		&i18n.Message{ID: "tgbot.days", Other: "Days"},
		&i18n.Message{ID: "tgbot.messages.expireIn", Other: "Expire In: {{ .Time }}\r\n"},
		&i18n.Message{ID: "tgbot.messages.expire", Other: "Expire Date: {{ .Time }}\r\n"},
		&i18n.Message{ID: "tgbot.wentWrong", Other: "went wrong"},
	)
	orig := locale.LocalizerBot
	t.Cleanup(func() { locale.LocalizerBot = orig })
	locale.LocalizerBot = i18n.NewLocalizer(bundle, "en-US")
}

// Regression test: a start-after-first-use client is stored as a negative duration,
// and a disabled one rendered it as a 1969 date.
func TestClientInfoShowsStartAfterFirstUseWhenDisabled(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	clientInfoLocalizer(t)

	traffic := &xray.ClientTraffic{
		Email:      "trial@example.com",
		Enable:     false,
		ExpiryTime: -30 * 24 * 60 * 60000,
	}

	out := (&Tgbot{}).clientInfoMsg(traffic, false, false, false, true, false, false)

	if strings.Contains(out, "1969") {
		t.Errorf("client info = %q, want the days left, not a 1969 date", out)
	}
	if !strings.Contains(out, "Expire In: 30 Days") {
		t.Errorf("client info = %q, want it to contain %q", out, "Expire In: 30 Days")
	}
}
