package locale

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func TestSlovakCatalogLoadsWithEveryLocale(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	if err := parseTranslationFiles(os.DirFS(".."), bundle); err != nil {
		t.Fatalf("load panel catalogs: %v", err)
	}
	loc := i18n.NewLocalizer(bundle, "sk-SK")
	for key, want := range map[string]string{
		"menu.dashboard":           "Prehľad",
		"pages.clients.totalGB":    "Dátový limit (GB)",
		"discord.alerts.xrayCrash": "Jadro Xray spadlo",
	} {
		got, err := loc.Localize(&i18n.LocalizeConfig{MessageID: key})
		if err != nil || got != want {
			t.Errorf("%s: got %q, err %v, want %q", key, got, err, want)
		}
	}
	got, err := loc.Localize(&i18n.LocalizeConfig{
		MessageID:    "tgbot.messages.inviteBound",
		TemplateData: map[string]any{"Email": "sample-client"},
	})
	if err != nil || !strings.Contains(got, "<code>sample-client</code>") || !strings.Contains(got, "priradený") {
		t.Fatalf("Slovak bot template: %q, %v", got, err)
	}
}

func TestSlovakBotTemplatesRender(t *testing.T) {
	data, err := os.ReadFile("../translation/sk-SK.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog map[string]any
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	field := regexp.MustCompile(`\{\{\s*\.([A-Za-z][A-Za-z0-9_]*)\s*\}\}`)
	var walk func(string, any)
	walk = func(key string, value any) {
		if children, ok := value.(map[string]any); ok {
			for name, child := range children {
				walk(key+"."+name, child)
			}
			return
		}
		text := value.(string)
		tmpl, err := template.New(key).Option("missingkey=error").Parse(text)
		if err != nil {
			t.Errorf("%s: %v", key, err)
			return
		}
		params := map[string]any{}
		for _, match := range field.FindAllStringSubmatch(text, -1) {
			params[match[1]] = "sample"
		}
		var out bytes.Buffer
		if err := tmpl.Execute(&out, params); err != nil {
			t.Errorf("%s: %v", key, err)
		}
	}
	walk("tgbot", catalog["tgbot"])
	walk("discord", catalog["discord"])
}
