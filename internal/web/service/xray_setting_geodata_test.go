package service

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"

	"github.com/xtls/xray-core/common/geodata"
	"google.golang.org/protobuf/proto"
)

func writeGeoSiteFixture(t *testing.T, dir, name string, codes ...string) {
	t.Helper()
	list := &geodata.GeoSiteList{}
	for _, c := range codes {
		list.Entry = append(list.Entry, &geodata.GeoSite{Code: c})
	}
	data, err := proto.Marshal(list)
	if err != nil {
		t.Fatalf("marshal fixture geosite list: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

func writeGeoIPFixture(t *testing.T, dir, name string, codes ...string) {
	t.Helper()
	list := &geodata.GeoIPList{}
	for _, c := range codes {
		list.Entry = append(list.Entry, &geodata.GeoIP{Code: c})
	}
	data, err := proto.Marshal(list)
	if err != nil {
		t.Fatalf("marshal fixture geoip list: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

func TestScanGeodataFiles(t *testing.T) {
	dir := t.TempDir()
	writeGeoSiteFixture(t, dir, "geosite.dat", "CN")
	writeGeoSiteFixture(t, dir, "geosite_roscom.dat", "SOME-CODE")
	writeGeoIPFixture(t, dir, "geoip.dat", "PRIVATE")
	writeGeoIPFixture(t, dir, "GEOIP_RU.DAT", "RU") // case-insensitive match

	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write unrelated file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "geosite_dir.dat"), 0o755); err != nil {
		t.Fatalf("mkdir geosite_dir.dat: %v", err)
	}

	entries := scanGeodataFiles(dir)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.name)
	}
	sort.Strings(names)

	want := []string{"GEOIP_RU.DAT", "geoip.dat", "geosite.dat", "geosite_roscom.dat"}
	sort.Strings(want)
	if !slices.Equal(names, want) {
		t.Fatalf("scanGeodataFiles names = %v, want %v", names, want)
	}

	for _, e := range entries {
		switch e.name {
		case "geosite.dat", "geosite_roscom.dat":
			if e.kind != geositeFile {
				t.Errorf("%s: kind = %v, want geositeFile", e.name, e.kind)
			}
		case "geoip.dat", "GEOIP_RU.DAT":
			if e.kind != geoipFile {
				t.Errorf("%s: kind = %v, want geoipFile", e.name, e.kind)
			}
		}
	}
}

func TestScanGeodataFilesMissingDir(t *testing.T) {
	entries := scanGeodataFiles(filepath.Join(t.TempDir(), "does-not-exist"))
	if len(entries) != 0 {
		t.Fatalf("expected no entries for a missing dir, got %v", entries)
	}
}

func TestParseGeodataFile(t *testing.T) {
	dir := t.TempDir()
	writeGeoSiteFixture(t, dir, "geosite.dat", "CN", "YOUTUBE")
	writeGeoIPFixture(t, dir, "geoip_rosip.dat", "RU")

	siteCodes, err := parseGeodataFile(geodataFileEntry{
		name: "geosite.dat", path: filepath.Join(dir, "geosite.dat"), kind: geositeFile,
	})
	if err != nil {
		t.Fatalf("parseGeodataFile(geosite.dat): %v", err)
	}
	if !slices.Equal(siteCodes, []string{"CN", "YOUTUBE"}) {
		t.Fatalf("parseGeodataFile(geosite.dat) = %v, want [CN YOUTUBE]", siteCodes)
	}

	ipCodes, err := parseGeodataFile(geodataFileEntry{
		name: "geoip_rosip.dat", path: filepath.Join(dir, "geoip_rosip.dat"), kind: geoipFile,
	})
	if err != nil {
		t.Fatalf("parseGeodataFile(geoip_rosip.dat): %v", err)
	}
	if !slices.Equal(ipCodes, []string{"RU"}) {
		t.Fatalf("parseGeodataFile(geoip_rosip.dat) = %v, want [RU]", ipCodes)
	}
}

func TestParseGeodataFileRejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "geosite.dat")
	// Sparse-write a file one byte past the cap without actually holding
	// maxGeodataFileSize+1 bytes of valid protobuf in memory for the test.
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	if err := f.Truncate(maxGeodataFileSize + 1); err != nil {
		t.Fatalf("truncate fixture to %d bytes: %v", maxGeodataFileSize+1, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close fixture: %v", err)
	}

	_, err = parseGeodataFile(geodataFileEntry{name: "geosite.dat", path: path, kind: geositeFile})
	if err == nil {
		t.Fatal("expected parseGeodataFile to reject a file over the size limit, got nil error")
	}
}

func TestFormatGeodataSuggestion(t *testing.T) {
	tests := []struct {
		name string
		kind geodataFileKind
		code string
		want string
	}{
		{name: "geosite.dat", kind: geositeFile, code: "CN", want: "geosite:cn"},
		{name: "geoip.dat", kind: geoipFile, code: "PRIVATE", want: "geoip:private"},
		{name: "geosite_roscom.dat", kind: geositeFile, code: "SOME-CODE", want: "ext:geosite_roscom.dat:some-code"},
		{name: "geoip_rosip.dat", kind: geoipFile, code: "RU", want: "ext:geoip_rosip.dat:ru"},
		// Default filenames are matched case-insensitively, same as
		// scanGeodataFiles -- a file that IS the default one on a
		// case-insensitive filesystem (e.g. Windows) must still get the
		// short geosite:/geoip: form, not ext:.
		{name: "GEOSITE.DAT", kind: geositeFile, code: "CN", want: "geosite:cn"},
		{name: "GeoIP.dat", kind: geoipFile, code: "PRIVATE", want: "geoip:private"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := geodataFileEntry{name: tt.name, kind: tt.kind}
			if got := formatGeodataSuggestion(entry, tt.code); got != tt.want {
				t.Errorf("formatGeodataSuggestion(%q, %q) = %q, want %q", tt.name, tt.code, got, tt.want)
			}
		})
	}
}

