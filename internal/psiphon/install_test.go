package psiphon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func sha256Sum(t *testing.T, s string) []byte {
	t.Helper()
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

// A 63-character constant (one hex digit short) made Install fail
// unconditionally -- confirmed live against the panel before this fix.
func TestReleaseSHA256Length(t *testing.T) {
	got, err := hex.DecodeString(releaseSHA256)
	if err != nil {
		t.Fatalf("releaseSHA256 does not decode as hex: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("releaseSHA256 decodes to %d bytes, want 32 (sha256.Size)", len(got))
	}
}

func TestAssetPath(t *testing.T) {
	for _, tc := range []struct {
		goos, goarch string
		want         string
		wantErr      bool
	}{
		{goos: "linux", goarch: "amd64", want: "linux/psiphon-tunnel-core-x86_64"},
		{goos: "linux", goarch: "arm64", wantErr: true},
		{goos: "windows", goarch: "amd64", wantErr: true},
		{goos: "darwin", goarch: "arm64", wantErr: true},
	} {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			got, err := assetPath(tc.goos, tc.goarch)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("assetPath returned %q, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("assetPath: %v", err)
			}
			if got != tc.want {
				t.Errorf("assetPath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDownloadBinaryVerifiesDigest(t *testing.T) {
	const body = "pretend-this-is-the-psiphon-binary"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	realURL := downloadURL
	downloadURL = func(string) string { return srv.URL }
	t.Cleanup(func() { downloadURL = realURL })

	sum := sha256Sum(t, body)
	dst := filepath.Join(t.TempDir(), "out")

	if err := downloadBinary(context.Background(), srv.Client(), "asset", sum, dst); err != nil {
		t.Fatalf("downloadBinary with the correct digest: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(got) != body {
		t.Errorf("downloaded content = %q, want %q", got, body)
	}
}

func TestDownloadBinaryRejectsWrongDigest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("actual content"))
	}))
	t.Cleanup(srv.Close)

	realURL := downloadURL
	downloadURL = func(string) string { return srv.URL }
	t.Cleanup(func() { downloadURL = realURL })

	wrong := sha256Sum(t, "not the actual content")
	dst := filepath.Join(t.TempDir(), "out")

	err := downloadBinary(context.Background(), srv.Client(), "asset", wrong, dst)
	if err == nil {
		t.Fatal("downloadBinary with a wrong digest returned nil error, want a checksum failure")
	}
}
