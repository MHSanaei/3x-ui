package service

import (
	"os"
	"strings"
	"testing"

	"github.com/op/go-logging"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func durablePostgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	if os.Getenv("XUI_DB_TYPE") != "postgres" || strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run commit-failure injection")
	}
	portConflictLoggerOnce.Do(func() { xuilogger.InitLogger(logging.ERROR) })
	if err := database.InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	return database.GetDB()
}

func durableTestInbound(nodeID *int, tag string, port int) *model.Inbound {
	return &model.Inbound{
		UserId:         1,
		NodeID:         nodeID,
		Tag:            tag,
		Remark:         tag,
		Enable:         true,
		Port:           port,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp","security":"tls"}`,
		Settings:       `{"clients":[],"decryption":"none"}`,
	}
}

func installDeferredCommitFailure(t *testing.T, db *gorm.DB, callbackKind, callbackName, triggerTable, parentTable, childTable string) {
	t.Helper()
	_ = db.Exec("DROP TABLE IF EXISTS " + childTable).Error
	_ = db.Exec("DROP TABLE IF EXISTS " + parentTable).Error
	if err := db.Exec("CREATE TABLE " + parentTable + " (id bigint PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE " + childTable + " (id bigint PRIMARY KEY, parent_id bigint REFERENCES " + parentTable + "(id) DEFERRABLE INITIALLY DEFERRED)").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE IF EXISTS " + childTable).Error
		_ = db.Exec("DROP TABLE IF EXISTS " + parentTable).Error
	})

	cb := func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != triggerTable {
			return
		}
		res := tx.Session(&gorm.Session{NewDB: true}).Exec("INSERT INTO " + childTable + " (id, parent_id) VALUES (1, 999999)")
		if res.Error != nil {
			tx.AddError(res.Error)
		}
	}
	switch callbackKind {
	case "create":
		if err := db.Callback().Create().After("gorm:create").Register(callbackName, cb); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Callback().Create().Remove(callbackName) })
	case "update":
		if err := db.Callback().Update().After("gorm:update").Register(callbackName, cb); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Callback().Update().Remove(callbackName) })
	default:
		t.Fatalf("unknown callback kind %q", callbackKind)
	}
}

func cleanupDurableInboundFixtures(t *testing.T, db *gorm.DB, tags ...string) {
	t.Helper()
	if len(tags) == 0 {
		return
	}
	cleanup := func() {
		_ = db.Exec("DELETE FROM hosts WHERE inbound_id IN (SELECT id FROM inbounds WHERE tag IN ?)", tags).Error
		_ = db.Exec("DELETE FROM client_inbounds WHERE inbound_id IN (SELECT id FROM inbounds WHERE tag IN ?)", tags).Error
		_ = db.Where("tag IN ?", tags).Delete(&model.Inbound{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)
}

func TestAddInbound_PostgresCommitFailureMakesNoRuntimeCall(t *testing.T) {
	db := durablePostgresDB(t)
	nodeID, fake := setupNodeRuntime(t)
	tag := "durable-add-" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	cleanupDurableInboundFixtures(t, db, tag)
	installDeferredCommitFailure(t, db, "create", "durable:add_inbound_commit_failure", "inbounds", "durable_add_parent", "durable_add_child")

	inbound := durableTestInbound(&nodeID, tag, 25443)
	_, _, err := (&InboundService{}).AddInbound(inbound)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Fatalf("AddInbound error = %v, want deferred foreign-key commit failure", err)
	}
	if fake.addInbound.Load() != 0 || fake.updateInbound.Load() != 0 || fake.delInbound.Load() != 0 {
		t.Fatalf("runtime calls before failed commit: add=%d update=%d del=%d, want zero", fake.addInbound.Load(), fake.updateInbound.Load(), fake.delInbound.Load())
	}
	var rows int64
	if err := db.Model(&model.Inbound{}).Where("tag = ?", inbound.Tag).Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("rolled-back inbound rows = %d, want 0", rows)
	}
	var persisted model.Node
	if err := db.First(&persisted, nodeID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.ConfigDirty {
		t.Fatal("node dirty flag changed across failed commit")
	}
}

func TestUpdateInbound_PostgresCommitFailureMakesNoRuntimeCall(t *testing.T) {
	db := durablePostgresDB(t)
	nodeID, fake := setupNodeRuntime(t)
	tag := "durable-update-" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	cleanupDurableInboundFixtures(t, db, tag)
	original := durableTestInbound(&nodeID, tag, 25444)
	original.Remark = "before"
	if err := db.Create(original).Error; err != nil {
		t.Fatal(err)
	}
	installDeferredCommitFailure(t, db, "update", "durable:update_inbound_commit_failure", "inbounds", "durable_update_parent", "durable_update_child")

	update := durableTestInbound(&nodeID, original.Tag, original.Port)
	update.Id = original.Id
	update.Remark = "after"
	_, _, err := (&InboundService{}).UpdateInbound(update)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Fatalf("UpdateInbound error = %v, want deferred foreign-key commit failure", err)
	}
	if fake.addInbound.Load() != 0 || fake.updateInbound.Load() != 0 || fake.delInbound.Load() != 0 {
		t.Fatalf("runtime calls before failed commit: add=%d update=%d del=%d, want zero", fake.addInbound.Load(), fake.updateInbound.Load(), fake.delInbound.Load())
	}
	var persisted model.Inbound
	if err := db.First(&persisted, original.Id).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Remark != "before" {
		t.Fatalf("remark after failed commit = %q, want before", persisted.Remark)
	}
	var persistedNode model.Node
	if err := db.First(&persistedNode, nodeID).Error; err != nil {
		t.Fatal(err)
	}
	if persistedNode.ConfigDirty {
		t.Fatal("node dirty flag changed across failed commit")
	}
}
