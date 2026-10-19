package amneziawgkern

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

func ownedTestConfig(t *testing.T, id int) string {
	t.Helper()
	inst := testInstance()
	inst.Id = id
	inst.InterfaceName = "awg" + strconv.Itoa(id)
	conf, err := Render(inst, "eth0", false)
	if err != nil {
		t.Fatal(err)
	}
	return "# 3x-ui-owned-amneziawg: " + inst.InterfaceName + "\n" + conf
}
func writeTestConfig(t *testing.T, dir, name, contents string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name+".conf")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestFreshManagerReconcileEmptyRemovesOnlyOwnedRegularConfigs(t *testing.T) {
	dir := t.TempDir()
	ownedPath := writeTestConfig(t, dir, "awg42", ownedTestConfig(t, 42))
	foreignPath := writeTestConfig(t, dir, "awg900", "[Interface]\nPrivateKey = foreign\n")
	legacyPath := writeTestConfig(t, dir, "awg901", "# legacy generated config\n[Interface]\nPrivateKey = old\n")
	linkPath := filepath.Join(dir, "awg43.conf")
	if err := os.Symlink(ownedPath, linkPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	dirPath := filepath.Join(dir, "awg44.conf")
	if err := os.Mkdir(dirPath, 0o700); err != nil {
		t.Fatal(err)
	}
	linkUp := true
	var calls [][]string
	m := &Manager{ifaces: map[int]*managed{}, confDir: dir, ops: managerOps{
		linkExists: func(name string) bool { return name == "awg42" && linkUp },
		run: func(name string, args ...string) (string, error) {
			calls = append(calls, append([]string{name}, args...))
			if name == "awg-quick" && len(args) == 2 && args[0] == "down" && args[1] == "awg42" {
				linkUp = false
				return "", nil
			}
			if name == "nft" && len(args) == 2 && args[0] == "list" && args[1] == "tables" {
				return "table ip awgnat42", nil
			}
			return "", nil
		},
	}}
	m.Reconcile(nil)
	if _, err := os.Lstat(ownedPath); !os.IsNotExist(err) {
		t.Errorf("owned config still exists (err=%v)", err)
	}
	for _, path := range []string{foreignPath, legacyPath, linkPath, dirPath} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("unowned or non-regular path %s was removed: %v", path, err)
		}
	}
	want := [][]string{{"awg-quick", "down", "awg42"}, {"nft", "list", "tables"}, {"nft", "delete", "table", "ip", "awgnat42"}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("cleanup calls = %v, want %v", calls, want)
	}
}
func TestStopAllThenFreshManagerAdoptsDesiredOwnedInterface(t *testing.T) {
	dir := t.TempDir()
	inst := testInstance()
	inst.Id = 45
	inst.InterfaceName = "awg45"
	if err := writeConf(dir, inst.InterfaceName, ownedTestConfig(t, inst.Id)); err != nil {
		t.Fatal(err)
	}
	linkUp := true
	newManager := func() *Manager {
		return &Manager{ifaces: map[int]*managed{}, confDir: dir, ops: managerOps{
			linkExists:           func(string) bool { return linkUp },
			liveInterfaceMatches: func(string, amneziawg.Instance) bool { return true },
			syncPeers:            func(string) error { return nil },
		}}
	}
	first := newManager()
	if err := first.Ensure(Desired{Instance: inst}); err != nil {
		t.Fatal(err)
	}
	first.StopAll()
	second := newManager()
	second.Reconcile([]Desired{{Instance: inst}})
	state, ok := second.TrafficStateFor(inst.Id)
	if !ok || !state.Adopted {
		t.Fatalf("restarted manager state = %+v, %v; want adopted baseline pending", state, ok)
	}
}
func TestRemoveRecoversOwnedConfigAfterDownAndNATCleanupFailure(t *testing.T) {
	dir := t.TempDir()
	path := writeTestConfig(t, dir, "awg46", ownedTestConfig(t, 46))
	linkUp := true
	deleteFails := true
	newManager := func() *Manager {
		return &Manager{ifaces: map[int]*managed{}, confDir: dir, ops: managerOps{
			linkExists: func(name string) bool { return name == "awg46" && linkUp },
			run: func(name string, args ...string) (string, error) {
				if name == "awg-quick" && args[0] == "down" {
					linkUp = false
					return "post-down hook failed", errors.New("down failed")
				}
				if name == "nft" && args[0] == "list" {
					return "table ip awgnat46", nil
				}
				if name == "nft" && args[0] == "delete" && deleteFails {
					return "busy", errors.New("delete failed")
				}
				return "", nil
			},
		}}
	}
	first := newManager()
	if err := first.Remove(46); err == nil {
		t.Fatal("Remove succeeded despite cleanup failure")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("failed cleanup removed config: %v", err)
	}
	deleteFails = false
	second := newManager()
	if err := second.Remove(46); err != nil {
		t.Fatalf("restart retry Remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("successful Remove left private config (err=%v)", err)
	}
}
func TestRemoveRejectsPoisonedOwnedHooksBeforeDown(t *testing.T) {
	dir := t.TempDir()
	config := strings.Replace(ownedTestConfig(t, 47), "[Peer]", "PreDown = touch /tmp/should-not-run\n\n[Peer]", 1)
	path := writeTestConfig(t, dir, "awg47", config)
	var calls int
	m := &Manager{ifaces: map[int]*managed{}, confDir: dir, ops: managerOps{
		linkExists: func(string) bool { return true },
		run:        func(string, ...string) (string, error) { calls++; return "", nil },
	}}
	if err := m.Remove(47); err == nil {
		t.Fatal("Remove accepted an owned config with an injected hook")
	}
	if calls != 0 {
		t.Fatalf("commands issued for poisoned config = %d, want none", calls)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("poisoned config was removed: %v", err)
	}
}
