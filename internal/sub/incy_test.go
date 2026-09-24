package sub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func fullIncyConfig() IncyConfig {
	return IncyConfig{
		AutoDetect:                    true,
		ProfileDescription:            "Fast and stable network",
		SortOrder:                     "ping",
		SupportEmail:                  "support@example.com",
		AnnounceUrl:                   "https://t.me/incy_news",
		PremiumUrl:                    "https://example.com/buy",
		BannerText:                    "Summer sale",
		BannerButtonText:              "Buy now",
		BannerButtonUrl:               "https://example.com/sale",
		BannerBgColor:                 "#E53E3E",
		BannerButtonColor:             "#38A169",
		HideUrl:                       "1",
		HideCheck:                     "true",
		NoLimitEnabled:                "0",
		PerAppProxyEnable:             "1",
		PerAppProxyMode:               "bypass",
		PerAppProxyList:               "com.google.chrome,org.telegram.messenger",
		FragmentationEnable:           "1",
		FragmentationLength:           "10-30",
		FragmentationInterval:         "20-40",
		FragmentationPackets:          "tlshello",
		NoisesEnable:                  "1",
		NoisesType:                    "rand",
		NoisesPacket:                  "10-20",
		NoisesDelay:                   "10-50",
		ServerAddressResolveEnable:    "1",
		ServerAddressResolveDnsDomain: "https://common.dot.dns.yandex.net/dns-query",
		ServerAddressResolveDnsIp:     "77.88.8.8",
	}
}

func applyIncyToHeaders(t *testing.T, cfg IncyConfig, userAgent string) http.Header {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/sub/test", nil)
	if userAgent != "" {
		ctx.Request.Header.Set("User-Agent", userAgent)
	}
	ApplyIncyHeaders(ctx, cfg, cfg.AutoDetect && IsIncyClient(ctx.GetHeader("User-Agent")))
	return recorder.Header()
}

func TestIsIncyClient(t *testing.T) {
	for _, tc := range []struct {
		userAgent string
		want      bool
	}{
		{"INCY/1.0.0/ios", true},
		{"incy/2.4/android", true},
		{"Mozilla/5.0 (Linux; Android 14) INCY/1.2.3/android", true},
		{"Happ/1.2.0 (iPhone; iOS 17.5)", false},
		{"v2rayNG/1.8.5", false},
		{"", false},
	} {
		if got := IsIncyClient(tc.userAgent); got != tc.want {
			t.Errorf("IsIncyClient(%q) = %v, want %v", tc.userAgent, got, tc.want)
		}
	}
}

func TestApplyIncyHeaders_AllDocumentedHeaders(t *testing.T) {
	cfg := fullIncyConfig()
	h := applyIncyToHeaders(t, cfg, "INCY/1.0.0/android")

	want := map[string]string{
		"Profile-Description":               "Fast and stable network",
		"Sort-Order":                        "ping",
		"Support-Email":                     "support@example.com",
		"Announce-Url":                      "https://t.me/incy_news",
		"Premium-Url":                       "https://example.com/buy",
		"Banner-Text":                       "Summer sale",
		"Banner-Button-Text":                "Buy now",
		"Banner-Button-Url":                 "https://example.com/sale",
		"Banner-Bg-Color":                   "#E53E3E",
		"Banner-Button-Color":               "#38A169",
		"Hide-Url":                          "1",
		"Hide-Check":                        "1",
		"No-Limit-Enabled":                  "0",
		"Per-App-Proxy-Enable":              "1",
		"Per-App-Proxy-Mode":                "bypass",
		"Per-App-Proxy-List":                "com.google.chrome,org.telegram.messenger",
		"Fragmentation-Enable":              "1",
		"Fragmentation-Length":              "10-30",
		"Fragmentation-Interval":            "20-40",
		"Fragmentation-Packets":             "tlshello",
		"Noises-Enable":                     "1",
		"Noises-Type":                       "rand",
		"Noises-Packet":                     "10-20",
		"Noises-Delay":                      "10-50",
		"Server-Address-Resolve-Enable":     "1",
		"Server-Address-Resolve-Dns-Domain": "https://common.dot.dns.yandex.net/dns-query",
		"Server-Address-Resolve-Dns-Ip":     "77.88.8.8",
	}
	for name, value := range want {
		if got := h.Get(name); got != value {
			t.Errorf("header %s = %q, want %q", name, got, value)
		}
	}
}

