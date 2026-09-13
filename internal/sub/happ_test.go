package sub

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNormalizeHappRouting_Off(t *testing.T) {
	got, err := normalizeHappRouting([]byte("happ://routing/off"))
	if err != nil {
		t.Fatalf("normalizeHappRouting(off) error: %v", err)
	}
	if got != "happ://routing/off" {
		t.Fatalf("normalizeHappRouting(off) = %q, want happ://routing/off", got)
	}
}

func TestApplyCommonHeaders_HappClientHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := HappConfig{
		AutoDetect:          true,
		ProviderId:          "pid-test-123",
		NewUrl:              "https://new.example.com/sub",
		FallbackUrl:         "https://backup.example.com/sub",
		SubInfoColor:        "primary",
		SubInfoText:         "Welcome to VIP Network",
		SubInfoButtonText:   "Telegram",
		SubInfoButtonLink:   "https://t.me/example",
		SubExpire:           true,
		SubExpireButtonLink: "https://renew.example.com",
		NotificationExpire:  true,
		NoLimit:             true,
		AlwaysHwid:          true,
		TunMode:             "gvisor",
		TunType:             "singbox",
		ExcludeRoutes:       "192.168.1.0/24, 10.0.0.0/8",
		ExcludeApns:         true,
		ColorProfile:        "{\"serverRowBackgroundColor\":\n\"#21003D67\"}",
		PingType:            "http",
		AutoConnect:         true,
		AutoConnectType:     "fastest",
		PerAppMode:          "include",
		PerAppList:          "com.google.chrome,com.meta.instagram",
	}

	controller := &SUBController{happConfig: cfg}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/sub/test", nil)
	ctx.Request.Header.Set("User-Agent", "Happ/1.2.0 (iPhone; iOS 17.5)")

	controller.ApplyCommonHeaders(ctx, "upload=0; download=100; total=1000; expire=1800000000", "12", "MyTitle", "", "", "", false, "happ://routing/onadd/existing-rules", false)

	h := recorder.Header()
	if h.Get("Routing-Enable") != "0" {
		t.Fatalf("Routing-Enable = %q, want 0 for Happ with disabled routing", h.Get("Routing-Enable"))
	}
	if h.Get("Routing") != "happ://routing/onadd/existing-rules" {
		t.Fatalf("Routing = %q, want happ://routing/onadd/existing-rules for Happ with configured routing", h.Get("Routing"))
	}
	if h.Get("Hide-Settings") != "0" {
		t.Fatalf("Hide-Settings = %q, want 0 for Happ with disabled hideSettings", h.Get("Hide-Settings"))
	}
	if h.Get("ProviderID") != "pid-test-123" {
		t.Fatalf("ProviderID = %q, want pid-test-123", h.Get("ProviderID"))
	}
	if h.Get("New-Url") != "https://new.example.com/sub" {
		t.Fatalf("New-Url = %q", h.Get("New-Url"))
	}
	if h.Get("Fallback-Url") != "https://backup.example.com/sub" {
		t.Fatalf("Fallback-Url = %q", h.Get("Fallback-Url"))
	}
	if h.Get("Sub-Info-Color") != "blue" || h.Get("Sub-Info-Text") != "Welcome to VIP Network" {
		t.Fatalf("Sub-Info = %s / %s, want blue / Welcome to VIP Network", h.Get("Sub-Info-Color"), h.Get("Sub-Info-Text"))
	}
	if h.Get("Sub-Info-Button-Text") != "Telegram" || h.Get("Sub-Info-Button-Link") != "https://t.me/example" {
		t.Fatalf("Sub-Info button = %s / %s", h.Get("Sub-Info-Button-Text"), h.Get("Sub-Info-Button-Link"))
	}
	if h.Get("Sub-Expire") != "1" || h.Get("Sub-Expire-Button-Link") != "https://renew.example.com" {
		t.Fatalf("Sub-Expire = %s / %s", h.Get("Sub-Expire"), h.Get("Sub-Expire-Button-Link"))
	}
	if h.Get("Notification-Subs-Expire") != "1" {
		t.Fatalf("Notification-Subs-Expire = %q", h.Get("Notification-Subs-Expire"))
	}
	if h.Get("No-Limit-Enabled") != "1" {
		t.Fatalf("No-Limit-Enabled = %q", h.Get("No-Limit-Enabled"))
	}
	if h.Get("Subscription-Always-Hwid-Enable") != "1" {
		t.Fatalf("Subscription-Always-Hwid-Enable = %q", h.Get("Subscription-Always-Hwid-Enable"))
	}
	if h.Get("Tun-Mode") != "gvisor" || h.Get("Tun-Type") != "singbox" {
		t.Fatalf("Tun mode/type = %s / %s", h.Get("Tun-Mode"), h.Get("Tun-Type"))
	}
	if h.Get("Exclude-Routes") != "192.168.1.0/24, 10.0.0.0/8" || h.Get("Exclude-Apns-Enable") != "true" {
		t.Fatalf("Exclude routes/apns = %s / %s", h.Get("Exclude-Routes"), h.Get("Exclude-Apns-Enable"))
	}
	if wantProfile := "{\"serverRowBackgroundColor\":\"#21003D67\"}"; h.Get("Color-Profile") != wantProfile {
		t.Fatalf("Color-Profile = %q, want %q", h.Get("Color-Profile"), wantProfile)
	}
	if h.Get("Ping-Type") != "proxy" {
		t.Fatalf("Ping-Type = %q, want proxy for http alias", h.Get("Ping-Type"))
	}
	if h.Get("Subscription-Autoconnect") != "1" || h.Get("Subscription-Autoconnect-Type") != "lowestdelay" {
		t.Fatalf("Autoconnect = %s / %s, want 1 / lowestdelay for fastest alias", h.Get("Subscription-Autoconnect"), h.Get("Subscription-Autoconnect-Type"))
	}
	if h.Get("Per-App-Proxy-Mode") != "on" || h.Get("Per-App-Proxy-List") != "com.google.chrome,com.meta.instagram" {
		t.Fatalf("Per-App = %s / %s, want on / com.google.chrome,com.meta.instagram", h.Get("Per-App-Proxy-Mode"), h.Get("Per-App-Proxy-List"))
	}
}

