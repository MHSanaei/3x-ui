package amneziawgkern

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

const (
	testPrivateKey  = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	otherPrivateKey = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="
)

var errInjected = errors.New("injected failure")

type fakeLink struct {
	publicKey string
	addresses []string
	mtu       int
	up        bool
	conf      string
}

// fakeHost models the kernel state the manager drives, so tests assert what
// the host ends up with rather than which commands ran.
type fakeHost struct {
	links      map[string]*fakeLink
	nat        map[string]string
	forward    []string
	iptables   bool
	module     bool
	forwarding bool
	commands   []string
	failing    map[string]error
}

func newFakeHost() *fakeHost {
	return &fakeHost{links: map[string]*fakeLink{}, nat: map[string]string{}, iptables: true, failing: map[string]error{}}
}

func (h *fakeHost) linkExists(name string) bool {
	_, ok := h.links[name]
	return ok
}

func (h *fakeHost) linkMatches(name string, addresses []string, mtu int) bool {
	l, ok := h.links[name]
	return ok && l.mtu == mtu && addressesSatisfied(addresses, l.addresses)
}

func (h *fakeHost) hasCommand(name string) bool { return name != "iptables" || h.iptables }
func (h *fakeHost) moduleLoaded() bool          { return h.module }

func (h *fakeHost) enableIPv4Forwarding() error {
	h.forwarding = true
	return nil
}

func (h *fakeHost) run(stdin, name string, args ...string) (string, error) {
	cmd := strings.Join(append([]string{name}, args...), " ")
	h.commands = append(h.commands, cmd)
	for prefix, err := range h.failing {
		if strings.HasPrefix(cmd, prefix) {
			return "injected", err
		}
	}
	switch name {
	case "modprobe":
		h.module = true
		return "", nil
	case "ip":
		return "", h.ip(args)
	case "awg":
		return h.awg(args)
	case "nft":
		h.nftApply(stdin)
		return "", nil
	case "iptables":
		return h.iptablesCmd(args)
	}
	return "", fmt.Errorf("fake host: unexpected command %q", cmd)
}

func (h *fakeHost) ip(args []string) error {
	switch {
	case len(args) == 6 && args[0] == "link" && args[1] == "add" && args[5] == "amneziawg":
		if h.linkExists(args[3]) {
			return errors.New("RTNETLINK answers: File exists")
		}
		h.links[args[3]] = &fakeLink{}
	case len(args) == 4 && args[0] == "link" && args[1] == "del":
		if !h.linkExists(args[3]) {
			return errors.New("Cannot find device")
		}
		delete(h.links, args[3])
	case len(args) == 6 && args[0] == "-4" && args[2] == "add":
		l, ok := h.links[args[5]]
		if !ok {
			return errors.New("Cannot find device")
		}
		l.addresses = append(l.addresses, args[3])
	case len(args) == 7 && args[0] == "link" && args[1] == "set" && args[4] == "mtu":
		l, ok := h.links[args[3]]
		if !ok {
			return errors.New("Cannot find device")
		}
		l.mtu, _ = strconv.Atoi(args[5])
		l.up = args[6] == "up"
	default:
		return fmt.Errorf("fake host: unexpected ip %v", args)
	}
	return nil
}

func (h *fakeHost) awg(args []string) (string, error) {
	if len(args) != 3 {
		return "", fmt.Errorf("fake host: unexpected awg %v", args)
	}
	l, ok := h.links[args[1]]
	if !ok {
		return "", errors.New("Unable to access interface: No such device")
	}
	switch args[0] {
	case "show":
		if l.publicKey == "" {
			return "(none)", nil
		}
		return l.publicKey, nil
	case "setconf", "syncconf":
		data, err := os.ReadFile(args[2])
		if err != nil {
			return "", err
		}
		pub, err := configInterfacePublicKey(string(data))
		if err != nil {
			return "", err
		}
		l.publicKey, l.conf = pub, string(data)
		return "", nil
	}
	return "", fmt.Errorf("fake host: unexpected awg %v", args)
}

func (h *fakeHost) nftApply(tx string) {
	for _, line := range strings.Split(tx, "\n") {
		f := strings.Fields(line)
		switch {
		case len(f) == 4 && f[0] == "delete" && f[1] == "table":
			delete(h.nat, f[3])
		case len(f) == 4 && f[0] == "table" && f[3] == "{":
			h.nat[f[2]] = tx
		}
	}
}

