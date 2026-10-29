package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type snapshotReadLog struct {
	logger.Interface
	statements []string
}

func (l *snapshotReadLog) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	q, _ := fc()
	l.statements = append(l.statements, q)
}

func TestNodeSnapshotUnchangedAvoidsSettingsAndDetectsDirectEdits(t *testing.T) {
	initNodeSnapshotDB(t, filepath.Join(t.TempDir(), "snapshot.db"))
	db := database.GetDB()
	ib := model.Inbound{UserId: 1, Tag: "snapshot", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[{"id":"uuid","email":"client","enable":true}]}`}
	if err := db.Create(&ib).Error; err != nil {
		t.Fatal(err)
	}
	client := model.ClientRecord{Email: "client", UUID: "uuid", Enable: true}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: "client", InboundId: ib.Id, Enable: true, Up: 10}).Error; err != nil {
		t.Fatal(err)
	}
	first, err := nodeTrafficSnapshot(db, 1, "", "node")
	if err != nil {
		t.Fatal(err)
	}
	log := &snapshotReadLog{Interface: logger.Discard}
	measured := db.Session(&gorm.Session{Logger: log})
	if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("up", 20).Error; err != nil {
		t.Fatal(err)
	}
	second, err := nodeTrafficSnapshot(measured, 1, first.Revision, "node")
	if err != nil {
		t.Fatal(err)
	}
	if second.Structure != nil || second.Revision != first.Revision || second.InboundTraffic[0].Up != 20 {
		t.Fatalf("unchanged structure/counters: %+v", second)
	}
	for _, q := range log.statements {
		lower := strings.ToLower(q)
		if strings.Contains(lower, "select * from `inbounds`") || strings.Contains(lower, "select * from \"inbounds\"") || strings.Contains(lower, "settings") {
			t.Fatalf("unchanged poll loaded configuration: %s", q)
		}
	}
	for _, change := range []struct {
		name, sql string
		args      []any
	}{
		{"direct settings", "UPDATE inbounds SET settings = ? WHERE id = ?", []any{`{"clients":[{"id":"uuid2","email":"client","enable":true}]}`, ib.Id}},
		{"canonical identity", "UPDATE clients SET uuid = ? WHERE id = ?", []any{"uuid3", client.Id}},
		{"lifecycle", "UPDATE client_traffics SET enable = ? WHERE email = ?", []any{false, "client"}},
		{"detach", "DELETE FROM client_inbounds WHERE client_id = ?", []any{client.Id}},
	} {
		t.Run(change.name, func(t *testing.T) {
			if err := db.Exec(change.sql, change.args...).Error; err != nil {
				t.Fatal(err)
			}
			changed, err := nodeTrafficSnapshot(db, 1, second.Revision, "node")
			if err != nil {
				t.Fatal(err)
			}
			if changed.Structure == nil || changed.Revision == second.Revision {
				t.Fatalf("missed %s", change.name)
			}
			second = changed
		})
	}
}

func TestNodeSnapshotMergeSkipsUnchangedStructureAndPreservesDeltas(t *testing.T) {
	db := initNodeSnapshotDB(t, filepath.Join(t.TempDir(), "merge.db"))
	seedNodeRow(t, db, &model.Node{Id: 1, Name: "snapshot", Address: "127.0.0.1", Port: 2096, ApiToken: "token", Enable: true})
	svc := &InboundService{}
	snap := snapshotWithClients(t, "snapshot", `{"clients":[{"email":"client","enable":true}]}`, xray.ClientTraffic{Email: "client", Enable: true, Up: 100})
	snap.Inbounds[0].Id = 1
	snap.Inbounds[0].Protocol = model.VLESS
	snap.Inbounds[0].Port = 443
	snap.Inbounds[0].Enable = true
	snap.StructuralRevision = "remote-1"
	snap.ManagedAliases = []string{"second", "first"}
	for range 4 {
		if _, err := svc.setRemoteTrafficLocked(1, snap, false, false); err != nil {
			t.Fatal(err)
		}
	}
	log := &snapshotReadLog{Interface: logger.Discard}
	old := db.Logger
	db.Logger = log
	t.Cleanup(func() { db.Logger = old })
	for _, up := range []int64{120, 120, 5, 15} {
		snap.Inbounds[0].ClientStats[0].Up = up
		if up%2 == 0 {
			snap.ManagedAliases = []string{"first", "second"}
		} else {
			snap.ManagedAliases = []string{"second", "first"}
		}
		if _, err := svc.setRemoteTrafficLocked(1, snap, false, false); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range log.statements {
		lower := strings.ToLower(q)
		if strings.Contains(lower, "select * from `inbounds`") || strings.Contains(lower, "select * from \"inbounds\"") || strings.Contains(lower, "select * from `clients`") || strings.Contains(lower, "select * from \"clients\"") {
			t.Fatalf("unchanged merge rebuilt configuration: %s", q)
		}
	}
	assertUpDown(t, readTraffic(t, db, "client"), 130, 0, "duplicate and lower reset")
	// A local metadata edit must force reconciliation even when the remote revision is unchanged.
	if err := db.Exec("UPDATE clients SET comment = ? WHERE email = ?", "master edit", "client").Error; err != nil {
		t.Fatal(err)
	}
	log.statements = nil
	if _, err := svc.setRemoteTrafficLocked(1, snap, false, false); err != nil {
		t.Fatal(err)
	}
	rebuilt := false
	for _, q := range log.statements {
		if strings.Contains(q, "SELECT * FROM `inbounds`") || strings.Contains(q, "SELECT * FROM \"inbounds\"") {
			rebuilt = true
		}
	}
	if !rebuilt {
		t.Fatal("local edit did not invalidate merge cache")
	}
	var record model.ClientRecord
	if err := db.Where("email = ?", "client").First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.Comment != "master edit" {
		t.Fatalf("lost master comment: %q", record.Comment)
	}
	if err := svc.RetainNodeSnapshotCaches(nil); err != nil {
		t.Fatal(err)
	}
	log.statements = nil
	if _, err := svc.SetRemoteTraffic(1, snap, false, false); err != nil {
		t.Fatal(err)
	}
	refreshed := false
	for _, q := range log.statements {
		if strings.Contains(q, "SELECT * FROM `inbounds`") || strings.Contains(q, "SELECT * FROM \"inbounds\"") {
			refreshed = true
		}
	}
	if !refreshed {
		t.Fatal("removed/offline node retained structural cache")
	}
}

func TestNodeSnapshotReadDoesNotHoldTrafficWriter(t *testing.T) {
	initNodeSnapshotDB(t, filepath.Join(t.TempDir(), "wal.db"))
	db := database.GetDB()
	ib := model.Inbound{Tag: "wal", UserId: 1, Settings: `{"clients":[]}`}
	if err := db.Create(&ib).Error; err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- readNodeSnapshot(db, func(tx *gorm.DB) error {
			var row model.Inbound
			if err := tx.First(&row, ib.Id).Error; err != nil {
				return err
			}
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := db.WithContext(ctx).Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("up", 42).Error
	close(release)
	if err != nil {
		t.Fatalf("snapshot blocked traffic writer: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var row model.Inbound
	if err := db.First(&row, ib.Id).Error; err != nil {
		t.Fatal(err)
	}
	if row.Up != 42 {
		t.Fatalf("lost concurrent traffic: %d", row.Up)
	}
}

func TestNodeSnapshotCanceledReadReleasesConnection(t *testing.T) {
	initNodeSnapshotDB(t, filepath.Join(t.TempDir(), "cancel.db"))
	db := database.GetDB()
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	ctx, cancel := context.WithCancel(context.Background())
	err = readNodeSnapshot(db.WithContext(ctx), func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.Inbound{}).Count(&count).Error; err != nil {
			return err
		}
		cancel()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read returned %v", err)
	}
	snapshot, err := nodeTrafficSnapshot(db, 1, "", "node")
	if err != nil || snapshot.Structure == nil || len(snapshot.InboundTraffic) != 0 {
		t.Fatalf("cancellation leaked transaction: %+v %v", snapshot, err)
	}
}

func TestNodeSnapshotReopenInvalidatesCatalog(t *testing.T) {
	file := filepath.Join(t.TempDir(), "reopen.db")
	initNodeSnapshotDB(t, file)
	if err := database.GetDB().Create(&model.Inbound{Tag: "reopen", UserId: 1, Settings: `{"clients":[]}`}).Error; err != nil {
		t.Fatal(err)
	}
	before, err := nodeTrafficSnapshot(database.GetDB(), 1, "", "node")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.InitDB(file); err != nil {
		t.Fatal(err)
	}
	after, err := nodeTrafficSnapshot(database.GetDB(), 1, before.Revision, "node")
	if err != nil {
		t.Fatal(err)
	}
	if after.Structure == nil || after.Revision == before.Revision {
		t.Fatal("database reopen reused catalog history")
	}
	if len(after.Structure.Inbounds) != 1 || after.Structure.Inbounds[0].Tag != "reopen" {
		t.Fatalf("reopen lost structure: %+v", after)
	}
}

func initNodeSnapshotDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	cleanup, err := testpg.IsolatePackage("node_snapshot")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	dbtest.InitDB(t, path)
	return database.GetDB()
}