func TestApplyCommonHeaders_HappRoutingOffDeeplink(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := HappConfig{AutoDetect: true}
	controller := &SUBController{happConfig: cfg}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/sub/test", nil)
	ctx.Request.Header.Set("User-Agent", "Happ/1.2.0 (iPhone)")

	// When rules is happ://routing/off, Routing-Enable is 0 and Routing is happ://routing/off
	controller.ApplyCommonHeaders(ctx, "", "", "Title", "", "", "", false, "happ://routing/off", false)

	h := recorder.Header()
	if h.Get("Routing-Enable") != "0" {
		t.Fatalf("Routing-Enable = %q, want 0 when rules is happ://routing/off", h.Get("Routing-Enable"))
	}
	if h.Get("Routing") != "happ://routing/off" {
		t.Fatalf("Routing = %q, want happ://routing/off", h.Get("Routing"))
	}
}

func TestApplyHappHeaders_Gating(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := HappConfig{
		AutoDetect:  true,
		ProviderId:  "pid-secret",
		SubInfoText: "Banner",
		TunMode:     "system",
	}

	t.Run("non-Happ User-Agent receives no headers", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/sub/test", nil)
		ctx.Request.Header.Set("User-Agent", "v2rayNG/1.8.5")

		controller := &SUBController{happConfig: cfg}
		controller.ApplyCommonHeaders(ctx, "", "", "Title", "", "", "", false, "", false)

		if got := recorder.Header().Get("ProviderID"); got != "" {
			t.Fatalf("ProviderID emitted to non-Happ client: %q", got)
		}
		if got := recorder.Header().Get("Sub-Info-Text"); got != "" {
			t.Fatalf("Sub-Info-Text emitted to non-Happ client: %q", got)
		}
		if got := recorder.Header().Get("Tun-Mode"); got != "" {
			t.Fatalf("Tun-Mode emitted to non-Happ client: %q", got)
		}
	})

	t.Run("AutoDetect disabled suppresses Happ headers", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/sub/test", nil)
		ctx.Request.Header.Set("User-Agent", "Happ/1.2.0 (Android)")

		disabledCfg := cfg
		disabledCfg.AutoDetect = false

		controller := &SUBController{happConfig: disabledCfg}
		controller.ApplyCommonHeaders(ctx, "", "", "Title", "", "", "", false, "", false)

		if got := recorder.Header().Get("ProviderID"); got != "" {
			t.Fatalf("ProviderID emitted when AutoDetect is false: %q", got)
		}
		if got := recorder.Header().Get("Sub-Info-Text"); got != "" {
			t.Fatalf("Sub-Info-Text emitted when AutoDetect is false: %q", got)
		}
		// happ.su documents routing-enable 0/false as "disables routing
		// globally", so it must stay behind the same opt-in as the rest.
		if got := recorder.Header().Get("Routing-Enable"); got != "" {
			t.Fatalf("Routing-Enable emitted when AutoDetect is false: %q", got)
		}
		if got := recorder.Header().Get("Hide-Settings"); got != "" {
			t.Fatalf("Hide-Settings emitted when AutoDetect is false: %q", got)
		}
		if got := recorder.Header().Get("Routing"); got != "" {
			t.Fatalf("Routing emitted when AutoDetect is false: %q", got)
		}
	})
}

