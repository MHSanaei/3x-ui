package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVKTurnDownloadRejectsTamperedAssetWithoutReplacingBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX packaging script is used by Linux and Docker release builds")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	curl := "#!/bin/sh\nwhile [ \"$#\" -gt 0 ]; do\n case \"$1\" in -o) shift; printf 'tampered binary' > \"$1\";; esac\n shift\ndone\n"
	if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(curl), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "vk-turn-proxy-server")
	if err := os.WriteFile(target, []byte("previous verified binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "download-vk-turn.sh", "linux", "amd64", target)
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "checksum mismatch") {
		t.Fatalf("tampered download must fail checksum verification, got %v\n%s", err, out)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "previous verified binary" {
		t.Fatalf("failed verification replaced existing binary with %q", content)
	}
}
