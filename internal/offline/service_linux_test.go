package offline

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

func TestApplyUpdateKeepsOrRestoresSQLite(t *testing.T) {
	for _, shouldStart := range []bool{true, false} {
		t.Run(fmt.Sprintf("ready=%t", shouldStart), func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XUI_DB_FOLDER", dir)
			t.Setenv("XUI_DB_TYPE", "sqlite")
			t.Setenv("XUI_BIN_FOLDER", dir)
			panel := filepath.Join(dir, "x-ui")
			geo := filepath.Join(dir, "geoip.dat")
			db := config.GetDBPath()
			wal := db + "-wal"
			for path, contents := range map[string]string{panel: "installed panel", geo: "original geo", db: "original DB"} {
				if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			archive := makeZIP(t,
				zipEntry{"format", Format + "\n", 0o644},
				zipEntry{"geoip.dat", "updated geo", 0o644},
			)
			var calls []string
			policy := func() error {
				calls = append(calls, "policy")
				return nil
			}
			control := func(args ...string) (string, error) {
				calls = append(calls, args[0])
				return "", nil
			}
			ready := func(bool) bool {
				if got, err := os.ReadFile(geo); err != nil || string(got) != "updated geo" {
					t.Errorf("staged geodata = %q, %v", got, err)
				}
				if !shouldStart {
					if err := os.WriteFile(db, []byte("partial migration"), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(wal, []byte("new WAL"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return shouldStart
			}
			err := apply(archive, panel, control, policy, ready)
			if (err == nil) != shouldStart {
				t.Fatalf("apply = %v, want success=%t", err, shouldStart)
			}
			wantGeo := "updated geo"
			wantCalls := []string{"policy", "stop", "start"}
			if !shouldStart {
				wantGeo = "original geo"
				wantCalls = append(wantCalls, "stop", "start")
			}
			if !slices.Equal(calls, wantCalls) {
				t.Errorf("service sequence = %v, want %v", calls, wantCalls)
			}
			for path, want := range map[string]string{panel: "installed panel", geo: wantGeo, db: "original DB"} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Errorf("%s = %q, %v; want %q", path, got, err, want)
				}
			}
			if _, err := os.Stat(wal); !os.IsNotExist(err) {
				t.Errorf("unexpected WAL after update: %v", err)
			}
		})
	}
}
