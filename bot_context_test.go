package main

// The bot prompts and the files they read (issue-analyst-*.md, REVIEW.md) state
// repo facts no build checks; a stale or oversized one fails silently, so pin them.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/goccy/go-yaml"
)

const (
	analystContextPath      = ".github/claude/issue-analyst-context.md"
	analystInstructionsPath = ".github/claude/issue-analyst-instructions.md"
	reviewPath              = "REVIEW.md"
	ciWorkflowPath          = ".github/workflows/ci.yml"
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
		t.Fatalf("%s no longer contains the heading %q", analystContextPath, from)
	}
	rest := doc[i+len(from):]
	if before, _, ok := strings.Cut(rest, to); ok {
		return before
	}
	return rest
}

func TestAnalystContextLocaleFileCount(t *testing.T) {
	doc := readRepoFile(t, analystContextPath)
	m := regexp.MustCompile("`internal/web/translation/` \\((\\d+) files\\)").FindStringSubmatch(doc)
	if m == nil {
		t.Fatalf("%s no longer states the locale file count as \"`internal/web/translation/` (N files)\"", analystContextPath)
	}
	files, err := filepath.Glob("internal/web/translation/*.json")
	if err != nil {
		t.Fatalf("glob locales: %v", err)
	}
	if got := len(files); m[1] != itoa(got) {
		t.Errorf("%s claims %s locale files, internal/web/translation/ holds %d; update the claim and every prompt that relies on it", analystContextPath, m[1], got)
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

func TestAnalystContextNamesRealCIJobs(t *testing.T) {
	doc := readRepoFile(t, analystContextPath)
	ci := readRepoFile(t, ciWorkflowPath)
	table := section(t, doc, "## What CI runs", "**What CI does NOT prove.**")
	rows := regexp.MustCompile("(?m)^\\| `([a-z0-9-]+)` \\|").FindAllStringSubmatch(table, -1)
	if len(rows) < 5 {
		t.Fatalf("expected the CI table in %s to list at least 5 jobs, found %d", analystContextPath, len(rows))
	}
	for _, r := range rows {
		t.Run(r[1], func(t *testing.T) {
			if !strings.Contains(ci, "\n  "+r[1]+":\n") {
				t.Errorf("%s describes a CI job %q that %s does not define", analystContextPath, r[1], ciWorkflowPath)
			}
		})
	}
}

func TestAnalystContextNamesRealPaths(t *testing.T) {
	// REVIEW.md briefs the review job the way the two issue-analyst files brief
	// the analyst, so all three get their paths pinned.
	// internal/web/dist and frontend/node_modules are build output: absent from a
	// fresh clone, created by `make dist-stub` and `npm ci`.
	generated := map[string]bool{
		"internal/web/dist/":      true,
		"frontend/node_modules":   true,
		"frontend/src/generated/": true,
	}
	seen := map[string]bool{}
	counts := map[string]int{}
	for _, src := range []string{analystContextPath, analystInstructionsPath, reviewPath} {
		for _, m := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(readRepoFile(t, src), -1) {
			p := m[1]
			if !regexp.MustCompile(`^(internal|frontend|docs|tools|\.github)/`).MatchString(p) ||
				strings.ContainsAny(p, "*{ ") || generated[p] || seen[p] {
				continue
			}
			seen[p] = true
			counts[src]++
			t.Run(p, func(t *testing.T) {
				if _, err := os.Stat(strings.TrimSuffix(p, "/")); err != nil {
					t.Errorf("%s names %q, which does not exist; the bot prompts trust this file", src, p)
				}
			})
		}
	}
	if counts[analystContextPath] < 20 {
		t.Errorf("expected the bot context to name at least 20 repository paths, found %d - has the file been gutted?", counts[analystContextPath])
	}
}

func TestAnalystContextSkipGatesExist(t *testing.T) {
	doc := readRepoFile(t, analystContextPath)
	table := section(t, doc, "**What CI does NOT prove.**", "Mutation testing")
	// [A-Z0-9_] and not [A-Z_]: XRAY_E2E_BINARY carries a digit, and excluding it
	// silently dropped that gate from the check instead of failing.
	gates := regexp.MustCompile("`((?:XUI|XRAY)_[A-Z0-9_]+)`").FindAllStringSubmatch(table, -1)
	if len(gates) < 5 {
		t.Fatalf("expected at least 5 skip-gate variables in %s, found %d", analystContextPath, len(gates))
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
			t.Errorf("%s lists %s as a test skip gate, but no .go file under internal/ reads it", analystContextPath, g[1])
		})
	}
}

