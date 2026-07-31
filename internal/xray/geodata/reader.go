package geodata

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

// ErrUnrecognized reports a file that parses as neither database layout, which
// in practice means a truncated download or an unrelated file renamed to .dat.
var ErrUnrecognized = errors.New("file is not a geosite or geoip database")

const (
	fieldListEntry     = 1
	fieldEntryCode     = 1
	fieldEntryPayload  = 2
	fieldDomainType    = 1
	fieldDomainValue   = 2
	fieldDomainAttr    = 3
	fieldAttrKey       = 1
	fieldCIDRAddress   = 1
	fieldCIDRPrefixLen = 2
)

const (
	domainTypeSubstr = 0
	domainTypeRegex  = 1
	domainTypeFull   = 3
)

type categoryScan struct {
	kind       GeoKind
	categories []GeoCategory
	usable     int
}

// scanIndex walks the database once and reports every category with its entry
// count and attribute keys, holding nothing else in memory.
func scanIndex(data []byte, kind GeoKind) (*categoryScan, error) {
	scan := &categoryScan{kind: kind}
	byCode := make(map[string]int)

	err := eachListEntry(data, func(entry []byte) error {
		code, payloads, err := splitEntry(entry)
		if err != nil || code == "" {
			return err
		}
		count := 0
		attributes := make(map[string]struct{})
		for _, payload := range payloads {
			if kind == KindSite {
				value, attrs, err := domainValue(payload)
				if err != nil {
					return err
				}
				if len(value) == 0 {
					continue
				}
				for _, attr := range attrs {
					attributes[attr] = struct{}{}
				}
			} else {
				_, ok, err := cidrBytes(payload)
				if err != nil {
					return err
				}
				if !ok {
					continue
				}
			}
			count++
		}
		scan.usable += count
		if position, seen := byCode[code]; seen {
			scan.categories[position].Entries += count
			scan.categories[position].Attributes = mergeAttributes(scan.categories[position].Attributes, attributes)
			return nil
		}
		byCode[code] = len(scan.categories)
		scan.categories = append(scan.categories, GeoCategory{
			Code:       code,
			Entries:    count,
			Attributes: mergeAttributes(nil, attributes),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(scan.categories, func(i, j int) bool { return scan.categories[i].Code < scan.categories[j].Code })
	return scan, nil
}

// scanEntries walks the database once and materialises only the requested page
// of one category, so browsing a category with hundreds of thousands of rules
// costs no more than browsing a small one.
func scanEntries(data []byte, kind GeoKind, code, query string, offset, limit int) (GeoEntryPage, bool, error) {
	page := GeoEntryPage{Items: []GeoEntry{}}
	found := false
	matched := 0

	err := eachListEntry(data, func(entry []byte) error {
		entryCode, payloads, err := splitEntry(entry)
		if err != nil || entryCode != code {
			return err
		}
		found = true
		for _, payload := range payloads {
			// Values stay as raw bytes until a row is known to belong on the
			// requested page: turning all 170k rules of a category into strings
			// to serve one screenful is what made this expensive.
			var raw []byte
			var ok bool
			if kind == KindSite {
				raw, _, err = domainValue(payload)
				if err != nil {
					return err
				}
				ok = len(raw) > 0
			} else {
				raw, ok, err = cidrBytes(payload)
				if err != nil {
					return err
				}
			}
			if !ok {
				continue
			}
			if query != "" && !containsFold(raw, query) {
				continue
			}
			if matched >= offset && len(page.Items) < limit {
				if kind == KindSite {
					page.Items = append(page.Items, GeoEntry{Kind: domainKind(payload), Value: string(raw)})
				} else {
					page.Items = append(page.Items, GeoEntry{Kind: "cidr", Value: string(raw)})
				}
			}
			matched++
		}
		return nil
	})
	if err != nil {
		return GeoEntryPage{}, false, err
	}
	page.Total = matched
	return page, found, nil
}

// detectKind reports which layout the file uses. The two share a wire layout
// whose field types disagree, so decoding one as the other yields no usable
// values at all — the count of readable entries is what tells them apart. The
// file name only picks which layout to try first, so the common case scans once.
func detectKind(data []byte, name string) (GeoKind, *categoryScan, error) {
	first, second := KindSite, KindIP
	if strings.Contains(strings.ToLower(name), "ip") {
		first, second = KindIP, KindSite
	}
	var firstErr error
	for _, kind := range [...]GeoKind{first, second} {
		scan, err := scanIndex(data, kind)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if scan.usable > 0 {
			return kind, scan, nil
		}
	}
	if firstErr != nil {
		// A truncated download is the common case here, and it reads very
		// differently to the user than "this is not a geo database at all".
		return "", nil, fmt.Errorf("%w: %w", ErrUnrecognized, firstErr)
	}
	return "", nil, ErrUnrecognized
}

func readDatabase(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func eachListEntry(data []byte, visit func(entry []byte) error) error {
	for len(data) > 0 {
		number, wireType, consumed := protowire.ConsumeTag(data)
		if consumed < 0 {
			return protowire.ParseError(consumed)
		}
		data = data[consumed:]
		if number == fieldListEntry && wireType == protowire.BytesType {
			entry, size := protowire.ConsumeBytes(data)
			if size < 0 {
				return protowire.ParseError(size)
			}
			if err := visit(entry); err != nil {
				return err
			}
			data = data[size:]
			continue
		}
		size := protowire.ConsumeFieldValue(number, wireType, data)
		if size < 0 {
			return protowire.ParseError(size)
		}
		data = data[size:]
	}
	return nil
}

func splitEntry(entry []byte) (string, [][]byte, error) {
	code := ""
	var payloads [][]byte
	for len(entry) > 0 {
		number, wireType, consumed := protowire.ConsumeTag(entry)
		if consumed < 0 {
			return "", nil, protowire.ParseError(consumed)
		}
		entry = entry[consumed:]
		switch {
		case number == fieldEntryCode && wireType == protowire.BytesType:
			value, size := protowire.ConsumeBytes(entry)
			if size < 0 {
				return "", nil, protowire.ParseError(size)
			}
			code = strings.ToLower(string(value))
			entry = entry[size:]
		case number == fieldEntryPayload && wireType == protowire.BytesType:
			payload, size := protowire.ConsumeBytes(entry)
			if size < 0 {
				return "", nil, protowire.ParseError(size)
			}
			payloads = append(payloads, payload)
			entry = entry[size:]
		default:
			size := protowire.ConsumeFieldValue(number, wireType, entry)
			if size < 0 {
				return "", nil, protowire.ParseError(size)
			}
			entry = entry[size:]
		}
	}
	return code, payloads, nil
}

func containsFold(haystack []byte, needle string) bool {
	return strings.Contains(strings.ToLower(string(haystack)), needle)
}

func domainValue(payload []byte) ([]byte, []string, error) {
	var value []byte
	var attributes []string
	for len(payload) > 0 {
		number, wireType, consumed := protowire.ConsumeTag(payload)
		if consumed < 0 {
			return nil, nil, protowire.ParseError(consumed)
		}
		payload = payload[consumed:]
		switch {
		case number == fieldDomainValue && wireType == protowire.BytesType:
			raw, size := protowire.ConsumeBytes(payload)
			if size < 0 {
				return nil, nil, protowire.ParseError(size)
			}
			value = raw
			payload = payload[size:]
		case number == fieldDomainAttr && wireType == protowire.BytesType:
			raw, size := protowire.ConsumeBytes(payload)
			if size < 0 {
				return nil, nil, protowire.ParseError(size)
			}
			if key := attributeKey(raw); key != "" {
				attributes = append(attributes, key)
			}
			payload = payload[size:]
		default:
			size := protowire.ConsumeFieldValue(number, wireType, payload)
			if size < 0 {
				return nil, nil, protowire.ParseError(size)
			}
			payload = payload[size:]
		}
	}
	return value, attributes, nil
}

func attributeKey(attribute []byte) string {
	for len(attribute) > 0 {
		number, wireType, consumed := protowire.ConsumeTag(attribute)
		if consumed < 0 {
			return ""
		}
		attribute = attribute[consumed:]
		if number == fieldAttrKey && wireType == protowire.BytesType {
			raw, size := protowire.ConsumeBytes(attribute)
			if size < 0 {
				return ""
			}
			return strings.ToLower(string(raw))
		}
		size := protowire.ConsumeFieldValue(number, wireType, attribute)
		if size < 0 {
			return ""
		}
		attribute = attribute[size:]
	}
	return ""
}

// domainKind maps a domain's match type. proto3 omits zero values, so a domain
// with no type field on the wire is a Substr (keyword) rule, not a domain one.
func domainKind(payload []byte) string {
	matchType := uint64(domainTypeSubstr)
	for len(payload) > 0 {
		number, wireType, consumed := protowire.ConsumeTag(payload)
		if consumed < 0 {
			break
		}
		payload = payload[consumed:]
		if number == fieldDomainType && wireType == protowire.VarintType {
			raw, size := protowire.ConsumeVarint(payload)
			if size < 0 {
				break
			}
			matchType = raw
			break
		}
		size := protowire.ConsumeFieldValue(number, wireType, payload)
		if size < 0 {
			break
		}
		payload = payload[size:]
	}
	switch matchType {
	case domainTypeFull:
		return "full"
	case domainTypeRegex:
		return "regexp"
	case domainTypeSubstr:
		return "keyword"
	default:
		return "domain"
	}
}

// cidrBytes renders one CIDR. proto3 omits zero values, so a missing prefix
// field means /0 — a default route, which a hand-built ext: database may well
// contain — and must not be read as "no prefix given".
func cidrBytes(payload []byte) ([]byte, bool, error) {
	var address []byte
	prefix := uint64(0)
	for len(payload) > 0 {
		number, wireType, consumed := protowire.ConsumeTag(payload)
		if consumed < 0 {
			return nil, false, protowire.ParseError(consumed)
		}
		payload = payload[consumed:]
		switch {
		case number == fieldCIDRAddress && wireType == protowire.BytesType:
			raw, size := protowire.ConsumeBytes(payload)
			if size < 0 {
				return nil, false, protowire.ParseError(size)
			}
			address = raw
			payload = payload[size:]
		case number == fieldCIDRPrefixLen && wireType == protowire.VarintType:
			raw, size := protowire.ConsumeVarint(payload)
			if size < 0 {
				return nil, false, protowire.ParseError(size)
			}
			prefix = raw
			payload = payload[size:]
		default:
			size := protowire.ConsumeFieldValue(number, wireType, payload)
			if size < 0 {
				return nil, false, protowire.ParseError(size)
			}
			payload = payload[size:]
		}
	}
	addr, ok := netip.AddrFromSlice(address)
	if !ok || prefix > uint64(addr.BitLen()) {
		return nil, false, nil
	}
	return []byte(netip.PrefixFrom(addr, int(prefix)).String()), true, nil
}

// mergeAttributes always returns a non-nil slice: the JSON contract declares
// attributes as an array, and a nil slice would marshal to null and break
// clients validating against it.
func mergeAttributes(existing []string, attributes map[string]struct{}) []string {
	if len(attributes) == 0 {
		if existing == nil {
			return []string{}
		}
		return existing
	}
	merged := make(map[string]struct{}, len(existing)+len(attributes))
	for _, attribute := range existing {
		merged[attribute] = struct{}{}
	}
	for attribute := range attributes {
		merged[attribute] = struct{}{}
	}
	out := make([]string, 0, len(merged))
	for attribute := range merged {
		out = append(out, attribute)
	}
	sort.Strings(out)
	return out
}
