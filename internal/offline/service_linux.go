//go:build linux

package offline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"golang.org/x/sys/unix"
)

const unit = "x-ui.service"

const networkPolicy = `[Service]
IPAddressDeny=any
IPAddressAllow=localhost
IPAddressAllow=10.0.0.0/8 172.16.0.0/12 192.168.0.0/16 fc00::/7
`

var uploadMutex sync.Mutex

func StartUpload(body io.Reader) (string, error) {
	if !uploadMutex.TryLock() {
		return "", errors.New("another update package is being uploaded")
	}
	defer uploadMutex.Unlock()
	if config.GetDBKind() != "sqlite" {
		return "", errors.New("offline replacement with automatic rollback currently requires SQLite")
	}
	if os.Getenv("XUI_IN_DOCKER") == "true" {
		return "", errors.New("in-panel replacement requires a Linux systemd installation")
	}
	stage, err := os.MkdirTemp("/tmp", "x-ui-upload-")
	if err != nil {
		return "", err
	}
	launched := false
	defer func() {
		if !launched {
			_ = os.RemoveAll(stage)
		}
	}()
	archive := filepath.Join(stage, "update.zip")
	file, err := os.OpenFile(archive, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	size, copyErr := io.Copy(file, io.LimitReader(body, MaxPackageSize+1))
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return "", err
	}
	if size == 0 || size > MaxPackageSize {
		return "", errors.New("update package must be non-empty and at most 256 MiB")
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	runID := strconv.FormatInt(time.Now().UnixNano(), 10)
	args := []string{"--collect", "--unit=x-ui-offline-update", "--property=IPAddressDeny=any",
		"--property=TimeoutStartSec=180", "--working-directory=" + wd}
	for _, key := range []string{"XUI_DB_FOLDER", "XUI_DB_TYPE", "XUI_BIN_FOLDER"} {
		if value, ok := os.LookupEnv(key); ok {
			args = append(args, "--setenv="+key+"="+value)
		}
	}
	args = append(args, "--", exe, "apply-local-upload", stage, runID)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "systemd-run", args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("start offline update service: %w: %s", err, strings.TrimSpace(string(out)))
	}
	launched = true
	return runID, nil
}

func ApplyUpload(stage, runID string) error {
	if filepath.Dir(stage) != "/tmp" || !strings.HasPrefix(filepath.Base(stage), "x-ui-upload-") {
		return errors.New("invalid offline upload staging directory")
	}
	defer os.RemoveAll(stage)
	// Let the authenticated HTTP request return before stopping the service.
	time.Sleep(2 * time.Second)
	exe, err := os.Executable()
	if err == nil {
		err = Apply(filepath.Join(stage, "update.zip"), exe)
	}
	state, code := "success", 0
	if err != nil {
		state, code = "failed", 1
	}
	status := fmt.Appendf(nil, "{\"runId\":%q,\"state\":%q,\"exitCode\":%d,\"finishedAt\":%d}\n", runID, state, code, time.Now().Unix())
	return errors.Join(err, writeFile(config.GetUpdateStatusFilePath(), status, 0o600))
}

// Apply updates one stopped systemd installation. The old files and SQLite
// state remain available until the service reports that its listeners started.
func Apply(archive, panelPath string) error {
	if os.Geteuid() != 0 {
		return errors.New("offline replacement requires root")
	}
	return apply(archive, panelPath, systemctl, ensureNetworkPolicy, waitReady)
}

