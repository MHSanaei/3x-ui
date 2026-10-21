package amneziawgkern

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

func TestOwnedInterfaceKeyRotationKeepsDiskAndLiveKeyInSync(t *testing.T) {
	dir := t.TempDir()
	inst := testInstance()
	inst.Id = 79
	inst.InterfaceName = ownedInterfaceName(inst.Id)
	inst.PrivateKey = managerTestPrivateKey
	path := writeTestConfig(t, dir, inst.InterfaceName, ownedTestConfig(t, inst.Id))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	oldPublicKey := managerTestPublicKey(t)
	inst.PrivateKey = managerTestForeignPrivateKey
	newPublicKey := managerTestForeignPublicKey(t)
	linkUp := true
	livePublicKey := oldPublicKey
	downCalls, upCalls := 0, 0
	m := &Manager{ifaces: map[int]*managed{}, confDir: dir, ops: managerOps{
		linkExists:           func(name string) bool { return name == inst.InterfaceName && linkUp },
		liveInterfaceMatches: func(string, amneziawg.Instance) bool { return false },
		run: func(name string, args ...string) (string, error) {
			switch {
			case name == "awg" && len(args) == 3 && args[0] == "show" && args[1] == inst.InterfaceName && args[2] == "public-key":
				return livePublicKey, nil
			case name == "awg-quick" && len(args) == 2 && args[0] == "down" && args[1] == inst.InterfaceName:
				if !linkUp {
					return "", errors.New("interface already down")
				}
				linkUp = false
				downCalls++
				return "", nil
			case name == "awg-quick" && len(args) == 2 && args[0] == "up" && args[1] == inst.InterfaceName:
				if linkUp {
					return "", errors.New("interface still up")
				}
				linkUp = true
				livePublicKey = newPublicKey
				upCalls++
				return "", nil
			case name == "nft" && len(args) == 2 && args[0] == "list" && args[1] == "tables":
				return "", nil
			default:
				return "", errors.New("unexpected command")
			}
		},
	}}

	if err := m.Ensure(Desired{Instance: inst, EgressInterface: "eth0"}); err != nil {
		t.Fatalf("Ensure key rotation failed: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rotated config: %v", err)
	}
	if string(after) == string(before) {
		t.Fatal("key rotation left the old config on disk")
	}
	configPublicKey, err := configInterfacePublicKey(string(after))
	if err != nil || configPublicKey != newPublicKey || livePublicKey != newPublicKey {
		t.Fatal("rotated config and live interface do not both use the requested key")
	}
	if downCalls != 1 || upCalls != 1 || !linkUp {
		t.Fatalf("rotation lifecycle: down=%d up=%d linkUp=%v", downCalls, upCalls, linkUp)
	}

	if err := m.Ensure(Desired{Instance: inst, EgressInterface: "eth0"}); err != nil {
		t.Fatalf("Ensure after key rotation failed: %v", err)
	}
	if downCalls != 1 || upCalls != 1 {
		t.Fatalf("unchanged Ensure rebuilt interface: down=%d up=%d", downCalls, upCalls)
	}
	if err := m.Remove(inst.Id); err != nil {
		t.Fatalf("Remove after key rotation failed: %v", err)
	}
	if linkUp {
		t.Fatal("Remove left rotated interface running")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("Remove left owned config on disk (err=%v)", err)
	}
}

func TestEnsurePreservesOwnedConfigWhenLiveKeyProbeFails(t *testing.T) {
	dir := t.TempDir()
	inst := testInstance()
	inst.Id = 80
	inst.InterfaceName = ownedInterfaceName(inst.Id)
	inst.PrivateKey = managerTestPrivateKey
	path := writeTestConfig(t, dir, inst.InterfaceName, ownedTestConfig(t, inst.Id))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	linkUp := true
	probeCalls, writeCalls, syncCalls, mutationCalls := 0, 0, 0, 0
	m := &Manager{ifaces: map[int]*managed{}, confDir: dir, ops: managerOps{
		linkExists: func(name string) bool { return name == inst.InterfaceName && linkUp },
		writeConf: func(string, string, string) error {
			writeCalls++
			return nil
		},
		syncPeers: func(string) error {
			syncCalls++
			return nil
		},
		run: func(name string, args ...string) (string, error) {
			if name == "awg" && len(args) == 3 && args[0] == "show" && args[1] == inst.InterfaceName && args[2] == "public-key" {
				probeCalls++
				return "", errors.New("probe unavailable")
			}
			mutationCalls++
			return "", errors.New("runtime mutation must not run")
		},
	}}

	err = m.Ensure(Desired{Instance: inst, EgressInterface: "eth0"})
	if err == nil || !strings.Contains(err.Error(), "cannot verify live AmneziaWG interface") {
		t.Fatal("Ensure did not report the live-key probe failure")
	}
	if strings.Contains(err.Error(), managerTestPrivateKey) || strings.Contains(err.Error(), managerTestForeignPrivateKey) {
		t.Fatal("Ensure exposed a private key in its error")
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil || string(after) != string(before) || !linkUp {
		t.Fatalf("probe failure changed config or link state: readErr=%v linkUp=%v", readErr, linkUp)
	}
	if probeCalls != 1 || writeCalls != 0 || syncCalls != 0 || mutationCalls != 0 {
		t.Fatalf("probe failure side effects: probes=%d writes=%d syncs=%d mutations=%d", probeCalls, writeCalls, syncCalls, mutationCalls)
	}
}
