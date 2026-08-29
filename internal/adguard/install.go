// Package adguard manages a real AdGuard Home instance as the panel's cover
// story: the reverse proxy's decoy mode serves it at the site root, so a
// visitor who probes the domain finds a genuine, working ad-blocking DNS
// service instead of a static page pretending to be one.
//
// Unlike internal/tor, which expects a system-installed binary, AdGuard Home
// is not packaged by any distro -- this package downloads the official release
// itself. It never touches the database: the caller resolves settings and
// hands in an HTTP client, matching internal/frontproxy's split.
package adguard

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

// releaseBase is the "latest" download endpoint, which redirects to whatever
// the newest tagged release is. Pinning a version here would mean shipping a
// panel update just to pick up an AdGuard Home release.
const releaseBase = "https://github.com/AdguardTeam/AdGuardHome/releases/latest/download"

// maxDownloadBytes bounds both the tarball and what it expands to. The real
// binary is well under 50 MiB; this only exists so a redirect to something
// unexpected cannot fill the disk.
const maxDownloadBytes = 128 << 20

// binName is the file inside the release tarball, and what it is called on
// disk once extracted.
const binName = "AdGuardHome"

// Dir is where the binary, its config and its data live, following the same
// "sidecar owns a subdirectory of bin/" convention the Tor sidecar uses.
func Dir() string { return config.GetBinFolderPath() + "/adguardhome" }

// BinPath is the AdGuard Home executable this package manages.
func BinPath() string { return filepath.Join(Dir(), binName) }

// ConfigPath is the seeded AdGuardHome.yaml. AdGuard Home rewrites this file
// itself whenever the admin changes something in its own UI.
func ConfigPath() string { return filepath.Join(Dir(), "AdGuardHome.yaml") }

// IsInstalled reports whether a usable binary is present, which is what the
// settings UI shows next to the install button.
func IsInstalled() bool {
	info, err := os.Stat(BinPath())
	return err == nil && info.Mode().IsRegular()
}

func assetName() (string, error) { return assetNameFor(runtime.GOOS, runtime.GOARCH) }

// assetNameFor is assetName with the platform passed in, so the mapping can be
// tested for architectures this build is not running on.
//
// GOARCH "arm" covers armv5/v6/v7, which the release splits apart and Go gives
// no runtime way to tell between. armv7 is the only one of the three still
// found on hardware that runs a panel, so it is the assumption; an armv6 board
// gets a clear exec-format failure rather than a silent wrong install.
func assetNameFor(goos, goarch string) (string, error) {
	// Only the Linux builds are tarballs; the others ship as zip, and a panel
	// serving a public decoy is a Linux deployment anyway.
	if goos != "linux" {
		return "", fmt.Errorf("AdGuard Home can only be installed by the panel on Linux, not %s", goos)
	}
	arch := map[string]string{
		"amd64":    "amd64",
		"arm64":    "arm64",
		"arm":      "armv7",
		"386":      "386",
		"mips":     "mips_softfloat",
		"mipsle":   "mipsle_softfloat",
		"mips64":   "mips64_softfloat",
		"mips64le": "mips64le_softfloat",
		"riscv64":  "riscv64",
		"ppc64le":  "ppc64le",
	}[goarch]
	if arch == "" {
		return "", fmt.Errorf("no AdGuard Home release for %s/%s", goos, goarch)
	}
	return fmt.Sprintf("%s_%s_%s.tar.gz", binName, goos, arch), nil
}

// Install downloads the current AdGuard Home release and puts the binary in
// place. Already-installed is a no-op rather than an error, so the caller can
// treat the button as "make sure it's there".
//
// client comes from the caller so the download honors the panel's own egress
// proxy -- on a server where GitHub is filtered, a direct fetch would be the
// one step of this feature that silently cannot work.
func Install(ctx context.Context, client *http.Client) error {
	if IsInstalled() {
		return nil
	}
	asset, err := assetName()
	if err != nil {
		return err
	}
	want, err := fetchChecksum(ctx, client, asset)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(Dir(), 0o750); err != nil {
		return fmt.Errorf("cannot create %s: %w", Dir(), err)
	}
	staging := BinPath() + ".new"
	if err := downloadBinary(ctx, client, asset, want, staging); err != nil {
		_ = os.Remove(staging)
		return err
	}
	if err := os.Rename(staging, BinPath()); err != nil {
		_ = os.Remove(staging)
		return fmt.Errorf("cannot put the AdGuard Home binary in place: %w", err)
	}
	return nil
}

