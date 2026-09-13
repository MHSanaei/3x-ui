package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
)

type fixedTgLang struct{}

func (fixedTgLang) GetTgLang() (string, error) { return "en-US", nil }

// TestMain loads the real translation files so embeds render text instead of bare keys.
func TestMain(m *testing.M) {
	if err := locale.InitLocalizer(os.DirFS("../.."), fixedTgLang{}); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestDiscordMessagesFollowDiscordLang(t *testing.T) {
	const lang = "ru-RU"
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordLang(lang)
	_ = settingService.SetDiscordBotToken("token")
	_ = settingService.SetDiscordChannelId("ch-1")
	_ = settingService.SetDiscordAdminIds("admin-1")

	titles := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p MessagePayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		if len(p.Embeds) > 0 {
			titles <- p.Embeds[0].Title
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	svc := NewDiscordService(settingService)
	svc.SetBaseURL(server.URL)
	svc.SetHTTPClient(server.Client())

	sentTitle := func(t *testing.T) string {
		t.Helper()
		select {
		case title := <-titles:
			return title
		default:
			t.Fatal("no embed reached Discord")
			return ""
		}
	}

	cases := []struct {
		key    string
		render func(t *testing.T) string
	}{
		{"discord.test.title", func(t *testing.T) string {
			if err := svc.SendTest(context.Background()); err != nil {
				t.Fatalf("SendTest: %v", err)
			}
			return sentTitle(t)
		}},
		{"discord.alerts.xrayCrash", func(t *testing.T) string {
			embed, _ := NewSubscriber(settingService, svc).FormatEmbed(eventbus.Event{Type: eventbus.EventXrayCrash})
			return embed.Title
		}},
		{"discord.report.title", func(t *testing.T) string {
			payload, _, err := svc.BuildReport(context.Background(), nil, nil)
			if err != nil {
				t.Fatalf("BuildReport: %v", err)
			}
			return payload.Embeds[0].Title
		}},
		{"discord.commands.helpTitle", func(t *testing.T) string {
			msg := MessageCreateData{ChannelID: "ch-1", Content: "!help"}
			msg.Author.ID = "admin-1"
			NewGatewayClient(svc, settingService, nil, nil, nil).handleMessage(context.Background(), msg)
			return sentTitle(t)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			want := locale.I18nForLang(lang, tc.key)
			if want == locale.I18nForLang("en-US", tc.key) {
				t.Fatalf("%s has no distinct %s translation", tc.key, lang)
			}
			if got := tc.render(t); got != want {
				t.Fatalf("title = %q, want the %s text %q", got, lang, want)
			}
		})
	}
}