func TestApplyIncyHeaders_SkipsUnsetValues(t *testing.T) {
	cfg := IncyConfig{AutoDetect: true}
	cfg.HideUrl = ""
	cfg.SortOrder = ""
	h := applyIncyToHeaders(t, cfg, "INCY/1.0.0/ios")

	for _, name := range []string{"Hide-Url", "Sort-Order", "Fragmentation-Enable", "Banner-Text"} {
		if got := h.Get(name); got != "" {
			t.Errorf("unset header %s = %q, want omitted", name, got)
		}
	}
}

func TestApplyIncyHeaders_NotAppliedWithoutIncyClient(t *testing.T) {
	cfg := fullIncyConfig()

	for _, userAgent := range []string{"Happ/1.2.0 (iPhone)", "v2rayNG/1.8.5", ""} {
		h := applyIncyToHeaders(t, cfg, userAgent)
		if got := h.Get("Hide-Url"); got != "" {
			t.Errorf("user-agent %q: Hide-Url = %q, want omitted", userAgent, got)
		}
	}
}

func TestApplyIncyHeaders_NotAppliedWhenAutoDetectOff(t *testing.T) {
	cfg := fullIncyConfig()
	cfg.AutoDetect = false

	h := applyIncyToHeaders(t, cfg, "INCY/1.0.0/android")
	if got := h.Get("Hide-Url"); got != "" {
		t.Errorf("AutoDetect off: Hide-Url = %q, want omitted", got)
	}
}

func TestIncyHeaderText_Base64ForNonASCII(t *testing.T) {
	ascii := incyHeaderText("Plain banner")
	if ascii != "Plain banner" {
		t.Errorf("ASCII text = %q, want unchanged", ascii)
	}

	cyrillic := incyHeaderText("Здравствуйте")
	if !strings.HasPrefix(cyrillic, "base64:") {
		t.Fatalf("Cyrillic text = %q, want base64: prefix", cyrillic)
	}
	// The docs require base64 for anything outside the ASCII range, so the
	// raw value must not survive on the wire.
	if strings.Contains(cyrillic, "Здравствуйте") {
		t.Errorf("Cyrillic text = %q, want the raw value base64-encoded", cyrillic)
	}
}

func TestApplyIncyHeaders_RejectsUndocumentedValues(t *testing.T) {
	cfg := IncyConfig{
		AutoDetect:           true,
		SortOrder:            "alphabetical", // not none|ping|name
		PerAppProxyMode:      "include",      // Happ spelling, not bypass|proxy
		NoisesType:           "uuid",         // not rand|str|hex
		FragmentationLength:  "10..30",       // not min-max
		FragmentationPackets: "3",            // documented only as tlshello|1-3|1|all
		BannerBgColor:        "red",          // not #RRGGBB
		HideUrl:              "maybe",        // not 1|0
	}
	h := applyIncyToHeaders(t, cfg, "INCY/1.0.0/android")

	for _, name := range []string{
		"Sort-Order", "Per-App-Proxy-Mode", "Noises-Type",
		"Fragmentation-Length", "Fragmentation-Packets", "Banner-Bg-Color", "Hide-Url",
	} {
		if got := h.Get(name); got != "" {
			t.Errorf("undocumented value accepted for %s: %q", name, got)
		}
	}
}

func TestIncyExcludesHappOnlyHeaders(t *testing.T) {
	// Incy documents none of the Happ-only headers, so the Incy path must not
	// emit them even when the Happ config is populated.
	happCfg := HappConfig{
		AutoDetect:   true,
		ProviderId:   "pid-test",
		TunMode:      "gvisor",
		PingType:     "http",
		ColorProfile: `{"serverRowBackgroundColor":"#21003D67"}`,
	}
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/sub/test", nil)
	ctx.Request.Header.Set("User-Agent", "INCY/1.0.0/android")

	ApplyHappHeaders(ctx, happCfg, IsHappClient(ctx.GetHeader("User-Agent")))
	ApplyIncyHeaders(ctx, fullIncyConfig(), IsIncyClient(ctx.GetHeader("User-Agent")))

	for _, name := range []string{"ProviderID", "Tun-Mode", "Ping-Type", "Color-Profile"} {
		if got := recorder.Header().Get(name); got != "" {
			t.Errorf("Happ-only header %s leaked to an Incy client: %q", name, got)
		}
	}
}
