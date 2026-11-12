package sub

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestSubscriptionBannedHwidWithUnlimitedDevices(t *testing.T) {
	router, subID := initHwidSubRouter(t, 0)
	if r := requestSub(t, router, http.MethodGet, "/sub/"+subID, "banned-device", ""); r.Code != http.StatusOK {
		t.Fatalf("register: %d", r.Code)
	}
	svc := &service.ClientService{}
	list, err := svc.ListClientHwids("route@example.com")
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %+v, %v", list, err)
	}
	if err := svc.SetClientHwidBlocked("route@example.com", list[0].Id, true); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/sub/", "/json/", "/clash/", "/mihomo/", "/clash-legacy/"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			r := requestSub(t, router, method, path+subID, "banned-device", "")
			if r.Code != http.StatusNotFound || r.Header().Get("X-Hwid-Blocked") != "true" || r.Header().Get("X-Hwid-Active") != "true" || r.Header().Get("X-Hwid-Max-Devices-Reached") != "" || r.Body.Len() != 0 {
				t.Fatalf("%s %s: banned response: %d %v %s", method, path, r.Code, r.Header(), r.Body)
			}
		}
	}
	assertRealConfig := func(hwid string) {
		t.Helper()
		r := requestSub(t, router, http.MethodGet, "/sub/"+subID, hwid, "")
		body, err := base64.StdEncoding.DecodeString(r.Body.String())
		if err != nil || r.Code != http.StatusOK || !strings.Contains(string(body), "11111111-2222-4333-8444-555555555555") || r.Header().Get("X-Hwid-Blocked") != "" {
			t.Fatalf("allowed response: %d %s, %v", r.Code, r.Body, err)
		}
	}
	assertRealConfig("another-device")
	if r := requestSub(t, router, http.MethodGet, "/sub/unknown", "banned-device", ""); r.Code != http.StatusNotFound {
		t.Fatalf("unknown subscription: %d", r.Code)
	}
	if err := svc.SetClientHwidBlocked("route@example.com", list[0].Id, false); err != nil {
		t.Fatal(err)
	}
	assertRealConfig("banned-device")
}
