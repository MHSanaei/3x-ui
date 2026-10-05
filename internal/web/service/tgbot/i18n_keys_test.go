package tgbot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The bot resolves text through I18nBot, which returns an empty string for a key
// that is missing from the translation file: a screen then renders with a blank
// button or a blank body, and nothing fails loudly. That is how a live bot ended
// up answering /start with no menu. Every key a Go source asks for must exist.

var (
	i18nBotKeyPattern = regexp.MustCompile(`I18nBot\(\s*"([^"]+)"`)
	// Button labels go through btn(), which also calls I18nBot internally, so a
	// broken button key is invisible to a scan of I18nBot calls alone.
	btnKeyPattern = regexp.MustCompile(`\.btn(?:2)?\(\s*"([^"]+)"`)
	// The category screen asks for "tgbot.messages.category_" + name.
	categoryKeyPattern = regexp.MustCompile(`tgbot\.messages\.category_"`)
)

func TestEveryBotKeyExistsInTheTranslations(t *testing.T) {
	root := repoRootForTest(t)
	keys := map[string]string{} // key -> first file that asks for it
	err := filepath.Walk(filepath.Join(root, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		patterns := []*regexp.Regexp{i18nBotKeyPattern, btnKeyPattern}
		for _, pattern := range patterns {
			for _, m := range pattern.FindAllStringSubmatch(string(data), -1) {
				key := m[1]
				// Keys are also assembled at run time ("tgbot.screens." + kind,
				// "tgbot.messages.category_" + name). A literal that ends in a
				// separator is such a prefix, and the full set it expands to is
				// checked explicitly.
				if strings.ContainsAny(key, "%$") || strings.HasSuffix(key, ".") || strings.HasSuffix(key, "_") {
					continue
				}
				if _, seen := keys[key]; !seen {
					keys[key] = strings.TrimPrefix(path, root+"/")
				}
			}
		}
		if categoryKeyPattern.MatchString(string(data)) {
			for _, category := range []string{"server", "clients", "traffic", "maintenance"} {
				keys["tgbot.messages.category_"+category] = strings.TrimPrefix(path, root+"/")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan sources: %v", err)
	}

	// Keys assembled from a prefix at run time; their full set is checked so the
	// dynamic path cannot hide a missing entry.
	for _, kind := range screenKinds {
		keys["tgbot.screens."+kind] = "screens_art.go (screenKinds)"
	}

	files, err := filepath.Glob(filepath.Join(root, "internal/web/translation/*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no translation files found: %v", err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		var bundle map[string]any
		if err := json.Unmarshal(data, &bundle); err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		var missing []string
		for key, source := range keys {
			if !nestedKeyPresent(bundle, key) {
				missing = append(missing, fmt.Sprintf("%s (%s)", key, source))
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			t.Errorf("%s is missing %d key(s) the bot asks for:\n  %s",
				filepath.Base(file), len(missing), strings.Join(missing, "\n  "))
		}
	}
}

// nestedKeyPresent walks a dotted key through the nested translation objects.
func nestedKeyPresent(bundle map[string]any, key string) bool {
	var current any = bundle
	for _, part := range strings.Split(key, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = obj[part]
		if !ok {
			return false
		}
	}
	return true
}

// repoRootForTest walks up from the test's working directory to the module root.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("module root not found")
		}
		dir = parent
	}
}
