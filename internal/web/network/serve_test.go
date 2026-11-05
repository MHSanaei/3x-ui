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
			if entry.Name() == ".git" || entry.Name() == "vendor" || entry.Name() == "node_modules" || entry.Name() == "third_party" {
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

// queueListener hands out conns from a channel, each claiming the given remote address.
type queueListener struct{ conns chan net.Conn }

func (l queueListener) Accept() (net.Conn, error) {
	c, ok := <-l.conns
	if !ok {
		return nil, net.ErrClosed
	}
	return c, nil
}
func (queueListener) Close() error   { return nil }
func (queueListener) Addr() net.Addr { return testAddr("queue") }

type remoteConn struct {
	net.Conn
	remote net.Addr
	closed chan struct{}
}

func (c *remoteConn) RemoteAddr() net.Addr { return c.remote }
func (c *remoteConn) Close() error         { close(c.closed); return nil }

func newRemoteConn(addr string) *remoteConn {
	return &remoteConn{remote: &net.TCPAddr{IP: net.ParseIP(addr), Port: 40000}, closed: make(chan struct{})}
}

// Every panel connection holds buffers before any login, so one source opening
// thousands of them used to take the whole panel's memory.
func TestLimitListenerCapsConnectionsPerSourceAndInTotal(t *testing.T) {
	queue := queueListener{conns: make(chan net.Conn, 16)}
	l := NewLimitListener(queue, 4, 2)
	accept := func(c *remoteConn) net.Conn {
		t.Helper()
		queue.conns <- c
		accepted, err := l.Accept()
		if err != nil {
			t.Fatalf("Accept: %v", err)
		}
		return accepted
	}

	first := accept(newRemoteConn("203.0.113.1"))
	accept(newRemoteConn("203.0.113.1"))
	refused := newRemoteConn("203.0.113.1")
	queue.conns <- refused
	other := accept(newRemoteConn("2001:db8::1"))
	select {
	case <-refused.closed:
	default:
		t.Fatal("a source past its share was accepted")
	}
	if other.RemoteAddr().String() != "[2001:db8::1]:40000" {
		t.Fatalf("accepted %s, want the other source", other.RemoteAddr())
	}

	// Loopback is exempt from the per-source share but counts toward the total.
	accept(newRemoteConn("127.0.0.1"))
	overTotal := newRemoteConn("127.0.0.1")
	queue.conns <- overTotal
	next := make(chan net.Conn, 1)
	go func() {
		c, _ := l.Accept()
		next <- c
	}()
	select {
	case <-overTotal.closed:
	case c := <-next:
		t.Fatalf("a connection past the total was accepted: %s", c.RemoteAddr())
	}
	_ = first.Close()
	queue.conns <- newRemoteConn("203.0.113.1")
	again := <-next
	if again.RemoteAddr().String() != "203.0.113.1:40000" {
		t.Fatalf("a closed connection did not free its slot: accepted %s", again.RemoteAddr())
	}
}
