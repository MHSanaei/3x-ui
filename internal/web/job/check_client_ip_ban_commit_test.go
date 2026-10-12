package job

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"

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
	seedClientIps(t, email, []IPWithTimestamp{{IP: "198.51.100.10", Timestamp: now - 2}})

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
	seedClientIps(t, email, []IPWithTimestamp{{IP: "203.0.113.1", Timestamp: now - 500}})
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
	err := writeBanLines(failingWriter{err: want}, "write-failure@x", []IPWithTimestamp{
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
	seedClientIps(t, email, []IPWithTimestamp{{IP: "203.0.113.50", Timestamp: now - 500}})

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
