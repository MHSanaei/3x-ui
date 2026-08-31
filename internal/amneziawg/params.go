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

// awgHMax caps generated H values at 2^31-1: the spec allows the full uint32,
// but the amneziawg-windows-client config editor rejects anything above.
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

// DefaultMTU is WireGuard/AmneziaWG's usual tunnel MTU on a 1500-byte host
// link, before AmneziaWG's own S4 transport junk is prepended.
const DefaultMTU = 1420

// EffectiveMTU is the MTU a tunnel actually runs at: the admin's value when
// set, otherwise DefaultMTU minus S4.
//
// amneziawg prepends s4 random bytes to every transport packet and, unlike
// content padding and trailers, never clamps them against the tunnel MTU, so a
// plain 1420 tunnel puts full-size packets at 1480+S4 bytes on the wire and
// fragments every one of them once S4 passes 20.
//
// Both ends must agree, so every client-config emitter writes this same number:
// a config with no MTU line leaves the client on its own 1420 default and
// fragments the client-to-server direction even after the server is fixed. The
// 1280 floor is defensive -- validation caps S4 at 32, so it cannot be reached
// through the panel.
func EffectiveMTU(configuredMTU, s4 int) int {
	if configuredMTU > 0 {
		return configuredMTU
	}
	return max(DefaultMTU-max(s4, 0), 1280)
}

// GenerateObfuscation31 produces a randomized AmneziaWG 3.1 parameter set: a
// static value gets profiled by DPI, defeating the point.
func GenerateObfuscation31() Obfuscation31 {
	var o Obfuscation31

	o.Jc = randInt(3, 6)
	o.Jmin = randInt(40, 89)
	o.Jmax = o.Jmin + randInt(50, 250)

	o.S1 = randInt(15, 150)
	o.S2 = randInt(15, 150)
	// Kernel constraint: S1+56 != S2, else init and response handshake
	// packets end up the same size after padding.
	for o.S1+56 == o.S2 {
		o.S2 = randInt(15, 150)
	}
	// Floored at 12: HeaderProtectionKey is always generated below, and IpcSet
	// rejects header protection unless every S1-S4 is >= 12.
	o.S3 = randInt(12, 55) // cookie padding (max 64)
	o.S4 = randInt(12, 27) // transport padding (max 32)

	h := generateHRanges()
	o.H1, o.H2, o.H3, o.H4 = h[0], h[1], h[2], h[3]

	// CPS signature packet, N random bytes before each handshake. I2-I5 stay
	// empty, matching Amnezia's own generator.
	o.I1 = fmt.Sprintf("<r %d>", randInt(32, 256))

	o.HeaderProtectionKey = generateHeaderProtectionKey()

	// Total padding stays <= 64: it rides on full-size transport packets, the
	// same MTU headroom that caps S4 at 32.
	cpLo := randInt(8, 24)
	o.ContentPaddingAddition = fmt.Sprintf("%d-%d", cpLo, cpLo+randInt(8, 40))

	// Timing windows bracket WireGuard's own constants (rekey 120s, reject
	// 180s) so sessions still renew before expiry.
	rkLo := randInt(100, 120)
	rkHi := rkLo + randInt(10, 40)
	o.RekeyAfterTime = fmt.Sprintf("%d-%d", rkLo, rkHi)

	// Every reject value exceeds every rekey value by >= 30s by construction.
	rjLo := rkHi + randInt(30, 60)
	o.RejectAfterTime = fmt.Sprintf("%d-%d", rjLo, rjLo+randInt(30, 90))

	rtLo := randInt(3, 6)
	o.RekeyTimeout = fmt.Sprintf("%d-%d", rtLo, rtLo+randInt(1, 4))

	// Max 20s: under clients' typical 25s PersistentKeepalive and ~30s NAT UDP
	// timeouts, or idle links lose their NAT mapping.
	kaLo := randInt(8, 12)
	o.KeepaliveTimeout = fmt.Sprintf("%d-%d", kaLo, kaLo+randInt(2, 8))

	haLo := randInt(15, 25)
	o.MaxHandshakeAttempts = fmt.Sprintf("%d-%d", haLo, haLo+randInt(5, 25))

	o.RandomTrailers = true
	// Cookie replies are DPI-fingerprintable; this stealth default trades away
	// WG's handshake-flood mitigation and is toggleable per inbound.
	o.DisableCookies = true

	return o
}