func TestApplyHappHeaders_Aliases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		cfg        HappConfig
		wantHeader string
		wantValue  string
	}{
		{
			name:       "color warning maps to red",
			cfg:        HappConfig{AutoDetect: true, SubInfoText: "Alert", SubInfoColor: "warning"},
			wantHeader: "Sub-Info-Color",
			wantValue:  "red",
		},
		{
			name:       "color danger maps to red",
			cfg:        HappConfig{AutoDetect: true, SubInfoText: "Alert", SubInfoColor: "danger"},
			wantHeader: "Sub-Info-Color",
			wantValue:  "red",
		},
		{
			name:       "color success maps to green",
			cfg:        HappConfig{AutoDetect: true, SubInfoText: "Ok", SubInfoColor: "success"},
			wantHeader: "Sub-Info-Color",
			wantValue:  "green",
		},
		{
			name:       "autoconnect last maps to lastused",
			cfg:        HappConfig{AutoDetect: true, AutoConnect: true, AutoConnectType: "last"},
			wantHeader: "Subscription-Autoconnect-Type",
			wantValue:  "lastused",
		},
		{
			name:       "per-app exclude maps to bypass",
			cfg:        HappConfig{AutoDetect: true, PerAppMode: "exclude", PerAppList: "app.id"},
			wantHeader: "Per-App-Proxy-Mode",
			wantValue:  "bypass",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/sub/test", nil)
			ctx.Request.Header.Set("User-Agent", "Happ/1.2.0 (iOS)")

			controller := &SUBController{happConfig: tc.cfg}
			controller.ApplyCommonHeaders(ctx, "", "", "Title", "", "", "", false, "", false)

			if got := recorder.Header().Get(tc.wantHeader); got != tc.wantValue {
				t.Fatalf("%s = %q, want %q", tc.wantHeader, got, tc.wantValue)
			}
		})
	}
}

func TestAppendHappServerDescription(t *testing.T) {
	desc := "VIP Server"
	encoded := base64.StdEncoding.EncodeToString([]byte(desc))

	got := appendHappServerDescription("My Node", desc)
	want := "My Node?serverDescription=" + encoded
	if got != want {
		t.Fatalf("appendHappServerDescription = %q, want %q", got, want)
	}

	if gotEmpty := appendHappServerDescription("My Node", ""); gotEmpty != "My Node" {
		t.Fatalf("appendHappServerDescription with empty desc = %q, want My Node", gotEmpty)
	}
}

func TestAppendQueryAndFragment_PreservesServerDescription(t *testing.T) {
	desc := "Fast Server"
	encoded := base64.StdEncoding.EncodeToString([]byte(desc))

	t.Run("preserves serverDescription with encoded title", func(t *testing.T) {
		fragment := "Server 01?serverDescription=" + encoded
		link := appendQueryAndFragment("vless://user@host:443", nil, fragment, "", false)
		want := "vless://user@host:443#Server%2001?serverDescription=" + encoded
		if link != want {
			t.Fatalf("appendQueryAndFragment = %q, want %q", link, want)
		}
	})

	t.Run("properly escapes remark containing literal question mark without serverDescription", func(t *testing.T) {
		fragment := "Fast? Server"
		link := appendQueryAndFragment("vless://user@host:443", nil, fragment, "", false)
		want := "vless://user@host:443#Fast%3F%20Server"
		if link != want {
			t.Fatalf("appendQueryAndFragment = %q, want %q", link, want)
		}
	})

	t.Run("properly escapes remark containing literal question mark with serverDescription", func(t *testing.T) {
		fragment := "Fast? Server?serverDescription=" + encoded
		link := appendQueryAndFragment("vless://user@host:443", nil, fragment, "", false)
		want := "vless://user@host:443#Fast%3F%20Server?serverDescription=" + encoded
		if link != want {
			t.Fatalf("appendQueryAndFragment = %q, want %q", link, want)
		}
	})

	t.Run("escapes fragment when serverDescription tail contains invalid base64", func(t *testing.T) {
		fragment := "Node 1?serverDescription=not-base64!!!"
		link := appendQueryAndFragment("vless://user@host:443", nil, fragment, "", false)
		want := "vless://user@host:443#Node%201%3FserverDescription%3Dnot-base64%21%21%21"
		if link != want {
			t.Fatalf("appendQueryAndFragment = %q, want %q", link, want)
		}
	})

	t.Run("escapes fragment when serverDescription tail contains newline injection", func(t *testing.T) {
		fragment := "Node 1?serverDescription=" + encoded + "\nevil://inject"
		link := appendQueryAndFragment("vless://user@host:443", nil, fragment, "", false)
		if strings.Contains(link, "\n") {
			t.Fatalf("appendQueryAndFragment emitted raw newline: %q", link)
		}
	})
}

func TestIsHappClient(t *testing.T) {
	matching := []string{
		"Happ/1.2.0 (iPhone; iOS 17.5)",
		"happ/2.0",
		"happ",
		"HAPP/1.0",
		"Mozilla/5.0 Happ/1.0",
	}
	for _, ua := range matching {
		if !IsHappClient(ua) {
			t.Errorf("IsHappClient(%q) = false, want true", ua)
		}
	}

	nonMatching := []string{
		"Happy/2.0",
		"happier-client",
		"Happening/1.0",
		"unhappy",
		"v2rayNG/1.8.5",
		"",
	}
	for _, ua := range nonMatching {
		if IsHappClient(ua) {
			t.Errorf("IsHappClient(%q) = true, want false", ua)
		}
	}
}
