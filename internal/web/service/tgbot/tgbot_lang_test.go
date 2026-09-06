package tgbot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

func initLangDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

// The blob is written by us but read back from a database an operator can edit,
// so a damaged value must degrade to "nobody picked a language".
func TestParseUserLangs(t *testing.T) {
	tests := []struct {
		name string
		blob string
		want map[int64]string
	}{
		{name: "empty string", blob: "", want: map[int64]string{}},
		{name: "empty object", blob: "{}", want: map[int64]string{}},
		{name: "one user", blob: `{"42":"fa-IR"}`, want: map[int64]string{42: "fa-IR"}},
		{name: "several users", blob: `{"42":"fa-IR","7":"ru-RU"}`, want: map[int64]string{42: "fa-IR", 7: "ru-RU"}},
		{name: "malformed json", blob: `{"42":`, want: map[int64]string{}},
		{name: "unsupported tag is dropped", blob: `{"42":"kl-GL"}`, want: map[int64]string{}},
		{name: "non-numeric key is dropped", blob: `{"someone":"fa-IR"}`, want: map[int64]string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseUserLangs(tc.blob)
			if len(got) != len(tc.want) {
				t.Fatalf("parseUserLangs(%q) = %v, want %v", tc.blob, got, tc.want)
			}
			for id, lang := range tc.want {
				if got[id] != lang {
					t.Fatalf("parseUserLangs(%q)[%d] = %q, want %q", tc.blob, id, got[id], lang)
				}
			}
		})
	}
}

// A language tag arrives from a callback payload, so an unsupported one must be
// refused rather than stored and later handed to the localizer.
func TestIsSupportedLang(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{"en-US", true},
		{"fa-IR", true},
		{"pt-BR", true},
		{"", false},
		{"kl-GL", false},
		{"en", false},
		{"../../etc/passwd", false},
	}

	for _, tc := range tests {
		t.Run(tc.tag, func(t *testing.T) {
			if got := isSupportedLang(tc.tag); got != tc.want {
				t.Fatalf("isSupportedLang(%q) = %v, want %v", tc.tag, got, tc.want)
			}
		})
	}
}

// Every tag offered in the picker must have a translation file behind it, or the
// user picks a language and the bot keeps answering in the panel's.
func TestBotLanguagesHaveTranslations(t *testing.T) {
	for _, lang := range botLanguages {
		path := filepath.Join("..", "..", "translation", lang.tag+".json")
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("no translation file for %q: %v", lang.tag, err)
		}
	}
}

func TestUserLangRoundTrip(t *testing.T) {
	initLangDB(t)
	bot := new(Tgbot)

	if got := bot.langOf(42); got != "" {
		t.Fatalf("langOf before any choice = %q, want the panel default", got)
	}

	if err := bot.setUserLang(42, "fa-IR"); err != nil {
		t.Fatalf("setUserLang: %v", err)
	}
	if got := bot.langOf(42); got != "fa-IR" {
		t.Fatalf("langOf = %q, want fa-IR", got)
	}
	if got := bot.langOf(7); got != "" {
		t.Fatalf("langOf for another user = %q, want the panel default", got)
	}

	if err := bot.setUserLang(7, "ru-RU"); err != nil {
		t.Fatalf("setUserLang: %v", err)
	}
	if got := bot.langOf(42); got != "fa-IR" {
		t.Fatalf("second user's choice overwrote the first: langOf(42) = %q", got)
	}
}

func TestSetUserLangRejectsUnsupported(t *testing.T) {
	initLangDB(t)
	bot := new(Tgbot)

	if err := bot.setUserLang(42, "kl-GL"); err == nil {
		t.Fatal("setUserLang accepted an unsupported tag")
	}
	if got := bot.langOf(42); got != "" {
		t.Fatalf("langOf after a refused set = %q, want empty", got)
	}
}

// forUser is what carries a language into 601 untouched I18nBot call sites, so
// it must not disturb the receiver every other caller shares.
func TestForUserLeavesReceiverAlone(t *testing.T) {
	initLangDB(t)
	bot := new(Tgbot)
	if err := bot.setUserLang(42, "fa-IR"); err != nil {
		t.Fatalf("setUserLang: %v", err)
	}

	scoped := bot.forUser(42)
	if scoped.lang != "fa-IR" {
		t.Fatalf("forUser(42).lang = %q, want fa-IR", scoped.lang)
	}
	if bot.lang != "" {
		t.Fatalf("forUser mutated the shared receiver: lang = %q", bot.lang)
	}
	if scoped == bot {
		t.Fatal("forUser returned the shared receiver instead of a copy")
	}

	if plain := bot.forUser(7); plain.lang != "" {
		t.Fatalf("forUser for a user with no choice = %q, want empty", plain.lang)
	}
}
