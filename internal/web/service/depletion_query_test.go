package service

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

func depletedEmailsForTest(t *testing.T, db *gorm.DB, enabled bool) []string {
	t.Helper()
	cond, args := depletedCondWithGlobal(db, depletedClientsScanCond)
	var emails []string
	if err := db.Model(&xray.ClientTraffic{}).Where(cond, args...).Where("enable = ?", enabled).Order("email").Pluck("email", &emails).Error; err != nil {
		t.Fatal(err)
	}
	return emails
}

// The same tests run on SQLite by default and on PostgreSQL with XUI_DB_TYPE
// and XUI_DB_DSN. The existing helper gives PostgreSQL a disposable schema.
func TestDepletionQuerySemantics(t *testing.T) {
	db := initGlobalHeartbeatDB(t)
	clock := globalTrafficClock(t, db, false)
	now := clock.Load()
	fresh := now - globalTrafficFreshWindow.Milliseconds()
	type usage struct{ up, down, at int64 }
	cases := []struct {
		name                  string
		up, down, total, ends int64
		globals               []usage
		depleted              bool
	}{
		{"local equality", 40, 60, 100, 0, nil, true},
		{"local below", 40, 59, 100, 0, nil, false},
		{"expiry equality", 0, 0, 0, now, nil, true},
		{"expiry future", 0, 0, 0, now + 1, nil, false},
		{"delayed start", 0, 0, 0, -100, nil, false},
		{"unlimited", 1000, 1000, 0, 0, []usage{{1000, 1000, now}}, false},
		{"negative quota", 1000, 1000, -1, 0, []usage{{1000, 1000, now}}, false},
		{"global equality", 1, 2, 100, 0, []usage{{40, 60, now}}, true},
		{"fresh boundary", 1, 2, 100, 0, []usage{{40, 60, fresh}}, true},
		{"stale boundary", 1, 2, 100, 0, []usage{{400, 600, fresh - 1}}, false},
		{"stale high live low", 1, 2, 100, 0, []usage{{400, 600, fresh - 1}, {4, 6, now}}, false},
		{"one master exhausts", 1, 2, 100, 0, []usage{{40, 60, now}, {4, 6, now}}, true},
		{"duplicate candidates", 1, 2, 100, 0, []usage{{40, 60, now}, {60, 40, now}}, true},
		{"do not sum masters", 1, 2, 100, 0, []usage{{40, 20, now}, {20, 40, now}}, false},
		{"do not combine directions", 1, 2, 100, 0, []usage{{70, 1, now}, {1, 70, now}}, false},
		{"expiry and quota", 100, 0, 100, now, []usage{{100, 0, now}}, true},
		{"expiry with stale globals", 0, 0, 100, now, []usage{{100, 0, fresh - 1}}, true},
	}
	var expectedEnabled, expectedDisabled []string
	for i, tc := range cases {
		for _, enabled := range []bool{true, false} {
			email := fmt.Sprintf("case-%02d-enabled-%v", i, enabled)
			if err := db.Create(&xray.ClientTraffic{Email: email, Enable: enabled, Up: tc.up, Down: tc.down, Total: tc.total, ExpiryTime: tc.ends}).Error; err != nil {
				t.Fatal(err)
			}
			for j, g := range tc.globals {
				if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: fmt.Sprint(j), Email: email, Up: g.up, Down: g.down, UpdatedAt: g.at}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if tc.depleted {
				if enabled {
					expectedEnabled = append(expectedEnabled, email)
				} else {
					expectedDisabled = append(expectedDisabled, email)
				}
			}
			t.Logf("%s: %s depleted=%v", email, tc.name, tc.depleted)
		}
	}
	for _, enabled := range []bool{true, false} {
		want := expectedEnabled
		if !enabled {
			want = expectedDisabled
		}
		if got := depletedEmailsForTest(t, db, enabled); !slices.Equal(got, want) {
			t.Fatalf("enabled=%v got=%v want=%v", enabled, got, want)
		}
	}
	// Exercise the real disable path and repeated empty-result polls. Existing
	// disabled clients are excluded; duplicate masters must not duplicate writes.
	for poll := range 5 {
		var count int64
		if err := db.Transaction(func(tx *gorm.DB) error {
			_, n, _, err := (&InboundService{}).disableInvalidClients(tx, newTrafficMutationBatch())
			count = n
			return err
		}); err != nil {
			t.Fatal(err)
		}
		want := int64(0)
		if poll == 0 {
			want = int64(len(expectedEnabled))
		}
		if count != want {
			t.Fatalf("poll %d disabled %d want %d", poll, count, want)
		}
	}
}

