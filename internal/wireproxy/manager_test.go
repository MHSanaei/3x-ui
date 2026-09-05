package wireproxy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withFakeManagerBin points managerBinPath at a throwaway file for the
// duration of the test and restores it after -- never touches the real
// /usr/local/bin/warpwp.
func withFakeManagerBin(t *testing.T, present bool) {
	t.Helper()
	real := managerBinPath
	t.Cleanup(func() { managerBinPath = real })
	managerBinPath = filepath.Join(t.TempDir(), "warpwp")
	if present {
		if err := os.WriteFile(managerBinPath, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
			t.Fatalf("seeding fake manager binary: %v", err)
		}
	}
}

func TestIsInstalledReflectsManagerBinPath(t *testing.T) {
	withFakeManagerBin(t, false)
	if IsInstalled() {
		t.Error("IsInstalled() = true before the fake manager binary was written")
	}
	if err := os.WriteFile(managerBinPath, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatalf("writing fake manager binary: %v", err)
	}
	if !IsInstalled() {
		t.Error("IsInstalled() = false after the fake manager binary was written")
	}
}

func TestStartFailsWhenNotInstalled(t *testing.T) {
	withFakeManagerBin(t, false)
	err := Start(context.Background())
	if err == nil {
		t.Fatal("Start() with no manager binary present returned nil error")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("Start() error = %q, want it to say not installed", err.Error())
	}
}

func TestRepairRejectsUnknownMode(t *testing.T) {
	// No fake binary needed -- mode validation happens before Repair would
	// ever shell out, so this must fail without touching managerBinPath at all.
	withFakeManagerBin(t, false)
	err := Repair(context.Background(), RepairMode("--rm -rf /"))
	if err == nil {
		t.Fatal("Repair with an unrecognized mode returned nil error")
	}
	if !strings.Contains(err.Error(), "unknown repair mode") {
		t.Errorf("Repair error = %q, want it to say unknown repair mode", err.Error())
	}
}

// The embedded scripts are this package's entire security-relevant surface
// -- pin their shape lightly so a future edit can't accidentally embed the
// wrong file or truncate one.
func TestEmbeddedScriptsLookReal(t *testing.T) {
	if !strings.HasPrefix(string(managerScript), "#!/usr/bin/env bash") {
		t.Error("managerScript does not start with the expected shebang")
	}
	repoRawLine := ""
	for _, line := range strings.Split(string(managerScript), "\n") {
		if strings.HasPrefix(line, "REPO_RAW=") {
			repoRawLine = line
			break
		}
	}
	if repoRawLine == "" {
		t.Fatal("managerScript has no REPO_RAW= assignment -- expected the pinning patch to be there")
	}
	if !strings.Contains(repoRawLine, "kuzzrus/WARP_WireProxy_Manager") {
		t.Errorf("REPO_RAW line = %q, want it to reference kuzzrus/WARP_WireProxy_Manager", repoRawLine)
	}
	if strings.HasSuffix(repoRawLine, `/main"`) {
		t.Errorf("REPO_RAW line = %q, still points at the live \"main\" branch instead of a pinned commit", repoRawLine)
	}
	if !strings.HasPrefix(string(nativeScript), "#!/usr/bin/env bash") {
		t.Error("nativeScript does not start with the expected shebang")
	}
}
