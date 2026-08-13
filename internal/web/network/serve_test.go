package network

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type failingListener struct{ err error }

func (l failingListener) Accept() (net.Conn, error) { return nil, l.err }
func (failingListener) Close() error                { return nil }
func (failingListener) Addr() net.Addr              { return testAddr("failing") }

type testAddr string

func (a testAddr) Network() string { return string(a) }
func (a testAddr) String() string  { return string(a) }

func TestServeHTTPLogsUnexpectedListenerFailure(t *testing.T) {
	errInjected := errors.New("injected listener failure")
	ServeHTTP(&http.Server{}, failingListener{err: errInjected}, "Test server")

	for _, line := range logger.GetLogs(100, "error") {
		if strings.Contains(line, errInjected.Error()) {
			return
		}
	}
	t.Fatal("unexpected listener failure was not recorded in the panel log")
}

func TestServeHTTPSuppressesNormalServerClose(t *testing.T) {
	const marker = "normal-close-must-stay-silent"
	ServeHTTP(&http.Server{}, failingListener{err: http.ErrServerClosed}, marker)

	for _, line := range logger.GetLogs(100, "error") {
		if strings.Contains(line, marker) {
			t.Fatalf("normal http.ErrServerClosed was recorded as an error: %s", line)
		}
	}
}

func TestProductionHTTPServersUseServeHTTPWrapper(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../../.."))
	fset := token.NewFileSet()

	err := filepath.WalkDir(repoRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || path == currentFile || path == filepath.Join(filepath.Dir(currentFile), "serve.go") {
			return nil
		}

		parsed, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		usesHTTP := false
		for _, imp := range parsed.Imports {
			if imp.Path.Value == `"net/http"` {
				usesHTTP = true
				break
			}
		}
		if !usesHTTP {
			return nil
		}

		parsed, err = parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if ok && selector.Sel.Name == "Serve" {
				position := fset.Position(call.Pos())
				t.Errorf("direct Serve call at %s; production HTTP servers must use network.ServeHTTP", position)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan production Go files: %v", err)
	}
}
