package naive

import (
	"archive/tar"
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/ulikunitz/xz"
)

type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type Release struct {
	TagName string         `json:"tag_name"`
	Assets  []ReleaseAsset `json:"assets"`
}

func FetchReleases() ([]Release, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/klzgrad/naiveproxy/releases", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "3x-ui-naive")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases request failed: %s", resp.Status)
	}
	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	return releases, nil
}

func assetSuffix() (string, error) {
	var osName, archName, ext string
	switch runtime.GOOS {
	case "linux":
		osName, ext = "linux", ".tar.xz"
	case "windows":
		osName, ext = "win", ".zip"
	case "darwin":
		osName, ext = "mac", ".tar.xz"
	default:
		return "", fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}
	switch runtime.GOARCH {
	case "amd64":
		archName = "x64"
	case "arm64":
		archName = "arm64"
	case "arm":
		archName = "arm"
	default:
		return "", fmt.Errorf("unsupported arch %s", runtime.GOARCH)
	}
	if runtime.GOOS == "darwin" {
		return "-" + osName + "-" + archName + "-" + archName + ext, nil
	}
	return "-" + osName + "-" + archName + ext, nil
}

func Install(version string) (string, error) {
	if err := ValidateVersion(version); err != nil {
		return "", err
	}
	releases, err := FetchReleases()
	if err != nil {
		return "", err
	}
	suffix, err := assetSuffix()
	if err != nil {
		return "", err
	}
	var downloadURL string
	var assetName string
	for _, release := range releases {
		if release.TagName != version {
			continue
		}
		for _, item := range release.Assets {
			if strings.HasSuffix(item.Name, suffix) && strings.HasPrefix(item.Name, "naiveproxy-") {
				downloadURL = item.BrowserDownloadURL
				assetName = item.Name
				break
			}
		}
	}
	if downloadURL == "" {
		return "", errors.New("release asset not found")
	}
	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}
	if err := os.MkdirAll(config.GetBinFolderPath(), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp("", "naive-download-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	target := BinaryPath()
	if strings.HasSuffix(assetName, ".zip") {
		if err := extractZipBinary(tmpPath, target); err != nil {
			return "", err
		}
	} else {
		if err := extractTarXzBinary(tmpPath, target); err != nil {
			return "", err
		}
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(target, 0o755); err != nil {
			return "", err
		}
	}
	return version, nil
}

func extractZipBinary(src, target string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if strings.EqualFold(filepath.Base(file.Name), binaryName()) {
			rc, err := file.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, rc)
			return err
		}
	}
	return errors.New("naive binary not found in archive")
}

func extractTarXzBinary(src, target string) error {
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
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if filepath.Base(header.Name) != binaryName() {
			continue
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, tarReader)
		return err
	}
	return errors.New("naive binary not found in archive")
}
