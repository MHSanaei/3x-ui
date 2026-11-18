package mtproto

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// installFakeTelemt installs the test binary as the telemt sidecar (see TestMain)
// and returns its spawn and SIGHUP record files.
func installFakeTelemt(t *testing.T) (pidFile, hupFile string) {
	t.Helper()
	binDir := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}
	payload, err := os.ReadFile(self)
	if err != nil {
		t.Fatalf("read test binary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, GetTelemtBinaryName()), payload, 0o755); err != nil {
		t.Fatalf("install fake telemt: %v", err)
	}
	pidFile = filepath.Join(binDir, "telemt-pids.txt")
	hupFile = filepath.Join(binDir, "telemt-hups.txt")
	t.Setenv("XUI_BIN_FOLDER", binDir)
	t.Setenv("MTG_FAKE_CHILD", "1")
	t.Setenv("MTG_FAKE_PIDFILE", pidFile)
	t.Setenv("MTG_FAKE_HUPFILE", hupFile)
	return pidFile, hupFile
}

// scriptedState stands in for telemt's /v1/config/state: each call returns the
// next verdict, the last one repeating.
type scriptedState struct {
	mu       sync.Mutex
	verdicts []configVerdict
	calls    int
	// beforeReject runs before a rejection is reported, e.g. to wait for the spawn.
	beforeReject func()
}

func (s *scriptedState) set(v ...configVerdict) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verdicts = v
}

func (s *scriptedState) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *scriptedState) fetch(int, string) (configVerdict, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	v := s.verdicts[0]
	if len(s.verdicts) > 1 {
		s.verdicts = s.verdicts[1:]
	}
	if v == verdictRejected {
		if s.beforeReject != nil {
			s.beforeReject()
		}
		return v, "web.vhosts[0].profiles[0].user references unknown access user"
	}
	return v, ""
}

func stubTelemtState(t *testing.T, v ...configVerdict) *scriptedState {
	t.Helper()
	st := &scriptedState{verdicts: v}
	prevState, prevPoll, prevReload := telemtConfigState, telemtPollInterval, telemtReloadTimeout
	telemtConfigState = st.fetch
	telemtPollInterval = time.Millisecond
	telemtReloadTimeout = 50 * time.Millisecond
	t.Cleanup(func() {
		telemtConfigState, telemtPollInterval, telemtReloadTimeout = prevState, prevPoll, prevReload
	})
	return st
}

func lineCount(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return len(strings.Fields(string(data)))
}

