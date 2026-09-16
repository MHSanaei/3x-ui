package sub

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestRenderSubPlaceholders(t *testing.T) {
	data := subPlaceholderData{
		SubID: "sub-123",
		Context: remarkContext{client: model.Client{
			Email:  "Ilnur",
			ID:     "abcdef12-3456-7890-abcd-ef1234567890",
			SubID:  "sub-123",
			TgID:   42,
			Enable: true,
		}},
		HasCtx: true,
	}

	tests := []struct {
		name string
		tmpl string
		data subPlaceholderData
		want string
	}{
		{
			name: "identity tokens",
			tmpl: "{{EMAIL}}/{{ID}}/{{SHORT_ID}}/{{SUB_ID}}/{{TELEGRAM_ID}}",
			data: data,
			want: "Ilnur/abcdef12-3456-7890-abcd-ef1234567890/abcdef12/sub-123/42",
		},
		{
			name: "no template",
			tmpl: "isVPN",
			data: subPlaceholderData{SubID: "sub-123"},
			want: "isVPN",
		},
		{
			name: "unsupported tokens stay literal",
			tmpl: "{{SUB_ID}}/{{INBOUND}}/{{TRAFFIC_LEFT}}/{{PROTOCOL}}/{EMAIL}",
			data: subPlaceholderData{SubID: "sub-123"},
			want: "sub-123/{{INBOUND}}/{{TRAFFIC_LEFT}}/{{PROTOCOL}}/{EMAIL}",
		},
		{
			name: "URL values are escaped",
			tmpl: "https://support.example/?email={{EMAIL}}&sub={{SUB_ID}}",
			data: subPlaceholderData{
				SubID: "sub id",
				Context: remarkContext{client: model.Client{
					Email: "john doe@example.com",
					SubID: "sub id",
				}},
				HasCtx: true,
				Escape: true,
			},
			want: "https://support.example/?email=john+doe%40example.com&sub=sub+id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderSubPlaceholders(tt.tmpl, tt.data); got != tt.want {
				t.Fatalf("renderSubPlaceholders() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMetadataForSubRequestOmitsUnconfiguredProfileURL(t *testing.T) {
	a := &SUBController{
		subTitle:      "isVPN",
		subSupportUrl: "https://support.example/",
	}
	metadata := a.metadataForSubRequest(func() *SubService {
		t.Fatal("metadataForSubRequest loaded a subscription context without configured placeholders")
		return nil
	}, "sub-123", "https://sub.example/sub-123")

	if metadata.ProfileURL != "" {
		t.Fatalf("ProfileURL = %q, want no link when unconfigured", metadata.ProfileURL)
	}
}

func TestSubscriptionProfileURLRequiresExplicitConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initSubDB(t)
	seedInfoEndpointSub(t, "profile-sub", "profile@example.com")

	for _, config := range []struct {
		name, profileURL, want string
	}{
		{name: "empty"},
		{name: "whitespace", profileURL: "   "},
		{name: "explicit", profileURL: "https://portal.example.com/account", want: "https://portal.example.com/account"},
		{name: "template", profileURL: "https://portal.example.com/account?sub={{SUB_ID}}", want: "https://portal.example.com/account?sub=profile-sub"},
	} {
		t.Run(config.name, func(t *testing.T) {
			for _, client := range []struct {
				name, userAgent string
				happAutoDetect  bool
			}{
				{name: "Happ", userAgent: "Happ/3.22.0 (Android)", happAutoDetect: true},
				{name: "Happ without auto-detection", userAgent: "Happ/3.22.0 (Android)"},
				{name: "standard", userAgent: "v2rayNG/1.8.5", happAutoDetect: true},
			} {
				t.Run(client.name, func(t *testing.T) {
					// All formats must honor the opt-in, independently of Happ customization.
					router := gin.New()
					NewSUBController(router.Group("/"),
						WithSUBJsonEnabled(true), WithSUBClashEnabled(true),
						WithSUBProfileURL(config.profileURL),
						WithSUBProfileMode(service.SubProfileModeCustom),
						WithSUBHappConfig(HappConfig{AutoDetect: client.happAutoDetect}),
					)
					for _, path := range []string{"/sub/profile-sub", "/json/profile-sub", "/clash/profile-sub"} {
						t.Run(path, func(t *testing.T) {
							req := httptest.NewRequest(http.MethodGet, path, nil)
							req.Host = "sub.example.com"
							req.Header.Set("User-Agent", client.userAgent)
							resp := httptest.NewRecorder()
							router.ServeHTTP(resp, req)
							if resp.Code != http.StatusOK {
								t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
							}
							if got := resp.Header().Get("Profile-Web-Page-Url"); got != config.want {
								t.Fatalf("Profile-Web-Page-Url = %q, want %q", got, config.want)
							}
							if config.want == "" {
								if _, present := resp.Header()["Profile-Web-Page-Url"]; present {
									t.Fatal("unconfigured profile header must be omitted")
								}
							}
						})
					}
				})
			}
		})
	}
}

func TestMetadataForSubRequestUsesStableClientIdentity(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()
	first := model.ClientRecord{
		Email:  "john doe@example.com",
		SubID:  "sub-123",
		UUID:   "abcdef12-3456-7890-abcd-ef1234567890",
		TgID:   42,
		Enable: true,
	}
	second := model.ClientRecord{
		Email:  "jane@example.com",
		SubID:  "sub-123",
		UUID:   "fedcba98-3456-7890-abcd-ef1234567890",
		TgID:   99,
		Enable: true,
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("seed first client: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("seed second client: %v", err)
	}

	a := &SUBController{
		subTitle:       "isVPN — {{EMAIL}}",
		subSupportUrl:  "https://support.example/?email={{EMAIL}}&tg={{TELEGRAM_ID}}",
		subProfileUrl:  "https://profile.example/account/{{ID}}",
		subProfileMode: service.SubProfileModeCustom,
		subAnnounce:    "Subscription {{SUB_ID}}",
	}
	metadata := a.metadataForSubRequest(func() *SubService { return &SubService{} }, "sub-123", "https://sub.example/sub-123")

	if metadata.Title != "isVPN — john doe@example.com" {
		t.Fatalf("Title = %q", metadata.Title)
	}
	if metadata.SupportURL != "https://support.example/?email=john+doe%40example.com&tg=42" {
		t.Fatalf("SupportURL = %q", metadata.SupportURL)
	}
	if metadata.ProfileURL != "https://profile.example/account/abcdef12-3456-7890-abcd-ef1234567890" {
		t.Fatalf("ProfileURL = %q", metadata.ProfileURL)
	}
	if metadata.Announce != "Subscription sub-123" {
		t.Fatalf("Announce = %q", metadata.Announce)
	}
}
