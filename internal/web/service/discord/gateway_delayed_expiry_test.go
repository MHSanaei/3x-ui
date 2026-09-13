package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// expiryField is the position of the expiry field in the usage embed.
const expiryField = 5

func runUsageCommand(t *testing.T, settingService service.SettingService, inbounds []*model.Inbound, email string) MessagePayload {
	t.Helper()
	var mu sync.Mutex
	var sent []MessagePayload
	restServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload MessagePayload
		_ = json.NewDecoder(r.Body).Decode(&payload)
		mu.Lock()
		sent = append(sent, payload)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "msg-1"}`))
	}))
	t.Cleanup(restServer.Close)

	svc := NewDiscordService(settingService)
	svc.SetBaseURL(restServer.URL)
	svc.SetHTTPClient(restServer.Client())

	gw := NewGatewayClient(svc, settingService, &mockServerProvider{}, &mockInboundProvider{inbounds: inbounds}, &mockXrayRestart{})
	gw.handleMessage(context.Background(), MessageCreateData{
		ID:        "m1",
		ChannelID: "ch-1",
		Content:   "!usage " + email,
		Author: struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Bot      bool   `json:"bot"`
		}{ID: "u1", Username: "Alice"},
	})

	mu.Lock()
	defer mu.Unlock()
	if len(sent) == 0 {
		t.Fatalf("!usage %s sent nothing", email)
	}
	return sent[0]
}

func usageExpiryValue(t *testing.T, payload MessagePayload) string {
	t.Helper()
	if len(payload.Embeds) != 1 {
		t.Fatalf("expected one embed, got %d", len(payload.Embeds))
	}
	fields := payload.Embeds[0].Fields
	if len(fields) <= expiryField {
		t.Fatalf("usage embed has %d fields, want at least %d", len(fields), expiryField+1)
	}
	return fields[expiryField].Value
}

func TestUsageExpiryForDelayedStart(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotToken("test-bot-token")
	_ = settingService.SetDiscordChannelId("ch-1")
	_ = settingService.SetDiscordAdminIds("u1")

	const email = "delayed@test"

	t.Run("a start-after-first-use client counts down in days", func(t *testing.T) {
		// -2592000000 ms is what the panel stores for "Start After First Use: 30 days".
		inbounds := []*model.Inbound{{
			Id:          1,
			Remark:      "delayed",
			Port:        443,
			Protocol:    "vless",
			Enable:      true,
			ClientStats: []xray.ClientTraffic{{Email: email, Enable: true, ExpiryTime: -2592000000}},
		}}

		got := usageExpiryValue(t, runUsageCommand(t, settingService, inbounds, email))
		if got != "30 Days" {
			t.Errorf("delayed start shows %q, want %q", got, "30 Days")
		}
	})

	t.Run("an absolute deadline still shows as a date", func(t *testing.T) {
		const deadline = int64(4102444800000) // 2100-01-01 UTC, in ms
		inbounds := []*model.Inbound{{
			Id:          2,
			Remark:      "deadline",
			Port:        8443,
			Protocol:    "vless",
			Enable:      true,
			ClientStats: []xray.ClientTraffic{{Email: email, Enable: true, ExpiryTime: deadline}},
		}}

		got := usageExpiryValue(t, runUsageCommand(t, settingService, inbounds, email))
		if got == "Unlimited" || got == "30 Days" {
			t.Errorf("absolute deadline shows %q, want a formatted date", got)
		}
	})
}
