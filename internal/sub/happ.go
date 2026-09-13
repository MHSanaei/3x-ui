package sub

import (
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var happUserAgentRegex = regexp.MustCompile(`(?i)\bhapp\b`)

// HappConfig holds all Happ client customization parameters.
type HappConfig struct {
	AutoDetect          bool
	ProviderId          string
	NewUrl              string
	FallbackUrl         string
	SubInfoColor        string
	SubInfoText         string
	SubInfoButtonText   string
	SubInfoButtonLink   string
	SubExpire           bool
	SubExpireButtonLink string
	NotificationExpire  bool
	NoLimit             bool
	AlwaysHwid          bool
	TunMode             string
	TunType             string
	ExcludeRoutes       string
	ExcludeApns         bool
	ColorProfile        string
	PingType            string
	AutoConnect         bool
	AutoConnectType     string
	PerAppMode          string
	PerAppList          string
}

// IsHappClient checks if the client user-agent identifies as Happ.
func IsHappClient(userAgent string) bool {
	return happUserAgentRegex.MatchString(userAgent)
}

func sanitizeHeaderValue(v string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(v), "\r", ""), "\n", "")
}

// ApplyHappHeaders sets standard and advanced Happ subscription headers.
func ApplyHappHeaders(c *gin.Context, cfg HappConfig, isHapp bool) {
	if c == nil || c.Writer == nil || !cfg.AutoDetect || !isHapp {
		return
	}
	if cfg.ProviderId != "" {
		c.Writer.Header().Set("ProviderID", strings.TrimSpace(cfg.ProviderId))
	}
	if cfg.NewUrl != "" {
		c.Writer.Header().Set("New-Url", strings.TrimSpace(cfg.NewUrl))
	}
	if cfg.FallbackUrl != "" {
		c.Writer.Header().Set("Fallback-Url", strings.TrimSpace(cfg.FallbackUrl))
	}
	if text := sanitizeHeaderValue(cfg.SubInfoText); text != "" {
		color := strings.TrimSpace(cfg.SubInfoColor)
		switch strings.ToLower(color) {
		case "primary", "info":
			color = "blue"
		case "success":
			color = "green"
		case "warning", "danger":
			color = "red"
		case "":
			color = "blue"
		}
		c.Writer.Header().Set("Sub-Info-Color", color)
		c.Writer.Header().Set("Sub-Info-Text", text)
		if btnText := sanitizeHeaderValue(cfg.SubInfoButtonText); btnText != "" {
			c.Writer.Header().Set("Sub-Info-Button-Text", btnText)
		}
		if btnLink := sanitizeHeaderValue(cfg.SubInfoButtonLink); btnLink != "" {
			c.Writer.Header().Set("Sub-Info-Button-Link", btnLink)
		}
	}
	if cfg.SubExpire {
		c.Writer.Header().Set("Sub-Expire", "1")
		if link := strings.TrimSpace(cfg.SubExpireButtonLink); link != "" {
			c.Writer.Header().Set("Sub-Expire-Button-Link", link)
		}
	}
	if cfg.NotificationExpire {
		c.Writer.Header().Set("Notification-Subs-Expire", "1")
	}
	if cfg.NoLimit {
		c.Writer.Header().Set("No-Limit-Enabled", "1")
	}
	if cfg.AlwaysHwid {
		c.Writer.Header().Set("Subscription-Always-Hwid-Enable", "1")
	}
	if cfg.TunMode != "" {
		c.Writer.Header().Set("Tun-Mode", cfg.TunMode)
	}
	if cfg.TunType != "" {
		c.Writer.Header().Set("Tun-Type", cfg.TunType)
	}
	if routes := strings.TrimSpace(cfg.ExcludeRoutes); routes != "" {
		c.Writer.Header().Set("Exclude-Routes", routes)
	}
	if cfg.ExcludeApns {
		c.Writer.Header().Set("Exclude-Apns-Enable", "true")
	}
	if profile := sanitizeHeaderValue(cfg.ColorProfile); profile != "" {
		c.Writer.Header().Set("Color-Profile", profile)
	}
	if ping := strings.TrimSpace(cfg.PingType); ping != "" {
		if strings.EqualFold(ping, "http") {
			ping = "proxy"
		}
		c.Writer.Header().Set("Ping-Type", ping)
	}
	if cfg.AutoConnect {
		c.Writer.Header().Set("Subscription-Autoconnect", "1")
		autoType := strings.TrimSpace(cfg.AutoConnectType)
		switch strings.ToLower(autoType) {
		case "fastest":
			autoType = "lowestdelay"
		case "last":
			autoType = "lastused"
		}
		if autoType != "" {
			c.Writer.Header().Set("Subscription-Autoconnect-Type", autoType)
		}
	}
	if mode := strings.TrimSpace(cfg.PerAppMode); mode != "" && mode != "off" {
		switch strings.ToLower(mode) {
		case "include":
			mode = "on"
		case "exclude":
			mode = "bypass"
		}
		c.Writer.Header().Set("Per-App-Proxy-Mode", mode)
		if list := strings.TrimSpace(cfg.PerAppList); list != "" {
			c.Writer.Header().Set("Per-App-Proxy-List", list)
		}
	}
}
