package job

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
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
	seedInboundOnlyWithClient(t, tag, email, limitIp)
}

func seedInboundOnlyWithClient(t *testing.T, tag, email string, limitIp int) *model.Inbound {
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
	return inbound
}

func seedLinkedInboundWithClient(t *testing.T, tag, email string, limitIp int) *model.Inbound {
	t.Helper()
	inbound := seedInboundOnlyWithClient(t, tag, email, limitIp)
	client := &model.ClientRecord{Email: email, LimitIP: limitIp}
	if err := database.GetDB().Create(client).Error; err != nil {
		t.Fatalf("seed client record: %v", err)
	}
	link := &model.ClientInbound{ClientId: client.Id, InboundId: inbound.Id}
	if err := database.GetDB().Create(link).Error; err != nil {
		t.Fatalf("seed client inbound link: %v", err)
	}
	return inbound
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

// With the access-log fallback removed, an unavailable online-stats API (xray
// down, as in this unit test) must make Run a clean no-op: no fail2ban probe, no
// ban log, and no inbound_client_ips rows — never a crash or partial work.
func TestRun_NoOpWhenOnlineApiUnavailable(t *testing.T) {
	setupIntegrationDB(t)
	t.Setenv("XUI_ENABLE_FAIL2BAN", "true")
	marker := fakeFail2BanClient(t)

	const email = "no-api-user"
	seedInboundWithClient(t, "inbound-no-api", email, 1)

	NewCheckClientIpJob().Run()

	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("fail2ban-client should not have been probed when the online API is unavailable, stat error: %v", err)
	}
	if info, err := os.Stat(readIpLimitLogPath()); err == nil && info.Size() > 0 {
		body, _ := os.ReadFile(readIpLimitLogPath())
		t.Fatalf("3xipl.log should be empty when Run no-ops, got:\n%s", body)
	}
	var count int64
	if err := database.GetDB().Model(&model.InboundClientIps{}).Where("client_email = ?", email).Count(&count).Error; err != nil {
		t.Fatalf("count InboundClientIps: %v", err)
	}
	if count != 0 {
		t.Fatalf("no IP-limit rows should be persisted when Run no-ops, got %d", count)
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
	shouldCleanLog, banned := j.updateInboundClientIps(database.GetDB(), row, inbound, email, 3, live, true, false)

	if shouldCleanLog {
		t.Fatalf("shouldCleanLog must be false, nothing should have been banned with 1 live ip under limit 3")
	}
	if banned {
		t.Fatalf("banned must be false with 1 live ip under limit 3")
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
	shouldCleanLog, banned := j.updateInboundClientIps(database.GetDB(), row, inbound, email, 1, live, true, false)

	if !shouldCleanLog {
		t.Fatalf("shouldCleanLog must be true when the live set exceeds the limit")
	}
	if !banned {
		t.Fatalf("banned must be true when the live set exceeds the limit")
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

// #4800: per-client IP tracking must populate even when no client has an IP
// limit. processObserved records observed IPs for the panel regardless of any
// limit; only enforcement is gated, so a limit-free install still shows IPs. No
// ban may be written since there's no limit.
func TestProcessObserved_CollectsIpsWithoutLimit(t *testing.T) {
	setupIntegrationDB(t)

	const email = "no-limit-user"
	seedInboundWithClient(t, "inbound-no-limit", email, 0) // limitIp = 0

	observed := map[string]map[string]int64{
		email: {"203.0.113.10": time.Now().Unix()},
	}
	NewCheckClientIpJob().processObserved(observed, true, true)

	ips := readClientIps(t, email)
	if len(ips) != 1 || ips[0].IP != "203.0.113.10" {
		t.Fatalf("expected the observed IP to be collected without a limit, got %v", ips)
	}

	if info, err := os.Stat(readIpLimitLogPath()); err == nil && info.Size() > 0 {
		body, _ := os.ReadFile(readIpLimitLogPath())
		t.Fatalf("3xipl.log should be empty with no limit set, got:\n%s", body)
	}
}

// #4963: an observed IP for a renamed/deleted client (its email no longer maps
// to any inbound) must not create or resurrect an inbound_client_ips row, and
// must drop any orphan left behind — instead of erroring every run.
func TestProcessObserved_StaleEmailIsSkippedAndOrphanDropped(t *testing.T) {
	setupIntegrationDB(t)

	const staleEmail = "renamed-away"
	// No inbound references staleEmail. Pre-seed an orphan tracking row to
	// confirm the job removes it rather than leaving it to error forever.
	seedClientIps(t, staleEmail, []IPWithTimestamp{{IP: "203.0.113.5", Timestamp: time.Now().Unix()}})

	observed := map[string]map[string]int64{
		staleEmail: {"203.0.113.5": time.Now().Unix()},
	}
	NewCheckClientIpJob().processObserved(observed, true, true)

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

// the exact clients/client_inbounds relation must win over the substring scan,
// so a client is resolved to its own inbound even when another inbound holds a
// superstring email.
func TestGetInboundByEmailUsesClientInboundLink(t *testing.T) {
	setupIntegrationDB(t)

	want := seedLinkedInboundWithClient(t, "linked-inbound", "exact@example.com", 1)
	seedInboundOnlyWithClient(t, "other-inbound", "not-exact@example.com", 1)

	got, err := (&CheckClientIpJob{}).getInboundByEmail("exact@example.com")
	if err != nil {
		t.Fatalf("getInboundByEmail returned error: %v", err)
	}
	if got.Id != want.Id {
		t.Fatalf("getInboundByEmail returned inbound %d, want %d", got.Id, want.Id)
	}
}

// the substring fallback must still verify the exact email inside settings, so
// "ann@example.com" does not match an inbound holding "joann@example.com".
func TestGetInboundByEmailRejectsSubstringFallbackMatch(t *testing.T) {
	setupIntegrationDB(t)

	seedInboundOnlyWithClient(t, "substring-only", "joann@example.com", 1)

	if got, err := (&CheckClientIpJob{}).getInboundByEmail("ann@example.com"); err == nil {
		t.Fatalf("substring email matched inbound %d; want no exact match", got.Id)
	}
}

// hasLimitIp gates every 10s scan on the normalized clients table: a bare
// "limitIp":0 in settings JSON (which the old LIKE scan matched and parsed)
// must not enable enforcement, while a single clients.limit_ip > 0 row must.
func TestHasLimitIp_ProbesClientRecords(t *testing.T) {
	setupIntegrationDB(t)
	j := &CheckClientIpJob{}

	if j.hasLimitIp() {
		t.Fatal("hasLimitIp = true on an empty database")
	}

	seedLinkedInboundWithClient(t, "no-limit", "nolimit@example.com", 0)
	if j.hasLimitIp() {
		t.Fatal("hasLimitIp = true with only limit_ip=0 clients")
	}

	limited := &model.ClientRecord{Email: "limited@example.com", LimitIP: 2}
	if err := database.GetDB().Create(limited).Error; err != nil {
		t.Fatalf("seed limited client: %v", err)
	}
	if !j.hasLimitIp() {
		t.Fatal("hasLimitIp = false with a limit_ip=2 client present")
	}
}
