package job

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
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

	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
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

func TestRun_DisabledFail2BanSkipsProbeAndBanLog(t *testing.T) {
	setupIntegrationDB(t)
	t.Setenv("XUI_ENABLE_FAIL2BAN", "false")
	marker := fakeFail2BanClient(t)

	const email = "disabled-fail2ban"
	seedInboundWithClient(t, "inbound-disabled-fail2ban", email, 1)

	binDir := t.TempDir()
	accessLog := filepath.Join(t.TempDir(), "access.log")
	t.Setenv("XUI_BIN_FOLDER", binDir)
	configData, err := json.Marshal(map[string]any{
		"log": map[string]any{"access": accessLog},
	})
	if err != nil {
		t.Fatalf("marshal xray config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "config.json"), configData, 0644); err != nil {
		t.Fatalf("write xray config: %v", err)
	}
	if err := os.WriteFile(accessLog, []byte("2026/05/26 12:00:00 from tcp:203.0.113.10:443 accepted tcp:example.com:443 email: disabled-fail2ban\n"), 0644); err != nil {
		t.Fatalf("write access log: %v", err)
	}

	j := NewCheckClientIpJob()
	j.Run()

	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("fail2ban-client should not have been executed, stat error: %v", err)
	}
	if info, err := os.Stat(readIpLimitLogPath()); err == nil && info.Size() > 0 {
		body, _ := os.ReadFile(readIpLimitLogPath())
		t.Fatalf("3xipl.log should be empty when fail2ban is disabled, got:\n%s", body)
	}
	var count int64
	if err := database.GetDB().Model(&model.InboundClientIps{}).Where("client_email = ?", email).Count(&count).Error; err != nil {
		t.Fatalf("count InboundClientIps: %v", err)
	}
	if count != 0 {
		t.Fatalf("disabled fail2ban should not persist IP-limit rows, got %d", count)
	}
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

	inbound, err := j.getInboundByEmail(email)
	if err != nil {
		t.Fatalf("getInboundByEmail: %v", err)
	}
	shouldCleanLog := j.updateInboundClientIps(row, inbound, email, live, true, false)

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
// the limit, the oldest connection is dropped and the most recent one
// keeps the slot (last-IP-wins policy from #3735, restored in #4699).
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
	// is deterministic: 10.1.0.1 is the original (older) and must get
	// banned; 192.0.2.9 joined later and keeps the slot (last IP wins).
	live := []IPWithTimestamp{
		{IP: "10.1.0.1", Timestamp: now - 5},
		{IP: "192.0.2.9", Timestamp: now},
	}

	inbound, err := j.getInboundByEmail(email)
	if err != nil {
		t.Fatalf("getInboundByEmail: %v", err)
	}
	shouldCleanLog := j.updateInboundClientIps(row, inbound, email, live, true, false)

	if !shouldCleanLog {
		t.Fatalf("shouldCleanLog must be true when the live set exceeds the limit")
	}
	if len(j.disAllowedIps) != 1 || j.disAllowedIps[0] != "10.1.0.1" {
		t.Fatalf("expected 10.1.0.1 to be banned; disAllowedIps = %v", j.disAllowedIps)
	}

	persisted := ipSet(readClientIps(t, email))
	if _, ok := persisted["192.0.2.9"]; !ok {
		t.Errorf("newest IP 192.0.2.9 must still be persisted; got %v", persisted)
	}
	if _, ok := persisted["10.1.0.1"]; ok {
		t.Errorf("banned IP 10.1.0.1 must NOT be persisted; got %v", persisted)
	}

	// 3xipl.log must contain the ban line in the exact fail2ban format.
	body, err := os.ReadFile(readIpLimitLogPath())
	if err != nil {
		t.Fatalf("read 3xipl.log: %v", err)
	}
	wantSubstr := "[LIMIT_IP] Email = pr4091-abuse || Disconnecting OLD IP = 10.1.0.1"
	if !contains(string(body), wantSubstr) {
		t.Fatalf("3xipl.log missing expected ban line %q\nfull log:\n%s", wantSubstr, body)
	}
}

// writeXrayAccessLog points bin/config.json at a fresh access.log holding a
// single default-format Xray line (`from tcp:<ip>:<port> accepted … email: <e>`)
// for the given client, so Run() has something to scrape.
func writeXrayAccessLog(t *testing.T, email, ip string) {
	t.Helper()
	binDir := t.TempDir()
	accessLog := filepath.Join(t.TempDir(), "access.log")
	t.Setenv("XUI_BIN_FOLDER", binDir)
	configData, err := json.Marshal(map[string]any{
		"log": map[string]any{"access": accessLog},
	})
	if err != nil {
		t.Fatalf("marshal xray config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "config.json"), configData, 0644); err != nil {
		t.Fatalf("write xray config: %v", err)
	}
	line := "2026/06/02 13:35:53 from tcp:" + ip + ":2387 accepted tcp:example.com:443 email: " + email + "\n"
	if err := os.WriteFile(accessLog, []byte(line), 0644); err != nil {
		t.Fatalf("write access log: %v", err)
	}
}

// #4800: the per-client IP log must populate even when no client has an IP
// limit. Before the fix, Run() only scraped the access log when an IP limit
// was active, so a limit-free install always showed an empty IP log despite
// valid access-log lines. No ban may be written since there's no limit.
func TestRun_CollectsIpsWithoutLimit(t *testing.T) {
	setupIntegrationDB(t)
	t.Setenv("XUI_ENABLE_FAIL2BAN", "true")
	fakeFail2BanClient(t)

	const email = "no-limit-user"
	seedInboundWithClient(t, "inbound-no-limit", email, 0) // limitIp = 0
	writeXrayAccessLog(t, email, "203.0.113.10")

	NewCheckClientIpJob().Run()

	ips := readClientIps(t, email)
	if len(ips) != 1 || ips[0].IP != "203.0.113.10" {
		t.Fatalf("expected the access-log IP to be collected without a limit, got %v", ips)
	}

	if info, err := os.Stat(readIpLimitLogPath()); err == nil && info.Size() > 0 {
		body, _ := os.ReadFile(readIpLimitLogPath())
		t.Fatalf("3xipl.log should be empty with no limit set, got:\n%s", body)
	}
}

// #4963: a stale access-log entry for a renamed/deleted client (its email no
// longer maps to any inbound) must not create or resurrect an
// inbound_client_ips row, and must drop any orphan left behind — instead of
// spamming "failed to fetch inbound settings" every run.
func TestRun_StaleAccessLogEmailIsSkippedAndOrphanDropped(t *testing.T) {
	setupIntegrationDB(t)
	t.Setenv("XUI_ENABLE_FAIL2BAN", "true")
	fakeFail2BanClient(t)

	const staleEmail = "renamed-away"
	// No inbound references staleEmail. Pre-seed an orphan tracking row to
	// confirm the job removes it rather than leaving it to error forever.
	seedClientIps(t, staleEmail, []IPWithTimestamp{{IP: "203.0.113.5", Timestamp: time.Now().Unix()}})
	writeXrayAccessLog(t, staleEmail, "203.0.113.5")

	NewCheckClientIpJob().Run()

	var count int64
	if err := database.GetDB().Model(&model.InboundClientIps{}).Where("client_email = ?", staleEmail).Count(&count).Error; err != nil {
		t.Fatalf("count InboundClientIps: %v", err)
	}
	if count != 0 {
		t.Fatalf("stale-email orphan row should be deleted, got %d row(s)", count)
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
