package service

import (
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestClientTrafficLifecyclePostgres(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || os.Getenv("XUI_DB_DSN") == "" {
		t.Skip("requires a disposable PostgreSQL database")
	}
	for _, test := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"inbound save", TestInboundEditWritesOnlyDriftedTraffic},
		{"no update per unchanged client", TestInboundEditSendsNoUpdateForUnchangedClients},
		{"null lifecycle columns", TestInboundEditRepairsNullLifecycleColumns},
		{"fields and errors", TestUpdateClientStatPreservesLifecycleAndCounters},
		{"row locks", checkUpdateClientStatRowLocks},
		{"one connection", TestUpdateClientTrafficsStaysOnOneConnection},
		{"rename onto existing stats", TestUpdateInboundClientRenameOntoExistingStatStaysOnOneConnection},
		{"master lifecycle", TestInboundUpdateFromMasterAppliesClientLifecycle},
		{"sibling lifecycle", TestNodeInboundPushDisablingASharedClientRemovesItFromEveryHandler},
		{"detached lifecycle", TestUpdate_NoInbound_SyncsEnforcementRow},
		{"reattachment", TestAddClientStat_MultiInboundReattachStaysIdempotent},
	} {
		t.Run(test.name, func(t *testing.T) {
			cleanup, err := testpg.IsolatePackage("client_traffic_lifecycle")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(cleanup)
			test.run(t)
		})
	}
}

