package wechat

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
)

func TestSubscriberFiltersEventsAndUsesIndependentWechatThresholds(t *testing.T) {
	settings := setupSettings(t, map[string]string{
		"wechatEnable": "true", "wechatMode": "webhook", "wechatWebhookURL": "https://receiver.example/notify",
		"wechatEnabledEvents": "cpu.high,memory.high,node.down", "wechatCpu": "80", "wechatMemory": "70",
		"tgCpu": "1", "tgMemory": "1", "discordCpu": "1", "discordMemory": "1", "wechatLang": "en-US",
	})
	var payloads []Envelope
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) {
		var payload Envelope
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		payloads = append(payloads, payload)
	})
	subscriber := NewSubscriber(settings, svc)
	for _, event := range []eventbus.Event{
		{Type: eventbus.EventCPUHigh, Data: eventbus.SystemMetricData{Percent: 79}},
		{Type: eventbus.EventMemoryHigh, Data: &eventbus.SystemMetricData{Percent: 69}},
		{Type: eventbus.EventOutboundDown, Source: "filtered"},
		{Type: eventbus.EventCPUHigh, Data: eventbus.SystemMetricData{Percent: 85}},
		{Type: eventbus.EventMemoryHigh, Data: &eventbus.SystemMetricData{Percent: 75}},
		{Type: eventbus.EventNodeDown, Source: "node-a"},
	} {
		subscriber.HandleEvent(event)
	}
	if len(payloads) != 3 || payloads[0].Event != "cpu.high" || payloads[1].Event != "memory.high" || payloads[2].Event != "node.down" {
		t.Fatalf("event filters or independent thresholds failed: %+v", payloads)
	}
}

func TestSubscriberFailedSendDoesNotConsumeSuccessfulSendCooldown(t *testing.T) {
	settings := setupSettings(t, map[string]string{
		"wechatEnable": "true", "wechatMode": "webhook", "wechatWebhookURL": "https://receiver.example/notify",
		"wechatEnabledEvents": "node.down", "wechatLang": "en-US",
	})
	var calls int
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	subscriber := NewSubscriber(settings, svc)
	clock := time.Unix(1000, 0)
	subscriber.now = func() time.Time { return clock }
	event := eventbus.Event{Type: eventbus.EventNodeDown, Source: "node-a"}
	subscriber.HandleEvent(event)
	subscriber.HandleEvent(event)
	subscriber.HandleEvent(event)
	if calls != 2 {
		t.Fatalf("failure consumed cooldown or success was duplicated: %d requests", calls)
	}
	clock = clock.Add(time.Minute)
	subscriber.HandleEvent(event)
	if calls != 3 {
		t.Fatalf("cooldown did not expire: %d requests", calls)
	}
}

func TestLoginFailureDoesNotSuppressSuccessFromTheSameIP(t *testing.T) {
	settings := setupSettings(t, map[string]string{
		"wechatEnable": "true", "wechatMode": "webhook", "wechatWebhookURL": "https://receiver.example/notify",
		"wechatEnabledEvents": "login.attempt", "wechatLang": "en-US",
	})
	var texts []string
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) {
		var payload Envelope
		_ = json.NewDecoder(r.Body).Decode(&payload)
		texts = append(texts, payload.Text)
	})
	subscriber := NewSubscriber(settings, svc)
	for _, status := range []string{"fail", "success", "success"} {
		subscriber.HandleEvent(eventbus.Event{
			Type: eventbus.EventLoginAttempt, Source: "203.0.113.10",
			Data: eventbus.LoginEventData{IP: "203.0.113.10", Username: "test", Status: status},
		})
	}
	if len(texts) != 2 || !strings.Contains(texts[0], "Login Failed") || !strings.Contains(texts[1], "Login Success") {
		t.Fatalf("login success was lost after failure: %q", texts)
	}
}

func TestSubscriberSupportsAllNativeEventsWithoutFilesOrGlobalBotLocale(t *testing.T) {
	settings := setupSettings(t, map[string]string{
		"wechatEnable": "true", "wechatMode": "webhook", "wechatWebhookURL": "https://receiver.example/notify",
		"wechatEnabledEvents": "outbound.down,outbound.up,node.down,node.up,xray.crash,cpu.high,memory.high,login.attempt",
		"wechatCpu":           "50", "wechatMemory": "50", "wechatLang": "en-US", "tgLang": "ru-RU",
	})
	var received []string
	var texts []string
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload Envelope
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Error(err)
		}
		if payload.Type != "event" || payload.Text == "" || payload.ID == "" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("invalid event request: %s", body)
		}
		received = append(received, payload.Event)
		texts = append(texts, payload.Text)
	})
	subscriber := NewSubscriber(settings, svc)
	events := []eventbus.Event{
		{Type: eventbus.EventOutboundDown, Source: "proxy-a", Data: eventbus.OutboundHealthData{Error: "connection refused"}},
		{Type: eventbus.EventOutboundUp, Source: "proxy-a", Data: &eventbus.OutboundHealthData{Delay: 18}},
		{Type: eventbus.EventNodeDown, Source: "node-a", Data: &eventbus.NodeHealthData{XrayError: "stopped"}},
		{Type: eventbus.EventNodeUp, Source: "node-a", Data: eventbus.NodeHealthData{LatencyMs: 20}},
		{Type: eventbus.EventXrayCrash, Data: "process exited"},
		{Type: eventbus.EventCPUHigh, Data: eventbus.SystemMetricData{Percent: 90}},
		{Type: eventbus.EventMemoryHigh, Data: eventbus.SystemMetricData{Percent: 90}},
		{Type: eventbus.EventLoginAttempt, Source: "203.0.113.10", Data: eventbus.LoginEventData{Username: "test", IP: "203.0.113.10", Status: "success"}},
	}
	for _, event := range events {
		subscriber.HandleEvent(event)
	}
	if len(received) != 8 {
		t.Fatalf("expected all eight event types, got %v", received)
	}
	for i, event := range events {
		if received[i] != string(event.Type) {
			t.Fatalf("event %d became %q", i, received[i])
		}
	}
	if !strings.Contains(texts[4], "Xray Core Crashed") {
		t.Fatalf("Telegram's global Russian locale changed WeChat text: %s", texts[4])
	}
}
