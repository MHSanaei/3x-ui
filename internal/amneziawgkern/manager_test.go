package amneziawgkern

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

// TestStructuralFingerprintIgnoresPeers is what keeps live sessions alive: if adding a client changed
// the fingerprint, every admin edit would take the interface down and back up and reset every peer's
func TestStructuralFingerprintIgnoresPeers(t *testing.T) {
	t.Parallel()

	base := testInstance()
	withPeer := testInstance()
	withPeer.Peers = append(withPeer.Peers, amneziawg.Peer{
		PublicKey:  "peerCPublicKeyBase64ValueForTestsPadding000=",
		AllowedIPs: []string{"10.8.1.4/32"},
	})

	a, err := Render(base, "eth0", false)
	if err != nil {
		t.Fatalf("Render(base): %v", err)
	}
	b, err := Render(withPeer, "eth0", false)
	if err != nil {
		t.Fatalf("Render(withPeer): %v", err)
	}
	if a == b {
		t.Fatal("adding a peer must change the rendered config")
	}
	if structuralFingerprint(a) != structuralFingerprint(b) {
		t.Error("adding a peer must not change the structural fingerprint, or syncconf is never used")
	}
}

func TestEnsureAdoptionPreservesTrafficBaselineUntilDump(t *testing.T) {
	commands := make([][]string, 0, 2)
	m := &Manager{
		ifaces:  map[int]*managed{},
		confDir: t.TempDir(),
		ops: managerOps{
			linkExists:           func(string) bool { return true },
			liveInterfaceMatches: func(string, amneziawg.Instance) bool { return true },
			writeConf:            func(string, string, string) error { return nil },
			syncPeers:            func(string) error { return nil },
			run: func(name string, args ...string) (string, error) {
				commands = append(commands, append([]string{name}, args...))
				return "", nil
			},
		},
	}
	inst := testInstance()
	inst.InterfaceName = "awg-traffic-lifecycle-test"
	collector := NewCollector()
	if err := m.Ensure(Desired{Instance: inst}); err != nil {
		t.Fatalf("Ensure adopted interface: %v", err)
	}
	state, ok := m.TrafficStateFor(inst.Id)
	if !ok || !state.Adopted {
		t.Fatalf("adopted traffic state = %+v, %v; want pending baseline", state, ok)
	}

	inst.Peers = append(inst.Peers, amneziawg.Peer{PublicKey: "newPeerPublicKeyBase64ValueForTests000000="})
	if err := m.Ensure(Desired{Instance: inst}); err != nil {
		t.Fatalf("Ensure peer sync: %v", err)
	}
	afterSync, _ := m.TrafficStateFor(inst.Id)
	if afterSync != state {
		t.Fatalf("peer sync changed traffic state: before %+v, after %+v", state, afterSync)
	}

	if !m.PrimeAdoptedTrafficBaseline(inst.Id, state.Generation, []PeerStat{{PublicKey: "p", RxBytes: 125, TxBytes: 250}}, collector) {
		t.Fatal("first successful adopted dump did not prime the collector")
	}
	if got := collector.Delta(inst.InterfaceName, []PeerStat{{PublicKey: "p", RxBytes: 125, TxBytes: 250}}); len(got) != 0 {
		t.Fatalf("same counters after prime = %+v, want no delta", got)
	}
	got := collector.Delta(inst.InterfaceName, []PeerStat{{PublicKey: "p", RxBytes: 150, TxBytes: 300}})
	if len(got) != 1 || got[0].Up != 25 || got[0].Down != 50 {
		t.Fatalf("delta after prime = %+v, want 25/50", got)
	}
	afterPrime, _ := m.TrafficStateFor(inst.Id)
	if afterPrime.Adopted || afterPrime.Generation != state.Generation {
		t.Fatalf("traffic state after prime = %+v, want same generation without pending baseline", afterPrime)
	}

	inst.MTU++
	if err := m.Ensure(Desired{Instance: inst}); err != nil {
		t.Fatalf("Ensure rebuilt interface: %v", err)
	}
	afterRebuild, _ := m.TrafficStateFor(inst.Id)
	if afterRebuild.Adopted || afterRebuild.Generation == state.Generation {
		t.Fatalf("rebuilt traffic state = %+v, want a fresh generation without adoption baseline", afterRebuild)
	}
	if m.PrimeAdoptedTrafficBaseline(inst.Id, state.Generation, []PeerStat{{PublicKey: "p", RxBytes: 1, TxBytes: 2}}, collector) {
		t.Fatal("stale adoption generation primed a rebuilt interface")
	}
	if got = collector.Delta(inst.InterfaceName, []PeerStat{{PublicKey: "p", RxBytes: 150, TxBytes: 300}}); len(got) != 0 {
		t.Fatalf("stale prime changed collector baseline after rebuild: %+v", got)
	}
	got = collector.Delta(inst.InterfaceName, []PeerStat{{PublicKey: "p", RxBytes: 175, TxBytes: 350}})
	if len(got) != 1 || got[0].Up != 25 || got[0].Down != 50 {
		t.Fatalf("delta after rejected stale prime = %+v, want 25/50", got)
	}
	if !reflect.DeepEqual(commands, [][]string{{"awg-quick", "down", interfaceName(inst)}, {"awg-quick", "up", interfaceName(inst)}}) {
		t.Fatalf("rebuild commands = %v, want awg-quick down/up", commands)
	}
}

