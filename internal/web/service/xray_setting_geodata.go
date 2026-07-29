package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/xtls/xray-core/common/geodata"
	"google.golang.org/protobuf/encoding/protowire"
)

// maxGeodataFileSize bounds how much of one .dat file parseGeodataFile will
// read from disk. Real Loyalsoldier-published geoip.dat/geosite.dat files
// are a few MB to a few tens of MB; this leaves generous headroom while
// still refusing an unbounded/corrupt file instead of reading it whole.
const maxGeodataFileSize = 256 << 20 // 256 MiB

// geodataParseCalls counts parseGeodataFile invocations. Not read by any
// non-test code; exists so tests can assert that GetGeodataCategories'
// cache actually skips re-parsing on an unchanged fingerprint, rather than
// only checking the (identical either way) returned value.
var geodataParseCalls atomic.Int64

// GeodataCategories lists every geosite/geoip category found in the .dat
// files currently present in the Xray bin folder, already formatted as
// ready-to-use xray-core routing-rule values (see formatGeodataSuggestion).
// Returned by XraySettingService.GetGeodataCategories and served as
// GET /panel/api/xray/getGeodataCategories for the routing rule editor's
// Domain/IP autocomplete.
type GeodataCategories struct {
	Domain []string `json:"domain" example:"[\"geosite:cn\",\"geosite:youtube\"]"`
	IP     []string `json:"ip" example:"[\"geoip:cn\",\"geoip:private\"]"`
}

// geodataFileKind distinguishes a geosite-shaped .dat file (parsed as a
// geodata.GeoSiteList) from a geoip-shaped one (geodata.GeoIPList).
type geodataFileKind int

const (
	geositeFile geodataFileKind = iota
	geoipFile
)

// geodataFileEntry is one matched .dat file plus the (size, modTime)
// fingerprint used to invalidate the parsed-category cache.
type geodataFileEntry struct {
	name    string // base filename, e.g. "geosite_roscom.dat"
	path    string
	size    int64
	modTime time.Time
	kind    geodataFileKind
}

// geodataFileFingerprint is the comparable, cacheable projection of one
// geodataFileEntry used to detect whether a re-parse is needed.
type geodataFileFingerprint struct {
	name    string
	size    int64
	modTime int64 // UnixNano
}

// geodataCategoryCache holds the last computed result plus the exact file
// fingerprint it was computed from.
type geodataCategoryCache struct {
	fingerprint []geodataFileFingerprint
	result      GeodataCategories
}

// GetGeodataCategories scans config.GetBinFolderPath() for geosite*.dat /
// geoip*.dat files -- whatever is actually on disk right now, including any
// custom files the admin added via the Geodata auto-update feature -- and
// returns every category code they contain as a suggestion string:
// "geosite:<code>"/"geoip:<code>" for the default file, "ext:<file>:<code>"
// for any other file (xray-core's rule parser has no shorthand for those;
// see common/geodata/rule_parser.go in the vendored xray-core module).
//
// The result is cached in memory keyed by the (name, size, modTime)
// fingerprint of the matched files, so repeated calls are cheap until a
// file actually changes -- e.g. because xray-core's own geodata auto-update
// downloaded a new one. A file that fails to parse (e.g. an interrupted
// download) is skipped with a logged warning; it never fails the request.
//
// The mutex is held for the full scan-and-maybe-parse instead of being
// released around buildGeodataCategories: the work is bounded and
// idempotent (a full-file parse, not an unbounded/blocking operation), so
// serializing it is simpler and cheaper than the alternative of N
// concurrent cache misses (e.g. several browser tabs open at once) each
// independently re-parsing every .dat file before any of them get to store
// a result.
func (s *XraySettingService) GetGeodataCategories() GeodataCategories {
	dir := config.GetBinFolderPath()
	entries := scanGeodataFiles(dir)
	fingerprint := geodataFingerprintOf(entries)

	s.geodataMu.Lock()
	defer s.geodataMu.Unlock()

	if s.geodataCache != nil && slices.Equal(s.geodataCache.fingerprint, fingerprint) {
		return cloneGeodataCategories(s.geodataCache.result)
	}

	result := buildGeodataCategories(entries)
	s.geodataCache = &geodataCategoryCache{fingerprint: fingerprint, result: result}
	return cloneGeodataCategories(result)
}

// cloneGeodataCategories returns a copy whose slices don't alias the
// cache's own, so a caller mutating (e.g. sorting, appending to) its result
// can never corrupt what other goroutines read from the shared cache.
func cloneGeodataCategories(c GeodataCategories) GeodataCategories {
	return GeodataCategories{Domain: slices.Clone(c.Domain), IP: slices.Clone(c.IP)}
}

// scanGeodataFiles lists dir for files matched by name: geosite*.dat parses
// as a geodata.GeoSiteList, geoip*.dat as a geodata.GeoIPList. Matching is
// case-insensitive on the prefix/extension -- every real filename observed
// so far (geoip.dat, geosite_IR.dat, geosite_roscom.dat, ...) is
// lowercase-prefixed, but nothing enforces that when an admin drops a file
// in by hand, so this stays tolerant. A missing or unreadable bin folder
// yields an empty scan (logged, not an error) rather than failing the
// endpoint.
func scanGeodataFiles(dir string) []geodataFileEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warning("geodata categories: failed to read bin folder:", err)
		}
		return nil
	}

	out := make([]geodataFileEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".dat") {
			continue
		}
		var kind geodataFileKind
		switch {
		case strings.HasPrefix(lower, "geosite"):
			kind = geositeFile
		case strings.HasPrefix(lower, "geoip"):
			kind = geoipFile
		default:
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue // vanished between ReadDir and Info; skip
		}
		out = append(out, geodataFileEntry{
			name:    name,
			path:    filepath.Join(dir, name),
			size:    info.Size(),
			modTime: info.ModTime(),
			kind:    kind,
		})
	}
	return out
}

