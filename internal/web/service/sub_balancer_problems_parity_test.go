package service

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// A protocol the JSON service never renders must not count as a working member.
// Both sides are parsed from source, so drift fails here rather than in production.
func TestJSONIncapableProtocolsMatchRenderer(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	subDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "sub")

	capable := rendererCapableProtocols(t, filepath.Join(subDir, "json_service.go"))
	allowlist := subscriptionAllowlist(t, filepath.Join(subDir, "service.go"))

	if len(capable) == 0 || len(allowlist) == 0 {
		t.Fatalf("parsed nothing meaningful (capable=%d allowlist=%d); did the sources move?", len(capable), len(allowlist))
	}

	want := make([]string, 0, len(allowlist))
	for _, protocol := range allowlist {
		if !capable[protocol] {
			want = append(want, protocol)
		}
	}
	got := make([]string, 0, len(jsonIncapableProtocols))
	for protocol := range jsonIncapableProtocols {
		got = append(got, protocol)
	}
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(want, ",") != strings.Join(got, ",") {
		t.Fatalf("jsonIncapableProtocols = %v, want %v (allowlist minus renderer-handled protocols)", got, want)
	}
}

// rendererCapableProtocols returns the protocols whose case in the Protocol
// switch adds an outbound; a case that only continues (or is absent) cannot.
func rendererCapableProtocols(t *testing.T, path string) map[string]bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	capable := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		switchStmt, ok := n.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		tag, ok := switchStmt.Tag.(*ast.SelectorExpr)
		if !ok || tag.Sel.Name != "Protocol" {
			return true
		}
		for _, stmt := range switchStmt.Body.List {
			clause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			onlyContinue := len(clause.Body) == 1
			if onlyContinue {
				if branch, ok := clause.Body[0].(*ast.BranchStmt); !ok || branch.Tok != token.CONTINUE {
					onlyContinue = false
				}
			}
			if onlyContinue {
				continue
			}
			for _, expr := range clause.List {
				if lit, ok := expr.(*ast.BasicLit); ok {
					capable[strings.Trim(lit.Value, `"`)] = true
				}
			}
		}
		return true
	})
	return capable
}

// subscriptionAllowlist reads the protocol list the subscription query selects,
// so a protocol added there without renderer support is caught too.
func subscriptionAllowlist(t *testing.T, path string) []string {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	clause := regexp.MustCompile(`inbounds\.protocol in \(([^)]*)\)`).FindSubmatch(source)
	if clause == nil {
		t.Fatalf("subscription protocol allowlist not found in %s", path)
	}
	quoted := regexp.MustCompile(`'([a-z]+)'`).FindAllSubmatch(clause[1], -1)
	protocols := make([]string, 0, len(quoted))
	for _, match := range quoted {
		protocols = append(protocols, string(match[1]))
	}
	return protocols
}

// The form offers a fixed protocol list; the advisory must agree with it, or the
// UI hides an inbound the diagnosis calls healthy (or offers one it calls broken).
func TestJSONIncapableProtocolsMatchFormList(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	formPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "frontend", "src",
		"pages", "settings", "SubBalancerFormModal.tsx")
	source, err := os.ReadFile(formPath)
	if err != nil {
		t.Fatalf("read %s: %v", formPath, err)
	}
	block := regexp.MustCompile(`MULTI_CLIENT_PROTOCOLS\s*=\s*new Set\(\[([^\]]*)\]\)`).FindSubmatch(source)
	if block == nil {
		t.Fatalf("MULTI_CLIENT_PROTOCOLS not found in %s", formPath)
	}
	offered := map[string]bool{}
	for _, m := range regexp.MustCompile(`'([a-z0-9]+)'`).FindAllSubmatch(block[1], -1) {
		offered[string(m[1])] = true
	}
	if len(offered) == 0 {
		t.Fatal("parsed no protocols from the form list")
	}

	allowlist := subscriptionAllowlist(t, filepath.Join(filepath.Dir(thisFile), "..", "..", "sub", "service.go"))
	for _, protocol := range allowlist {
		offeredByForm := offered[protocol]
		listedByUs := false
		if _, ok := jsonIncapableProtocols[protocol]; ok {
			listedByUs = true
		}
		if offeredByForm == listedByUs {
			t.Errorf("protocol %q: form offers=%v, advisory lists incapable=%v — they must be opposites",
				protocol, offeredByForm, listedByUs)
		}
	}
}
