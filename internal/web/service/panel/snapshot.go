package panel

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

// SnapshotInfo stores metadata about a preserved stable panel installation.
type SnapshotInfo struct {
	Version   string `json:"version"`
	CreatedAt int64  `json:"createdAt"`
	Arch      string `json:"arch"`
}

func getSnapshotFolder() string {
	return filepath.Join(config.GetDBFolderPath(), "snapshots", "stable")
}

// SaveStableSnapshot copies the current active stable binary and version metadata
// into the persistent snapshot folder before upgrading to dev.
func SaveStableSnapshot(mainFolder string, currentVersion string) error {
	srcPath := filepath.Join(mainFolder, "x-ui")
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("source binary not found: %w", err)
	}
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("source binary is not a regular file")
	}

	snapshotDir := getSnapshotFolder()
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}

	dstBinary := filepath.Join(snapshotDir, "x-ui")
	tmpBinary := filepath.Join(snapshotDir, "x-ui.tmp")
	if err := copyBinaryFile(srcPath, tmpBinary, 0o755); err != nil {
		_ = os.Remove(tmpBinary)
		return fmt.Errorf("copy snapshot binary: %w", err)
	}
	if err := os.Rename(tmpBinary, dstBinary); err != nil {
		_ = os.Remove(tmpBinary)
		return fmt.Errorf("rename snapshot binary: %w", err)
	}

	info := SnapshotInfo{
		Version:   currentVersion,
		CreatedAt: time.Now().Unix(),
		Arch:      runtime.GOARCH,
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot metadata: %w", err)
	}

	tmpMeta := filepath.Join(snapshotDir, "version.json.tmp")
	dstMeta := filepath.Join(snapshotDir, "version.json")
	if err := os.WriteFile(tmpMeta, data, 0o644); err != nil {
		_ = os.Remove(tmpMeta)
		return fmt.Errorf("write snapshot metadata: %w", err)
	}
	if err := os.Rename(tmpMeta, dstMeta); err != nil {
		_ = os.Remove(tmpMeta)
		return fmt.Errorf("rename snapshot metadata: %w", err)
	}
	return nil
}

// GetStableSnapshotInfo reads snapshot metadata and checks if the binary exists.
func GetStableSnapshotInfo() (*SnapshotInfo, bool) {
	snapshotDir := getSnapshotFolder()
	dstMeta := filepath.Join(snapshotDir, "version.json")
	data, err := os.ReadFile(dstMeta)
	if err != nil {
		return nil, false
	}

	var info SnapshotInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, false
	}

	dstBinary := filepath.Join(snapshotDir, "x-ui")
	fi, err := os.Stat(dstBinary)
	if err != nil || !fi.Mode().IsRegular() {
		return nil, false
	}
	return &info, true
}

// RestoreStableSnapshot restores the preserved stable binary back to mainFolder.
func RestoreStableSnapshot(mainFolder string) error {
	info, ok := GetStableSnapshotInfo()
	if !ok || info == nil {
		return fmt.Errorf("stable snapshot not found or invalid")
	}

	srcBinary := filepath.Join(getSnapshotFolder(), "x-ui")
	dstBinary := filepath.Join(mainFolder, "x-ui")
	tmpBinary := filepath.Join(mainFolder, "x-ui.restore.tmp")

	if err := copyBinaryFile(srcBinary, tmpBinary, 0o755); err != nil {
		_ = os.Remove(tmpBinary)
		return fmt.Errorf("copy restored binary: %w", err)
	}
	if err := os.Rename(tmpBinary, dstBinary); err != nil {
		_ = os.Remove(tmpBinary)
		return fmt.Errorf("replace active binary: %w", err)
	}
	if err := os.Chmod(dstBinary, 0o755); err != nil {
		return fmt.Errorf("chmod restored binary: %w", err)
	}
	return nil
}

func copyBinaryFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