func execLifecycleTestSQL(t testing.TB, queries ...string) {
	t.Helper()
	for _, query := range queries {
		if err := database.GetDB().Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func auditLifecycleWrites(t *testing.T) func() int64 {
	t.Helper()
	execLifecycleTestSQL(t, "CREATE TABLE lifecycle_write_audit (email text)")
	db := database.GetDB()
	if db.Name() == "postgres" {
		execLifecycleTestSQL(t, `CREATE FUNCTION audit_lifecycle_write() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN INSERT INTO lifecycle_write_audit VALUES (NEW.email); RETURN NEW; END;
		$$`, `CREATE TRIGGER audit_lifecycle_write AFTER UPDATE ON client_traffics
			FOR EACH ROW EXECUTE FUNCTION audit_lifecycle_write()`)
	} else {
		execLifecycleTestSQL(t, `CREATE TRIGGER audit_lifecycle_write AFTER UPDATE ON client_traffics
			BEGIN INSERT INTO lifecycle_write_audit VALUES (NEW.email); END`)
	}
	return func() int64 {
		t.Helper()
		var count int64
		if err := db.Table("lifecycle_write_audit").Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		return count
	}
}

func seedTrafficLifecycleInbound(t testing.TB, svc *InboundService, clients []model.Client) *model.Inbound {
	t.Helper()
	inbound, _, err := svc.AddInbound(&model.Inbound{
		UserId: 1, Tag: "lifecycle-edit", Remark: "before", Port: 43021,
		Enable: true, Protocol: model.VLESS, Settings: clientsSettings(t, clients),
		StreamSettings: `{"network":"tcp","security":"none"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	return inbound
}

func TestInboundEditWritesOnlyDriftedTraffic(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	ib := seedTrafficLifecycleInbound(t, svc, []model.Client{
		{Email: "stable@x", ID: "00000000-0000-4000-8000-000000000001", Enable: true, TotalGB: 1 << 30},
		{Email: "drifted@x", ID: "00000000-0000-4000-8000-000000000002", Enable: true, TotalGB: 2 << 30},
	})
	stable := readClientTraffic(t, "stable@x")
	execLifecycleTestSQL(t, "UPDATE client_traffics SET total = 0, up = 73, down = 19, last_online = 100, reset_count = 2 WHERE email = 'drifted@x'")
	drifted := readClientTraffic(t, "drifted@x")
	drifted.Total = 2 << 30
	count := auditLifecycleWrites(t)
	form := *ib
	form.Remark = "edited"
	if _, _, err := svc.UpdateInbound(&form); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 1 {
		t.Fatalf("row trigger fired %d times, want only the drift repair", got)
	}
	if got := readClientTraffic(t, stable.Email); got != stable {
		t.Fatalf("identical row changed: %+v, want %+v", got, stable)
	}
	if got := readClientTraffic(t, drifted.Email); got != drifted {
		t.Fatalf("drift repair = %+v, want %+v", got, drifted)
	}
	form.Remark = "edited again"
	if _, _, err := svc.UpdateInbound(&form); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 1 {
		t.Fatalf("second edit fired additional row triggers: got %d total", got)
	}
	saved, err := svc.GetInbound(ib.Id)
	if err != nil || saved.Remark != "edited again" {
		t.Fatalf("inbound edit did not persist: %v, %v", saved, err)
	}
}

func TestUpdateClientStatPreservesLifecycleAndCounters(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	svc := &InboundService{}
	count := auditLifecycleWrites(t)
	for _, tc := range []struct {
		name string
		set  func(*xray.ClientTraffic, *model.Client)
	}{
		{"case-only rename", func(row *xray.ClientTraffic, c *model.Client) { c.Email = strings.ToUpper(row.Email) }},
		{"disable", func(_ *xray.ClientTraffic, c *model.Client) { c.Enable = false }},
		{"enable", func(row *xray.ClientTraffic, c *model.Client) { row.Enable, c.Enable = false, true }},
		{"clear total", func(_ *xray.ClientTraffic, c *model.Client) { c.TotalGB = 0 }},
		{"delayed expiry", func(_ *xray.ClientTraffic, c *model.Client) { c.ExpiryTime = -86400000 }},
		{"interval", func(_ *xray.ClientTraffic, c *model.Client) { c.Reset = 30 }},
		{"billing day", func(_ *xray.ClientTraffic, c *model.Client) { c.ResetDay = 15 }},
		{"weekly day", func(_ *xray.ClientTraffic, c *model.Client) { c.ResetWeekday = 7 }},
		{"max renewals", func(_ *xray.ClientTraffic, c *model.Client) { c.ResetMax = 4 }},
		{"repair nulls", func(_ *xray.ClientTraffic, _ *model.Client) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := xray.ClientTraffic{
				Email: tc.name + "@x", Enable: true, Total: 1 << 30,
				Up: 3, Down: 5, LastOnline: 7, LastSubFetch: 9, ResetCount: 11, InboundId: 13,
			}
			client := model.Client{Email: row.Email, Enable: row.Enable, TotalGB: row.Total}
			tc.set(&row, &client)
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			if tc.name == "repair nulls" {
				if err := db.Model(&row).Updates(map[string]any{
					"enable": nil, "total": nil, "expiry_time": nil, "reset": nil,
					"reset_day": nil, "reset_weekday": nil, "reset_max": nil,
				}).Error; err != nil {
					t.Fatal(err)
				}
			}
			before := count()
			if err := db.Transaction(func(tx *gorm.DB) error { return svc.UpdateClientStat(tx, row.Email, &client) }); err != nil {
				t.Fatal(err)
			}
			row.Email, row.Enable, row.Total, row.ExpiryTime = client.Email, client.Enable, client.TotalGB, client.ExpiryTime
			row.Reset, row.ResetDay, row.ResetWeekday, row.ResetMax = client.Reset, client.ResetDay, client.ResetWeekday, client.ResetMax
			if got := readClientTraffic(t, client.Email); got != row || count() != before+1 {
				t.Fatalf("updated row = %+v, want %+v and exactly one write", got, row)
			}
			if tc.name == "repair nulls" {
				var nulls int64
				if err := db.Model(xray.ClientTraffic{}).Where("id = ?", row.Id).
					Where("(enable IS NULL OR total IS NULL OR expiry_time IS NULL OR reset IS NULL OR reset_day IS NULL OR reset_weekday IS NULL OR reset_max IS NULL)").
					Count(&nulls).Error; err != nil || nulls != 0 {
					t.Fatalf("NULL repair = %d rows remain, error=%v", nulls, err)
				}
			}
		})
	}
	before := count()
	if err := svc.UpdateClientStat(db, "missing@x", &model.Client{Email: "missing@x"}); err != nil || count() != before {
		t.Fatalf("missing row: error=%v, writes=%d", err, count()-before)
	}
	invalid := xray.ClientTraffic{Email: "invalid@x", ResetWeekday: 8}
	if err := db.Create(&invalid).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateClientStat(db, invalid.Email, &model.Client{Email: invalid.Email, ResetWeekday: 8}); err == nil ||
		err.Error() != "client resetWeekday must be between 0 and 7, got: 8\n" || count() != before {
		t.Fatalf("equal invalid row: error=%v, writes=%d", err, count()-before)
	}
	if err := svc.UpdateClientStat(db, "disable@x", &model.Client{Email: "enable@x"}); !lifecycleUniqueViolation(err) {
		t.Fatalf("rename onto existing email: expected unique constraint error, got %v", err)
	}
	if got := readClientTraffic(t, "disable@x"); got.Total != 1<<30 || count() != before {
		t.Fatalf("failed rename changed source: %+v", got)
	}
}

func lifecycleUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	var sqliteErr sqlite3.Error
	return errors.As(err, &pgErr) && pgErr.Code == "23505" ||
		errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
}

func checkUpdateClientStatRowLocks(t *testing.T) {
	t.Helper()
	setupBulkDB(t)
	db := database.GetDB()
	svc := &InboundService{}
	for _, test := range []struct {
		name       string
		total      int64
		wantLocked bool
	}{
		{"identical", 1000, false},
		{"changed", 2000, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := xray.ClientTraffic{Email: test.name + "@x", Enable: true, Total: 1000}
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			tx := db.Begin()
			if tx.Error != nil {
				t.Fatal(tx.Error)
			}
			defer func() { _ = tx.Rollback().Error }()
			client := model.Client{Email: row.Email, Enable: true, TotalGB: test.total}
			if err := svc.UpdateClientStat(tx, row.Email, &client); err != nil {
				t.Fatal(err)
			}

			// Probe on another connection while the UPDATE transaction is still open.
			err := db.Transaction(func(other *gorm.DB) error {
				var locked xray.ClientTraffic
				return other.Clauses(clause.Locking{Strength: "UPDATE", Options: "NOWAIT"}).
					Where("id = ?", row.Id).Take(&locked).Error
			})
			if test.wantLocked {
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
					t.Fatalf("changed update should hold a row lock (SQLSTATE 55P03), got %v", err)
				}
			} else if err != nil {
				t.Fatalf("identical update should leave the row available, got %v", err)
			}
			if err := tx.Commit().Error; err != nil {
				t.Fatal(err)
			}
			if got := readClientTraffic(t, row.Email).Total; got != test.total {
				t.Fatalf("committed total = %d, want %d", got, test.total)
			}
		})
	}
}

// A form save cannot change clients, yet it sent one UPDATE per client it kept: on
// PostgreSQL a 100k-client inbound spent about a minute on them and the request died.
func TestInboundEditSendsNoUpdateForUnchangedClients(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	clients := makeScaleClients(sqlInChunk + 1)
	for i := range clients {
		clients[i].TotalGB = 1 << 30
	}
	ib := seedTrafficLifecycleInbound(t, svc, clients)
	drifted := clients[len(clients)-1].Email
	execLifecycleTestSQL(t, "UPDATE client_traffics SET total = 0 WHERE email = '"+drifted+"'")

	var updates atomic.Int64
	if err := database.GetDB().Callback().Update().After("gorm:update").Register("test:count_lifecycle_updates", func(tx *gorm.DB) {
		if tx.Statement.Table == "client_traffics" {
			updates.Add(1)
		}
	}); err != nil {
		t.Fatal(err)
	}
	form := *ib
	form.Remark = "edited"
	if _, _, err := svc.UpdateInbound(&form); err != nil {
		t.Fatal(err)
	}
	if got := updates.Load(); got != 1 {
		t.Fatalf("save sent %d client_traffics UPDATEs for %d clients, want 1 for the drifted row", got, len(clients))
	}
	if got := readClientTraffic(t, drifted).Total; got != 1<<30 {
		t.Fatalf("drifted total = %d, want it repaired to %d", got, 1<<30)
	}
}

// NULL is not the zero the settings hold; the row comparison must still call it drift.
func TestInboundEditRepairsNullLifecycleColumns(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	columns := []string{"enable", "expiry_time", "total", "reset", "reset_day", "reset_max", "reset_weekday"}
	clients := makeScaleClients(len(columns))
	ib := seedTrafficLifecycleInbound(t, svc, clients)
	for i, column := range columns {
		execLifecycleTestSQL(t, "UPDATE client_traffics SET "+column+" = NULL WHERE email = '"+clients[i].Email+"'")
	}
	form := *ib
	form.Remark = "edited"
	if _, _, err := svc.UpdateInbound(&form); err != nil {
		t.Fatal(err)
	}
	for i, column := range columns {
		var nulls int64
		if err := database.GetDB().Model(xray.ClientTraffic{}).
			Where("email = ? AND "+column+" IS NULL", clients[i].Email).Count(&nulls).Error; err != nil {
			t.Fatal(err)
		}
		if nulls != 0 {
			t.Errorf("a NULL %s survived the save", column)
		}
	}
}
