package sub

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// #6559: the Master panel must send a stable X-HWID when fetching external
// subscriptions, otherwise an HWID-limited donor answers 404.
func TestFetchSendsStableHwid(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	var gotHwid string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHwid = r.Header.Get("X-HWID")
		_, _ = w.Write([]byte("vless://uuid@host:443?security=none#x"))
	}))
	defer srv.Close()

	res := fetchSubscriptionLinks(srv.URL)
	if res.err != nil {
		t.Fatalf("fetch: %v", res.err)
	}
	if len(res.links) != 1 {
		t.Fatalf("links = %v", res.links)
	}
	if want := service.ExternalSubscriptionHwid(); gotHwid == "" || gotHwid != want {
		t.Fatalf("X-HWID = %q, want the panel's stable %q", gotHwid, want)
	}
}
