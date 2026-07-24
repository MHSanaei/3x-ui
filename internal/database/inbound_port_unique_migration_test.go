package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const legacyInboundsDDL = "CREATE TABLE `inbounds` (`id` integer PRIMARY KEY AUTOINCREMENT,`user_id` integer,`up` integer,`down` integer,`total` integer,`remark` text,`enable` numeric,`expiry_time` integer,`listen` text,`port` integer UNIQUE,`protocol` text,`settings` text,`stream_settings` text,`tag` text UNIQUE,`sniffing` text)"

func openLegacyPortUniqueDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	legacy, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if err := legacy.Exec(legacyInboundsDDL).Error; err != nil {
		t.Fatalf("create legacy inbounds: %v", err)
	}
	if err := legacy.Exec(
		`INSERT INTO inbounds (user_id, remark, enable, port, protocol, settings, stream_settings, tag, sniffing)
		 VALUES (1, 'preexisting', 1, 80, 'vless', '{"clients":[]}', '{}', 'in-80-tcp', '{}')`,
	).Error; err != nil {
		t.Fatalf("seed legacy inbound: %v", err)
	}
	sqlDB, err := legacy.DB()
	if err != nil {
		t.Fatalf("legacy db handle: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}
	return dbPath
}

func TestDropLegacyInboundPortUnique(t *testing.T) {
	dbPath := openLegacyPortUniqueDB(t)

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB over legacy schema: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	var preexisting model.Inbound
	if err := GetDB().Where("tag = ?", "in-80-tcp").First(&preexisting).Error; err != nil {
		t.Fatalf("preexisting inbound lost in rebuild: %v", err)
	}
	if preexisting.Remark != "preexisting" || preexisting.Port != 80 {
		t.Fatalf("preexisting inbound corrupted: remark=%q port=%d", preexisting.Remark, preexisting.Port)
	}

	nodeA, nodeB := 1, 2
	samePortA := &model.Inbound{UserId: 1, Tag: "node-a-80", Enable: true, Port: 80, Protocol: model.VLESS, Remark: "a", Settings: `{"clients":[]}`, NodeID: &nodeA}
	samePortB := &model.Inbound{UserId: 1, Tag: "node-b-80", Enable: true, Port: 80, Protocol: model.VLESS, Remark: "b", Settings: `{"clients":[]}`, NodeID: &nodeB}
	if err := GetDB().Create(samePortA).Error; err != nil {
		t.Fatalf("create node-a inbound on port 80: %v", err)
	}
	if err := GetDB().Create(samePortB).Error; err != nil {
		t.Fatalf("create node-b inbound on port 80: %v", err)
	}

	var dupTag model.Inbound
	dupErr := GetDB().Create(&model.Inbound{UserId: 1, Tag: "in-80-tcp", Enable: true, Port: 90, Protocol: model.VLESS, Settings: `{"clients":[]}`}).Error
	if dupErr == nil {
		t.Fatalf("duplicate tag insert succeeded, want unique violation; row=%v", dupTag)
	}

	if err := dropLegacyInboundPortUnique(); err != nil {
		t.Fatalf("second dropLegacyInboundPortUnique run: %v", err)
	}
}