// withBody issues one GET and hands the response body to fn.
//
// The body is read and closed inside this function rather than returned, so
// there is exactly one place responsible for closing it no matter which caller
// is fetching what.
func withBody(ctx context.Context, client *http.Client, url string, fn func(io.Reader) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned HTTP %d", url, resp.StatusCode)
	}
	return fn(resp.Body)
}

// fetchChecksum pulls the release's checksums.txt and returns the digest
// recorded for one asset. Verifying is worth the extra request: this ends with
// executing whatever came back, so "it was HTTPS" is a weaker guarantee than
// matching the digest the release itself publishes.
func fetchChecksum(ctx context.Context, client *http.Client, asset string) ([]byte, error) {
	return fetchChecksumFrom(ctx, client, releaseBase, asset)
}

// fetchChecksumFrom is fetchChecksum with the release location passed in, so
// the parsing can be tested without reaching GitHub.
func fetchChecksumFrom(ctx context.Context, client *http.Client, base, asset string) ([]byte, error) {
	var sum []byte
	err := withBody(ctx, client, base+"/checksums.txt", func(body io.Reader) error {
		raw, err := io.ReadAll(io.LimitReader(body, 1<<20))
		if err != nil {
			return fmt.Errorf("cannot read AdGuard Home checksums: %w", err)
		}
		sum, err = parseChecksum(string(raw), asset)
		return err
	})
	if err != nil {
		return nil, err
	}
	return sum, nil
}

// parseChecksum finds one asset's digest in sha256sum-style output.
func parseChecksum(raw, asset string) ([]byte, error) {
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		// AdGuard Home publishes the names as "./AdGuardHome_linux_amd64.tar.gz",
		// and coreutils marks binary-mode entries with a leading "*". Comparing
		// on the base name accepts both without having to predict which.
		if path.Base(strings.TrimPrefix(fields[1], "*")) != asset {
			continue
		}
		sum, err := hex.DecodeString(fields[0])
		if err != nil || len(sum) != sha256.Size {
			return nil, fmt.Errorf("malformed checksum for %s", asset)
		}
		return sum, nil
	}
	return nil, fmt.Errorf("no checksum published for %s", asset)
}

// downloadBinary streams the release tarball to dst, extracting only the
// executable and refusing to keep it if the archive's digest does not match.
//
// The whole stream is hashed, so the digest covers the compressed bytes
// exactly as published -- extraction happens as it downloads, and the result
// is discarded by the caller if verification then fails.
func downloadBinary(ctx context.Context, client *http.Client, asset string, want []byte, dst string) error {
	return withBody(ctx, client, releaseBase+"/"+asset, func(body io.Reader) error {
		digest := sha256.New()
		counted := io.TeeReader(io.LimitReader(body, maxDownloadBytes+1), digest)
		gz, err := gzip.NewReader(counted)
		if err != nil {
			return fmt.Errorf("AdGuard Home download is not a gzip archive: %w", err)
		}
		defer gz.Close()

		if err := extractBinary(tar.NewReader(gz), dst); err != nil {
			return err
		}
		// The tar reader stops at the entry it wanted, leaving the rest of the
		// stream unread -- and unhashed. Drain it so the digest covers the
		// whole published file rather than just its first entries.
		if _, err := io.Copy(io.Discard, counted); err != nil {
			return fmt.Errorf("cannot read AdGuard Home download: %w", err)
		}
		if got := digest.Sum(nil); !bytes.Equal(got, want) {
			return fmt.Errorf("AdGuard Home download failed checksum verification (got %x, want %x)", got, want)
		}
		return nil
	})
}

// extractBinary writes the single executable entry out of the release tarball.
func extractBinary(tr *tar.Reader, dst string) error {
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("AdGuard Home archive contains no %s executable", binName)
		}
		if err != nil {
			return fmt.Errorf("cannot read AdGuard Home archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg || path.Base(header.Name) != binName {
			continue
		}
		out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o750)
		if err != nil {
			return fmt.Errorf("cannot write %s: %w", dst, err)
		}
		n, err := io.Copy(out, io.LimitReader(tr, maxDownloadBytes+1))
		closeErr := out.Close()
		if err != nil {
			return fmt.Errorf("cannot write %s: %w", dst, err)
		}
		if closeErr != nil {
			return fmt.Errorf("cannot write %s: %w", dst, closeErr)
		}
		if n > maxDownloadBytes {
			return fmt.Errorf("AdGuard Home binary is larger than the %d MiB limit", maxDownloadBytes>>20)
		}
		return nil
	}
}

// Uninstall stops the daemon and removes everything this package installed,
// including the AdGuard Home config and its filter data.
func Uninstall() error {
	GetManager().StopAll()
	if err := os.RemoveAll(Dir()); err != nil {
		return fmt.Errorf("cannot remove %s: %w", Dir(), err)
	}
	return nil
}
