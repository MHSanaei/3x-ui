package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"unicode/utf16"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// Discord's published caps, pinned here and not read from the package so the
// assertions still redden if those caps are ever loosened.
const (
	discordFieldNameLimit   = 256
	discordFieldValueLimit  = 1024
	discordEmbedFieldCap    = 25
	discordEmbedsPerMessage = 10
	discordMessageCharCap   = 6000
)

// utf16Units counts the way Discord counts: its caps follow JavaScript string
// length, where an astral rune is two units.
func utf16Units(s string) int {
	return len(utf16.Encode([]rune(s)))
}

func inboundFixtures(count int) []*model.Inbound {
	inbounds := make([]*model.Inbound, 0, count)
	for i := range count {
		inbounds = append(inbounds, &model.Inbound{
			Id:       i + 1,
			Remark:   fmt.Sprintf("inbound-%d", i),
			Port:     10000 + i,
			Protocol: "vless",
			Enable:   i%2 == 0,
			ClientStats: []xray.ClientTraffic{
				{Email: fmt.Sprintf("client-%d@test", i), Enable: true},
			},
		})
	}
	return inbounds
}

// assertsDiscordLimits checks every message against the caps Discord enforces and
// returns how many inbound fields the reply carried in total.
func assertsDiscordLimits(t *testing.T, msgs []MessagePayload) int {
	t.Helper()
	fields := 0
	for i, msg := range msgs {
		if len(msg.Embeds) > discordEmbedsPerMessage {
			t.Errorf("message %d carries %d embeds, Discord accepts %d", i, len(msg.Embeds), discordEmbedsPerMessage)
		}
		chars := 0
		for _, embed := range msg.Embeds {
			footer := ""
			if embed.Footer != nil {
				footer = embed.Footer.Text
			}
			chars += utf16Units(embed.Title) + utf16Units(embed.Description) + utf16Units(footer)
			if len(embed.Fields) > discordEmbedFieldCap {
				t.Errorf("message %d has an embed with %d fields, Discord accepts %d", i, len(embed.Fields), discordEmbedFieldCap)
			}
			for _, field := range embed.Fields {
				fields++
				chars += utf16Units(field.Name) + utf16Units(field.Value)
				if n := utf16Units(field.Name); n > discordFieldNameLimit {
					t.Errorf("field name is %d units, Discord accepts %d", n, discordFieldNameLimit)
				}
				if n := utf16Units(field.Value); n > discordFieldValueLimit {
					t.Errorf("field value is %d units, Discord accepts %d", n, discordFieldValueLimit)
				}
			}
		}
		if chars > discordMessageCharCap {
			t.Errorf("message %d carries %d units, Discord accepts %d", i, chars, discordMessageCharCap)
		}
	}
	return fields
}

// runInboundsCommandWith drives !inbounds against a channel that answers each
// POST through respond, reporting what Discord accepted and the POST count.
func runInboundsCommandWith(t *testing.T, settingService service.SettingService, inbounds []*model.Inbound, respond func(post int) (int, string)) ([]MessagePayload, int) {
	t.Helper()
	var mu sync.Mutex
	var sent []MessagePayload
	posts := 0
	restServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload MessagePayload
		_ = json.NewDecoder(r.Body).Decode(&payload)
		mu.Lock()
		posts++
		post := posts
		mu.Unlock()

		status, body := respond(post)
		if status == http.StatusOK {
			mu.Lock()
			sent = append(sent, payload)
			mu.Unlock()
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(restServer.Close)

	svc := NewDiscordService(settingService)
	svc.SetBaseURL(restServer.URL)
	svc.SetHTTPClient(restServer.Client())

	gw := NewGatewayClient(svc, settingService, &mockServerProvider{}, &mockInboundProvider{inbounds: inbounds}, &mockXrayRestart{})
	gw.handleMessage(context.Background(), MessageCreateData{
		ID:        "m1",
		ChannelID: "ch-1",
		Content:   "!inbounds",
		Author: struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Bot      bool   `json:"bot"`
		}{ID: "u1", Username: "Alice"},
	})

	mu.Lock()
	defer mu.Unlock()
	return append([]MessagePayload(nil), sent...), posts
}

func runInboundsCommand(t *testing.T, settingService service.SettingService, inbounds []*model.Inbound) []MessagePayload {
	t.Helper()
	msgs, _ := runInboundsCommandWith(t, settingService, inbounds, func(int) (int, string) {
		return http.StatusOK, `{"id": "msg-1"}`
	})
	return msgs
}

func TestInboundsCommandStaysWithinDiscordLimits(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotToken("test-bot-token")
	_ = settingService.SetDiscordChannelId("ch-1")
	_ = settingService.SetDiscordAdminIds("u1")

	t.Run("a large panel is paged instead of rejected", func(t *testing.T) {
		const count = 300
		msgs := runInboundsCommand(t, settingService, inboundFixtures(count))

		if got := assertsDiscordLimits(t, msgs); got != count {
			t.Errorf("reply listed %d inbounds, want %d", got, count)
		}
		if len(msgs) < 2 {
			t.Errorf("%d inbounds reached Discord in %d message(s), want the reply paged", count, len(msgs))
		}
	})

	t.Run("a remark past the field name cap is truncated, not dropped", func(t *testing.T) {
		inbounds := inboundFixtures(3)
		inbounds[0].Remark = strings.Repeat("r", 400)
		msgs := runInboundsCommand(t, settingService, inbounds)

		if got := assertsDiscordLimits(t, msgs); got != len(inbounds) {
			t.Fatalf("reply listed %d inbounds, want %d", got, len(inbounds))
		}
		if len(msgs) != 1 {
			t.Fatalf("expected one message for %d inbounds, got %d", len(inbounds), len(msgs))
		}
		name := msgs[0].Embeds[0].Fields[0].Name
		if units := utf16Units(name); units != discordFieldNameLimit {
			t.Errorf("truncated name is %d units, want %d: %q", units, discordFieldNameLimit, name)
		}
		if !strings.HasPrefix(name, "📍 "+strings.Repeat("r", 100)) {
			t.Errorf("truncated name lost the remark: %q", name)
		}
	})

	t.Run("an astral remark is cut to the cap, which runes would overshoot", func(t *testing.T) {
		inbounds := inboundFixtures(40)
		for _, in := range inbounds {
			in.Remark = strings.Repeat("🚀", 300)
		}
		msgs := runInboundsCommand(t, settingService, inbounds)

		if got := assertsDiscordLimits(t, msgs); got != len(inbounds) {
			t.Errorf("reply listed %d inbounds, want %d", got, len(inbounds))
		}
	})
}

func TestInboundsCommandRetriesARateLimitedPage(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotToken("test-bot-token")
	_ = settingService.SetDiscordChannelId("ch-1")
	_ = settingService.SetDiscordAdminIds("u1")

	const count = 300
	msgs, posts := runInboundsCommandWith(t, settingService, inboundFixtures(count), func(post int) (int, string) {
		if post == 1 {
			return http.StatusTooManyRequests, `{"message": "You are being rate limited.", "retry_after": 0.05}`
		}
		return http.StatusOK, `{"id": "msg-1"}`
	})

	if got := assertsDiscordLimits(t, msgs); got != count {
		t.Errorf("reply listed %d inbounds after the retry, want %d", got, count)
	}
	if len(msgs) < 2 {
		t.Errorf("rate limited page left %d message(s), want the rest of the reply", len(msgs))
	}
	if posts != len(msgs)+1 {
		t.Errorf("posted %d times for %d messages, want one retry of the limited page", posts, len(msgs))
	}
}
