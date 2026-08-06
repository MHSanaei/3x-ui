package amneziawg

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

// awgHMax is the upper bound for generated H values: 2^31-1. The AmneziaWG
// spec allows the full uint32, but the amneziawg-windows-client config editor
// rejects values above 2^31-1, so generation stays in the safe half for
// cross-client compatibility.
const awgHMax = 2147483647

// hMinWidth is the minimum width of each generated H1-H4 range.
const hMinWidth = 1000

// hMaxValid is the largest value ValidateObfuscation accepts for an H
// parameter: uint32 max, the kernel's own limit.
const hMaxValid int64 = 4294967295

// randInt returns a uniform random int in [min, max] using crypto/rand. Falls
// back to min on the (practically impossible) RNG error.
func randInt(min, max int) int {
	if max <= min {
		return min
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min)+1))
	if err != nil {
		return min
	}
	return min + int(n.Int64())
}

// GenerateObfuscation20 produces a randomized AmneziaWG 2.0 parameter set.
// preset "mobile" tunes junk packets for restrictive mobile carriers; any
// other value uses the balanced "default" preset. Values are randomized per
// call so each server gets a unique fingerprint — a static value gets
// profiled by DPI, defeating the point of the obfuscation.
func GenerateObfuscation20(preset string) Obfuscation20 {
	var o Obfuscation20

	switch preset {
	case "mobile":
		// Jc=3 and a narrow Jmax survive carriers like Tele2/Yota/Megafon.
		o.Jc = 3
		o.Jmin = randInt(30, 50)
		o.Jmax = o.Jmin + randInt(20, 80)
	default:
		o.Jc = randInt(3, 6)
		o.Jmin = randInt(40, 89)
		o.Jmax = o.Jmin + randInt(50, 250)
	}

	o.S1 = randInt(15, 150)
	o.S2 = randInt(15, 150)
	// Kernel constraint: S1+56 must not equal S2 (else init and response
	// handshake packets end up the same size after padding).
	for o.S1+56 == o.S2 {
		o.S2 = randInt(15, 150)
	}
	o.S3 = randInt(8, 55) // cookie padding (max 64)
	o.S4 = randInt(4, 27) // transport padding (max 32)

	h := generateHRanges()
	o.H1, o.H2, o.H3, o.H4 = h[0], h[1], h[2], h[3]

	// CPS signature packet: N random bytes prepended before each handshake.
	o.I1 = fmt.Sprintf("<r %d>", randInt(32, 256))

	return o
}

// generateHRanges returns four non-overlapping "low-high" ranges for H1-H4.
// Each is at least hMinWidth wide, the lowest bound is >= 5 (values 1-4 are
// reserved for vanilla WireGuard message types) and the highest is <=
// 2^31-1. The space is split into four bands and a random sub-range is taken
// from each, which guarantees non-overlap (with a gap) and a valid width
// without retries.
func generateHRanges() [4]string {
	const lo = 5
	bandSize := (awgHMax - lo + 1) / 4
	var out [4]string
	for i := 0; i < 4; i++ {
		bandLo := lo + i*bandSize
		bandHi := bandLo + bandSize - 1
		start := randInt(bandLo, bandHi-hMinWidth-1)
		end := randInt(start+hMinWidth, bandHi-1)
		out[i] = fmt.Sprintf("%d-%d", start, end)
	}
	return out
}

// ValidateObfuscation rejects malformed obfuscation parameters before they
// are saved and applied, so a bad manual entry can't bring the interface
// down on `awg-quick up`. Empty H values are allowed (they fall back to a
// default when the config is generated). Each H value accepts either a
// single integer ("1") or a range ("100-800").
func ValidateObfuscation(o Obfuscation20) error {
	if o.Jmin > o.Jmax {
		return fmt.Errorf("invalid Jmin/Jmax: %d must not exceed %d", o.Jmin, o.Jmax)
	}
	if o.S3 < 0 || o.S3 > 64 {
		return fmt.Errorf("invalid S3 value %d (must be 0..64)", o.S3)
	}
	if o.S4 < 0 || o.S4 > 32 {
		return fmt.Errorf("invalid S4 value %d (must be 0..32)", o.S4)
	}
	if o.S1+56 == o.S2 {
		return fmt.Errorf("invalid S1/S2: S1+56 must not equal S2 (%d+56 == %d)", o.S1, o.S2)
	}
	for i, h := range []string{o.H1, o.H2, o.H3, o.H4} {
		if err := validateHValue(h); err != nil {
			return fmt.Errorf("invalid H%d: %w", i+1, err)
		}
	}
	return nil
}

