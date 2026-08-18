package geodata

import (
	"encoding/json"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	xraygeodata "github.com/xtls/xray-core/common/geodata"
	"google.golang.org/protobuf/proto"
)

func writeSiteDB(t *testing.T, dir, name string, sites ...*xraygeodata.GeoSite) string {
	t.Helper()
	data, err := proto.Marshal(&xraygeodata.GeoSiteList{Entry: sites})
	if err != nil {
		t.Fatalf("marshal geosite list: %v", err)
	}
	return writeFile(t, dir, name, data)
}

func writeIPDB(t *testing.T, dir, name string, geoips ...*xraygeodata.GeoIP) string {
	t.Helper()
	data, err := proto.Marshal(&xraygeodata.GeoIPList{Entry: geoips})
	if err != nil {
		t.Fatalf("marshal geoip list: %v", err)
	}
	return writeFile(t, dir, name, data)
}

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func site(code string, domains ...*xraygeodata.Domain) *xraygeodata.GeoSite {
	return &xraygeodata.GeoSite{Code: code, Domain: domains}
}

func domain(domainType xraygeodata.Domain_Type, value string, attributes ...string) *xraygeodata.Domain {
	d := &xraygeodata.Domain{Type: domainType, Value: value}
	for _, attribute := range attributes {
		d.Attribute = append(d.Attribute, &xraygeodata.Domain_Attribute{
			Key:        attribute,
			TypedValue: &xraygeodata.Domain_Attribute_BoolValue{BoolValue: true},
		})
	}
	return d
}

func geoip(code string, prefixes ...string) *xraygeodata.GeoIP {
	entry := &xraygeodata.GeoIP{Code: code}
	for _, raw := range prefixes {
		prefix := netip.MustParsePrefix(raw)
		entry.Cidr = append(entry.Cidr, &xraygeodata.CIDR{
			Ip:     prefix.Addr().AsSlice(),
			Prefix: uint32(prefix.Bits()),
		})
	}
	return entry
}

func sampleSiteDB(t *testing.T, dir string) {
	t.Helper()
	writeSiteDB(t, dir, "geosite.dat",
		site("google",
			domain(xraygeodata.Domain_Domain, "google.com"),
			domain(xraygeodata.Domain_Full, "ads.google.com", "ads"),
			domain(xraygeodata.Domain_Substr, "googlevideo", "cn"),
			domain(xraygeodata.Domain_Regex, `^g.*\.cn$`),
		),
		site("CN",
			domain(xraygeodata.Domain_Domain, "baidu.com"),
			domain(xraygeodata.Domain_Domain, "qq.com"),
		),
	)
}

func TestListFilesReportsKindAndCategories(t *testing.T) {
	dir := t.TempDir()
	sampleSiteDB(t, dir)
	writeIPDB(t, dir, "geoip.dat", geoip("cn", "1.0.1.0/24"), geoip("private", "10.0.0.0/8", "fc00::/7"))

	files, err := NewStore(dir).ListFiles()
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ListFiles() returned %d files, want 2", len(files))
	}

	byName := make(map[string]GeoFile, len(files))
	for _, file := range files {
		byName[file.Name] = file
	}

	geosite := byName["geosite.dat"]
	if geosite.Kind != KindSite {
		t.Errorf("geosite.dat kind = %q, want %q", geosite.Kind, KindSite)
	}
	if geosite.Categories != 2 {
		t.Errorf("geosite.dat categories = %d, want 2", geosite.Categories)
	}
	if geosite.Error != "" {
		t.Errorf("geosite.dat error = %q, want empty", geosite.Error)
	}

	geoipFile := byName["geoip.dat"]
	if geoipFile.Kind != KindIP {
		t.Errorf("geoip.dat kind = %q, want %q", geoipFile.Kind, KindIP)
	}
	if geoipFile.Categories != 2 {
		t.Errorf("geoip.dat categories = %d, want 2", geoipFile.Categories)
	}
}

func TestKindDetectedFromContentsNotName(t *testing.T) {
	dir := t.TempDir()
	writeSiteDB(t, dir, "my_ip_rules.dat", site("corp", domain(xraygeodata.Domain_Domain, "intranet.corp.local")))
	writeIPDB(t, dir, "custom_sites.dat", geoip("office", "192.168.7.0/24"))

	store := NewStore(dir)

	sitePage, err := store.Categories("my_ip_rules.dat", "", 0, 10)
	if err != nil {
		t.Fatalf("Categories(my_ip_rules.dat) error = %v", err)
	}
	if sitePage.Total != 1 || sitePage.Items[0].Code != "corp" {
		t.Fatalf("Categories(my_ip_rules.dat) = %+v, want single category corp", sitePage)
	}

	entries, err := store.Entries("custom_sites.dat", "office", "", 0, 10)
	if err != nil {
		t.Fatalf("Entries(custom_sites.dat) error = %v", err)
	}
	if len(entries.Items) != 1 {
		t.Fatalf("Entries(custom_sites.dat) returned %d items, want 1", len(entries.Items))
	}
	if got := entries.Items[0]; got.Kind != "cidr" || got.Value != "192.168.7.0/24" {
		t.Errorf("entry = %+v, want cidr 192.168.7.0/24", got)
	}
}

