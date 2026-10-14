package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestSponsorVisibilityPersistsAndExposesOnlyPublicBoolean(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	dbtest.InitDB(t, dbPath)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewIndexController(router.Group("/secret"))
	NewSettingController(router.Group("/secret/panel/api"))
	svc := &service.SettingService{}
	if err := svc.UpdateSecret("tgBotToken", "private-token-must-not-leak"); err != nil {
		t.Fatal(err)
	}

	checkPublic := func(want bool) {
		t.Helper()
		for range 2 {
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/secret/sponsors/enabled", nil))
			var msg struct {
				Success bool            `json:"success"`
				Obj     json.RawMessage `json:"obj"`
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &msg); err != nil {
				t.Fatal(err)
			}
			wantJSON := "false"
			if want {
				wantJSON = "true"
			}
			if resp.Code != http.StatusOK || !msg.Success || string(msg.Obj) != wantJSON {
				t.Fatalf("public response = %d %s, want only obj:%s", resp.Code, resp.Body.String(), wantJSON)
			}
			if resp.Header().Get("Cache-Control") != "no-store" || strings.Contains(resp.Body.String(), "private-token") {
				t.Fatalf("unsafe public response: %v %s", resp.Header(), resp.Body.String())
			}
		}
	}

	save := func(value *bool) {
		t.Helper()
		settings, err := svc.GetAllSetting()
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(settings)
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		if value == nil {
			delete(payload, "showSponsorAds")
		} else {
			payload["showSponsorAds"] = *value
		}
		body, err = json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/secret/panel/api/setting/update", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if !strings.Contains(resp.Body.String(), `"success":true`) {
			t.Fatalf("save = %s", resp.Body.String())
		}
	}

	checkPublic(true)
	save(nil)
	checkPublic(true)
	save(new(false))
	checkPublic(false)
	save(nil)
	checkPublic(false)
	if err := database.InitDB(dbPath); err != nil {
		t.Fatal(err)
	}
	checkPublic(false)
	settings, err := svc.GetAllSetting()
	if err != nil || settings.ShowSponsorAds {
		t.Fatalf("settings after restart/migrations = %+v, %v", settings, err)
	}
	for _, path := range []string{"/secret/sponsors", "/secret/sponsors/logo/acme.png"} {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
		if resp.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: cache header = %q", path, resp.Header().Get("Cache-Control"))
		}
		if strings.HasSuffix(path, ".png") {
			if resp.Code != http.StatusNotFound || resp.Body.Len() != 0 {
				t.Fatalf("disabled logo = %d %s", resp.Code, resp.Body.String())
			}
		} else if !strings.Contains(resp.Body.String(), `"obj":{"sponsors":[]}`) {
			t.Fatalf("disabled list = %s", resp.Body.String())
		}
	}
	save(new(true))
	checkPublic(true)
	if err := database.GetDB().Table("settings").Where("key = ?", "showSponsorAds").Update("value", "private-invalid-value").Error; err != nil {
		t.Fatal(err)
	}
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/secret/sponsors/enabled", nil))
	if resp.Body.String() != `{"success":false,"msg":"","obj":false}` {
		t.Fatalf("setting read error leaked details or omitted the boolean: %s", resp.Body.String())
	}
}