func waitLines(t *testing.T, path string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for lineCount(t, path) < want {
		if time.Now().After(deadline) {
			t.Fatalf("%s: expected %d line(s), got %d", filepath.Base(path), want, lineCount(t, path))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func testManager(t *testing.T) *Manager {
	t.Helper()
	mgr := newManager()
	mgr.swept = true
	t.Cleanup(mgr.StopAll)
	return mgr
}

var (
	telemtAlice = SecretEntry{Name: "alice", Secret: eeSecret(keyAlice, "example.com")}
	telemtBob   = SecretEntry{Name: "bob", Secret: eeSecret(keyBob, "example.com")}
)

func TestTelemtClientEditReloadsInPlace(t *testing.T) {
	pidFile, hupFile := installFakeTelemt(t)
	stubTelemtState(t, verdictAccepted)
	mgr := testManager(t)

	if err := mgr.Ensure(telemtInst(telemtAlice)); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitLines(t, pidFile, 1)
	cur := mgr.procs[7]
	proc, mainBefore := cur.proc, readFile(t, cur.telemt.mainPath())

	if err := mgr.Ensure(telemtInst(telemtAlice, telemtBob)); err != nil {
		t.Fatalf("client add: %v", err)
	}
	waitLines(t, hupFile, 1)
	if got := lineCount(t, pidFile); got != 1 {
		t.Fatalf("a client edit must not restart telemt, spawns=%d", got)
	}
	if mgr.procs[7].proc != proc {
		t.Fatal("a client edit must keep the running process")
	}
	if got := readFile(t, cur.telemt.usersPath()); got != renderTelemtUsers(telemtInst(telemtAlice, telemtBob)) {
		t.Fatalf("users.toml must carry the new client:\n%s", got)
	}
	if readFile(t, cur.telemt.mainPath()) != mainBefore {
		t.Fatal("a client edit must leave telemt.toml untouched")
	}
}

// TestTelemtRejectedReloadRollsBack covers the silent failure telemt is known
// for: a refused reload keeps it on the old set, so the file must match that.
func TestTelemtRejectedReloadRollsBack(t *testing.T) {
	pidFile, hupFile := installFakeTelemt(t)
	state := stubTelemtState(t, verdictAccepted)
	mgr := testManager(t)

	if err := mgr.Ensure(telemtInst(telemtAlice)); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitLines(t, pidFile, 1)
	cur := mgr.procs[7]
	served := readFile(t, cur.telemt.usersPath())

	state.set(verdictRejected)
	// Pending SIGHUPs coalesce, so the rollback one is sent only after the first arrived.
	state.beforeReject = func() { waitLines(t, hupFile, 1) }
	err := mgr.Ensure(telemtInst(telemtAlice, telemtBob))
	if err == nil || !strings.Contains(err.Error(), "references unknown access user") {
		t.Fatalf("a rejected reload must surface telemt's reason, got %v", err)
	}
	if got := readFile(t, cur.telemt.usersPath()); got != served {
		t.Fatalf("a rejected users.toml must be rolled back to the served one:\n%s", got)
	}
	waitLines(t, hupFile, 2)
	if cur.secretsFP != served {
		t.Fatal("the served fingerprint must not advance past a rejected reload")
	}

	calls := state.callCount()
	if err := mgr.Ensure(telemtInst(telemtAlice, telemtBob)); err != nil {
		t.Fatalf("re-ensuring the rejected set must stay quiet, got %v", err)
	}
	if state.callCount() != calls || lineCount(t, hupFile) != 2 {
		t.Fatal("the reconcile tick must not rewrite and roll back the same rejected set again")
	}

	state.set(verdictAccepted)
	carol := SecretEntry{Name: "carol", Secret: eeSecret(keyBob, "example.com")}
	if err := mgr.Ensure(telemtInst(telemtAlice, carol)); err != nil {
		t.Fatalf("a changed client set must be tried again: %v", err)
	}
	if !strings.Contains(readFile(t, cur.telemt.usersPath()), `"carol"`) {
		t.Fatal("the changed set must be applied")
	}
}

func TestTelemtPendingReloadRetries(t *testing.T) {
	pidFile, hupFile := installFakeTelemt(t)
	state := stubTelemtState(t, verdictAccepted)
	mgr := testManager(t)

	if err := mgr.Ensure(telemtInst(telemtAlice)); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitLines(t, pidFile, 1)
	served := mgr.procs[7].secretsFP

	state.set(verdictPending)
	want := telemtInst(telemtAlice, telemtBob)
	if err := mgr.Ensure(want); err == nil || !strings.Contains(err.Error(), "has not applied") {
		t.Fatalf("an unconfirmed reload must be reported, got %v", err)
	}
	if mgr.procs[7].secretsFP != served {
		t.Fatal("an unconfirmed reload must not count as applied")
	}
	state.set(verdictAccepted)
	if err := mgr.Ensure(want); err != nil {
		t.Fatalf("retry: %v", err)
	}
	waitLines(t, hupFile, 2)
	if mgr.procs[7].secretsFP != renderTelemtUsers(want) {
		t.Fatal("the next reconcile must apply the pending set")
	}
}

// TestTelemtRefusedStartRestoresPrevious: telemt exits on a config it refuses,
// so a structural change it rejects must bring the previous config back.
func TestTelemtRefusedStartRestoresPrevious(t *testing.T) {
	pidFile, _ := installFakeTelemt(t)
	state := stubTelemtState(t, verdictAccepted)
	mgr := testManager(t)

	if err := mgr.Ensure(telemtInst(telemtAlice)); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitLines(t, pidFile, 1)
	prevMain := mgr.procs[7].telemt.mainText

	bad := telemtInst(telemtAlice)
	bad.Port = 9443
	state.set(verdictRejected, verdictAccepted)
	state.beforeReject = func() { waitLines(t, pidFile, 2) }
	err := mgr.Ensure(bad)
	if !errors.Is(err, errTelemtRefused) {
		t.Fatalf("a refused start must be reported, got %v", err)
	}
	waitLines(t, pidFile, 3)
	cur, ok := mgr.procs[7]
	if !ok || !cur.proc.IsRunning() {
		t.Fatal("the previous config must be running again")
	}
	if got := readFile(t, cur.telemt.mainPath()); got != prevMain {
		t.Fatalf("telemt.toml must be the previous one:\n%s", got)
	}

	restored := cur.proc
	if err := mgr.Ensure(bad); err != nil {
		t.Fatalf("the refused config must not be retried: %v", err)
	}
	if mgr.procs[7].proc != restored || !restored.IsRunning() {
		t.Fatal("the reconcile tick must not restart into a refused config")
	}
}

func TestBackendSwitchReplacesSidecar(t *testing.T) {
	pidFile, _ := installFakeTelemt(t)
	payload, err := os.ReadFile(filepath.Join(os.Getenv("XUI_BIN_FOLDER"), GetTelemtBinaryName()))
	if err != nil {
		t.Fatalf("read fake: %v", err)
	}
	if err := os.WriteFile(filepath.Join(os.Getenv("XUI_BIN_FOLDER"), GetBinaryName()), payload, 0o755); err != nil {
		t.Fatalf("install fake mtg: %v", err)
	}
	stubTelemtState(t, verdictAccepted)
	mgr := testManager(t)

	mtgSide := telemtInst(telemtAlice)
	mtgSide.Backend = BackendMtg
	if err := mgr.Ensure(mtgSide); err != nil {
		t.Fatalf("mtg start: %v", err)
	}
	waitLines(t, pidFile, 1)
	mtgProc := mgr.procs[7].proc

	if err := mgr.Ensure(telemtInst(telemtAlice)); err != nil {
		t.Fatalf("switch to telemt: %v", err)
	}
	waitLines(t, pidFile, 2)
	if mtgProc.IsRunning() {
		t.Fatal("switching to telemt must stop mtg, which holds the same port")
	}
	if _, err := os.Stat(configPathForID(7)); !os.IsNotExist(err) {
		t.Fatalf("the mtg config must be removed, stat err=%v", err)
	}
	if mgr.procs[7].backend != BackendTelemt {
		t.Fatal("the inbound must now be served by telemt")
	}
}

func TestClassifyConfigState(t *testing.T) {
	cases := []struct {
		name, body string
		want       configVerdict
		reason     string
	}{
		{"published", `{"ok":true,"data":{"desired_status":"valid","watcher_matches_desired":true,"watcher":{"phase":"published"},"comparison":{"status":"published_match"}}}`, verdictAccepted, ""},
		{"written not yet seen", `{"ok":true,"data":{"desired_status":"valid","watcher_matches_desired":false,"watcher":{"phase":"published"},"comparison":{"status":"pending_publication"}}}`, verdictPending, "pending_publication"},
		{"invalid on disk", `{"ok":true,"data":{"desired_status":"invalid","desired_error":{"path":"access.users","code":"parse","message":"duplicate key"}}}`, verdictRejected, "access.users parse duplicate key"},
		{"reload rejected", `{"ok":true,"data":{"desired_status":"valid","watcher_matches_desired":true,"watcher":{"phase":"rejected","error_code":"effective_config_invalid"},"comparison":{"status":"pending_publication"}}}`, verdictRejected, "reload rejected effective_config_invalid"},
		{"old rejection, new file", `{"ok":true,"data":{"desired_status":"valid","watcher_matches_desired":false,"watcher":{"phase":"rejected"},"comparison":{"status":"pending_publication"}}}`, verdictPending, "pending_publication"},
		{"file not yet observed", `{"ok":true,"data":{"desired_status":"valid","watcher_matches_desired":false,"watcher":{"phase":"published"},"comparison":{"status":"published_match"}}}`, verdictPending, "published_match"},
		{"busy", `{"ok":true,"data":{"desired_status":"busy"}}`, verdictPending, "desired busy"},
		{"not ok", `{"ok":false}`, verdictUnavailable, "unreadable config state"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, reason := classifyConfigState([]byte(c.body))
			if got != c.want || !strings.Contains(reason, c.reason) {
				t.Fatalf("got %v %q, want %v containing %q", got, reason, c.want, c.reason)
			}
		})
	}
}

func TestFetchConfigState(t *testing.T) {
	var gotAuth, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config/state" {
			http.NotFound(w, r)
			return
		}
		gotAuth, gotQuery = r.Header.Get("Authorization"), r.URL.RawQuery
		_, _ = w.Write([]byte(`{"ok":true,"data":{"desired_status":"valid","watcher_matches_desired":true,"comparison":{"status":"published_match"}}}`))
	}))
	defer srv.Close()
	if v, _ := fetchConfigState(serverPort(t, srv), "tok"); v != verdictAccepted {
		t.Fatalf("verdict = %v", v)
	}
	if gotAuth != "Bearer tok" || gotQuery != "sections=access" {
		t.Fatalf("auth %q query %q: telemt compares the whole auth_header", gotAuth, gotQuery)
	}

	old := httptest.NewServer(http.NotFoundHandler())
	defer old.Close()
	if v, _ := fetchConfigState(serverPort(t, old), "tok"); v != verdictUnsupported {
		t.Fatalf("telemt without /v1/config/state must read as unsupported, got %v", v)
	}
}

