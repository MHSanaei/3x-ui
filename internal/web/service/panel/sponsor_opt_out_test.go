package panel

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestSponsorOptOutBlocksColdAndWarmCaches(t *testing.T) {
	for _, warm := range []bool{false, true} {
		t.Run(map[bool]string{false: "cold", true: "warm"}[warm], func(t *testing.T) {
			hits := setupSponsorServer(t, `{"contact":"https://contact.example/","sponsors":[{"id":"acme","slots":["page","login","dashboard"],"until":"2099-01-01T00:00:00Z","link":"https://acme.example/","logo":"acme.png"}]}`)
			svc := &PanelService{}
			if warm {
				if _, _, err := svc.GetSponsorLogo("acme.png"); err != nil {
					t.Fatal(err)
				}
			}
			before := hits.Load()
			if err := database.GetDB().Create(&model.Setting{Key: "showSponsorAds", Value: "false"}).Error; err != nil {
				t.Fatal(err)
			}
			for range 2 {
				list, err := svc.GetSponsors()
				if err != nil || list == nil || list.Sponsors == nil || len(list.Sponsors) != 0 || list.Contact != "" {
					t.Fatalf("disabled list = %+v, %v; want empty sponsors and no contact", list, err)
				}
				data, contentType, err := svc.GetSponsorLogo("acme.png")
				if !errors.Is(err, ErrSponsorLogoUnknown) || len(data) != 0 || contentType != "" {
					t.Fatalf("disabled logo = %x, %q, %v", data, contentType, err)
				}
			}
			if got := hits.Load(); got != before {
				t.Fatalf("disabled requests reached upstream: hits = %d, want %d", got, before)
			}
			if err := database.GetDB().Model(&model.Setting{}).Where("key = ?", "showSponsorAds").Update("value", "true").Error; err != nil {
				t.Fatal(err)
			}
			list, err := svc.GetSponsors()
			if err != nil || len(list.Sponsors) != 1 || list.Contact != "https://contact.example/" {
				t.Fatalf("re-enabled list = %+v, %v", list, err)
			}
			if data, contentType, err := svc.GetSponsorLogo("acme.png"); err != nil || len(data) == 0 || contentType != "image/png" {
				t.Fatalf("re-enabled logo = %x, %q, %v", data, contentType, err)
			}
		})
	}
}

func TestDisablingSponsorsWaitsForInFlightFetch(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	releaseFetch := func() { releaseOnce.Do(func() { close(release) }) }
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		close(entered)
		<-release
		_, _ = w.Write([]byte(`{"sponsors":[]}`))
	}))
	previousURL := sponsorsURL
	sponsorsURL = srv.URL
	resetSponsorCache()
	t.Cleanup(func() {
		releaseFetch()
		srv.Close()
		sponsorsURL = previousURL
		resetSponsorCache()
	})
	svc := &PanelService{}
	readDone := make(chan error, 1)
	go func() { _, err := svc.GetSponsors(); readDone <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("sponsor fetch did not start")
	}
	settings := &service.SettingService{}
	all, err := settings.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	all.ShowSponsorAds = false
	saveDone := make(chan error, 1)
	go func() { saveDone <- settings.UpdateAllSetting(all, service.SecretClears{}) }()
	select {
	case err := <-saveDone:
		t.Fatalf("disable completed before the in-flight fetch drained: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	releaseFetch()
	if err := <-readDone; err != nil {
		t.Fatal(err)
	}
	if err := <-saveDone; err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if list, err := svc.GetSponsors(); err != nil || len(list.Sponsors) != 0 {
			t.Fatalf("disabled list = %+v, %v", list, err)
		}
		if _, _, err := svc.GetSponsorLogo("acme.png"); !errors.Is(err, ErrSponsorLogoUnknown) {
			t.Fatalf("disabled logo = %v", err)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("upstream requests after disable: %d, want only the pre-disable request", got)
	}
}

func TestSponsorSettingErrorDoesNotServeCachedContent(t *testing.T) {
	hits := setupSponsorServer(t, `{"sponsors":[{"id":"acme","slots":["page"],"until":"2099-01-01T00:00:00Z","link":"https://acme.example/","logo":"acme.png"}]}`)
	svc := &PanelService{}
	if _, _, err := svc.GetSponsorLogo("acme.png"); err != nil {
		t.Fatal(err)
	}
	before := hits.Load()
	if err := database.GetDB().Create(&model.Setting{Key: "showSponsorAds", Value: "invalid"}).Error; err != nil {
		t.Fatal(err)
	}
	if list, err := svc.GetSponsors(); err == nil || list != nil {
		t.Fatalf("invalid setting served cached list: %+v, %v", list, err)
	}
	if data, _, err := svc.GetSponsorLogo("acme.png"); err == nil || len(data) != 0 {
		t.Fatalf("invalid setting served cached logo: %x, %v", data, err)
	}
	if got := hits.Load(); got != before {
		t.Fatalf("invalid setting reached upstream: hits = %d, want %d", got, before)
	}
}
