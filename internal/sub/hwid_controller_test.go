package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-yaml"
	"github.com/xtls/xray-core/infra/conf"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestSubscriptionHwidReplacementRemovesRealNodes(t *testing.T) {
	router, subID := initHwidSubRouter(t, 2, WithSUBJsonUserAgentRegex("(?i)xray"))
	settingsService := &service.SettingService{}
	settings, err := settingsService.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if settings.SubHwidRejectWithConfig {
		t.Fatal("replacement must be opt-in")
	}
	settings.SubHwidRejectWithConfig = true
	if err := settingsService.UpdateAllSetting(settings, service.SecretClears{}); err != nil {
		t.Fatal(err)
	}
	if rec := requestSub(t, router, http.MethodGet, "/sub/"+subID, "device-owner", ""); rec.Code != http.StatusOK {
		t.Fatalf("register owner = %d", rec.Code)
	}
	if rec := requestSub(t, router, http.MethodGet, "/sub/"+subID, "device-rejected", ""); rec.Code != http.StatusOK {
		t.Fatalf("register second device = %d", rec.Code)
	}
	var removed model.ClientHwid
	if err := database.GetDB().Where("sub_id = ?", subID).Order("id DESC").First(&removed).Error; err != nil {
		t.Fatal(err)
	}
	clientService := &service.ClientService{}
	if err := clientService.DeleteClientHwid("route@example.com", removed.Id); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("sub_id = ?", subID).Update("limit_hwid", 1).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, userAgent, format string
	}{
		{"/sub/", "Happ/4.0", "links"},
		{"/json/", "Happ/4.0", "json"},
		{"/json/", "", "json"},
		{"/clash/", "", "clash"},
		{"/mihomo/", "", "clash"},
		{"/clash-legacy/", "", "clash"},
		{"/sub/", "Mihomo/1.0", "clash"},
		{"/sub/", "Xray/1.0", "json-array"},
	} {
		t.Run(tc.path+tc.userAgent, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path+subID+"?view=raw", nil)
			req.Header.Set("X-HWID", "device-rejected")
			req.Header.Set("User-Agent", tc.userAgent)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || rec.Header().Get("X-Hwid-Active") != "true" || rec.Header().Get("X-Hwid-Max-Devices-Reached") != "" || rec.Header().Get("X-Hwid-Limit") != "" {
				t.Fatalf("rejected device = %d, headers=%v, body=%s", rec.Code, rec.Header(), rec.Body)
			}
			body := rec.Body.String()
			switch tc.format {
			case "links":
				link, err := base64.StdEncoding.DecodeString(body)
				if err != nil {
					t.Fatal(err)
				}
				body = string(link)
				if !strings.HasPrefix(body, "vless://") || !strings.Contains(body, "@192.0.2.1:1?") {
					t.Fatalf("replacement link = %s", body)
				}
			case "json", "json-array":
				if tc.format == "json-array" {
					var configs []json.RawMessage
					if err := json.Unmarshal([]byte(body), &configs); err != nil || len(configs) != 1 {
						t.Fatalf("auto JSON array = %s, err=%v", body, err)
					}
					body = string(configs[0])
				}
				var config conf.Config
				if err := json.Unmarshal([]byte(body), &config); err != nil {
					t.Fatal(err)
				}
				if len(config.OutboundConfigs) != 1 || config.OutboundConfigs[0].Protocol != "blackhole" {
					t.Fatalf("JSON must block all traffic: %s", body)
				}
				if _, err := config.Build(); err != nil {
					t.Fatalf("Xray rejected replacement config: %v", err)
				}
			case "clash":
				var profile struct {
					Proxies []struct {
						Server string `yaml:"server"`
						Port   int    `yaml:"port"`
					} `yaml:"proxies"`
					Rules []string `yaml:"rules"`
				}
				if err := yaml.Unmarshal(rec.Body.Bytes(), &profile); err != nil {
					t.Fatal(err)
				}
				if len(profile.Proxies) != 1 || profile.Proxies[0].Server != "192.0.2.1" || profile.Proxies[0].Port != 1 || !reflect.DeepEqual(profile.Rules, []string{"MATCH,HWID-BLOCKED"}) || strings.Contains(body, "DIRECT") {
					t.Fatalf("Clash must only route to the dummy node: %s", body)
				}
			}
			if strings.Contains(body, "11111111-2222-4333-8444-555555555555") {
				t.Fatalf("replacement leaked real credentials: %s", body)
			}
		})
	}
	owner := requestSub(t, router, http.MethodGet, "/sub/"+subID, "device-owner", "")
	ownerLinks, err := base64.StdEncoding.DecodeString(owner.Body.String())
	if err != nil || owner.Code != http.StatusOK || !strings.Contains(string(ownerLinks), "11111111-2222-4333-8444-555555555555") {
		t.Fatalf("owner lost real config: %d %s, err=%v", owner.Code, ownerLinks, err)
	}
	if rec := requestSub(t, router, http.MethodGet, "/sub/unknown", "device-rejected", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown subscription = %d, want 404", rec.Code)
	}
}

