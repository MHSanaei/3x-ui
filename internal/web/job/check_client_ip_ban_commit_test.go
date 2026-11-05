package job

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

// installClientIpCommitFailure fails the transaction at COMMIT, not at a
// statement, so every write inside it succeeds before the rollback.
func installClientIpCommitFailure(t *testing.T) {
	t.Helper()
	db := database.GetDB()
	switch db.Name() {
	case "sqlite":
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("get sql DB: %v", err)
		}
		// foreign_keys is per connection, so the injection only holds while the
		// pool cannot hand the scan a fresh one with the pragma back at OFF.
		sqlDB.SetMaxOpenConns(1)
		for _, statement := range []string{
			"DROP TRIGGER IF EXISTS commitfail_on_ips",
			"DROP TABLE IF EXISTS commitfail_child",
			"DROP TABLE IF EXISTS commitfail_parent",
			"PRAGMA foreign_keys = ON",
			"CREATE TABLE commitfail_parent (id INTEGER PRIMARY KEY)",
			"CREATE TABLE commitfail_child (parent_id INTEGER, FOREIGN KEY(parent_id) REFERENCES commitfail_parent(id) DEFERRABLE INITIALLY DEFERRED)",
			"CREATE TRIGGER commitfail_on_ips AFTER UPDATE OF ips ON inbound_client_ips BEGIN INSERT INTO commitfail_child(parent_id) VALUES (999); END",
		} {
			if err := db.Exec(statement).Error; err != nil {
				t.Fatalf("install SQLite commit-failure injection %q: %v", statement, err)
			}
		}
		t.Cleanup(func() {
			_ = db.Exec("DROP TRIGGER IF EXISTS commitfail_on_ips").Error
			_ = db.Exec("DROP TABLE IF EXISTS commitfail_child").Error
			_ = db.Exec("DROP TABLE IF EXISTS commitfail_parent").Error
			_ = db.Exec("PRAGMA foreign_keys = OFF").Error
		})
	case "postgres":
		for _, statement := range []string{
			"DROP TABLE IF EXISTS commitfail_child",
			"DROP TABLE IF EXISTS commitfail_parent",
			"CREATE TABLE commitfail_parent (id bigint PRIMARY KEY)",
			"CREATE TABLE commitfail_child (id bigint PRIMARY KEY, parent_id bigint REFERENCES commitfail_parent(id) DEFERRABLE INITIALLY DEFERRED)",
		} {
			if err := db.Exec(statement).Error; err != nil {
				t.Fatalf("install PostgreSQL commit-failure injection %q: %v", statement, err)
			}
		}
		const callbackName = "test:client_ip_commit_failure"
		if err := db.Callback().Update().After("gorm:update").Register(callbackName, func(tx *gorm.DB) {
			stmt := tx.Statement
			if stmt == nil || stmt.Schema == nil || stmt.Schema.Table != "inbound_client_ips" {
				return
			}
			result := tx.Session(&gorm.Session{NewDB: true}).Exec("INSERT INTO commitfail_child (id, parent_id) VALUES (1, 999)")
			if result.Error != nil {
				_ = tx.AddError(result.Error)
			}
		}); err != nil {
			t.Fatalf("register PostgreSQL commit-failure callback: %v", err)
		}
		t.Cleanup(func() {
			_ = db.Callback().Update().Remove(callbackName)
			_ = db.Exec("DROP TABLE IF EXISTS commitfail_child").Error
			_ = db.Exec("DROP TABLE IF EXISTS commitfail_parent").Error
		})
	default:
		t.Fatalf("unsupported test database dialect %q", db.Name())
	}
}

// A fail2ban line is not a row a rollback can take back, so nothing may be
// appended, and bannedSeen not advanced, until the scan has committed.
func TestProcessObserved_CommitFailureDoesNotPublishBan(t *testing.T) {
	setupIntegrationDB(t)

	const email = "rollback-must-not-ban@x"
	seedLinkedInboundWithClient(t, "rollback-must-not-ban", email, 1)
	now := time.Now().Unix()
	seedClientIps(t, email, []model.ClientIpEntry{{IP: "198.51.100.10", Timestamp: now - 2}})

	installClientIpCommitFailure(t)

	j := NewCheckClientIpJob()
	cleaned := j.processObserved(map[string]map[string]int64{
		email: {
			"198.51.100.10": now - 1,
			"198.51.100.11": now,
		},
	}, true, true)
	if cleaned {
		t.Errorf("processObserved reported a published ban after the commit failed")
	}
	if got := ipSet(readClientIps(t, email)); len(got) != 1 || got["198.51.100.10"] != now-2 {
		t.Errorf("rolled-back IP row = %v, want only the original client address", got)
	}
	if _, err := os.Stat(readIpLimitLogPath()); !os.IsNotExist(err) {
		body, _ := os.ReadFile(readIpLimitLogPath())
		t.Errorf("the rollback still touched the fail2ban trigger file (stat=%v):\n%s", err, body)
	}
	if _, seen := j.bannedSeen[email+"|198.51.100.10"]; seen {
		t.Errorf("the rollback advanced bannedSeen and would suppress the retry")
	}
}

