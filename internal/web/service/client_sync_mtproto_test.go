package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestSyncInbound_UpdatesMtprotoSecretAndAdTag(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	mtproto := &model.Inbound{Tag: "mtproto-in", Enable: true, Port: 10004, Protocol: model.MTProto}
	if err := db.Create(mtproto).Error; err != nil {
		t.Fatalf("create mtproto inbound: %v", err)
	}

	svc := ClientService{}
	const email = "tg@example.com"
	const firstSecret = "ee0123456789abcdef0123456789abcdef6578616d706c652e636f6d"
	const rekeyedSecret = "eefedcba9876543210fedcba98765432106578616d706c652e636f6d"
	const firstTag = "0123456789abcdef0123456789abcdef"
	const retaggedTag = "fedcba9876543210fedcba9876543210"

	first := model.Client{Email: email, Secret: firstSecret, AdTag: firstTag, Enable: true}
	if err := svc.SyncInbound(nil, mtproto.Id, []model.Client{first}); err != nil {
		t.Fatalf("SyncInbound (create): %v", err)
	}

	var row model.ClientRecord
	if err := db.Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("lookup client row: %v", err)
	}
	if row.Secret != firstSecret || row.AdTag != firstTag {
		t.Fatalf("create must store secret and ad tag: got secret=%q adTag=%q", row.Secret, row.AdTag)
	}

	rekeyed := model.Client{Email: email, Secret: rekeyedSecret, AdTag: retaggedTag, Enable: true}
	if err := svc.SyncInbound(nil, mtproto.Id, []model.Client{rekeyed}); err != nil {
		t.Fatalf("SyncInbound (rekey): %v", err)
	}
	if err := db.Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("lookup client row after rekey: %v", err)
	}
	if row.Secret != rekeyedSecret {
		t.Errorf("a re-keyed secret must reach the client record (sub links and the clients page read it), got %q", row.Secret)
	}
	if row.AdTag != retaggedTag {
		t.Errorf("a changed ad tag must reach the client record, got %q", row.AdTag)
	}

	secretless := model.Client{Email: email, Enable: true}
	if err := svc.SyncInbound(nil, mtproto.Id, []model.Client{secretless}); err != nil {
		t.Fatalf("SyncInbound (secretless): %v", err)
	}
	if err := db.Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("lookup client row after secretless sync: %v", err)
	}
	if row.Secret != rekeyedSecret || row.AdTag != retaggedTag {
		t.Errorf("a payload without mtproto fields must not wipe them: got secret=%q adTag=%q", row.Secret, row.AdTag)
	}
}