func (h *fakeHost) iptablesCmd(args []string) (string, error) {
	if len(args) < 4 || args[0] != "-w" || args[3] != "FORWARD" {
		return "", fmt.Errorf("fake host: unexpected iptables %v", args)
	}
	rest := args[4:]
	switch args[2] {
	case "-S":
		// Like iptables-nft, print the comment quoted.
		listing := []string{"-P FORWARD DROP"}
		for _, r := range h.forward {
			f := strings.Fields(r)
			for i := 0; i+1 < len(f); i++ {
				if f[i] == "--comment" {
					f[i+1] = strconv.Quote(f[i+1])
				}
			}
			listing = append(listing, strings.Join(f, " "))
		}
		return strings.Join(listing, "\n"), nil
	case "-I":
		h.forward = append([]string{"-A FORWARD " + strings.Join(rest[1:], " ")}, h.forward...)
		return "", nil
	case "-D":
		rule := "-A FORWARD " + strings.Join(rest, " ")
		for i, r := range h.forward {
			if r == rule {
				h.forward = append(h.forward[:i], h.forward[i+1:]...)
				return "", nil
			}
		}
		return "", errors.New("Bad rule (does a matching rule exist in that chain?)")
	}
	return "", fmt.Errorf("fake host: unexpected iptables %v", args)
}

