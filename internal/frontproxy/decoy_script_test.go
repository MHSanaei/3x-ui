package frontproxy

import (
	"regexp"
	"strings"
	"testing"
)

var scriptBlock = regexp.MustCompile(`(?s)<script>(.*?)</script>`)

// html/template lexes <script> bodies as JavaScript to decide how to escape
// template actions. The templates here put none inside a script, so the body
// must come through byte-for-byte -- a regex literal misread as division would
// silently ship a broken page that still renders and still passes every other
// test here.
//
// One rewrite is expected and unavoidable: html/template strips JavaScript
// comments. Keep the templates free of them rather than loosening this check,
// since a comment that never reaches the browser is only misleading anyway.
func TestScriptBodiesSurviveRendering(t *testing.T) {
	for _, name := range DecoyTemplateNames() {
		raw, err := decoyTemplates.ReadFile("templates/" + name + ".html")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		source := scriptBlock.FindAllStringSubmatch(string(raw), -1)
		if len(source) == 0 {
			continue
		}
		rendered, err := renderDecoyTemplate(name, "seed")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, block := range source {
			if strings.Contains(block[1], "{{") {
				t.Errorf("%s: template action inside <script>; keep the theming in CSS", name)
				continue
			}
			if !strings.Contains(string(rendered), block[1]) {
				t.Errorf("%s: script body was rewritten during rendering", name)
			}
		}
	}
}

// A decoy is only convincing if it works offline: anything fetched from
// another host fails on a server that is pretending to be an ordinary site.
func TestTemplatesReferenceNoExternalResources(t *testing.T) {
	for _, name := range DecoyTemplateNames() {
		body, err := renderDecoyTemplate(name, "seed")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, needle := range []string{"http://", "https://", "//cdn", "src=\"/", "@import"} {
			if strings.Contains(string(body), needle) {
				t.Errorf("%s: references %q, but decoys must be self-contained", name, needle)
			}
		}
	}
}
