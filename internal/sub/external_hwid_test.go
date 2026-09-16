package sub

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// #6559: the Master panel must send a stable X-HWID when fetching external
// subscriptions, otherwise an HWID-limited donor answers 404.
func TestServerHwidStableAcrossCalls(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	first := serverHwid()
	if first == "" {
		t.Fatal("serverHwid returned empty")
	}
	if !strings.HasPrefix(first, "3x-ui-server-") {
		t.Fatalf("unexpected hwid format: %q", first)
	}
	if len(first) < 6 {
		t.Fatalf("hwid too short for donor minHwidLength: %q", first)
	}

	second := serverHwid()
	if second != first {
		t.Fatalf("hwid not stable: %q vs %q", first, second)
	}

	var row model.Setting
	if err := database.GetDB().Where("key = ?", serverHwidKey).First(&row).Error; err != nil {
		t.Fatalf("hwid not persisted: %v", err)
	}
	if row.Value != first {
		t.Fatalf("persisted hwid %q != returned %q", row.Value, first)
	}
}

// The fetch must carry the stable id by default so an HWID-limited donor
// lets it through, and must drop it when the operator opts out.
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
	if gotHwid == "" {
		t.Fatal("X-HWID header missing on fetch")
	}
	if gotHwid != serverHwid() {
		t.Fatalf("sent %q != stable %q", gotHwid, serverHwid())
	}

	if err := database.GetDB().Create(
		&model.Setting{Key: sendHwidKey, Value: "false"}).Error; err != nil {
		t.Fatalf("opt out: %v", err)
	}
	gotHwid = "sentinel"
	fetchSubscriptionLinks(srv.URL + "/other")
	if gotHwid != "" {
		t.Fatalf("X-HWID sent despite opt-out: %q", gotHwid)
	}
}
