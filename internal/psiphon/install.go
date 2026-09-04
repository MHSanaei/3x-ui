package psiphon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

// releaseCommit/releaseSHA256 pin one commit of psiphon-tunnel-core-binaries --
// that repo has no checksums.txt, so this is this package's own reviewed digest, bumped only deliberately.
const (
	releaseCommit = "70d99fd87bb010654e5e0759f347e3ab0f4a952f"
	releaseSHA256 = "47f8956f3f3cf9813d4cbee4665adc99b1f8ffa788c13dc5e03e824cc29217b0"
)

// maxDownloadBytes bounds the download. The real binary is ~10 MiB; this only
// exists so a redirect to something unexpected cannot fill the disk.
const maxDownloadBytes = 64 << 20

// binName is the file this package writes the binary to on disk.
const binName = "psiphon-tunnel-core"

// Dir is where the binary, config, notices and data live, following the
// "sidecar owns a subdirectory of bin/" convention Tor and AdGuard Home use.
func Dir() string { return config.GetBinFolderPath() + "/psiphon" }

// BinPath is the Psiphon ConsoleClient executable this package manages.
func BinPath() string { return filepath.Join(Dir(), binName) }

// IsInstalled reports whether a usable binary is present, which together with
// IsConfigured is what the settings UI checks before offering to start.
func IsInstalled() bool {
	info, err := os.Stat(BinPath())
	return err == nil && info.Mode().IsRegular()
}

// assetPath returns the path within psiphon-tunnel-core-binaries for the
// running platform. Only linux/amd64 is published there, verified against the repo's own listing.
func assetPath(goos, goarch string) (string, error) {
	if goos == "linux" && goarch == "amd64" {
		return "linux/psiphon-tunnel-core-x86_64", nil
	}
	return "", fmt.Errorf("no pinned Psiphon binary for %s/%s (only linux/amd64 today)", goos, goarch)
}

// downloadURL is the raw file at the pinned commit, not "latest". A var, not
// a func, so tests can point it at an httptest server.
var downloadURL = func(asset string) string {
	return fmt.Sprintf(
		"https://raw.githubusercontent.com/Psiphon-Labs/psiphon-tunnel-core-binaries/%s/%s",
		releaseCommit, asset,
	)
}

// Install downloads the pinned Psiphon ConsoleClient release. Already-installed
// is a no-op. client comes from the caller so the download honors the panel's own proxy, matching adguard.Install.
func Install(ctx context.Context, client *http.Client) error {
	if IsInstalled() {
		return nil
	}
	asset, err := assetPath(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	want, err := hex.DecodeString(releaseSHA256)
	if err != nil {
		return fmt.Errorf("malformed pinned checksum: %w", err)
	}
	if len(want) != sha256.Size {
		return fmt.Errorf("malformed pinned checksum: got %d bytes, want %d", len(want), sha256.Size)
	}
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return fmt.Errorf("cannot create %s: %w", Dir(), err)
	}
	staging := BinPath() + ".new"
	if err := downloadBinary(ctx, client, asset, want, staging); err != nil {
		_ = os.Remove(staging)
		return err
	}
	if err := os.Rename(staging, BinPath()); err != nil {
		_ = os.Remove(staging)
		return fmt.Errorf("cannot put the Psiphon binary in place: %w", err)
	}
	return nil
}

// downloadBinary streams the pinned file to dst, and the caller discards it
// if the hashed-while-downloading digest doesn't match the pin.
func downloadBinary(ctx context.Context, client *http.Client, asset string, want []byte, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL(asset), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach %s: %w", downloadURL(asset), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned HTTP %d", downloadURL(asset), resp.StatusCode)
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o750)
	if err != nil {
		return fmt.Errorf("cannot write %s: %w", dst, err)
	}
	digest := sha256.New()
	body := io.LimitReader(resp.Body, maxDownloadBytes+1)
	n, err := io.Copy(out, io.TeeReader(body, digest))
	closeErr := out.Close()
	if err != nil {
		return fmt.Errorf("cannot write %s: %w", dst, err)
	}
	if closeErr != nil {
		return fmt.Errorf("cannot write %s: %w", dst, closeErr)
	}
	if n > maxDownloadBytes {
		return fmt.Errorf("Psiphon binary is larger than the %d MiB limit", maxDownloadBytes>>20)
	}
	if got := digest.Sum(nil); !bytes.Equal(got, want) {
		return fmt.Errorf("Psiphon download failed checksum verification (got %x, want %x)", got, want)
	}
	return nil
}

// Uninstall stops the daemon and removes everything this package installed,
// including the admin's psiphon.config and Psiphon's own data directory.
func Uninstall() error {
	GetManager().StopAll()
	if err := os.RemoveAll(Dir()); err != nil {
		return fmt.Errorf("cannot remove %s: %w", Dir(), err)
	}
	return nil
}
