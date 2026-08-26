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

// resourceSrc/linkHref catch a page *loading* something from another host on
// render -- src is exclusive to resource-loading elements (script/img/
// iframe/...), and link's own href is a stylesheet/preload, never present on
// a plain <a>. That distinction matters: several login-mock templates now
// carry real external <a href> links on purpose (the real product's own
// privacy-policy/forgot-password/report-a-problem links, see
// decoy_login_homeassistant.go and adguardhome.html) -- those only fire on a
// click and cost nothing if unreachable, unlike a resource the page itself
// depends on to render.
var (
	resourceSrc = regexp.MustCompile(`(?i)\bsrc\s*=\s*["'](https?:)?//`)
	linkHref    = regexp.MustCompile(`(?i)<link\b[^>]*\bhref\s*=\s*["'](https?:)?//`)
)

// A decoy is only convincing if it renders offline: anything the page itself
// fetches on load from another host fails on a server that is pretending to
// be an ordinary site.
func TestTemplatesReferenceNoExternalResources(t *testing.T) {
	for _, name := range DecoyTemplateNames() {
		body, err := renderDecoyTemplate(name, "seed")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		text := string(body)
		for _, needle := range []string{"//cdn", "src=\"/", "@import"} {
			if strings.Contains(text, needle) {
				t.Errorf("%s: references %q, but decoys must render without fetching it", name, needle)
			}
		}
		if m := resourceSrc.FindString(text); m != "" {
			t.Errorf("%s: loads an external resource via %q", name, m)
		}
		if m := linkHref.FindString(text); m != "" {
			t.Errorf("%s: loads an external stylesheet/resource via %q", name, m)
		}
	}
}

var blankTargetLink = regexp.MustCompile(`(?i)<a\b[^>]*\btarget\s*=\s*["']_blank["'][^>]*>`)

// A target=_blank link without rel=noopener hands the opened page a live
// window.opener handle back to this one -- tabnabbing, a real phishing
// technique -- which would be an odd thing for a decoy to expose on a link
// it added on purpose to look more like the real product.
func TestExternalLinksAvoidTabnabbing(t *testing.T) {
	for _, name := range DecoyTemplateNames() {
		body, err := renderDecoyTemplate(name, "seed")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, tag := range blankTargetLink.FindAllString(string(body), -1) {
			if !strings.Contains(tag, "noopener") {
				t.Errorf("%s: target=_blank link missing rel=noopener: %s", name, tag)
			}
		}
	}
}