func apply(
	archive, panelPath string,
	serviceControl func(...string) (string, error),
	preparePolicy func() error,
	isReady func(requireCore bool) bool,
) (result error) {
	if config.GetDBKind() != "sqlite" {
		return errors.New("offline replacement with automatic rollback currently requires SQLite")
	}
	archive, err := filepath.Abs(archive)
	if err != nil {
		return err
	}
	panelPath, err = filepath.Abs(panelPath)
	if err != nil {
		return err
	}
	panelPath, err = filepath.EvalSymlinks(panelPath)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(config.GetBinFolderPath()) || !filepath.IsAbs(config.GetDBFolderPath()) {
		workingDir, err := serviceControl("show", "--property=WorkingDirectory", "--value", unit)
		if err != nil {
			return err
		}
		if workingDir == "" {
			workingDir = filepath.Dir(panelPath)
		}
		if err := os.Chdir(workingDir); err != nil {
			return err
		}
	}
	lock, err := os.OpenFile(filepath.Join(config.GetDBFolderPath(), ".offline-update.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fmt.Errorf("lock offline update: %w", err)
	}
	stage, err := os.MkdirTemp("", "x-ui-validate-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	files, err := Extract(archive, stage)
	if err != nil {
		return err
	}
	binDir := config.GetBinFolderPath()
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}

	var swaps []replacement
	defer func() {
		for _, item := range swaps {
			if item.dir != "" && !item.keep {
				_ = os.RemoveAll(item.dir)
			}
		}
	}()
	for _, name := range files {
		dest, mode := filepath.Join(binDir, name), os.FileMode(0o644)
		switch name {
		case "x-ui":
			dest, mode = panelPath, 0o755
		case "xray":
			dest, mode = xray.GetBinaryPath(), 0o755
		}
		item, err := prepareReplacement(dest, filepath.Join(stage, name), mode)
		if err != nil {
			return err
		}
		swaps = append(swaps, item)
	}
	if err := preparePolicy(); err != nil {
		return err
	}
	if _, err := serviceControl("stop", unit); err != nil {
		return err
	}

	var dbBackups []replacement
	defer func() {
		if result != nil {
			if _, err := serviceControl("stop", unit); err != nil {
				result = errors.Join(result, err)
				for i := range swaps {
					swaps[i].keep = true
				}
				return
			}
			for i := len(swaps) - 1; i >= 0; i-- {
				if err := swaps[i].restore(); err != nil {
					swaps[i].keep = true
					result = errors.Join(result, err)
				}
			}
			for i := range dbBackups {
				if err := dbBackups[i].restore(); err != nil {
					dbBackups[i].keep = true
					result = errors.Join(result, err)
				}
			}
			_, err := serviceControl("start", unit)
			result = errors.Join(result, err)
		}
		for _, item := range dbBackups {
			if item.dir != "" && !item.keep {
				_ = os.RemoveAll(item.dir)
			}
		}
	}()

	// Snapshot SQLite after stopping the panel and before allowing the new
	// binary to run migrations. A WAL/SHM newly created on startup is removed
	// on rollback when the old installation did not have one.
	for _, suffix := range []string{"", "-wal", "-shm"} {
		path := config.GetDBPath() + suffix
		item, err := backup(path)
		if err != nil {
			return err
		}
		dbBackups = append(dbBackups, item)
	}
	for i := range swaps {
		if err := swaps[i].install(); err != nil {
			return err
		}
	}
	if err := os.Remove(readyPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	if _, err := serviceControl("start", unit); err != nil {
		return err
	}
	_, coreErr := os.Stat(xray.GetBinaryPath())
	requireCore := coreErr == nil && (len(files) > 1 || files[0] != "x-ui")
	if !isReady(requireCore) {
		return errors.New("updated panel or Xray did not report readiness")
	}
	return nil
}

func ensureNetworkPolicy() error {
	dir := "/etc/systemd/system/x-ui.service.d"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "50-offline-network.conf"), []byte(networkPolicy), 0o644); err != nil {
		return err
	}
	_, err := systemctl("daemon-reload")
	return err
}

func systemctl(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "systemctl", args...).CombinedOutput()
	value := strings.TrimSpace(string(out))
	if err != nil {
		return value, fmt.Errorf("systemctl %s: %w: %s", strings.Join(args, " "), err, value)
	}
	return value, nil
}

func waitReady(requireCore bool) bool {
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		pid, _ := systemctl("show", "--property=MainPID", "--value", unit)
		ready, _ := os.ReadFile(readyPath())
		fields := strings.Fields(string(ready))
		if len(fields) == 2 && pid != "0" && fields[0] == pid {
			return !requireCore || fields[1] == "true"
		}
		time.Sleep(time.Second)
	}
	return false
}

// Install provisions a stock Linux systemd installation from local files.
func Install(archive, unitTemplate, cli string) (result error) {
	if os.Geteuid() != 0 {
		return errors.New("offline installation requires root")
	}
	const panelDir = "/usr/local/x-ui"
	const unitPath = "/etc/systemd/system/x-ui.service"
	state, err := systemctl("show", "--property=LoadState", "--value", unit)
	if state != "not-found" {
		return fmt.Errorf("fresh install requires x-ui.service to be absent (state %q): %v", state, err)
	}
	if _, err := os.Lstat(unitPath); !os.IsNotExist(err) {
		return errors.New("x-ui.service already exists; use offline-update")
	}
	if err := os.Mkdir(panelDir, 0o755); err != nil {
		return fmt.Errorf("fresh install requires an unused %s: %w", panelDir, err)
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(panelDir)
		}
	}()
	stage, err := os.MkdirTemp("", "x-ui-install-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	names, err := Extract(archive, stage)
	if err != nil {
		return err
	}
	havePanel, haveCore := false, false
	binDir := filepath.Join(panelDir, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		return err
	}
	for _, name := range names {
		dest, mode := filepath.Join(binDir, name), os.FileMode(0o644)
		switch name {
		case "x-ui":
			dest, mode, havePanel = filepath.Join(panelDir, name), 0o755, true
		case "xray":
			dest, mode, haveCore = filepath.Join(binDir, xray.GetBinaryName()), 0o755, true
		}
		if err := copyFile(filepath.Join(stage, name), dest, mode); err != nil {
			return err
		}
	}
	if !havePanel || !haveCore {
		return errors.New("fresh install requires x-ui and xray in the update ZIP")
	}
	if err := copyFile(unitTemplate, unitPath, 0o644); err != nil {
		return err
	}
	defer func() {
		if !success {
			_, stopErr := systemctl("disable", "--now", unit)
			removeErr := os.Remove(unitPath)
			_, reloadErr := systemctl("daemon-reload")
			result = errors.Join(result, stopErr, removeErr, reloadErr)
		}
	}()
	for _, path := range []string{"/etc/x-ui", "/var/log/x-ui"} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
	}
	if err := ensureNetworkPolicy(); err != nil {
		return err
	}
	if _, err := systemctl("enable", "--now", unit); err != nil {
		return err
	}
	if !waitReady(true) {
		return errors.New("installed panel did not report readiness")
	}
	success = true
	data, err := os.ReadFile(cli)
	if err != nil {
		return err
	}
	return writeFile("/usr/local/bin/x-ui", data, 0o755)
}
