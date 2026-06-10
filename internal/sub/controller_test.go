package sub

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestSUBController builds a controller with just the bits loadSubTemplate
// needs, so the template tests don't require a database.
func newTestSUBController() *SUBController {
	return &SUBController{subTemplateCache: map[string]*cachedSubTemplate{}}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func renderTemplate(t *testing.T, a *SUBController, dir string, data map[string]any) string {
	t.Helper()
	tmpl, err := a.loadSubTemplate(dir)
	if err != nil {
		t.Fatalf("loadSubTemplate: unexpected error: %v", err)
	}
	if tmpl == nil {
		t.Fatal("loadSubTemplate: expected a template, got nil")
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return buf.String()
}

func TestLoadSubTemplate_RendersIndex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), `<h1>{{ .sId }}</h1>`)

	got := renderTemplate(t, newTestSUBController(), dir, map[string]any{"sId": "abc-123"})
	if want := `<h1>abc-123</h1>`; got != want {
		t.Fatalf("rendered = %q, want %q", got, want)
	}
}

func TestLoadSubTemplate_PrefersSubHTML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), `from-index`)
	writeFile(t, filepath.Join(dir, "sub.html"), `from-sub`)

	got := renderTemplate(t, newTestSUBController(), dir, nil)
	if got != "from-sub" {
		t.Fatalf("rendered = %q, want %q (sub.html should take precedence)", got, "from-sub")
	}
}

func TestLoadSubTemplate_FallbackCases(t *testing.T) {
	a := newTestSUBController()

	t.Run("missing dir", func(t *testing.T) {
		tmpl, err := a.loadSubTemplate(filepath.Join(t.TempDir(), "does-not-exist"))
		if tmpl != nil || err != nil {
			t.Fatalf("got (%v, %v), want (nil, nil)", tmpl, err)
		}
	})

	t.Run("path is a file not a dir", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "index.html")
		writeFile(t, file, `whatever`)
		tmpl, err := a.loadSubTemplate(file)
		if tmpl != nil || err != nil {
			t.Fatalf("got (%v, %v), want (nil, nil)", tmpl, err)
		}
	})

	t.Run("dir without template file", func(t *testing.T) {
		tmpl, err := a.loadSubTemplate(t.TempDir())
		if tmpl != nil || err != nil {
			t.Fatalf("got (%v, %v), want (nil, nil)", tmpl, err)
		}
	})
}

func TestLoadSubTemplate_MalformedTemplate(t *testing.T) {
	dir := t.TempDir()
	// Unterminated action — html/template fails to parse this.
	writeFile(t, filepath.Join(dir, "index.html"), `<h1>{{ .sId </h1>`)

	tmpl, err := newTestSUBController().loadSubTemplate(dir)
	if err == nil {
		t.Fatal("expected a parse error for a malformed template, got nil")
	}
	if tmpl != nil {
		t.Fatalf("expected nil template on parse error, got %v", tmpl)
	}
}

func TestLoadSubTemplate_CacheHitAndInvalidation(t *testing.T) {
	a := newTestSUBController()
	dir := t.TempDir()
	path := filepath.Join(dir, "index.html")

	// v1 with a fixed mtime.
	writeFile(t, path, `v1`)
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, t1, t1); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	first, err := a.loadSubTemplate(dir)
	if err != nil || first == nil {
		t.Fatalf("first load: (%v, %v)", first, err)
	}

	// Same mtime → cache hit returns the identical parsed template.
	second, err := a.loadSubTemplate(dir)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if second != first {
		t.Fatal("expected cache hit to return the same *template.Template pointer")
	}

	// New content + newer mtime → cache invalidated, fresh content served.
	writeFile(t, path, `v2`)
	t2 := t1.Add(time.Hour)
	if err := os.Chtimes(path, t2, t2); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	third, err := a.loadSubTemplate(dir)
	if err != nil || third == nil {
		t.Fatalf("third load: (%v, %v)", third, err)
	}
	if third == first {
		t.Fatal("expected cache invalidation to re-parse the template after mtime change")
	}
	var buf bytes.Buffer
	if err := third.Execute(&buf, nil); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if buf.String() != "v2" {
		t.Fatalf("rendered = %q, want %q after edit", buf.String(), "v2")
	}
}
