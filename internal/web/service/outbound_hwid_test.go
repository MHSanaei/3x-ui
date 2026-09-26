package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// #6574: an outbound subscription fetch must send the id client external links
// already send (#6559), or an HWID-limited provider counts the panel twice.
func TestOutboundFetchSendsExternalSubscriptionHwid(t *testing.T) {
	setupSettingTestDB(t)

	var gotHwid string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHwid = r.Header.Get("X-HWID")
		_, _ = w.Write([]byte("outbounds:\n"))
	}))
	defer srv.Close()

	svc := NewOutboundSubscriptionService()
	sub, err := svc.Create("hwid-test", srv.URL, "", "", true, 3600, true, false, false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = svc.Delete(sub.Id) })

	if _, err := svc.Refresh(sub.Id); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	var row model.Setting
	if err := database.GetDB().Where("key = ?", "externalSubHwid").First(&row).Error; err != nil {
		t.Fatalf("no persisted externalSubHwid after the fetch: %v", err)
	}
	if gotHwid == "" || gotHwid != row.Value {
		t.Fatalf("X-HWID = %q, want the persisted externalSubHwid %q", gotHwid, row.Value)
	}
}

func TestExternalSubscriptionHwidIsStableAndPersisted(t *testing.T) {
	setupSettingTestDB(t)

	first := ExternalSubscriptionHwid()
	if first == "" {
		t.Fatal("ExternalSubscriptionHwid returned empty")
	}
	if second := ExternalSubscriptionHwid(); second != first {
		t.Fatalf("hwid not stable: %q vs %q", first, second)
	}
	var row model.Setting
	if err := database.GetDB().Where("key = ?", "externalSubHwid").First(&row).Error; err != nil {
		t.Fatalf("hwid not persisted: %v", err)
	}
	if row.Value != first {
		t.Fatalf("persisted hwid %q != returned %q", row.Value, first)
	}
}