func TestParseTelemtMetrics(t *testing.T) {
	text := `# HELP telemt_user_connections_current Per-user active connections
# TYPE telemt_user_connections_current gauge
telemt_user_connections_current{user="alice@example.com"} 2
telemt_user_connections_current{user="bob"} 0
telemt_user_octets_from_client_total{user="alice@example.com"} 1500
telemt_user_octets_to_client_total{user="alice@example.com"} 98000
telemt_user_octets_to_client_total{user="bob"} 7
telemt_user_msgs_to_client_total{user="bob"} 99
telemt_connections_total 5
`
	got, err := parseTelemtMetrics(strings.NewReader(text))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := map[string]statsUser{
		"alice@example.com": {Connections: 2, BytesIn: 1500, BytesOut: 98000},
		"bob":               {BytesOut: 7},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for k, w := range want {
		if got[k] != w {
			t.Fatalf("%s: got %+v, want %+v", k, got[k], w)
		}
	}
}

// TestTelemtFirstScrapeCountsFromZero: telemt creates a user's counters at its first
// connection (and again after 24h idle), so their first sighting is all new traffic.
func TestTelemtFirstScrapeCountsFromZero(t *testing.T) {
	pidFile, _ := installFakeTelemt(t)
	stubTelemtState(t, verdictAccepted)
	mgr := testManager(t)
	if err := mgr.Ensure(telemtInst(telemtAlice)); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitLines(t, pidFile, 1)

	body := "telemt_user_octets_from_client_total{user=\"alice\"} 1500\ntelemt_user_octets_to_client_total{user=\"alice\"} 98000\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	mgr.procs[7].telemt.metricsPort = serverPort(t, srv)

	got, _ := mgr.CollectTraffic()
	want := []Traffic{{Tag: "inbound-7", Email: "alice", Up: 1500, Down: 98000}}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("first scrape = %+v, want %+v", got, want)
	}
	body = "telemt_user_octets_from_client_total{user=\"alice\"} 1600\ntelemt_user_octets_to_client_total{user=\"alice\"} 98000\n"
	got, _ = mgr.CollectTraffic()
	if len(got) != 1 || got[0].Up != 100 || got[0].Down != 0 {
		t.Fatalf("second scrape must count the delta only, got %+v", got)
	}
}

