package frontproxy

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zipOf builds an in-memory archive from name -> content pairs, in the order
// given so tests can control which entry the reader sees first.
func zipOf(t *testing.T, entries ...[2]string) (*bytes.Reader, int64) {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for _, e := range entries {
		w, err := zw.Create(e[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buf.Bytes()), int64(buf.Len())
}

func TestInstallDecoyArchiveFlat(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "decoy")
	r, size := zipOf(t,
		[2]string{"index.html", "<h1>hello</h1>"},
		[2]string{"assets/app.css", "body{}"},
	)
	if err := InstallDecoyArchive(dir, r, size); err != nil {
		t.Fatalf("InstallDecoyArchive: %v", err)
	}
	if !DecoyInstalled(dir) {
		t.Fatal("DecoyInstalled is false after a successful install")
	}
	got, err := os.ReadFile(filepath.Join(dir, "assets", "app.css"))
	if err != nil || string(got) != "body{}" {
		t.Errorf("nested file = %q, %v", got, err)
	}
}

// Zipping a folder is what an admin actually does, so the wrapping directory
// must be stripped rather than rejected for having no index.html at the root.
func TestInstallDecoyArchiveStripsSingleTopFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "decoy")
	r, size := zipOf(t,
		[2]string{"mysite/", ""},
		[2]string{"mysite/index.html", "<h1>wrapped</h1>"},
		[2]string{"mysite/css/a.css", "a{}"},
	)
	if err := InstallDecoyArchive(dir, r, size); err != nil {
		t.Fatalf("InstallDecoyArchive: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil || !strings.Contains(string(got), "wrapped") {
		t.Errorf("index.html = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "mysite")); err == nil {
		t.Error("wrapper directory was kept instead of stripped")
	}
}

func TestInstallDecoyArchiveRejects(t *testing.T) {
	cases := []struct {
		name    string
		entries [][2]string
	}{
		{"no index", [][2]string{{"readme.txt", "x"}}},
		{"traversal", [][2]string{{"index.html", "x"}, {"../evil.html", "x"}}},
		{"absolute", [][2]string{{"/etc/passwd", "x"}, {"index.html", "x"}}},
		{"two top folders without index", [][2]string{{"a/index.html", "x"}, {"b/index.html", "x"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "decoy")
			r, size := zipOf(t, tc.entries...)
			if err := InstallDecoyArchive(dir, r, size); err == nil {
				t.Fatal("install succeeded, want refusal")
			}
			if _, err := os.Stat(dir); err == nil {
				t.Error("a refused install still created the decoy directory")
			}
		})
	}
}

func TestInstallDecoyArchiveRejectsTooManyEntries(t *testing.T) {
	entries := make([][2]string, 0, maxDecoyEntries+1)
	entries = append(entries, [2]string{"index.html", "x"})
	for i := 0; i < maxDecoyEntries; i++ {
		entries = append(entries, [2]string{fmt.Sprintf("f%d.txt", i), "x"})
	}
	dir := filepath.Join(t.TempDir(), "decoy")
	r, size := zipOf(t, entries...)
	if err := InstallDecoyArchive(dir, r, size); err == nil {
		t.Fatal("install succeeded with an over-long archive, want refusal")
	}
}

// A rejected upload must not take the working site down with it.
func TestInstallDecoyArchiveKeepsPreviousOnFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "decoy")
	first, size := zipOf(t, [2]string{"index.html", "<h1>original</h1>"})
	if err := InstallDecoyArchive(dir, first, size); err != nil {
		t.Fatalf("first install: %v", err)
	}
	bad, badSize := zipOf(t, [2]string{"readme.txt", "no index here"})
	if err := InstallDecoyArchive(dir, bad, badSize); err == nil {
		t.Fatal("second install succeeded, want refusal")
	}
	got, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil || !strings.Contains(string(got), "original") {
		t.Errorf("previous site lost: %q, %v", got, err)
	}
}

// Replacing a site must not leave files from the old one behind, or a removed
// page would keep answering.
func TestInstallDecoyArchiveReplacesOldFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "decoy")
	first, size := zipOf(t, [2]string{"index.html", "one"}, [2]string{"gone.html", "stale"})
	if err := InstallDecoyArchive(dir, first, size); err != nil {
		t.Fatalf("first install: %v", err)
	}
	second, size2 := zipOf(t, [2]string{"index.html", "two"})
	if err := InstallDecoyArchive(dir, second, size2); err != nil {
		t.Fatalf("second install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "gone.html")); err == nil {
		t.Error("file from the replaced site is still present")
	}
}

// The uploaded site has to be reachable through the decoy handler itself,
// not just present on disk.
func TestUploadedArchiveIsServed(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "decoy")
	r, size := zipOf(t, [2]string{"index.html", "<h1>uploaded site</h1>"})
	if err := InstallDecoyArchive(dir, r, size); err != nil {
		t.Fatalf("InstallDecoyArchive: %v", err)
	}
	h := newDecoyHandler(DecoyConfig{Mode: DecoyUpload, Dir: dir})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(rec.Body.String(), "uploaded site") {
		t.Errorf("body = %q, want the uploaded page", rec.Body.String())
	}
}

func TestRemoveDecoy(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "decoy")
	r, size := zipOf(t, [2]string{"index.html", "x"})
	if err := InstallDecoyArchive(dir, r, size); err != nil {
		t.Fatalf("InstallDecoyArchive: %v", err)
	}
	if err := RemoveDecoy(dir); err != nil {
		t.Fatalf("RemoveDecoy: %v", err)
	}
	if DecoyInstalled(dir) {
		t.Error("DecoyInstalled is still true after RemoveDecoy")
	}
	if err := RemoveDecoy(dir); err != nil {
		t.Errorf("RemoveDecoy on a missing directory returned %v, want nil", err)
	}
}
