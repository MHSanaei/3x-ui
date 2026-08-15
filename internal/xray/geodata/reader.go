package geodata

import (
	"errors"
	"fmt"
	"io"
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
	spans      map[string][]byteSpan
	usable     int
}

// byteSpan locates one category's record inside the database file, so a page of
// its rules can be read without pulling the whole file into memory again.
type byteSpan struct {
	offset int64
	length int64
}

// scanIndex walks the database once and reports every category with its entry
// count and attribute keys, holding nothing else in memory.
func scanIndex(data []byte, kind GeoKind) (*categoryScan, error) {
	scan := &categoryScan{kind: kind, spans: make(map[string][]byteSpan)}
	byCode := make(map[string]int)

	err := eachListEntry(data, func(entry []byte, span byteSpan) error {
		count := 0
		attributes := make(map[string]struct{})
		code, err := walkEntry(entry, func(payload []byte) error {
			if kind == KindSite {
				value, attrs, err := domainValue(payload)
				if err != nil {
					return err
				}
				if len(value) == 0 {
					return nil
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
					return nil
				}
			}
			count++
			return nil
		})
		if err != nil || code == "" {
			return err
		}
		scan.usable += count
		scan.spans[code] = append(scan.spans[code], span)
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
func scanEntries(records [][]byte, kind GeoKind, code, query string, offset, limit int) (GeoEntryPage, error) {
	page := GeoEntryPage{Items: []GeoEntry{}}
	matched := 0

	for _, entry := range records {
		// Values stay as raw bytes until a row is known to belong on the
		// requested page: turning all 170k rules of a category into strings
		// to serve one screenful is what made this expensive.
		if _, err := walkEntry(entry, func(payload []byte) error {
			var raw []byte
			var ok bool
			var err error
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
				return nil
			}
			if query != "" && !containsFold(raw, query) {
				return nil
			}
			if matched >= offset && len(page.Items) < limit {
				if kind == KindSite {
					page.Items = append(page.Items, GeoEntry{Kind: domainKind(payload), Value: string(raw)})
				} else {
					page.Items = append(page.Items, GeoEntry{Kind: "cidr", Value: string(raw)})
				}
			}
			matched++
			return nil
		}); err != nil {
			return GeoEntryPage{}, err
		}
	}
	page.Total = matched
	return page, nil
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

// readSpans reads only the recorded slices of the file, so serving a page of a
// category costs its own record rather than the whole database. The handle is
// opened through an os.Root for the same reason readDatabase is.
func readSpans(dir, name string, spans []byteSpan) ([][]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	records := make([][]byte, 0, len(spans))
	for _, span := range spans {
		if span.length <= 0 || span.length > MaxFileSize {
			return nil, ErrUnrecognized
		}
		record := make([]byte, span.length)
		if _, err := file.ReadAt(record, span.offset); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

// readDatabase reads one database through an os.Root rooted at the asset
// directory. Going through the root rather than a joined path means the file
// name — which arrives from an HTTP request — never becomes a path this code
// resolves itself: a symlink planted in the folder, or swapped in between the
// check and the read, cannot pull in a file from elsewhere on disk.
func readDatabase(dir, name string) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrInvalidName
	}
	if info.Size() > MaxFileSize {
		return nil, ErrFileTooLarge
	}
	return io.ReadAll(io.LimitReader(file, MaxFileSize))
}

func eachListEntry(data []byte, visit func(entry []byte, span byteSpan) error) error {
	total := int64(len(data))
	for len(data) > 0 {
		consumedSoFar := total - int64(len(data))
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
			span := byteSpan{offset: consumedSoFar + int64(consumed) + int64(size) - int64(len(entry)), length: int64(len(entry))}
			if err := visit(entry, span); err != nil {
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

// walkEntry reports a record's category code and hands each rule to visit.
// The rules are not collected into a slice first: a single category can hold
// a hundred thousand of them, and that slice was the bulk of what serving one
// page allocated.
func walkEntry(entry []byte, visit func(payload []byte) error) (string, error) {
	code := ""
	for len(entry) > 0 {
		number, wireType, consumed := protowire.ConsumeTag(entry)
		if consumed < 0 {
			return "", protowire.ParseError(consumed)
		}
		entry = entry[consumed:]
		switch {
		case number == fieldEntryCode && wireType == protowire.BytesType:
			value, size := protowire.ConsumeBytes(entry)
			if size < 0 {
				return "", protowire.ParseError(size)
			}
			code = strings.ToLower(string(value))
			entry = entry[size:]
		case number == fieldEntryPayload && wireType == protowire.BytesType:
			payload, size := protowire.ConsumeBytes(entry)
			if size < 0 {
				return "", protowire.ParseError(size)
			}
			if visit != nil {
				if err := visit(payload); err != nil {
					return "", err
				}
			}
			entry = entry[size:]
		default:
			size := protowire.ConsumeFieldValue(number, wireType, entry)
			if size < 0 {
				return "", protowire.ParseError(size)
			}
			entry = entry[size:]
		}
	}
	return code, nil
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