// TestTelemtRefusedStartRetriedLater: telemt also exits on transient trouble (a
// port still held), so a refused config is retried after a pause, not never.
func TestTelemtRefusedStartRetriedLater(t *testing.T) {
	pidFile, _ := installFakeTelemt(t)
	state := stubTelemtState(t, verdictRejected)
	state.beforeReject = func() { waitLines(t, pidFile, 1) }
	prev := telemtRetryAfter
	telemtRetryAfter = 300 * time.Millisecond
	t.Cleanup(func() { telemtRetryAfter = prev })
	mgr := testManager(t)

	if err := mgr.Ensure(telemtInst(telemtAlice)); !errors.Is(err, errTelemtRefused) {
		t.Fatalf("first start: %v", err)
	}
	if err := mgr.Ensure(telemtInst(telemtAlice)); err != nil {
		t.Fatalf("a refused config must not be retried at once: %v", err)
	}
	if _, ok := mgr.procs[7]; ok {
		t.Fatal("nothing may run right after the refusal")
	}

	time.Sleep(telemtRetryAfter)
	state.set(verdictAccepted)
	if err := mgr.Ensure(telemtInst(telemtAlice)); err != nil {
		t.Fatalf("retry after the pause: %v", err)
	}
	if cur, ok := mgr.procs[7]; !ok || !cur.proc.IsRunning() {
		t.Fatal("the inbound must be served once the pause is over")
	}
}
