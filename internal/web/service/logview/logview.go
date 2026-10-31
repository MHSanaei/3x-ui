// Package logview serves read-only, bounded views of the log files in XUI_LOG_FOLDER:
// listing, paged search and aggregate statistics.
package logview

import (
	"cmp"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"github.com/mhsanaei/3x-ui/v3/internal/xray/geodata"
)

const (
	defaultLimit = 200
	maxLimit     = 1000
	maxTermBytes = 256
	// defaultBudget covers a whole Xray log: PruneXrayLogsJob truncates those at 64 MiB.
	defaultBudget      = 64 << 20
	defaultMaxDistinct = 50000
	maxConcurrentScans = 4
	topListSize        = 10

	maxTimelineBuckets = 120
	maxRawBuckets      = 4096
	// sinceSlack tolerates writers that log slightly out of order around the range's edge.
	sinceSlack = time.Minute

	uncategorized = "other"
)

var countryIdleRelease = 10 * time.Minute

var (
	ErrInvalidFile   = errors.New("invalid log file name")
	ErrFileNotFound  = errors.New("log file not found")
	ErrUnreadable    = errors.New("log file could not be read")
	ErrInvalidFilter = errors.New("invalid log filter")

	// XUI_LOG_FOLDER can double as the DB folder, so the extension allowlist is what keeps x-ui.db out.
	logFileNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.log(\.[0-9]+)?(\.gz)?$`)

	scanSlots = make(chan struct{}, maxConcurrentScans)
)

// LogFile describes one readable file in the log folder.
type LogFile struct {
	Name       string `json:"name" example:"3xui.log"`
	Size       int64  `json:"size" example:"1048576"`
	ModTime    int64  `json:"modTime" example:"1767225600000"`
	Compressed bool   `json:"compressed" example:"false"`
}

// LogExternalPath is an Xray log configured to a path this feature refuses to read.
type LogExternalPath struct {
	Kind      string `json:"kind" example:"access"`
	Path      string `json:"path" example:"./access.log"`
	Suggested string `json:"suggested" example:"/var/log/x-ui/access.log"`
}

// LogFileList is the listing of the log folder.
type LogFileList struct {
	Folder   string            `json:"folder" example:"/var/log/x-ui"`
	Files    []LogFile         `json:"files"`
	External []LogExternalPath `json:"external"`
}

// LogLine is one parsed line; fields the line did not contain are empty.
type LogLine struct {
	Offset    int64  `json:"offset" example:"52428"`
	Time      int64  `json:"time" example:"1767225600000"`
	Level     string `json:"level" example:"info"`
	IP        string `json:"ip" example:"203.0.113.9"`
	Country   string `json:"country" example:"DE"`
	User      string `json:"user" example:"alice@example.com"`
	Host      string `json:"host" example:"example.com"`
	Protocol  string `json:"protocol" example:"tcp"`
	Inbound   string `json:"inbound" example:"inbound-443"`
	Outbound  string `json:"outbound" example:"direct"`
	Status    string `json:"status" example:"accepted"`
	Category  string `json:"category" example:"app/proxyman/inbound"`
	Message   string `json:"message" example:"from 203.0.113.9:5555 accepted tcp:example.com:443 [inbound-443 >> direct]"`
	Raw       string `json:"raw" example:"2026/01/01 00:00:00.000000 from 203.0.113.9:5555 accepted tcp:example.com:443 [inbound-443 >> direct]"`
	Truncated bool   `json:"truncated" example:"false"`
}

// LogPage is one newest-first page of matching lines.
type LogPage struct {
	File            string    `json:"file" example:"3xui.log"`
	FileSize        int64     `json:"fileSize" example:"1048576"`
	Lines           []LogLine `json:"lines"`
	NextCursor      int64     `json:"nextCursor" example:"52428"`
	ScannedBytes    int64     `json:"scannedBytes" example:"65536"`
	BudgetExhausted bool      `json:"budgetExhausted" example:"false"`
}

// LogCount is one row of a top list.
type LogCount struct {
	Value string `json:"value" example:"203.0.113.9"`
	Count int    `json:"count" example:"42"`
}

// LogBucket is one point of the volume timeline.
type LogBucket struct {
	Start    int64 `json:"start" example:"1767225600000"`
	Total    int   `json:"total" example:"120"`
	Errors   int   `json:"errors" example:"3"`
	Warnings int   `json:"warnings" example:"7"`
}

// LogStats aggregates the lines matching a filter.
type LogStats struct {
	File               string      `json:"file" example:"3xui.log"`
	FileSize           int64       `json:"fileSize" example:"1048576"`
	ScannedBytes       int64       `json:"scannedBytes" example:"1048576"`
	BudgetExhausted    bool        `json:"budgetExhausted" example:"false"`
	ScannedLines       int         `json:"scannedLines" example:"9000"`
	MatchedLines       int         `json:"matchedLines" example:"8200"`
	Errors             int         `json:"errors" example:"12"`
	Warnings           int         `json:"warnings" example:"40"`
	Infos              int         `json:"infos" example:"8000"`
	Debugs             int         `json:"debugs" example:"100"`
	Unknown            int         `json:"unknown" example:"48"`
	UniqueIPs          int         `json:"uniqueIps" example:"312"`
	UniqueUsers        int         `json:"uniqueUsers" example:"25"`
	UniqueHosts        int         `json:"uniqueHosts" example:"1800"`
	UniqueCapped       bool        `json:"uniqueCapped" example:"false"`
	FirstTime          int64       `json:"firstTime" example:"1767225600000"`
	LastTime           int64       `json:"lastTime" example:"1767312000000"`
	BucketMillis       int64       `json:"bucketMillis" example:"60000"`
	Timeline           []LogBucket `json:"timeline"`
	TopIPs             []LogCount  `json:"topIps"`
	TopUsers           []LogCount  `json:"topUsers"`
	TopHosts           []LogCount  `json:"topHosts"`
	TopProtocols       []LogCount  `json:"topProtocols"`
	TopInbounds        []LogCount  `json:"topInbounds"`
	TopStatuses        []LogCount  `json:"topStatuses"`
	TopErrorCategories []LogCount  `json:"topErrorCategories"`
	GeoAvailable       bool        `json:"geoAvailable" example:"true"`
	UnlocatedLines     int         `json:"unlocatedLines" example:"14"`
	Countries          []LogCount  `json:"countries"`
}

// Filter narrows a read or a stats scan. Text terms are case-insensitive substrings;
// Levels is comma-separated; Since and Until are inclusive Unix milliseconds.
type Filter struct {
	Query    string `form:"query"`
	IP       string `form:"ip"`
	Country  string `form:"country"`
	User     string `form:"user"`
	Host     string `form:"host"`
	Inbound  string `form:"inbound"`
	Protocol string `form:"protocol"`
	Status   string `form:"status"`
	Levels   string `form:"levels"`
	Since    int64  `form:"since"`
	Until    int64  `form:"until"`
}

// ReadRequest asks for one page of a file; Cursor is a previous page's NextCursor, 0 for the newest.
type ReadRequest struct {
	File   string `form:"file"`
	Filter Filter
	Limit  int   `form:"limit"`
	Cursor int64 `form:"cursor"`
}

// StatsRequest asks for the aggregates of a file.
type StatsRequest struct {
	File   string `form:"file"`
	Filter Filter
}

// Service reads the log folder; the zero value uses the production limits.
type Service struct {
	budget      int64
	maxDistinct int
}

func (s *Service) scanBudget() int64 {
	if s.budget > 0 {
		return s.budget
	}
	return defaultBudget
}

func isCompressed(name string) bool {
	return strings.HasSuffix(name, ".gz")
}

// ListFiles returns the readable logs, most recently written first. A missing folder is an empty list.
func (s *Service) ListFiles() (*LogFileList, error) {
	folder := config.GetLogFolder()
	list := &LogFileList{Folder: folder, Files: []LogFile{}, External: externalXrayLogs(folder)}
	entries, err := os.ReadDir(folder)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return list, nil
		}
		logger.Warning("logview: list log folder:", err)
		return nil, ErrUnreadable
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !logFileNameRe.MatchString(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		list.Files = append(list.Files, LogFile{
			Name:       entry.Name(),
			Size:       info.Size(),
			ModTime:    info.ModTime().UnixMilli(),
			Compressed: isCompressed(entry.Name()),
		})
	}
	slices.SortStableFunc(list.Files, func(a, b LogFile) int {
		return cmp.Or(cmp.Compare(b.ModTime, a.ModTime), cmp.Compare(a.Name, b.Name))
	})
	return list, nil
}

// externalXrayLogs names Xray logs living outside the folder. Xray inherits the panel's
// working directory, so a relative path like the ./access.log preset resolves against it.
func externalXrayLogs(folder string) []LogExternalPath {
	out := []LogExternalPath{}
	absFolder, err := filepath.Abs(folder)
	if err != nil {
		return out
	}
	for _, source := range []struct {
		kind string
		path func() (string, error)
	}{{"access", xray.GetAccessLogPath}, {"error", xray.GetErrorLogPath}} {
		path, err := source.path()
		if err != nil || path == "" || path == "none" {
			continue
		}
		abs, err := filepath.Abs(path)
		if err != nil || filepath.Dir(abs) == absFolder {
			continue
		}
		out = append(out, LogExternalPath{Kind: source.kind, Path: path, Suggested: filepath.Join(absFolder, source.kind+".log")})
	}
	return out
}

// openLog is the only way this package reaches a file. os.Root confines the open to the
// folder; refusing symlinks stops a .log-named link reaching a non-log file inside it.
func openLog(name string) (*os.File, fs.FileInfo, error) {
	if len(name) > 255 || !logFileNameRe.MatchString(name) {
		return nil, nil, ErrInvalidFile
	}
	root, err := os.OpenRoot(config.GetLogFolder())
	if err != nil {
		return nil, nil, openError(err)
	}
	defer root.Close()
	before, err := root.Lstat(name)
	if err != nil {
		return nil, nil, openError(err)
	}
	if !before.Mode().IsRegular() {
		return nil, nil, ErrInvalidFile
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, nil, openError(err)
	}
	opened, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, openError(err)
	}
	// The Lstat check is only meaningful if the name still refers to that same file.
	if !os.SameFile(before, opened) {
		f.Close()
		return nil, nil, ErrInvalidFile
	}
	return f, opened, nil
}

// openError keeps absolute paths out of API responses, which os.PathError would otherwise carry.
func openError(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return ErrFileNotFound
	}
	logger.Warning("logview: open log:", err)
	return ErrUnreadable
}

func acquireScan(ctx context.Context) (func(), error) {
	select {
	case scanSlots <- struct{}{}:
		return func() { <-scanSlots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// matcher is a validated Filter with its terms lower-cased once rather than per line.
type matcher struct {
	query, ip, country, user, host, inbound, protocol, status string
	levels                                                    map[string]bool
	since, until                                              int64
}

func (f Filter) compile() (*matcher, error) {
	for _, term := range []string{f.Query, f.IP, f.Country, f.User, f.Host, f.Inbound, f.Protocol, f.Status} {
		if len(term) > maxTermBytes {
			return nil, fmt.Errorf("%w: a search term exceeds %d bytes", ErrInvalidFilter, maxTermBytes)
		}
	}
	if f.Since < 0 || f.Until < 0 || (f.Until > 0 && f.Since > f.Until) {
		return nil, fmt.Errorf("%w: bad time range", ErrInvalidFilter)
	}
	m := &matcher{
		query:    strings.ToLower(f.Query),
		ip:       strings.ToLower(f.IP),
		country:  strings.ToLower(f.Country),
		user:     strings.ToLower(f.User),
		host:     strings.ToLower(f.Host),
		inbound:  strings.ToLower(f.Inbound),
		protocol: strings.ToLower(f.Protocol),
		status:   strings.ToLower(f.Status),
		since:    f.Since,
		until:    f.Until,
	}
	if f.Levels != "" {
		m.levels = make(map[string]bool)
		for level := range strings.SplitSeq(f.Levels, ",") {
			switch level {
			case LevelError, LevelWarning, LevelInfo, LevelDebug:
				m.levels[level] = true
			case LevelUnknown:
				m.levels[""] = true
			default:
				return nil, fmt.Errorf("%w: unknown level", ErrInvalidFilter)
			}
		}
	}
	return m, nil
}

func fieldHas(value, term string) bool {
	return term == "" || strings.Contains(strings.ToLower(value), term)
}

// matches excludes unstamped lines from a time range: nothing proves they fall inside it.
func (m *matcher) matches(l *LogLine) bool {
	if m.since > 0 && (l.Time == 0 || l.Time < m.since) {
		return false
	}
	if m.until > 0 && (l.Time == 0 || l.Time > m.until) {
		return false
	}
	if m.levels != nil && !m.levels[l.Level] {
		return false
	}
	return fieldHas(l.IP, m.ip) &&
		fieldHas(l.Country, m.country) &&
		fieldHas(l.User, m.user) &&
		fieldHas(l.Host, m.host) &&
		fieldHas(l.Inbound, m.inbound) &&
		fieldHas(l.Protocol, m.protocol) &&
		(fieldHas(l.Status, m.status) || fieldHas(l.Message, m.status)) &&
		fieldHas(l.Raw, m.query)
}

func (m *matcher) olderThanRange(l *LogLine) bool {
	return m.since > 0 && l.Time > 0 && l.Time < m.since-sinceSlack.Milliseconds()
}

// isInternalAPILine hides the panel's own gRPC polling of Xray, as the dashboard's access log does.
func isInternalAPILine(line string) bool {
	return strings.Contains(line, "api -> api") || strings.Contains(line, "api >> api")
}

// countryCache keeps the table of the current geoip.dat; a panel update swaps the file,
// which changes the key. A nil index is remembered too, so a broken file parses only once.
type countryCache struct {
	mu        sync.Mutex
	key       countryCacheKey
	index     *geodata.CountryIndex
	idleTimer *time.Timer
}

type countryCacheKey struct {
	dir     string
	size    int64
	modTime time.Time
}

var countries countryCache

// get returns nil when the database is absent or unusable: logs stay readable without it.
func (c *countryCache) get() *geodata.CountryIndex {
	dir := xray.AssetDir()
	info, err := os.Stat(filepath.Join(dir, geodata.CountryFileName))
	if err != nil {
		return nil
	}
	key := countryCacheKey{dir: dir, size: info.Size(), modTime: info.ModTime()}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keepAliveLocked()
	if key == c.key {
		return c.index
	}
	index, err := geodata.LoadCountryIndex(dir, geodata.CountryFileName)
	if err != nil {
		logger.Warning("logview: load country database:", err)
		index = nil
	}
	c.key, c.index = key, index
	return index
}

// keepAliveLocked lets go of the table once the page is closed: its ~13 MB is only
// worth holding while someone is looking, and a reload costs a fraction of a second.
func (c *countryCache) keepAliveLocked() {
	if c.idleTimer != nil {
		c.idleTimer.Reset(countryIdleRelease)
		return
	}
	c.idleTimer = time.AfterFunc(countryIdleRelease, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.key, c.index, c.idleTimer = countryCacheKey{}, nil, nil
	})
}

// visibleLine parses a raw line, or reports false for one no view should show.
func visibleLine(raw []byte, offset int64, truncated bool, located *geodata.CountryIndex) (LogLine, bool) {
	text := string(raw)
	if isInternalAPILine(text) {
		return LogLine{}, false
	}
	l := parseLine(text)
	l.Offset = offset
	l.Truncated = truncated
	if located != nil && l.IP != "" {
		if addr, err := netip.ParseAddr(l.IP); err == nil {
			l.Country = located.Country(addr)
		}
	}
	return l, true
}

// Read returns one page of matching lines, newest first. A page can be short when the
// scan budget runs out first: BudgetExhausted is set and NextCursor continues the search.
func (s *Service) Read(ctx context.Context, req ReadRequest) (*LogPage, error) {
	m, err := req.Filter.compile()
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)

	f, info, err := openLog(req.File)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	release, err := acquireScan(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	page := &LogPage{File: req.File, FileSize: info.Size(), Lines: []LogLine{}}
	if isCompressed(req.File) {
		err = s.readCompressed(ctx, f, m, limit, req.Cursor, page)
	} else {
		err = s.readPlain(ctx, f, info.Size(), m, limit, req.Cursor, page)
	}
	if err != nil {
		return nil, scanError(err)
	}
	return page, nil
}

func (s *Service) readPlain(ctx context.Context, r io.ReaderAt, size int64, m *matcher, limit int, cursor int64, page *LogPage) error {
	located := countries.get()
	end := cursor
	// A cursor past the end predates a rotation or truncation, so it restarts at the newest line.
	if end <= 0 || end > size {
		end = size
	}
	reachedRangeStart := false
	out, err := scanBackward(ctx, r, end, s.scanBudget(), func(raw []byte, offset int64, truncated bool) bool {
		l, ok := visibleLine(raw, offset, truncated, located)
		if !ok {
			return true
		}
		if m.olderThanRange(&l) {
			reachedRangeStart = true
			return false
		}
		if !m.matches(&l) {
			return true
		}
		page.Lines = append(page.Lines, l)
		return len(page.Lines) < limit
	})
	if err != nil {
		return err
	}
	page.ScannedBytes = out.scanned
	page.BudgetExhausted = out.exhausted
	page.NextCursor = out.next
	if reachedRangeStart {
		page.NextCursor = 0
	}
	return nil
}

// readCompressed keeps a ring of the last limit matches: gzip reads only forward,
// and the newest lines of the requested span are the ones this page has to return.
func (s *Service) readCompressed(ctx context.Context, r io.Reader, m *matcher, limit int, cursor int64, page *LogPage) error {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer zr.Close()
	located := countries.get()
	ring := make([]LogLine, 0, limit)
	head := 0
	overflowed := false
	out, err := scanForward(ctx, zr, s.scanBudget(), func(raw []byte, offset int64, truncated bool) bool {
		if cursor > 0 && offset >= cursor {
			return false
		}
		l, ok := visibleLine(raw, offset, truncated, located)
		if !ok || !m.matches(&l) {
			return true
		}
		if len(ring) < limit {
			ring = append(ring, l)
			return true
		}
		ring[head] = l
		head = (head + 1) % limit
		overflowed = true
		return true
	})
	if err != nil {
		return err
	}
	for i := len(ring) - 1; i >= 0; i-- {
		page.Lines = append(page.Lines, ring[(head+i)%len(ring)])
	}
	page.ScannedBytes = out.scanned
	page.BudgetExhausted = out.exhausted
	if overflowed {
		page.NextCursor = ring[head].Offset
	}
	return nil
}

// scanError passes cancellation through and hides paths and decoder details from the API.
func scanError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	logger.Warning("logview: scan log:", err)
	return ErrUnreadable
}

// Stats aggregates the matching lines of a file up to the scan budget: newest first, except gzip.
func (s *Service) Stats(ctx context.Context, req StatsRequest) (*LogStats, error) {
	m, err := req.Filter.compile()
	if err != nil {
		return nil, err
	}
	f, info, err := openLog(req.File)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	release, err := acquireScan(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	maxDistinct := s.maxDistinct
	if maxDistinct <= 0 {
		maxDistinct = defaultMaxDistinct
	}
	c := newCollector(maxDistinct)
	located := countries.get()
	c.stats.GeoAvailable = located != nil
	var out scanOutcome
	if isCompressed(req.File) {
		zr, zerr := gzip.NewReader(f)
		if zerr != nil {
			return nil, scanError(zerr)
		}
		defer zr.Close()
		out, err = scanForward(ctx, zr, s.scanBudget(), func(raw []byte, offset int64, truncated bool) bool {
			if l, ok := visibleLine(raw, offset, truncated, located); ok {
				c.add(&l, m)
			}
			return true
		})
	} else {
		out, err = scanBackward(ctx, f, info.Size(), s.scanBudget(), func(raw []byte, offset int64, truncated bool) bool {
			l, ok := visibleLine(raw, offset, truncated, located)
			if !ok {
				return true
			}
			if m.olderThanRange(&l) {
				return false
			}
			c.add(&l, m)
			return true
		})
	}
	if err != nil {
		return nil, scanError(err)
	}
	stats := c.build(m, time.Now().UnixMilli())
	stats.File = req.File
	stats.FileSize = info.Size()
	stats.ScannedBytes = out.scanned
	stats.BudgetExhausted = out.exhausted
	return stats, nil
}

// counter tallies distinct values up to a cap, because destinations come from clients.
type counter struct {
	counts map[string]int
	limit  int
	capped bool
}

func newCounter(limit int) *counter {
	return &counter{counts: make(map[string]int), limit: limit}
}

func (c *counter) add(value string) {
	if value == "" {
		return
	}
	if _, seen := c.counts[value]; seen {
		c.counts[value]++
		return
	}
	if len(c.counts) >= c.limit {
		c.capped = true
		return
	}
	// A key sliced from the line would pin the whole line's memory for the scan.
	c.counts[strings.Clone(value)] = 1
}

func (c *counter) top(n int) []LogCount {
	out := make([]LogCount, 0, len(c.counts))
	for value, count := range c.counts {
		out = append(out, LogCount{Value: value, Count: count})
	}
	slices.SortFunc(out, func(a, b LogCount) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Value, b.Value))
	})
	return out[:min(n, len(out))]
}

type bucketCounts struct{ total, errors, warnings int }

type collector struct {
	stats                                                            LogStats
	ips, users, hosts, protocols, inbounds, statuses, cat, countries *counter
	step                                                             int64
	buckets                                                          map[int64]bucketCounts
}

func newCollector(maxDistinct int) *collector {
	return &collector{
		ips:       newCounter(maxDistinct),
		countries: newCounter(maxDistinct),
		users:     newCounter(maxDistinct),
		hosts:     newCounter(maxDistinct),
		protocols: newCounter(maxDistinct),
		inbounds:  newCounter(maxDistinct),
		statuses:  newCounter(maxDistinct),
		cat:       newCounter(maxDistinct),
		step:      time.Minute.Milliseconds(),
		buckets:   make(map[int64]bucketCounts),
	}
}

func (c *collector) add(l *LogLine, m *matcher) {
	c.stats.ScannedLines++
	if !m.matches(l) {
		return
	}
	c.stats.MatchedLines++
	switch l.Level {
	case LevelError:
		c.stats.Errors++
	case LevelWarning:
		c.stats.Warnings++
	case LevelInfo:
		c.stats.Infos++
	case LevelDebug:
		c.stats.Debugs++
	default:
		c.stats.Unknown++
	}
	c.ips.add(l.IP)
	c.countries.add(l.Country)
	if l.IP != "" && l.Country == "" && c.stats.GeoAvailable {
		c.stats.UnlocatedLines++
	}
	c.users.add(l.User)
	c.hosts.add(l.Host)
	c.protocols.add(l.Protocol)
	c.inbounds.add(l.Inbound)
	c.statuses.add(l.Status)
	if l.Level == LevelError || l.Level == LevelWarning {
		c.cat.add(cmp.Or(l.Category, uncategorized))
	}
	if l.Time == 0 {
		return
	}
	if c.stats.FirstTime == 0 || l.Time < c.stats.FirstTime {
		c.stats.FirstTime = l.Time
	}
	if l.Time > c.stats.LastTime {
		c.stats.LastTime = l.Time
	}
	key := floorTo(l.Time, c.step)
	b := c.buckets[key]
	b.total++
	switch l.Level {
	case LevelError:
		b.errors++
	case LevelWarning:
		b.warnings++
	}
	c.buckets[key] = b
	if len(c.buckets) > maxRawBuckets {
		c.step = nextStep(c.step)
		c.buckets = mergeBuckets(c.buckets, c.step)
	}
}

func floorTo(value, step int64) int64 {
	return value - value%step
}

var niceSteps = []time.Duration{
	time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute, time.Hour, 3 * time.Hour,
	6 * time.Hour, 12 * time.Hour, 24 * time.Hour, 48 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour,
}

func nextStep(step int64) int64 {
	for _, nice := range niceSteps {
		if nice.Milliseconds() > step {
			return nice.Milliseconds()
		}
	}
	return step * 2
}

func mergeBuckets(buckets map[int64]bucketCounts, step int64) map[int64]bucketCounts {
	merged := make(map[int64]bucketCounts, len(buckets))
	for start, b := range buckets {
		key := floorTo(start, step)
		sum := merged[key]
		sum.total += b.total
		sum.errors += b.errors
		sum.warnings += b.warnings
		merged[key] = sum
	}
	return merged
}

func (c *collector) build(m *matcher, now int64) *LogStats {
	stats := &c.stats
	stats.UniqueIPs = len(c.ips.counts)
	stats.UniqueUsers = len(c.users.counts)
	stats.UniqueHosts = len(c.hosts.counts)
	stats.UniqueCapped = c.ips.capped || c.users.capped || c.hosts.capped
	stats.TopIPs = c.ips.top(topListSize)
	stats.TopUsers = c.users.top(topListSize)
	stats.TopHosts = c.hosts.top(topListSize)
	stats.TopProtocols = c.protocols.top(topListSize)
	stats.TopInbounds = c.inbounds.top(topListSize)
	stats.TopStatuses = c.statuses.top(topListSize)
	stats.TopErrorCategories = c.cat.top(topListSize)
	stats.Countries = c.countries.top(len(c.countries.counts))
	stats.Timeline, stats.BucketMillis = c.timeline(m, now)
	return stats
}

// timeline is dense, zero-filled and spans the requested range when there is one, so
// quiet periods read as quiet instead of vanishing from the axis.
func (c *collector) timeline(m *matcher, now int64) ([]LogBucket, int64) {
	start := cmp.Or(m.since, c.stats.FirstTime)
	end := m.until
	if end == 0 {
		end = c.stats.LastTime
		if m.since > 0 {
			end = max(end, now)
		}
	}
	if start == 0 || end < start {
		return []LogBucket{}, c.step
	}
	step := c.step
	for (end-start)/step >= maxTimelineBuckets {
		step = nextStep(step)
	}
	merged := mergeBuckets(c.buckets, step)
	out := make([]LogBucket, 0, (end-start)/step+2)
	for at := floorTo(start, step); at <= end; at += step {
		b := merged[at]
		out = append(out, LogBucket{Start: at, Total: b.total, Errors: b.errors, Warnings: b.warnings})
	}
	return out, step
}
