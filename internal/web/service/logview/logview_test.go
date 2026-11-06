package logview

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	xraygeodata "github.com/xtls/xray-core/common/geodata"
	"google.golang.org/protobuf/proto"

	"github.com/mhsanaei/3x-ui/v3/internal/xray/geodata"
)

func logFolder(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XUI_LOG_FOLDER", dir)
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	return dir
}

func writeLog(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o640); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func writeGzipLog(t *testing.T, dir, name, content string) {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(content)); err != nil {
		t.Fatalf("gzip %s: %v", name, err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o640); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func numberedLines(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line-%03d\n", i)
	}
	return b.String()
}

func rawOf(page *LogPage) []string {
	out := make([]string, 0, len(page.Lines))
	for _, l := range page.Lines {
		out = append(out, l.Raw)
	}
	return out
}

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlinks need a privilege on Windows: %v", err)
		}
		t.Fatalf("symlink: %v", err)
	}
}

func TestListFiles(t *testing.T) {
	dir := logFolder(t)
	writeLog(t, dir, "3xui.log", "a\n")
	writeLog(t, dir, "access.log", "bb\n")
	writeLog(t, dir, "x-ui.db", "secret database")
	writeLog(t, dir, ".hidden.log", "x")
	writeGzipLog(t, dir, "3xui-2026-01-01T00-00-00.000.log.gz", "old\n")
	if err := os.Mkdir(filepath.Join(dir, "dir.log"), 0o750); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "3xui.log"), old, old); err != nil {
		t.Fatal(err)
	}

	list, err := (&Service{}).ListFiles()
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	names := make([]string, 0, len(list.Files))
	for _, f := range list.Files {
		names = append(names, f.Name)
	}
	slices.Sort(names)
	want := []string{"3xui-2026-01-01T00-00-00.000.log.gz", "3xui.log", "access.log"}
	if !slices.Equal(names, want) {
		t.Fatalf("listed %v, want %v", names, want)
	}
	if list.Files[len(list.Files)-1].Name != "3xui.log" {
		t.Errorf("least recently written file should sort last, got %q", list.Files[len(list.Files)-1].Name)
	}
	for _, f := range list.Files {
		if got, want := f.Compressed, strings.HasSuffix(f.Name, ".gz"); got != want {
			t.Errorf("%s compressed = %v, want %v", f.Name, got, want)
		}
	}
}

func TestListFilesWithoutFolderIsEmpty(t *testing.T) {
	t.Setenv("XUI_LOG_FOLDER", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	list, err := (&Service{}).ListFiles()
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if list.Files == nil || len(list.Files) != 0 {
		t.Errorf("Files = %#v, want a non-nil empty slice so the JSON stays an array", list.Files)
	}
}

// The Xray presets write beside the panel, not into XUI_LOG_FOLDER, so the page
// has to say why traffic logs are absent instead of showing an unexplained void.
func TestListFilesReportsXrayLogsOutsideFolder(t *testing.T) {
	dir := logFolder(t)
	bin := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", bin)
	cfg := fmt.Sprintf(`{"log":{"access":"./access.log","error":%q}}`, filepath.Join(dir, "error.log"))
	if err := os.WriteFile(filepath.Join(bin, "config.json"), []byte(cfg), 0o640); err != nil {
		t.Fatal(err)
	}

	list, err := (&Service{}).ListFiles()
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	want := []LogExternalPath{{Kind: "access", Path: "./access.log", Suggested: filepath.Join(dir, "access.log")}}
	if !slices.Equal(list.External, want) {
		t.Errorf("External = %+v, want %+v", list.External, want)
	}
}

func TestReadRejectsUnsafeNames(t *testing.T) {
	dir := logFolder(t)
	outside := filepath.Join(filepath.Dir(dir), "outside.log")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })
	writeLog(t, dir, "x-ui.db", "secret\n")
	writeLog(t, dir, ".hidden.log", "secret\n")

	names := []string{
		"", ".", "..", "../outside.log", `..\outside.log`, "sub/../../outside.log", outside,
		"/etc/passwd", "x-ui.db", ".hidden.log", "a.log/../x-ui.db", "3xui.log:ads", "3xui.log\x00.txt", "3xui.logs",
		strings.Repeat("a", 300) + ".log",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			_, err := (&Service{}).Read(context.Background(), ReadRequest{File: name})
			if !errors.Is(err, ErrInvalidFile) {
				t.Errorf("Read(%q) error = %v, want ErrInvalidFile", name, err)
			}
		})
	}
}

