package sub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestSubscriptionProfileModesFromSavedSettings(t *testing.T) {
	oldFS, oldMode := distFS, gin.Mode()
	oldWriter, oldErrorWriter := gin.DefaultWriter, gin.DefaultErrorWriter
	SetDistFS(testDistFS)
	t.Cleanup(func() {
		SetDistFS(oldFS)
		gin.SetMode(oldMode)
		gin.DefaultWriter, gin.DefaultErrorWriter = oldWriter, oldErrorWriter
	})
	for _, config := range []struct {
		name, mode, profileURL, want string
	}{
		{name: "new installation"},
		{name: "legacy whitespace", profileURL: "   "},
		{name: "legacy custom", profileURL: "https://portal.example/account", want: "https://portal.example/account"},
		{name: "none retains custom", mode: "none", profileURL: "https://portal.example/account"},
		{name: "builtin", mode: "builtin", profileURL: "https://portal.example/account"},
		{name: "custom", mode: "custom", profileURL: "https://portal.example/?sub={{SUB_ID}}", want: "https://portal.example/?sub=profile-sub"},
		{name: "empty custom", mode: "custom"},
		{name: "invalid mode", mode: "invalid", profileURL: "https://portal.example/account"},
	} {
		t.Run(config.name, func(t *testing.T) {
			initSubDB(t)
			seedInfoEndpointSub(t, "profile-sub", "profile@example.com")
			settings := []model.Setting{
				{Key: "subPath", Value: "/sub/"},
				{Key: "subJsonPath", Value: "/json/"},
				{Key: "subClashPath", Value: "/clash/"},
				{Key: "subJsonEnable", Value: "true"},
				{Key: "subClashEnable", Value: "true"},
				{Key: "subProfileUrl", Value: config.profileURL},
			}
			if config.mode != "" {
				settings = append(settings, model.Setting{Key: "subProfileMode", Value: config.mode})
			}
			for _, setting := range settings {
				if err := database.GetDB().Where("key = ?", setting.Key).Delete(&model.Setting{}).Error; err != nil {
					t.Fatal(err)
				}
				if err := database.GetDB().Create(&setting).Error; err != nil {
					t.Fatal(err)
				}
			}
			router, err := (&Server{}).initRouter()
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"/sub/profile-sub", "/json/profile-sub", "/clash/profile-sub", "/json/profile-sub?view=raw", "/clash/profile-sub?view=raw", "/mihomo/profile-sub"} {
				for _, userAgent := range []string{"Happ/3.22.0 (Android)", "v2rayNG/1.8.5"} {
					t.Run(path+"/"+userAgent, func(t *testing.T) {
						req := httptest.NewRequest(http.MethodGet, "https://sub.example.com:8443"+path, nil)
						req.Header.Set("User-Agent", userAgent)
						resp := httptest.NewRecorder()
						router.ServeHTTP(resp, req)
						if resp.Code != http.StatusOK {
							t.Fatalf("status = %d; body=%s", resp.Code, resp.Body.String())
						}
						want := config.want
						if config.mode == "builtin" {
							want = "https://sub.example.com:8443" + req.URL.EscapedPath() + "?html=1"
						}
						if got := resp.Header().Get("Profile-Web-Page-Url"); got != want {
							t.Fatalf("Profile-Web-Page-Url = %q, want %q", got, want)
						}
						if want == "" {
							if _, present := resp.Header()["Profile-Web-Page-Url"]; present {
								t.Fatal("disabled profile header must be absent")
							}
						}
						if config.mode == "builtin" {
							// The restored link must open the page, even when copied from a raw download.
							page := httptest.NewRecorder()
							router.ServeHTTP(page, httptest.NewRequest(http.MethodGet, want, nil))
							if page.Code != http.StatusOK || !strings.Contains(page.Header().Get("Content-Type"), "text/html") {
								t.Fatalf("builtin link did not serve HTML: status=%d, type=%q", page.Code, page.Header().Get("Content-Type"))
							}
						}
					})
				}
			}
		})
	}
}
