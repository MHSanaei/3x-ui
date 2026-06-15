package runtime

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func insertColons(h string) string {
	var b strings.Builder
	for i := 0; i < len(h); i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(h[i : i+2])
	}
	return b.String()
}

// TestProp_DecodeCertPin_FormatAgnostic asserts that for ANY 32-byte pin, every
// accepted encoding (hex lower/upper, openssl colon-hex, base64 std/raw/url) decodes
// back to the same bytes. Generalizes the fixed-input TestDecodeCertPin so a mutant
// that breaks one decoding path is caught across the whole input space.
func TestProp_DecodeCertPin_FormatAgnostic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.SliceOfN(rapid.Byte(), sha256.Size, sha256.Size).Draw(t, "raw")
		hx := hex.EncodeToString(raw)
		forms := []string{
			hx,
			strings.ToUpper(hx),
			insertColons(hx),
			base64.StdEncoding.EncodeToString(raw),
			base64.RawStdEncoding.EncodeToString(raw),
			base64.URLEncoding.EncodeToString(raw),
			base64.RawURLEncoding.EncodeToString(raw),
		}
		for _, f := range forms {
			got, err := DecodeCertPin(f)
			if err != nil {
				t.Fatalf("DecodeCertPin(%q) errored: %v", f, err)
			}
			if !bytes.Equal(got, raw) {
				t.Fatalf("DecodeCertPin(%q) = %x, want %x", f, got, raw)
			}
		}
	})
}

// FuzzDecodeCertPin asserts the security-load-bearing decoder never panics, never
// returns a non-32-byte slice with a nil error, and never returns bytes alongside an
// error. Seeded from the known-good/known-bad cases.
func FuzzDecodeCertPin(f *testing.F) {
	seed := sha256.Sum256([]byte("seed"))
	f.Add(hex.EncodeToString(seed[:]))
	f.Add(base64.StdEncoding.EncodeToString(seed[:]))
	f.Add(insertColons(hex.EncodeToString(seed[:])))
	f.Add("")
	f.Add("not-a-pin")
	f.Fuzz(func(t *testing.T, s string) {
		got, err := DecodeCertPin(s)
		if err == nil && len(got) != sha256.Size {
			t.Fatalf("DecodeCertPin(%q): nil error but %d bytes, want %d", s, len(got), sha256.Size)
		}
		if err != nil && got != nil {
			t.Fatalf("DecodeCertPin(%q): error %v but returned bytes %x", s, err, got)
		}
	})
}
