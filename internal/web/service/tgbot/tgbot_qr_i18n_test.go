package tgbot

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// Regression test for #6562: the QR caption must go through I18nBot so it
// follows the configured bot language instead of hardcoded English.
func TestQRCodeForClientLocalizes(t *testing.T) {
	bundle := i18n.NewBundle(language.MustParse("en-US"))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	_ = bundle.AddMessages(language.MustParse("en-US"), &i18n.Message{
		ID:    "tgbot.answers.qrCodeForClient",
		Other: "QRCode for client {{ .Email }}:",
	})
	_ = bundle.AddMessages(language.MustParse("ru-RU"), &i18n.Message{
		ID:    "tgbot.answers.qrCodeForClient",
		Other: "QR-код для клиента {{ .Email }}:",
	})
	orig := locale.LocalizerBot
	t.Cleanup(func() { locale.LocalizerBot = orig })

	locale.LocalizerBot = i18n.NewLocalizer(bundle, "en-US")
	en := (&Tgbot{}).I18nBot("tgbot.answers.qrCodeForClient", "Email==alice@example.com")
	if en != "QRCode for client alice@example.com:" {
		t.Errorf("en caption = %q", en)
	}

	locale.LocalizerBot = i18n.NewLocalizer(bundle, "ru-RU")
	ru := (&Tgbot{}).I18nBot("tgbot.answers.qrCodeForClient", "Email==alice@example.com")
	if ru != "QR-код для клиента alice@example.com:" {
		t.Errorf("ru caption = %q", ru)
	}
	if strings.Contains(ru, "QRCode for client") {
		t.Errorf("ru caption still hardcoded English: %q", ru)
	}
}
