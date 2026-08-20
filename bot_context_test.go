package main

// The Claude bot prompts in .github/workflows/claude-bot.yml no longer restate
// repository facts; they read .github/claude/repo-context.md instead. A stale
// claim in that file is invisible until it produces a wrong review, so every
// claim a machine can check is pinned here.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	botContextPath = ".github/claude/repo-context.md"
	botRubricPath  = ".github/claude/review-rubric.md"
	ciWorkflowPath = ".github/workflows/ci.yml"
)

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// section returns the text between two markers, so a table is matched only
// inside the heading that owns it.
func section(t *testing.T, doc, from, to string) string {
	t.Helper()
	i := strings.Index(doc, from)
	if i < 0 {
		t.Fatalf("%s no longer contains the heading %q", botContextPath, from)
	}
	rest := doc[i+len(from):]
	if j := strings.Index(rest, to); j >= 0 {
		return rest[:j]
	}
	return rest
}

func TestBotContextLocaleFileCount(t *testing.T) {
	doc := readRepoFile(t, botContextPath)
	m := regexp.MustCompile("`internal/web/translation/` \\((\\d+) files\\)").FindStringSubmatch(doc)
	if m == nil {
		t.Fatalf("%s no longer states the locale file count as \"`internal/web/translation/` (N files)\"", botContextPath)
	}
	files, err := filepath.Glob("internal/web/translation/*.json")
	if err != nil {
		t.Fatalf("glob locales: %v", err)
	}
	if got := len(files); m[1] != itoa(got) {
		t.Errorf("%s claims %s locale files, internal/web/translation/ holds %d; update the claim and every prompt that relies on it", botContextPath, m[1], got)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestBotContextNamesRealCIJobs(t *testing.T) {
	doc := readRepoFile(t, botContextPath)
	ci := readRepoFile(t, ciWorkflowPath)
	table := section(t, doc, "## What CI runs", "**What CI does NOT prove.**")
	rows := regexp.MustCompile("(?m)^\\| `([a-z0-9-]+)` \\|").FindAllStringSubmatch(table, -1)
	if len(rows) < 5 {
		t.Fatalf("expected the CI table in %s to list at least 5 jobs, found %d", botContextPath, len(rows))
	}
	for _, r := range rows {
		t.Run(r[1], func(t *testing.T) {
			if !strings.Contains(ci, "\n  "+r[1]+":\n") {
				t.Errorf("%s describes a CI job %q that %s does not define", botContextPath, r[1], ciWorkflowPath)
			}
		})
	}
}

func TestBotContextNamesRealPaths(t *testing.T) {
	doc := readRepoFile(t, botContextPath) + readRepoFile(t, botRubricPath)
	// internal/web/dist and frontend/node_modules are build output: absent from a
	// fresh clone, created by `make dist-stub` and `npm ci`.
	generated := map[string]bool{
		"internal/web/dist/":      true,
		"frontend/node_modules":   true,
		"frontend/src/generated/": true,
	}
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(doc, -1) {
		p := m[1]
		if !regexp.MustCompile(`^(internal|frontend|docs|tools|\.github)/`).MatchString(p) ||
			strings.ContainsAny(p, "*{ ") || generated[p] || seen[p] {
			continue
		}
		seen[p] = true
		t.Run(p, func(t *testing.T) {
			if _, err := os.Stat(strings.TrimSuffix(p, "/")); err != nil {
				t.Errorf("%s names %q, which does not exist; the bot prompts trust this file", botContextPath, p)
			}
		})
	}
	if len(seen) < 20 {
		t.Errorf("expected the bot context to name at least 20 repository paths, found %d - has the file been gutted?", len(seen))
	}
}

func TestBotContextSkipGatesExist(t *testing.T) {
	doc := readRepoFile(t, botContextPath)
	table := section(t, doc, "**What CI does NOT prove.**", "Mutation testing")
	// [A-Z0-9_] and not [A-Z_]: XRAY_E2E_BINARY carries a digit, and excluding it
	// silently dropped that gate from the check instead of failing.
	gates := regexp.MustCompile("`((?:XUI|XRAY)_[A-Z0-9_]+)`").FindAllStringSubmatch(table, -1)
	if len(gates) < 5 {
		t.Fatalf("expected at least 5 skip-gate variables in %s, found %d", botContextPath, len(gates))
	}
	var sources []string
	err := filepath.WalkDir("internal", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			sources = append(sources, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal: %v", err)
	}
	for _, g := range gates {
		t.Run(g[1], func(t *testing.T) {
			for _, f := range sources {
				if strings.Contains(readRepoFile(t, f), g[1]) {
					return
				}
			}
			t.Errorf("%s lists %s as a test skip gate, but no .go file under internal/ reads it", botContextPath, g[1])
		})
	}
}
