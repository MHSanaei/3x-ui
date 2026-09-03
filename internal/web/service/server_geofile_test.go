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
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

// The three upstreams record the asset differently in their .sha256sum files:
// Loyalsoldier and runetfreedom write "geoip.dat", chocolate4u writes
// "release/geoip.dat" — the path from its own build.
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
		name  string
		sums  string
		asset string
	}{
		// The sidecar describes a different asset; accepting it would verify
		// geoip.dat against the digest published for geosite.dat.
		{"names-another-asset", digest + "  geosite.dat\n", "geoip.dat"},
		{"malformed-short", "deadbeef  geoip.dat\n", "geoip.dat"},
		{"not-hex", strings.Repeat("z", 64) + "  geoip.dat\n", "geoip.dat"},
		{"empty", "", "geoip.dat"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseGeofileDigest([]byte(tc.sums), tc.asset); err == nil {
				t.Fatalf("%s: expected an error", tc.name)
			}
		})
	}
}

// geofileTestEnv points the service at a temp asset folder and a throwaway DB,
// and swaps in an allowlist served by srv. It returns the asset folder.
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

// geofileServer serves each asset in files plus its .sha256sum sidecar. A
// sidecar whose digest is deliberately wrong is registered through corrupt.
func geofileServer(t *testing.T, files map[string]string, corrupt map[string]bool) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	for asset, body := range files {
		mux.HandleFunc("/"+asset, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		})

		payload := body
		if corrupt[asset] {
			payload = body + " tampered"
		}
		sum := sha256.Sum256([]byte(payload))
		line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), asset)
		mux.HandleFunc("/"+asset+".sha256sum", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(line))
		})
	}

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestUpdateGeofileInstallsVerifiedFile(t *testing.T) {
	srv := geofileServer(t, map[string]string{"geoip.dat": "good geoip payload"}, nil)
	binFolder := geofileTestEnv(t, map[string]geofileEntry{
		"geoip.dat": {srv.URL + "/geoip.dat", "geoip.dat"},
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
	srv := geofileServer(t,
		map[string]string{"geoip.dat": "good geoip payload"},
		map[string]bool{"geoip.dat": true},
	)
	binFolder := geofileTestEnv(t, map[string]geofileEntry{
		"geoip.dat": {srv.URL + "/geoip.dat", "geoip.dat"},
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

// All six or none: one bad database must not leave the core running a mix of
// releases, so a single failure installs nothing at all.
func TestUpdateGeofileInstallsNothingWhenOneFileFails(t *testing.T) {
	srv := geofileServer(t,
		map[string]string{"geoip.dat": "good geoip payload", "geosite.dat": "good geosite payload"},
		map[string]bool{"geosite.dat": true},
	)
	binFolder := geofileTestEnv(t, map[string]geofileEntry{
		"geoip.dat":   {srv.URL + "/geoip.dat", "geoip.dat"},
		"geosite.dat": {srv.URL + "/geosite.dat", "geosite.dat"},
	})

	var restarted bool
	restartStub(t, &restarted)

	if err := (&ServerService{}).UpdateGeofile(""); err == nil {
		t.Fatal("expected an error when one of the databases fails verification")
	}

	for _, name := range []string{"geoip.dat", "geosite.dat"} {
		if _, statErr := os.Stat(filepath.Join(binFolder, name)); !os.IsNotExist(statErr) {
			t.Fatalf("%s was installed even though a sibling failed verification", name)
		}
	}
	if restarted {
		t.Fatal("nothing was installed, so xray must not be restarted")
	}
}

func TestUpdateGeofileSkipsRestartWhenNotModified(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/geoip.dat", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Modified-Since") == "" {
			t.Errorf("expected a conditional GET carrying If-Modified-Since")
		}
		w.WriteHeader(http.StatusNotModified)
	})
	mux.HandleFunc("/geoip.dat.sha256sum", func(w http.ResponseWriter, r *http.Request) {
		t.Error("the sidecar must not be fetched when the asset is unchanged")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	binFolder := geofileTestEnv(t, map[string]geofileEntry{
		"geoip.dat": {srv.URL + "/geoip.dat", "geoip.dat"},
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
		"geoip.dat": {"https://example.invalid/geoip.dat", "geoip.dat"},
	})

	err := (&ServerService{}).UpdateGeofile("../../etc/passwd")
	if err == nil {
		t.Fatal("expected an error for a name outside the allowlist")
	}
	if !strings.Contains(err.Error(), "not in allowlist") {
		t.Fatalf("error = %q, want it to name the allowlist", err)
	}
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
