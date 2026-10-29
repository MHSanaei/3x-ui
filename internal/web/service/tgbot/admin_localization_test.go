package tgbot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func botLocaleStrings(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	var visit func(string, map[string]any)
	visit = func(prefix string, object map[string]any) {
		for key, value := range object {
			switch v := value.(type) {
			case string:
				out[prefix+key] = v
			case map[string]any:
				visit(prefix+key+".", v)
			}
		}
	}
	visit("", root["tgbot"].(map[string]any))
	return out
}

func TestBotTranslationsPreserveTemplateParameters(t *testing.T) {
	root := filepath.Join(repoRootForTest(t), "internal/web/translation")
	en := botLocaleStrings(t, filepath.Join(root, "en-US.json"))
	paths, err := filepath.Glob(filepath.Join(root, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	parameter := regexp.MustCompile(`\.[A-Za-z][A-Za-z0-9_]*`)
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			values := botLocaleStrings(t, path)
			for key, source := range en {
				translation, ok := values[key]
				if !ok || translation == "" {
					t.Errorf("missing bot translation: %s", key)
					continue
				}
				want, got := parameter.FindAllString(source, -1), parameter.FindAllString(translation, -1)
				slices.Sort(want)
				slices.Sort(got)
				want, got = slices.Compact(want), slices.Compact(got)
				if !reflect.DeepEqual(want, got) {
					t.Errorf("%s parameters: got %v, want %v", key, got, want)
				}
			}
		})
	}
}

func TestPersianClientDraftAndNavigationAreLocalizedAndEscaped(t *testing.T) {
	localizeWithRealBundle(t, "fa-IR")
	tb := &Tgbot{}
	body := tb.BuildClientDraftMessage(&clientDraft{email: "علی<&>", comment: "یادداشت<&>", enable: true})
	for _, want := range []string{"پیش\u200cنویس کاربر جدید", "علی&lt;&amp;&gt;", "یادداشت&lt;&amp;&gt;"} {
		if !strings.Contains(body, want) {
			t.Errorf("draft missing %q: %s", want, body)
		}
	}
	for _, forbidden := range []string{"New client", "Unlimited(Reset)", "Email:", "Attached:", "{{"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("draft contains untranslated/broken text %q: %s", forbidden, body)
		}
	}
	for _, row := range tb.homeRows() {
		for _, button := range row {
			if button.Text == "" || strings.Contains(button.Text, "tgbot.") {
				t.Fatalf("invalid home label: %+v", button)
			}
		}
	}
	for _, key := range []string{"tgbot.buttons.back", "tgbot.buttons.searchInbounds", "tgbot.buttons.individualLinks", "tgbot.inline.offBody"} {
		if value := tb.I18nBot(key); regexp.MustCompile(`[A-Za-z]{4}`).MatchString(strings.ReplaceAll(strings.ReplaceAll(value, "@BotFather", ""), "/setinline", "")) {
			t.Errorf("untranslated Persian label %s: %s", key, value)
		}
	}
}