// generateHeaderProtectionKey returns base64 of 32 crypto/rand bytes, the
// format amneziawg-tools' HeaderProtectionKey parser expects.
func generateHeaderProtectionKey() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(key)
}

// generateHRanges returns four non-overlapping "low-high" ranges for H1-H4,
// one per band of the space so non-overlap needs no retries. The low bound is
// >= 5: values 1-4 are reserved for vanilla WireGuard message types.
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

// ValidateObfuscation rejects malformed parameters before they are saved, so
// a bad manual entry can't break the embedded amneziawg-go device's own
// UAPI config apply (internal/amneziawgnet's buildUAPIConfig/IpcSet) or
// produce a client config the official app rejects outright. Blank H values
// are allowed (they fall back to a default); each accepts an integer or a
// "100-800" range.
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
	if o.HeaderProtectionKey != "" {
		for i, s := range []int{o.S1, o.S2, o.S3, o.S4} {
			if s < 12 {
				return fmt.Errorf("invalid S%d value %d: header protection requires S1-S4 >= 12", i+1, s)
			}
		}
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
	// Sessions must renew before hard expiry, so every possible rekey fires
	// before the earliest reject. A blank side means WireGuard's own default.
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

// CanonicalizeUintRange stores a pasted "110 - 140" as "110-140", and
// collapses a whitespace-only value back to "feature off".
func CanonicalizeUintRange(v string) string {
	return strings.ReplaceAll(strings.TrimSpace(v), " ", "")
}

// validateHeaderProtectionKey accepts blank (feature off) or a base64 32-byte
// key. Control chars are rejected up front: DecodeString silently ignores
// \r\n, so a line-wrapped pasted key would pass and then split client configs.
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

// ValidateIPv6Subnet rejects a malformed subnet before it's saved. A blank
// value is only valid when IPv6 itself is disabled.
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

// interfaceNamePattern matches a plausible Linux device name (eth0, br-lan,
// eno1.100, eth0:0), capped at 15 bytes (IFNAMSIZ-1).
var interfaceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.@:-]{1,15}$`)

// ValidateInterfaceName guards the NIC names generateServerConfig interpolates
// unescaped into a root-executed PostUp/PostDown line. Blank is allowed and
// means auto-detect (or, for IPv6ExternalInterface, reuse the IPv4 one).
func ValidateInterfaceName(name string) error {
	if name == "" {
		return nil
	}
	if !interfaceNamePattern.MatchString(name) {
		return fmt.Errorf("invalid interface name %q: must be 1-15 characters of letters, digits, '.', '_', '@', ':' or '-'", name)
	}
	return nil
}

// ValidateSubnetIPv4 guards subnetIP, which lands in the MASQUERADE rule the
// same way ExternalInterface does. subnetCIDR <= 0 means unset, mirroring
// serverAddress's own default-to-/24 leniency.
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

// ValidateConfigValue rejects control characters in any value interpolated
// verbatim into a rendered .conf: a newline re-opens an [Interface] section
// whose "PostUp = ..." runs as root the moment whoever downloaded that
// config -- the client app, or an admin importing it into the official
// awg-quick CLI directly -- applies it. The panel's own server side never
// runs awg-quick itself (internal/amneziawgnet applies config via
// amneziawg-go's UAPI, not a parsed text file), but this exact value still
// reaches a real text-based config downstream. field names the value.
func ValidateConfigValue(field, v string) error {
	for _, r := range v {
		if r == '\n' || r == '\r' || r < 0x20 || r == 0x7f {
			return fmt.Errorf("invalid %s: control characters are not allowed", field)
		}
	}
	return nil
}

// validateUintRange checks a uint32-range parameter (H1-H4, the 3.x padding
// and timing fields): blank, an integer, or "low-high" within the bounds.
func validateUintRange(v string, minAllowed int64) error {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	// parseUintRange trims each half, so "110\n-140" would otherwise pass and
	// then split a rendered config line in two.
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

// parseUintRange parses "N" (lo == hi) or "low-high"; ok is false when blank
// or non-numeric. Bounds are NOT checked here.
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
