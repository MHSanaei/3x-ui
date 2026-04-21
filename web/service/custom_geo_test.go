package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database/model"
)

// disableSSRFCheck disables the SSRF guard for the duration of a test,
// allowing httptest servers on localhost. It restores the original on cleanup.
func disableSSRFCheck(t *testing.T) {
	t.Helper()
	orig := checkSSRF
	checkSSRF = func(_ context.Context, _ string) error { return nil }
	t.Cleanup(func() { checkSSRF = orig })
}

func TestNormalizeAliasKey(t *testing.T) {
	if got := NormalizeAliasKey("GeoIP-IR"); got != "geoip_ir" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeAliasKey("a-b_c"); got != "a_b_c" {
		t.Fatalf("got %q", got)
	}
}

func TestNewCustomGeoService(t *testing.T) {
	s := NewCustomGeoService()
	if err := s.validateAlias("ok_alias-1"); err != nil {
		t.Fatal(err)
	}
}

func TestTriggerUpdateAllAllSuccess(t *testing.T) {
	s := CustomGeoService{}
	s.updateAllGetAll = func() ([]model.CustomGeoResource, error) {
		return []model.CustomGeoResource{
			{Id: 1, Alias: "a"},
			{Id: 2, Alias: "b"},
		}, nil
	}
	s.updateAllApply = func(id int, onStartup bool) (string, error) {
		return fmt.Sprintf("geo_%d.dat", id), nil
	}
	restartCalls := 0
	s.updateAllRestart = func() error {
		restartCalls++
		return nil
	}

	res, err := s.TriggerUpdateAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Succeeded) != 2 || len(res.Failed) != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if restartCalls != 1 {
		t.Fatalf("expected 1 restart, got %d", restartCalls)
	}
}

func TestTriggerUpdateAllPartialSuccess(t *testing.T) {
	s := CustomGeoService{}
	s.updateAllGetAll = func() ([]model.CustomGeoResource, error) {
		return []model.CustomGeoResource{
			{Id: 1, Alias: "ok"},
			{Id: 2, Alias: "bad"},
		}, nil
	}
	s.updateAllApply = func(id int, onStartup bool) (string, error) {
		if id == 2 {
			return "geo_2.dat", ErrCustomGeoDownload
		}
		return "geo_1.dat", nil
	}
	restartCalls := 0
	s.updateAllRestart = func() error {
		restartCalls++
		return nil
	}

	res, err := s.TriggerUpdateAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Succeeded) != 1 || len(res.Failed) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if restartCalls != 1 {
		t.Fatalf("expected 1 restart, got %d", restartCalls)
	}
}

func TestTriggerUpdateAllAllFailure(t *testing.T) {
	s := CustomGeoService{}
	s.updateAllGetAll = func() ([]model.CustomGeoResource, error) {
		return []model.CustomGeoResource{
			{Id: 1, Alias: "a"},
			{Id: 2, Alias: "b"},
		}, nil
	}
	s.updateAllApply = func(id int, onStartup bool) (string, error) {
		return fmt.Sprintf("geo_%d.dat", id), ErrCustomGeoDownload
	}
	restartCalls := 0
	s.updateAllRestart = func() error {
		restartCalls++
		return nil
	}

	res, err := s.TriggerUpdateAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Succeeded) != 0 || len(res.Failed) != 2 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if restartCalls != 0 {
		t.Fatalf("expected 0 restart, got %d", restartCalls)
	}
}

func TestCustomGeoValidateAlias(t *testing.T) {
	s := CustomGeoService{}
	if err := s.validateAlias(""); !errors.Is(err, ErrCustomGeoAliasRequired) {
		t.Fatal("empty alias")
	}
	if err := s.validateAlias("Bad"); !errors.Is(err, ErrCustomGeoAliasPattern) {
		t.Fatal("uppercase")
	}
	if err := s.validateAlias("a b"); !errors.Is(err, ErrCustomGeoAliasPattern) {
		t.Fatal("space")
	}
	if err := s.validateAlias("ok_alias-1"); err != nil {
		t.Fatal(err)
	}
	if err := s.validateAlias("geoip"); !errors.Is(err, ErrCustomGeoAliasReserved) {
		t.Fatal("reserved")
	}
}

func TestCustomGeoValidateURL(t *testing.T) {
	s := CustomGeoService{}
	if _, err := s.sanitizeURL(""); !errors.Is(err, ErrCustomGeoURLRequired) {
		t.Fatal("empty")
	}
	if _, err := s.sanitizeURL("ftp://x"); !errors.Is(err, ErrCustomGeoURLScheme) {
		t.Fatal("ftp")
	}
	if sanitized, err := s.sanitizeURL("https://example.com/a.dat"); err != nil {
		t.Fatal(err)
	} else if sanitized != "https://example.com/a.dat" {
		t.Fatalf("unexpected sanitized URL: %s", sanitized)
	}
}

