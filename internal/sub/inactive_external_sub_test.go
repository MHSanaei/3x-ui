package sub

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func seedInactiveExternalOnlySub(t *testing.T, subID, email string, enabled bool, expiry int64) {
	t.Helper()
	db := database.GetDB()
	rec := &model.ClientRecord{Email: email, SubID: subID, UUID: subID + "-uuid", Enable: true, ExpiryTime: expiry}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if !enabled {
		if err := db.Model(rec).Update("enable", false).Error; err != nil {
			t.Fatalf("disable client: %v", err)
		}
	}
	if err := db.Create(&xray.ClientTraffic{Email: email, Up: 11, Down: 22, Total: 1024, ExpiryTime: expiry}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}
	link := "vless://11111111-1111-1111-1111-111111111111@example.com:443?type=tcp&security=reality&pbk=abc&sid=12&fp=chrome#external"
	if err := db.Create(&model.ClientExternalLink{ClientId: rec.Id, Kind: model.ExternalLinkKindLink, Value: link, SortIndex: 1}).Error; err != nil {
		t.Fatalf("seed external link: %v", err)
	}
}

func TestInactiveExternalOnlySubRemainsKnownWithoutExposingLinks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	states := []struct {
		name    string
		enabled bool
		expiry  int64
	}{
		{name: "disabled", enabled: false, expiry: time.Now().Add(time.Hour).UnixMilli()},
		{name: "expired", enabled: true, expiry: time.Now().Add(-time.Hour).UnixMilli()},
	}

	for _, state := range states {
		t.Run(state.name, func(t *testing.T) {
			initSubDB(t)
			subID := "external-" + state.name
			email := state.name + "@example.com"
			seedInactiveExternalOnlySub(t, subID, email, state.enabled, state.expiry)

			oldDistFS := distFS
			distFS = testDistFS
			t.Cleanup(func() { distFS = oldDistFS })

			router := gin.New()
			NewSUBController(
				router.Group("/"),
				WithSUBJsonEnabled(true),
				WithSUBClashEnabled(true),
				WithSUBEncryption(false),
			)

			wantHeader := fmt.Sprintf("upload=11; download=22; total=1024; expire=%d", state.expiry/1000)
			// Each format's "nothing active" body is shaped differently: raw is
			// genuinely empty, JSON/Clash must stay parseable so a client update
			// doesn't error out -- see the format-specific checks below.
			cases := []struct {
				path      string
				wantEmpty bool
			}{
				{"/sub/" + subID, true},
				{"/json/" + subID + "?view=raw", false},
				{"/clash/" + subID + "?view=raw", false},
			}
			for _, tc := range cases {
				t.Run(tc.path, func(t *testing.T) {
					if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", email).Update("last_sub_fetch", 0).Error; err != nil {
						t.Fatalf("reset last_sub_fetch: %v", err)
					}
					req := httptest.NewRequest(http.MethodGet, tc.path, nil)
					req.Host = "sub.example.com"
					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)

					if w.Code != http.StatusOK {
						t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
					}
					if tc.wantEmpty && w.Body.Len() != 0 {
						t.Fatalf("want empty body, got: %s", w.Body.String())
					}
					if strings.Contains(w.Body.String(), "11111111-1111-1111-1111-111111111111") {
						t.Fatalf("inactive external link leaked in body: %s", w.Body.String())
					}
					if got := w.Header().Get("Subscription-Userinfo"); got != wantHeader {
						t.Fatalf("Subscription-Userinfo = %q, want %q", got, wantHeader)
					}
					var traffic xray.ClientTraffic
					if err := database.GetDB().Where("email = ?", email).First(&traffic).Error; err != nil {
						t.Fatalf("load traffic: %v", err)
					}
					if traffic.LastSubFetch == 0 {
						t.Fatal("successful response did not update last_sub_fetch")
					}
				})
			}

			req := httptest.NewRequest(http.MethodGet, "/sub/"+subID, nil)
			req.Host = "sub.example.com"
			req.Header.Set("Accept", "text/html")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("HTML status = %d, want 200; body=%s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "11111111-1111-1111-1111-111111111111") {
				t.Fatalf("HTML page exposed inactive external link: %s", w.Body.String())
			}
			if !strings.Contains(w.Body.String(), `"links":[]`) {
				t.Fatalf("HTML page did not render an empty links list: %s", w.Body.String())
			}
		})
	}
}
