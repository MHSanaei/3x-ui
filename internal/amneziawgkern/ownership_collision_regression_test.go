package amneziawgkern

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

func TestEnsureRefusesForeignInterfaceBeforeConfigOrRuntimeMutation(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string) (amneziawg.Instance, string, []byte, bool, bool)
	}{
		{
			name: "unmarked config",
			setup: func(t *testing.T, dir string) (amneziawg.Instance, string, []byte, bool, bool) {
				inst := testInstance()
				inst.Id = 70
				inst.InterfaceName = ownedInterfaceName(inst.Id)
				path := writeTestConfig(t, dir, "awg70", "[Interface]\nPrivateKey = foreign\n")
				before, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				return inst, path, before, false, false
			},
		},
		{
			name: "symlink config",
			setup: func(t *testing.T, dir string) (amneziawg.Instance, string, []byte, bool, bool) {
				inst := testInstance()
				inst.Id = 71
				inst.InterfaceName = ownedInterfaceName(inst.Id)
				target := writeTestConfig(t, dir, "foreign", "[Interface]\nPrivateKey = foreign\n")
				path := filepath.Join(dir, "awg71.conf")
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				before, err := os.ReadFile(target)
				if err != nil {
					t.Fatal(err)
				}
				return inst, path, before, true, false
			},
		},
		{
			name: "non-regular config",
			setup: func(t *testing.T, dir string) (amneziawg.Instance, string, []byte, bool, bool) {
				inst := testInstance()
				inst.Id = 72
				inst.InterfaceName = ownedInterfaceName(inst.Id)
				path := filepath.Join(dir, "awg72.conf")
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				return inst, path, nil, false, true
			},
		},
		{
			name: "occupied link without config",
			setup: func(t *testing.T, _ string) (amneziawg.Instance, string, []byte, bool, bool) {
				inst := testInstance()
				inst.Id = 73
				inst.InterfaceName = ownedInterfaceName(inst.Id)
				return inst, "", nil, false, false
			},
		},
		{
			name: "owned config with foreign interface key",
			setup: func(t *testing.T, dir string) (amneziawg.Instance, string, []byte, bool, bool) {
				inst := testInstance()
				inst.Id = 74
				inst.InterfaceName = "awg74"
				inst.PrivateKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
				conf, err := Render(inst, "eth0", false)
				if err != nil {
					t.Fatal(err)
				}
				path := writeTestConfig(t, dir, "awg74", conf)
				before, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				return inst, path, before, false, false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			inst, configPath, before, wantSymlink, wantDir := tt.setup(t, dir)
			var writes, syncs int
			var mutatingCommands [][]string
			m := &Manager{ifaces: map[int]*managed{}, confDir: dir, ops: managerOps{
				linkExists:           func(string) bool { return true },
				liveInterfaceMatches: func(string, amneziawg.Instance) bool { return true },
				writeConf: func(dir, name, conf string) error {
					writes++
					return writeConf(dir, name, conf)
				},
				syncPeers: func(string) error {
					syncs++
					return nil
				},
				run: func(name string, args ...string) (string, error) {
					if name == "awg" && len(args) == 3 && args[0] == "show" && args[1] == ownedInterfaceName(inst.Id) && args[2] == "public-key" {
						return "foreign-interface-public-key", nil
					}
					mutatingCommands = append(mutatingCommands, append([]string{name}, args...))
					return "", errors.New("unexpected command")
				},
			}}

			err := m.Ensure(Desired{Instance: inst, EgressInterface: "eth0"})
			if err == nil || !strings.Contains(err.Error(), "refusing") {
				t.Fatalf("Ensure error = %v, want an explicit ownership refusal", err)
			}
			if writes != 0 || syncs != 0 || len(mutatingCommands) != 0 {
				t.Fatalf("foreign interface side effects: writes=%d syncs=%d commands=%v", writes, syncs, mutatingCommands)
			}
			if configPath == "" {
				if _, statErr := os.Lstat(ConfPathIn(dir, ownedInterfaceName(inst.Id))); !os.IsNotExist(statErr) {
					t.Fatalf("occupied link created a config (stat error=%v)", statErr)
				}
				return
			}
			fi, statErr := os.Lstat(configPath)
			if statErr != nil {
				t.Fatalf("config path changed or disappeared: %v", statErr)
			}
			if wantSymlink {
				if fi.Mode()&os.ModeSymlink == 0 {
					t.Fatal("foreign symlink was replaced")
				}
				target, err := os.Readlink(configPath)
				if err != nil {
					t.Fatal(err)
				}
				after, err := os.ReadFile(target)
				if err != nil || string(after) != string(before) {
					t.Fatalf("symlink target changed: read error=%v", err)
				}
				return
			}
			if wantDir {
				if !fi.IsDir() {
					t.Fatal("non-regular config path was replaced")
				}
				return
			}
			after, err := os.ReadFile(configPath)
			if err != nil || string(after) != string(before) {
				t.Fatalf("foreign config changed: read error=%v", err)
			}
		})
	}
}
