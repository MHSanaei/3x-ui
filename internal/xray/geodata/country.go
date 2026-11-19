package geodata

import (
	"cmp"
	"encoding/binary"
	"net/netip"
	"slices"
)

// CountryFileName is the Xray-shipped database that holds country ranges.
const CountryFileName = "geoip.dat"

// CountryIndex answers which country an address belongs to. The database's million
// ranges are flattened into sorted breakpoints, each naming the country from there on.
type CountryIndex struct {
	starts []addrKey
	codes  [][2]byte
}

// addrKey orders addresses numerically; IPv4 sits in its v4-mapped place in the IPv6 space.
type addrKey struct{ hi, lo uint64 }

func (k addrKey) compare(other addrKey) int {
	return cmp.Or(cmp.Compare(k.hi, other.hi), cmp.Compare(k.lo, other.lo))
}

func (k addrKey) next() (addrKey, bool) {
	if k.lo != ^uint64(0) {
		return addrKey{k.hi, k.lo + 1}, true
	}
	if k.hi != ^uint64(0) {
		return addrKey{k.hi + 1, 0}, true
	}
	return addrKey{}, false
}

func keyOf(addr netip.Addr) addrKey {
	raw := addr.As16()
	return addrKey{binary.BigEndian.Uint64(raw[:8]), binary.BigEndian.Uint64(raw[8:])}
}

// countryRange is one CIDR while the table is built. A database holds a million of
// these, so it stores no end address: that alone would add 16 MB to the load's peak.
type countryRange struct {
	first addrKey
	bits  uint8
	code  [2]byte
}

func (r countryRange) last() addrKey {
	switch {
	case r.bits == 0:
		return addrKey{^uint64(0), ^uint64(0)}
	case r.bits <= 64:
		return addrKey{r.first.hi | ^uint64(0)>>uint(r.bits), ^uint64(0)}
	case r.bits < 128:
		return addrKey{r.first.hi, r.first.lo | ^uint64(0)>>uint(r.bits-64)}
	}
	return r.first
}

func newCountryRange(prefix netip.Prefix, code [2]byte) countryRange {
	masked := prefix.Masked()
	bits := masked.Bits()
	if masked.Addr().Is4() {
		bits += 96
	}
	return countryRange{first: keyOf(masked.Addr()), bits: uint8(bits), code: code}
}

// countryCode accepts ISO-style codes only: geoip.dat also carries service
// categories such as "cloudflare" and "private", which are not places.
func countryCode(code string) ([2]byte, bool) {
	if len(code) != 2 || code[0] < 'a' || code[0] > 'z' || code[1] < 'a' || code[1] > 'z' {
		return [2]byte{}, false
	}
	return [2]byte{code[0] - 'a' + 'A', code[1] - 'a' + 'A'}, true
}

// LoadCountryIndex reads a geoip database from dir. Reading goes through the same
// os.Root confinement as browsing it, so name cannot reach outside the folder.
func LoadCountryIndex(dir, name string) (*CountryIndex, error) {
	data, err := readDatabase(dir, name)
	if err != nil {
		return nil, err
	}
	// Counting first avoids the garbage of growing a slice to a million entries by doubling.
	total := 0
	err = eachListEntry(data, func(entry []byte, _ byteSpan) error {
		_, err := walkEntry(entry, func([]byte) error {
			total++
			return nil
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	ranges := make([]countryRange, 0, total)
	err = eachListEntry(data, func(entry []byte, _ byteSpan) error {
		rawCode, err := walkEntry(entry, nil)
		if err != nil {
			return err
		}
		code, ok := countryCode(rawCode)
		if !ok {
			return nil
		}
		_, err = walkEntry(entry, func(payload []byte) error {
			prefix, ok, err := parseCIDR(payload)
			if err != nil || !ok {
				return err
			}
			ranges = append(ranges, newCountryRange(prefix, code))
			return nil
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return flattenCountryRanges(ranges), nil
}

// flattenCountryRanges relies on CIDRs nesting or being disjoint, never partly overlapping:
// sorted outermost first, a stack of open ranges always has the most specific on top.
func flattenCountryRanges(ranges []countryRange) *CountryIndex {
	slices.SortFunc(ranges, func(a, b countryRange) int {
		return cmp.Or(a.first.compare(b.first), cmp.Compare(a.bits, b.bits), slices.Compare(a.code[:], b.code[:]))
	})
	idx := &CountryIndex{
		starts: make([]addrKey, 0, len(ranges)),
		codes:  make([][2]byte, 0, len(ranges)),
	}
	emit := func(at addrKey, code [2]byte) {
		if n := len(idx.starts); n > 0 && idx.starts[n-1] == at {
			idx.starts, idx.codes = idx.starts[:n-1], idx.codes[:n-1]
		}
		if n := len(idx.codes); n > 0 && idx.codes[n-1] == code {
			return
		}
		idx.starts = append(idx.starts, at)
		idx.codes = append(idx.codes, code)
	}
	var open []countryRange
	closeBefore := func(limit addrKey, unbounded bool) {
		for len(open) > 0 && (unbounded || open[len(open)-1].last().compare(limit) < 0) {
			closed := open[len(open)-1]
			open = open[:len(open)-1]
			after, ok := closed.last().next()
			if !ok {
				continue
			}
			var code [2]byte
			if len(open) > 0 {
				code = open[len(open)-1].code
			}
			emit(after, code)
		}
	}
	for _, r := range ranges {
		closeBefore(r.first, false)
		open = append(open, r)
		emit(r.first, r.code)
	}
	closeBefore(addrKey{}, true)
	// A clone, not Clip: merging leaves about a third of the capacity unused for good.
	idx.starts = slices.Clone(idx.starts)
	idx.codes = slices.Clone(idx.codes)
	return idx
}

// Country returns the ISO code covering addr, or "" when no country range does.
func (x *CountryIndex) Country(addr netip.Addr) string {
	i, found := slices.BinarySearchFunc(x.starts, keyOf(addr), addrKey.compare)
	if !found {
		i--
	}
	if i < 0 || x.codes[i] == ([2]byte{}) {
		return ""
	}
	return string(x.codes[i][:])
}
