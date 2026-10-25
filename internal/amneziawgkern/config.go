// Package amneziawgkern runs an AmneziaWG inbound on the Linux kernel module instead of
// internal/amneziawgnet's embedded amneziawg-go + gVisor stack; nothing it writes is executed.
package amneziawgkern

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

// interfacePrefix keeps panel interfaces out of the awg0/awg1 names operators
// give their own awg-quick tunnels.
const interfacePrefix = "xui-awg"

// fallbackInterface is the last resort when no usable host NIC is detected;
// the overwhelmingly common name on the VPS images this runs on.
const fallbackInterface = "eth0"

// skippedInterfacePrefixes are never NAT egress candidates: tunnels, and the
// virtual bridges container runtimes leave lying around.
var skippedInterfacePrefixes = []string{interfacePrefix, "awg", "wg", "docker", "br-", "veth"}

// defaultConfDir is panel-private, so no operator config can ever sit at a
// path the panel writes.
func defaultConfDir() string {
	return filepath.Join(config.GetDBFolderPath(), "amneziawg")
}

// ConfPathIn returns where one interface's setconf input is kept.
func ConfPathIn(dir, interfaceName string) string {
	return filepath.Join(dir, interfaceName+".conf")
}

func ownedInterfaceName(id int) string {
	return fmt.Sprintf("%s%d", interfacePrefix, id)
}

func natTableName(id int) string {
	return fmt.Sprintf("xui_awg%d", id)
}

// spec is everything one inbound's kernel interface is built from.
type spec struct {
	name string
	// conf is the `awg setconf` input: keys, port, obfuscation and peers.
	conf      string
	addresses []string
	mtu       int
	// structFP covers what syncconf cannot change on a live link.
	structFP string
	// nat is the nft transaction that replaces this inbound's NAT table.
	nat     string
	forward forwardRules
}

// forwardRules lets tunnel traffic through a FORWARD chain whose policy is
// DROP (Docker and ufw both set one); zero value means no NAT egress.
type forwardRules struct {
	tunnel, egress string
}

func buildSpec(d Desired) (spec, error) {
	inst := d.Instance
	if inst.Id <= 0 {
		return spec{}, fmt.Errorf("amneziawgkern: invalid inbound id %d", inst.Id)
	}
	name := ownedInterfaceName(inst.Id)
	if err := amneziawg.ValidateInterfaceName(name); err != nil {
		return spec{}, fmt.Errorf("amneziawgkern: inbound %d: %w", inst.Id, err)
	}
	if listenNeedsPinnedBind(inst.Listen) {
		return spec{}, fmt.Errorf("amneziawgkern: kernel mode cannot bind to listen address %q; clear it or use a wildcard", inst.Listen)
	}
	conf, err := Render(inst)
	if err != nil {
		return spec{}, err
	}
	addresses, err := interfaceAddresses(inst)
	if err != nil {
		return spec{}, err
	}
	s := spec{name: name, conf: conf, addresses: addresses, mtu: effectiveMTU(inst)}
	s.structFP = structuralFingerprint(conf, addresses, s.mtu)
	s.nat = natTransaction(inst.Id, "")
	if !d.RouteThroughXray {
		egress := d.EgressInterface
		if egress == "" {
			egress = DetectDefaultInterface()
		}
		if err := amneziawg.ValidateInterfaceName(egress); err != nil {
			return spec{}, fmt.Errorf("amneziawgkern: %w", err)
		}
		s.nat = natTransaction(inst.Id, natChain(addresses, egress))
		s.forward = forwardRules{tunnel: name, egress: egress}
	}
	return s, nil
}

// effectiveMTU is the tunnel MTU this instance runs at: the admin's value when
// set, otherwise the S4-aware default shared with the embedded datapath.
func effectiveMTU(inst amneziawg.Instance) int {
	return amneziawg.EffectiveMTU(inst.MTU, inst.Obfuscation.S4)
}

