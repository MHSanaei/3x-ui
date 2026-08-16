package amneziawg

import (
	"crypto/rand"
	"encoding/base64"
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

// GenerateObfuscation31 produces a randomized AmneziaWG 3.1 parameter set.
// Values are randomized per call so each server gets a unique fingerprint —
// a static value gets profiled by DPI, defeating the point.
func GenerateObfuscation31() Obfuscation31 {
	var o Obfuscation31

	o.Jc = randInt(3, 6)
	o.Jmin = randInt(40, 89)
	o.Jmax = o.Jmin + randInt(50, 250)

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
	// I2-I5 stay empty, matching Amnezia's own generator.
	o.I1 = fmt.Sprintf("<r %d>", randInt(32, 256))

	o.HeaderProtectionKey = generateHeaderProtectionKey()

	// Total padding stays <= 64: it rides on full-size transport packets, the
	// same MTU-headroom concern that caps S4 at 32.
	cpLo := randInt(8, 24)
	o.ContentPaddingAddition = fmt.Sprintf("%d-%d", cpLo, cpLo+randInt(8, 40))

	// Timing windows bracket WireGuard's own constants (rekey 120s, reject
	// 180s, retry 5s, keepalive 10s) so sessions still renew before expiry.
	rkLo := randInt(100, 120)
	rkHi := rkLo + randInt(10, 40)
	o.RekeyAfterTime = fmt.Sprintf("%d-%d", rkLo, rkHi)

	// Every reject value exceeds every rekey value by >= 30s by construction.
	rjLo := rkHi + randInt(30, 60)
	o.RejectAfterTime = fmt.Sprintf("%d-%d", rjLo, rjLo+randInt(30, 90))

	rtLo := randInt(3, 6)
	o.RekeyTimeout = fmt.Sprintf("%d-%d", rtLo, rtLo+randInt(1, 4))

	// Max 20s: must stay under clients' typical 25s PersistentKeepalive and
	// common ~30s NAT UDP timeouts, or idle links lose their NAT mapping.
	kaLo := randInt(8, 12)
	o.KeepaliveTimeout = fmt.Sprintf("%d-%d", kaLo, kaLo+randInt(2, 8))

	haLo := randInt(15, 25)
	o.MaxHandshakeAttempts = fmt.Sprintf("%d-%d", haLo, haLo+randInt(5, 25))

	o.RandomTrailers = true
	// Cookie replies are a DPI-fingerprintable message type; stealth default
	// trades away WG's handshake-flood mitigation, toggleable per inbound.
	o.DisableCookies = true

	return o
}

// generateHeaderProtectionKey returns base64.StdEncoding of 32 crypto/rand
// bytes — the key format amneziawg-tools' HeaderProtectionKey parser expects.
func generateHeaderProtectionKey() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(key)
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
func ValidateObfuscation(o Obfuscation31) error {
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
		if err := validateUintRange(h, 0); err != nil {
			return fmt.Errorf("invalid H%d: %w", i+1, err)
		}
	}
	if err := validateHeaderProtectionKey(o.HeaderProtectionKey); err != nil {
		return err
	}
	if err := validateUintRange(o.ContentPaddingAddition, 0); err != nil {
		return fmt.Errorf("invalid contentPaddingAddition: %w", err)
	}
	timing := []struct{ field, v string }{
		{"rekeyAfterTime", o.RekeyAfterTime},
		{"rekeyTimeout", o.RekeyTimeout},
		{"rejectAfterTime", o.RejectAfterTime},
		{"keepaliveTimeout", o.KeepaliveTimeout},
		{"maxHandshakeAttempts", o.MaxHandshakeAttempts},
	}
	for _, tf := range timing {
		// Zero would disable the timer or retry loop outright, so min is 1.
		if err := validateUintRange(tf.v, 1); err != nil {
			return fmt.Errorf("invalid %s: %w", tf.field, err)
		}
	}
	// Sessions must renew before hard expiry: every possible rekey interval
	// has to fire before the earliest possible reject cutoff. A blank side
	// means the WireGuard default (rekey 120s, reject 180s) still applies.
	if o.RekeyAfterTime != "" || o.RejectAfterTime != "" {
		rekeyHi, rejectLo := int64(120), int64(180)
		if o.RekeyAfterTime != "" {
			_, rekeyHi, _ = parseUintRange(o.RekeyAfterTime)
		}
		if o.RejectAfterTime != "" {
			rejectLo, _, _ = parseUintRange(o.RejectAfterTime)
		}
		if rekeyHi >= rejectLo {
			return fmt.Errorf("invalid rekeyAfterTime/rejectAfterTime: max rekey %d must be below min reject %d", rekeyHi, rejectLo)
		}
	}
	return nil
}

// CanonicalizeUintRange trims a range value and removes inner spaces, so a
// pasted "110 - 140" is stored (and later emitted) as "110-140" and a
// whitespace-only value collapses back to "feature off".
func CanonicalizeUintRange(v string) string {
	return strings.ReplaceAll(strings.TrimSpace(v), " ", "")
}

// validateHeaderProtectionKey accepts an empty value (feature off) or a
// base64-encoded 32-byte key, the exact shape amneziawg-tools parses.
// Control chars are rejected up front: DecodeString silently IGNORES \r\n,
// so a line-wrapped pasted key would otherwise pass and split client configs.
func validateHeaderProtectionKey(v string) error {
	if v == "" {
		return nil
	}
	if err := ValidateConfigValue("headerProtectionKey", v); err != nil {
		return err
	}
	key, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return fmt.Errorf("invalid headerProtectionKey: not base64: %w", err)
	}
	if len(key) != 32 {
		return fmt.Errorf("invalid headerProtectionKey: got %d bytes, want 32", len(key))
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

// validateUintRange checks a uint32-range parameter (H1-H4, the 3.x padding
// and timing fields): empty, a single integer, or "low-high", with
// minAllowed <= low <= high <= uint32 max.
func validateUintRange(v string, minAllowed int64) error {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	// parseUintRange trims each half, so "110\n-140" would otherwise pass
	// validation and then split a rendered config line in two.
	if err := ValidateConfigValue("range", v); err != nil {
		return fmt.Errorf("value %q must not contain control characters", v)
	}
	lo, hi, ok := parseUintRange(v)
	if !ok {
		return fmt.Errorf("value %q must be an integer or a low-high range", v)
	}
	if lo < minAllowed || hi > hMaxValid || lo > hi {
		return fmt.Errorf("range %q must satisfy %d <= low <= high <= %d", v, minAllowed, hMaxValid)
	}
	return nil
}

// parseUintRange parses "N" (lo == hi) or "low-high"; ok is false when the
// value is empty or not numeric. Bounds are NOT checked here.
func parseUintRange(v string) (lo, hi int64, ok bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, 0, false
	}
	if loS, hiS, isRange := strings.Cut(v, "-"); isRange {
		l, err1 := strconv.ParseInt(strings.TrimSpace(loS), 10, 64)
		h, err2 := strconv.ParseInt(strings.TrimSpace(hiS), 10, 64)
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
		return l, h, true
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return n, n, true
}
