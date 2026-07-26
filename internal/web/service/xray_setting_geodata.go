package service

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/xtls/xray-core/common/geodata"
	"google.golang.org/protobuf/proto"
)

// GeodataCategories lists every geosite/geoip category found in the .dat
// files currently present in the Xray bin folder, already formatted as
// ready-to-use xray-core routing-rule values (see formatGeodataSuggestion).
// Returned by XraySettingService.GetGeodataCategories and served as
// GET /panel/api/xray/getGeodataCategories for the routing rule editor's
// Domain/IP autocomplete.
type GeodataCategories struct {
	Domain []string `json:"domain"`
	IP     []string `json:"ip"`
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
func (s *XraySettingService) GetGeodataCategories() GeodataCategories {
	dir := config.GetBinFolderPath()
	entries := scanGeodataFiles(dir)
	fingerprint := geodataFingerprintOf(entries)

	s.geodataMu.Lock()
	if s.geodataCache != nil && slices.Equal(s.geodataCache.fingerprint, fingerprint) {
		result := s.geodataCache.result
		s.geodataMu.Unlock()
		return result
	}
	s.geodataMu.Unlock()

	result := buildGeodataCategories(entries)

	s.geodataMu.Lock()
	s.geodataCache = &geodataCategoryCache{fingerprint: fingerprint, result: result}
	s.geodataMu.Unlock()

	return result
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

// parseGeodataFile fully unmarshals one geosite*/geoip*.dat file and returns
// every category Code it contains. xray-core's own loaders (loadSite/loadIP
// in common/geodata/geodat_loader.go) stream a single named category out of
// a file via an unexported, custom varint-prefixed scanner; enumerating
// *every* category instead needs a full-file proto.Unmarshal into the
// package's exported GeoSiteList/GeoIPList message types.
func parseGeodataFile(entry geodataFileEntry) ([]string, error) {
	data, err := os.ReadFile(entry.path)
	if err != nil {
		return nil, err
	}

	switch entry.kind {
	case geositeFile:
		var list geodata.GeoSiteList
		if err := proto.Unmarshal(data, &list); err != nil {
			return nil, err
		}
		codes := make([]string, 0, len(list.GetEntry()))
		for _, site := range list.GetEntry() {
			if code := site.GetCode(); code != "" {
				codes = append(codes, code)
			}
		}
		return codes, nil
	case geoipFile:
		var list geodata.GeoIPList
		if err := proto.Unmarshal(data, &list); err != nil {
			return nil, err
		}
		codes := make([]string, 0, len(list.GetEntry()))
		for _, ip := range list.GetEntry() {
			if code := ip.GetCode(); code != "" {
				codes = append(codes, code)
			}
		}
		return codes, nil
	}
	return nil, nil
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
		if entry.name == geodata.DefaultGeoSiteDat {
			return "geosite:" + lowerCode
		}
	case geoipFile:
		if entry.name == geodata.DefaultGeoIPDat {
			return "geoip:" + lowerCode
		}
	}
	return "ext:" + entry.name + ":" + lowerCode
}