func TestEntriesMapDomainTypesAndAttributes(t *testing.T) {
	dir := t.TempDir()
	sampleSiteDB(t, dir)
	store := NewStore(dir)

	page, err := store.Entries("geosite.dat", "google", "", 0, 10)
	if err != nil {
		t.Fatalf("Entries() error = %v", err)
	}
	want := []GeoEntry{
		{Kind: "domain", Value: "google.com"},
		{Kind: "full", Value: "ads.google.com"},
		{Kind: "keyword", Value: "googlevideo"},
		{Kind: "regexp", Value: `^g.*\.cn$`},
	}
	if page.Total != len(want) {
		t.Fatalf("Entries() total = %d, want %d", page.Total, len(want))
	}
	for i, entry := range want {
		if page.Items[i] != entry {
			t.Errorf("entry %d = %+v, want %+v", i, page.Items[i], entry)
		}
	}

	category, err := store.Lookup("geosite.dat", "google")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if len(category.Attributes) != 2 || category.Attributes[0] != "ads" || category.Attributes[1] != "cn" {
		t.Errorf("attributes = %v, want [ads cn]", category.Attributes)
	}
}

func TestCategoriesWithoutAttributesMarshalAsEmptyArray(t *testing.T) {
	dir := t.TempDir()
	sampleSiteDB(t, dir)

	page, err := NewStore(dir).Categories("geosite.dat", "cn", 0, 10)
	if err != nil {
		t.Fatalf("Categories() error = %v", err)
	}
	if page.Items[0].Attributes == nil {
		t.Fatal("attributes are nil, want an empty slice so the JSON stays an array")
	}
	encoded, err := json.Marshal(page.Items[0])
	if err != nil {
		t.Fatalf("marshal category: %v", err)
	}
	if !strings.Contains(string(encoded), `"attributes":[]`) {
		t.Errorf("encoded category = %s, want an empty attributes array", encoded)
	}
}

func TestCategoryCodesAreLowercasedAndSorted(t *testing.T) {
	dir := t.TempDir()
	sampleSiteDB(t, dir)

	page, err := NewStore(dir).Categories("geosite.dat", "", 0, 10)
	if err != nil {
		t.Fatalf("Categories() error = %v", err)
	}
	if page.Items[0].Code != "cn" || page.Items[1].Code != "google" {
		t.Errorf("codes = %q, %q; want cn, google", page.Items[0].Code, page.Items[1].Code)
	}
}

func TestSearchFilters(t *testing.T) {
	dir := t.TempDir()
	sampleSiteDB(t, dir)
	store := NewStore(dir)

	categories, err := store.Categories("geosite.dat", "OOG", 0, 10)
	if err != nil {
		t.Fatalf("Categories() error = %v", err)
	}
	if categories.Total != 1 || categories.Items[0].Code != "google" {
		t.Errorf("Categories(OOG) = %+v, want only google", categories)
	}

	entries, err := store.Entries("geosite.dat", "google", "ADS.", 0, 10)
	if err != nil {
		t.Fatalf("Entries() error = %v", err)
	}
	if entries.Total != 1 || entries.Items[0].Value != "ads.google.com" {
		t.Errorf("Entries(ADS.) = %+v, want only ads.google.com", entries)
	}
}

func TestPagination(t *testing.T) {
	dir := t.TempDir()
	domains := make([]*xraygeodata.Domain, 0, 250)
	for i := range 250 {
		domains = append(domains, domain(xraygeodata.Domain_Domain, "host"+strconv.Itoa(i)+".example.com"))
	}
	writeSiteDB(t, dir, "geosite.dat", site("bulk", domains...))
	store := NewStore(dir)

	tests := []struct {
		name      string
		offset    int
		limit     int
		wantCount int
		wantFirst string
	}{
		{name: "first page", offset: 0, limit: 10, wantCount: 10, wantFirst: "host0.example.com"},
		{name: "middle page", offset: 20, limit: 5, wantCount: 5, wantFirst: "host20.example.com"},
		{name: "negative offset clamps to start", offset: -5, limit: 3, wantCount: 3, wantFirst: "host0.example.com"},
		{name: "tail shorter than limit", offset: 245, limit: 50, wantCount: 5, wantFirst: "host245.example.com"},
		{name: "offset past end", offset: 900, limit: 10, wantCount: 0},
		{name: "limit above cap", offset: 0, limit: 5000, wantCount: 250, wantFirst: "host0.example.com"},
		{name: "zero limit uses cap", offset: 0, limit: 0, wantCount: 250, wantFirst: "host0.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := store.Entries("geosite.dat", "bulk", "", tt.offset, tt.limit)
			if err != nil {
				t.Fatalf("Entries() error = %v", err)
			}
			if page.Total != 250 {
				t.Errorf("total = %d, want 250", page.Total)
			}
			if len(page.Items) != tt.wantCount {
				t.Fatalf("items = %d, want %d", len(page.Items), tt.wantCount)
			}
			if tt.wantFirst != "" && page.Items[0].Value != tt.wantFirst {
				t.Errorf("first item = %q, want %q", page.Items[0].Value, tt.wantFirst)
			}
		})
	}
}