// TestStructuralFingerprintCatchesInterfaceChanges: these are the properties the kernel fixes when the
// interface is created, so missing one here would leave the live interface permanently out of sync
func TestStructuralFingerprintCatchesInterfaceChanges(t *testing.T) {
	t.Parallel()

	base, err := Render(testInstance(), "eth0", false)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	baseFP := structuralFingerprint(base)

	cases := map[string]func(*amneziawg.Instance){
		"listen port": func(i *amneziawg.Instance) { i.ListenPort = 40000 },
		"mtu":         func(i *amneziawg.Instance) { i.MTU = 1280 },
		"address":     func(i *amneziawg.Instance) { i.Address = []string{"10.9.9.1/24"} },
		"private key": func(i *amneziawg.Instance) { i.PrivateKey = "otherPrivateKeyBase64ValueForTestsPadding00=" },
		"obfuscation": func(i *amneziawg.Instance) { i.Obfuscation.S4 = 21 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			inst := testInstance()
			mutate(&inst)
			got, err := Render(inst, "eth0", false)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if structuralFingerprint(got) == baseFP {
				t.Errorf("changing the %s must change the structural fingerprint", name)
			}
		})
	}
}

// TestStructuralFingerprintCatchesEgressInterface: the NAT block lives in the interface section, and a
// masquerade pointing at the wrong NIC black-holes every peer.
func TestStructuralFingerprintCatchesEgressInterface(t *testing.T) {
	t.Parallel()

	a, err := Render(testInstance(), "eth0", false)
	if err != nil {
		t.Fatalf("Render(eth0): %v", err)
	}
	b, err := Render(testInstance(), "ens3", false)
	if err != nil {
		t.Fatalf("Render(ens3): %v", err)
	}
	if structuralFingerprint(a) == structuralFingerprint(b) {
		t.Error("changing the egress interface must force a rebuild, not a peer sync")
	}
}

func TestInterfaceNameFallsBackToInboundID(t *testing.T) {
	t.Parallel()

	inst := testInstance()
	inst.InterfaceName = ""
	inst.Id = 42
	if got, want := interfaceName(inst), "awg42"; got != want {
		t.Errorf("interfaceName = %q, want %q", got, want)
	}

	inst.InterfaceName = "awgcustom"
	if got, want := interfaceName(inst), "awgcustom"; got != want {
		t.Errorf("interfaceName = %q, want %q", got, want)
	}
}

// TestWriteConfIsPrivateAndComplete: awg-quick refuses a world-readable
// config, and the file holds the interface private key.
func TestWriteConfIsPrivateAndComplete(t *testing.T) {
	dir := t.TempDir()

	conf, err := Render(testInstance(), "eth0", false)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if err := writeConf(dir, "awgtest", conf); err != nil {
		t.Fatalf("writeConf: %v", err)
	}

	path := filepath.Join(dir, "awgtest.conf")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written config: %v", err)
	}
	if string(got) != conf {
		t.Error("written config does not match what was rendered")
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if perm := fi.Mode().Perm(); perm != 0o600 {
			t.Errorf("config mode = %o, want 600", perm)
		}
	}

	// Rewriting must replace, not append or leave a temp file behind.
	if err := writeConf(dir, "awgtest", conf); err != nil {
		t.Fatalf("second writeConf: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected only the config to remain, got %v", names)
	}
}

