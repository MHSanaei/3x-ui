package tgbot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
)

func TestConntrackEventMessages(t *testing.T) {
	draftLocalizer(t,
		&i18n.Message{ID: "tgbot.messages.conntrackHigh", Other: "High {{ .Count }}/{{ .Max }} ({{ .Percent }}%, threshold {{ .Threshold }}%)"},
	)
	tb := initReportDB(t)
	for _, test := range []struct {
		name      string
		threshold int
		data      any
		want      string
	}{
		{"at configured threshold", 75, &eventbus.ConntrackData{Count: 5760, Max: 7680, Percent: 75}, "High 5760/7680 (75.0%, threshold 75%)"},
		{"above threshold", 60, &eventbus.ConntrackData{Count: 6144, Max: 7680, Percent: 80}, "High 6144/7680 (80.0%, threshold 60%)"},
		{"below threshold", 90, &eventbus.ConntrackData{Count: 6144, Max: 7680, Percent: 80}, ""},
		{"disabled", 0, &eventbus.ConntrackData{Count: 7680, Max: 7680, Percent: 100}, ""},
		{"missing data", 80, nil, ""},
		{"invalid data", 80, "invalid", ""},
		{"nil data", 80, (*eventbus.ConntrackData)(nil), ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := tb.settingService.SetTgConntrack(test.threshold); err != nil {
				t.Fatal(err)
			}
			got := tb.formatEventMessage(eventbus.Event{Type: eventbus.EventConntrackHigh, Data: test.data})
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("message = %q, want %q", got, test.want)
			}
		})
	}
}

func TestConntrackEventsRespectTelegramSelection(t *testing.T) {
	tb := initReportDB(t)
	if err := tb.settingService.SetTgEnabledEvents("conntrack.high"); err != nil {
		t.Fatal(err)
	}
	if !tb.isEventEnabled(eventbus.EventConntrackHigh) {
		t.Fatal("selected high-usage event is disabled")
	}
	if err := tb.settingService.SetTgEnabledEvents(""); err != nil {
		t.Fatal(err)
	}
	if tb.isEventEnabled(eventbus.EventConntrackHigh) {
		t.Fatal("unselected high-usage event is enabled")
	}
}

func TestConntrackTranslations(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(repoRootForTest(t), "internal/web/translation/*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("find translations: %v", err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var bundle map[string]any
		if err := json.Unmarshal(data, &bundle); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{
			"tgbot.messages.conntrackHigh",
			"pages.settings.eventConntrackHigh",
		} {
			if !nestedKeyPresent(bundle, key) {
				t.Errorf("%s is missing %s", filepath.Base(file), key)
			}
		}
	}
}
