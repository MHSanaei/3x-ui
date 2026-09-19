package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// #6574: outbound fetches must carry the stable panel id so an
// HWID-limited donor lets them through (same as #6559 for client links).
func TestOutboundFetchSendsHwid(t *testing.T) {
	setupSettingTestDB(t)

	var gotHwid, gotOS string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHwid = r.Header.Get("X-HWID")
		gotOS = r.Header.Get("X-Device-OS")
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
	if gotHwid == "" {
		t.Fatal("X-HWID header missing on outbound fetch")
	}
	if gotOS != "3x-ui" {
		t.Fatalf("X-Device-OS = %q, want 3x-ui", gotOS)
	}
	guid, err := svc.settingService.GetPanelGuid()
	if err != nil {
		t.Fatalf("guid: %v", err)
	}
	if gotHwid != guid {
		t.Fatalf("sent %q != panel guid %q", gotHwid, guid)
	}

	if err := database.GetDB().Create(
		&model.Setting{Key: "externalSubSendHwid", Value: "false"}).Error; err != nil {
		t.Fatalf("opt out: %v", err)
	}
	gotHwid = "sentinel"
	if _, err := svc.Refresh(sub.Id); err != nil {
		t.Fatalf("refresh after opt-out: %v", err)
	}
	if gotHwid != "" {
		t.Fatalf("X-HWID sent despite opt-out: %q", gotHwid)
	}
}
