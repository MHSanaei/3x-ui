package tgbot

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
)

var cachedHostname string

func getHostname() string {
	if cachedHostname != "" {
		return cachedHostname
	}
	h, err := os.Hostname()
	if err != nil {
		cachedHostname = "unknown"
	} else {
		cachedHostname = h
	}
	return cachedHostname
}

var tgEventLimiter = eventbus.NewRateLimiter(1 * time.Minute)

// HandleEvent is the eventbus subscriber callback. It formats incoming events
// as Telegram messages and sends them to all admin chats.
func (t *Tgbot) HandleEvent(e eventbus.Event) {
	if !t.isEventEnabled(e.Type) {
		return
	}
	if e.Type != eventbus.EventLoginAttempt {
		if !tgEventLimiter.Allow(e.Type, e.Source) {
			return
		}
	}
	msg := t.formatEventMessage(e)
	if msg != "" {
		t.SendMsgToTgbotAdmins(msg)
	}
}

func (t *Tgbot) isEventEnabled(eventType eventbus.EventType) bool {
	events, err := t.settingService.GetTgEnabledEvents()
	if err != nil || events == "" {
		return false
	}
	for e := range strings.SplitSeq(events, ",") {
		if strings.TrimSpace(e) == string(eventType) {
			return true
		}
	}
	return false
}

func (t *Tgbot) formatEventMessage(e eventbus.Event) string {
	host := getHostname()
	header := fmt.Sprintf("<b>📡 %s</b>\n", host)

	switch e.Type {
	case eventbus.EventOutboundDown:
		msg := header + t.I18nBot("tgbot.messages.eventOutboundDown",
			"Tag=="+e.Source)
		if data, ok := e.Data.(*eventbus.OutboundHealthData); ok {
			if data.Error != "" {
				msg += "\n" + t.I18nBot("tgbot.messages.eventErrorDetail",
					"Error=="+data.Error)
			}
			if data.Delay > 0 {
				msg += "\n" + t.I18nBot("tgbot.messages.eventDelayDetail",
					"Delay=="+fmt.Sprintf("%d", data.Delay))
			}
		}
		return msg

	case eventbus.EventOutboundUp:
		msg := header + t.I18nBot("tgbot.messages.eventOutboundUp",
			"Tag=="+e.Source)
		if data, ok := e.Data.(*eventbus.OutboundHealthData); ok && data.Delay > 0 {
			msg += "\n" + t.I18nBot("tgbot.messages.eventDelayDetail",
				"Delay=="+fmt.Sprintf("%d", data.Delay))
		}
		return msg

	case eventbus.EventXrayCrash:
		errStr := ""
		if e.Data != nil {
			errStr = fmt.Sprint(e.Data)
		}
		msg := header + "🔥 " + t.I18nBot("tgbot.messages.eventXrayCrash")
		if errStr != "" {
			msg += "\n" + t.I18nBot("tgbot.messages.eventXrayCrashError", "Error=="+errStr)
		}
		return msg

	case eventbus.EventNodeDown:
		msg := header + "🔴 " + t.I18nBot("tgbot.messages.eventNodeDown", "Name=="+e.Source)
		if data, ok := e.Data.(*eventbus.NodeHealthData); ok && data.XrayError != "" {
			msg += "\n" + t.I18nBot("tgbot.messages.eventErrorDetail", "Error=="+data.XrayError)
		}
		return msg

	case eventbus.EventNodeUp:
		msg := header + "🟢 " + t.I18nBot("tgbot.messages.eventNodeUp", "Name=="+e.Source)
		if data, ok := e.Data.(*eventbus.NodeHealthData); ok && data.LatencyMs > 0 {
			msg += "\n" + t.I18nBot("tgbot.messages.eventDelayDetail", "Delay=="+fmt.Sprintf("%d", data.LatencyMs))
		}
		return msg

	case eventbus.EventCPUHigh:
		if data, ok := e.Data.(*eventbus.SystemMetricData); ok {
			tgCpu, err := t.settingService.GetTgCpu()
			if err != nil || tgCpu <= 0 || data.Percent <= float64(tgCpu) {
				return ""
			}
			return header + "🔴 " + t.I18nBot("tgbot.messages.cpuThreshold",
				"Percent=="+strconv.FormatFloat(data.Percent, 'f', 2, 64),
				"Threshold=="+strconv.Itoa(tgCpu))
		}
		return ""

	case eventbus.EventLoginAttempt:
		if data, ok := e.Data.(*eventbus.LoginEventData); ok {
			if data.Status == "success" {
				msg := t.I18nBot("tgbot.messages.loginSuccess")
				msg += t.I18nBot("tgbot.messages.hostname", "Hostname=="+host)
				msg += t.I18nBot("tgbot.messages.username", "Username=="+data.Username)
				msg += t.I18nBot("tgbot.messages.ip", "IP=="+data.IP)
				msg += t.I18nBot("tgbot.messages.time", "Time=="+data.Time)
				return msg
			}
			msg := t.I18nBot("tgbot.messages.loginFailed")
			msg += t.I18nBot("tgbot.messages.hostname", "Hostname=="+host)
			if data.Reason != "" {
				msg += t.I18nBot("tgbot.messages.reason", "Reason=="+data.Reason)
			}
			msg += t.I18nBot("tgbot.messages.username", "Username=="+data.Username)
			msg += t.I18nBot("tgbot.messages.ip", "IP=="+data.IP)
			msg += t.I18nBot("tgbot.messages.time", "Time=="+data.Time)
			return msg
		}
		return header + t.I18nBot("tgbot.messages.eventLoginFallback", "Source=="+e.Source)
	}

	return ""
}
