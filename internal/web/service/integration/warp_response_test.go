package integration

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
)

func TestDoWarpRequestCapsResponseBody(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))

	oversize := maxResponseSize + 4096
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("a"), oversize))
	}))
	defer srv.Close()

	s := &WarpService{}
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	body, err := s.doWarpRequest(req)
	if err != nil {
		t.Fatalf("doWarpRequest: %v", err)
	}
	if len(body) != maxResponseSize {
		t.Fatalf("response body not capped: got %d bytes, want %d", len(body), maxResponseSize)
	}
}
