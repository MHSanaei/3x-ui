package service

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/crypto/nodetoken"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// enableNodeTokenEncryption installs a test keyring and restores off mode so
// the package-global codec cannot leak between tests.
func enableNodeTokenEncryption(t *testing.T) {
	t.Helper()
	var k [32]byte
	for i := range k {
		k[i] = byte(i + 1)
	}
	ring := &nodetoken.Keyring{ActiveID: "t1", Keys: map[string][32]byte{"t1": k}}
	codec, err := nodetoken.NewCodec(nodetoken.ModeRequired, ring)
	if err != nil {
		t.Fatalf("new codec: %v", err)
	}
	nodetoken.Init(codec)
	t.Cleanup(func() {
		off, _ := nodetoken.NewCodec(nodetoken.ModeOff, nil)
		nodetoken.Init(off)
	})
}

func TestNodeToken_CreateNeverInsertsPlaintextTuple(t *testing.T) {
	setupConflictDB(t)
	enableNodeTokenEncryption(t)
	db := database.GetDB()
	const callback = "test:no-plaintext-node-insert"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if node, ok := tx.Statement.Dest.(*model.Node); ok && node.ApiToken != "" {
			tx.AddError(errors.New("plaintext token reached node INSERT"))
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })

	n := &model.Node{Name: "no-plain", Address: "127.0.0.1", Port: 2096, ApiToken: "secret", Enable: true}
	if err := (&NodeService{}).Create(n); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if n.ApiToken != "secret" {
		t.Fatalf("in-memory token = %q, want plaintext response value", n.ApiToken)
	}
}

func rawStoredToken(t *testing.T, id int) string {
	t.Helper()
	var n model.Node
	if err := database.GetDB().Model(model.Node{}).Where("id = ?", id).First(&n).Error; err != nil {
		t.Fatalf("raw load: %v", err)
	}
	return n.ApiToken
}

// Create stores the token encrypted at rest; GetById returns it decrypted.
func TestNodeToken_EncryptedAtRest_PlaintextInMemory(t *testing.T) {
	setupConflictDB(t)
	enableNodeTokenEncryption(t)
	svc := &NodeService{}

	n := &model.Node{Name: "enc1", Address: "127.0.0.1", Port: 2096, ApiToken: "super-secret", Enable: true}
	if err := svc.Create(n); err != nil {
		t.Fatalf("create: %v", err)
	}

	stored := rawStoredToken(t, n.Id)
	if !nodetoken.IsEncrypted(stored) {
		t.Fatalf("token at rest is not encrypted: %q", stored)
	}
	if strings.Contains(stored, "super-secret") {
		t.Fatalf("plaintext leaked into stored column: %q", stored)
	}

	got, err := svc.GetById(n.Id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ApiToken != "super-secret" {
		t.Fatalf("GetById should return plaintext, got %q", got.ApiToken)
	}
}

// A blank token on Update keeps the stored one (the UI doesn't echo secrets).
func TestNodeToken_UpdateBlankKeepsExisting(t *testing.T) {
	setupConflictDB(t)
	enableNodeTokenEncryption(t)
	svc := &NodeService{}

	n := &model.Node{Name: "enc2", Address: "127.0.0.1", Port: 2096, ApiToken: "keep-me", Enable: true}
	if err := svc.Create(n); err != nil {
		t.Fatalf("create: %v", err)
	}
	before := rawStoredToken(t, n.Id)

	// Update with empty token must not wipe or change the stored ciphertext.
	upd := &model.Node{Name: "enc2-renamed", Address: "127.0.0.1", Port: 2096, ApiToken: "", Enable: true}
	if err := svc.Update(n.Id, upd); err != nil {
		t.Fatalf("update: %v", err)
	}
	if after := rawStoredToken(t, n.Id); after != before {
		t.Fatalf("blank-token update changed stored token: %q -> %q", before, after)
	}
	got, _ := svc.GetById(n.Id)
	if got.ApiToken != "keep-me" {
		t.Fatalf("token lost after blank update, got %q", got.ApiToken)
	}
	if got.Name != "enc2-renamed" {
		t.Fatalf("other fields should still update, got name %q", got.Name)
	}
}

// The migration re-encrypts a legacy plaintext row under the active key (CAS).
func TestNodeToken_MigratePlaintextRows(t *testing.T) {
	setupConflictDB(t)
	// Insert a legacy plaintext row directly (encryption off at insert time).
	db := database.GetDB()
	legacy := &model.Node{Name: "legacy", Address: "127.0.0.1", Port: 2096, ApiToken: "legacy-plain", Enable: true}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("create legacy: %v", err)
	}
	if rawStoredToken(t, legacy.Id) != "legacy-plain" {
		t.Fatal("precondition: legacy row should be plaintext")
	}

	enableNodeTokenEncryption(t)
	changed, _, err := (&NodeService{}).MigrateNodeTokensToActiveKey()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if changed != 1 {
		t.Fatalf("expected 1 row re-encrypted, got %d", changed)
	}
	if stored := rawStoredToken(t, legacy.Id); !nodetoken.IsEncrypted(stored) {
		t.Fatalf("legacy row not encrypted after migration: %q", stored)
	}
	got, _ := (&NodeService{}).GetById(legacy.Id)
	if got.ApiToken != "legacy-plain" {
		t.Fatalf("migrated token no longer decrypts to original: %q", got.ApiToken)
	}

	// Idempotent: a second run changes nothing.
	changed2, _, _ := (&NodeService{}).MigrateNodeTokensToActiveKey()
	if changed2 != 0 {
		t.Fatalf("second migration should be a no-op, changed %d", changed2)
	}
}

func TestNodeToken_MigrationRejectsCorruptActiveCiphertext(t *testing.T) {
	setupConflictDB(t)
	enableNodeTokenEncryption(t)
	n := &model.Node{Name: "corrupt", Address: "127.0.0.1", Port: 2096, ApiToken: "secret", Enable: true}
	if err := (&NodeService{}).Create(n); err != nil {
		t.Fatalf("Create: %v", err)
	}
	stored := rawStoredToken(t, n.Id)
	body := strings.LastIndexByte(stored, ':') + 1
	replacement := byte('A')
	if stored[body] == replacement {
		replacement = 'B'
	}
	corrupt := stored[:body] + string(replacement) + stored[body+1:]
	if err := database.GetDB().Model(&model.Node{}).Where("id = ?", n.Id).Update("api_token", corrupt).Error; err != nil {
		t.Fatalf("corrupt row: %v", err)
	}
	if _, _, err := (&NodeService{}).MigrateNodeTokensToActiveKey(); err == nil {
		t.Fatal("migration trusted a corrupt active-key ciphertext")
	}
}