func (h *fakeHost) ran(prefix string) bool {
	for _, c := range h.commands {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func publicKeyOf(t *testing.T, privateKey string) string {
	t.Helper()
	pub, err := wgutil.PublicKeyFromPrivate(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func managerDesired(id int) Desired {
	inst := testInstance()
	inst.Id = id
	inst.PrivateKey = testPrivateKey
	return Desired{Instance: inst, EgressInterface: "eth0"}
}

func newTestManager(t *testing.T) (*Manager, *fakeHost, string) {
	t.Helper()
	dir := t.TempDir()
	h := newFakeHost()
	return newManager(h, dir), h, dir
}

func diskKey(t *testing.T, dir string, id int) string {
	t.Helper()
	conf, found, err := readOwnedConfig(dir, id)
	if err != nil || !found {
		t.Fatalf("owned config for %d: found=%v err=%v", id, found, err)
	}
	pub, err := configInterfacePublicKey(conf)
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func forwardRulesFor(h *fakeHost, name string) []string {
	var out []string
	for _, r := range h.forward {
		if strings.Contains(r, "--comment "+forwardComment(name)) {
			out = append(out, r)
		}
	}
	return out
}

// TestEnsureBuildsInterfaceWithoutShell: every step is a direct exec, so no
// client-controlled value ever reaches a shell running as root.
func TestEnsureBuildsInterfaceWithoutShell(t *testing.T) {
	m, h, dir := newTestManager(t)
	if err := m.Ensure(managerDesired(1)); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	link := h.links["xui-awg1"]
	if link == nil || !link.up || link.mtu != 1380 || strings.Join(link.addresses, ",") != "10.8.1.1/24" {
		t.Fatalf("link = %+v, want xui-awg1 up at MTU 1380 with 10.8.1.1/24", link)
	}
	if link.publicKey != publicKeyOf(t, testPrivateKey) || !strings.Contains(link.conf, "PublicKey = peerBPublicKeyBase64ValueForTestsPadding000=") {
		t.Fatalf("link was not configured from the rendered config: %+v", link)
	}
	if !h.module || !h.forwarding {
		t.Fatalf("module loaded=%v ip_forward=%v, want both", h.module, h.forwarding)
	}
	if !strings.Contains(h.nat["xui_awg1"], `ip saddr 10.8.1.0/24 oifname "eth0" masquerade`) {
		t.Fatalf("NAT table = %q", h.nat["xui_awg1"])
	}
	if got := forwardRulesFor(h, "xui-awg1"); len(got) != 2 {
		t.Fatalf("FORWARD accepts = %v, want the outbound and the established-return rule", got)
	}
	for _, c := range h.commands {
		switch strings.Fields(c)[0] {
		case "ip", "awg", "nft", "iptables", "modprobe":
		default:
			t.Errorf("unexpected command %q", c)
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(ConfPathIn(dir, "xui-awg1"))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("config holding the private key: mode %v err %v, want 0600", info.Mode().Perm(), err)
		}
	}
}

// TestEnsureUnchangedTickRunsNothing: the job re-ensures every inbound every
// 10s; an unchanged one must not fork a single process.
func TestEnsureUnchangedTickRunsNothing(t *testing.T) {
	m, h, _ := newTestManager(t)
	if err := m.Ensure(managerDesired(1)); err != nil {
		t.Fatal(err)
	}
	h.commands = nil
	if err := m.Ensure(managerDesired(1)); err != nil {
		t.Fatal(err)
	}
	if len(h.commands) != 0 {
		t.Fatalf("unchanged ensure ran %v", h.commands)
	}
}

// TestEnsurePeerEditSyncsInPlace: rebuilding the link on a peer edit would
// drop every other client's session.
func TestEnsurePeerEditSyncsInPlace(t *testing.T) {
	m, h, _ := newTestManager(t)
	if err := m.Ensure(managerDesired(1)); err != nil {
		t.Fatal(err)
	}
	h.commands = nil
	d := managerDesired(1)
	d.Instance.Peers = d.Instance.Peers[:1]
	if err := m.Ensure(d); err != nil {
		t.Fatal(err)
	}
	if h.ran("ip link del") || h.ran("ip link add") || !h.ran("awg syncconf xui-awg1") {
		t.Fatalf("peer edit ran %v, want only a syncconf", h.commands)
	}
	if strings.Contains(h.links["xui-awg1"].conf, "peerB") {
		t.Fatal("removed peer is still on the live interface")
	}
}

// TestEnsureEgressChangeMovesNATAndForwardRules: a stale masquerade or
// FORWARD accept would keep pointing at the NIC the admin moved away from.
func TestEnsureEgressChangeMovesNATAndForwardRules(t *testing.T) {
	m, h, _ := newTestManager(t)
	if err := m.Ensure(managerDesired(1)); err != nil {
		t.Fatal(err)
	}
	d := managerDesired(1)
	d.EgressInterface = "ens3"
	if err := m.Ensure(d); err != nil {
		t.Fatal(err)
	}
	if h.ran("ip link del") {
		t.Fatal("an egress change rebuilt the link")
	}
	if nat := h.nat["xui_awg1"]; strings.Contains(nat, `"eth0"`) || !strings.Contains(nat, `oifname "ens3"`) {
		t.Fatalf("NAT table after egress change = %q", nat)
	}
	rules := forwardRulesFor(h, "xui-awg1")
	if len(rules) != 2 || strings.Contains(strings.Join(rules, "\n"), "eth0") {
		t.Fatalf("FORWARD accepts after egress change = %v, want two for ens3 only", rules)
	}
}

// TestKeyRotationKeepsOldConfigUntilOldLinkIsGone: the old key on disk is
// what proves a leftover link is the panel's after a failed teardown.
func TestKeyRotationKeepsOldConfigUntilOldLinkIsGone(t *testing.T) {
	m, h, dir := newTestManager(t)
	if err := m.Ensure(managerDesired(1)); err != nil {
		t.Fatal(err)
	}
	rotated := managerDesired(1)
	rotated.Instance.PrivateKey = otherPrivateKey

	h.failing["ip link del"] = errInjected
	if err := m.Ensure(rotated); err == nil {
		t.Fatal("rotation succeeded although the old link could not be removed")
	}
	if got := diskKey(t, dir, 1); got != publicKeyOf(t, testPrivateKey) {
		t.Fatal("failed rotation replaced the config the live link still runs")
	}

	delete(h.failing, "ip link del")
	if err := m.Ensure(rotated); err != nil {
		t.Fatalf("retry: %v", err)
	}
	want := publicKeyOf(t, otherPrivateKey)
	if h.links["xui-awg1"].publicKey != want || diskKey(t, dir, 1) != want {
		t.Fatal("retried rotation did not converge link and disk on the new key")
	}
}

// TestCreateFailureLeavesNoHalfBuiltLink: a keyless leftover would hold the
// name and port with nothing listening.
func TestCreateFailureLeavesNoHalfBuiltLink(t *testing.T) {
	m, h, _ := newTestManager(t)
	h.failing["ip -4 address add"] = errInjected
	if err := m.Ensure(managerDesired(1)); err == nil {
		t.Fatal("Ensure succeeded although address setup failed")
	}
	if h.linkExists("xui-awg1") || m.HasRunning() {
		t.Fatal("failed create left a link or a managed entry behind")
	}
	delete(h.failing, "ip -4 address add")
	if err := m.Ensure(managerDesired(1)); err != nil {
		t.Fatalf("retry: %v", err)
	}
}

// TestFreshManagerAdoptsSurvivingInterface: after a crash the next process
// must take the link over without dropping its sessions.
func TestFreshManagerAdoptsSurvivingInterface(t *testing.T) {
	first, h, dir := newTestManager(t)
	if err := first.Ensure(managerDesired(1)); err != nil {
		t.Fatal(err)
	}
	h.commands = nil
	second := newManager(h, dir)
	if err := second.Ensure(managerDesired(1)); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if h.ran("ip link del") || h.ran("ip link add") {
		t.Fatalf("adoption rebuilt the link: %v", h.commands)
	}
	state, ok := second.TrafficStateFor(1)
	if !ok || !state.Adopted {
		t.Fatalf("traffic state = %+v ok=%v, want an adopted baseline pending", state, ok)
	}
	if !h.ran("nft -f -") {
		t.Fatal("adoption did not re-assert the NAT table it cannot see")
	}
}

// TestFreshManagerRebuildsKeylessLeftover: a crash between link add and
// setconf leaves a keyless link that must not block the inbound forever.
func TestFreshManagerRebuildsKeylessLeftover(t *testing.T) {
	m, h, dir := newTestManager(t)
	s, err := buildSpec(managerDesired(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeConf(dir, s.name, s.conf); err != nil {
		t.Fatal(err)
	}
	h.links["xui-awg1"] = &fakeLink{}
	if err := m.Ensure(managerDesired(1)); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if h.links["xui-awg1"].publicKey != publicKeyOf(t, testPrivateKey) {
		t.Fatal("keyless leftover was not rebuilt")
	}
}

// TestEnsureRefusesInterfacesItCannotProveAreItsOwn: a same-named link
// without the panel's config, or with another key, belongs to someone else.
func TestEnsureRefusesInterfacesItCannotProveAreItsOwn(t *testing.T) {
	foreignKey := publicKeyOf(t, otherPrivateKey)
	cases := map[string]func(t *testing.T, dir string){
		"no owned config": func(*testing.T, string) {},
		"other key": func(t *testing.T, dir string) {
			s, err := buildSpec(managerDesired(1))
			if err != nil {
				t.Fatal(err)
			}
			if err := writeConf(dir, s.name, s.conf); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, seed := range cases {
		t.Run(name, func(t *testing.T) {
			m, h, dir := newTestManager(t)
			seed(t, dir)
			before, _ := os.ReadFile(ConfPathIn(dir, "xui-awg1"))
			h.links["xui-awg1"] = &fakeLink{publicKey: foreignKey, mtu: 1380, addresses: []string{"10.8.1.1/24"}}

			if err := m.Ensure(managerDesired(1)); err == nil || !strings.Contains(err.Error(), "refusing") {
				t.Fatalf("Ensure err = %v, want a refusal", err)
			}
			if err := m.Remove(1); err == nil {
				t.Fatal("Remove tore down an interface the panel cannot prove is its own")
			}
			after, _ := os.ReadFile(ConfPathIn(dir, "xui-awg1"))
			if h.links["xui-awg1"] == nil || h.links["xui-awg1"].publicKey != foreignKey || string(before) != string(after) {
				t.Fatal("a refused interface or its config was changed")
			}
		})
	}
}

// TestEnsureRefusesNonRegularOrForeignConfig: following a symlink or a
// directory at the config path would write the private key somewhere else.
func TestEnsureRefusesNonRegularOrForeignConfig(t *testing.T) {
	cases := map[string]func(t *testing.T, path string){
		"directory": func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		},
		"unmarked file": func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("[Interface]\nPrivateKey = x\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"symlink": func(t *testing.T, path string) {
			target := filepath.Join(filepath.Dir(path), "elsewhere")
			if err := os.WriteFile(target, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		},
	}
	for name, seed := range cases {
		t.Run(name, func(t *testing.T) {
			m, h, dir := newTestManager(t)
			path := ConfPathIn(dir, "xui-awg1")
			seed(t, path)
			before, _ := os.Lstat(path)
			if err := m.Ensure(managerDesired(1)); err == nil {
				t.Fatal("Ensure accepted a config path the panel did not write")
			}
			after, err := os.Lstat(path)
			if err != nil || after.Mode() != before.Mode() || after.ModTime() != before.ModTime() || h.linkExists("xui-awg1") {
				t.Fatal("a refused config path was replaced or a link was created")
			}
		})
	}
}

// TestReconcileTearsDownRecoveredUndesiredInterface: an inbound disabled
// while the panel was down must not keep forwarding with no quota enforced.
func TestReconcileTearsDownRecoveredUndesiredInterface(t *testing.T) {
	first, h, dir := newTestManager(t)
	if err := first.Ensure(managerDesired(1)); err != nil {
		t.Fatal(err)
	}
	newManager(h, dir).Reconcile(nil)
	if h.linkExists("xui-awg1") || h.nat["xui_awg1"] != "" || len(forwardRulesFor(h, "xui-awg1")) != 0 {
		t.Fatalf("recovered interface survived: links=%v nat=%v forward=%v", h.links, h.nat, h.forward)
	}
	if _, err := os.Stat(ConfPathIn(dir, "xui-awg1")); !os.IsNotExist(err) {
		t.Fatalf("private config survived teardown: %v", err)
	}
}

// TestReconcileLeavesForeignStateAlone: only state the panel can prove it
// wrote may be removed.
func TestReconcileLeavesForeignStateAlone(t *testing.T) {
	m, h, dir := newTestManager(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	foreign := map[string]string{
		"xui-awg2.conf": "[Interface]\nPrivateKey = x\n",
		"awg0.conf":     ownershipMarker("awg0") + "\n[Interface]\nPrivateKey = " + testPrivateKey + "\n",
	}
	for name, body := range foreign {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	h.links["xui-awg4"] = &fakeLink{publicKey: publicKeyOf(t, otherPrivateKey)}
	h.forward = []string{"-A FORWARD -i eth0 -j DROP"}

	m.Reconcile(nil)
	for name := range foreign {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("foreign %s was removed: %v", name, err)
		}
	}
	if !h.linkExists("xui-awg4") || len(h.forward) != 1 {
		t.Fatal("a link or FORWARD rule the panel did not create was removed")
	}
}

// TestStopAllTearsInterfacesDown: a stopped panel enforces no quota or
// expiry, so its peers must stop forwarding, as on the embedded datapath.
func TestStopAllTearsInterfacesDown(t *testing.T) {
	m, h, _ := newTestManager(t)
	for _, id := range []int{1, 2} {
		d := managerDesired(id)
		d.Instance.ListenPort += id
		if err := m.Ensure(d); err != nil {
			t.Fatal(err)
		}
	}
	m.StopAll()
	if len(h.links) != 0 || len(h.nat) != 0 || len(h.forward) != 0 || m.HasRunning() {
		t.Fatalf("after StopAll: links=%v nat=%v forward=%v running=%v", h.links, h.nat, h.forward, m.HasRunning())
	}
}

// TestTeardownFailureKeepsInterfaceManagedForRetry: forgetting a link that
// is still up would leak its UDP port and block embedded mode from binding it.
func TestTeardownFailureKeepsInterfaceManagedForRetry(t *testing.T) {
	m, h, _ := newTestManager(t)
	if err := m.Ensure(managerDesired(1)); err != nil {
		t.Fatal(err)
	}
	h.failing["ip link del"] = errInjected
	if err := m.Remove(1); err == nil {
		t.Fatal("Remove reported success although the link is still up")
	}
	m.Reconcile(nil)
	if !m.Running(1) || !m.HasRunning() {
		t.Fatal("a link that failed to come down is no longer reported running")
	}
	delete(h.failing, "ip link del")
	m.Reconcile(nil)
	if m.Running(1) || h.linkExists("xui-awg1") {
		t.Fatal("retry did not complete the teardown")
	}
}

// TestTeardownRemovesOnlyItsOwnForwardRules: another inbound's accepts and
// the operator's own rules must survive one inbound's removal.
func TestTeardownRemovesOnlyItsOwnForwardRules(t *testing.T) {
	m, h, _ := newTestManager(t)
	h.forward = []string{"-A FORWARD -i eth0 -j DROP"}
	for _, id := range []int{1, 2} {
		d := managerDesired(id)
		d.Instance.ListenPort += id
		if err := m.Ensure(d); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Remove(1); err != nil {
		t.Fatal(err)
	}
	if len(forwardRulesFor(h, "xui-awg1")) != 0 || len(forwardRulesFor(h, "xui-awg2")) != 2 ||
		!strings.Contains(strings.Join(h.forward, "\n"), "-A FORWARD -i eth0 -j DROP") {
		t.Fatalf("FORWARD after removing inbound 1 = %v", h.forward)
	}
}

// TestEnsureWithoutIptablesStillForwards: on a pure nftables host the
// FORWARD accepts are an optimisation, not a requirement.
func TestEnsureWithoutIptablesStillForwards(t *testing.T) {
	m, h, _ := newTestManager(t)
	h.iptables = false
	if err := m.Ensure(managerDesired(1)); err != nil {
		t.Fatalf("Ensure without iptables: %v", err)
	}
	if h.ran("iptables") || h.nat["xui_awg1"] == "" {
		t.Fatalf("commands = %v nat = %v", h.commands, h.nat)
	}
	if err := m.Remove(1); err != nil {
		t.Fatalf("Remove without iptables: %v", err)
	}
}
