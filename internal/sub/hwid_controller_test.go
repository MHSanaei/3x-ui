package sub

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		"/sub/",
		"/json/",
		"/clash/",
		true,
		true,
		false,
		"",
		"",
		"",
		"",
		"",
		false,
		"",
		"",
		"",
		"",
		"",
		false,
		"",
		false,
		false,
		"",
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
