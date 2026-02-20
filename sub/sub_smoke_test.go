package sub

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database"
)

func TestSubRouterSmoke(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sub-smoke.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer func() { _ = database.CloseDB() }()

	s := NewServer()
	engine, err := s.initRouter()
	if err != nil {
		t.Fatalf("initRouter failed: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sub/non-existent-id", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound {
		t.Fatalf("expected configured sub route to exist, got %d", rec.Code)
	}
}
