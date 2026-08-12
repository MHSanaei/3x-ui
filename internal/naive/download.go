package naive

import (
	"archive/tar"
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

const (
	naiveReleasesURL     = "https://api.github.com/repos/klzgrad/naiveproxy/releases?per_page=100"
	maxNaiveReleaseBytes = 4 << 20
	maxNaiveArchiveBytes = 200 << 20
	maxNaiveBinaryBytes  = 200 << 20
)

type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
	Size               int64  `json:"size"`
}

type Release struct {
	TagName     string         `json:"tag_name"`
	PublishedAt string         `json:"published_at"`
	Prerelease  bool           `json:"prerelease"`
	Installable bool           `json:"installable"`
	Assets      []ReleaseAsset `json:"assets"`
}

func FetchReleases(client *http.Client) ([]Release, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, naiveReleasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "3x-ui-naive")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases request failed: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxNaiveReleaseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxNaiveReleaseBytes {
		return nil, fmt.Errorf("github releases response exceeds %d bytes", maxNaiveReleaseBytes)
	}
	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, err
	}
	suffix, err := assetSuffix()
	if err != nil {
		return nil, err
	}
	for i := range releases {
		releases[i].Installable = compatibleReleaseAsset(&releases[i], suffix) != nil
	}
	return releases, nil
}

func assetSuffix() (string, error) {
	return assetSuffixFor(runtime.GOOS, runtime.GOARCH)
}

func assetSuffixFor(goos, goarch string) (string, error) {
	var osName, archName, ext string
	switch goos {
	case "linux":
		osName, ext = "linux", ".tar.xz"
	case "windows":
		osName, ext = "win", ".zip"
	case "darwin":
		osName, ext = "mac", ".tar.xz"
	default:
		return "", fmt.Errorf("unsupported OS %s", goos)
	}
	switch goarch {
	case "amd64":
		archName = "x64"
	case "386":
		archName = "x86"
	case "arm64":
		archName = "arm64"
	case "arm":
		archName = "arm"
	case "loong64":
		archName = "loong64"
	case "riscv64":
		archName = "riscv64"
	default:
		return "", fmt.Errorf("unsupported arch %s", goarch)
	}
	if goos != "linux" && (goarch == "arm" || goarch == "loong64" || goarch == "riscv64") {
		return "", fmt.Errorf("unsupported arch %s for %s", goarch, goos)
	}
	if goos == "darwin" {
		if goarch != "amd64" && goarch != "arm64" {
			return "", fmt.Errorf("unsupported arch %s for %s", goarch, goos)
		}
		return "-" + osName + "-" + archName + "-" + archName + ext, nil
	}
	return "-" + osName + "-" + archName + ext, nil
}

func compatibleReleaseAsset(release *Release, suffix string) *ReleaseAsset {
	for i := range release.Assets {
		asset := &release.Assets[i]
		if strings.HasPrefix(asset.Name, "naiveproxy-") && strings.HasSuffix(asset.Name, suffix) {
			return asset
		}
	}
	return nil
}

func Install(client *http.Client, version string) (string, error) {
	if err := ValidateVersion(version); err != nil {
		return "", err
	}
	releases, err := FetchReleases(client)
	if err != nil {
		return "", err
	}
	suffix, err := assetSuffix()
	if err != nil {
		return "", err
	}
	var selected *ReleaseAsset
	for i := range releases {
		release := &releases[i]
		if release.TagName != version {
			continue
		}
		selected = compatibleReleaseAsset(release, suffix)
		break
	}
	if selected == nil {
		return "", errors.New("release asset not found")
	}
	if err := installReleaseAsset(client, *selected); err != nil {
		return "", err
	}
	_ = storeInstalledReleaseTag(version)
	return version, nil
}

