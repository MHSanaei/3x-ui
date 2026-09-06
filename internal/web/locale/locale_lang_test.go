package locale

import (
	"encoding/json"
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// Rebuilds the package globals from two tiny message files so the test does not
// depend on the embedded translations staying worded the way they are today.
func initTestBundle(t *testing.T) {
	t.Helper()

	previousBundle, previousBot := i18nBundle, LocalizerBot
	t.Cleanup(func() {
		i18nBundle, LocalizerBot = previousBundle, previousBot
		localizerCacheMu.Lock()
		localizerCache = map[string]*i18n.Localizer{}
		localizerCacheMu.Unlock()
	})

	bundle := i18n.NewBundle(language.MustParse("en-US"))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	files := map[string]string{
		"en.json": `{"greet": "Hello {{ .Name }}"}`,
		"fa.json": `{"greet": "سلام {{ .Name }}"}`,
	}
	for name, body := range files {
		tag := "en-US"
		if name == "fa.json" {
			tag = "fa-IR"
		}
		if _, err := bundle.ParseMessageFileBytes([]byte(body), tag+".json"); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
	}

	i18nBundle = bundle
	LocalizerBot = i18n.NewLocalizer(bundle, "en-US")
	localizerCacheMu.Lock()
	localizerCache = map[string]*i18n.Localizer{}
	localizerCacheMu.Unlock()
}

func TestI18nLang(t *testing.T) {
	initTestBundle(t)

	tests := []struct {
		name string
		lang string
		want string
	}{
		{name: "explicit language wins", lang: "fa-IR", want: "سلام Amy"},
		{name: "empty falls back to the panel language", lang: "", want: "Hello Amy"},
		{name: "unknown language falls back to the bundle default", lang: "kl-GL", want: "Hello Amy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := I18nLang(tt.lang, "greet", "Name==Amy"); got != tt.want {
				t.Fatalf("I18nLang(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

// A second call for the same language must reuse the localizer rather than
// rebuild it, since every Telegram update resolves one.
func TestI18nLangCachesLocalizers(t *testing.T) {
	initTestBundle(t)

	I18nLang("fa-IR", "greet", "Name==Amy")
	localizerCacheMu.RLock()
	first, ok := localizerCache["fa-IR"]
	localizerCacheMu.RUnlock()
	if !ok {
		t.Fatal("fa-IR localizer was not cached")
	}

	I18nLang("fa-IR", "greet", "Name==Amy")
	localizerCacheMu.RLock()
	second := localizerCache["fa-IR"]
	localizerCacheMu.RUnlock()
	if first != second {
		t.Fatal("localizer was rebuilt instead of reused")
	}
}

// A nil bundle is the sub-server's start-up state; localizing must not panic.
func TestI18nLangWithoutBundle(t *testing.T) {
	previousBundle, previousBot := i18nBundle, LocalizerBot
	t.Cleanup(func() { i18nBundle, LocalizerBot = previousBundle, previousBot })
	i18nBundle, LocalizerBot = nil, nil

	if got := I18nLang("fa-IR", "greet"); got != "greet" {
		t.Fatalf("I18nLang without a bundle = %q, want the key back", got)
	}
}
