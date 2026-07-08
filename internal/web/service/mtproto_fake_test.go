package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const (
	mtprotoTestSecretA = "ee00112233445566778899aabbccddeeff6578616d706c652e636f6d"
	mtprotoTestSecretB = "ee101112131415161718191a1b1c1d1e1f6578616d706c652e636f6d"
	mtprotoTestSecretC = "ee202122232425262728292a2b2c2d2e2f6578616d706c652e636f6d"
	mtprotoTestSecretD = "ee303132333435363738393a3b3c3d3e3f6578616d706c652e636f6d"
)

func seedClientTraffic(t *testing.T, inboundId int, email string, enable bool) {
	t.Helper()
	row := xray.ClientTraffic{InboundId: inboundId, Email: email, Enable: enable}
	if err := database.GetDB().Create(&row).Error; err != nil {
		t.Fatalf("seed traffic %s: %v", email, err)
	}
}

func loadInboundByTag(t *testing.T, tag string) *model.Inbound {
	t.Helper()
	var ib model.Inbound
	if err := database.GetDB().Where("tag = ?", tag).First(&ib).Error; err != nil {
		t.Fatalf("load inbound %s: %v", tag, err)
	}
	return &ib
}

// fakeMtgChildMain is what the re-executed test binary runs when posing as an
// mtg child process: it appends its pid to the file named by MTG_FAKE_PIDFILE
// so tests can count spawns, then blocks until the manager kills it.
func fakeMtgChildMain() {
	if f, err := os.OpenFile(os.Getenv("MTG_FAKE_PIDFILE"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		fmt.Fprintf(f, "%d\n", os.Getpid())
		f.Close()
	}
	select {}
}

// installFakeMtg points the mtproto manager at a copy of the running test
// binary posing as mtg (via the MTG_FAKE_CHILD gate in TestMain) and returns
// the pid file whose line count equals the number of processes spawned so far.
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
	if err := os.WriteFile(filepath.Join(binDir, mtproto.GetBinaryName()), payload, 0o755); err != nil {
		t.Fatalf("install fake mtg: %v", err)
	}
	pidFile := filepath.Join(binDir, "mtg-pids.txt")
	t.Setenv("XUI_BIN_FOLDER", binDir)
	t.Setenv("MTG_FAKE_CHILD", "1")
	t.Setenv("MTG_FAKE_PIDFILE", pidFile)
	return pidFile
}

func countSpawns(t *testing.T, pidFile string) int {
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

// waitForSpawns polls until exactly want processes have registered, failing
// fast when the count overshoots and on timeout.
func waitForSpawns(t *testing.T, pidFile string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := countSpawns(t, pidFile)
		if got == want {
			return
		}
		if got > want {
			t.Fatalf("expected %d mtg spawn(s), got %d", want, got)
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d mtg spawn(s), still %d after timeout", want, got)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// assertNoNewSpawns gives a wrongly spawned child time to register, then
// asserts the spawn count is still exactly want.
func assertNoNewSpawns(t *testing.T, pidFile string, want int) {
	t.Helper()
	time.Sleep(500 * time.Millisecond)
	if got := countSpawns(t, pidFile); got != want {
		t.Fatalf("expected the mtg process to be kept (%d spawn(s)), got %d", want, got)
	}
}