// A .log-named symlink would otherwise launder any file through the name allowlist.
func TestSymlinksAreNeitherListedNorRead(t *testing.T) {
	dir := logFolder(t)
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("secret\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	writeLog(t, dir, "x-ui.db", "database\n")
	symlinkOrSkip(t, secret, filepath.Join(dir, "escape.log"))
	symlinkOrSkip(t, filepath.Join(dir, "x-ui.db"), filepath.Join(dir, "inside.log"))
	symlinkOrSkip(t, filepath.Join(dir, "missing"), filepath.Join(dir, "dangling.log"))

	list, err := (&Service{}).ListFiles()
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(list.Files) != 0 {
		t.Errorf("listed symlinks: %+v", list.Files)
	}
	for _, name := range []string{"escape.log", "inside.log", "dangling.log"} {
		if _, err := (&Service{}).Read(context.Background(), ReadRequest{File: name}); !errors.Is(err, ErrInvalidFile) {
			t.Errorf("Read(%q) error = %v, want ErrInvalidFile", name, err)
		}
		if _, err := (&Service{}).Stats(context.Background(), StatsRequest{File: name}); !errors.Is(err, ErrInvalidFile) {
			t.Errorf("Stats(%q) error = %v, want ErrInvalidFile", name, err)
		}
	}
}

func TestReadMissingFile(t *testing.T) {
	logFolder(t)
	if _, err := (&Service{}).Read(context.Background(), ReadRequest{File: "gone.log"}); !errors.Is(err, ErrFileNotFound) {
		t.Errorf("error = %v, want ErrFileNotFound", err)
	}
	t.Setenv("XUI_LOG_FOLDER", filepath.Join(t.TempDir(), "missing"))
	if _, err := (&Service{}).Read(context.Background(), ReadRequest{File: "gone.log"}); !errors.Is(err, ErrFileNotFound) {
		t.Errorf("missing folder error = %v, want ErrFileNotFound", err)
	}
}

func TestReadPagesNewestFirstWithStableCursor(t *testing.T) {
	dir := logFolder(t)
	writeLog(t, dir, "a.log", numberedLines(5))
	svc := &Service{}

	first, err := svc.Read(context.Background(), ReadRequest{File: "a.log", Limit: 2})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if got, want := rawOf(first), []string{"line-005", "line-004"}; !slices.Equal(got, want) {
		t.Fatalf("first page %v, want %v", got, want)
	}

	// Lines appended between pages must neither shift nor repeat the next page.
	f, err := os.OpenFile(filepath.Join(dir, "a.log"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("line-006\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	second, err := svc.Read(context.Background(), ReadRequest{File: "a.log", Limit: 2, Cursor: first.NextCursor})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if got, want := rawOf(second), []string{"line-003", "line-002"}; !slices.Equal(got, want) {
		t.Fatalf("second page %v, want %v", got, want)
	}

	third, err := svc.Read(context.Background(), ReadRequest{File: "a.log", Limit: 2, Cursor: second.NextCursor})
	if err != nil {
		t.Fatalf("third page: %v", err)
	}
	if got, want := rawOf(third), []string{"line-001"}; !slices.Equal(got, want) {
		t.Fatalf("third page %v, want %v", got, want)
	}
	if third.NextCursor != 0 {
		t.Errorf("NextCursor = %d at the start of the file, want 0", third.NextCursor)
	}
}

// A cursor from before a rotation or truncation points past the new end of the file.
func TestReadCursorBeyondEndStartsFromNewest(t *testing.T) {
	dir := logFolder(t)
	writeLog(t, dir, "a.log", numberedLines(2))
	page, err := (&Service{}).Read(context.Background(), ReadRequest{File: "a.log", Cursor: 1 << 40})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got, want := rawOf(page), []string{"line-002", "line-001"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestReadHandlesChunkBoundariesAndCRLF(t *testing.T) {
	dir := logFolder(t)
	// Lines straddle the 64 KiB read chunks, so a line must be stitched across two reads.
	var b strings.Builder
	var want []string
	for i := 0; b.Len() < 3*chunkSize; i++ {
		line := fmt.Sprintf("%06d-%s", i, strings.Repeat("x", 977))
		want = append(want, line)
		b.WriteString(line + "\r\n")
	}
	writeLog(t, dir, "a.log", b.String())
	slices.Reverse(want)

	page, err := (&Service{}).Read(context.Background(), ReadRequest{File: "a.log", Limit: maxLimit})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !slices.Equal(rawOf(page), want) {
		t.Fatalf("got %d lines, want %d, or content differs", len(page.Lines), len(want))
	}
	for i := 1; i < len(page.Lines); i++ {
		if page.Lines[i].Offset >= page.Lines[i-1].Offset {
			t.Fatalf("offsets not descending at %d: %d >= %d", i, page.Lines[i].Offset, page.Lines[i-1].Offset)
		}
	}
}

func TestReadTruncatesOversizedLineKeepingItsHead(t *testing.T) {
	dir := logFolder(t)
	huge := "HEAD" + strings.Repeat("x", 5*maxLineBytes) + "TAIL"
	writeLog(t, dir, "plain.log", "before\n"+huge+"\nafter\n")
	writeGzipLog(t, dir, "packed.log.gz", "before\n"+huge+"\nafter\n")

	for _, name := range []string{"plain.log", "packed.log.gz"} {
		t.Run(name, func(t *testing.T) {
			page, err := (&Service{}).Read(context.Background(), ReadRequest{File: name})
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if len(page.Lines) != 3 {
				t.Fatalf("got %d lines, want 3", len(page.Lines))
			}
			big := page.Lines[1]
			if !big.Truncated || len(big.Raw) != maxLineBytes || !strings.HasPrefix(big.Raw, "HEAD") {
				t.Errorf("truncated=%v len=%d prefix=%q, want true/%d/HEAD", big.Truncated, len(big.Raw), big.Raw[:4], maxLineBytes)
			}
			if page.Lines[0].Raw != "after" || page.Lines[2].Raw != "before" || page.Lines[0].Truncated {
				t.Errorf("neighbouring lines damaged: %q, %q", page.Lines[0].Raw, page.Lines[2].Raw)
			}
		})
	}
}

func TestReadGzipPagesLikeAPlainFile(t *testing.T) {
	dir := logFolder(t)
	writeGzipLog(t, dir, "a.log.gz", numberedLines(5))
	svc := &Service{}

	first, err := svc.Read(context.Background(), ReadRequest{File: "a.log.gz", Limit: 2})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if got, want := rawOf(first), []string{"line-005", "line-004"}; !slices.Equal(got, want) {
		t.Fatalf("first page %v, want %v", got, want)
	}
	if first.NextCursor == 0 {
		t.Fatal("NextCursor = 0 while older lines remain")
	}
	second, err := svc.Read(context.Background(), ReadRequest{File: "a.log.gz", Limit: 3, Cursor: first.NextCursor})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if got, want := rawOf(second), []string{"line-003", "line-002", "line-001"}; !slices.Equal(got, want) {
		t.Fatalf("second page %v, want %v", got, want)
	}
	if second.NextCursor != 0 {
		t.Errorf("NextCursor = %d at the start, want 0", second.NextCursor)
	}
}

func TestReadCorruptGzipReportsUnreadable(t *testing.T) {
	dir := logFolder(t)
	writeLog(t, dir, "a.log.gz", "this is not gzip")
	if _, err := (&Service{}).Read(context.Background(), ReadRequest{File: "a.log.gz"}); !errors.Is(err, ErrUnreadable) {
		t.Errorf("error = %v, want ErrUnreadable", err)
	}
}

// The budget caps the work one request can do; the cursor has to let the next
// request resume exactly where it stopped, without losing or repeating a line.
func TestReadBudgetStopsAndResumesWithoutLosingLines(t *testing.T) {
	dir := logFolder(t)
	// Long lines, so the budget rather than the page limit is what ends each request.
	const total = 4000
	var b strings.Builder
	for i := range total {
		fmt.Fprintf(&b, "line-%05d-%s\n", i, strings.Repeat("z", 200))
	}
	writeLog(t, dir, "a.log", b.String())
	svc := &Service{budget: chunkSize}

	seen := 0
	cursor := int64(0)
	pages := 0
	for {
		page, err := svc.Read(context.Background(), ReadRequest{File: "a.log", Filter: Filter{Query: "line-"}, Limit: maxLimit, Cursor: cursor})
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		pages++
		seen += len(page.Lines)
		if page.ScannedBytes > 2*chunkSize {
			t.Fatalf("scanned %d bytes on a %d budget", page.ScannedBytes, chunkSize)
		}
		if page.NextCursor == 0 {
			break
		}
		cursor = page.NextCursor
		if pages > total {
			t.Fatal("cursor is not advancing")
		}
	}
	if seen != total {
		t.Errorf("saw %d lines across %d pages, want %d", seen, pages, total)
	}
}

func TestReadBudgetReportsExhaustionOnRareMatch(t *testing.T) {
	dir := logFolder(t)
	writeLog(t, dir, "a.log", numberedLines(40000))
	page, err := (&Service{budget: chunkSize}).Read(context.Background(), ReadRequest{File: "a.log", Filter: Filter{Query: "no such text"}})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(page.Lines) != 0 || !page.BudgetExhausted || page.NextCursor == 0 {
		t.Errorf("lines=%d exhausted=%v next=%d, want 0/true/non-zero so the UI can keep searching", len(page.Lines), page.BudgetExhausted, page.NextCursor)
	}
}

func TestReadBudgetCannotStallOnALineLongerThanTheBudget(t *testing.T) {
	dir := logFolder(t)
	writeLog(t, dir, "a.log", "first\n"+strings.Repeat("y", 3*chunkSize))
	svc := &Service{budget: chunkSize}
	cursor := int64(0)
	for range 10 {
		page, err := svc.Read(context.Background(), ReadRequest{File: "a.log", Cursor: cursor})
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		for _, l := range page.Lines {
			if l.Raw == "first" {
				return
			}
		}
		if page.NextCursor == 0 {
			t.Fatal("reached the start without seeing the first line")
		}
		if cursor != 0 && page.NextCursor >= cursor {
			t.Fatalf("cursor stalled at %d", cursor)
		}
		cursor = page.NextCursor
	}
	t.Fatal("did not reach the first line in 10 pages")
}

func accessLine(at time.Time, ip, user, host, inbound, network string) string {
	return fmt.Sprintf("%s from %s:1000 accepted %s:%s:443 [%s >> direct] email: %s",
		at.Format("2006/01/02 15:04:05.000000"), ip, network, host, inbound, user)
}

func TestReadFilters(t *testing.T) {
	dir := logFolder(t)
	base := time.Date(2026, 3, 4, 10, 0, 0, 0, time.Local)
	lines := []string{
		accessLine(base, "203.0.113.1", "alice@x", "a.example", "in-1", "tcp"),
		accessLine(base.Add(time.Minute), "203.0.113.2", "bob@x", "b.example", "in-2", "udp"),
		base.Add(2*time.Minute).Format("2006/01/02 15:04:05") + " [Error] [77] proxy/trojan/inbound: handshake Failed",
		base.Add(3*time.Minute).Format("2006/01/02 15:04:05") + " [Warning] [78] app/dns: slow lookup",
		"unstamped stack frame",
	}
	writeLog(t, dir, "a.log", strings.Join(lines, "\n")+"\n")

	cases := []struct {
		name   string
		filter Filter
		want   []int
	}{
		{"no filter", Filter{}, []int{4, 3, 2, 1, 0}},
		{"ip partial", Filter{IP: "113.2"}, []int{1}},
		{"user is case-insensitive", Filter{User: "ALICE"}, []int{0}},
		{"host", Filter{Host: "b.exam"}, []int{1}},
		{"inbound", Filter{Inbound: "in-1"}, []int{0}},
		{"protocol network", Filter{Protocol: "UDP"}, []int{1}},
		{"protocol from xray component", Filter{Protocol: "trojan"}, []int{2}},
		{"status", Filter{Status: "accepted"}, []int{1, 0}},
		{"status also searches the error text", Filter{Status: "handshake failed"}, []int{2}},
		{"free text covers unstructured lines", Filter{Query: "STACK"}, []int{4}},
		{"error only", Filter{Levels: "error"}, []int{2}},
		{"warn only", Filter{Levels: "warning"}, []int{3}},
		{"info only", Filter{Levels: "info"}, []int{1, 0}},
		{"several levels", Filter{Levels: "error,warning"}, []int{3, 2}},
		{"lines without a level", Filter{Levels: LevelUnknown}, []int{4}},
		{"since drops older and unstamped lines", Filter{Since: base.Add(2 * time.Minute).UnixMilli()}, []int{3, 2}},
		{"until drops newer lines", Filter{Until: base.Add(time.Minute).UnixMilli()}, []int{1, 0}},
		{"range and field combine", Filter{Since: base.UnixMilli(), Until: base.Add(time.Hour).UnixMilli(), User: "bob"}, []int{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := (&Service{}).Read(context.Background(), ReadRequest{File: "a.log", Filter: tc.filter})
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			want := make([]string, 0, len(tc.want))
			for _, i := range tc.want {
				want = append(want, lines[i])
			}
			if got := rawOf(page); !slices.Equal(got, want) {
				t.Errorf("got %q\nwant %q", got, want)
			}
		})
	}
}

func TestRejectsInvalidFilter(t *testing.T) {
	dir := logFolder(t)
	writeLog(t, dir, "a.log", "x\n")
	cases := []struct {
		name   string
		filter Filter
	}{
		{"unknown level", Filter{Levels: "error,loud"}},
		{"inverted range", Filter{Since: 2000, Until: 1000}},
		{"negative time", Filter{Since: -1}},
		{"oversized term", Filter{Query: strings.Repeat("q", maxTermBytes+1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := (&Service{}).Read(context.Background(), ReadRequest{File: "a.log", Filter: tc.filter}); !errors.Is(err, ErrInvalidFilter) {
				t.Errorf("Read error = %v, want ErrInvalidFilter", err)
			}
			if _, err := (&Service{}).Stats(context.Background(), StatsRequest{File: "a.log", Filter: tc.filter}); !errors.Is(err, ErrInvalidFilter) {
				t.Errorf("Stats error = %v, want ErrInvalidFilter", err)
			}
		})
	}
}

func TestReadLimitIsClamped(t *testing.T) {
	dir := logFolder(t)
	writeLog(t, dir, "a.log", numberedLines(maxLimit+50))
	for _, tc := range []struct{ limit, want int }{{0, defaultLimit}, {-5, defaultLimit}, {3, 3}, {maxLimit * 10, maxLimit}} {
		page, err := (&Service{}).Read(context.Background(), ReadRequest{File: "a.log", Limit: tc.limit})
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if len(page.Lines) != tc.want {
			t.Errorf("limit %d returned %d lines, want %d", tc.limit, len(page.Lines), tc.want)
		}
	}
}

// A time-bounded view of a large file must not pay for the history before the range.
func TestReadStopsScanningOnceOlderThanSince(t *testing.T) {
	dir := logFolder(t)
	base := time.Date(2026, 3, 4, 10, 0, 0, 0, time.Local)
	var b strings.Builder
	for i := range 20000 {
		b.WriteString(accessLine(base.Add(time.Duration(i)*time.Second), "203.0.113.1", "a", "a.example", "in", "tcp") + "\n")
	}
	writeLog(t, dir, "a.log", b.String())

	since := base.Add(19990 * time.Second)
	page, err := (&Service{}).Read(context.Background(), ReadRequest{File: "a.log", Filter: Filter{Since: since.UnixMilli(), User: "nobody"}})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if page.NextCursor != 0 || page.BudgetExhausted {
		t.Errorf("next=%d exhausted=%v, want 0/false: nothing older can match", page.NextCursor, page.BudgetExhausted)
	}
	if page.ScannedBytes >= int64(b.Len())/2 {
		t.Errorf("scanned %d of %d bytes for a 10-second range", page.ScannedBytes, b.Len())
	}
}

func TestReadHidesInternalAPITraffic(t *testing.T) {
	dir := logFolder(t)
	writeLog(t, dir, "a.log", strings.Join([]string{
		"2026/03/04 05:06:07 from 127.0.0.1:5 accepted tcp:127.0.0.1:62789 [api -> api]",
		"2026/03/04 05:06:07 from 127.0.0.1:5 accepted tcp:127.0.0.1:62789 [api >> api]",
		"2026/03/04 05:06:08 from 203.0.113.1:5 accepted tcp:a.example:443 [in >> direct]",
	}, "\n")+"\n")
	page, err := (&Service{}).Read(context.Background(), ReadRequest{File: "a.log"})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(page.Lines) != 1 || page.Lines[0].Host != "a.example" {
		t.Errorf("got %q, want only the user connection", rawOf(page))
	}
}

func TestReadCancelledContextStops(t *testing.T) {
	dir := logFolder(t)
	writeLog(t, dir, "a.log", numberedLines(100))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (&Service{}).Read(ctx, ReadRequest{File: "a.log"}); !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestStatsAggregates(t *testing.T) {
	dir := logFolder(t)
	base := time.Date(2026, 3, 4, 10, 0, 0, 0, time.Local)
	stamp := func(d time.Duration) string { return base.Add(d).Format("2006/01/02 15:04:05") }
	lines := []string{
		accessLine(base, "203.0.113.1", "alice", "a.example", "in-1", "tcp"),
		accessLine(base.Add(time.Minute), "203.0.113.1", "alice", "a.example", "in-1", "tcp"),
		accessLine(base.Add(2*time.Minute), "203.0.113.2", "bob", "b.example", "in-2", "udp"),
		stamp(3*time.Minute) + " from 203.0.113.3:1 rejected proxy/vless/encoding: bad user",
		stamp(4*time.Minute) + " [Error] [7] app/dns: failed",
		stamp(5*time.Minute) + " [Error] [8] app/dns: failed again",
		stamp(6*time.Minute) + " [Warning] [9] transport/internet: slow",
		stamp(7*time.Minute) + " DEBUG - tick",
		"no structure at all",
	}
	writeLog(t, dir, "a.log", strings.Join(lines, "\n")+"\n")

	got, err := (&Service{}).Stats(context.Background(), StatsRequest{File: "a.log"})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if got.MatchedLines != 9 || got.Errors != 2 || got.Warnings != 2 || got.Infos != 3 || got.Debugs != 1 || got.Unknown != 1 {
		t.Errorf("counts matched=%d errors=%d warnings=%d infos=%d debugs=%d unknown=%d", got.MatchedLines, got.Errors, got.Warnings, got.Infos, got.Debugs, got.Unknown)
	}
	if got.UniqueIPs != 3 || got.UniqueUsers != 2 || got.UniqueHosts != 2 || got.UniqueCapped {
		t.Errorf("unique ips=%d users=%d hosts=%d capped=%v", got.UniqueIPs, got.UniqueUsers, got.UniqueHosts, got.UniqueCapped)
	}
	if got.FirstTime != base.UnixMilli() || got.LastTime != base.Add(7*time.Minute).UnixMilli() {
		t.Errorf("range %d..%d", got.FirstTime, got.LastTime)
	}
	checks := []struct {
		name string
		got  []LogCount
		want []LogCount
	}{
		{"ips", got.TopIPs, []LogCount{{"203.0.113.1", 2}, {"203.0.113.2", 1}, {"203.0.113.3", 1}}},
		{"users", got.TopUsers, []LogCount{{"alice", 2}, {"bob", 1}}},
		{"hosts", got.TopHosts, []LogCount{{"a.example", 2}, {"b.example", 1}}},
		{"protocols", got.TopProtocols, []LogCount{{"tcp", 2}, {"udp", 1}, {"vless", 1}}},
		{"inbounds", got.TopInbounds, []LogCount{{"in-1", 2}, {"in-2", 1}}},
		{"statuses", got.TopStatuses, []LogCount{{"accepted", 3}, {"rejected", 1}}},
		{"error categories", got.TopErrorCategories, []LogCount{{"app/dns", 2}, {"other", 1}, {"transport/internet", 1}}},
	}
	for _, c := range checks {
		if !slices.Equal(c.got, c.want) {
			t.Errorf("%s = %+v, want %+v", c.name, c.got, c.want)
		}
	}

	var total, errs, warns int
	for _, b := range got.Timeline {
		total += b.Total
		errs += b.Errors
		warns += b.Warnings
	}
	if total != 8 || errs != 2 || warns != 2 {
		t.Errorf("timeline sums total=%d errors=%d warnings=%d, want 8/2/2 (unstamped line excluded)", total, errs, warns)
	}
	if got.BucketMillis != 60000 || len(got.Timeline) != 8 {
		t.Errorf("bucket=%dms points=%d, want 60000ms and 8", got.BucketMillis, len(got.Timeline))
	}
}

func TestStatsHonoursFilter(t *testing.T) {
	dir := logFolder(t)
	base := time.Date(2026, 3, 4, 10, 0, 0, 0, time.Local)
	writeLog(t, dir, "a.log", strings.Join([]string{
		accessLine(base, "203.0.113.1", "alice", "a.example", "in-1", "tcp"),
		accessLine(base.Add(time.Minute), "203.0.113.2", "bob", "b.example", "in-2", "udp"),
	}, "\n")+"\n")
	got, err := (&Service{}).Stats(context.Background(), StatsRequest{File: "a.log", Filter: Filter{User: "bob"}})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if got.MatchedLines != 1 || got.ScannedLines != 2 {
		t.Errorf("matched=%d scanned=%d, want 1/2", got.MatchedLines, got.ScannedLines)
	}
	if want := []LogCount{{"203.0.113.2", 1}}; !slices.Equal(got.TopIPs, want) {
		t.Errorf("TopIPs = %+v, want %+v", got.TopIPs, want)
	}
}

func TestStatsTimelineStaysSmallAndCoversARequestedRange(t *testing.T) {
	dir := logFolder(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	var b strings.Builder
	for i := range 5000 {
		b.WriteString(accessLine(base.Add(time.Duration(i)*time.Hour), "203.0.113.1", "a", "a.example", "in", "tcp") + "\n")
	}
	writeLog(t, dir, "a.log", b.String())

	got, err := (&Service{}).Stats(context.Background(), StatsRequest{File: "a.log"})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if n := len(got.Timeline); n == 0 || n > maxTimelineBuckets+1 {
		t.Errorf("timeline has %d points, want 1..%d", n, maxTimelineBuckets+1)
	}
	sum := 0
	for _, bucket := range got.Timeline {
		sum += bucket.Total
	}
	if sum != 5000 {
		t.Errorf("merged timeline counts %d lines, want 5000", sum)
	}

	since := base.Add(4990 * time.Hour)
	until := since.Add(24 * time.Hour)
	ranged, err := (&Service{}).Stats(context.Background(), StatsRequest{File: "a.log", Filter: Filter{Since: since.UnixMilli(), Until: until.UnixMilli()}})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	first, last := ranged.Timeline[0], ranged.Timeline[len(ranged.Timeline)-1]
	if first.Start > since.UnixMilli() || last.Start+ranged.BucketMillis < until.UnixMilli() {
		t.Errorf("axis %d..%d does not span the requested %d..%d", first.Start, last.Start+ranged.BucketMillis, since.UnixMilli(), until.UnixMilli())
	}
}

// Destinations come from clients, so the distinct-value maps must not grow with them.
func TestStatsDistinctValuesAreCapped(t *testing.T) {
	dir := logFolder(t)
	base := time.Date(2026, 3, 4, 10, 0, 0, 0, time.Local)
	var b strings.Builder
	for i := range 50 {
		b.WriteString(accessLine(base, "203.0.113.1", "a", fmt.Sprintf("h%d.example", i), "in", "tcp") + "\n")
	}
	writeLog(t, dir, "a.log", b.String())
	got, err := (&Service{maxDistinct: 10}).Stats(context.Background(), StatsRequest{File: "a.log"})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if got.UniqueHosts != 10 || !got.UniqueCapped {
		t.Errorf("UniqueHosts=%d capped=%v, want 10/true", got.UniqueHosts, got.UniqueCapped)
	}
}

func TestStatsBudgetIsReported(t *testing.T) {
	dir := logFolder(t)
	writeLog(t, dir, "a.log", numberedLines(40000))
	got, err := (&Service{budget: chunkSize}).Stats(context.Background(), StatsRequest{File: "a.log"})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if !got.BudgetExhausted || got.ScannedLines == 0 || got.ScannedLines >= 40000 {
		t.Errorf("exhausted=%v scanned=%d, want a partial scan flagged as such", got.BudgetExhausted, got.ScannedLines)
	}
	if got.ScannedBytes != chunkSize || got.FileSize <= got.ScannedBytes {
		t.Errorf("FileSize=%d ScannedBytes=%d, want one chunk of a larger file", got.FileSize, got.ScannedBytes)
	}
}

// The prune job truncates a log in place, so a chunk can vanish between stat and read.
type truncatedReader struct{}

func (truncatedReader) ReadAt([]byte, int64) (int, error) { return 0, io.EOF }

func TestScanBackwardSurvivesConcurrentTruncation(t *testing.T) {
	visited := 0
	out, err := scanBackward(context.Background(), truncatedReader{}, 1024, defaultBudget, func([]byte, int64, bool) bool {
		visited++
		return true
	})
	if err != nil || out.next != 0 || visited != 0 {
		t.Errorf("err=%v next=%d visited=%d, want a clean empty result", err, out.next, visited)
	}
}

func TestStatsReadsGzip(t *testing.T) {
	dir := logFolder(t)
	base := time.Date(2026, 3, 4, 10, 0, 0, 0, time.Local)
	writeGzipLog(t, dir, "a.log.gz", accessLine(base, "203.0.113.1", "alice", "a.example", "in", "tcp")+"\n")
	got, err := (&Service{}).Stats(context.Background(), StatsRequest{File: "a.log.gz"})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if got.MatchedLines != 1 || got.UniqueUsers != 1 {
		t.Errorf("matched=%d users=%d, want 1/1", got.MatchedLines, got.UniqueUsers)
	}
}

// A gzip bomb is bounded by the budget on decompressed bytes, not by the file's size on disk.
func TestGzipBudgetBoundsDecompression(t *testing.T) {
	dir := logFolder(t)
	writeGzipLog(t, dir, "bomb.log.gz", strings.Repeat("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n", 1<<20))
	const budget = 1 << 20
	got, err := (&Service{budget: budget}).Stats(context.Background(), StatsRequest{File: "bomb.log.gz"})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if !got.BudgetExhausted || got.ScannedBytes > budget+maxLineBytes {
		t.Errorf("exhausted=%v scanned=%d on a %d budget", got.BudgetExhausted, got.ScannedBytes, budget)
	}
}

// Each view of the same line must serialise as the shape the generated schemas promise.
func TestEmptyResultsSerialiseAsArrays(t *testing.T) {
	dir := logFolder(t)
	writeLog(t, dir, "empty.log", "")
	page, err := (&Service{}).Read(context.Background(), ReadRequest{File: "empty.log"})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	stats, err := (&Service{}).Stats(context.Background(), StatsRequest{File: "empty.log"})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	encoded, err := json.Marshal(map[string]any{"page": page, "stats": stats})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "null") {
		t.Errorf("empty file produced a null: %s", encoded)
	}
}

func writeCountryDB(t *testing.T, entries map[string][]string) {
	t.Helper()
	var list xraygeodata.GeoIPList
	for code, cidrs := range entries {
		entry := &xraygeodata.GeoIP{Code: code}
		for _, cidr := range cidrs {
			prefix := netip.MustParsePrefix(cidr)
			entry.Cidr = append(entry.Cidr, &xraygeodata.CIDR{Ip: prefix.Addr().AsSlice(), Prefix: uint32(prefix.Bits())})
		}
		list.Entry = append(list.Entry, entry)
	}
	data, err := proto.Marshal(&list)
	if err != nil {
		t.Fatalf("marshal geoip: %v", err)
	}
	if err := os.WriteFile(filepath.Join(os.Getenv("XUI_BIN_FOLDER"), geodata.CountryFileName), data, 0o640); err != nil {
		t.Fatalf("write geoip: %v", err)
	}
}

const (
	lineFromGermany = "2026/03/04 05:06:07 from 198.51.100.7:1 accepted tcp:a.example:443 [in >> out] email: alice"
	lineFromFrance  = "2026/03/04 05:06:08 from [2001:db8::9]:1 accepted tcp:b.example:443 [in >> out] email: bob"
	lineFromPrivate = "2026/03/04 05:06:09 from 192.168.1.5:1 accepted tcp:c.example:443 [in >> out]"
	lineFromMasked  = "2026/03/04 05:06:10 from 198.51.*.*:1 accepted tcp:d.example:443 [in >> out]"
	lineWithoutIP   = "2026/03/04 05:06:11 INFO - nothing about a client here"
)

func writeCountryFixture(t *testing.T) {
	t.Helper()
	dir := logFolder(t)
	writeLog(t, dir, "a.log", strings.Join([]string{lineFromGermany, lineFromFrance, lineFromGermany, lineFromPrivate, lineFromMasked, lineWithoutIP}, "\n")+"\n")
}

func TestLinesAndStatsCarryTheCountryOfTheClientIP(t *testing.T) {
	writeCountryFixture(t)
	writeCountryDB(t, map[string][]string{"DE": {"198.51.100.0/24"}, "FR": {"2001:db8::/32"}, "PRIVATE": {"192.168.0.0/16"}})

	page, err := (&Service{}).Read(context.Background(), ReadRequest{File: "a.log"})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got := map[string]string{}
	for _, l := range page.Lines {
		got[l.Raw] = l.Country
	}
	want := map[string]string{lineFromGermany: "DE", lineFromFrance: "FR", lineFromPrivate: "", lineFromMasked: "", lineWithoutIP: ""}
	if !maps.Equal(got, want) {
		t.Errorf("countries = %v, want %v", got, want)
	}

	stats, err := (&Service{}).Stats(context.Background(), StatsRequest{File: "a.log"})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if !stats.GeoAvailable {
		t.Error("GeoAvailable = false with a database present")
	}
	if want := []LogCount{{"DE", 2}, {"FR", 1}}; !slices.Equal(stats.Countries, want) {
		t.Errorf("Countries = %+v, want %+v", stats.Countries, want)
	}
	// The private and the masked client; a line naming no client is not "unlocated".
	if stats.UnlocatedLines != 2 {
		t.Errorf("UnlocatedLines = %d, want 2", stats.UnlocatedLines)
	}
}

func TestCountryIsAFilter(t *testing.T) {
	writeCountryFixture(t)
	writeCountryDB(t, map[string][]string{"DE": {"198.51.100.0/24"}, "FR": {"2001:db8::/32"}})

	page, err := (&Service{}).Read(context.Background(), ReadRequest{File: "a.log", Filter: Filter{Country: "fr"}})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got := rawOf(page); !slices.Equal(got, []string{lineFromFrance}) {
		t.Errorf("Read = %q, want the French line", got)
	}
	stats, err := (&Service{}).Stats(context.Background(), StatsRequest{File: "a.log", Filter: Filter{Country: "DE"}})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.MatchedLines != 2 || stats.UniqueUsers != 1 {
		t.Errorf("matched=%d users=%d, want 2/1", stats.MatchedLines, stats.UniqueUsers)
	}
}

// geoip.dat can be absent: the panel downloads it separately, and a dev checkout has none.
func TestLogsStayReadableWithoutACountryDatabase(t *testing.T) {
	cases := map[string]func(t *testing.T){
		"missing": func(*testing.T) {},
		"corrupt": func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(os.Getenv("XUI_BIN_FOLDER"), geodata.CountryFileName), []byte{0xff, 0xff, 0xff}, 0o640); err != nil {
				t.Fatal(err)
			}
		},
		"symlink out of the folder": func(t *testing.T) {
			outside := filepath.Join(t.TempDir(), "outside.dat")
			if err := os.WriteFile(outside, nil, 0o640); err != nil {
				t.Fatal(err)
			}
			symlinkOrSkip(t, outside, filepath.Join(os.Getenv("XUI_BIN_FOLDER"), geodata.CountryFileName))
		},
	}
	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			writeCountryFixture(t)
			prepare(t)
			stats, err := (&Service{}).Stats(context.Background(), StatsRequest{File: "a.log"})
			if err != nil {
				t.Fatalf("Stats: %v", err)
			}
			if stats.GeoAvailable || len(stats.Countries) != 0 || stats.Countries == nil || stats.UnlocatedLines != 0 {
				t.Errorf("available=%v countries=%#v unlocated=%d, want false/empty/0", stats.GeoAvailable, stats.Countries, stats.UnlocatedLines)
			}
			if stats.MatchedLines != 6 {
				t.Errorf("MatchedLines = %d, want 6", stats.MatchedLines)
			}
		})
	}
}

// The panel's "update geo files" action replaces the database while the process keeps running.
func TestCountryDatabaseIsReloadedWhenReplaced(t *testing.T) {
	writeCountryFixture(t)
	writeCountryDB(t, map[string][]string{"DE": {"198.51.100.0/24"}})
	first, err := (&Service{}).Stats(context.Background(), StatsRequest{File: "a.log"})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if want := []LogCount{{"DE", 2}}; !slices.Equal(first.Countries, want) {
		t.Fatalf("Countries = %+v, want %+v", first.Countries, want)
	}

	writeCountryDB(t, map[string][]string{"NL": {"198.51.100.0/24"}})
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(os.Getenv("XUI_BIN_FOLDER"), geodata.CountryFileName), later, later); err != nil {
		t.Fatal(err)
	}
	second, err := (&Service{}).Stats(context.Background(), StatsRequest{File: "a.log"})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if want := []LogCount{{"NL", 2}}; !slices.Equal(second.Countries, want) {
		t.Errorf("Countries after replacing the file = %+v, want %+v", second.Countries, want)
	}
}

func TestCountryTableIsReleasedWhenIdleAndReloadsOnDemand(t *testing.T) {
	previous := countryIdleRelease
	countryIdleRelease = 20 * time.Millisecond
	t.Cleanup(func() { countryIdleRelease = previous })
	writeCountryFixture(t)
	writeCountryDB(t, map[string][]string{"DE": {"198.51.100.0/24"}})

	if countries.get() == nil {
		t.Fatal("no table with a database present")
	}
	held := func() bool {
		countries.mu.Lock()
		defer countries.mu.Unlock()
		return countries.index != nil
	}
	deadline := time.Now().Add(5 * time.Second)
	for held() {
		if time.Now().After(deadline) {
			t.Fatal("table still held after the idle period")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if countries.get() == nil {
		t.Error("table did not reload on the next use")
	}
}
