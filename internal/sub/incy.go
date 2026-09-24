package sub

import (
	"encoding/base64"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// INCY clients identify themselves as INCY/<version>/<platform>.
var incyUserAgentRegex = regexp.MustCompile(`(?i)\bincy\b`)

var (
	incyRangeRegex = regexp.MustCompile(`^\d+(-\d+)?$`)
	incyHexRegex   = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)
	// Incy documents fragmentation-packets as tlshello | 1-3 | 1 | all.
	incyPacketsRangeRegex = regexp.MustCompile(`^\d+-\d+$`)
)

// IncyConfig holds the Incy app-management headers the panel can push.
// A "" field omits its header, so an untouched panel never overrides the app.
type IncyConfig struct {
	AutoDetect bool

	ProfileDescription string
	SortOrder          string
	SupportEmail       string
	AnnounceUrl        string
	PremiumUrl         string

	BannerText        string
	BannerButtonText  string
	BannerButtonUrl   string
	BannerBgColor     string
	BannerButtonColor string

	HideUrl        string
	HideCheck      string
	NoLimitEnabled string

	PerAppProxyEnable string
	PerAppProxyMode   string
	PerAppProxyList   string

	FragmentationEnable   string
	FragmentationLength   string
	FragmentationInterval string
	FragmentationPackets  string

	NoisesEnable string
	NoisesType   string
	NoisesPacket string
	NoisesDelay  string

	ServerAddressResolveEnable    string
	ServerAddressResolveDnsDomain string
	ServerAddressResolveDnsIp     string
}

// IsIncyClient checks if the client user-agent identifies as INCY.
func IsIncyClient(userAgent string) bool {
	return incyUserAgentRegex.MatchString(userAgent)
}

// incyOnOff maps a switch setting to the documented `1`/`0` literal. An
// unrecognised or unset value stays empty so the header is omitted.
func incyOnOff(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return "1"
	case "0", "false", "no", "off":
		return "0"
	}
	return ""
}

// incyEnum passes through only a documented literal from a fixed value set.
func incyEnum(v string, allowed ...string) string {
	value := strings.ToLower(strings.TrimSpace(v))
	for _, a := range allowed {
		if value == a {
			return value
		}
	}
	return ""
}

// incyMatch returns the trimmed value only when it matches a documented shape.
func incyMatch(v string, re *regexp.Regexp) string {
	value := strings.TrimSpace(v)
	if re.MatchString(value) {
		return value
	}
	return ""
}

func incyASCII(v string) bool {
	for i := 0; i < len(v); i++ {
		if v[i] > 0x7e || v[i] < 0x20 {
			return false
		}
	}
	return true
}

// incyHeaderText base64-wraps non-ASCII text because the docs require
// `base64:<...>` for anything outside the ASCII range (Cyrillic, CJK, emoji).
func incyHeaderText(v string) string {
	text := sanitizeHeaderValue(v)
	if text == "" || incyASCII(text) {
		return text
	}
	return "base64:" + base64.StdEncoding.EncodeToString([]byte(text))
}

// ApplyIncyHeaders sets the Incy app-management headers from
// https://docs.incy.cc/en/app-management/ (matched case-insensitively).
func ApplyIncyHeaders(c *gin.Context, cfg IncyConfig, isIncy bool) {
	if c == nil || c.Writer == nil || !cfg.AutoDetect || !isIncy {
		return
	}
	h := c.Writer.Header()
	set := func(name, value string) {
		if value != "" {
			h.Set(name, value)
		}
	}

	set("Profile-Description", incyHeaderText(cfg.ProfileDescription))
	set("Sort-Order", incyEnum(cfg.SortOrder, "none", "ping", "name"))
	set("Support-Email", sanitizeHeaderValue(cfg.SupportEmail))
	set("Announce-Url", sanitizeHeaderValue(cfg.AnnounceUrl))
	set("Premium-Url", sanitizeHeaderValue(cfg.PremiumUrl))

	set("Banner-Text", incyHeaderText(cfg.BannerText))
	set("Banner-Button-Text", incyHeaderText(cfg.BannerButtonText))
	set("Banner-Button-Url", sanitizeHeaderValue(cfg.BannerButtonUrl))
	set("Banner-Bg-Color", incyMatch(cfg.BannerBgColor, incyHexRegex))
	set("Banner-Button-Color", incyMatch(cfg.BannerButtonColor, incyHexRegex))

	set("Hide-Url", incyOnOff(cfg.HideUrl))
	set("Hide-Check", incyOnOff(cfg.HideCheck))
	set("No-Limit-Enabled", incyOnOff(cfg.NoLimitEnabled))

	set("Per-App-Proxy-Enable", incyOnOff(cfg.PerAppProxyEnable))
	set("Per-App-Proxy-Mode", incyEnum(cfg.PerAppProxyMode, "bypass", "proxy"))
	set("Per-App-Proxy-List", sanitizeHeaderValue(cfg.PerAppProxyList))

	set("Fragmentation-Enable", incyOnOff(cfg.FragmentationEnable))
	set("Fragmentation-Length", incyMatch(cfg.FragmentationLength, incyRangeRegex))
	set("Fragmentation-Interval", incyMatch(cfg.FragmentationInterval, incyRangeRegex))
	if packets := strings.ToLower(strings.TrimSpace(cfg.FragmentationPackets)); packets != "" {
		if packets == "tlshello" || packets == "all" || packets == "1" || incyPacketsRangeRegex.MatchString(packets) {
			set("Fragmentation-Packets", packets)
		}
	}

	set("Noises-Enable", incyOnOff(cfg.NoisesEnable))
	set("Noises-Type", incyEnum(cfg.NoisesType, "rand", "str", "hex"))
	set("Noises-Packet", sanitizeHeaderValue(cfg.NoisesPacket))
	set("Noises-Delay", incyMatch(cfg.NoisesDelay, incyRangeRegex))

	set("Server-Address-Resolve-Enable", incyOnOff(cfg.ServerAddressResolveEnable))
	set("Server-Address-Resolve-Dns-Domain", sanitizeHeaderValue(cfg.ServerAddressResolveDnsDomain))
	set("Server-Address-Resolve-Dns-Ip", sanitizeHeaderValue(cfg.ServerAddressResolveDnsIp))
}
