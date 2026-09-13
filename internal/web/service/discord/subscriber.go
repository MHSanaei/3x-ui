package discord

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Subscriber handles event bus messages and forwards them to Discord.
type Subscriber struct {
	settingService service.SettingService
	discordService *DiscordService
	limiter        *eventbus.RateLimiter
}

// NewSubscriber creates a new Discord event subscriber.
func NewSubscriber(settingService service.SettingService, discordService *DiscordService) *Subscriber {
	return &Subscriber{
		settingService: settingService,
		discordService: discordService,
		limiter:        eventbus.NewRateLimiter(1 * time.Minute),
	}
}

// HandleEvent is the eventbus subscriber callback.
func (s *Subscriber) HandleEvent(e eventbus.Event) {
	if s.discordService == nil {
		return
	}
	if on, err := s.settingService.GetDiscordBotEnable(); err != nil || !on {
		return
	}
	if !s.isEventEnabled(e.Type) {
		return
	}
	embed, ok := s.FormatEmbed(e)
	if !ok {
		return
	}
	if e.Type != eventbus.EventLoginAttempt {
		if !s.limiter.Allow(e.Type, e.Source) {
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.discordService.SendEmbed(ctx, embed); err != nil {
		logger.Warning("discord subscriber: send failed:", err)
	}
}

func (s *Subscriber) isEventEnabled(t eventbus.EventType) bool {
	events, err := s.settingService.GetDiscordEnabledEvents()
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

func truncateRunes(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	if maxRunes <= 3 {
		return string(r[:maxRunes])
	}
	return string(r[:maxRunes-3]) + "..."
}

func cleanField(name, value string, inline bool) EmbedField {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "-"
	} else {
		name = truncateRunes(name, 256)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		value = "-"
	} else {
		value = truncateRunes(value, 1024)
	}
	return EmbedField{
		Name:   name,
		Value:  value,
		Inline: inline,
	}
}

// FormatEmbed converts an eventbus.Event into a Discord Embed.
// Returns false if the event should not produce a message (e.g. thresholds not exceeded).
func (s *Subscriber) FormatEmbed(e eventbus.Event) (Embed, bool) {
	h, _ := os.Hostname()
	if h == "" {
		h = "unknown"
	}
	var ts string
	if e.Timestamp.IsZero() {
		ts = time.Now().UTC().Format(time.RFC3339)
	} else {
		ts = e.Timestamp.UTC().Format(time.RFC3339)
	}

	footer := &EmbedFooter{
		Text: truncateRunes("3x-ui • "+h, 2048),
	}
	tr := translator(s.settingService)

	switch e.Type {
	case eventbus.EventOutboundDown:
		fields := []EmbedField{
			cleanField(tr("discord.fields.outbound"), e.Source, true),
		}
		var data *eventbus.OutboundHealthData
		switch d := e.Data.(type) {
		case *eventbus.OutboundHealthData:
			data = d
		case eventbus.OutboundHealthData:
			data = &d
		}
		if data != nil {
			if data.Error != "" {
				fields = append(fields, cleanField(tr("discord.fields.error"), data.Error, false))
			}
			if data.Delay > 0 {
				fields = append(fields, cleanField(tr("discord.fields.delay"), fmt.Sprintf("%dms", data.Delay), true))
			}
		}
		return Embed{
			Title:     tr("discord.alerts.outboundDown"),
			Color:     ColorRed,
			Timestamp: ts,
			Fields:    fields,
			Footer:    footer,
		}, true

	case eventbus.EventOutboundUp:
		fields := []EmbedField{
			cleanField(tr("discord.fields.outbound"), e.Source, true),
		}
		var data *eventbus.OutboundHealthData
		switch d := e.Data.(type) {
		case *eventbus.OutboundHealthData:
			data = d
		case eventbus.OutboundHealthData:
			data = &d
		}
		if data != nil && data.Delay > 0 {
			fields = append(fields, cleanField(tr("discord.fields.delay"), fmt.Sprintf("%dms", data.Delay), true))
		}
		return Embed{
			Title:     tr("discord.alerts.outboundUp"),
			Color:     ColorGreen,
			Timestamp: ts,
			Fields:    fields,
			Footer:    footer,
		}, true

	case eventbus.EventNodeDown:
		fields := []EmbedField{
			cleanField(tr("discord.fields.node"), e.Source, true),
		}
		var data *eventbus.NodeHealthData
		switch d := e.Data.(type) {
		case *eventbus.NodeHealthData:
			data = d
		case eventbus.NodeHealthData:
			data = &d
		}
		if data != nil && data.XrayError != "" {
			fields = append(fields, cleanField(tr("discord.fields.error"), data.XrayError, false))
		}
		return Embed{
			Title:     tr("discord.alerts.nodeDown"),
			Color:     ColorRed,
			Timestamp: ts,
			Fields:    fields,
			Footer:    footer,
		}, true

	case eventbus.EventNodeUp:
		fields := []EmbedField{
			cleanField(tr("discord.fields.node"), e.Source, true),
		}
		var data *eventbus.NodeHealthData
		switch d := e.Data.(type) {
		case *eventbus.NodeHealthData:
			data = d
		case eventbus.NodeHealthData:
			data = &d
		}
		if data != nil && data.LatencyMs > 0 {
			fields = append(fields, cleanField(tr("discord.fields.delay"), fmt.Sprintf("%dms", data.LatencyMs), true))
		}
		return Embed{
			Title:     tr("discord.alerts.nodeUp"),
			Color:     ColorGreen,
			Timestamp: ts,
			Fields:    fields,
			Footer:    footer,
		}, true

	case eventbus.EventXrayCrash:
		var fields []EmbedField
		if e.Data != nil {
			fields = append(fields, cleanField(tr("discord.fields.error"), fmt.Sprint(e.Data), false))
		}
		return Embed{
			Title:     tr("discord.alerts.xrayCrash"),
			Color:     ColorRed,
			Timestamp: ts,
			Fields:    fields,
			Footer:    footer,
		}, true

	case eventbus.EventCPUHigh:
		var data *eventbus.SystemMetricData
		switch d := e.Data.(type) {
		case *eventbus.SystemMetricData:
			data = d
		case eventbus.SystemMetricData:
			data = &d
		}
		if data != nil {
			discordCpu, err := s.settingService.GetDiscordCpu()
			if err != nil || discordCpu <= 0 || data.Percent <= float64(discordCpu) {
				return Embed{}, false
			}
			fields := []EmbedField{
				cleanField(tr("usage"), fmt.Sprintf("%.2f%%", data.Percent), true),
				cleanField(tr("discord.fields.threshold"), fmt.Sprintf("%d%%", discordCpu), true),
			}
			return Embed{
				Title:     tr("discord.alerts.cpuHigh"),
				Color:     ColorOrange,
				Timestamp: ts,
				Fields:    fields,
				Footer:    footer,
			}, true
		}
		return Embed{}, false

	case eventbus.EventMemoryHigh:
		var data *eventbus.SystemMetricData
		switch d := e.Data.(type) {
		case *eventbus.SystemMetricData:
			data = d
		case eventbus.SystemMetricData:
			data = &d
		}
		if data != nil {
			discordMem, err := s.settingService.GetDiscordMemory()
			if err != nil || discordMem <= 0 || data.Percent <= float64(discordMem) {
				return Embed{}, false
			}
			fields := []EmbedField{
				cleanField(tr("usage"), fmt.Sprintf("%.2f%%", data.Percent), true),
				cleanField(tr("discord.fields.threshold"), fmt.Sprintf("%d%%", discordMem), true),
			}
			return Embed{
				Title:     tr("discord.alerts.memoryHigh"),
				Color:     ColorOrange,
				Timestamp: ts,
				Fields:    fields,
				Footer:    footer,
			}, true
		}
		return Embed{}, false

	case eventbus.EventLoginAttempt:
		var data *eventbus.LoginEventData
		switch d := e.Data.(type) {
		case *eventbus.LoginEventData:
			data = d
		case eventbus.LoginEventData:
			data = &d
		}
		if data != nil {
			if data.Status == "success" {
				fields := []EmbedField{
					cleanField(tr("username"), data.Username, true),
					cleanField("IP", data.IP, true),
				}
				if data.Time != "" {
					fields = append(fields, cleanField(tr("discord.fields.time"), data.Time, true))
				}
				return Embed{
					Title:     tr("discord.alerts.loginSuccess"),
					Color:     ColorGreen,
					Timestamp: ts,
					Fields:    fields,
					Footer:    footer,
				}, true
			}
			fields := []EmbedField{
				cleanField(tr("username"), data.Username, true),
				cleanField("IP", data.IP, true),
			}
			if data.Reason != "" {
				fields = append(fields, cleanField(tr("discord.fields.reason"), data.Reason, false))
			}
			if data.Time != "" {
				fields = append(fields, cleanField(tr("discord.fields.time"), data.Time, true))
			}
			return Embed{
				Title:     tr("discord.alerts.loginFailed"),
				Color:     ColorRed,
				Timestamp: ts,
				Fields:    fields,
				Footer:    footer,
			}, true
		}
		fields := []EmbedField{
			cleanField(tr("discord.fields.source"), e.Source, true),
		}
		return Embed{
			Title:     tr("discord.alerts.loginFailed"),
			Color:     ColorRed,
			Timestamp: ts,
			Fields:    fields,
			Footer:    footer,
		}, true
	}

	return Embed{}, false
}