func TestCustomGeoValidateType(t *testing.T) {
	s := CustomGeoService{}
	if err := s.validateType("geosite"); err != nil {
		t.Fatal(err)
	}
	if err := s.validateType("x"); !errors.Is(err, ErrCustomGeoInvalidType) {
		t.Fatal("bad type")
	}
}

func TestCustomGeoDownloadToPath(t *testing.T) {
	disableSSRFCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "1")
		if r.Header.Get("If-Modified-Since") != "" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, minDatBytes+1))
	}))
	defer ts.Close()
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	dest := filepath.Join(dir, "geoip_t.dat")
	s := CustomGeoService{}
	skipped, _, err := s.downloadToPath(ts.URL, dest, "")
	if err != nil {
		t.Fatal(err)
	}
	if skipped {
		t.Fatal("expected download")
	}
	st, err := os.Stat(dest)
	if err != nil || st.Size() < minDatBytes {
		t.Fatalf("file %v", err)
	}
	skipped2, _, err2 := s.downloadToPath(ts.URL, dest, "")
	if err2 != nil || !skipped2 {
		t.Fatalf("304 expected skipped=%v err=%v", skipped2, err2)
	}
}

func TestCustomGeoDownloadToPath_missingLocalSendsNoIMSFromDB(t *testing.T) {
	disableSSRFCheck(t)
	lm := "Wed, 21 Oct 2015 07:28:00 GMT"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Modified-Since") != "" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Last-Modified", lm)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, minDatBytes+1))
	}))
	defer ts.Close()
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	dest := filepath.Join(dir, "geoip_rebuild.dat")
	s := CustomGeoService{}
	skipped, _, err := s.downloadToPath(ts.URL, dest, lm)
	if err != nil {
		t.Fatal(err)
	}
	if skipped {
		t.Fatal("must not treat as not-modified when local file is missing")
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatal("file should exist after container-style rebuild")
	}
}

func TestCustomGeoDownloadToPath_repairSkipsConditional(t *testing.T) {
	disableSSRFCheck(t)
	lm := "Wed, 21 Oct 2015 07:28:00 GMT"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Modified-Since") != "" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Last-Modified", lm)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, minDatBytes+1))
	}))
	defer ts.Close()
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	dest := filepath.Join(dir, "geoip_bad.dat")
	if err := os.WriteFile(dest, make([]byte, minDatBytes-1), 0o644); err != nil {
		t.Fatal(err)
	}
	s := CustomGeoService{}
	skipped, _, err := s.downloadToPath(ts.URL, dest, lm)
	if err != nil {
		t.Fatal(err)
	}
	if skipped {
		t.Fatal("corrupt local file must be re-downloaded, not 304")
	}
	st, err := os.Stat(dest)
	if err != nil || st.Size() < minDatBytes {
		t.Fatalf("file repaired: %v", err)
	}
}

func TestCustomGeoFileNameFor(t *testing.T) {
	s := CustomGeoService{}
	if s.fileNameFor("geoip", "a") != "geoip_a.dat" {
		t.Fatal("geoip name")
	}
	if s.fileNameFor("geosite", "b") != "geosite_b.dat" {
		t.Fatal("geosite name")
	}
}

func TestLocalDatFileNeedsRepair(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	if !localDatFileNeedsRepair(filepath.Join(dir, "missing.dat")) {
		t.Fatal("missing")
	}
	smallPath := filepath.Join(dir, "small.dat")
	if err := os.WriteFile(smallPath, make([]byte, minDatBytes-1), 0o644); err != nil {
		t.Fatal(err)
	}
	if !localDatFileNeedsRepair(smallPath) {
		t.Fatal("small")
	}
	okPath := filepath.Join(dir, "ok.dat")
	if err := os.WriteFile(okPath, make([]byte, minDatBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	if localDatFileNeedsRepair(okPath) {
		t.Fatal("ok size")
	}
	dirPath := filepath.Join(dir, "isdir.dat")
	if err := os.Mkdir(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if !localDatFileNeedsRepair(dirPath) {
		t.Fatal("dir should need repair")
	}
	if !CustomGeoLocalFileNeedsRepair(dirPath) {
		t.Fatal("exported wrapper dir")
	}
	if CustomGeoLocalFileNeedsRepair(okPath) {
		t.Fatal("exported wrapper ok file")
	}
}

func TestProbeCustomGeoURL_HEADOK(t *testing.T) {
	disableSSRFCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	if err := probeCustomGeoURL(ts.URL); err != nil {
		t.Fatal(err)
	}
}

func TestProbeCustomGeoURL_HEAD405GETRange(t *testing.T) {
	disableSSRFCheck(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Method == http.MethodGet && r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte{0})
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()
	if err := probeCustomGeoURL(ts.URL); err != nil {
		t.Fatal(err)
	}
}
