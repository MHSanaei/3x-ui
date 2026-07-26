package webhook

import (
	"net/http"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
)

func TestHandleEvent_DisabledByDefault_NoRequest(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookURL(srv.URL))
	mustSet(t, settingService.SetWebhookEnabledEvents(string(eventbus.EventXrayCrash)))
	// webhookEnable left at its "false" default

	sub := NewSubscriber(settingService, NewWebhookService(settingService))
	sub.HandleEvent(eventbus.Event{Type: eventbus.EventXrayCrash})

	if srv.hitCount() != 0 {
		t.Errorf("expected no request while webhookEnable is false, got %d", srv.hitCount())
	}
}

func TestHandleEvent_EventNotInEnabledList_NoRequest(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookEnable(true))
	mustSet(t, settingService.SetWebhookURL(srv.URL))
	mustSet(t, settingService.SetWebhookEnabledEvents(string(eventbus.EventXrayCrash)))

	sub := NewSubscriber(settingService, NewWebhookService(settingService))
	// node.up is not in the enabled list
	sub.HandleEvent(eventbus.Event{Type: eventbus.EventNodeUp, Source: "node-1"})

	if srv.hitCount() != 0 {
		t.Errorf("expected no request for a disabled event type, got %d", srv.hitCount())
	}
}

func TestHandleEvent_EnabledEventFires(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookEnable(true))
	mustSet(t, settingService.SetWebhookURL(srv.URL))
	mustSet(t, settingService.SetWebhookEnabledEvents(string(eventbus.EventXrayCrash)+","+string(eventbus.EventNodeUp)))

	sub := NewSubscriber(settingService, NewWebhookService(settingService))
	sub.HandleEvent(eventbus.Event{Type: eventbus.EventXrayCrash})

	if srv.hitCount() != 1 {
		t.Errorf("expected exactly 1 request, got %d", srv.hitCount())
	}
}

func TestHandleEvent_RateLimited(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookEnable(true))
	mustSet(t, settingService.SetWebhookURL(srv.URL))
	mustSet(t, settingService.SetWebhookEnabledEvents(string(eventbus.EventOutboundDown)))

	sub := NewSubscriber(settingService, NewWebhookService(settingService))
	sub.HandleEvent(eventbus.Event{Type: eventbus.EventOutboundDown, Source: "proxy-1"})
	sub.HandleEvent(eventbus.Event{Type: eventbus.EventOutboundDown, Source: "proxy-1"})

	if srv.hitCount() != 1 {
		t.Errorf("expected the second event within cooldown to be rate-limited, got %d requests", srv.hitCount())
	}
}

func TestHandleEvent_RateLimiterIsPerSource(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookEnable(true))
	mustSet(t, settingService.SetWebhookURL(srv.URL))
	mustSet(t, settingService.SetWebhookEnabledEvents(string(eventbus.EventOutboundDown)))

	sub := NewSubscriber(settingService, NewWebhookService(settingService))
	sub.HandleEvent(eventbus.Event{Type: eventbus.EventOutboundDown, Source: "proxy-1"})
	sub.HandleEvent(eventbus.Event{Type: eventbus.EventOutboundDown, Source: "proxy-2"})

	if srv.hitCount() != 2 {
		t.Errorf("expected independent rate limits per source, got %d requests", srv.hitCount())
	}
}

func TestHandleEvent_LoginAttemptBypassesRateLimit(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookEnable(true))
	mustSet(t, settingService.SetWebhookURL(srv.URL))
	mustSet(t, settingService.SetWebhookEnabledEvents(string(eventbus.EventLoginAttempt)))

	sub := NewSubscriber(settingService, NewWebhookService(settingService))
	sub.HandleEvent(eventbus.Event{Type: eventbus.EventLoginAttempt, Source: "1.2.3.4"})
	sub.HandleEvent(eventbus.Event{Type: eventbus.EventLoginAttempt, Source: "1.2.3.4"})

	if srv.hitCount() != 2 {
		t.Errorf("login.attempt should bypass the rate limiter, got %d requests, want 2", srv.hitCount())
	}
}
