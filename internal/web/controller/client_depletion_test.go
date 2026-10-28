package controller

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestClientDerivedDepletionHTTP(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	testClientDerivedDepletionHTTP(t)
}

func TestClientDerivedDepletionPostgres(t *testing.T) {
	dsn := os.Getenv("XUI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set XUI_TEST_PG_DSN to run the read-only API contract on PostgreSQL")
	}
	t.Setenv("XUI_DB_TYPE", "postgres")
	t.Setenv("XUI_DB_DSN", dsn)
	cleanup, err := testpg.IsolatePackage("client_depletion")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	if err := database.InitDB(""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	testClientDerivedDepletionHTTP(t)
}

func testClientDerivedDepletionHTTP(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := database.GetDB()
	clock := time.Now().Truncate(time.Millisecond)
	db.NowFunc = func() time.Time { return clock }
	now := clock.UnixMilli()
	client := model.ClientRecord{Email: "depleted@x", SubID: "derived-sub", TgID: 6883, UUID: "derived-uuid", Enable: true}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}
	inbound := model.Inbound{Tag: "derived", Protocol: model.VLESS, Settings: `{"clients":[]}`, StreamSettings: `{"network":"tcp","security":"tls"}`}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: inbound.Id, FlowOverride: "xtls-rprx-vision"}).Error; err != nil {
		t.Fatal(err)
	}
	enabled := true
	if err := db.Create(&model.ClientExternalLink{ClientId: client.Id, Kind: "link", Value: "vless://example", Enable: &enabled}).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("I18n", func(_ locale.I18nType, key string, _ ...string) string { return key })
		c.Next()
	})
	NewClientController(router.Group("/panel/api/clients"))
	monitoring := false
	writes, trafficReads := 0, 0
	blockWrite := func(tx *gorm.DB) {
		if monitoring {
			writes++
			tx.AddError(fmt.Errorf("unexpected write during depletion read"))
		}
	}
	const callback = "test:depletion-read-only"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, blockWrite); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Update().Before("gorm:update").Register(callback, blockWrite); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register(callback, blockWrite); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Raw().Before("gorm:raw").Register(callback, blockWrite); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if monitoring && tx.Statement.Table == "client_traffics" {
			trafficReads++
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Create().Remove(callback)
		_ = db.Callback().Update().Remove(callback)
		_ = db.Callback().Delete().Remove(callback)
		_ = db.Callback().Raw().Remove(callback)
		_ = db.Callback().Query().Remove(callback)
	})
	read := func(t *testing.T, path string) (bool, json.RawMessage) {
		t.Helper()
		response := httptest.NewRecorder()
		monitoring = true
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panel/api/clients"+path, nil))
		monitoring = false
		var envelope struct {
			Success bool            `json:"success"`
			Msg     string          `json:"msg"`
			Obj     json.RawMessage `json:"obj"`
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(response.Body.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || len(fields) != 3 {
			t.Fatalf("changed response envelope: %s", response.Body)
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if writes != 0 {
			t.Fatalf("API read attempted %d database writes", writes)
		}
		return envelope.Success, envelope.Obj
	}
	baseTraffic := xray.ClientTraffic{Email: client.Email, Enable: true}
	if err := db.Create(&baseTraffic).Error; err != nil {
		t.Fatal(err)
	}
	type report struct{ up, down, age int64 }
	for _, tc := range []struct {
		name                                string
		up, down, total, expiry, lastOnline int64
		disabled, nullable                  bool
		reports                             []report
		wantUp, wantDown, wantUsed, at      int64
		reason                              string
	}{
		{name: "standalone traffic before disable job", up: 60, down: 40, total: 100, lastOnline: 1234, wantUp: 60, wantDown: 40, wantUsed: 100, at: 1234, reason: "traffic"},
		{name: "reset below quota immediately clears metadata", disabled: true, up: 10, down: 20, total: 100, lastOnline: 1234, wantUp: 10, wantDown: 20, wantUsed: 30},
		{name: "quota increase immediately restores validity", disabled: true, up: 60, down: 40, total: 200, lastOnline: 1234, wantUp: 60, wantDown: 40, wantUsed: 100},
		{name: "expiration only", total: 100, expiry: now - 10000, lastOnline: 1234, at: now - 10000, reason: "expiry"},
		{name: "extended expiration immediately restores validity", disabled: true, total: 100, expiry: now + 3600000, lastOnline: 1234},
		{name: "manual disable without depleted limits", disabled: true, up: 10, down: 20, total: 100, lastOnline: 1234, wantUp: 10, wantDown: 20, wantUsed: 30},
		{name: "fresh global quota exhausted", up: 10, down: 10, total: 100, lastOnline: 1234, reports: []report{{60, 60, 0}}, wantUp: 60, wantDown: 60, wantUsed: 120, at: 1234, reason: "traffic"},
		{name: "global is not added to local", up: 20, total: 100, lastOnline: 1234, reports: []report{{80, 0, 0}}, wantUp: 80, wantUsed: 80},
		{name: "directional maxima across reports", up: 40, down: 10, total: 100, lastOnline: 1234, reports: []report{{20, 60, 0}, {60, 20, 0}}, wantUp: 60, wantDown: 60, wantUsed: 120, at: 1234, reason: "traffic"},
		{name: "overlapping masters are not summed", up: 20, total: 100, lastOnline: 1234, reports: []report{{60, 0, 0}, {60, 0, 0}}, wantUp: 60, wantUsed: 60},
		{name: "stale global ignored", up: 10, down: 10, total: 100, lastOnline: 1234, reports: []report{{60, 60, int64(25 * time.Hour / time.Millisecond)}}, wantUp: 10, wantDown: 10, wantUsed: 20},
		{name: "local larger than global", up: 80, down: 20, total: 100, lastOnline: 1234, reports: []report{{10, 10, 0}}, wantUp: 80, wantDown: 20, wantUsed: 100, at: 1234, reason: "traffic"},
		{name: "global depletion then expiry", total: 100, expiry: now - 10000, lastOnline: now - 20000, reports: []report{{60, 60, 0}}, wantUp: 60, wantDown: 60, wantUsed: 120, at: now - 20000, reason: "both"},
		{name: "expiry precedes global depletion estimate", total: 100, expiry: now - 20000, lastOnline: now - 10000, reports: []report{{60, 60, 0}}, wantUp: 60, wantDown: 60, wantUsed: 120, at: now - 20000, reason: "both"},
		{name: "global depleted with unknown last online", total: 100, reports: []report{{60, 60, 0}}, wantUp: 60, wantDown: 60, wantUsed: 120, reason: "traffic"},
		{name: "both with unknown last online", total: 100, expiry: now - 10000, reports: []report{{60, 60, 0}}, wantUp: 60, wantDown: 60, wantUsed: 120, at: now - 10000, reason: "both"},
		{name: "counter overflow", up: math.MaxInt64, down: 1, total: math.MaxInt64, lastOnline: 1234, wantUp: math.MaxInt64, wantDown: 1, wantUsed: math.MaxInt64, at: 1234, reason: "traffic"},
		{name: "nullable legacy values", nullable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.Where("email = ?", client.Email).Delete(&model.ClientGlobalTraffic{}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&client).Update("enable", !tc.disabled).Error; err != nil {
				t.Fatal(err)
			}
			stored := xray.ClientTraffic{Id: baseTraffic.Id, Email: client.Email, InboundId: inbound.Id, Up: tc.up, Down: tc.down, Total: tc.total, ExpiryTime: tc.expiry, LastOnline: tc.lastOnline, Enable: true}
			if err := db.Save(&stored).Error; err != nil {
				t.Fatal(err)
			}
			if tc.disabled {
				if err := db.Model(&stored).Update("enable", false).Error; err != nil {
					t.Fatal(err)
				}
				stored.Enable = false
			}
			if tc.nullable {
				if err := db.Model(&stored).Updates(map[string]any{"up": nil, "down": nil, "total": nil, "expiry_time": nil, "last_online": nil}).Error; err != nil {
					t.Fatal(err)
				}
			}
			for i, r := range tc.reports {
				if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: strconv.Itoa(i), Email: client.Email, Up: r.up, Down: r.down, UpdatedAt: now - r.age}).Error; err != nil {
					t.Fatal(err)
				}
			}
			for _, path := range []string{"/get/", "/traffic/"} {
				trafficReads = 0
				success, raw := read(t, path+client.Email)
				if !success {
					t.Fatalf("request rejected: %s", raw)
				}
				if trafficReads != 1 {
					t.Fatalf("traffic looked up %d times, want one", trafficReads)
				}
				var got map[string]json.RawMessage
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatal(err)
				}
				at, err := strconv.ParseInt(string(got["depletedAt"]), 10, 64)
				if err != nil || at != tc.at || string(got["depletedReason"]) != strconv.Quote(tc.reason) {
					t.Fatalf("depletion = %s/%s, want %d/%q", got["depletedAt"], got["depletedReason"], tc.at, tc.reason)
				}
				delete(got, "depletedAt")
				delete(got, "depletedReason")
				if path == "/traffic/" {
					expected := stored
					expected.Up = tc.wantUp
					expected.Down = tc.wantDown
					expected.UUID = client.UUID
					expected.SubId = client.SubID
					encoded, _ := json.Marshal(expected)
					var expectedFields map[string]json.RawMessage
					_ = json.Unmarshal(encoded, &expectedFields)
					if !reflect.DeepEqual(got, expectedFields) {
						t.Fatalf("legacy traffic fields changed: got %s; want %s", raw, encoded)
					}
				} else {
					if len(got) != 5 || string(got["usedTraffic"]) != strconv.FormatInt(tc.wantUsed, 10) || string(got["inboundIds"]) != fmt.Sprintf("[%d]", inbound.Id) || string(got["tunnelAllowedIPs"]) != "{}" {
						t.Fatalf("legacy detail fields changed: %s", raw)
					}
					if !strings.Contains(string(got["client"]), `"email":"depleted@x"`) || !strings.Contains(string(got["externalLinks"]), `"value":"vless://example"`) {
						t.Fatalf("client/links lost: %s", raw)
					}
				}
			}
			var after xray.ClientTraffic
			if err := db.First(&after, stored.Id).Error; err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, stored) {
				t.Fatalf("read mutated source traffic: got %+v, want %+v", after, stored)
			}
		})
	}
	for _, path := range []string{"/get/subId/derived-sub", "/get/tgId/6883", "/list", "/list/paged"} {
		success, raw := read(t, path)
		if !success || strings.Contains(string(raw), `"depletedAt"`) || strings.Contains(string(raw), `"depletedReason"`) {
			t.Fatalf("out-of-scope API changed at %s: %s", path, raw)
		}
	}
	if err := db.Where("email = ?", client.Email).Delete(&xray.ClientTraffic{}).Error; err != nil {
		t.Fatal(err)
	}
	success, raw := read(t, "/get/"+client.Email)
	if !success || !strings.Contains(string(raw), `"depletedAt":0`) || !strings.Contains(string(raw), `"depletedReason":""`) {
		t.Fatalf("missing traffic detail: %s", raw)
	}
	if err := db.Create(&xray.ClientTraffic{Email: "orphan@x", Up: 100, Total: 1, LastOnline: 1234}).Error; err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/traffic/" + client.Email, "/traffic/nonexistent@x", "/traffic/orphan@x", "/get/nonexistent@x"} {
		success, raw := read(t, path)
		if success != (path != "/get/nonexistent@x" && path != "/traffic/orphan@x") || string(raw) != "null" {
			t.Fatalf("missing-record semantics changed at %s: success=%v obj=%s", path, success, raw)
		}
	}
}