// geodataFingerprintOf reduces a file list to a sorted, comparable slice so
// GetGeodataCategories can detect "nothing changed" with slices.Equal
// regardless of os.ReadDir's ordering.
func geodataFingerprintOf(entries []geodataFileEntry) []geodataFileFingerprint {
	out := make([]geodataFileFingerprint, len(entries))
	for i, e := range entries {
		out[i] = geodataFileFingerprint{name: e.name, size: e.size, modTime: e.modTime.UnixNano()}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// buildGeodataCategories parses every matched file and collects its category
// codes into the Domain/IP suggestion lists, de-duplicating within each list
// and skipping (with a logged warning) any file that fails to parse.
func buildGeodataCategories(entries []geodataFileEntry) GeodataCategories {
	var result GeodataCategories
	seenDomain := make(map[string]struct{})
	seenIP := make(map[string]struct{})

	for _, entry := range entries {
		codes, err := parseGeodataFile(entry)
		if err != nil {
			logger.Warningf("geodata categories: skipping %s: %v", entry.name, err)
			continue
		}
		for _, code := range codes {
			suggestion := formatGeodataSuggestion(entry, code)
			switch entry.kind {
			case geositeFile:
				if _, dup := seenDomain[suggestion]; dup {
					continue
				}
				seenDomain[suggestion] = struct{}{}
				result.Domain = append(result.Domain, suggestion)
			case geoipFile:
				if _, dup := seenIP[suggestion]; dup {
					continue
				}
				seenIP[suggestion] = struct{}{}
				result.IP = append(result.IP, suggestion)
			}
		}
	}
	sort.Strings(result.Domain)
	sort.Strings(result.IP)
	return result
}

// parseGeodataFile returns every category Code found in one geosite*/geoip*.dat
// file. xray-core's own loaders (loadSite/loadIP in
// common/geodata/geodat_loader.go) stream a single named category out of a
// file via an unexported, custom varint-prefixed scanner; enumerating
// *every* category here instead walks the raw protobuf wire format directly
// via codesFromGeodataList rather than a full proto.Unmarshal, since the
// only field ever read is each entry's Code -- a full unmarshal would also
// materialize every Domain/CIDR message the file contains (the bulk of a
// real geoip.dat/geosite.dat's size) just to throw it away unused.
func parseGeodataFile(entry geodataFileEntry) ([]string, error) {
	geodataParseCalls.Add(1)

	f, err := os.Open(entry.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxGeodataFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxGeodataFileSize {
		return nil, fmt.Errorf("file exceeds %d byte limit", maxGeodataFileSize)
	}

	switch entry.kind {
	case geositeFile, geoipFile:
		return codesFromGeodataList(data)
	}
	return nil, nil
}

// codesFromGeodataList extracts every entry's Code from a serialized
// GeoSiteList or GeoIPList without unmarshaling into either message type.
// Both list messages put their repeated "entry" on field 1, and both
// GeoSite and GeoIP put "code" on field 1 of that entry (see
// common/geodata/geodat.proto) -- so every other field, including the
// repeated Domain/CIDR payloads that make up nearly all of a real file's
// size, is skipped via protowire.ConsumeFieldValue without ever being
// decoded into an allocated message.
func codesFromGeodataList(data []byte) ([]string, error) {
	var codes []string
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		data = data[n:]
		if num != 1 || typ != protowire.BytesType {
			m := protowire.ConsumeFieldValue(num, typ, data)
			if m < 0 {
				return nil, protowire.ParseError(m)
			}
			data = data[m:]
			continue
		}
		entry, m := protowire.ConsumeBytes(data)
		if m < 0 {
			return nil, protowire.ParseError(m)
		}
		data = data[m:]
		if code, ok := geodataEntryCode(entry); ok && code != "" {
			codes = append(codes, code)
		}
	}
	return codes, nil
}

// geodataEntryCode returns field 1 (GeoSite.code / GeoIP.code, both plain
// proto3 strings) of one serialized entry submessage.
func geodataEntryCode(data []byte) (string, bool) {
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return "", false
		}
		data = data[n:]
		if num != 1 || typ != protowire.BytesType {
			m := protowire.ConsumeFieldValue(num, typ, data)
			if m < 0 {
				return "", false
			}
			data = data[m:]
			continue
		}
		value, m := protowire.ConsumeBytes(data)
		if m < 0 {
			return "", false
		}
		return string(value), true
	}
	return "", false
}

// formatGeodataSuggestion builds the exact rule value xray-core's rule
// parser (common/geodata/rule_parser.go) accepts for one category code from
// one file. The default file gets the short "geosite:"/"geoip:" form; every
// other file -- including every custom file the admin added -- MUST use the
// "ext:<file>:<code>" form since there is no shorthand for non-default
// files. Code casing never matters to xray-core (it upcases internally,
// rule_parser.go), but lowercase reads better and matches this panel's
// existing convention (frontend/src/pages/xray/basics/constants.ts).
func formatGeodataSuggestion(entry geodataFileEntry, code string) string {
	lowerCode := strings.ToLower(code)
	switch entry.kind {
	case geositeFile:
		if strings.EqualFold(entry.name, geodata.DefaultGeoSiteDat) {
			return "geosite:" + lowerCode
		}
	case geoipFile:
		if strings.EqualFold(entry.name, geodata.DefaultGeoIPDat) {
			return "geoip:" + lowerCode
		}
	}
	return "ext:" + entry.name + ":" + lowerCode
}
