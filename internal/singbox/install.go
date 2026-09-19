package singbox

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const releaseAPI = "https://api.github.com/repos/SagerNet/sing-box/releases/latest"

type releaseInfo struct {
	TagName string `json:"tag_name"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func InstallLatest(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseAPI, nil)
	if err != nil { return "", err }
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "3x-ui")
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "", fmt.Errorf("fetch sing-box release: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return "", fmt.Errorf("sing-box release API returned %s", resp.Status) }

	var rel releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil { return "", err }
	asset, sums, err := selectAssets(rel)
	if err != nil { return "", err }

	tmpDir, err := os.MkdirTemp("", "3x-ui-singbox-*")
	if err != nil { return "", err }
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset.Name)
	if err := download(ctx, asset.URL, archivePath); err != nil { return "", err }

	if expected := sums[asset.Name]; expected != "" {
		if err := verifySHA256(archivePath, expected); err != nil { return "", err }
	}

	extracted, err := extractBinary(archivePath, tmpDir)
	if err != nil { return "", err }

	dest := GetBinaryPath()
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil { return "", err }
	staged := dest + ".new"
	if err := copyFile(extracted, staged); err != nil { return "", err }
	if err := os.Chmod(staged, 0755); err != nil { _ = os.Remove(staged); return "", err }
	if err := os.Rename(staged, dest); err != nil { _ = os.Remove(staged); return "", err }

	return rel.TagName, nil
}

func selectAssets(rel releaseInfo) (struct{ Name, URL string }, map[string]string, error) {
	arch := runtime.GOARCH
	if arch == "arm" {
		arch = "armv7"
	}
	target := runtime.GOOS + "-" + arch
	var archive struct{ Name, URL string }
	sums := map[string]string{}
	for _, a := range rel.Assets {
		n := a.Name
		l := strings.ToLower(n)
		if strings.Contains(l, "sha256") && (strings.Contains(l, "sum") || strings.Contains(l, "checksums")) {
			// Parsed below after the release asset is downloaded.
			continue
		}
		if strings.Contains(l, "linux") || strings.Contains(l, "windows") || strings.Contains(l, "darwin") {
			if strings.Contains(l, strings.ToLower(target)) && (strings.HasSuffix(l, ".tar.gz") || strings.HasSuffix(l, ".zip")) {
				archive.Name, archive.URL = a.Name, a.URL
			}
		}
	}
	if archive.Name == "" {
		// sing-box uses GOOS/GOARCH names in official release archives.
		for _, a := range rel.Assets {
			l := strings.ToLower(a.Name)
			if strings.Contains(l, runtime.GOOS) && strings.Contains(l, runtime.GOARCH) && (strings.HasSuffix(l, ".tar.gz") || strings.HasSuffix(l, ".zip")) {
				archive.Name, archive.URL = a.Name, a.URL
				break
			}
		}
	}
	if archive.Name == "" { return archive, sums, fmt.Errorf("no sing-box release asset for %s/%s", runtime.GOOS, runtime.GOARCH) }

	// The official release API may expose a separate SHA256SUMS asset. It is
	// intentionally not required: releases without one remain installable.
	for _, a := range rel.Assets {
		if strings.Contains(strings.ToLower(a.Name), "sha256") && strings.Contains(strings.ToLower(a.Name), "sum") {
			if body, err := fetchText(ctx, a.URL); err == nil {
				for _, line := range strings.Split(body, "
") {
					fields := strings.Fields(line)
					if len(fields) >= 2 { sums[strings.TrimPrefix(fields[1], "*")] = fields[0] }
				}
			}
		}
	}
	return archive, sums, nil
}

func download(ctx context.Context, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil { return err }
	req.Header.Set("User-Agent", "3x-ui")
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return fmt.Errorf("download returned %s", resp.Status) }
	out, err := os.Create(dst)
	if err != nil { return err }
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func fetchText(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil { return "", err }
	req.Header.Set("User-Agent", "3x-ui")
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return "", fmt.Errorf("download returned %s", resp.Status) }
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

func verifySHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil { return err }
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil { return err }
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(expected)) {
		return fmt.Errorf("sing-box archive checksum mismatch: got %s, want %s", got, expected)
	}
	return nil
}

func extractBinary(archivePath, dir string) (string, error) {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		z, err := zip.OpenReader(archivePath)
		if err != nil { return "", err }
		defer z.Close()
		for _, f := range z.File {
			if filepath.Base(f.Name) == "sing-box" || filepath.Base(f.Name) == "sing-box.exe" {
				dst := filepath.Join(dir, filepath.Base(f.Name))
				if err := extractZipFile(f, dst); err != nil { return "", err }
				return dst, nil
			}
		}
		return "", fmt.Errorf("sing-box binary not found in archive")
	}
	f, err := os.Open(archivePath)
	if err != nil { return "", err }
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil { return "", err }
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF { break }
		if err != nil { return "", err }
		if filepath.Base(h.Name) != "sing-box" { continue }
		dst := filepath.Join(dir, "sing-box")
		out, err := os.Create(dst)
		if err != nil { return "", err }
		if _, err = io.Copy(out, tr); err != nil { out.Close(); return "", err }
		if err = out.Close(); err != nil { return "", err }
		return dst, nil
	}
	return "", fmt.Errorf("sing-box binary not found in archive")
}

func extractZipFile(f *zip.File, dst string) error {
	r, err := f.Open()
	if err != nil { return err }
	defer r.Close()
	out, err := os.Create(dst)
	if err != nil { return err }
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}
