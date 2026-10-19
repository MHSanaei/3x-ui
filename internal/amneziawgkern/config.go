// Package amneziawgkern drives an AmneziaWG inbound through the DKMS kernel module and awg-quick
// instead of internal/amneziawgnet's embedded amneziawg-go + gVisor stack.
package amneziawgkern

import (
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

// ConfDir is where awg-quick looks up an interface's config by name, so
// InterfaceName + ".conf" here is what `awg-quick up <name>` resolves to.
const ConfDir = "/etc/amnezia/amneziawg"

// fallbackInterface is the last resort when no usable host NIC is detected;
// the overwhelmingly common name on the VPS images this runs on.
const fallbackInterface = "eth0"

// skippedInterfacePrefixes are never NAT egress candidates: our own tunnels,
// and the virtual bridges container runtimes leave lying around.
var skippedInterfacePrefixes = []string{"awg", "wg", "docker", "br-", "veth"}

// ConfPath returns the awg-quick config path for one inbound's interface.
func ConfPath(interfaceName string) string {
	return ConfPathIn(ConfDir, interfaceName)
}

// ConfPathIn is ConfPath against an explicit directory, so the write path can
// be exercised without touching /etc.
func ConfPathIn(dir, interfaceName string) string {
	return dir + "/" + interfaceName + ".conf"
}

// effectiveMTU is the tunnel MTU this instance runs at: the admin's value when
// set, otherwise the S4-aware default shared with the embedded datapath.
func effectiveMTU(inst amneziawg.Instance) int {
	return amneziawg.EffectiveMTU(inst.MTU, inst.Obfuscation.S4)
}

// natTableName keeps each interface's masquerade rule in its own nftables
// table, so bringing one inbound down cannot delete another one's NAT.
func natTableName(inst amneziawg.Instance) string {
	return fmt.Sprintf("awgnat%d", inst.Id)
}

// Render builds the awg-quick config for inst.
func Render(inst amneziawg.Instance, egressInterface string, routeThroughXray bool) (string, error) {
	if listenNeedsPinnedBind(inst.Listen) {
		return "", fmt.Errorf("amneziawgkern: kernel mode cannot bind to listen address %q; clear it or use a wildcard", inst.Listen)
	}
	if inst.PrivateKey == "" {
		return "", fmt.Errorf("amneziawgkern: inbound %d has no private key", inst.Id)
	}
	if err := validateConfigValues(inst); err != nil {
		return "", err
	}
	if len(inst.Address) == 0 {
		return "", fmt.Errorf("amneziawgkern: inbound %d has no interface address", inst.Id)
	}

	var b strings.Builder
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", inst.PrivateKey)
	fmt.Fprintf(&b, "Address = %s\n", strings.Join(inst.Address, ", "))
	fmt.Fprintf(&b, "ListenPort = %d\n", inst.ListenPort)
	// Always emit an MTU. Leaving it out lets the netdev keep its own default,
	// which knows nothing about S4 and so fragments every full-size packet.
	fmt.Fprintf(&b, "MTU = %d\n", effectiveMTU(inst))
	writeObfuscation(&b, inst.Obfuscation)

	if !routeThroughXray {
		rules, err := natRules(inst, egressInterface)
		if err != nil {
			return "", err
		}
		b.WriteString(rules)
	}

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
		{"listen", inst.Listen},
		{"externalInterface", inst.ExternalInterface},
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
	for i, value := range inst.Address {
		values = append(values, struct{ field, value string }{fmt.Sprintf("address[%d]", i), value})
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

// natRules builds the PostUp/PostDown block.
func natRules(inst amneziawg.Instance, egressInterface string) (string, error) {
	subnets, err := v4Subnets(inst.Address)
	if err != nil {
		return "", err
	}
	if len(subnets) == 0 {
		return "", fmt.Errorf("amneziawgkern: inbound %d has no IPv4 address to NAT", inst.Id)
	}
	if egressInterface == "" {
		egressInterface = DetectDefaultInterface()
	}
	if err := amneziawg.ValidateInterfaceName(egressInterface); err != nil {
		return "", fmt.Errorf("amneziawgkern: %w", err)
	}

	table := natTableName(inst)
	var b strings.Builder
	fmt.Fprintf(&b, "PostUp = sysctl -q -w net.ipv4.ip_forward=1\n")
	fmt.Fprintf(&b, "PostUp = nft add table ip %s\n", table)
	fmt.Fprintf(&b, "PostUp = nft add chain ip %s post '{ type nat hook postrouting priority 100 ; }'\n", table)
	for _, s := range subnets {
		fmt.Fprintf(&b, "PostUp = nft add rule ip %s post ip saddr %s oifname \"%s\" masquerade\n",
			table, s, egressInterface)
	}
	fmt.Fprintf(&b, "PostDown = nft delete table ip %s\n", table)
	return b.String(), nil
}

// v4Subnets turns the interface's own addresses ("10.8.1.1/24") into the network each one covers
// ("10.8.1.0/24"), which is what masquerade matches on.
func v4Subnets(addresses []string) ([]string, error) {
	var out []string
	for _, a := range addresses {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		p, err := netip.ParsePrefix(a)
		if err != nil {
			return nil, fmt.Errorf("amneziawgkern: parse interface address %q: %w", a, err)
		}
		if !p.Addr().Is4() {
			continue
		}
		out = append(out, p.Masked().String())
	}
	return out, nil
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