func TestCategoriesReturnEverythingWithoutLimit(t *testing.T) {
	dir := t.TempDir()
	sites := make([]*xraygeodata.GeoSite, 0, MaxPageSize+20)
	for i := range MaxPageSize + 20 {
		sites = append(sites, site("cat"+strconv.Itoa(i), domain(xraygeodata.Domain_Domain, "example.com")))
	}
	writeSiteDB(t, dir, "geosite.dat", sites...)
	store := NewStore(dir)

	all, err := store.Categories("geosite.dat", "", 0, 0)
	if err != nil {
		t.Fatalf("Categories() error = %v", err)
	}
	if len(all.Items) != MaxPageSize+20 {
		t.Errorf("items without a limit = %d, want %d", len(all.Items), MaxPageSize+20)
	}

	capped, err := store.Categories("geosite.dat", "", 0, 10)
	if err != nil {
		t.Fatalf("Categories() error = %v", err)
	}
	if len(capped.Items) != 10 || capped.Total != MaxPageSize+20 {
		t.Errorf("explicit limit gave %d items with total %d, want 10 and %d", len(capped.Items), capped.Total, MaxPageSize+20)
	}

	entries, err := store.Entries("geosite.dat", "cat0", "", 0, 0)
	if err != nil {
		t.Fatalf("Entries() error = %v", err)
	}
	if len(entries.Items) != 1 {
		t.Errorf("entries = %d, want 1", len(entries.Items))
	}
}

func TestErrors(t *testing.T) {
	dir := t.TempDir()
	sampleSiteDB(t, dir)
	writeFile(t, dir, "broken.dat", []byte("this is not a protobuf message at all"))
	store := NewStore(dir)

	tests := []struct {
		name string
		call func() error
		want error
	}{
		{
			name: "unknown category",
			call: func() error { _, err := store.Entries("geosite.dat", "nope", "", 0, 10); return err },
			want: ErrUnknownCategory,
		},
		{
			name: "lookup of unknown category",
			call: func() error { _, err := store.Lookup("geosite.dat", "nope"); return err },
			want: ErrUnknownCategory,
		},
		{
			name: "path traversal",
			call: func() error { _, err := store.Categories("../geosite.dat", "", 0, 10); return err },
			want: ErrInvalidName,
		},
		{
			name: "non dat extension",
			call: func() error { _, err := store.Categories("x-ui.db", "", 0, 10); return err },
			want: ErrInvalidName,
		},
		{
			name: "empty name",
			call: func() error { _, err := store.Categories("", "", 0, 10); return err },
			want: ErrInvalidName,
		},
		{
			name: "unparsable file",
			call: func() error { _, err := store.Categories("broken.dat", "", 0, 10); return err },
			want: ErrUnrecognized,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestBrokenFileIsListedWithReason(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "broken.dat", []byte("not a database"))

	files, err := NewStore(dir).ListFiles()
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("ListFiles() returned %d files, want 1", len(files))
	}
	if !strings.HasPrefix(files[0].Error, ErrUnrecognized.Error()) {
		t.Errorf("error = %q, want it to start with %q", files[0].Error, ErrUnrecognized.Error())
	}
	if files[0].Kind != "" {
		t.Errorf("kind = %q, want empty", files[0].Kind)
	}
}

func TestFileAboveSizeLimitIsRejected(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "huge.dat", []byte("x"))
	if err := os.Truncate(path, MaxFileSize+1); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	store := NewStore(dir)
	if _, err := store.Categories("huge.dat", "", 0, 10); !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("error = %v, want %v", err, ErrFileTooLarge)
	}

	files, err := store.ListFiles()
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}
	if len(files) != 1 || files[0].Error != ErrFileTooLarge.Error() {
		t.Errorf("ListFiles() = %+v, want the file listed with a too-large error", files)
	}
}

