package naive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompatibleReleaseAsset(t *testing.T) {
	release := Release{Assets: []ReleaseAsset{
		{Name: "notes.txt"},
		{Name: "naiveproxy-v148.0.7778.96-5-linux-x64.tar.xz"},
	}}
	asset := compatibleReleaseAsset(&release, "-linux-x64.tar.xz")
	if asset == nil || asset.Name != "naiveproxy-v148.0.7778.96-5-linux-x64.tar.xz" {
		t.Fatalf("compatibleReleaseAsset() = %#v", asset)
	}
	if got := compatibleReleaseAsset(&Release{}, "-linux-x64.tar.xz"); got != nil {
		t.Fatalf("assetless release unexpectedly installable: %#v", got)
	}
}

func TestInstalledReleaseTagRoundTrip(t *testing.T) {
	binDir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", binDir)
	if err := os.WriteFile(filepath.Join(binDir, binaryName()), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	const tag = "v148.0.7778.96-5"
	if err := storeInstalledReleaseTag(tag); err != nil {
		t.Fatal(err)
	}
	if got := InstalledReleaseTag(); got != tag {
		t.Fatalf("InstalledReleaseTag() = %q, want %q", got, tag)
	}
	if err := UninstallBinary(); err != nil {
		t.Fatal(err)
	}
	if got := InstalledReleaseTag(); got != "" {
		t.Fatalf("InstalledReleaseTag() after uninstall = %q", got)
	}
	if _, err := os.Stat(releaseTagPath()); !os.IsNotExist(err) {
		t.Fatalf("release tag sidecar remains after uninstall: %v", err)
	}
}