func installReleaseAsset(client *http.Client, asset ReleaseAsset) error {
	wantDigest, err := parseReleaseDigest(asset.Digest)
	if err != nil {
		return err
	}
	if asset.Size > maxNaiveArchiveBytes {
		return fmt.Errorf("naive archive exceeds %d bytes", maxNaiveArchiveBytes)
	}
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "3x-ui-naive")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	if resp.ContentLength > maxNaiveArchiveBytes {
		return fmt.Errorf("naive archive exceeds %d bytes", maxNaiveArchiveBytes)
	}
	archive, err := os.CreateTemp("", "naive-download-*")
	if err != nil {
		return err
	}
	archivePath := archive.Name()
	defer os.Remove(archivePath)
	hasher := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(archive, hasher), io.LimitReader(resp.Body, maxNaiveArchiveBytes+1))
	closeErr := archive.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n > maxNaiveArchiveBytes {
		return fmt.Errorf("naive archive exceeds %d bytes", maxNaiveArchiveBytes)
	}
	if asset.Size > 0 && n != asset.Size {
		return fmt.Errorf("naive archive size mismatch: expected %d, got %d", asset.Size, n)
	}
	gotDigest := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(gotDigest, wantDigest) {
		return fmt.Errorf("naive archive checksum mismatch: expected %s, got %s", wantDigest, gotDigest)
	}
	return installArchiveBinary(archivePath, asset.Name, BinaryPath())
}

func parseReleaseDigest(raw string) (string, error) {
	algorithm, value, ok := strings.Cut(strings.TrimSpace(raw), ":")
	if !ok || !strings.EqualFold(algorithm, "sha256") {
		return "", errors.New("release asset is missing a SHA-256 digest")
	}
	value = strings.ToLower(strings.TrimSpace(value))
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return "", errors.New("release asset has an invalid SHA-256 digest")
	}
	return value, nil
}

func installArchiveBinary(src, assetName, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	staged, err := os.CreateTemp(filepath.Dir(target), ".naive-*")
	if err != nil {
		return err
	}
	stagedPath := staged.Name()
	installed := false
	defer func() {
		_ = staged.Close()
		if !installed {
			_ = os.Remove(stagedPath)
		}
	}()
	if strings.HasSuffix(strings.ToLower(assetName), ".zip") {
		err = extractZipBinary(src, staged)
	} else {
		err = extractTarXzBinary(src, staged)
	}
	if err != nil {
		return err
	}
	if err := staged.Chmod(0o755); err != nil {
		return err
	}
	if err := staged.Sync(); err != nil {
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	if err := replaceStagedBinary(stagedPath, target, runtime.GOOS); err != nil {
		return err
	}
	installed = true
	return nil
}

func replaceStagedBinary(stagedPath, target, goos string) error {
	if goos != "windows" {
		return os.Rename(stagedPath, target)
	}

	backup := target + ".previous"
	_ = os.Remove(backup)
	hadTarget := false
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
		hadTarget = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.Rename(stagedPath, target); err != nil {
		if hadTarget {
			if restoreErr := os.Rename(backup, target); restoreErr != nil {
				return fmt.Errorf("replace naive binary: %w; restore previous binary: %w", err, restoreErr)
			}
		}
		return err
	}
	if hadTarget {
		_ = os.Remove(backup)
	}
	return nil
}

func extractZipBinary(src string, dst io.Writer) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if !strings.EqualFold(filepath.Base(file.Name), binaryName()) {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		err = copyBinary(dst, rc)
		closeErr := rc.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
	return errors.New("naive binary not found in archive")
}

func extractTarXzBinary(src string, dst io.Writer) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()
	xzReader, err := xz.NewReader(file)
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(xzReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(header.Name) != binaryName() {
			continue
		}
		return copyBinary(dst, tarReader)
	}
	return errors.New("naive binary not found in archive")
}

func copyBinary(dst io.Writer, src io.Reader) error {
	n, err := io.Copy(dst, io.LimitReader(src, maxNaiveBinaryBytes+1))
	if err != nil {
		return err
	}
	if n > maxNaiveBinaryBytes {
		return fmt.Errorf("naive binary exceeds %d bytes", maxNaiveBinaryBytes)
	}
	return nil
}
