package job

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/op/go-logging"
)

// 3x-ui logger must be initialised once before any code path that can
// log a warning. otherwise log.Warningf panics on a nil logger.
var loggerInitOnce sync.Once

// setupIntegrationDB wires a temp sqlite db and log folder so
// updateInboundClientIps can run end to end. closes the db before
// TempDir cleanup so windows doesn't complain about the file being in
// use.
func setupIntegrationDB(t *testing.T) {
	t.Helper()

	loggerInitOnce.Do(func() {
		xuilogger.InitLogger(logging.ERROR)
	})

	dbDir := t.TempDir()
	logDir := t.TempDir()

	t.Setenv("XUI_DB_FOLDER", dbDir)
	t.Setenv("XUI_LOG_FOLDER", logDir)

	// updateInboundClientIps calls log.SetOutput on the package global,
	// which would leak to other tests in the same binary.
	origLogWriter := log.Writer()
	origLogFlags := log.Flags()
	t.Cleanup(func() {
		log.SetOutput(origLogWriter)
		log.SetFlags(origLogFlags)
	})

	if err := database.InitDB(filepath.Join(dbDir, "3x-ui.db")); err != nil {
		t.Fatalf("database.InitDB failed: %v", err)
	}
	// LIFO cleanup order: this runs before t.TempDir's own cleanup.
	t.Cleanup(func() {
		if err := database.CloseDB(); err != nil {
			t.Logf("database.CloseDB warning: %v", err)
		}
	})
}

