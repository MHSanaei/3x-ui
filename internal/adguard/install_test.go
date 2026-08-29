package adguard

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAssetNameFor(t *testing.T) {
	for _, tc := range []struct {
		goos, goarch string
		want         string
		wantErr      bool
	}{
		{goos: "linux", goarch: "amd64", want: "AdGuardHome_linux_amd64.tar.gz"},
		{goos: "linux", goarch: "arm64", want: "AdGuardHome_linux_arm64.tar.gz"},
		// Go cannot tell armv5/v6/v7 apart at runtime; armv7 is the assumption.
		{goos: "linux", goarch: "arm", want: "AdGuardHome_linux_armv7.tar.gz"},
		// The mips releases carry a softfloat suffix the plain GOARCH lacks.
		{goos: "linux", goarch: "mipsle", want: "AdGuardHome_linux_mipsle_softfloat.tar.gz"},
		{goos: "linux", goarch: "mips64le", want: "AdGuardHome_linux_mips64le_softfloat.tar.gz"},
		// Not published as a tarball, so refused rather than mis-fetched.
		{goos: "windows", goarch: "amd64", wantErr: true},
		{goos: "darwin", goarch: "arm64", wantErr: true},
		{goos: "linux", goarch: "s390x", wantErr: true},
	} {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			got, err := assetNameFor(tc.goos, tc.goarch)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("assetNameFor returned %q, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("assetNameFor: %v", err)
			}
			if got != tc.want {
				t.Errorf("assetNameFor = %q, want %q", got, tc.want)
			}
		})
	}
}

// checksumServer stands in for the release's checksums.txt.
func checksumServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checksums.txt" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchChecksum(t *testing.T) {
	want := "3b1f2c4d5e6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c"
	// The real published file writes every name with a "./" prefix. Getting
	// this wrong is not a theoretical risk: an earlier version compared the
	// field verbatim and failed on the live release with "no checksum
	// published", after passing a test that used the bare form.
	body := "" +
		"0000000000000000000000000000000000000000000000000000000000000000  ./AdGuardHome_linux_arm64.tar.gz\n" +
		want + "  ./AdGuardHome_linux_amd64.tar.gz\n"

	t.Run("picks the line for the requested asset", func(t *testing.T) {
		srv := checksumServer(t, body)
		got, err := fetchChecksumFrom(context.Background(), srv.Client(), srv.URL, "AdGuardHome_linux_amd64.tar.gz")
		if err != nil {
			t.Fatalf("fetchChecksumFrom: %v", err)
		}
		if hex.EncodeToString(got) != want {
			t.Errorf("checksum = %s, want %s", hex.EncodeToString(got), want)
		}
	})

	// Neither form is what AdGuard Home currently publishes, but both are
	// ordinary sha256sum output and cost nothing to accept.
	for _, name := range []string{"AdGuardHome_linux_amd64.tar.gz", "*AdGuardHome_linux_amd64.tar.gz"} {
		t.Run("accepts the name written as "+name, func(t *testing.T) {
			srv := checksumServer(t, want+"  "+name+"\n")
			got, err := fetchChecksumFrom(context.Background(), srv.Client(), srv.URL, "AdGuardHome_linux_amd64.tar.gz")
			if err != nil {
				t.Fatalf("fetchChecksumFrom: %v", err)
			}
			if hex.EncodeToString(got) != want {
				t.Errorf("checksum = %s, want %s", hex.EncodeToString(got), want)
			}
		})
	}

	t.Run("refuses an asset with no published checksum", func(t *testing.T) {
		srv := checksumServer(t, body)
		if _, err := fetchChecksumFrom(context.Background(), srv.Client(), srv.URL, "AdGuardHome_linux_mips_softfloat.tar.gz"); err == nil {
			t.Error("fetchChecksumFrom accepted an asset that is not listed")
		}
	})

	t.Run("refuses a malformed digest", func(t *testing.T) {
		srv := checksumServer(t, "nothexatall  AdGuardHome_linux_amd64.tar.gz\n")
		if _, err := fetchChecksumFrom(context.Background(), srv.Client(), srv.URL, "AdGuardHome_linux_amd64.tar.gz"); err == nil {
			t.Error("fetchChecksumFrom accepted a digest that is not a sha256")
		}
	})
}

func TestIsInstalled(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	if IsInstalled() {
		t.Fatal("IsInstalled is true with nothing installed")
	}
	if err := os.MkdirAll(Dir(), 0o750); err != nil {
		t.Fatalf("preparing the directory: %v", err)
	}
	if err := os.WriteFile(BinPath(), []byte("#!/bin/sh\n"), 0o750); err != nil {
		t.Fatalf("writing a stand-in binary: %v", err)
	}
	if !IsInstalled() {
		t.Error("IsInstalled is false with a binary in place")
	}
}
