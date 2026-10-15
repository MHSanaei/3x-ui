package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

var xrayVersionMu sync.Mutex

func supportedXrayVersion(version string) bool {
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) != 3 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	patch, err3 := strconv.Atoi(parts[2])
	return err1 == nil && err2 == nil && err3 == nil &&
		(major > 26 || (major == 26 && minor > 6) || (major == 26 && minor == 6 && patch >= 27))
}

func (s *ServerService) UpdateXray(version string) error {
	xrayVersionMu.Lock()
	defer xrayVersionMu.Unlock()

	container := os.Getenv("XUI_IN_DOCKER") == "true"
	imageVersion := container && version == "latest"
	localBinary := ""
	if imageVersion {
		localBinary = bundledXrayBinaryPath()
	} else if container {
		localBinary = localXrayBinaryPath(version)
	}
	if localBinary == "" {
		versions, err := s.GetXrayVersions()
		if err != nil {
			return err
		}
		if !slices.Contains(versions, version) {
			return fmt.Errorf("xray version %q is not in the fetched release list", version)
		}
	}

	if err := s.StopXrayService(); err != nil {
		logger.Warning("failed to stop xray before update:", err)
	}
	var err error
	if localBinary == "" {
		err = s.installXrayBinary(version)
	} else if localBinary != xray.GetBinaryPath() {
		err = restoreImageXray()
	}
	if err != nil {
		return err
	}
	if err := s.RestartXrayService(); err != nil {
		return err
	}
	if !container {
		return nil
	}
	if imageVersion {
		return database.GetDB().Where("key = ?", "xrayVersion").Delete(&model.Setting{}).Error
	}
	return s.settingService.setString("xrayVersion", version)
}

func (s *ServerService) RestoreXrayVersion() error {
	if os.Getenv("XUI_IN_DOCKER") != "true" {
		return nil
	}
	xrayVersionMu.Lock()
	defer xrayVersionMu.Unlock()

	version, err := s.settingService.getString("xrayVersion")
	if err != nil {
		return fmt.Errorf("read selected Xray version: %w", err)
	}
	if version != "" {
		if supportedXrayVersion(version) {
			if localBinary := localXrayBinaryPath(version); localBinary != "" {
				if localBinary == xray.GetBinaryPath() {
					return nil
				}
				return restoreImageXray()
			}
			if err := s.installXrayBinary(version); err == nil {
				return nil
			} else {
				logger.Warningf("Could not restore Xray %s, using bundled version: %v", version, err)
			}
		} else {
			logger.Warningf("Xray %q is unsupported, using bundled version", version)
		}
	}
	return restoreImageXray()
}

func localXrayBinaryPath(version string) string {
	if !supportedXrayVersion(version) {
		return ""
	}
	for _, path := range []string{xray.GetBinaryPath(), bundledXrayBinaryPath()} {
		if xrayBinaryVersion(path) == strings.TrimPrefix(version, "v") {
			return path
		}
	}
	return ""
}

func xrayBinaryVersion(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "-version").Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(output))
	if len(fields) < 2 || fields[0] != "Xray" {
		return ""
	}
	return fields[1]
}

func bundledXrayBinaryPath() string {
	mainFolder := os.Getenv("XUI_MAIN_FOLDER")
	if mainFolder == "" {
		mainFolder = "."
	}
	return filepath.Join(mainFolder, "xray-image", xray.GetBinaryName())
}

func restoreImageXray() error {
	file, err := os.Open(bundledXrayBinaryPath())
	if err != nil {
		return fmt.Errorf("open bundled Xray: %w", err)
	}
	defer file.Close()
	return replaceXrayBinary(file, xray.GetBinaryPath())
}

func replaceXrayBinary(source io.Reader, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".xray-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	n, err := io.Copy(file, io.LimitReader(source, maxXrayBinaryBytes+1))
	if err != nil {
		return err
	}
	if n > maxXrayBinaryBytes {
		return fmt.Errorf("xray binary exceeds %d bytes", maxXrayBinaryBytes)
	}
	if err := file.Chmod(0o755); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(target)
	}
	return os.Rename(file.Name(), target)
}
