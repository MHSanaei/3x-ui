package panel

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

var sponsorTestNow = time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

func validSponsor() Sponsor {
	return Sponsor{
		ID:    "acme",
		Name:  "Acme",
		Slots: []string{"dashboard", "sidebar"},
		Until: sponsorTestNow.Add(24 * time.Hour),
		Logo:  "acme.png",
		Link:  "https://acme.example/",
	}
}

func TestActiveSponsorsFilters(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Sponsor)
		kept   bool
	}{
		{"valid", func(*Sponsor) {}, true},
		{"expired", func(s *Sponsor) { s.Until = sponsorTestNow }, false},
		{"enable false with future until", func(s *Sponsor) { s.Enable = new(false) }, false},
		{"enable true without until", func(s *Sponsor) { s.Enable, s.Until = new(true), time.Time{} }, false},
		{"enable true with future until", func(s *Sponsor) { s.Enable = new(true) }, true},
		{"missing id", func(s *Sponsor) { s.ID = "" }, false},
		{"http link", func(s *Sponsor) { s.Link = "http://acme.example/" }, false},
		{"javascript link", func(s *Sponsor) { s.Link = "javascript:alert(1)" }, false},
		{"only unknown slots", func(s *Sponsor) { s.Slots = []string{"subpage"} }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sp := validSponsor()
			tc.mutate(&sp)
			got := activeSponsors(&SponsorList{Sponsors: []Sponsor{sp}}, sponsorTestNow)
			if kept := len(got.Sponsors) == 1; kept != tc.kept {
				t.Fatalf("kept = %v, want %v", kept, tc.kept)
			}
		})
	}
}

func TestActiveSponsorsCapsSidebarAtThree(t *testing.T) {
	raw := &SponsorList{}
	for _, id := range []string{"expired", "a", "b", "c", "d", "e"} {
		sp := validSponsor()
		sp.ID = id
		sp.Slots = []string{"sidebar", "page"}
		if id == "expired" {
			sp.Until = sponsorTestNow
		}
		if id == "e" {
			sp.Slots = []string{"sidebar"}
		}
		raw.Sponsors = append(raw.Sponsors, sp)
	}
	got := activeSponsors(raw, sponsorTestNow)
	want := map[string][]string{
		"a": {"sidebar", "page"}, "b": {"sidebar", "page"}, "c": {"sidebar", "page"}, "d": {"page"},
	}
	if len(got.Sponsors) != len(want) {
		t.Fatalf("got %d sponsors, want %d (e has only sidebar and must drop)", len(got.Sponsors), len(want))
	}
	for _, sp := range got.Sponsors {
		if !slices.Equal(sp.Slots, want[sp.ID]) {
			t.Errorf("%s slots = %v, want %v", sp.ID, sp.Slots, want[sp.ID])
		}
	}
}

func TestActiveSponsorsLogoName(t *testing.T) {
	cases := []struct{ logo, want string }{
		{"acme.png", "/sponsors/logo/acme.png"},
		{"VPS.png", "/sponsors/logo/VPS.png"},
		{"../x.png", ""},
		{"https://evil.example/x.png", ""},
		{"..png", ""},
		{"logo.svg", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.logo, func(t *testing.T) {
			sp := validSponsor()
			sp.Logo = tc.logo
			got := activeSponsors(&SponsorList{Sponsors: []Sponsor{sp}}, sponsorTestNow)
			if len(got.Sponsors) != 1 {
				t.Fatalf("sponsor dropped for logo %q; want it kept", tc.logo)
			}
			if got.Sponsors[0].Logo != tc.want {
				t.Errorf("logo = %q, want %q", got.Sponsors[0].Logo, tc.want)
			}
		})
	}
}

func TestActiveSponsorsResolvesLogoAndSlots(t *testing.T) {
	sp := validSponsor()
	sp.Slots = []string{"subpage", "login"}
	got := activeSponsors(&SponsorList{Contact: "javascript:x", Sponsors: []Sponsor{sp}}, sponsorTestNow)
	if len(got.Sponsors) != 1 {
		t.Fatalf("got %d sponsors, want 1", len(got.Sponsors))
	}
	if want := "/sponsors/logo/acme.png"; got.Sponsors[0].Logo != want {
		t.Errorf("logo = %q, want %q", got.Sponsors[0].Logo, want)
	}
	if s := got.Sponsors[0].Slots; len(s) != 1 || s[0] != "login" {
		t.Errorf("slots = %v, want [login]", s)
	}
	if got.Contact != "" {
		t.Errorf("contact = %q, want empty for non-https", got.Contact)
	}
}