// ValidateIPv6Subnet rejects a malformed IPv6 subnet before it's saved, so a
// bad manual entry can't bring the interface down on `awg-quick up`. A blank
// subnet is only valid when IPv6 itself is disabled.
func ValidateIPv6Subnet(enabled bool, subnet string) error {
	if !enabled {
		return nil
	}
	if strings.TrimSpace(subnet) == "" {
		return fmt.Errorf("ipv6Subnet is required when IPv6 is enabled")
	}
	prefix, err := netip.ParsePrefix(subnet)
	if err != nil {
		return fmt.Errorf("invalid ipv6Subnet %q: %w", subnet, err)
	}
	if !prefix.Addr().Is6() {
		return fmt.Errorf("invalid ipv6Subnet %q: not an IPv6 prefix", subnet)
	}
	return nil
}

// interfaceNamePattern matches a plausible Linux network interface name:
// letters, digits, and the handful of separators seen in real device names
// (eth0, wg0, br-lan, eno1.100, eth0:0), capped at 15 bytes (IFNAMSIZ-1).
var interfaceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.@:-]{1,15}$`)

// ValidateInterfaceName rejects a value that isn't a plausible network
// interface name before it's saved. ExternalInterface and
// IPv6ExternalInterface are interpolated unescaped into a shell-executed
// PostUp/PostDown line by generateServerConfig, so — unlike the client email
// (already hashed for exactly this reason, see routeEgressComment) — an
// unvalidated value here could carry a shell metacharacter straight into a
// root-executed command. A blank value is allowed: it means "auto-detect"
// for ExternalInterface, or "reuse ExternalInterface" for
// IPv6ExternalInterface.
func ValidateInterfaceName(name string) error {
	if name == "" {
		return nil
	}
	if !interfaceNamePattern.MatchString(name) {
		return fmt.Errorf("invalid interface name %q: must be 1-15 characters of letters, digits, '.', '_', '@', ':' or '-'", name)
	}
	return nil
}

// ValidateSubnetIPv4 rejects a malformed IPv4 tunnel subnet before it's
// saved: subnetIP is interpolated into the PostUp/PostDown MASQUERADE rule
// the same way ExternalInterface is (see ValidateInterfaceName), so it needs
// the same protection against a value that isn't really an address at all.
// subnetCIDR <= 0 is treated as unset, mirroring serverAddress's own
// default-to-/24 leniency.
func ValidateSubnetIPv4(subnetIP string, subnetCIDR int) error {
	cidr := subnetCIDR
	if cidr <= 0 {
		cidr = 24
	}
	if cidr > 32 {
		return fmt.Errorf("invalid subnetCidr %d: must be 0..32", subnetCIDR)
	}
	prefix, err := netip.ParsePrefix(fmt.Sprintf("%s/%d", subnetIP, cidr))
	if err != nil {
		return fmt.Errorf("invalid subnetIp %q: %w", subnetIP, err)
	}
	if !prefix.Addr().Is4() {
		return fmt.Errorf("invalid subnetIp %q: not an IPv4 address", subnetIP)
	}
	return nil
}

// ValidateConfigValue rejects a value containing a newline, carriage return,
// or other control character before it's saved. PrivateKey/PublicKey/I1 on
// the server block, and Email/PublicKey/PreSharedKey per client, all get
// interpolated verbatim into the generated .conf by generateServerConfig; a
// newline in any of them lets a later line re-open a new [Interface]/[Peer]
// section, and awg-quick's parser collects a following "PostUp = ..." line
// into a hook it executes as root on the next apply — the same class this
// package already closes for ExternalInterface/IPv6ExternalInterface (see
// ValidateInterfaceName) and SubnetIP (see ValidateSubnetIPv4). field names
// the value in the returned error, e.g. "email" or "publicKey".
func ValidateConfigValue(field, v string) error {
	for _, r := range v {
		if r == '\n' || r == '\r' || r < 0x20 || r == 0x7f {
			return fmt.Errorf("invalid %s: control characters are not allowed", field)
		}
	}
	return nil
}

// validateHValue checks one H parameter: empty, a single uint32, or
// "low-high" with 0 <= low <= high <= uint32 max.
func validateHValue(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if lo, hi, isRange := strings.Cut(v, "-"); isRange {
		l, err1 := strconv.ParseInt(strings.TrimSpace(lo), 10, 64)
		h, err2 := strconv.ParseInt(strings.TrimSpace(hi), 10, 64)
		if err1 != nil || err2 != nil {
			return fmt.Errorf("range %q must be two integers", v)
		}
		if l < 0 || h > hMaxValid || l > h {
			return fmt.Errorf("range %q must satisfy 0 <= low <= high <= %d", v, hMaxValid)
		}
		return nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 || n > hMaxValid {
		return fmt.Errorf("value %q must be an integer in 0..%d or a low-high range", v, hMaxValid)
	}
	return nil
}
