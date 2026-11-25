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

// The diagnosis must agree with the renderer about which protocols can become a
// balancer member. A protocol the JSON service never turns into an outbound would
// otherwise be counted as contributing: the balancer looks healthy while emitting
// nothing. Both lists are derived from the real sources, so either one drifting
// fails here instead of in production.
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