func TestDepletionQueryFreshnessAndReset(t *testing.T) {
	db := initGlobalHeartbeatDB(t)
	clock := globalTrafficClock(t, db, true)
	seedClientRow(t, "quota", 1, 1, 2, 100)
	assertDepleted := func(want bool) {
		t.Helper()
		got := depletedEmailsForTest(t, db, true)
		if (len(got) == 1) != want {
			t.Fatalf("depleted=%v want %v", got, want)
		}
	}
	assertDepleted(false) // empty; no freshness decision is cached
	for _, up := range []int64{99, 100, 1, 0, 100} {
		pushGlobalTraffic(t, "master", &xray.ClientTraffic{Email: "quota", Up: up})
		assertDepleted(up >= 100)
	}
	clock.Add(globalTrafficFreshWindow.Milliseconds())
	assertDepleted(true) // inclusive 24h
	clock.Add(1)
	assertDepleted(false)
	// Same counters, fresh idle heartbeat: the next poll must enforce again.
	pushGlobalTraffic(t, "master", &xray.ClientTraffic{Email: "quota", Up: 100})
	assertDepleted(true)
	if _, count, _, err := (&InboundService{}).disableInvalidClients(db, newTrafficMutationBatch()); err != nil || count != 1 {
		t.Fatalf("disable count=%d err=%v", count, err)
	}
	pushGlobalTraffic(t, "master", &xray.ClientTraffic{Email: "quota"})
	if got := readTraffic(t, db, "quota"); got.Enable || got.Up != 1 || got.Down != 2 {
		t.Fatalf("reset must not re-enable a disabled client or overwrite local counters: %+v", got)
	}
}

func TestDepletionQueryConcurrentCommit(t *testing.T) {
	db := initGlobalHeartbeatDB(t)
	clock := globalTrafficClock(t, db, false)
	seedClientRow(t, "concurrent", 1, 1, 2, 100)
	pushGlobalTraffic(t, "master", &xray.ClientTraffic{Email: "concurrent", Up: 1})
	for _, tc := range []struct {
		up            int64
		before, after bool
	}{{100, false, true}, {0, true, false}} {
		written, release := make(chan struct{}), make(chan struct{})
		result := make(chan error, 1)
		go func() {
			result <- db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Model(&model.ClientGlobalTraffic{}).Where("email = ?", "concurrent").Updates(map[string]any{"up": tc.up, "updated_at": clock.Load()}).Error; err != nil {
					return err
				}
				close(written)
				<-release
				return nil
			})
		}()
		select {
		case <-written:
		case err := <-result:
			t.Fatalf("writer before query: %v", err)
		case <-time.After(10 * time.Second):
			close(release)
			t.Fatal("writer did not reach commit barrier")
		}
		// Release the writer even if a reader assertion fails.
		func() {
			defer close(release)
			if got := len(depletedEmailsForTest(t, db, true)) == 1; got != tc.before {
				t.Errorf("uncommitted global visible: depleted=%v want %v", got, tc.before)
			}
		}()
		if err := <-result; err != nil {
			t.Fatal(err)
		}
		if got := len(depletedEmailsForTest(t, db, true)) == 1; got != tc.after {
			t.Fatalf("committed change not visible on next poll: depleted=%v want %v", got, tc.after)
		}
	}
}

// Reuse the lifecycle regression fixtures on both backends. In particular a
// capped catch-up must retain global usage, while a real renewal must clear it.
func TestDepletionQueryRenewals(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"automatic renewal", TestAutoRenewClients_RenewedClientLosesCrossPanelTraffic},
		{"capped renewal", TestAutoRenewClients_TruncatedCatchUpKeepsCrossPanelTraffic},
		{"manual quota extension", TestBulkAdjust_ReenablesOverQuota_WhenAddBytesClearsQuota},
		{"manually disabled", TestBulkAdjust_DoesNotReenable_ManuallyDisabledNotDepleted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanup, err := testpg.IsolatePackage("depletion_renewal")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(cleanup)
			tc.run(t)
		})
	}
}
