package sub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestSubscriptionCalendarExpireInclusive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		setting    string
		zone       string
		boundary   string
		resetDay   int
		trafficDay int
		wantSnap   bool
	}{
		{"default unchanged", "", "UTC", "2030-10-01T00:00:00Z", 1, 1, false},
		{"disabled unchanged", "false", "UTC", "2030-10-01T00:00:00Z", 1, 1, false},
		{"30 day month", "true", "UTC", "2030-10-01T00:00:00Z", 1, 1, true},
		{"31 day month", "true", "UTC", "2030-11-01T00:00:00Z", 1, 1, true},
		{"non leap February", "true", "UTC", "2030-03-01T00:00:00Z", 1, 1, true},
		{"leap February", "true", "UTC", "2028-03-01T00:00:00Z", 1, 1, true},
		{"Taipei midnight", "true", "Asia/Taipei", "2030-10-01T00:00:00+08:00", 1, 1, true},
		{"New York daylight time", "true", "America/New_York", "2030-10-01T00:00:00-04:00", 1, 1, true},
		{"New York standard time", "true", "America/New_York", "2030-02-01T00:00:00-05:00", 1, 1, true},
		{"Havana first midnight", "true", "America/Havana", "2026-11-01T00:00:00-04:00", 1, 1, true},
		{"Havana repeated midnight", "true", "America/Havana", "2026-11-01T00:00:00-05:00", 1, 1, false},
		{"UTC midnight is not Taipei midnight", "true", "Asia/Taipei", "2030-10-01T00:00:00Z", 1, 1, false},
		{"midday unchanged", "true", "UTC", "2030-10-01T12:00:00Z", 1, 1, false},
		{"other midnight unchanged", "true", "UTC", "2030-10-02T00:00:00Z", 1, 1, false},
		{"fractional midnight unchanged", "true", "UTC", "2030-10-01T00:00:00.001Z", 1, 1, false},
		{"legacy inclusive input unchanged", "true", "UTC", "2030-09-30T23:59:59Z", 1, 1, false},
		{"interval renewal unchanged", "true", "UTC", "2030-10-01T00:00:00Z", 0, 0, false},
		{"other billing day unchanged", "true", "UTC", "2030-10-01T00:00:00Z", 31, 31, false},
		{"client calendar overrides stale traffic", "true", "UTC", "2030-10-01T00:00:00Z", 1, 0, true},
		{"client interval overrides stale traffic", "true", "UTC", "2030-10-01T00:00:00Z", 0, 1, false},
		{"unlimited unchanged", "true", "UTC", "", 1, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seedSubDB(t)
			db := database.GetDB()
			if err := db.Create(&model.Setting{Key: "timeLocation", Value: tt.zone}).Error; err != nil {
				t.Fatal(err)
			}
			if tt.setting != "" {
				if err := db.Create(&model.Setting{Key: "subCalendarExpireInclusive", Value: tt.setting}).Error; err != nil {
					t.Fatal(err)
				}
			}
			var expiry int64
			if tt.boundary != "" {
				at, err := time.Parse(time.RFC3339Nano, tt.boundary)
				if err != nil {
					t.Fatal(err)
				}
				expiry = at.UnixMilli()
			}
			seedSubProtocolInbound(t, "calendar", "monthly", 4931, 1, `{"network":"tcp","security":"none"}`, model.VMESS)
			if err := db.Model(&model.ClientRecord{}).Where("email = ?", "monthly@e").Updates(map[string]any{
				"expiry_time": expiry, "reset_day": tt.resetDay,
			}).Error; err != nil {
				t.Fatal(err)
			}
			// Node snapshots may omit limits; the clients table still owns the calendar.
			if err := db.Create(&xray.ClientTraffic{
				Email: "monthly@e", Enable: true, Up: 11, Down: 22, ResetDay: tt.trafficDay, ResetCount: 7,
			}).Error; err != nil {
				t.Fatal(err)
			}
			router := newSubscriptionTestRouter(subscriptionTestRouterConfig{})
			wantExpiry := expiry / 1000
			if tt.wantSnap {
				wantExpiry--
			}
			wantHeader := fmt.Sprintf("upload=11; download=22; total=0; expire=%d", wantExpiry)
			for _, path := range []string{"/sub/calendar", "/json/calendar", "/clash/calendar", "/mihomo/calendar", "/clash-legacy/calendar"} {
				resp := httptest.NewRecorder()
				router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "http://sub.example.com"+path, nil))
				if resp.Code != http.StatusOK {
					t.Fatalf("GET %s: status=%d body=%s", path, resp.Code, resp.Body.String())
				}
				if got := resp.Header().Get("Subscription-Userinfo"); got != wantHeader {
					t.Fatalf("GET %s: userinfo=%q, want %q", path, got, wantHeader)
				}
			}

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/calendar?format=info", nil))
			var info struct {
				Expire int64 `json:"expire"`
			}
			if resp.Code != http.StatusOK {
				t.Fatalf("info status=%d body=%s", resp.Code, resp.Body.String())
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &info); err != nil {
				t.Fatal(err)
			}
			if info.Expire != expiry/1000 {
				t.Fatalf("info cutoff=%d, want canonical %d", info.Expire, expiry/1000)
			}
			var client model.ClientRecord
			var traffic xray.ClientTraffic
			if err := db.Where("email = ?", "monthly@e").First(&client).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Where("email = ?", "monthly@e").First(&traffic).Error; err != nil {
				t.Fatal(err)
			}
			if client.ExpiryTime != expiry || traffic.ResetCount != 7 || traffic.Up != 11 || traffic.Down != 22 {
				t.Fatalf("subscription presentation mutated scheduling/accounting: client=%+v traffic=%+v", client, traffic)
			}
		})
	}
}

