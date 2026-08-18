package naive

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallReleaseAsset(t *testing.T) {
	archive := testZipArchive(t, []byte("new binary"))
	sum := sha256.Sum256(archive)
	validDigest := "sha256:" + hex.EncodeToString(sum[:])

	tests := []struct {
		name       string
		digest     string
		size       int64
		wantErr    string
		wantBinary string
	}{
		{name: "valid digest replaces binary", digest: validDigest, wantBinary: "new binary"},
		{name: "mismatched digest preserves binary", digest: "sha256:" + strings.Repeat("0", 64), wantErr: "checksum mismatch", wantBinary: "old binary"},
		{name: "mismatched size preserves binary", digest: validDigest, size: int64(len(archive) + 1), wantErr: "size mismatch", wantBinary: "old binary"},
		{name: "missing digest preserves binary", wantErr: "missing a SHA-256 digest", wantBinary: "old binary"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binDir := t.TempDir()
			t.Setenv("XUI_BIN_FOLDER", binDir)
			target := filepath.Join(binDir, binaryName())
			if err := os.WriteFile(target, []byte("old binary"), 0o755); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(archive)
			}))
			defer server.Close()

			err := installReleaseAsset(server.Client(), ReleaseAsset{
				Name:               "naiveproxy-test.zip",
				BrowserDownloadURL: server.URL,
				Digest:             tt.digest,
				Size:               tt.size,
			})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("installReleaseAsset() error = %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("installReleaseAsset() error = %v, want substring %q", err, tt.wantErr)
			}
			got, readErr := os.ReadFile(target)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(got) != tt.wantBinary {
				t.Fatalf("binary = %q, want %q", got, tt.wantBinary)
			}
		})
	}
}

func TestAssetSuffixFor(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		goarch  string
		want    string
		wantErr bool
	}{
		{name: "linux amd64", goos: "linux", goarch: "amd64", want: "-linux-x64.tar.xz"},
		{name: "linux 386", goos: "linux", goarch: "386", want: "-linux-x86.tar.xz"},
		{name: "linux arm64", goos: "linux", goarch: "arm64", want: "-linux-arm64.tar.xz"},
		{name: "linux arm", goos: "linux", goarch: "arm", want: "-linux-arm.tar.xz"},
		{name: "linux loong64", goos: "linux", goarch: "loong64", want: "-linux-loong64.tar.xz"},
		{name: "linux riscv64", goos: "linux", goarch: "riscv64", want: "-linux-riscv64.tar.xz"},
		{name: "windows amd64", goos: "windows", goarch: "amd64", want: "-win-x64.zip"},
		{name: "windows 386", goos: "windows", goarch: "386", want: "-win-x86.zip"},
		{name: "windows arm64", goos: "windows", goarch: "arm64", want: "-win-arm64.zip"},
		{name: "darwin amd64", goos: "darwin", goarch: "amd64", want: "-mac-x64-x64.tar.xz"},
		{name: "darwin arm64", goos: "darwin", goarch: "arm64", want: "-mac-arm64-arm64.tar.xz"},
		{name: "unsupported s390x", goos: "linux", goarch: "s390x", wantErr: true},
		{name: "unsupported windows arm", goos: "windows", goarch: "arm", wantErr: true},
		{name: "unsupported darwin 386", goos: "darwin", goarch: "386", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assetSuffixFor(tt.goos, tt.goarch)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("assetSuffixFor(%q, %q) unexpectedly succeeded with %q", tt.goos, tt.goarch, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("assetSuffixFor(%q, %q) error = %v", tt.goos, tt.goarch, err)
			}
			if got != tt.want {
				t.Fatalf("assetSuffixFor(%q, %q) = %q, want %q", tt.goos, tt.goarch, got, tt.want)
			}
		})
	}
}

func TestReplaceStagedBinaryWindows(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "naive.exe")
	staged := filepath.Join(dir, "staged.exe")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceStagedBinary(staged, target, "windows"); err != nil {
		t.Fatalf("replaceStagedBinary() error = %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("target = %q, want new", got)
	}
	if _, err := os.Stat(target + ".previous"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backup remains after success: %v", err)
	}
}

func TestReplaceStagedBinaryWindowsRestoresPrevious(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "naive.exe")
	missingStaged := filepath.Join(dir, "missing.exe")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := replaceStagedBinary(missingStaged, target, "windows")
	if err == nil {
		t.Fatal("replaceStagedBinary() unexpectedly succeeded")
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "old" {
		t.Fatalf("target = %q, want old", got)
	}
	if _, statErr := os.Stat(target + ".previous"); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("backup remains after rollback: %v", statErr)
	}
}

func testZipArchive(t *testing.T, binary []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	entry, err := writer.Create(binaryName())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