// Render builds the `awg setconf` input for inst.
func Render(inst amneziawg.Instance) (string, error) {
	if inst.PrivateKey == "" {
		return "", fmt.Errorf("amneziawgkern: inbound %d has no private key", inst.Id)
	}
	if err := validateConfigValues(inst); err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(ownershipMarker(ownedInterfaceName(inst.Id)) + "\n[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", inst.PrivateKey)
	fmt.Fprintf(&b, "ListenPort = %d\n", inst.ListenPort)
	writeObfuscation(&b, inst.Obfuscation)

	for _, p := range inst.Peers {
		if p.PublicKey == "" || len(p.AllowedIPs) == 0 {
			continue
		}
		b.WriteString("\n[Peer]\n")
		if p.Email != "" {
			fmt.Fprintf(&b, "# %s\n", p.Email)
		}
		fmt.Fprintf(&b, "PublicKey = %s\n", p.PublicKey)
		if p.PresharedKey != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", p.PresharedKey)
		}
		fmt.Fprintf(&b, "AllowedIPs = %s\n", strings.Join(p.AllowedIPs, ", "))
	}
	return b.String(), nil
}

func validateConfigValues(inst amneziawg.Instance) error {
	values := []struct{ field, value string }{
		{"privateKey", inst.PrivateKey},
		{"headerProtectionKey", inst.Obfuscation.HeaderProtectionKey},
		{"contentPaddingAddition", inst.Obfuscation.ContentPaddingAddition},
		{"rekeyAfterTime", inst.Obfuscation.RekeyAfterTime},
		{"rekeyTimeout", inst.Obfuscation.RekeyTimeout},
		{"rejectAfterTime", inst.Obfuscation.RejectAfterTime},
		{"keepaliveTimeout", inst.Obfuscation.KeepaliveTimeout},
		{"maxHandshakeAttempts", inst.Obfuscation.MaxHandshakeAttempts},
	}
	for i, value := range []string{inst.Obfuscation.H1, inst.Obfuscation.H2, inst.Obfuscation.H3, inst.Obfuscation.H4} {
		values = append(values, struct{ field, value string }{fmt.Sprintf("H%d", i+1), value})
	}
	for i, value := range []string{inst.Obfuscation.I1, inst.Obfuscation.I2, inst.Obfuscation.I3, inst.Obfuscation.I4, inst.Obfuscation.I5} {
		values = append(values, struct{ field, value string }{fmt.Sprintf("I%d", i+1), value})
	}
	for _, peer := range inst.Peers {
		values = append(values,
			struct{ field, value string }{"email", peer.Email},
			struct{ field, value string }{"publicKey", peer.PublicKey},
			struct{ field, value string }{"presharedKey", peer.PresharedKey},
		)
		for i, allowedIP := range peer.AllowedIPs {
			values = append(values, struct{ field, value string }{fmt.Sprintf("allowedIPs[%d]", i), allowedIP})
		}
	}
	for _, value := range values {
		if err := amneziawg.ValidateConfigValue(value.field, value.value); err != nil {
			return fmt.Errorf("amneziawgkern: %w", err)
		}
	}
	return nil
}

func listenNeedsPinnedBind(listen string) bool {
	switch strings.TrimSpace(listen) {
	case "", "0.0.0.0", "::", "::0":
		return false
	default:
		return true
	}
}

// writeObfuscation emits the AmneziaWG parameter block.
func writeObfuscation(b *strings.Builder, o amneziawg.Obfuscation31) {
	fmt.Fprintf(b, "Jc = %d\n", o.Jc)
	fmt.Fprintf(b, "Jmin = %d\n", o.Jmin)
	fmt.Fprintf(b, "Jmax = %d\n", o.Jmax)
	fmt.Fprintf(b, "S1 = %d\n", o.S1)
	fmt.Fprintf(b, "S2 = %d\n", o.S2)
	if o.S3 > 0 {
		fmt.Fprintf(b, "S3 = %d\n", o.S3)
	}
	if o.S4 > 0 {
		fmt.Fprintf(b, "S4 = %d\n", o.S4)
	}
	for i, v := range []string{o.H1, o.H2, o.H3, o.H4} {
		fmt.Fprintf(b, "H%d = %s\n", i+1, headerOrDefault(v, i+1))
	}
	for i, v := range []string{o.I1, o.I2, o.I3, o.I4, o.I5} {
		if v != "" {
			fmt.Fprintf(b, "I%d = %s\n", i+1, v)
		}
	}
	for _, p := range []struct{ key, v string }{
		{"HeaderProtectionKey", o.HeaderProtectionKey},
		{"ContentPaddingAddition", o.ContentPaddingAddition},
		{"RekeyAfterTime", o.RekeyAfterTime},
		{"RekeyTimeout", o.RekeyTimeout},
		{"RejectAfterTime", o.RejectAfterTime},
		{"KeepaliveTimeout", o.KeepaliveTimeout},
		{"MaxHandshakeAttempts", o.MaxHandshakeAttempts},
	} {
		if v := strings.TrimSpace(p.v); v != "" {
			fmt.Fprintf(b, "%s = %s\n", p.key, v)
		}
	}
	if o.RandomTrailers {
		b.WriteString("RandomTrailers = on\n")
	}
	if o.DisableCookies {
		b.WriteString("DisableCookies = on\n")
	}
}

