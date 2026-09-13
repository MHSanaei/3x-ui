package discord

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
)

func TestFormatEmbed_OutboundDownAndUp(t *testing.T) {
	settingService := setupTestDB(t)
	discordService := NewDiscordService(settingService)
	sub := NewSubscriber(settingService, discordService)

	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)

	// Outbound down
	downEvent := eventbus.Event{
		Type:      eventbus.EventOutboundDown,
		Source:    "proxy-1",
		Timestamp: now,
		Data: &eventbus.OutboundHealthData{
			Delay: 500,
			Error: "timeout connecting",
		},
	}
	embed, ok := sub.FormatEmbed(downEvent)
	if !ok {
		t.Fatal("expected embed to be formatted")
	}
	if embed.Color != ColorRed {
		t.Errorf("expected ColorRed, got 0x%X", embed.Color)
	}
	if embed.Timestamp != "2026-09-12T12:00:00Z" {
		t.Errorf("expected RFC3339 UTC timestamp, got %s", embed.Timestamp)
	}
	if len(embed.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(embed.Fields))
	}

	// Outbound up
	upEvent := eventbus.Event{
		Type:      eventbus.EventOutboundUp,
		Source:    "proxy-1",
		Timestamp: now,
		Data: &eventbus.OutboundHealthData{
			Delay: 120,
		},
	}
	embedUp, ok := sub.FormatEmbed(upEvent)
	if !ok {
		t.Fatal("expected embed to be formatted")
	}
	if embedUp.Color != ColorGreen {
		t.Errorf("expected ColorGreen, got 0x%X", embedUp.Color)
	}
}

func TestFormatEmbed_NodeDownAndUp(t *testing.T) {
	settingService := setupTestDB(t)
	discordService := NewDiscordService(settingService)
	sub := NewSubscriber(settingService, discordService)

	now := time.Now().UTC()

	// Node down
	downEvent := eventbus.Event{
		Type:      eventbus.EventNodeDown,
		Source:    "node-us",
		Timestamp: now,
		Data: &eventbus.NodeHealthData{
			XrayError: "connection refused",
		},
	}
	embed, ok := sub.FormatEmbed(downEvent)
	if !ok {
		t.Fatal("expected embed to be formatted")
	}
	if embed.Color != ColorRed {
		t.Errorf("expected ColorRed, got 0x%X", embed.Color)
	}

	// Node up
	upEvent := eventbus.Event{
		Type:      eventbus.EventNodeUp,
		Source:    "node-us",
		Timestamp: now,
		Data: &eventbus.NodeHealthData{
			LatencyMs: 45,
		},
	}
	embedUp, ok := sub.FormatEmbed(upEvent)
	if !ok {
		t.Fatal("expected embed to be formatted")
	}
	if embedUp.Color != ColorGreen {
		t.Errorf("expected ColorGreen, got 0x%X", embedUp.Color)
	}
}

func TestFormatEmbed_XrayCrash(t *testing.T) {
	settingService := setupTestDB(t)
	discordService := NewDiscordService(settingService)
	sub := NewSubscriber(settingService, discordService)

	crashEvent := eventbus.Event{
		Type:      eventbus.EventXrayCrash,
		Timestamp: time.Now().UTC(),
		Data:      "panic: core dump",
	}
	embed, ok := sub.FormatEmbed(crashEvent)
	if !ok {
		t.Fatal("expected embed to be formatted")
	}
	if embed.Color != ColorRed {
		t.Errorf("expected ColorRed, got 0x%X", embed.Color)
	}
}