func setupSponsorServer(t *testing.T, body string) *atomic.Int32 {
	t.Helper()
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		isLogo := strings.HasPrefix(r.URL.Path, "/media/")
		if (isLogo && failSponsorLogos.Load()) || (!isLogo && failSponsorList.Load()) {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		switch r.URL.Path {
		case "/media/acme.png":
			_, _ = w.Write(pngMagic)
		case "/media/fake.png":
			_, _ = w.Write([]byte("<svg onload=alert(1)>"))
		default:
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(srv.Close)

	prevURL, prevBase, prevNow := sponsorsURL, sponsorLogoBase, sponsorNow
	sponsorsURL, sponsorLogoBase = srv.URL+"/sponsors.json", srv.URL+"/media/"
	resetSponsorCache()
	t.Cleanup(func() {
		sponsorsURL, sponsorLogoBase, sponsorNow = prevURL, prevBase, prevNow
		failSponsorList.Store(false)
		failSponsorLogos.Store(false)
		resetSponsorCache()
	})
	return &hits
}

var failSponsorList, failSponsorLogos atomic.Bool

func TestGetSponsorsKeepsLastListWhenRefreshFails(t *testing.T) {
	hits := setupSponsorServer(t, `{"sponsors":[{"id":"acme","slots":["page"],
		"until":"2099-01-01T00:00:00Z","link":"https://acme.example/"}]}`)
	now := sponsorTestNow
	sponsorNow = func() time.Time { return now }
	svc := &PanelService{}
	if got, err := svc.GetSponsors(); err != nil || len(got.Sponsors) != 1 {
		t.Fatalf("first call = %+v, %v; want 1 sponsor", got, err)
	}

	failSponsorList.Store(true)
	now = now.Add(sponsorsTTL)
	got, err := svc.GetSponsors()
	if err != nil || len(got.Sponsors) != 1 || got.Sponsors[0].ID != "acme" {
		t.Fatalf("after failed refresh = %+v, %v; want the last good sponsor kept", got, err)
	}
	now = now.Add(sponsorsErrTTL - time.Second)
	if _, err := svc.GetSponsors(); err != nil {
		t.Fatal(err)
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("remote hits = %d, want 2 (failed refresh retried only after sponsorsErrTTL)", n)
	}
}

func TestGetSponsorLogoCachesFailuresAndKeepsLastImage(t *testing.T) {
	hits := setupSponsorServer(t, `{"sponsors":[
		{"id":"acme","slots":["page"],"until":"2099-01-01T00:00:00Z","link":"https://a.example/","logo":"acme.png"},
		{"id":"fake","slots":["page"],"until":"2099-01-01T00:00:00Z","link":"https://f.example/","logo":"fake.png"}]}`)
	now := sponsorTestNow
	sponsorNow = func() time.Time { return now }
	svc := &PanelService{}

	const wantErr = "sponsor logo fake.png has content type text/plain; charset=utf-8"
	for range 2 {
		if _, _, err := svc.GetSponsorLogo("fake.png"); err == nil || err.Error() != wantErr {
			t.Fatalf("fake.png err = %v, want %q", err, wantErr)
		}
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("remote hits = %d, want 2 (list + one fake.png fetch; the failure must be cached)", n)
	}

	if _, _, err := svc.GetSponsorLogo("acme.png"); err != nil {
		t.Fatal(err)
	}
	failSponsorLogos.Store(true)
	now = now.Add(sponsorsTTL)
	data, ctype, err := svc.GetSponsorLogo("acme.png")
	if err != nil || ctype != "image/png" || string(data) != string(pngMagic) {
		t.Fatalf("acme.png after failed refresh = %q, %q, %v; want the last good image", data, ctype, err)
	}
}

func resetSponsorCache() {
	sponsorsMu.Lock()
	defer sponsorsMu.Unlock()
	sponsorsRaw, sponsorsErr, sponsorsRetryAt = nil, nil, time.Time{}
	logosMu.Lock()
	defer logosMu.Unlock()
	logos = map[string]sponsorLogo{}
}

var pngMagic = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func TestGetSponsorLogo(t *testing.T) {
	hits := setupSponsorServer(t, `{"sponsors":[
		{"id":"acme","slots":["page"],"until":"2099-01-01T00:00:00Z","link":"https://a.example/","logo":"acme.png"},
		{"id":"fake","slots":["page"],"until":"2099-01-01T00:00:00Z","link":"https://f.example/","logo":"fake.png"},
		{"id":"old","slots":["page"],"until":"2000-01-01T00:00:00Z","link":"https://o.example/","logo":"old.png"}]}`)
	svc := &PanelService{}

	data, ctype, err := svc.GetSponsorLogo("acme.png")
	if err != nil || ctype != "image/png" || string(data) != string(pngMagic) {
		t.Fatalf("acme.png = %q, %q, %v; want png bytes", data, ctype, err)
	}
	before := hits.Load()
	if _, _, err := svc.GetSponsorLogo("acme.png"); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != before {
		t.Fatalf("second logo fetch hit the remote; want cached")
	}

	for _, name := range []string{"old.png", "other.png", "../sponsors.json"} {
		if _, _, err := svc.GetSponsorLogo(name); !errors.Is(err, ErrSponsorLogoUnknown) {
			t.Errorf("%s: err = %v, want ErrSponsorLogoUnknown", name, err)
		}
	}
	_, _, err = svc.GetSponsorLogo("fake.png")
	if want := "sponsor logo fake.png has content type text/plain; charset=utf-8"; err == nil || err.Error() != want {
		t.Errorf("fake.png: err = %v, want %q", err, want)
	}
}

func TestGetSponsorsCachesAndExpiresWhileCached(t *testing.T) {
	hits := setupSponsorServer(t, `{"sponsors":[{"id":"acme","slots":["page"],
		"until":"2026-10-15T00:30:00Z","link":"https://acme.example/"}]}`)
	now := sponsorTestNow
	sponsorNow = func() time.Time { return now }
	svc := &PanelService{}

	got, err := svc.GetSponsors()
	if err != nil || len(got.Sponsors) != 1 {
		t.Fatalf("first call = %+v, %v; want 1 sponsor", got, err)
	}
	now = now.Add(45 * time.Minute)
	got, err = svc.GetSponsors()
	if err != nil || len(got.Sponsors) != 0 {
		t.Fatalf("after until = %+v, %v; want 0 sponsors", got, err)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("remote hits = %d, want 1 (cached within TTL)", n)
	}
	now = now.Add(sponsorsTTL)
	if _, err := svc.GetSponsors(); err != nil {
		t.Fatal(err)
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("remote hits after TTL = %d, want 2", n)
	}
}

func TestGetSponsorsRejectsOversizeBody(t *testing.T) {
	setupSponsorServer(t, `{"contact":"`+strings.Repeat("a", maxSponsorsBytes)+`"}`)
	_, err := (&PanelService{}).GetSponsors()
	want := sponsorsURL + " exceeds 262144 bytes"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestGetSponsorsDebugReadsLocalCheckout(t *testing.T) {
	hits := setupSponsorServer(t, `{"sponsors":[]}`)
	t.Setenv("XUI_DEBUG", "true")
	root := t.TempDir()
	local := filepath.Join(root, "sponsors", "3X")
	if err := os.MkdirAll(local, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "3x-ui"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "3x-ui"))
	write := func(id string) {
		t.Helper()
		body := `{"sponsors":[{"id":"` + id + `","slots":["page"],"until":"2099-01-01T00:00:00Z","link":"https://a.example/"}]}`
		if err := os.WriteFile(filepath.Join(local, "sponsors.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	svc := &PanelService{}
	for _, id := range []string{"first", "edited"} {
		write(id)
		got, err := svc.GetSponsors()
		if err != nil || len(got.Sponsors) != 1 || got.Sponsors[0].ID != id {
			t.Fatalf("GetSponsors() = %+v, %v; want local sponsor %q", got, err, id)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("remote hits = %d, want 0 in debug mode", n)
	}
}
