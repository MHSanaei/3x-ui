package wechat

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type Subscriber struct {
	settings service.SettingService
	service  *Service
	mu       sync.Mutex
	lastSent map[string]time.Time
	now      func() time.Time
}

func NewSubscriber(settings service.SettingService, svc *Service) *Subscriber {
	return &Subscriber{settings: settings, service: svc, lastSent: make(map[string]time.Time), now: time.Now}
}

func (s *Subscriber) HandleEvent(event eventbus.Event) {
	if s.service == nil {
		return
	}
	enabled, err := s.settings.GetWechatEnable()
	if err != nil || !enabled || !s.eventEnabled(event.Type) {
		return
	}
	text, ok := s.format(event)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for key, sent := range s.lastSent {
		if now.Sub(sent) >= time.Minute || now.Before(sent) {
			delete(s.lastSent, key)
		}
	}
	key := string(event.Type) + ":" + event.Source
	if event.Type == eventbus.EventLoginAttempt {
		if data := eventData[eventbus.LoginEventData](event.Data); data != nil {
			key += ":" + data.Status
		}
	}
	if sent, found := s.lastSent[key]; found && now.Sub(sent) < time.Minute {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := s.service.send(ctx, "event", string(event.Type), text, event.Timestamp); err != nil {
		logger.Warning("wechat subscriber: ", err)
		return
	}
	s.lastSent[key] = s.now()
}

func (s *Subscriber) eventEnabled(kind eventbus.EventType) bool {
	events, err := s.settings.GetWechatEnabledEvents()
	if err != nil {
		return false
	}
	for value := range strings.SplitSeq(events, ",") {
		if strings.TrimSpace(value) == string(kind) {
			return true
		}
	}
	return false
}

func eventData[T any](value any) *T {
	switch data := value.(type) {
	case T:
		return &data
	case *T:
		return data
	default:
		return nil
	}
}

func plain(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 256 {
		return string(runes[:256]) + "…"
	}
	return string(runes)
}

func (s *Subscriber) format(event eventbus.Event) (string, bool) {
	tr := s.service.translator()
	var title string
	fields := []string{}
	if source := plain(event.Source); source != "" {
		fields = append(fields, tr("discord.fields.source")+": "+source)
	}
	switch event.Type {
	case eventbus.EventOutboundDown, eventbus.EventOutboundUp:
		title = tr("discord.alerts.outboundDown")
		if event.Type == eventbus.EventOutboundUp {
			title = tr("discord.alerts.outboundUp")
		}
		if data := eventData[eventbus.OutboundHealthData](event.Data); data != nil {
			if data.Error != "" {
				fields = append(fields, tr("discord.fields.error")+": "+plain(data.Error))
			}
			if data.Delay > 0 {
				fields = append(fields, fmt.Sprintf("%s: %d ms", tr("discord.fields.delay"), data.Delay))
			}
		}
	case eventbus.EventNodeDown, eventbus.EventNodeUp:
		title = tr("discord.alerts.nodeDown")
		if event.Type == eventbus.EventNodeUp {
			title = tr("discord.alerts.nodeUp")
		}
		if data := eventData[eventbus.NodeHealthData](event.Data); data != nil {
			if data.XrayError != "" {
				fields = append(fields, tr("discord.fields.error")+": "+plain(data.XrayError))
			}
			if data.LatencyMs > 0 {
				fields = append(fields, fmt.Sprintf("%s: %d ms", tr("discord.fields.delay"), data.LatencyMs))
			}
		}
	case eventbus.EventXrayCrash:
		title = tr("discord.alerts.xrayCrash")
		if event.Data != nil {
			fields = append(fields, tr("discord.fields.error")+": "+plain(fmt.Sprint(event.Data)))
		}
	case eventbus.EventCPUHigh, eventbus.EventMemoryHigh:
		data := eventData[eventbus.SystemMetricData](event.Data)
		if data == nil || math.IsNaN(data.Percent) || math.IsInf(data.Percent, 0) {
			return "", false
		}
		threshold, err := s.settings.GetWechatCpu()
		title = tr("discord.alerts.cpuHigh")
		if event.Type == eventbus.EventMemoryHigh {
			threshold, err = s.settings.GetWechatMemory()
			title = tr("discord.alerts.memoryHigh")
		}
		if err != nil || threshold <= 0 || data.Percent <= float64(threshold) {
			return "", false
		}
		fields = append(fields, fmt.Sprintf("%s: %.2f%%; %s: %d%%", tr("usage"), data.Percent, tr("discord.fields.threshold"), threshold))
	case eventbus.EventLoginAttempt:
		title = tr("discord.alerts.loginFailed")
		if data := eventData[eventbus.LoginEventData](event.Data); data != nil {
			if data.Status == "success" {
				title = tr("discord.alerts.loginSuccess")
			}
			fields = append(fields, tr("username")+": "+plain(data.Username), "IP: "+plain(data.IP))
			if data.Reason != "" {
				fields = append(fields, tr("discord.fields.reason")+": "+plain(data.Reason))
			}
			if data.Time != "" {
				fields = append(fields, tr("discord.fields.time")+": "+plain(data.Time))
			}
		}
	default:
		return "", false
	}
	return strings.Join(append([]string{"3x-ui · " + title}, fields...), "\n"), true
}
