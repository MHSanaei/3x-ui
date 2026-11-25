package mtproto

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestQuotaResetRestartsUnsupportedSidecarAndRetriesInterruptedRestart(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		t.Run(strconv.FormatBool(interrupted), func(t *testing.T) {
			pidFile := installFakeMtg(t)
			mgr := &Manager{procs: map[int]*managed{}, swept: true}
			t.Cleanup(mgr.StopAll)
			inst := mtgInst(91, SecretEntry{Name: "ali", Secret: "ee01", QuotaBytes: 300, ExpiresUnix: 2000000000})
			if err := mgr.Ensure(inst); err != nil {
				t.Fatal(err)
			}
			waitSpawnCount(t, pidFile, 1)
			before := mgr.procs[inst.Id].proc
			other := mtgInst(93, SecretEntry{Name: "bob", Secret: "ee02", QuotaBytes: 500})
			if err := mgr.Ensure(other); err != nil {
				t.Fatal(err)
			}
			waitSpawnCount(t, pidFile, 2)
			otherProcess := mgr.procs[other.Id].proc
			mgr.procs[inst.Id].last["ali"] = clientCounters{up: 100, down: 200}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
			defer server.Close()
			mgr.procs[inst.Id].apiPort = server.Listener.Addr().(*net.TCPAddr).Port
			mgr.procs[other.Id].apiPort = server.Listener.Addr().(*net.TCPAddr).Port
			binary := GetBinaryPath()
			if interrupted {
				t.Setenv("XUI_BIN_FOLDER", t.TempDir())
				if err := mgr.ResetQuotaContext(context.Background(), "ali", inst.Id); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("restart failure hidden: %v", err)
				}
				t.Setenv("XUI_BIN_FOLDER", filepath.Dir(binary))
			}
			if err := mgr.ResetQuotaContext(context.Background(), "ali"); err != nil {
				t.Fatal(err)
			}
			waitSpawnCount(t, pidFile, 3)
			if mgr.procs[other.Id].proc != otherProcess || !otherProcess.IsRunning() {
				t.Fatal("quota reset restarted an unrelated inbound")
			}
			cur := mgr.procs[inst.Id]
			if cur.proc == before || before.IsRunning() || !cur.proc.IsRunning() {
				t.Fatal("unsupported quota endpoint did not replace the running process")
			}
			if len(cur.last) != 0 {
				t.Fatalf("stale runtime baselines survived restart: %+v", cur.last)
			}
			config, err := os.ReadFile(configPathForID(inst.Id))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(config), `quota = "300B"`) || !strings.Contains(string(config), `[secret-limits."ali"]`) {
				t.Fatalf("restart lost new quota: %s", config)
			}
		})
	}
}

func TestQuotaResetDoesNotRestartOnAuthorizationOrServerFailures(t *testing.T) {
	pidFile := installFakeMtg(t)
	mgr := &Manager{procs: map[int]*managed{}, swept: true}
	t.Cleanup(mgr.StopAll)
	inst := mtgInst(92, SecretEntry{Name: "ali", Secret: "ee01"})
	if err := mgr.Ensure(inst); err != nil {
		t.Fatal(err)
	}
	waitSpawnCount(t, pidFile, 1)
	before := mgr.procs[inst.Id].proc
	for _, code := range []int{200, 204, 401, 403, 500, 503} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
		mgr.procs[inst.Id].apiPort = server.Listener.Addr().(*net.TCPAddr).Port
		err := mgr.ResetQuotaContext(context.Background(), "ali", inst.Id)
		server.Close()
		if code < 300 && err != nil {
			t.Fatal(err)
		}
		if code >= 400 && (err == nil || !strings.Contains(err.Error(), strconv.Itoa(code))) {
			t.Fatalf("HTTP %d was hidden: %v", code, err)
		}
		if mgr.procs[inst.Id].proc != before || spawnCount(t, pidFile) != 1 {
			t.Fatalf("HTTP %d restarted sidecar", code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := mgr.ResetQuotaContext(ctx, "ali", inst.Id); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation hidden: %v", err)
	}
}

func TestQuotaResetReportsHTTPFailureAndUsesCallerContext(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusServiceUnavailable} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer test-token" || r.URL.EscapedPath() != "/secrets/ali%2Fone/reset-quota" {
					t.Errorf("incorrect reset request: %s %s", r.Method, r.URL.EscapedPath())
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			u, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			_, portString, err := net.SplitHostPort(u.Host)
			if err != nil {
				t.Fatal(err)
			}
			port, err := strconv.Atoi(portString)
			if err != nil {
				t.Fatal(err)
			}
			err = resetQuotaContext(context.Background(), port, "test-token", "ali/one")
			if (err != nil) != (status >= 400) {
				t.Fatalf("status %d: %v", status, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := resetQuotaContext(ctx, port, "test-token", "ali/one"); err == nil {
				t.Fatal("cancelled reset reported success")
			}
		})
	}
}