func initHwidSubRouter(t *testing.T, limit int, options ...SUBControllerOption) (*gin.Engine, string) {
	t.Helper()
	tmp := t.TempDir()
	t.Chdir(tmp)
	if err := os.MkdirAll("internal/web/dist", 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	if err := os.WriteFile("internal/web/dist/subpage.html", []byte("<html><head></head><body></body></html>"), 0o644); err != nil {
		t.Fatalf("write subpage: %v", err)
	}

	t.Setenv("XUI_DB_FOLDER", tmp)
	dbtest.InitDB(t, filepath.Join(tmp, "x-ui.db"))

	const subID = "sub-hwid-route"
	const email = "route@example.com"
	const uuid = "11111111-2222-4333-8444-555555555555"
	db := database.GetDB()
	ib := &model.Inbound{
		UserId:         1,
		Tag:            "hwid-sub",
		Enable:         true,
		Port:           443,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := &model.ClientRecord{Email: email, SubID: subID, UUID: uuid, Enable: true, LimitHwid: limit}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client inbound: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	defaults := []SUBControllerOption{
		WithSUBPath("/sub/"),
		WithSUBJsonPath("/json/"),
		WithSUBClashPath("/clash/"),
		WithSUBClashAutoDetect(true),
		WithSUBJsonAutoDetect(true),
		WithSUBJsonEnabled(true),
		WithSUBClashEnabled(true),
	}
	controller := NewSUBController(router.Group("/"), append(defaults, options...)...)
	t.Cleanup(controller.Close)
	return router, subID
}

func TestSubscriptionHwidReplacementForMissingHwidAndAutoJSON(t *testing.T) {
	router, subID := initHwidSubRouter(t, 1,
		WithSUBEncryption(false),
		WithSUBJsonUserAgentRegex("(?i)xray"),
		WithSUBJsonAlwaysArray(true),
	)
	if err := database.GetDB().Create(&model.Setting{Key: "subHwidRejectWithConfig", Value: "true"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/sub/", "/json/", "/clash/"} {
		rec := requestSub(t, router, http.MethodGet, path+subID, "", "")
		if rec.Code != http.StatusOK || rec.Header().Get("X-Hwid-Active") != "true" || rec.Header().Get("X-Hwid-Not-Supported") != "" {
			t.Fatalf("%s missing HWID = %d %v", path, rec.Code, rec.Header())
		}
		if path == "/sub/" && !strings.HasPrefix(rec.Body.String(), "vless://") {
			t.Fatalf("unencrypted replacement = %s", rec.Body)
		}
		if path == "/json/" && !strings.HasPrefix(rec.Body.String(), "[") {
			t.Fatalf("JSON array setting ignored: %s", rec.Body)
		}
		head := requestSub(t, router, http.MethodHead, path+subID, "", "")
		if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Type") != rec.Header().Get("Content-Type") {
			t.Fatalf("%s HEAD = %d %v %q", path, head.Code, head.Header(), head.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/sub/"+subID, nil)
	req.Header.Set("User-Agent", "Xray/1.0")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var configs []json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &configs); err != nil || len(configs) != 1 {
		t.Fatalf("auto JSON = %s, error=%v", rec.Body, err)
	}
	assertHwidStatus(t, requestSub(t, router, http.MethodGet, "/sub/"+subID+"/hwid-status", "", ""), true, 1, 0, 1, false)
}

func requestSub(t *testing.T, router *gin.Engine, method string, path string, hwid string, accept string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Host = "sub.example.com"
	if hwid != "" {
		req.Header.Set("X-HWID", hwid)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestSubscriptionHwidGateAcrossBodyRoutes(t *testing.T) {
	router, subID := initHwidSubRouter(t, 1)

	// ?view=raw only tells /json/ and /clash/ to serve the body instead of the
	// HTML page, so it stays gated like the plain route (#GHSA-7ww3).
	bodyRoutes := []string{
		"/sub/" + subID,
		"/json/" + subID,
		"/clash/" + subID,
		"/json/" + subID + "?view=raw",
		"/clash/" + subID + "?view=RaW",
	}

	for _, path := range bodyRoutes {
		rec := requestSub(t, router, http.MethodGet, path, "", "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s missing HWID status = %d, want 404", path, rec.Code)
		}
		if rec.Header().Get("X-Hwid-Active") != "true" || rec.Header().Get("X-Hwid-Not-Supported") != "true" {
			t.Fatalf("%s missing HWID headers = %#v", path, rec.Header())
		}
	}

	rec := requestSub(t, router, http.MethodHead, "/sub/"+subID, "", "")
	if rec.Code != http.StatusNotFound || rec.Header().Get("X-Hwid-Not-Supported") != "true" {
		t.Fatalf("HEAD missing HWID = %d %#v", rec.Code, rec.Header())
	}

	for _, path := range bodyRoutes {
		rec = requestSub(t, router, http.MethodGet, path, "device-one", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s registered HWID status = %d, body=%q", path, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("X-Hwid-Active") != "true" {
			t.Fatalf("%s allowed response missing active HWID header", path)
		}
	}

	for _, path := range bodyRoutes {
		rec = requestSub(t, router, http.MethodGet, path, "device-two", "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s new HWID after limit status = %d, want 404", path, rec.Code)
		}
		if rec.Header().Get("X-Hwid-Max-Devices-Reached") != "true" || rec.Header().Get("X-Hwid-Limit") != "true" {
			t.Fatalf("%s limit headers missing: %#v", path, rec.Header())
		}
	}
}

func TestSubscriptionHwidGateSkipsHtmlInfoPage(t *testing.T) {
	router, subID := initHwidSubRouter(t, 1)

	rec := requestSub(t, router, http.MethodGet, "/sub/"+subID, "", "text/html")
	if rec.Code != http.StatusOK {
		t.Fatalf("HTML sub page status = %d, want 200, body=%q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Hwid-Not-Supported") != "" {
		t.Fatalf("HTML sub page should not be HWID-gated: %#v", rec.Header())
	}
}

// Decoding into a map rather than the service struct keeps the exact field set
// asserted, so an extra field leaking into the response fails the test.
func assertHwidStatus(t *testing.T, rec *httptest.ResponseRecorder, active bool, limit, registered, remaining int, full bool) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("hwid-status status = %d, body=%q", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode hwid-status body %q: %v", rec.Body.String(), err)
	}
	want := map[string]any{
		"active":     active,
		"limit":      float64(limit),
		"registered": float64(registered),
		"remaining":  float64(remaining),
		"full":       full,
	}
	if len(got) != len(want) {
		t.Fatalf("hwid-status fields = %#v, want exactly %#v", got, want)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("hwid-status[%q] = %#v, want %#v (body %#v)", key, got[key], value, got)
		}
	}
}

func TestSubscriptionHwidStatusCountsRegisteredDevices(t *testing.T) {
	router, subID := initHwidSubRouter(t, 2)
	statusPath := "/sub/" + subID + "/hwid-status"

	assertHwidStatus(t, requestSub(t, router, http.MethodGet, statusPath, "", ""), true, 2, 0, 2, false)

	for i, hwid := range []string{"device-one", "device-two"} {
		if rec := requestSub(t, router, http.MethodGet, "/sub/"+subID, hwid, ""); rec.Code != http.StatusOK {
			t.Fatalf("register %s = %d, want 200", hwid, rec.Code)
		}
		registered := i + 1
		rec := requestSub(t, router, http.MethodGet, statusPath, "", "")
		assertHwidStatus(t, rec, true, 2, registered, 2-registered, registered == 2)
	}

	if rec := requestSub(t, router, http.MethodHead, statusPath, "", ""); rec.Code != http.StatusOK {
		t.Fatalf("HEAD hwid-status = %d, want 200", rec.Code)
	}
}

// The endpoint must stay SELECT-only: asking about slots while carrying an
// X-HWID header must not spend the slot the caller is asking about.
func TestSubscriptionHwidStatusDoesNotRegisterDevice(t *testing.T) {
	router, subID := initHwidSubRouter(t, 1)

	rec := requestSub(t, router, http.MethodGet, "/sub/"+subID+"/hwid-status", "device-probe", "")
	assertHwidStatus(t, rec, true, 1, 0, 1, false)
	for _, header := range []string{"X-Hwid-Active", "X-Hwid-Limit", "X-Hwid-Not-Supported", "X-Hwid-Max-Devices-Reached"} {
		if value := rec.Header().Get(header); value != "" {
			t.Fatalf("hwid-status leaked gate header %s = %q", header, value)
		}
	}

	var count int64
	if err := database.GetDB().Model(&model.ClientHwid{}).Where("sub_id = ?", subID).Count(&count).Error; err != nil {
		t.Fatalf("count hwids: %v", err)
	}
	if count != 0 {
		t.Fatalf("client_hwids rows after status probe = %d, want 0", count)
	}
	if rec := requestSub(t, router, http.MethodGet, "/sub/"+subID, "device-probe", ""); rec.Code != http.StatusOK {
		t.Fatalf("subscription fetch after probe = %d, want 200", rec.Code)
	}
}

// A 404 for a subId nobody was issued must not persist anything: the gate runs
// before the lookup, and an attacker can send any number of distinct subIds.
func TestSubscriptionHwidGateIgnoresUnknownSubID(t *testing.T) {
	router, _ := initHwidSubRouter(t, 0)

	for _, path := range []string{"/sub/never-issued", "/json/never-issued", "/clash/never-issued"} {
		rec := requestSub(t, router, http.MethodGet, path, "device-ghost", "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s unknown subId status = %d, want 404", path, rec.Code)
		}
	}

	var count int64
	if err := database.GetDB().Model(&model.ClientHwid{}).Where("sub_id = ?", "never-issued").Count(&count).Error; err != nil {
		t.Fatalf("count hwids: %v", err)
	}
	if count != 0 {
		t.Fatalf("client_hwids rows for an unknown subId = %d, want 0", count)
	}
}

func TestSubscriptionHwidStatusWithoutLimit(t *testing.T) {
	router, subID := initHwidSubRouter(t, 0)

	assertHwidStatus(t, requestSub(t, router, http.MethodGet, "/sub/"+subID+"/hwid-status", "", ""), false, 0, 0, 0, false)
}

// An unknown and a disabled subscription must be indistinguishable, so a
// caller cannot probe which subscription ids exist.
func TestSubscriptionHwidStatusHidesUnknownVersusDisabled(t *testing.T) {
	router, subID := initHwidSubRouter(t, 1)

	unknown := requestSub(t, router, http.MethodGet, "/sub/does-not-exist/hwid-status", "", "")
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown subId status = %d, want 404", unknown.Code)
	}

	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("sub_id = ?", subID).
		UpdateColumn("enable", false).Error; err != nil {
		t.Fatalf("disable client: %v", err)
	}
	disabled := requestSub(t, router, http.MethodGet, "/sub/"+subID+"/hwid-status", "", "")

	if disabled.Code != unknown.Code {
		t.Fatalf("disabled status = %d, unknown status = %d, want identical", disabled.Code, unknown.Code)
	}
	if disabled.Body.String() != unknown.Body.String() {
		t.Fatalf("disabled body = %q, unknown body = %q, want identical", disabled.Body.String(), unknown.Body.String())
	}
	if !reflect.DeepEqual(disabled.Header(), unknown.Header()) {
		t.Fatalf("disabled headers = %#v, unknown headers = %#v, want identical", disabled.Header(), unknown.Header())
	}
	if disabled.Body.Len() != 0 {
		t.Fatalf("404 body = %q, want empty", disabled.Body.String())
	}
}
