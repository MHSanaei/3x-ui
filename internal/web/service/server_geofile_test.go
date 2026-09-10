package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

// Loyalsoldier and runetfreedom write "<hash>  geoip.dat"; chocolate4u writes
// "<hash>  release/geoip.dat", the path from its own build.
func TestParseGeofileDigest(t *testing.T) {
	const digest = "0d5d2ba0c5a5c58027fd1347a6afd57c9470799b6bb3cbc274fd4657ed8de382"

	for _, tc := range []struct {
		name  string
		sums  string
		asset string
		want  string
	}{
		{"bare-name", digest + "  geoip.dat\n", "geoip.dat", digest},
		{"build-path", digest + "  release/geoip.dat\n", "geoip.dat", digest},
		{"binary-mode-marker", digest + "  *geoip.dat\n", "geoip.dat", digest},
		{"uppercase-digest", strings.ToUpper(digest) + "  geoip.dat\n", "geoip.dat", digest},
		{"picks-matching-line", "aaaa  geosite.dat\n" + digest + "  geoip.dat\n", "geoip.dat", digest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGeofileDigest([]byte(tc.sums), tc.asset)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got != tc.want {
				t.Fatalf("digest = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseGeofileDigest_Errors(t *testing.T) {
	const digest = "0d5d2ba0c5a5c58027fd1347a6afd57c9470799b6bb3cbc274fd4657ed8de382"

	for _, tc := range []struct {
		name    string
		sums    string
		asset   string
		wantErr string
	}{
		// Accepting this would verify geoip.dat against geosite.dat's digest.
		{"names-another-asset", digest + "  geosite.dat\n", "geoip.dat", "no SHA-256 entry for geoip.dat"},
		{"empty", "", "geoip.dat", "no SHA-256 entry for geoip.dat"},
		{"malformed-short", "deadbeef  geoip.dat\n", "geoip.dat", "malformed SHA-256 entry for geoip.dat"},
		{"not-hex", strings.Repeat("z", 64) + "  geoip.dat\n", "geoip.dat", "malformed SHA-256 entry for geoip.dat"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseGeofileDigest([]byte(tc.sums), tc.asset)
			if err == nil {
				t.Fatalf("%s: expected an error", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestGeofileTagFromLocation(t *testing.T) {
	got, err := geofileTagFromLocation("https://github.com/o/r/releases/download/202609022346/geoip.dat")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != "202609022346" {
		t.Fatalf("tag = %q, want 202609022346", got)
	}
	for _, bad := range []string{
		"https://github.com/o/r/releases/latest/download/geoip.dat",
		"https://github.com/o/r/releases/download/202609022346",
		"",
	} {
		if _, err := geofileTagFromLocation(bad); err == nil {
			t.Fatalf("expected an error for %q", bad)
		}
	}
}

// fakeUpstream serves one repo's release: a `latest` download redirecting to a
// tagged asset, the asset itself, and its .sha256sum sidecar.
type fakeUpstream struct {
	repo    string
	assets  map[string]string
	corrupt map[string]bool
}

// geofileServer mounts every upstream on one test server, mimicking GitHub's
// `releases/latest/download` -> `releases/download/<tag>` redirect.
func geofileServer(t *testing.T, ups []fakeUpstream) (*httptest.Server, *sync.Map) {
	t.Helper()

	hits := &sync.Map{}
	mux := http.NewServeMux()
	for _, up := range ups {
		for asset, body := range up.assets {
			tagged := "/" + up.repo + "/releases/download/v1/" + asset

			mux.HandleFunc("/"+up.repo+"/releases/latest/download/"+asset, func(w http.ResponseWriter, r *http.Request) {
				hits.Store("latest:"+up.repo, true)
				http.Redirect(w, r, tagged, http.StatusFound)
			})
			mux.HandleFunc(tagged, func(w http.ResponseWriter, r *http.Request) {
				hits.Store("body:"+up.repo+"/"+asset, true)
				_, _ = w.Write([]byte(body))
			})

			payload := body
			if up.corrupt[asset] {
				payload = body + " tampered"
			}
			sum := sha256.Sum256([]byte(payload))
			line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), asset)
			mux.HandleFunc(tagged+".sha256sum", func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(line))
			})
		}
	}

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, hits
}

// geofileTestEnv points the service at a temp asset folder and a throwaway DB.
func geofileTestEnv(t *testing.T, entries map[string]geofileEntry) string {
	t.Helper()

	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	binFolder := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", binFolder)

	originalAllowlist := geofileAllowlist
	geofileAllowlist = entries
	t.Cleanup(func() { geofileAllowlist = originalAllowlist })

	return binFolder
}

func restartStub(t *testing.T, called *bool) {
	t.Helper()
	original := restartXrayAfterGeofileUpdate
	restartXrayAfterGeofileUpdate = func(*ServerService) error {
		*called = true
		return nil
	}
	t.Cleanup(func() { restartXrayAfterGeofileUpdate = original })
}

func TestUpdateGeofileInstallsVerifiedFile(t *testing.T) {
	srv, _ := geofileServer(t, []fakeUpstream{{repo: "a", assets: map[string]string{"geoip.dat": "good geoip payload"}}})
	binFolder := geofileTestEnv(t, map[string]geofileEntry{
		"geoip.dat": {srv.URL + "/a", "geoip.dat", "geoip.dat"},
	})

	var restarted bool
	restartStub(t, &restarted)

	if err := (&ServerService{}).UpdateGeofile(""); err != nil {
		t.Fatalf("UpdateGeofile: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(binFolder, "geoip.dat"))
	if err != nil {
		t.Fatalf("read installed geofile: %v", err)
	}
	if string(got) != "good geoip payload" {
		t.Fatalf("installed content = %q, want %q", got, "good geoip payload")
	}
	if !restarted {
		t.Fatal("a file was installed, so xray should have been restarted")
	}
}

func TestUpdateGeofileRejectsDigestMismatch(t *testing.T) {
	srv, _ := geofileServer(t, []fakeUpstream{{
		repo:    "a",
		assets:  map[string]string{"geoip.dat": "good geoip payload"},
		corrupt: map[string]bool{"geoip.dat": true},
	}})
	binFolder := geofileTestEnv(t, map[string]geofileEntry{
		"geoip.dat": {srv.URL + "/a", "geoip.dat", "geoip.dat"},
	})

	var restarted bool
	restartStub(t, &restarted)

	err := (&ServerService{}).UpdateGeofile("")
	if err == nil {
		t.Fatal("expected an error when the download does not match its published digest")
	}
	if !strings.Contains(err.Error(), "does not match the published SHA-256 checksum") {
		t.Fatalf("error = %q, want it to name the checksum mismatch", err)
	}

	if _, statErr := os.Stat(filepath.Join(binFolder, "geoip.dat")); !os.IsNotExist(statErr) {
		t.Fatalf("a file failing verification must not be installed (stat: %v)", statErr)
	}
	if restarted {
		t.Fatal("nothing was installed, so xray must not be restarted")
	}
}

// Within one upstream the pair installs together. geoip sorts before geosite
// and is staged first, so a trivially-passing "abort before download" is ruled out.
func TestUpdateGeofileInstallsNeitherFileOfAFailedUpstream(t *testing.T) {
	srv, hits := geofileServer(t, []fakeUpstream{{
		repo:    "a",
		assets:  map[string]string{"geoip.dat": "good geoip", "geosite.dat": "good geosite"},
		corrupt: map[string]bool{"geosite.dat": true},
	}})
	binFolder := geofileTestEnv(t, map[string]geofileEntry{
		"geoip.dat":   {srv.URL + "/a", "geoip.dat", "geoip.dat"},
		"geosite.dat": {srv.URL + "/a", "geosite.dat", "geosite.dat"},
	})

	var restarted bool
	restartStub(t, &restarted)

	if err := (&ServerService{}).UpdateGeofile(""); err == nil {
		t.Fatal("expected an error when one of the databases fails verification")
	}

	if _, ok := hits.Load("body:a/geoip.dat"); !ok {
		t.Fatal("geoip.dat was never downloaded, so this run never exercised staging")
	}
	for _, name := range []string{"geoip.dat", "geosite.dat"} {
		if _, statErr := os.Stat(filepath.Join(binFolder, name)); !os.IsNotExist(statErr) {
			t.Fatalf("%s was installed even though its sibling failed verification", name)
		}
	}
	if restarted {
		t.Fatal("nothing was installed, so xray must not be restarted")
	}
}

// A broken upstream must not discard a healthy one's verified download.
func TestUpdateGeofileKeepsGoodUpstreamWhenAnotherFails(t *testing.T) {
	srv, _ := geofileServer(t, []fakeUpstream{
		{repo: "aaa", assets: map[string]string{"geoip.dat": "healthy payload"}},
		{
			repo:    "zzz",
			assets:  map[string]string{"geoip.dat": "broken payload"},
			corrupt: map[string]bool{"geoip.dat": true},
		},
	})
	binFolder := geofileTestEnv(t, map[string]geofileEntry{
		"geoip.dat":    {srv.URL + "/aaa", "geoip.dat", "geoip.dat"},
		"geoip_RU.dat": {srv.URL + "/zzz", "geoip.dat", "geoip_RU.dat"},
	})

	var restarted bool
	restartStub(t, &restarted)

	err := (&ServerService{}).UpdateGeofile("")
	if err == nil {
		t.Fatal("expected an error naming the failing upstream")
	}
	if !strings.Contains(err.Error(), "geoip_RU.dat") {
		t.Fatalf("error = %q, want it to name geoip_RU.dat", err)
	}

	got, readErr := os.ReadFile(filepath.Join(binFolder, "geoip.dat"))
	if readErr != nil {
		t.Fatalf("the healthy upstream's file must still be installed: %v", readErr)
	}
	if string(got) != "healthy payload" {
		t.Fatalf("installed content = %q, want %q", got, "healthy payload")
	}
	if _, statErr := os.Stat(filepath.Join(binFolder, "geoip_RU.dat")); !os.IsNotExist(statErr) {
		t.Fatal("the failing upstream's file must not be installed")
	}
	if !restarted {
		t.Fatal("a file was installed, so xray should have been restarted")
	}
}

// The upstreams publish several times a day. Once `latest` is resolved, the
// asset and its digest must both come from that release, not from a newer one.
func TestUpdateGeofileSurvivesReleaseRotation(t *testing.T) {
	const oldBody = "release one payload"
	oldSum := sha256.Sum256([]byte(oldBody))
	newSum := sha256.Sum256([]byte("release two payload"))

	mux := http.NewServeMux()
	mux.HandleFunc("/a/releases/latest/download/geoip.dat", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/a/releases/download/v1/geoip.dat", http.StatusFound)
	})
	mux.HandleFunc("/a/releases/download/v1/geoip.dat", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(oldBody))
	})
	mux.HandleFunc("/a/releases/download/v1/geoip.dat.sha256sum", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fmt.Appendf(nil, "%s  geoip.dat\n", hex.EncodeToString(oldSum[:])))
	})
	// "latest" has already moved on to v2. Anything still resolving it gets a
	// digest for bytes we never downloaded.
	mux.HandleFunc("/a/releases/latest/download/geoip.dat.sha256sum", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fmt.Appendf(nil, "%s  geoip.dat\n", hex.EncodeToString(newSum[:])))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	binFolder := geofileTestEnv(t, map[string]geofileEntry{
		"geoip.dat": {srv.URL + "/a", "geoip.dat", "geoip.dat"},
	})

	var restarted bool
	restartStub(t, &restarted)

	if err := (&ServerService{}).UpdateGeofile(""); err != nil {
		t.Fatalf("a release landing mid-batch must not look like tampering: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(binFolder, "geoip.dat")); err != nil || string(got) != oldBody {
		t.Fatalf("installed = %q (err %v), want the pinned release's bytes", got, err)
	}
}

func TestUpdateGeofileSkipsRestartWhenNotModified(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a/releases/latest/download/geoip.dat", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/a/releases/download/v1/geoip.dat", http.StatusFound)
	})
	mux.HandleFunc("/a/releases/download/v1/geoip.dat", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Modified-Since") == "" {
			t.Errorf("expected a conditional GET carrying If-Modified-Since")
		}
		w.WriteHeader(http.StatusNotModified)
	})
	mux.HandleFunc("/a/releases/download/v1/geoip.dat.sha256sum", func(w http.ResponseWriter, r *http.Request) {
		t.Error("the sidecar must not be fetched when the asset is unchanged")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	binFolder := geofileTestEnv(t, map[string]geofileEntry{
		"geoip.dat": {srv.URL + "/a", "geoip.dat", "geoip.dat"},
	})

	existing := filepath.Join(binFolder, "geoip.dat")
	if err := os.WriteFile(existing, []byte("already current"), 0o644); err != nil {
		t.Fatalf("seed existing geofile: %v", err)
	}

	var restarted bool
	restartStub(t, &restarted)

	if err := (&ServerService{}).UpdateGeofile(""); err != nil {
		t.Fatalf("UpdateGeofile: %v", err)
	}

	if restarted {
		t.Fatal("a 304 from every upstream must not restart xray and drop client connections")
	}
	got, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("read existing geofile: %v", err)
	}
	if string(got) != "already current" {
		t.Fatalf("existing content = %q, want it left alone", got)
	}
}

func TestUpdateGeofileRejectsNameOutsideAllowlist(t *testing.T) {
	geofileTestEnv(t, map[string]geofileEntry{
		"geoip.dat": {"https://example.invalid", "geoip.dat", "geoip.dat"},
	})

	err := (&ServerService{}).UpdateGeofile("../../etc/passwd")
	if err == nil {
		t.Fatal("expected an error for a name outside the allowlist")
	}
	if !strings.Contains(err.Error(), "not in allowlist") {
		t.Fatalf("error = %q, want it to name the allowlist", err)
	}
}
