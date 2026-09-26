package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestClientRenewalPreviewHTTP(t *testing.T) {
	for _, tt := range []struct {
		name, zone, expiry, renewAt, validThrough, next string
		wantError                                       string
		reset, day, weekday, max, count                 int
		periods                                         int
		canRenew, delayed, invalid                      bool
	}{
		{name: "monthly uses Taipei and preserves legacy precedence", zone: "Asia/Taipei", expiry: "2030-01-01T00:00:00+08:00", renewAt: "2030-01-01T00:00:00+08:00", validThrough: "2029-12-31T23:59:59+08:00", next: "2030-02-01T00:00:00+08:00", reset: 7, day: 1, canRenew: true},
		{name: "31st clamps in February", zone: "UTC", expiry: "2030-01-31T00:00:00Z", renewAt: "2030-01-31T00:00:00Z", validThrough: "2030-01-30T23:59:59Z", next: "2030-02-28T00:00:00Z", day: 31, canRenew: true},
		{name: "31st returns after February", zone: "UTC", expiry: "2030-02-28T00:00:00Z", renewAt: "2030-02-28T00:00:00Z", validThrough: "2030-02-27T23:59:59Z", next: "2030-03-31T00:00:00Z", day: 31, canRenew: true},
		{name: "leap February", zone: "UTC", expiry: "2028-01-31T00:00:00Z", renewAt: "2028-01-31T00:00:00Z", validThrough: "2028-01-30T23:59:59Z", next: "2028-02-29T00:00:00Z", day: 31, canRenew: true},
		{name: "weekly crosses New York daylight saving", zone: "America/New_York", expiry: "2030-03-10T00:00:00-05:00", renewAt: "2030-03-10T00:00:00-05:00", validThrough: "2030-03-09T23:59:59-05:00", next: "2030-03-17T00:00:00-04:00", weekday: 7, canRenew: true},
		{name: "interval preserves 168 hours", zone: "America/New_York", expiry: "2030-03-10T00:00:00-05:00", renewAt: "2030-03-10T00:00:00-05:00", validThrough: "2030-03-09T23:59:59-05:00", next: "2030-03-17T01:00:00-04:00", reset: 7, canRenew: true},
		{name: "inclusive month end charges full month", zone: "UTC", expiry: "2030-01-31T23:59:59Z", renewAt: "2030-02-01T00:00:00Z", validThrough: "2030-01-31T23:59:58Z", next: "2030-03-01T00:00:00Z", day: 1, max: 1, canRenew: true},
		{name: "inclusive week end charges full week", zone: "UTC", expiry: "2030-01-06T23:59:59Z", renewAt: "2030-01-07T00:00:00Z", validThrough: "2030-01-06T23:59:58Z", next: "2030-01-14T00:00:00Z", weekday: 1, max: 1, canRenew: true},
		{name: "partial offline catch-up remains expired", zone: "UTC", expiry: "2026-03-01T00:00:00Z", renewAt: "2026-03-01T00:00:00Z", validThrough: "2026-02-28T23:59:59Z", next: "2026-03-08T00:00:00Z", weekday: 7, max: 3, count: 2, periods: 1},
		{name: "arbitrary initial cutoff is not silently realigned", zone: "UTC", expiry: "2030-01-08T12:00:00Z", renewAt: "2030-01-08T12:00:00Z", validThrough: "2030-01-08T11:59:59Z", next: "2030-01-14T00:00:00Z", weekday: 1, canRenew: true},
		{name: "cap exhausted", zone: "UTC", expiry: "2030-01-01T00:00:00Z", renewAt: "2030-01-01T00:00:00Z", validThrough: "2029-12-31T23:59:59Z", day: 1, max: 3, count: 3},
		{name: "unset cutoff only suggests a boundary", zone: "UTC", weekday: 1},
		{name: "first-use waits for activation", zone: "UTC", expiry: "first-use", weekday: 1, delayed: true},
		{name: "disabled ignores absolute expiry", zone: "UTC", expiry: "2030-01-01T00:00:00Z"},
		{name: "invalid weekday", zone: "UTC", weekday: 8, invalid: true, wantError: "client resetWeekday must be between 0 and 7, got: 8\n"},
		{name: "conflicting weekly and monthly", zone: "UTC", day: 1, weekday: 1, invalid: true, wantError: "client weekly renewal cannot be combined with reset or resetDay\n"},
		{name: "conflicting weekly and interval", zone: "UTC", reset: 7, weekday: 1, invalid: true, wantError: "client weekly renewal cannot be combined with reset or resetDay\n"},
		{name: "negative count", zone: "UTC", count: -1, invalid: true, wantError: "renewal preview reset and resetCount must not be negative\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = database.CloseDB() })
			db := database.GetDB()
			if err := db.Create(&model.Setting{Key: "timeLocation", Value: tt.zone}).Error; err != nil {
				t.Fatal(err)
			}
			snapshot := xray.ClientTraffic{Email: "preview-sentinel", ExpiryTime: 1893456000000, ResetCount: 7, Up: 111, Down: 222, Enable: true}
			if err := db.Create(&snapshot).Error; err != nil {
				t.Fatal(err)
			}
			request := service.ClientRenewalPreviewRequest{Reset: tt.reset, ResetDay: tt.day, ResetWeekday: tt.weekday, ResetMax: tt.max, ResetCount: tt.count}
			if tt.expiry == "first-use" {
				request.ExpiryTime = -7 * 86400000
			} else if tt.expiry != "" {
				at, err := time.Parse(time.RFC3339, tt.expiry)
				if err != nil {
					t.Fatal(err)
				}
				request.ExpiryTime = at.UnixMilli()
			}
			payload, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			NewClientController(router.Group("/panel/api/clients"))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/panel/api/clients/renewalPreview", bytes.NewReader(payload))
			r.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, r)
			var msg struct {
				Success bool                         `json:"success"`
				Msg     string                       `json:"msg"`
				Obj     service.ClientRenewalPreview `json:"obj"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &msg); err != nil {
				t.Fatal(err)
			}
			if w.Code != http.StatusOK || msg.Success == tt.invalid {
				t.Fatalf("status/body = %d/%s", w.Code, w.Body.String())
			}
			if tt.invalid && msg.Msg != " ("+tt.wantError+")" {
				t.Fatalf("validation error = %q, want wrapped error %q", msg.Msg, tt.wantError)
			}
			if !tt.invalid {
				got := msg.Obj
				if got.TimeZone != tt.zone || got.RenewAt != tt.renewAt || got.ValidThrough != tt.validThrough || got.NextExpiry != tt.next || got.CanRenew != tt.canRenew || got.DelayedStart != tt.delayed {
					t.Fatalf("preview = %+v, want boundary/valid/next %q/%q/%q", got, tt.renewAt, tt.validThrough, tt.next)
				}
				wantRenewals := 0
				if tt.canRenew {
					wantRenewals = 1
				}
				if tt.periods > 0 {
					wantRenewals = tt.periods
				}
				if got.Renewals != wantRenewals {
					t.Fatalf("periods = %d, want %d", got.Renewals, wantRenewals)
				}
				if tt.weekday > 0 || tt.day > 0 {
					if got.SuggestedExpiryTime <= time.Now().UnixMilli() || got.SuggestedExpiry == "" {
						t.Fatalf("missing future suggestion: %+v", got)
					}
				}
			}
			var after xray.ClientTraffic
			if err := db.Where("email = ?", snapshot.Email).First(&after).Error; err != nil {
				t.Fatal(err)
			}
			if after != snapshot {
				t.Fatalf("read-only preview mutated traffic: %+v", after)
			}
		})
	}
}