func TestGeodataFingerprintOf(t *testing.T) {
	a := []geodataFileEntry{
		{name: "geoip.dat", size: 100, modTime: time.Unix(1, 0)},
		{name: "geosite.dat", size: 200, modTime: time.Unix(2, 0)},
	}

	t.Run("order independent", func(t *testing.T) {
		b := []geodataFileEntry{ // same content, different order
			{name: "geosite.dat", size: 200, modTime: time.Unix(2, 0)},
			{name: "geoip.dat", size: 100, modTime: time.Unix(1, 0)},
		}
		if !slices.Equal(geodataFingerprintOf(a), geodataFingerprintOf(b)) {
			t.Fatal("fingerprints should be equal regardless of input order")
		}
	})

	t.Run("size change invalidates", func(t *testing.T) {
		c := []geodataFileEntry{
			{name: "geoip.dat", size: 999, modTime: time.Unix(1, 0)}, // size changed
			{name: "geosite.dat", size: 200, modTime: time.Unix(2, 0)},
		}
		if slices.Equal(geodataFingerprintOf(a), geodataFingerprintOf(c)) {
			t.Fatal("fingerprints should differ when a file's size changes")
		}
	})
}

func TestGetGeodataCategories_SkipsMalformedFile(t *testing.T) {
	dir := t.TempDir()
	writeGeoSiteFixture(t, dir, "geosite.dat", "CN")
	// An unterminated varint (continuation bit set on every byte) is
	// guaranteed to fail parsing, unlike an arbitrary text string which
	// might accidentally parse as protobuf garbage.
	if err := os.WriteFile(filepath.Join(dir, "geosite_broken.dat"), []byte{0xFF, 0xFF, 0xFF}, 0o644); err != nil {
		t.Fatalf("write broken fixture: %v", err)
	}

	entries := scanGeodataFiles(dir)
	result := buildGeodataCategories(entries)

	// Exact equality, not just Contains: a regression that made the broken
	// file emit junk suggestions alongside the good one must fail this.
	if !slices.Equal(result.Domain, []string{"geosite:cn"}) {
		t.Fatalf("result.Domain = %v, want [geosite:cn]", result.Domain)
	}
	if len(result.IP) != 0 {
		t.Fatalf("result.IP = %v, want empty", result.IP)
	}
}

func TestGetGeodataCategories_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	if config.GetBinFolderPath() != dir {
		t.Fatalf("XUI_BIN_FOLDER override not respected: got %q, want %q", config.GetBinFolderPath(), dir)
	}

	writeGeoSiteFixture(t, dir, "geosite.dat", "CN")
	writeGeoIPFixture(t, dir, "geoip.dat", "PRIVATE")
	writeGeoSiteFixture(t, dir, "geosite_roscom.dat", "SOME-CODE")

	svc := &XraySettingService{}
	result := svc.GetGeodataCategories()

	wantDomain := []string{"ext:geosite_roscom.dat:some-code", "geosite:cn"}
	if !slices.Equal(result.Domain, wantDomain) {
		t.Errorf("Domain = %v, want %v", result.Domain, wantDomain)
	}
	if !slices.Equal(result.IP, []string{"geoip:private"}) {
		t.Errorf("IP = %v, want [geoip:private]", result.IP)
	}

	// Cache must reflect a file that appears after the first call.
	writeGeoIPFixture(t, dir, "geoip_rosip.dat", "RU")
	result = svc.GetGeodataCategories()
	wantIP := []string{"ext:geoip_rosip.dat:ru", "geoip:private"}
	if !slices.Equal(result.IP, wantIP) {
		t.Errorf("IP after adding a new file = %v, want %v", result.IP, wantIP)
	}
}

func TestGetGeodataCategories_CacheHitSkipsReparse(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	writeGeoSiteFixture(t, dir, "geosite.dat", "CN")
	writeGeoIPFixture(t, dir, "geoip.dat", "PRIVATE")

	svc := &XraySettingService{}

	before := geodataParseCalls.Load()
	first := svc.GetGeodataCategories()
	afterFirst := geodataParseCalls.Load()
	if afterFirst == before {
		t.Fatal("expected the first call (a cache miss) to parse at least one file")
	}

	second := svc.GetGeodataCategories()
	afterSecond := geodataParseCalls.Load()
	if afterSecond != afterFirst {
		t.Fatalf("expected a second call with an unchanged fingerprint to skip re-parsing entirely, but the parse count went from %d to %d", afterFirst, afterSecond)
	}
	if !slices.Equal(first.Domain, second.Domain) || !slices.Equal(first.IP, second.IP) {
		t.Fatalf("cached result differs from the original: first=%+v second=%+v", first, second)
	}
}

func TestGetGeodataCategories_ReturnsIndependentSlices(t *testing.T) {
	// The cache must never hand out its own backing array: a caller that
	// mutates its result (e.g. sorts or appends to it) must not corrupt what
	// other callers read from the shared cache on their next call.
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	writeGeoSiteFixture(t, dir, "geosite.dat", "CN", "YOUTUBE")

	svc := &XraySettingService{}
	first := svc.GetGeodataCategories()
	first.Domain[0] = "tampered"

	second := svc.GetGeodataCategories()
	if slices.Contains(second.Domain, "tampered") {
		t.Fatalf("mutating one caller's result leaked into the cache: %v", second.Domain)
	}
}