func TestFormatEmbed_CpuAndMemoryThresholds(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordCpu(80)
	_ = settingService.SetDiscordMemory(75)

	discordService := NewDiscordService(settingService)
	sub := NewSubscriber(settingService, discordService)

	now := time.Now().UTC()

	// CPU below threshold -> no embed
	_, ok := sub.FormatEmbed(eventbus.Event{
		Type:      eventbus.EventCPUHigh,
		Timestamp: now,
		Data:      &eventbus.SystemMetricData{Percent: 79.5},
	})
	if ok {
		t.Error("expected no embed when CPU is below threshold")
	}

	// CPU above threshold -> Orange embed
	embedCpu, ok := sub.FormatEmbed(eventbus.Event{
		Type:      eventbus.EventCPUHigh,
		Timestamp: now,
		Data:      &eventbus.SystemMetricData{Percent: 85.2},
	})
	if !ok {
		t.Fatal("expected embed when CPU is above threshold")
	}
	if embedCpu.Color != ColorOrange {
		t.Errorf("expected ColorOrange (0x%X), got 0x%X", ColorOrange, embedCpu.Color)
	}

	// Memory below threshold -> no embed
	_, ok = sub.FormatEmbed(eventbus.Event{
		Type:      eventbus.EventMemoryHigh,
		Timestamp: now,
		Data:      &eventbus.SystemMetricData{Percent: 70.0},
	})
	if ok {
		t.Error("expected no embed when Memory is below threshold")
	}

	// Memory above threshold -> Orange embed
	embedMem, ok := sub.FormatEmbed(eventbus.Event{
		Type:      eventbus.EventMemoryHigh,
		Timestamp: now,
		Data:      &eventbus.SystemMetricData{Percent: 90.0},
	})
	if !ok {
		t.Fatal("expected embed when Memory is above threshold")
	}
	if embedMem.Color != ColorOrange {
		t.Errorf("expected ColorOrange (0x%X), got 0x%X", ColorOrange, embedMem.Color)
	}
}

func TestFormatEmbed_LoginAttempt(t *testing.T) {
	settingService := setupTestDB(t)
	discordService := NewDiscordService(settingService)
	sub := NewSubscriber(settingService, discordService)

	now := time.Now().UTC()

	// Login success -> Green
	successEvent := eventbus.Event{
		Type:      eventbus.EventLoginAttempt,
		Timestamp: now,
		Data: &eventbus.LoginEventData{
			Username: "admin",
			IP:       "1.2.3.4",
			Time:     "2026-09-12 12:00:00",
			Status:   "success",
		},
	}
	embedSuccess, ok := sub.FormatEmbed(successEvent)
	if !ok {
		t.Fatal("expected embed for login success")
	}
	if embedSuccess.Color != ColorGreen {
		t.Errorf("expected ColorGreen, got 0x%X", embedSuccess.Color)
	}

	// Login fail -> Red
	failEvent := eventbus.Event{
		Type:      eventbus.EventLoginAttempt,
		Timestamp: now,
		Data: &eventbus.LoginEventData{
			Username: "attacker",
			IP:       "5.6.7.8",
			Time:     "2026-09-12 12:01:00",
			Status:   "fail",
			Reason:   "wrong password",
		},
	}
	embedFail, ok := sub.FormatEmbed(failEvent)
	if !ok {
		t.Fatal("expected embed for login failure")
	}
	if embedFail.Color != ColorRed {
		t.Errorf("expected ColorRed, got 0x%X", embedFail.Color)
	}

	// Fallback when data is nil
	fallbackEvent := eventbus.Event{
		Type:      eventbus.EventLoginAttempt,
		Source:    "unknown-source",
		Timestamp: now,
	}
	embedFallback, ok := sub.FormatEmbed(fallbackEvent)
	if !ok {
		t.Fatal("expected embed for fallback login")
	}
	if embedFallback.Color != ColorRed {
		t.Errorf("expected ColorRed, got 0x%X", embedFallback.Color)
	}
}

func TestCleanField_Protection(t *testing.T) {
	field := cleanField("", "  ", true)
	if field.Name != "-" || field.Value != "-" {
		t.Errorf("expected '-' for empty field name/value, got name=%q, value=%q", field.Name, field.Value)
	}

	field2 := cleanField(" Name ", " Value ", false)
	if field2.Name != "Name" || field2.Value != "Value" {
		t.Errorf("expected trimmed name/value, got name=%q, value=%q", field2.Name, field2.Value)
	}
}