func TestIndexCacheInvalidatedWhenFileChanges(t *testing.T) {
	dir := t.TempDir()
	sampleSiteDB(t, dir)
	store := NewStore(dir)

	before, err := store.Categories("geosite.dat", "", 0, 10)
	if err != nil {
		t.Fatalf("Categories() error = %v", err)
	}
	if before.Total != 2 {
		t.Fatalf("total before rewrite = %d, want 2", before.Total)
	}

	path := writeSiteDB(t, dir, "geosite.dat",
		site("google", domain(xraygeodata.Domain_Domain, "google.com")),
		site("cn", domain(xraygeodata.Domain_Domain, "baidu.com")),
		site("telegram", domain(xraygeodata.Domain_Domain, "t.me")),
	)
	touch(t, path, time.Now().Add(time.Second))

	after, err := store.Categories("geosite.dat", "", 0, 10)
	if err != nil {
		t.Fatalf("Categories() after rewrite error = %v", err)
	}
	if after.Total != 3 {
		t.Errorf("total after rewrite = %d, want 3", after.Total)
	}
	if len(store.indexes) != 1 {
		t.Errorf("cached indexes = %d, want 1 after the stale entry is dropped", len(store.indexes))
	}
}

func touch(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

func TestDefaultRouteCIDRSurvives(t *testing.T) {
	dir := t.TempDir()
	writeIPDB(t, dir, "geoip.dat", geoip("any", "0.0.0.0/0", "::/0"), geoip("cn", "1.0.1.0/24"))

	page, err := NewStore(dir).Entries("geoip.dat", "any", "", 0, 10)
	if err != nil {
		t.Fatalf("Entries() error = %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("total = %d, want 2 — a zero prefix is omitted by proto3 and must not be dropped", page.Total)
	}
	if page.Items[0].Value != "0.0.0.0/0" || page.Items[1].Value != "::/0" {
		t.Errorf("items = %+v, want the two default routes", page.Items)
	}
}

func TestBrokenFileIsParsedOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "broken.dat", []byte("not a database"))
	store := NewStore(dir)

	for range 3 {
		if _, err := store.Categories("broken.dat", "", 0, 10); !errors.Is(err, ErrUnrecognized) {
			t.Fatalf("error = %v, want %v", err, ErrUnrecognized)
		}
	}
	if len(store.indexes) != 1 {
		t.Errorf("cached indexes = %d, want the failure cached once", len(store.indexes))
	}
}

func TestConcurrentReadsAreConsistent(t *testing.T) {
	dir := t.TempDir()
	sampleSiteDB(t, dir)
	writeIPDB(t, dir, "geoip.dat", geoip("private", "10.0.0.0/8"))
	store := NewStore(dir)

	var wg sync.WaitGroup
	for i := range 24 {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			switch worker % 3 {
			case 0:
				page, err := store.Categories("geosite.dat", "", 0, 0)
				if err != nil || page.Total != 2 {
					t.Errorf("Categories() = %+v, err = %v; want 2 categories", page, err)
				}
			case 1:
				page, err := store.Entries("geosite.dat", "google", "", 0, 10)
				if err != nil || page.Total != 4 {
					t.Errorf("Entries() = %+v, err = %v; want 4 entries", page, err)
				}
			default:
				files, err := store.ListFiles()
				if err != nil || len(files) != 2 {
					t.Errorf("ListFiles() = %d files, err = %v; want 2 files", len(files), err)
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestLookupDoesNotForgiveStraySpaces(t *testing.T) {
	dir := t.TempDir()
	sampleSiteDB(t, dir)
	store := NewStore(dir)

	if _, err := store.Lookup("geosite.dat", "google"); err != nil {
		t.Fatalf("Lookup(google) error = %v", err)
	}
	for _, code := range []string{" google", "google ", "goo gle"} {
		if _, err := store.Lookup("geosite.dat", code); !errors.Is(err, ErrUnknownCategory) {
			t.Errorf("Lookup(%q) error = %v, want %v — the core does not trim either", code, err, ErrUnknownCategory)
		}
	}
}

func TestSymlinkOutOfTheAssetFolderIsRefused(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.dat")
	if err := os.WriteFile(secret, []byte("not yours"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	dir := t.TempDir()
	sampleSiteDB(t, dir)
	if err := os.Symlink(secret, filepath.Join(dir, "escape.dat")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	store := NewStore(dir)
	if _, err := store.Categories("escape.dat", "", 0, 10); err == nil {
		t.Error("Categories() read through a symlink pointing outside the asset folder")
	}
	if _, err := store.Entries("escape.dat", "google", "", 0, 10); err == nil {
		t.Error("Entries() read through a symlink pointing outside the asset folder")
	}

	files, err := store.ListFiles()
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}
	for _, file := range files {
		if file.Name == "escape.dat" && file.Error == "" {
			t.Error("ListFiles() reported an escaping symlink as a usable database")
		}
	}
}
