package frontproxy

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"html/template"
	"math/rand"
)

// Variant is the per-install appearance of a decoy page.
//
// Without it every install of this panel would serve byte-identical decoys,
// so anyone who suspected the panel could hash the page and confirm it. That
// would make the decoy an identifier rather than a disguise. Deriving the look
// from an install-local seed means no two installs render the same bytes.
//
// It is deliberately derived, not random per request: a static-looking site
// whose markup changes on every reload is its own kind of strange.
// The colour and font fields are template.CSS because they are rendered into
// a style block. They are safe by construction -- built here from integers and
// a fixed font list, never from anything a request carries.
type Variant struct {
	Hue     int
	Accent  template.CSS
	Ink     template.CSS
	Muted   template.CSS
	Surface template.CSS
	Panel   template.CSS
	Font    template.CSS
	Radius  int
	Pad     int
	Wide    int
}

var variantFonts = []string{
	`-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif`,
	`"Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif`,
	`system-ui, -apple-system, "Segoe UI", Arial, sans-serif`,
	`Roboto, "Noto Sans", "Segoe UI", Arial, sans-serif`,
	`"Helvetica Neue", Helvetica, Arial, "PT Sans", sans-serif`,
	`Verdana, Geneva, "DejaVu Sans", Arial, sans-serif`,
	`Tahoma, Verdana, Segoe, Arial, sans-serif`,
	`Georgia, "Times New Roman", "PT Serif", serif`,
}

// variantRand builds a generator that is stable for one install and one theme,
// so a page looks the same on every reload but differs between installs.
func variantRand(seed, theme string) *rand.Rand {
	sum := sha256.Sum256([]byte(seed + "\x00" + theme))
	return rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(sum[:8])))) //nolint:gosec // appearance only, not a secret
}

func cssf(format string, args ...any) template.CSS {
	return template.CSS(fmt.Sprintf(format, args...))
}

// NewVariant derives the look of one theme for one install.
func NewVariant(seed, theme string) Variant {
	r := variantRand(seed, theme)
	hue := r.Intn(360)
	return Variant{
		Hue:     hue,
		Accent:  cssf("hsl(%d %d%% %d%%)", hue, 55+r.Intn(25), 42+r.Intn(10)),
		Ink:     cssf("hsl(%d %d%% %d%%)", hue, 8+r.Intn(10), 14+r.Intn(8)),
		Muted:   cssf("hsl(%d %d%% %d%%)", hue, 6+r.Intn(8), 44+r.Intn(10)),
		Surface: cssf("hsl(%d %d%% %d%%)", hue, 10+r.Intn(14), 95+r.Intn(4)),
		Panel:   cssf("hsl(%d %d%% %d%%)", hue, 12+r.Intn(14), 99),
		Font:    template.CSS(variantFonts[r.Intn(len(variantFonts))]),
		Radius:  []int{0, 4, 6, 8, 12, 16}[r.Intn(6)],
		Pad:     20 + r.Intn(16),
		Wide:    28 + r.Intn(12),
	}
}

// Pick returns one of the supplied wordings, chosen per install. Callers give
// several ways of saying the same thing so two installs do not phrase a page
// identically even when they land on the same theme.
func (v Variant) Pick(theme, slot string, options ...string) string {
	if len(options) == 0 {
		return ""
	}
	r := variantRand(fmt.Sprintf("%d", v.Hue)+theme, slot)
	return options[r.Intn(len(options))]
}
