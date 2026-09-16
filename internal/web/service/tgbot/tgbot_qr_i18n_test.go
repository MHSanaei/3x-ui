package tgbot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// realCaption reads tgbot.answers.qrCodeForClient from the shipped locale
// file. It fails the test when the JSON is malformed or the key is missing,
// so broken translation files can no longer ship alongside a green suite
// (see #6564 review).
func realCaption(t *testing.T, lang string) string {
	t.Helper()
	path := filepath.Join("..", "..", "translation", lang+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read locale file: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("locale file %s is invalid JSON: %v", lang, err)
	}
	answers, ok := raw["tgbot"].(map[string]any)["answers"].(map[string]any)
	if !ok {
		t.Fatalf("locale file %s has no tgbot.answers section", lang)
	}
	msg, ok := answers["qrCodeForClient"].(string)
	if !ok || msg == "" {
		t.Fatalf("locale file %s missing tgbot.answers.qrCodeForClient", lang)
	}
	return msg
}

// Regression test for #6562: the QR caption must go through I18nBot so it
// follows the configured bot language instead of hardcoded English. Messages
// come from the real shipped files, not a synthetic bundle.
func TestQRCodeForClientLocalizes(t *testing.T) {
	enMsg := realCaption(t, "en-US")
	ruMsg := realCaption(t, "ru-RU")

	bundle := i18n.NewBundle(language.MustParse("en-US"))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	_ = bundle.AddMessages(language.MustParse("en-US"), &i18n.Message{
		ID:    "tgbot.answers.qrCodeForClient",
		Other: enMsg,
	})
	_ = bundle.AddMessages(language.MustParse("ru-RU"), &i18n.Message{
		ID:    "tgbot.answers.qrCodeForClient",
		Other: ruMsg,
	})

	orig := locale.LocalizerBot
	t.Cleanup(func() { locale.LocalizerBot = orig })

	locale.LocalizerBot = i18n.NewLocalizer(bundle, "en-US")
	en := (&Tgbot{}).I18nBot("tgbot.answers.qrCodeForClient", "Email==alice@example.com")
	if !strings.Contains(en, "alice@example.com") {
		t.Errorf("en caption missing email: %q", en)
	}
	if en == "" || en == "tgbot.answers.qrCodeForClient" {
		t.Errorf("en caption not localized: %q", en)
	}

	locale.LocalizerBot = i18n.NewLocalizer(bundle, "ru-RU")
	ru := (&Tgbot{}).I18nBot("tgbot.answers.qrCodeForClient", "Email==alice@example.com")
	if !strings.Contains(ru, "alice@example.com") {
		t.Errorf("ru caption missing email: %q", ru)
	}
	if strings.Contains(ru, "QRCode for client") {
		t.Errorf("ru caption still hardcoded English: %q", ru)
	}
}
