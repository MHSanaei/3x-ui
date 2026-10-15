package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestDockerInitPackagesOneXrayBinaryWithRelocatableSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("DockerInit uses a Unix shell and symlinks")
	}
	script, err := filepath.Abs("DockerInit.sh")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	write := func(name string, data []byte, mode fs.FileMode) string {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	var xrayArchive bytes.Buffer
	zw := zip.NewWriter(&xrayArchive)
	header := &zip.FileHeader{Name: "xray"}
	header.SetMode(0o755)
	entry, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("#!/bin/sh\nprintf 'Xray 26.10.10 fixture\\n'\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	xrayArchivePath := write("xray.zip", xrayArchive.Bytes(), 0o600)
	var mtgArchive bytes.Buffer
	gz := gzip.NewWriter(&mtgArchive)
	tw := tar.NewWriter(gz)
	const mtg = "#!/bin/sh\nexit 0\n"
	if err := tw.WriteHeader(&tar.Header{Name: "mtg-multi-1.0.0-linux-amd64/mtg-multi", Mode: 0o755, Size: int64(len(mtg))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(mtg)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	mtgArchivePath := write("mtg.tar.gz", mtgArchive.Bytes(), 0o600)
	write("tools/curl", []byte(`#!/bin/sh
for arg in "$@"; do url=$arg; done
case "$url" in
    https://api.github.com/repos/mhsanaei/mtg-multi/releases/latest)
        printf '{"tag_name":"v1.0.0"}\n'
        ;;
    */Xray-*.zip)
        cp "$XRAY_ARCHIVE_FIXTURE" "${url##*/}"
        ;;
    */mtg-multi-*.tar.gz)
        cp "$MTG_ARCHIVE_FIXTURE" "${url##*/}"
        ;;
    */geoip.dat|*/geosite.dat)
        target=${url##*/}
        if [ "$1" = '-sfLRo' ]; then target=$2; fi
        printf 'geodata fixture\n' > "$target"
        ;;
    *) exit 1 ;;
esac
`), 0o755)
	cmd := exec.Command("sh", script, "amd64")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "tools")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"XRAY_ARCHIVE_FIXTURE="+xrayArchivePath, "MTG_ARCHIVE_FIXTURE="+mtgArchivePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("DockerInit: %v\n%s", err, output)
	}
	app := filepath.Join(root, "app")
	if err := os.Rename(filepath.Join(root, "build"), app); err != nil {
		t.Fatal(err)
	}
	activePath := filepath.Join(app, "bin", "xray-linux-amd64")
	target, err := os.Readlink(activePath)
	if err != nil {
		t.Fatalf("working Xray must be a symlink: %v", err)
	}
	if filepath.IsAbs(target) {
		t.Fatalf("build symlink is tied to the builder path: %q", target)
	}
	output, err := exec.Command(activePath, "-version").CombinedOutput()
	if err != nil || string(output) != "Xray 26.10.10 fixture\n" {
		t.Fatalf("relocated Xray = %q, %v", output, err)
	}
	var binaries []string
	if err := filepath.WalkDir(app, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.Type().IsRegular() && strings.HasPrefix(entry.Name(), "xray") {
			relative, relErr := filepath.Rel(app, path)
			if relErr != nil {
				return relErr
			}
			binaries = append(binaries, relative)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(binaries, []string{filepath.Join("xray-image", "xray-linux-amd64")}) {
		t.Fatalf("physical Xray binaries = %q, want only xray-image/xray-linux-amd64", binaries)
	}
}
