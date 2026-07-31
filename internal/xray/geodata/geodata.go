// Package geodata reads Xray's geosite/geoip .dat databases so the panel can
// browse their categories instead of asking the user to type category names
// from memory.
//
// The databases are protobuf, but decoding them into Go structs is what makes
// them expensive: a 10 MB geosite.dat holds well over a million domains, and
// materialising all of them costs hundreds of megabytes on a panel that often
// runs with 512 MB of RAM. The readers therefore walk the wire format directly
// and allocate only what the caller asked for — category counts for the index,
// one page of values for the browser.
package geodata

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// MaxFileSize is the largest database the panel will parse. Community rule
// sets are far bigger than the official ones — russia-v2ray-rules-dat ships a
// 70 MB geosite — so the ceiling is set well above them; reading is streaming
// and serialised, so a scan costs about the file's own size once, not per
// request. The limit exists only to keep a stray huge file in the asset folder
// from taking the panel down with it.
const MaxFileSize int64 = 256 << 20

// MaxPageSize caps how many rows a single page may carry, independent of what
// the caller asks for.
const MaxPageSize = 500

var (
	// ErrFileTooLarge reports a database above MaxFileSize.
	ErrFileTooLarge = errors.New("geodata file is too large to browse")
	// ErrInvalidName reports a file name that does not resolve to a .dat file
	// directly inside the asset directory.
	ErrInvalidName = errors.New("invalid geodata file name")
	// ErrUnknownCategory reports a category code missing from the database.
	ErrUnknownCategory = errors.New("unknown geodata category")
)

// GeoKind tells apart the two database layouts Xray ships.
type GeoKind string

const (
	KindSite GeoKind = "site"
	KindIP   GeoKind = "ip"
)

// GeoFile describes one .dat database found in the asset directory.
type GeoFile struct {
	Name       string  `json:"name" example:"geosite.dat"`
	Kind       GeoKind `json:"kind" example:"site"`
	Size       int64   `json:"size" example:"1467392"`
	ModifiedAt int64   `json:"modifiedAt" example:"1769558400000"`
	Categories int     `json:"categories" example:"1043"`
	Error      string  `json:"error,omitempty" example:""`
}

// GeoCategory is one code inside a database, such as geosite's "google".
type GeoCategory struct {
	Code       string   `json:"code" example:"google"`
	Entries    int      `json:"entries" example:"1284"`
	Attributes []string `json:"attributes" example:"[\"ads\",\"cn\"]"`
}

// GeoEntry is a single rule inside a category: a domain rule for geosite
// databases, a CIDR for geoip ones.
type GeoEntry struct {
	Kind  string `json:"kind" example:"domain"`
	Value string `json:"value" example:"google.com"`
}

// GeoCategoryPage is one page of categories plus the unpaged total.
type GeoCategoryPage struct {
	Total int           `json:"total" example:"1043"`
	Items []GeoCategory `json:"items"`
}

// GeoEntryPage is one page of category entries plus the unpaged total.
type GeoEntryPage struct {
	Total int        `json:"total" example:"1284"`
	Items []GeoEntry `json:"items"`
}

type fileKey struct {
	name    string
	size    int64
	modTime int64
}

type index struct {
	kind       GeoKind
	categories []GeoCategory
	byCode     map[string]GeoCategory
	err        error
}

// Store reads databases from one asset directory. Only the category index is
// cached, for as long as the file on disk is unchanged; entry pages are scanned
// out of the file on demand, which keeps a browsing session's memory close to
// the size of the page being shown rather than the size of the database.
//
// Scans are serialised on purpose. Reading a database allocates on the order of
// its own size, so letting a page's parallel requests — or a scripted caller —
// scan several databases at once is what turns a browsable panel into an
// out-of-memory kill on a small VPS.
type Store struct {
	dir string

	mu      sync.Mutex
	indexes map[fileKey]*index

	scan sync.Mutex
}

// NewStore returns a Store reading databases from dir.
func NewStore(dir string) *Store {
	return &Store{dir: dir, indexes: make(map[fileKey]*index)}
}

// ListFiles reports every .dat database in the asset directory. A database that
// cannot be parsed is still listed, with the reason in GeoFile.Error, so the panel
// can show a broken download instead of hiding it.
func (s *Store) ListFiles() ([]GeoFile, error) {
	dirEntries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	files := make([]GeoFile, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() || !strings.HasSuffix(strings.ToLower(dirEntry.Name()), ".dat") {
			continue
		}
		info, err := dirEntry.Info()
		if err != nil {
			continue
		}
		file := GeoFile{
			Name:       dirEntry.Name(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().UnixMilli(),
		}
		idx, err := s.index(dirEntry.Name())
		if err != nil {
			file.Error = err.Error()
		} else {
			file.Kind = idx.kind
			file.Categories = len(idx.categories)
		}
		files = append(files, file)
	}
	return files, nil
}

// resolve validates a client-supplied file name and stats it through an
// os.Root, so a symlink planted in the asset folder cannot be used to read a
// file from elsewhere on disk.
func (s *Store) resolve(name string) (string, os.FileInfo, error) {
	if name == "" || name != filepath.Base(name) || !strings.HasSuffix(strings.ToLower(name), ".dat") {
		return "", nil, ErrInvalidName
	}
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return "", nil, err
	}
	defer root.Close()

	info, err := root.Stat(name)
	if err != nil {
		return "", nil, err
	}
	if !info.Mode().IsRegular() {
		return "", nil, ErrInvalidName
	}
	if info.Size() > MaxFileSize {
		return "", nil, ErrFileTooLarge
	}
	return filepath.Join(s.dir, name), info, nil
}

func (s *Store) index(name string) (*index, error) {
	path, info, err := s.resolve(name)
	if err != nil {
		return nil, err
	}
	key := fileKey{name: name, size: info.Size(), modTime: info.ModTime().UnixNano()}
	if cached, ok := s.cachedIndex(key); ok {
		return cached, cached.err
	}

	s.scan.Lock()
	defer s.scan.Unlock()
	// Another request may have built this index while this one waited.
	if cached, ok := s.cachedIndex(key); ok {
		return cached, cached.err
	}

	idx := buildIndex(path, name)

	s.mu.Lock()
	s.indexes[key] = idx
	s.dropStaleIndexesLocked(name, key)
	s.mu.Unlock()
	return idx, idx.err
}

func (s *Store) cachedIndex(key fileKey) (*index, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cached, ok := s.indexes[key]
	return cached, ok
}

// buildIndex never fails outright: a database that cannot be read is cached as
// a failed index, so a broken download is reported without being re-parsed on
// every request.
func buildIndex(path, name string) *index {
	data, err := readDatabase(path)
	if err != nil {
		return &index{err: err}
	}
	kind, scan, err := detectKind(data, name)
	if err != nil {
		return &index{err: err}
	}
	idx := &index{
		kind:       kind,
		categories: scan.categories,
		byCode:     make(map[string]GeoCategory, len(scan.categories)),
	}
	for _, category := range scan.categories {
		idx.byCode[category.Code] = category
	}
	return idx
}

func (s *Store) dropStaleIndexesLocked(name string, keep fileKey) {
	for key := range s.indexes {
		if key.name == name && key != keep {
			delete(s.indexes, key)
		}
	}
}
