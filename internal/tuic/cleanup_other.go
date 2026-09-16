//go:build !linux

package tuic

import (
	"os"
	"path/filepath"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

// cleanupLegacySidecar cleans up obsolete sidecar files on non-Linux platforms.
func cleanupLegacySidecar() {
	binDir := config.GetBinFolderPath()
	if binDir == "" {
		binDir = "bin"
	}
	_ = os.RemoveAll(filepath.Join(binDir, "tuic"))
	_ = os.Remove(filepath.Join(binDir, "tuic-server"))
	_ = os.Remove(filepath.Join(binDir, "tuic-server.exe"))
}