func TestSubscriptionCalendarExpireInclusiveMixedClients(t *testing.T) {
	tests := []struct {
		name       string
		days       [2]int
		different  bool
		wantExpiry int64
	}{
		{"calendar then interval", [2]int{1, 0}, false, 1917043200},
		{"interval then calendar", [2]int{0, 1}, false, 1917043200},
		{"same calendar", [2]int{1, 1}, false, 1917043199},
		{"different cutoffs", [2]int{1, 1}, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seedSubDB(t)
			db := database.GetDB()
			for key, value := range map[string]string{"timeLocation": "UTC", "subCalendarExpireInclusive": "true"} {
				if err := db.Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
					t.Fatal(err)
				}
			}
			const expiry = int64(1917043200000) // 2030-10-01 00:00:00 UTC
			for i, day := range tt.days {
				tag := fmt.Sprintf("client%d", i)
				seedSubInbound(t, "mixed", tag, 4932+i, i, `{"network":"tcp","security":"none"}`)
				clientExpiry := expiry
				if i == 1 && tt.different {
					clientExpiry += 31 * 24 * time.Hour.Milliseconds()
				}
				if err := db.Model(&model.ClientRecord{}).Where("email = ?", tag+"@e").Updates(map[string]any{
					"expiry_time": clientExpiry, "reset_day": day,
				}).Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Create(&xray.ClientTraffic{Email: tag + "@e", Enable: true}).Error; err != nil {
					t.Fatal(err)
				}
			}
			router := newSubscriptionTestRouter(subscriptionTestRouterConfig{})
			for _, path := range []string{"/sub/mixed", "/json/mixed", "/clash/mixed"} {
				resp := httptest.NewRecorder()
				router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "http://sub.example.com"+path, nil))
				if resp.Code != http.StatusOK || !strings.HasSuffix(resp.Header().Get("Subscription-Userinfo"), fmt.Sprintf("expire=%d", tt.wantExpiry)) {
					t.Fatalf("GET %s: status=%d userinfo=%q, want expire=%d", path, resp.Code, resp.Header().Get("Subscription-Userinfo"), tt.wantExpiry)
				}
			}
		})
	}
}

func TestSubscriptionCalendarExpireInclusiveSettingRoundTrip(t *testing.T) {
	seedSubDB(t)
	db := database.GetDB()
	seedSubInbound(t, "toggle", "monthly", 4935, 1, `{"network":"tcp","security":"none"}`)
	const expiry = int64(1917043200000)
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "monthly@e").Updates(map[string]any{
		"expiry_time": expiry, "reset_day": 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: "monthly@e", Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	settings := &service.SettingService{}
	all, err := settings.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if all.SubCalendarExpireInclusive {
		t.Fatal("inclusive presentation must default off")
	}
	all.TimeLocation = "UTC"
	router := newSubscriptionTestRouter(subscriptionTestRouterConfig{})
	for _, enabled := range []bool{false, true, false} {
		all.SubCalendarExpireInclusive = enabled
		if err := settings.UpdateAllSetting(all, service.SecretClears{}); err != nil {
			t.Fatal(err)
		}
		stored, err := settings.GetAllSetting()
		if err != nil || stored.SubCalendarExpireInclusive != enabled {
			t.Fatalf("setting round trip: enabled=%v stored=%+v err=%v", enabled, stored, err)
		}
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/toggle", nil))
		want := expiry / 1000
		if enabled {
			want--
		}
		if got := resp.Header().Get("Subscription-Userinfo"); resp.Code != http.StatusOK || got != fmt.Sprintf("upload=0; download=0; total=0; expire=%d", want) {
			t.Fatalf("enabled=%v: status=%d userinfo=%q, want expire=%d", enabled, resp.Code, got, want)
		}
	}
}

func TestSubscriptionCalendarExpireInclusiveFirstUseDuration(t *testing.T) {
	seedSubDB(t)
	db := database.GetDB()
	const duration = -int64(24 * time.Hour / time.Millisecond)
	if err := db.Create(&model.ClientRecord{Email: "first-use@e", ExpiryTime: duration, ResetDay: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: "first-use@e", ExpiryTime: duration, ResetDay: 1}).Error; err != nil {
		t.Fatal(err)
	}
	before := time.Now().UnixMilli()
	agg, _ := (&SubService{}).AggregateTrafficByEmails([]string{"first-use@e"})
	after := time.Now().UnixMilli()
	if agg.ResetDay != 0 || agg.ExpiryTime < before-duration || agg.ExpiryTime > after-duration {
		t.Fatalf("first-use duration must not become a canonical calendar cutoff: %+v", agg)
	}
}