// The committed row has already dropped the address, so a bannedSeen entry
// recorded ahead of a failed write would suppress the ban for good.
func TestProcessObserved_PublishFailureLeavesBanRetryable(t *testing.T) {
	setupIntegrationDB(t)

	const email = "publish-failure@x"
	seedLinkedInboundWithClient(t, "publish-failure", email, 1)
	now := time.Now().Unix()

	// A directory where the log file belongs makes every open fail.
	if err := os.MkdirAll(readIpLimitLogPath(), 0o755); err != nil {
		t.Fatalf("block the log path: %v", err)
	}

	j := NewCheckClientIpJob()
	observed := map[string]map[string]int64{
		email: {"198.51.100.20": now - 1, "198.51.100.21": now},
	}
	if cleaned := j.processObserved(observed, true, true); cleaned {
		t.Errorf("processObserved reported a publication that could not happen")
	}
	for key := range j.bannedSeen {
		t.Errorf("bannedSeen recorded %q although nothing was written", key)
	}
}

// A client back under its limit produces no candidates, so pruning cannot live
// in the selection step: a surviving entry suppresses its next real ban.
func TestProcessObserved_ForgetsBannedSeenWhenClientReturnsUnderLimit(t *testing.T) {
	setupIntegrationDB(t)

	const email = "prune-banned-seen@x"
	seedLinkedInboundWithClient(t, "prune-banned-seen", email, 1)
	now := time.Now().Unix()
	seedClientIps(t, email, []model.ClientIpEntry{{IP: "203.0.113.1", Timestamp: now - 500}})
	j := NewCheckClientIpJob()

	j.processObserved(map[string]map[string]int64{
		email: {"203.0.113.1": now - 400, "203.0.113.2": now - 300},
	}, true, true)
	if got := banLineCount(t, email); got != 1 {
		t.Fatalf("ban lines after the first scan = %d, want 1", got)
	}

	// Back under the limit: no candidates, so the stale entry must be dropped here.
	j.processObserved(map[string]map[string]int64{
		email: {"203.0.113.1": now - 400},
	}, true, true)
	if len(j.bannedSeen) != 0 {
		t.Fatalf("bannedSeen = %v, want empty once the client is under its limit", j.bannedSeen)
	}

	j.processObserved(map[string]map[string]int64{
		email: {"203.0.113.1": now - 400, "203.0.113.3": now},
	}, true, true)
	if got := banLineCount(t, email); got != 2 {
		t.Fatalf("ban lines after the client goes over again = %d, want 2", got)
	}
}

type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }

// A dropped write error would let publishBans record an address the jail never
// sees, so it has to reach the caller.
func TestWriteBanLinesSurfacesWriteFailure(t *testing.T) {
	want := errors.New("no space left on device")
	err := writeBanLines(failingWriter{err: want}, "write-failure@x", []model.ClientIpEntry{
		{IP: "203.0.113.9", Timestamp: time.Now().Unix()},
	})
	if !errors.Is(err, want) {
		t.Fatalf("writeBanLines error = %v, want the writer's own error", err)
	}
}

// A committed over-limit scan writes its line and hands the client on.
func TestProcessObserved_PublishesBanForCommittedScan(t *testing.T) {
	setupIntegrationDB(t)

	const email = "published-ban@x"
	seedLinkedInboundWithClient(t, "published-ban", email, 1)
	now := time.Now().Unix()
	seedClientIps(t, email, []model.ClientIpEntry{{IP: "203.0.113.50", Timestamp: now - 500}})

	j := NewCheckClientIpJob()
	if cleaned := j.processObserved(map[string]map[string]int64{
		email: {"203.0.113.50": now - 400, "203.0.113.51": now},
	}, true, true); !cleaned {
		t.Fatalf("a published ban must report the access log as worth cleaning")
	}
	if got := banLineCount(t, email); got != 1 {
		t.Fatalf("ban lines = %d, want 1", got)
	}
	if _, seen := j.bannedSeen[email+"|203.0.113.50"]; !seen {
		t.Fatalf("a published address must be recorded so the next scan does not repeat it")
	}
}

const (
	rawStream      = `{"network":"tcp","security":"none"}`
	shadowsocksUDP = `{"method":"2022-blake3-aes-256-gcm","network":"tcp,udp","clients":[{"email":"mallory","limitIp":1,"enable":true}]}`
)

