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
		&i18n.Message{ID: "tgbot.messages.conntrackHigh", Other: "High {{ .Count }}/{{ .Max }} ({{ .Percent }}%)"},
		&i18n.Message{ID: "tgbot.messages.conntrackRecovered", Other: "Recovered {{ .Count }}/{{ .Max }} ({{ .Percent }}%)"},
	)
	tb := &Tgbot{}
	for _, test := range []struct {
		name  string
		event eventbus.Event
		want  string
	}{
		{"high", eventbus.Event{Type: eventbus.EventConntrackHigh, Data: &eventbus.ConntrackData{Count: 6144, Max: 7680, Percent: 80}}, "High 6144/7680 (80.0%)"},
		{"recovered", eventbus.Event{Type: eventbus.EventConntrackRecovered, Data: &eventbus.ConntrackData{Count: 5000, Max: 7680, Percent: 65.104}}, "Recovered 5000/7680 (65.1%)"},
		{"missing data", eventbus.Event{Type: eventbus.EventConntrackHigh}, ""},
		{"invalid data", eventbus.Event{Type: eventbus.EventConntrackRecovered, Data: "invalid"}, ""},
		{"nil data", eventbus.Event{Type: eventbus.EventConntrackHigh, Data: (*eventbus.ConntrackData)(nil)}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := tb.formatEventMessage(test.event)
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("message = %q, want %q", got, test.want)
			}
		})
	}
}

func TestConntrackEventsShareNoticeCard(t *testing.T) {
	tb := &Tgbot{}
	high := eventbus.Event{Type: eventbus.EventConntrackHigh, Source: "local"}
	recovered := eventbus.Event{Type: eventbus.EventConntrackRecovered, Source: "local"}
	if tb.eventNoticeKind(high) != tb.eventNoticeKind(recovered) {
		t.Fatal("recovery must update the high-usage notice")
	}
	recovered.Source = "other"
	if tb.eventNoticeKind(high) == tb.eventNoticeKind(recovered) {
		t.Fatal("different sources must keep separate notices")
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
	if tb.isEventEnabled(eventbus.EventConntrackRecovered) {
		t.Fatal("unselected recovery event is enabled")
	}
	if err := tb.settingService.SetTgEnabledEvents("conntrack.recovered"); err != nil {
		t.Fatal(err)
	}
	if tb.isEventEnabled(eventbus.EventConntrackHigh) || !tb.isEventEnabled(eventbus.EventConntrackRecovered) {
		t.Fatal("event selection was not refreshed")
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
			"tgbot.messages.conntrackRecovered",
			"pages.settings.eventConntrackHigh",
			"pages.settings.eventConntrackRecovered",
		} {
			if !nestedKeyPresent(bundle, key) {
				t.Errorf("%s is missing %s", filepath.Base(file), key)
			}
		}
	}
}
