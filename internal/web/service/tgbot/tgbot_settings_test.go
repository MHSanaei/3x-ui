package tgbot

import (
	"slices"
	"strings"
	"testing"

	"github.com/mymmrac/telego"
)

func callbackData(markup *telego.InlineKeyboardMarkup) []string {
	var data []string
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			data = append(data, button.CallbackData)
		}
	}
	return data
}

func buttonLabels(markup *telego.InlineKeyboardMarkup) []string {
	var labels []string
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			labels = append(labels, button.Text)
		}
	}
	return labels
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// The customer keyboard is the whole navigable surface for a level-1 user, so
// what it does and does not offer is the feature.
func TestClientKeyboardButtons(t *testing.T) {
	initLangDB(t)
	tg := &Tgbot{}
	markup := tg.clientKeyboard(levelClient)
	data := callbackData(markup)

	for _, want := range []string{"client_configs", "client_traffic", "client_help", "client_pm", "client_settings"} {
		if !contains(data, want) {
			t.Fatalf("client keyboard is missing %q: %v", want, data)
		}
	}
	// Every other customer action hangs off My Configs or Help now. A button that
	// climbs back to the top level is what made the old keyboard unreadable.
	for _, moved := range []string{
		"client_sub_links", "client_individual_links", "client_qr_links",
		"client_guide", "client_commands", "client_reset_self",
	} {
		if contains(data, moved) {
			t.Fatalf("%q belongs in a submenu but is on the main keyboard: %v", moved, data)
		}
	}
	if contains(data, "admin_panel") {
		t.Fatalf("a customer keyboard hints at the admin panel: %v", data)
	}

	// 2-1-2, with Help alone in the middle.
	rows := make([]int, 0, len(markup.InlineKeyboard))
	for _, row := range markup.InlineKeyboard {
		rows = append(rows, len(row))
	}
	if want := []int{2, 1, 2}; !slices.Equal(rows, want) {
		t.Fatalf("client keyboard rows are %v, want %v", rows, want)
	}
}

func TestClientKeyboardShowsAdminPanelToAdmins(t *testing.T) {
	initLangDB(t)
	tg := &Tgbot{}
	if data := callbackData(tg.clientKeyboard(levelAdmin)); !contains(data, "admin_panel") {
		t.Fatalf("admin keyboard lost the admin panel: %v", data)
	}
}

// A picker that does not say which language is active reads as if nothing was
// saved, since the labels are in their own language either way.
func TestLanguageKeyboardMarksCurrent(t *testing.T) {
	tg := &Tgbot{}
	markup := tg.languageKeyboard("fa-IR")

	data := callbackData(markup)
	for _, lang := range botLanguages {
		if !contains(data, "settings_setlang "+lang.tag) {
			t.Fatalf("picker is missing %q: %v", lang.tag, data)
		}
	}
	if !contains(data, "client_settings") {
		t.Fatalf("picker has no way back: %v", data)
	}

	var marked []string
	for _, label := range buttonLabels(markup) {
		if strings.HasPrefix(label, languageMark) {
			marked = append(marked, label)
		}
	}
	if len(marked) != 1 {
		t.Fatalf("want exactly one marked language, got %v", marked)
	}
	if !strings.Contains(marked[0], "فارسی") {
		t.Fatalf("marked the wrong language: %q", marked[0])
	}
}

// With nothing chosen the bot answers in the panel language, so that is what
// the picker must show as active rather than nothing at all.
func TestLanguageKeyboardWithNoChoiceMarksNothing(t *testing.T) {
	tg := &Tgbot{}
	var marked int
	for _, label := range buttonLabels(tg.languageKeyboard("")) {
		if strings.HasPrefix(label, languageMark) {
			marked++
		}
	}
	if marked != 0 {
		t.Fatalf("want no language marked when none is chosen, got %d", marked)
	}
}

// The tag arrives as attacker-controlled callback data, so the parser is the
// gate that keeps an arbitrary string out of the language store.
func TestParseLangCallback(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
		ok   bool
	}{
		{name: "supported tag", data: "settings_setlang fa-IR", want: "fa-IR", ok: true},
		{name: "another supported tag", data: "settings_setlang pt-BR", want: "pt-BR", ok: true},
		{name: "unsupported tag", data: "settings_setlang kl-GL", ok: false},
		{name: "no tag", data: "settings_setlang", ok: false},
		{name: "empty tag", data: "settings_setlang ", ok: false},
		{name: "path traversal", data: "settings_setlang ../../etc/passwd", ok: false},
		{name: "different verb", data: "settings_lang fa-IR", ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseLangCallback(tc.data)
			if ok != tc.ok {
				t.Fatalf("parseLangCallback(%q) ok = %v, want %v", tc.data, ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("parseLangCallback(%q) = %q, want %q", tc.data, got, tc.want)
			}
		})
	}
}
