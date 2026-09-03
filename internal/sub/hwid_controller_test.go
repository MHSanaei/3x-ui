package sub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func initHwidSubRouter(t *testing.T, limit int) (*gin.Engine, string) {
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
	if err := database.InitDB(filepath.Join(tmp, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

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
	NewSUBController(
		router.Group("/"),
		WithSUBPath("/sub/"),
		WithSUBJsonPath("/json/"),
		WithSUBClashPath("/clash/"),
		WithSUBClashAutoDetect(true),
		WithSUBJsonAutoDetect(true),
		WithSUBJsonEnabled(true),
		WithSUBClashEnabled(true),
	)
	return router, subID
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

	for _, path := range []string{"/sub/" + subID, "/json/" + subID, "/clash/" + subID} {
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

	for _, path := range []string{"/sub/" + subID, "/json/" + subID, "/clash/" + subID} {
		rec = requestSub(t, router, http.MethodGet, path, "device-one", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s registered HWID status = %d, body=%q", path, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("X-Hwid-Active") != "true" {
			t.Fatalf("%s allowed response missing active HWID header", path)
		}
	}

	rec = requestSub(t, router, http.MethodGet, "/json/"+subID, "device-two", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("new HWID after limit status = %d, want 404", rec.Code)
	}
	if rec.Header().Get("X-Hwid-Max-Devices-Reached") != "true" || rec.Header().Get("X-Hwid-Limit") != "true" {
		t.Fatalf("limit headers missing: %#v", rec.Header())
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