// REVIEW.md tells the reviewer which CI job proves what, and which skip gates
// mean a green run proved nothing. Both go stale silently on a rename.
func TestReviewNamesRealCIJobsAndGates(t *testing.T) {
	doc := readRepoFile(t, reviewPath)
	ci := readRepoFile(t, ciWorkflowPath)
	// Hyphenated only: a single-word job name is indistinguishable from prose.
	jobs := regexp.MustCompile("`([a-z0-9]+(?:-[a-z0-9]+)+)`").FindAllStringSubmatch(doc, -1)
	if len(jobs) < 2 {
		t.Fatalf("expected %s to name at least 2 CI jobs in backticks, found %d", reviewPath, len(jobs))
	}
	for _, j := range jobs {
		t.Run(j[1], func(t *testing.T) {
			if !strings.Contains(ci, "\n  "+j[1]+":\n") {
				t.Errorf("%s names a CI job %q that %s does not define", reviewPath, j[1], ciWorkflowPath)
			}
		})
	}
	for _, g := range regexp.MustCompile("`((?:XUI|XRAY)_[A-Z0-9_]+)`").FindAllStringSubmatch(doc, -1) {
		t.Run(g[1], func(t *testing.T) {
			if strings.Contains(ci, g[1]) {
				t.Errorf("%s claims %s is never set in CI, but %s sets it", reviewPath, g[1], ciWorkflowPath)
			}
		})
	}
}

// The i18n rule is the one REVIEW.md states as a number, so it is the one that
// goes wrong silently when a locale is added.
func TestReviewLocaleFileCount(t *testing.T) {
	doc := readRepoFile(t, reviewPath)
	m := regexp.MustCompile(`(\d+) locale files`).FindStringSubmatch(doc)
	if m == nil {
		t.Fatalf("%s no longer states the i18n rule as \"N locale files\"", reviewPath)
	}
	files, err := filepath.Glob("internal/web/translation/*.json")
	if err != nil {
		t.Fatalf("glob locales: %v", err)
	}
	if got := len(files); m[1] != itoa(got) {
		t.Errorf("%s tells the reviewer to expect %s locale files, internal/web/translation/ holds %d", reviewPath, m[1], got)
	}
}

// GitHub rejects the whole workflow once a value holding ${{ }} passes 21,000
// characters, so every trigger fails with no job to read; actionlint misses it.
func TestBotPromptsFitGitHubExpressionLimit(t *testing.T) {
	const limit = 21000
	files, err := filepath.Glob(".github/workflows/claude-*.yml")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob bot workflows: %v (%d files)", err, len(files))
	}
	checked := 0
	for _, path := range files {
		var wf struct {
			Jobs map[string]struct {
				Steps []struct {
					With map[string]any `yaml:"with"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(readRepoFile(t, path)), &wf); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for job, j := range wf.Jobs {
			for _, step := range j.Steps {
				for key, v := range step.With {
					s, ok := v.(string)
					if !ok || !strings.Contains(s, "${{") {
						continue
					}
					checked++
					// GitHub counts .NET string length: UTF-16 code units.
					if n := len(utf16.Encode([]rune(s))); n > limit {
						t.Errorf("%s job %s with.%s is %d characters; GitHub rejects the workflow over %d", path, job, key, n, limit)
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no ${{ }} value found in the bot workflows: the parse no longer reaches the prompts")
	}
}