// TestAddressesSatisfiedIsASubsetCheck: the kernel adds an IPv6 link-local address nobody configured,
// so requiring an exact match would make every interface look wrong and force a needless rebuild on
func TestAddressesSatisfiedIsASubsetCheck(t *testing.T) {
	t.Parallel()

	live := []string{"10.8.1.1/24", "fe80::be24:11ff:fe3f:5f75/64"}
	if !addressesSatisfied([]string{"10.8.1.1/24"}, live) {
		t.Error("an extra kernel-added address must not disqualify the interface")
	}
	if addressesSatisfied([]string{"10.9.9.1/24"}, live) {
		t.Error("a different address must disqualify the interface")
	}
	if addressesSatisfied([]string{"10.8.1.1/24", "fd00::1/64"}, live) {
		t.Error("a missing wanted address must disqualify the interface")
	}
	if addressesSatisfied(nil, live) {
		t.Error("an instance with no address must never be adopted")
	}
}

// TestLiveInterfaceMatchesRejectsMissingLink is the guard that keeps adoption
// from claiming an interface that is not there.
func TestLiveInterfaceMatchesRejectsMissingLink(t *testing.T) {
	t.Parallel()

	if liveInterfaceMatches("awg-does-not-exist-9999", testInstance()) {
		t.Error("a nonexistent interface must never be adopted")
	}
}

// TestStopAllLeavesInterfacesRunning: the kernel datapath keeps forwarding while the panel is down, so
// shutdown must only drop bookkeeping.
func TestStopAllLeavesInterfacesRunning(t *testing.T) {
	m := &Manager{ifaces: map[int]*managed{
		1: {name: "awg-test-never-created", conf: "x", structFP: "y"},
	}, confDir: t.TempDir()}

	m.StopAll()

	if m.HasRunning() {
		t.Error("StopAll must clear the manager's bookkeeping")
	}
	// Clearing bookkeeping must not issue commands during process shutdown.
}

func TestRemoveRetainsManagedInterfaceAfterTeardownFailure(t *testing.T) {
	calls := 0
	m := &Manager{
		ifaces: map[int]*managed{7: {name: "awg-remove-retry"}},
		ops: managerOps{
			linkExists: func(string) bool { return true },
			run: func(name string, args ...string) (string, error) {
				calls++
				if calls == 1 {
					return "temporary failure", os.ErrProcessDone
				}
				return "", nil
			},
		},
	}

	if err := m.Remove(7); err == nil {
		t.Fatal("Remove must report a failed awg-quick down")
	}
	if !m.Running(7) {
		t.Fatal("failed teardown must remain managed so the next reconcile can retry")
	}
	if err := m.Remove(7); err != nil {
		t.Fatalf("Remove retry: %v", err)
	}
	if m.Running(7) {
		t.Fatal("successful retry must forget the interface")
	}
	if calls != 2 {
		t.Fatalf("teardown attempts = %d, want 2", calls)
	}
}

func TestReconcileRetriesRemovedInterfaceAfterTeardownFailure(t *testing.T) {
	calls := 0
	m := &Manager{
		ifaces: map[int]*managed{8: {name: "awg-reconcile-retry"}},
		ops: managerOps{
			linkExists: func(string) bool { return true },
			run: func(name string, args ...string) (string, error) {
				calls++
				if calls == 1 {
					return "temporary failure", os.ErrProcessDone
				}
				return "", nil
			},
		},
	}

	m.Reconcile(nil)
	if !m.Running(8) {
		t.Fatal("failed reconcile teardown must remain managed for retry")
	}
	m.Reconcile(nil)
	if m.Running(8) {
		t.Fatal("successful reconcile retry must forget the interface")
	}
	if calls != 2 {
		t.Fatalf("teardown attempts = %d, want 2", calls)
	}
}
