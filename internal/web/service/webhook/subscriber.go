package webhook

import (
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Subscriber handles event bus messages and sends webhook notifications.
type Subscriber struct {
	settingService service.SettingService
	webhookService *WebhookService
	limiter        *eventbus.RateLimiter
}

// NewSubscriber creates a new webhook event subscriber.
func NewSubscriber(settingService service.SettingService, webhookService *WebhookService) *Subscriber {
	return &Subscriber{
		settingService: settingService,
		webhookService: webhookService,
		limiter:        eventbus.NewRateLimiter(1 * time.Minute),
	}
}

// HandleEvent is the eventbus subscriber callback.
func (s *Subscriber) HandleEvent(e eventbus.Event) {
	if on, err := s.settingService.GetWebhookEnable(); err != nil || !on {
		return
	}
	if !s.isEventEnabled(e.Type) {
		return
	}
	if e.Type != eventbus.EventLoginAttempt {
		if !s.limiter.Allow(e.Type, e.Source) {
			return
		}
	}
	if err := s.webhookService.Send(e); err != nil {
		logger.Warning("webhook subscriber: send failed:", err)
	}
}

func (s *Subscriber) isEventEnabled(t eventbus.EventType) bool {
	events, err := s.settingService.GetWebhookEnabledEvents()
	if err != nil || events == "" {
		return false
	}
	for e := range strings.SplitSeq(events, ",") {
		if strings.TrimSpace(e) == string(t) {
			return true
		}
	}
	return false
}