// One datagram with a forged source puts any address on the core's online list,
// so an address seen for such a client must not reach the file fail2ban follows.
func TestPublishBans_WithholdsAddressesOnlyADatagramVouchesFor(t *testing.T) {
	const email, forged = "mallory", "192.0.2.77"
	node := 1
	for _, tc := range []struct {
		name      string
		second    *model.Inbound
		wantInLog bool
	}{
		{"only a TCP inbound", nil, true},
		{"also on Shadowsocks with UDP", &model.Inbound{Enable: true}, false},
		{"Shadowsocks with UDP is disabled", &model.Inbound{Enable: false}, true},
		// Node syncs merge the node's observations into the same row (#6587).
		{"Shadowsocks with UDP runs on a node", &model.Inbound{Enable: true, NodeID: &node}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupIntegrationDB(t)
			db := database.GetDB()
			// The scan resolves a client to its lowest inbound id, which is this one.
			inbound := seedLinkedInboundWithClient(t, "inbound-vless", email, 1)
			if tc.second != nil {
				tc.second.Tag, tc.second.Port, tc.second.Protocol = "inbound-ss", 4322, model.Shadowsocks
				tc.second.Settings, tc.second.StreamSettings = shadowsocksUDP, rawStream
				if err := db.Create(tc.second).Error; err != nil {
					t.Fatalf("seed second inbound: %v", err)
				}
				var client model.ClientRecord
				if err := db.Where("email = ?", email).First(&client).Error; err != nil {
					t.Fatalf("read client: %v", err)
				}
				if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: tc.second.Id}).Error; err != nil {
					t.Fatalf("link second inbound: %v", err)
				}
			}

			now := time.Now().Unix()
			row := seedClientIps(t, email, []model.ClientIpEntry{{IP: forged, Timestamp: now - 5}})
			live := []model.ClientIpEntry{{IP: forged, Timestamp: now - 5}, {IP: "203.0.113.4", Timestamp: now}}
			overLimit, _ := NewCheckClientIpJob().enforceIpLimitForTest(t, row, inbound, email, 1, live, true)

			if !overLimit {
				t.Error("the client is over its limit and must still be disconnected")
			}
			body, _ := os.ReadFile(readIpLimitLogPath())
			if got := contains(string(body), forged); got != tc.wantInLog {
				t.Errorf("older address in the fail2ban log = %v, want %v:\n%s", got, tc.wantInLog, body)
			}
		})
	}
}

// getInboundByEmail's settings scan is all that ties such a client to an inbound.
func TestPublishBans_ClientOutsideTheRelationIsJudgedByItsResolvedInbound(t *testing.T) {
	setupIntegrationDB(t)
	const email, forged = "mallory", "192.0.2.77"
	inbound := seedInboundOnlyWithClient(t, "inbound-ss", email, 1)
	inbound.Protocol, inbound.Settings, inbound.StreamSettings = model.Shadowsocks, shadowsocksUDP, rawStream

	now := time.Now().Unix()
	row := seedClientIps(t, email, []model.ClientIpEntry{{IP: forged, Timestamp: now - 5}})
	live := []model.ClientIpEntry{{IP: forged, Timestamp: now - 5}, {IP: "203.0.113.4", Timestamp: now}}
	NewCheckClientIpJob().enforceIpLimitForTest(t, row, inbound, email, 1, live, true)

	if body, _ := os.ReadFile(readIpLimitLogPath()); contains(string(body), forged) {
		t.Errorf("an address only a datagram vouches for was written for fail2ban:\n%s", body)
	}
}

func TestAcceptsUnverifiedSources(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol model.Protocol
		settings string
		stream   string
		want     bool
	}{
		{"shadowsocks tcp,udp", model.Shadowsocks, shadowsocksUDP, rawStream, true},
		{"shadowsocks network list", model.Shadowsocks, `{"network":["tcp","UDP"]}`, rawStream, true},
		{"shadowsocks tcp", model.Shadowsocks, `{"network":"tcp"}`, rawStream, false},
		{"shadowsocks core default", model.Shadowsocks, `{}`, rawStream, false},
		{"vless raw", model.VLESS, `{"clients":[]}`, rawStream, false},
		{"vless ignores settings.network", model.VLESS, `{"network":"udp"}`, rawStream, false},
		{"vless kcp", model.VLESS, `{"clients":[]}`, `{"network":"kcp"}`, true},
		{"vless mkcp by its newer key", model.VLESS, `{"clients":[]}`, `{"network":"tcp","method":"mKCP"}`, true},
		{"hysteria proves the address in its QUIC handshake", model.Hysteria, `{"clients":[]}`, `{"network":"hysteria"}`, false},
	} {
		inbound := &model.Inbound{Protocol: tc.protocol, Settings: tc.settings, StreamSettings: tc.stream}
		if got := acceptsUnverifiedSources(inbound); got != tc.want {
			t.Errorf("%s: acceptsUnverifiedSources = %v, want %v", tc.name, got, tc.want)
		}
	}
}
