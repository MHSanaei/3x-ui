package sub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func seedInfoEndpointSub(t *testing.T, subId, email string) {
	t.Helper()
	db := database.GetDB()
	rec := &model.ClientRecord{Email: email, SubID: subId, UUID: "info-uuid", Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	link := "vless://11111111-1111-1111-1111-111111111111@example.com:443?type=tcp&security=reality&pbk=abc&sid=12&fp=chrome#orig"
	if err := db.Create(&model.ClientExternalLink{ClientId: rec.Id, Kind: model.ExternalLinkKindLink, Value: link, Remark: "DE-Provider", SortIndex: 1}).Error; err != nil {
		t.Fatalf("seed external link: %v", err)
	}
}

func TestSubInfoEndpoint_ServesStatusJSONEvenForBrowsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initSubDB(t)
	seedInfoEndpointSub(t, "info-sub", "info@x")

	router := gin.New()
	NewSUBController(router.Group("/"))

	req := httptest.NewRequest(http.MethodGet, "/sub/info-sub?format=info", nil)
	req.Host = "sub.example.com"
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Fatalf("Cache-Control = %q, want a no-store directive", cc)
	}

	var info map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatalf("body is not valid JSON: %v; body=%s", err, w.Body.String())
	}
	if info["sId"] != "info-sub" {
		t.Fatalf("sId = %v, want %q", info["sId"], "info-sub")
	}
	if _, hasLinks := info["links"]; hasLinks {
		t.Fatal("info payload must not include the links list")
	}
	if isOnline, ok := info["isOnline"].(bool); !ok || isOnline {
		t.Fatalf("isOnline = %v, want false with no live xray", info["isOnline"])
	}
	emails, ok := info["emails"].([]any)
	if !ok || len(emails) != 1 || emails[0] != "info@x" {
		t.Fatalf("emails = %v, want [info@x]", info["emails"])
	}
	subUrl, _ := info["subUrl"].(string)
	if !strings.HasSuffix(subUrl, "/sub/info-sub") {
		t.Fatalf("subUrl = %q, want a /sub/info-sub suffix", subUrl)
	}
	for _, key := range []string{"enabled", "used", "remained", "expire", "lastOnline", "datepicker", "announce"} {
		if _, present := info[key]; !present {
			t.Fatalf("info payload missing %q; body=%s", key, w.Body.String())
		}
	}
	if info["subUpdates"] != float64(12) {
		t.Fatalf("subUpdates = %v, want the default 12-hour interval", info["subUpdates"])
	}
}

func TestSubInfoEndpoint_UnknownSubIs404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initSubDB(t)

	router := gin.New()
	NewSUBController(router.Group("/"))

	req := httptest.NewRequest(http.MethodGet, "/sub/does-not-exist?format=info", nil)
	req.Host = "sub.example.com"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestSubInfoEndpoint_HTMLPageStillWinsWithoutFormatParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initSubDB(t)
	seedInfoEndpointSub(t, "html-sub", "html@x")
	oldDistFS := distFS
	distFS = testDistFS
	t.Cleanup(func() { distFS = oldDistFS })

	router := gin.New()
	NewSUBController(router.Group("/"))

	req := httptest.NewRequest(http.MethodGet, "/sub/html-sub", nil)
	req.Host = "sub.example.com"
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html for a browser request", ct)
	}
	if !strings.Contains(w.Body.String(), "__SUB_PAGE_DATA__") {
		t.Fatal("browser request must still get the SPA page with injected page data")
	}
	if !strings.Contains(w.Body.String(), `"isOnline":false`) {
		t.Fatalf("injected page data must carry isOnline; body=%s", w.Body.String())
	}
}

func TestSubPageDataNamesTheFormatOfTheVisitedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initSubDB(t)
	seedInfoEndpointSub(t, "fmt-sub", "fmt@x")
	for key, value := range map[string]string{
		"subPath": "/sub/", "subJsonPath": "/json/", "subClashPath": "/clash/",
		"subJsonEnable": "true", "subClashEnable": "true",
	} {
		if err := database.GetDB().Where("key = ?", key).Delete(&model.Setting{}).Error; err != nil {
			t.Fatalf("clear %s: %v", key, err)
		}
		if err := database.GetDB().Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}
	oldDistFS := distFS
	distFS = testDistFS
	t.Cleanup(func() { distFS = oldDistFS })

	router, err := (&Server{}).initRouter()
	if err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]string{
		"/sub/fmt-sub":          "raw",
		"/json/fmt-sub":         "json",
		"/clash/fmt-sub":        "clash",
		"/mihomo/fmt-sub":       "clash",
		"/clash-legacy/fmt-sub": "clash",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Host = "sub.example.com"
			req.Header.Set("Accept", "text/html")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
			}
			if marker := `"subFormat":"` + want + `"`; !strings.Contains(w.Body.String(), marker) {
				t.Fatalf("page data lacks %s; body=%s", marker, w.Body.String())
			}
		})
	}
}
