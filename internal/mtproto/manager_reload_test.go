package mtproto

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain lets the test binary re-exec itself as a stand-in for the mtg
// child process: with MTG_FAKE_CHILD=1 it records its pid and blocks, so the
// manager can start and stop it without a real mtg-multi binary.
func TestMain(m *testing.M) {
	if os.Getenv("MTG_FAKE_CHILD") == "1" {
		if f, err := os.OpenFile(os.Getenv("MTG_FAKE_PIDFILE"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintf(f, "%d\n", os.Getpid())
			f.Close()
		}
		select {}
	}
	os.Exit(m.Run())
}

func installFakeMtg(t *testing.T) string {
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
	if err := os.WriteFile(filepath.Join(binDir, GetBinaryName()), payload, 0o755); err != nil {
		t.Fatalf("install fake mtg: %v", err)
	}
	pidFile := filepath.Join(binDir, "mtg-pids.txt")
	t.Setenv("XUI_BIN_FOLDER", binDir)
	t.Setenv("MTG_FAKE_CHILD", "1")
	t.Setenv("MTG_FAKE_PIDFILE", pidFile)
	return pidFile
}

func spawnCount(t *testing.T, pidFile string) int {
	t.Helper()
	data, err := os.ReadFile(pidFile)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	return len(strings.Fields(string(data)))
}

func waitSpawnCount(t *testing.T, pidFile string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := spawnCount(t, pidFile)
		if got == want {
			return
		}
		if got > want {
			t.Fatalf("expected %d spawn(s), got %d", want, got)
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d spawn(s), still %d after timeout", want, got)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func mtgInst(id int, secrets ...SecretEntry) Instance {
	return Instance{Id: id, Tag: fmt.Sprintf("inbound-%d", id), Listen: "127.0.0.1", Port: 24000 + id, Secrets: secrets}
}

func TestEnsureActionFor(t *testing.T) {
	cases := []struct {
		name                                         string
		running                                      bool
		curStruct, curSecrets, newStruct, newSecrets string
		want                                         ensureAction
	}{
		{"dead process restarts", false, "s", "a", "s", "a", ensureRestart},
		{"structural change restarts", true, "s1", "a", "s2", "a", ensureRestart},
		{"secrets change reloads", true, "s", "a", "s", "b", ensureReload},
		{"identical is a noop", true, "s", "a", "s", "a", ensureNoop},
		{"dead beats a secrets-only change", false, "s", "a", "s", "b", ensureRestart},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ensureActionFor(tc.running, tc.curStruct, tc.curSecrets, tc.newStruct, tc.newSecrets); got != tc.want {
				t.Fatalf("ensureActionFor = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestApplySecrets(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   bool
	}{
		{"ok", http.StatusOK, true},
		{"not found on old binary", http.StatusNotFound, false},
		{"bad request", http.StatusBadRequest, false},
		{"unavailable", http.StatusServiceUnavailable, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath, gotAuth string
			var gotBody secretsPutBody
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
				_ = json.NewDecoder(r.Body).Decode(&gotBody)
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			inst := mtgInst(1,
				SecretEntry{Name: "alice", Secret: "ee01"},
				SecretEntry{Name: "bob", Secret: "ee02", AdTag: "fedcba9876543210fedcba9876543210"})
			if got := applySecrets(serverPort(t, srv), "sesame", inst); got != tc.want {
				t.Fatalf("applySecrets = %v, want %v", got, tc.want)
			}
			if gotMethod != http.MethodPut || gotPath != "/secrets" {
				t.Fatalf("expected PUT /secrets, got %s %s", gotMethod, gotPath)
			}
			if gotAuth != "Bearer sesame" {
				t.Fatalf("expected the bearer token on the request, got %q", gotAuth)
			}
			if gotBody.Secrets["alice"].Secret != "ee01" {
				t.Fatalf("payload must carry the secret: %+v", gotBody)
			}
			if gotBody.Secrets["alice"].AdTag != "" || gotBody.Secrets["bob"].AdTag != "fedcba9876543210fedcba9876543210" {
				t.Fatalf("payload must carry per-client ad-tags only where set: %+v", gotBody)
			}
		})
	}

	t.Run("refused connection", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		port := serverPort(t, srv)
		srv.Close()
		if applySecrets(port, "", mtgInst(1, SecretEntry{Name: "a", Secret: "ee"})) {
			t.Fatal("a refused connection must yield false")
		}
	})
}

func TestEnsureHotReloadKeepsProcess(t *testing.T) {
	pidFile := installFakeMtg(t)
	mgr := &Manager{procs: map[int]*managed{}, swept: true}

	inst := mtgInst(1, SecretEntry{Name: "alice", Secret: "ee01"})
	if err := mgr.Ensure(inst); err != nil {
		t.Fatalf("initial ensure: %v", err)
	}
	waitSpawnCount(t, pidFile, 1)
	orig := mgr.procs[1].proc
	origToken := mgr.procs[1].apiToken
	if origToken == "" {
		t.Fatal("a started process must get an api token")
	}

	reloaded := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/secrets" {
			reloaded <- struct{}{}
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	mgr.procs[1].apiPort = serverPort(t, srv)

	rekeyed := mtgInst(1, SecretEntry{Name: "alice", Secret: "ee01"}, SecretEntry{Name: "bob", Secret: "ee02"})
	if err := mgr.Ensure(rekeyed); err != nil {
		t.Fatalf("reload ensure: %v", err)
	}

	select {
	case <-reloaded:
	case <-time.After(3 * time.Second):
		t.Fatal("expected a PUT /secrets request")
	}
	if got := spawnCount(t, pidFile); got != 1 {
		t.Fatalf("hot reload must not spawn a new process, got %d", got)
	}
	if mgr.procs[1].proc != orig {
		t.Fatal("hot reload must keep the same process")
	}
	if mgr.procs[1].secretsFP != rekeyed.secretsFingerprint() {
		t.Fatal("stored secrets fingerprint must advance after a reload")
	}
	cfg, err := os.ReadFile(configPathForID(1))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(cfg), `"bob" = "ee02"`) {
		t.Fatalf("reloaded config must carry the new secret:\n%s", cfg)
	}
	if !strings.Contains(string(cfg), fmt.Sprintf("api-bind-to = \"127.0.0.1:%d\"", serverPort(t, srv))) {
		t.Fatalf("reload must reuse the same api port:\n%s", cfg)
	}
	if !strings.Contains(string(cfg), fmt.Sprintf("api-token = %q", origToken)) {
		t.Fatalf("reload must reuse the token the running process was started with:\n%s", cfg)
	}
	mgr.StopAll()
}

func TestEnsureReloadFallbackRestarts(t *testing.T) {
	pidFile := installFakeMtg(t)
	mgr := &Manager{procs: map[int]*managed{}, swept: true}

	if err := mgr.Ensure(mtgInst(2, SecretEntry{Name: "alice", Secret: "ee01"})); err != nil {
		t.Fatalf("initial ensure: %v", err)
	}
	waitSpawnCount(t, pidFile, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	mgr.procs[2].apiPort = serverPort(t, srv)

	if err := mgr.Ensure(mtgInst(2, SecretEntry{Name: "carol", Secret: "ee03"})); err != nil {
		t.Fatalf("fallback ensure: %v", err)
	}
	waitSpawnCount(t, pidFile, 2)
	mgr.StopAll()
}

func TestEnsureNoopKeepsProcess(t *testing.T) {
	pidFile := installFakeMtg(t)
	mgr := &Manager{procs: map[int]*managed{}, swept: true}

	inst := mtgInst(3, SecretEntry{Name: "alice", Secret: "ee01"}, SecretEntry{Name: "bob", Secret: "ee02"})
	if err := mgr.Ensure(inst); err != nil {
		t.Fatalf("initial ensure: %v", err)
	}
	waitSpawnCount(t, pidFile, 1)

	if err := mgr.Ensure(inst); err != nil {
		t.Fatalf("repeat ensure: %v", err)
	}
	reordered := mtgInst(3, SecretEntry{Name: "bob", Secret: "ee02"}, SecretEntry{Name: "alice", Secret: "ee01"})
	if err := mgr.Ensure(reordered); err != nil {
		t.Fatalf("reordered ensure: %v", err)
	}

	time.Sleep(300 * time.Millisecond)
	if got := spawnCount(t, pidFile); got != 1 {
		t.Fatalf("an unchanged instance must keep the one process, got %d spawns", got)
	}
	mgr.StopAll()
}
