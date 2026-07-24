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
		if !strings.HasPrefix(o.I1, "<r ") || !strings.HasSuffix(o.I1, ">") {
			t.Fatalf("I1 = %q, want \"<r N>\" form", o.I1)
		}
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(o.I1, "<r "), ">"))
		if err != nil || n < 32 || n > 256 {
			t.Fatalf("I1 = %q, embedded N must be an integer in [32,256]", o.I1)
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