func TestHandleEvent_EndToEndWithServer(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotEnable(true)
	_ = settingService.SetDiscordBotToken("test-token")
	_ = settingService.SetDiscordChannelId("ch-test")
	_ = settingService.SetDiscordEnabledEvents("login.attempt,outbound.down")

	receivedCh := make(chan MessagePayload, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p MessagePayload
		_ = json.Unmarshal(body, &p)
		receivedCh <- p
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	discordService := NewDiscordService(settingService)
	discordService.SetBaseURL(server.URL)
	discordService.SetHTTPClient(server.Client())

	sub := NewSubscriber(settingService, discordService)

	// 1. Send enabled event (outbound.down)
	sub.HandleEvent(eventbus.Event{
		Type:      eventbus.EventOutboundDown,
		Source:    "out-1",
		Timestamp: time.Now().UTC(),
	})

	select {
	case p := <-receivedCh:
		if len(p.Embeds) != 1 || p.Embeds[0].Color != ColorRed {
			t.Errorf("unexpected payload: %+v", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for outbound.down message")
	}

	// 2. Duplicate outbound.down within rate limit -> should be suppressed
	sub.HandleEvent(eventbus.Event{
		Type:      eventbus.EventOutboundDown,
		Source:    "out-1",
		Timestamp: time.Now().UTC(),
	})

	select {
	case p := <-receivedCh:
		t.Fatalf("rate limited event was unexpectedly sent: %+v", p)
	case <-time.After(150 * time.Millisecond):
		// OK
	}

	// 3. Login attempt bypasses rate limit
	sub.HandleEvent(eventbus.Event{
		Type:      eventbus.EventLoginAttempt,
		Timestamp: time.Now().UTC(),
		Data: &eventbus.LoginEventData{
			Username: "admin",
			IP:       "1.1.1.1",
			Status:   "success",
		},
	})
	sub.HandleEvent(eventbus.Event{
		Type:      eventbus.EventLoginAttempt,
		Timestamp: time.Now().UTC(),
		Data: &eventbus.LoginEventData{
			Username: "admin",
			IP:       "1.1.1.1",
			Status:   "success",
		},
	})

	// Both should arrive
	for i := 0; i < 2; i++ {
		select {
		case <-receivedCh:
			// OK
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for login attempt message %d", i+1)
		}
	}

	// 4. Disabled event type (cpu.high is not in discordEnabledEvents)
	_ = settingService.SetDiscordCpu(50)
	sub.HandleEvent(eventbus.Event{
		Type:      eventbus.EventCPUHigh,
		Timestamp: time.Now().UTC(),
		Data:      &eventbus.SystemMetricData{Percent: 99.0},
	})

	select {
	case p := <-receivedCh:
		t.Fatalf("disabled event was unexpectedly sent: %+v", p)
	case <-time.After(150 * time.Millisecond):
		// OK
	}

	// 5. Bot disabled entirely
	_ = settingService.SetDiscordBotEnable(false)
	sub.HandleEvent(eventbus.Event{
		Type:      eventbus.EventLoginAttempt,
		Timestamp: time.Now().UTC(),
		Data: &eventbus.LoginEventData{
			Username: "admin",
			IP:       "1.1.1.1",
			Status:   "success",
		},
	})

	select {
	case p := <-receivedCh:
		t.Fatalf("event sent while bot disabled: %+v", p)
	case <-time.After(150 * time.Millisecond):
		// OK
	}
}

func TestCleanField_Truncation(t *testing.T) {
	longName := strings.Repeat("А", 300)   // 300 runes of 2-byte UTF-8
	longValue := strings.Repeat("🔥", 1200) // 1200 runes of 4-byte UTF-8

	field := cleanField(longName, longValue, false)
	nameRunes := []rune(field.Name)
	valRunes := []rune(field.Value)

	if len(nameRunes) > 256 {
		t.Errorf("expected name runes <= 256, got %d", len(nameRunes))
	}
	if !strings.HasSuffix(field.Name, "...") {
		t.Errorf("expected truncated name to end with '...', got %s", field.Name)
	}

	if len(valRunes) > 1024 {
		t.Errorf("expected value runes <= 1024, got %d", len(valRunes))
	}
	if !strings.HasSuffix(field.Value, "...") {
		t.Errorf("expected truncated value to end with '...', got %s", field.Value)
	}
}

func TestHandleEvent_BelowThresholdDoesNotBurnRateLimiter(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotEnable(true)
	_ = settingService.SetDiscordBotToken("test-token")
	_ = settingService.SetDiscordChannelId("ch-test")
	_ = settingService.SetDiscordEnabledEvents("cpu.high")
	_ = settingService.SetDiscordCpu(80)

	receivedCh := make(chan MessagePayload, 5)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p MessagePayload
		_ = json.Unmarshal(body, &p)
		receivedCh <- p
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	discordService := NewDiscordService(settingService)
	discordService.SetBaseURL(server.URL)
	discordService.SetHTTPClient(server.Client())

	sub := NewSubscriber(settingService, discordService)

	// 1. CPU at 50% (below 80% threshold) - must NOT be sent and must NOT burn rate limiter
	sub.HandleEvent(eventbus.Event{
		Type:      eventbus.EventCPUHigh,
		Timestamp: time.Now().UTC(),
		Data:      &eventbus.SystemMetricData{Percent: 50.0},
	})

	select {
	case p := <-receivedCh:
		t.Fatalf("sub-threshold CPU event was unexpectedly sent: %+v", p)
	case <-time.After(150 * time.Millisecond):
		// OK
	}

	// 2. CPU immediately spikes to 95% (above 80% threshold) - MUST be sent!
	sub.HandleEvent(eventbus.Event{
		Type:      eventbus.EventCPUHigh,
		Timestamp: time.Now().UTC(),
		Data:      &eventbus.SystemMetricData{Percent: 95.0},
	})

	select {
	case p := <-receivedCh:
		if len(p.Embeds) != 1 || p.Embeds[0].Color != ColorOrange {
			t.Errorf("unexpected payload for critical CPU alert: %+v", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("critical CPU alert was incorrectly suppressed by rate limiter after below-threshold event")
	}
}

func TestFormatEmbed_ValueTypes(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordCpu(80)
	_ = settingService.SetDiscordMemory(80)
	sub := NewSubscriber(settingService, NewDiscordService(settingService))
	now := time.Now().UTC()

	// OutboundHealthData by value
	embed, ok := sub.FormatEmbed(eventbus.Event{
		Type:      eventbus.EventOutboundDown,
		Source:    "out-val",
		Timestamp: now,
		Data: eventbus.OutboundHealthData{
			Delay: 350,
			Error: "connection lost",
		},
	})
	if !ok || len(embed.Fields) != 3 {
		t.Fatalf("expected 3 fields for OutboundDown value type, got ok=%v, fields=%d", ok, len(embed.Fields))
	}

	// NodeHealthData by value
	embedNode, ok := sub.FormatEmbed(eventbus.Event{
		Type:      eventbus.EventNodeUp,
		Source:    "node-val",
		Timestamp: now,
		Data: eventbus.NodeHealthData{
			LatencyMs: 25,
		},
	})
	if !ok || len(embedNode.Fields) != 2 {
		t.Fatalf("expected 2 fields for NodeUp value type, got ok=%v, fields=%d", ok, len(embedNode.Fields))
	}

	// SystemMetricData by value
	embedCPU, ok := sub.FormatEmbed(eventbus.Event{
		Type:      eventbus.EventCPUHigh,
		Timestamp: now,
		Data: eventbus.SystemMetricData{
			Percent: 90.0,
		},
	})
	if !ok || embedCPU.Color != ColorOrange {
		t.Fatalf("expected orange embed for CPU high value type, got ok=%v", ok)
	}

	// LoginEventData by value
	embedLogin, ok := sub.FormatEmbed(eventbus.Event{
		Type:      eventbus.EventLoginAttempt,
		Timestamp: now,
		Data: eventbus.LoginEventData{
			Username: "admin",
			IP:       "127.0.0.1",
			Status:   "success",
		},
	})
	if !ok || embedLogin.Color != ColorGreen {
		t.Fatalf("expected green embed for Login success value type, got ok=%v", ok)
	}
}
