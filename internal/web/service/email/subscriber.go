package email

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Subscriber handles event bus messages and sends email notifications.
type Subscriber struct {
	settingService service.SettingService
	emailService   *EmailService
	limiter        *eventbus.RateLimiter
}

// NewSubscriber creates a new email event subscriber.
func NewSubscriber(settingService service.SettingService, emailService *EmailService) *Subscriber {
	return &Subscriber{
		settingService: settingService,
		emailService:   emailService,
		limiter:        eventbus.NewRateLimiter(1 * time.Minute),
	}
}

// HandleEvent is the eventbus subscriber callback.
func (s *Subscriber) HandleEvent(e eventbus.Event) {
	if !s.isEventEnabled(e.Type) {
		return
	}
	if e.Type != eventbus.EventLoginAttempt {
		if !s.limiter.Allow(e.Type, e.Source) {
			return
		}
	}
	subject, body := s.formatMessage(e)
	if subject == "" {
		return
	}
	if err := s.emailService.Send(subject, body); err != nil {
		logger.Warning("email subscriber: send failed:", err)
	}
}

func (s *Subscriber) isEventEnabled(t eventbus.EventType) bool {
	events, err := s.settingService.GetSmtpEnabledEvents()
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

func i18n(key string, params ...string) string {
	return locale.I18n(locale.Bot, key, params...)
}

func (s *Subscriber) formatMessage(e eventbus.Event) (subject, body string) {
	h, _ := hostname()
	host := h
	ts := e.Timestamp.Format("2006-01-02 15:04:05")

	wrap := func(title, content string) string {
		// Strip newlines from title to prevent broken HTML
		title = strings.ReplaceAll(title, "\r\n", "")
		title = strings.ReplaceAll(title, "\n", "")
		return fmt.Sprintf(`<html><body style="font-family:monospace;font-size:14px;color:#333">
<h2 style="color:#555;border-bottom:1px solid #ddd;padding-bottom:8px">📡 %s %s</h2>
%s
<p style="color:#999;font-size:12px;margin-top:20px">%s</p>
</body></html>`, host, title, content, i18n("tgbot.messages.time", "Time=="+ts))
	}

	kv := func(key, val string) string {
		return fmt.Sprintf("<p><b>%s:</b> %s</p>", key, val)
	}

	switch e.Type {
	case eventbus.EventOutboundDown:
		subject = host + " " + i18n("tgbot.messages.eventOutboundDown", "Tag=="+e.Source)
		content := kv(i18n("email.labelStatus"), `<span style="color:red">`+i18n("email.statusDown")+`</span>`)
		content += kv(i18n("email.labelOutbound"), e.Source)
		if data, ok := e.Data.(*eventbus.OutboundHealthData); ok {
			if data.Error != "" {
				content += kv(i18n("email.labelError"), data.Error)
			}
			if data.Delay > 0 {
				content += kv(i18n("email.labelDelay"), fmt.Sprintf("%dms", data.Delay))
			}
		}
		body = wrap(i18n("tgbot.messages.eventOutboundDown", "Tag=="+e.Source), content)

	case eventbus.EventOutboundUp:
		subject = host + " " + i18n("tgbot.messages.eventOutboundUp", "Tag=="+e.Source)
		content := kv(i18n("email.labelStatus"), `<span style="color:green">`+i18n("email.statusUp")+`</span>`)
		content += kv(i18n("email.labelOutbound"), e.Source)
		if data, ok := e.Data.(*eventbus.OutboundHealthData); ok && data.Delay > 0 {
			content += kv(i18n("email.labelDelay"), fmt.Sprintf("%dms", data.Delay))
		}
		body = wrap(i18n("tgbot.messages.eventOutboundUp", "Tag=="+e.Source), content)

	case eventbus.EventXrayCrash:
		subject = host + " " + i18n("tgbot.messages.eventXrayCrash")
		content := kv(i18n("email.labelStatus"), `<span style="color:red">`+i18n("email.statusCrashed")+`</span>`)
		if e.Data != nil {
			content += kv(i18n("email.labelError"), fmt.Sprint(e.Data))
		}
		body = wrap(i18n("tgbot.messages.eventXrayCrash"), content)

	case eventbus.EventNodeDown:
		subject = host + " " + i18n("tgbot.messages.eventNodeDown", "Name=="+e.Source)
		content := kv(i18n("email.labelStatus"), `<span style="color:red">`+i18n("email.statusDown")+`</span>`)
		content += kv(i18n("email.labelNode"), e.Source)
		if data, ok := e.Data.(*eventbus.NodeHealthData); ok && data.XrayError != "" {
			content += kv(i18n("email.labelError"), data.XrayError)
		}
		body = wrap(i18n("tgbot.messages.eventNodeDown", "Name=="+e.Source), content)

	case eventbus.EventNodeUp:
		subject = host + " " + i18n("tgbot.messages.eventNodeUp", "Name=="+e.Source)
		content := kv(i18n("email.labelStatus"), `<span style="color:green">`+i18n("email.statusUp")+`</span>`)
		content += kv(i18n("email.labelNode"), e.Source)
		if data, ok := e.Data.(*eventbus.NodeHealthData); ok && data.LatencyMs > 0 {
			content += kv(i18n("email.labelDelay"), fmt.Sprintf("%dms", data.LatencyMs))
		}
		body = wrap(i18n("tgbot.messages.eventNodeUp", "Name=="+e.Source), content)

	case eventbus.EventCPUHigh:
		if data, ok := e.Data.(*eventbus.SystemMetricData); ok {
			smtpCpu, err := s.settingService.GetSmtpCpu()
			if err != nil || smtpCpu <= 0 || data.Percent <= float64(smtpCpu) {
				return
			}
			subject = host + " " + i18n("tgbot.messages.cpuThreshold",
				"Percent=="+strconv.FormatFloat(data.Percent, 'f', 2, 64),
				"Threshold=="+fmt.Sprintf("%d", smtpCpu))
			content := kv(i18n("email.labelStatus"), `<span style="color:orange">`+i18n("email.statusHigh")+`</span>`)
			body = wrap(subject, content)
		}

	case eventbus.EventLoginAttempt:
		if data, ok := e.Data.(*eventbus.LoginEventData); ok {
			if data.Status == "success" {
				subject = host + " " + i18n("tgbot.messages.loginSuccess")
				content := kv(i18n("email.labelStatus"), `<span style="color:green">`+i18n("email.statusSuccess")+`</span>`)
				content += kv(i18n("email.labelUsername"), data.Username)
				content += kv(i18n("email.labelIP"), data.IP)
				body = wrap(i18n("tgbot.messages.loginSuccess"), content)
			} else {
				subject = host + " " + i18n("tgbot.messages.loginFailed")
				content := kv(i18n("email.labelStatus"), `<span style="color:red">`+i18n("email.statusFailed")+`</span>`)
				if data.Reason != "" {
					content += kv(i18n("email.labelReason"), data.Reason)
				}
				content += kv(i18n("email.labelUsername"), data.Username)
				content += kv(i18n("email.labelIP"), data.IP)
				body = wrap(i18n("tgbot.messages.loginFailed"), content)
			}
		} else {
			subject = host + " " + i18n("tgbot.messages.loginFailed")
			content := kv(i18n("email.labelStatus"), `<span style="color:red">`+i18n("email.statusFailed")+`</span>`)
			content += kv(i18n("email.labelSource"), e.Source)
			body = wrap(i18n("tgbot.messages.loginFailed"), content)
		}
	}

	return
}

func hostname() (string, error) {
	return os.Hostname()
}