// seed an inbound whose settings json has a single client with the
// given email and ip limit.
func seedInboundWithClient(t *testing.T, tag, email string, limitIp int) {
	t.Helper()
	settings := map[string]any{
		"clients": []map[string]any{
			{
				"email":   email,
				"limitIp": limitIp,
				"enable":  true,
			},
		},
	}
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	inbound := &model.Inbound{
		Tag:      tag,
		Enable:   true,
		Protocol: model.VLESS,
		Port:     4321,
		Settings: string(settingsJSON),
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
}

// seed an InboundClientIps row with the given blob.
func seedClientIps(t *testing.T, email string, ips []IPWithTimestamp) *model.InboundClientIps {
	t.Helper()
	blob, err := json.Marshal(ips)
	if err != nil {
		t.Fatalf("marshal ips: %v", err)
	}
	row := &model.InboundClientIps{
		ClientEmail: email,
		Ips:         string(blob),
	}
	if err := database.GetDB().Create(row).Error; err != nil {
		t.Fatalf("seed InboundClientIps: %v", err)
	}
	return row
}

// read the persisted blob and parse it back.
func readClientIps(t *testing.T, email string) []IPWithTimestamp {
	t.Helper()
	row := &model.InboundClientIps{}
	if err := database.GetDB().Where("client_email = ?", email).First(row).Error; err != nil {
		t.Fatalf("read InboundClientIps for %s: %v", email, err)
	}
	if row.Ips == "" {
		return nil
	}
	var out []IPWithTimestamp
	if err := json.Unmarshal([]byte(row.Ips), &out); err != nil {
		t.Fatalf("unmarshal Ips blob %q: %v", row.Ips, err)
	}
	return out
}

// make a lookup map so asserts don't depend on slice order.
func ipSet(entries []IPWithTimestamp) map[string]int64 {
	out := make(map[string]int64, len(entries))
	for _, e := range entries {
		out[e.IP] = e.Timestamp
	}
	return out
}

// #4091 repro: client has limit=3, db still holds 3 idle ips from a
// few minutes ago, only one live ip is actually connecting. pre-fix:
// live ip got banned every tick and never appeared in the panel.
// post-fix: no ban, live ip persisted, historical ips still visible.
func TestUpdateInboundClientIps_LiveIpNotBannedByStillFreshHistoricals(t *testing.T) {
	setupIntegrationDB(t)

	const email = "pr4091-repro"
	seedInboundWithClient(t, "inbound-pr4091", email, 3)

	now := time.Now().Unix()
	// idle but still within the 30min staleness window.
	row := seedClientIps(t, email, []IPWithTimestamp{
		{IP: "10.0.0.1", Timestamp: now - 20*60},
		{IP: "10.0.0.2", Timestamp: now - 15*60},
		{IP: "10.0.0.3", Timestamp: now - 10*60},
	})

	j := NewCheckClientIpJob()
	// the one that's actually connecting (user's 128.71.x.x).
	live := []IPWithTimestamp{
		{IP: "128.71.1.1", Timestamp: now},
	}

	shouldCleanLog := j.updateInboundClientIps(row, email, live)

	if shouldCleanLog {
		t.Fatalf("shouldCleanLog must be false, nothing should have been banned with 1 live ip under limit 3")
	}
	if len(j.disAllowedIps) != 0 {
		t.Fatalf("disAllowedIps must be empty, got %v", j.disAllowedIps)
	}

	persisted := ipSet(readClientIps(t, email))
	for _, want := range []string{"128.71.1.1", "10.0.0.1", "10.0.0.2", "10.0.0.3"} {
		if _, ok := persisted[want]; !ok {
			t.Errorf("expected %s to be persisted in inbound_client_ips.ips; got %v", want, persisted)
		}
	}
	if got := persisted["128.71.1.1"]; got != now {
		t.Errorf("live ip timestamp should match the scan timestamp %d, got %d", now, got)
	}

	// 3xipl.log must not contain a ban line.
	if info, err := os.Stat(readIpLimitLogPath()); err == nil && info.Size() > 0 {
		body, _ := os.ReadFile(readIpLimitLogPath())
		t.Fatalf("3xipl.log should be empty when no ips are banned, got:\n%s", body)
	}
}

// opposite invariant: when several ips are actually live and exceed
// the limit, the newcomer still gets banned.
func TestUpdateInboundClientIps_ExcessLiveIpIsStillBanned(t *testing.T) {
	setupIntegrationDB(t)

	const email = "pr4091-abuse"
	seedInboundWithClient(t, "inbound-pr4091-abuse", email, 1)

	now := time.Now().Unix()
	row := seedClientIps(t, email, []IPWithTimestamp{
		{IP: "10.1.0.1", Timestamp: now - 60}, // original connection
	})

	j := NewCheckClientIpJob()
	// both live, limit=1. use distinct timestamps so sort-by-timestamp
	// is deterministic: 10.1.0.1 is the original (older), 192.0.2.9
	// joined later and must get banned.
	live := []IPWithTimestamp{
		{IP: "10.1.0.1", Timestamp: now - 5},
		{IP: "192.0.2.9", Timestamp: now},
	}

	shouldCleanLog := j.updateInboundClientIps(row, email, live)

	if !shouldCleanLog {
		t.Fatalf("shouldCleanLog must be true when the live set exceeds the limit")
	}
	if len(j.disAllowedIps) != 1 || j.disAllowedIps[0] != "192.0.2.9" {
		t.Fatalf("expected 192.0.2.9 to be banned; disAllowedIps = %v", j.disAllowedIps)
	}

	persisted := ipSet(readClientIps(t, email))
	if _, ok := persisted["10.1.0.1"]; !ok {
		t.Errorf("original IP 10.1.0.1 must still be persisted; got %v", persisted)
	}
	if _, ok := persisted["192.0.2.9"]; ok {
		t.Errorf("banned IP 192.0.2.9 must NOT be persisted; got %v", persisted)
	}

	// 3xipl.log must contain the ban line in the exact fail2ban format.
	body, err := os.ReadFile(readIpLimitLogPath())
	if err != nil {
		t.Fatalf("read 3xipl.log: %v", err)
	}
	wantSubstr := "[LIMIT_IP] Email = pr4091-abuse || Disconnecting OLD IP = 192.0.2.9"
	if !contains(string(body), wantSubstr) {
		t.Fatalf("3xipl.log missing expected ban line %q\nfull log:\n%s", wantSubstr, body)
	}
}

// readIpLimitLogPath reads the 3xipl.log path the same way the job
// does via xray.GetIPLimitLogPath but without importing xray here
// just for the path helper (which would pull a lot more deps into the
// test binary). The env-derived log folder is deterministic.
func readIpLimitLogPath() string {
	folder := os.Getenv("XUI_LOG_FOLDER")
	if folder == "" {
		folder = filepath.Join(".", "log")
	}
	return filepath.Join(folder, "3xipl.log")
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
