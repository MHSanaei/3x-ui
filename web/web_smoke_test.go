package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/web/global"
	"github.com/robfig/cron/v3"
)

func TestRouterSmokeAuthAndCoreRoutes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "web-smoke.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer func() {
		_ = database.CloseDB()
	}()

	s := NewServer()
	s.cron = cron.New(cron.WithSeconds())
	global.SetWebServer(s)
	engine, err := s.initRouter()
	if err != nil {
		t.Fatalf("initRouter failed: %v", err)
	}

	// Login page should be reachable.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected GET / to return 200, got %d", rec.Code)
	}

	// Unauthenticated API request should be hidden as 404.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/panel/api/server/status", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected unauthenticated API to return 404, got %d", rec.Code)
	}

	// Panel root requires auth and should redirect.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/panel/", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected unauthenticated panel route to redirect, got %d", rec.Code)
	}

	for _, path := range []string{"/panel/inbounds", "/panel/settings", "/panel/xray"} {
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, path, nil)
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusTemporaryRedirect {
			t.Fatalf("expected unauthenticated %s to redirect, got %d", path, rec.Code)
		}
	}
}