func headerOrDefault(v string, n int) string {
	if s := strings.TrimSpace(v); s != "" {
		return s
	}
	return fmt.Sprint(n)
}

// interfaceAddresses parses the tunnel's own addresses into canonical prefixes.
func interfaceAddresses(inst amneziawg.Instance) ([]string, error) {
	var out []string
	for _, a := range inst.Address {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		p, err := netip.ParsePrefix(a)
		if err != nil {
			return nil, fmt.Errorf("amneziawgkern: parse interface address %q: %w", a, err)
		}
		if !p.Addr().Is4() {
			return nil, fmt.Errorf("amneziawgkern: inbound %d: kernel mode is IPv4-only, got %q", inst.Id, a)
		}
		out = append(out, p.String())
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("amneziawgkern: inbound %d has no interface address", inst.Id)
	}
	return out, nil
}

// natChain masquerades each tunnel subnet (not the host address) out of egress.
func natChain(addresses []string, egress string) string {
	var b strings.Builder
	b.WriteString("\tchain postrouting {\n\t\ttype nat hook postrouting priority 100; policy accept;\n")
	for _, a := range addresses {
		fmt.Fprintf(&b, "\t\tip saddr %s oifname %q masquerade\n", netip.MustParsePrefix(a).Masked(), egress)
	}
	b.WriteString("\t}\n")
	return b.String()
}

// natTransaction replaces the inbound's table atomically; an empty chain only
// deletes it, and "add" first makes the delete safe when the table is absent.
func natTransaction(id int, chain string) string {
	table := natTableName(id)
	tx := fmt.Sprintf("add table ip %s\ndelete table ip %s\n", table, table)
	if chain != "" {
		tx += fmt.Sprintf("table ip %s {\n%s}\n", table, chain)
	}
	return tx
}

// structuralFingerprint hashes everything above the first [Peer] section plus
// the link's addresses and MTU -- what a live syncconf cannot change.
func structuralFingerprint(conf string, addresses []string, mtu int) string {
	head := conf
	if i := strings.Index(conf, "\n[Peer]"); i >= 0 {
		head = conf[:i]
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\naddress=%s\nmtu=%d", head, strings.Join(addresses, ","), mtu)))
	return hex.EncodeToString(sum[:])
}

// DetectDefaultInterface picks the host NIC to masquerade out of when the admin left ExternalInterface
// blank: the first up, non-loopback, non-virtual interface holding a routable IPv4 address.
func DetectDefaultInterface() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return fallbackInterface
	}
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagLoopback != 0 || ifi.Flags&net.FlagUp == 0 {
			continue
		}
		if hasSkippedPrefix(ifi.Name) {
			continue
		}
		if interfaceHasRoutableV4(ifi) {
			return ifi.Name
		}
	}
	return fallbackInterface
}

func hasSkippedPrefix(name string) bool {
	for _, p := range skippedInterfacePrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func interfaceHasRoutableV4(ifi net.Interface) bool {
	addrs, err := ifi.Addrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip, ok := netip.AddrFromSlice(n.IP)
		if !ok {
			continue
		}
		ip = ip.Unmap()
		if ip.Is4() && !ip.IsLinkLocalUnicast() && !ip.IsLoopback() {
			return true
		}
	}
	return false
}
