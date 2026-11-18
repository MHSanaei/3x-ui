package offline

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type zipEntry struct {
	name, contents string
	mode           os.FileMode
}

func makeZIP(t *testing.T, entries ...zipEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "update.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetMode(entry.mode)
		w, err := z.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(entry.contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractUpdatePackage(t *testing.T) {
	format := zipEntry{"format", Format + "\n", 0o644}
	payload := zipEntry{"geoip.dat", "locally supplied data", 0o644}
	t.Run("local data", func(t *testing.T) {
		stage := t.TempDir()
		names, err := Extract(makeZIP(t, format, payload), stage)
		if err != nil || len(names) != 1 || names[0] != payload.name {
			t.Fatalf("Extract = %v, %v", names, err)
		}
		data, err := os.ReadFile(filepath.Join(stage, payload.name))
		if err != nil || string(data) != payload.contents {
			t.Fatalf("staged data = %q, %v", data, err)
		}
	})
	for _, tt := range []struct {
		name  string
		files []zipEntry
	}{
		{"traversal", []zipEntry{format, {"../geoip.dat", "data", 0o644}}},
		{"duplicate", []zipEntry{format, payload, payload}},
		{"symlink", []zipEntry{format, {"geoip.dat", "/tmp/file", os.ModeSymlink | 0o777}}},
		{"unknown format", []zipEntry{{"format", "other\n", 0o644}, payload}},
		{"configuration injection", []zipEntry{format, {"x-ui.db", "data", 0o644}}},
		{"invalid binary", []zipEntry{format, {"x-ui", "#!/bin/sh", 0o755}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Extract(makeZIP(t, tt.files...), t.TempDir()); err == nil {
				t.Fatal("accepted invalid archive")
			}
		})
	}
}

func TestReplacementRestoresExistingFile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux replacement semantics")
	}
	dir := t.TempDir()
	old, next := filepath.Join(dir, "installed"), filepath.Join(dir, "next")
	for path, text := range map[string]string{old: "original", next: "replacement"} {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	item, err := prepareReplacement(old, next, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if err := item.install(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(old); err != nil || string(got) != "replacement" {
		t.Fatalf("installed = %q, %v", got, err)
	}
	if err := item.restore(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(old); err != nil || string(got) != "original" {
		t.Fatalf("restored = %q, %v", got, err)
	}
}
