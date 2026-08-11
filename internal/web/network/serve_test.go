package network

import (
	"errors"
	"net"
	"net/http"
	"os"
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

func TestEveryHTTPServerReportsUnexpectedServeFailures(t *testing.T) {
	for _, file := range []string{"../web.go", "../../sub/sub.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if got := strings.Count(string(source), "network.ServeHTTP("); got != 1 {
			t.Fatalf("%s ServeHTTP call sites = %d, want 1", file, got)
		}
	}
}
