package random

import (
	"encoding/base64"
	"testing"
)

func TestSeq_LengthAndAlphabet(t *testing.T) {
	for _, n := range []int{0, 1, 8, 64, 256} {
		s := Seq(n)
		if len(s) != n {
			t.Fatalf("Seq(%d) returned length %d", n, len(s))
		}
		for i, r := range s {
			isDigit := r >= '0' && r <= '9'
			isLower := r >= 'a' && r <= 'z'
			isUpper := r >= 'A' && r <= 'Z'
			if !(isDigit || isLower || isUpper) {
				t.Fatalf("Seq(%d) byte %d = %q is not alphanumeric", n, i, r)
			}
		}
	}
}

func TestSeq_NotConstant(t *testing.T) {
	a := Seq(32)
	b := Seq(32)
	if a == b {
		t.Fatalf("two consecutive Seq(32) calls produced identical output: %q", a)
	}
}

func TestNum_InRange(t *testing.T) {
	for _, upper := range []int{1, 2, 10, 1000} {
		for range 200 {
			v := Num(upper)
			if v < 0 || v >= upper {
				t.Fatalf("Num(%d) returned %d, out of [0, %d)", upper, v, upper)
			}
		}
	}
}

func TestBase64Bytes_DecodesToRequestedSize(t *testing.T) {
	for _, n := range []int{1, 16, 32, 64} {
		out := Base64Bytes(n)
		decoded, err := base64.StdEncoding.DecodeString(out)
		if err != nil {
			t.Fatalf("Base64Bytes(%d) produced invalid base64 %q: %v", n, out, err)
		}
		if len(decoded) != n {
			t.Fatalf("Base64Bytes(%d) decoded to %d bytes", n, len(decoded))
		}
	}
}

func TestBase64Bytes_Random(t *testing.T) {
	a := Base64Bytes(32)
	b := Base64Bytes(32)
	if a == b {
		t.Fatalf("two consecutive Base64Bytes(32) calls produced identical output: %q", a)
	}
}
