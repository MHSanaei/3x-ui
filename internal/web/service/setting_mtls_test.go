package service

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

func setupSettingMtlsDB(t *testing.T) *SettingService {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	return &SettingService{}
}

func TestEnsureNodeMtlsCA_Idempotent(t *testing.T) {
	s := setupSettingMtlsDB(t)

	first, err := s.EnsureNodeMtlsCA()
	if err != nil {
		t.Fatalf("EnsureNodeMtlsCA (first): %v", err)
	}
	block, _ := pem.Decode(first.CertPEM)
	if block == nil {
		t.Fatal("CA cert is not valid PEM")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}
	if !caCert.IsCA {
		t.Fatal("stored CA must have IsCA=true")
	}

	second, err := s.EnsureNodeMtlsCA()
	if err != nil {
		t.Fatalf("EnsureNodeMtlsCA (second): %v", err)
	}
	if !bytes.Equal(first.CertPEM, second.CertPEM) || !bytes.Equal(first.KeyPEM, second.KeyPEM) {
		t.Fatal("EnsureNodeMtlsCA must be idempotent: second call returned different PEMs")
	}
}

func TestEnsureMasterClientCert_VerifiesAndIdempotent(t *testing.T) {
	s := setupSettingMtlsDB(t)

	ca, err := s.EnsureNodeMtlsCA()
	if err != nil {
		t.Fatalf("EnsureNodeMtlsCA: %v", err)
	}
	client, err := s.EnsureMasterClientCert()
	if err != nil {
		t.Fatalf("EnsureMasterClientCert: %v", err)
	}

	cblock, _ := pem.Decode(client.CertPEM)
	if cblock == nil {
		t.Fatal("client cert is not valid PEM")
	}
	leaf, err := x509.ParseCertificate(cblock.Bytes)
	if err != nil {
		t.Fatalf("parse client cert: %v", err)
	}
	caBlock, _ := pem.Decode(ca.CertPEM)
	roots := x509.NewCertPool()
	roots.AddCert(mustParse(t, caBlock.Bytes))
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Fatalf("master client cert must verify against the node CA for client auth: %v", err)
	}

	again, err := s.EnsureMasterClientCert()
	if err != nil {
		t.Fatalf("EnsureMasterClientCert (second): %v", err)
	}
	if !bytes.Equal(client.CertPEM, again.CertPEM) || !bytes.Equal(client.KeyPEM, again.KeyPEM) {
		t.Fatal("EnsureMasterClientCert must be idempotent")
	}
}

func TestEnsureMasterClientCert_RefusesSilentRotationWhenCredentialIsLost(t *testing.T) {
	s := setupSettingMtlsDB(t)

	client, err := s.EnsureMasterClientCert()
	if err != nil {
		t.Fatalf("EnsureMasterClientCert: %v", err)
	}
	pin, err := clientCertSHA256FromPEM(client.CertPEM)
	if err != nil {
		t.Fatalf("clientCertSHA256FromPEM: %v", err)
	}
	if err := s.setString(settingNodeMtlsClientPin, pin); err != nil {
		t.Fatalf("persist client pin: %v", err)
	}
	if err := s.setString(settingNodeMtlsClientCert, ""); err != nil {
		t.Fatalf("clear client cert: %v", err)
	}
	if err := s.setString(settingNodeMtlsClientKey, ""); err != nil {
		t.Fatalf("clear client key: %v", err)
	}

	_, err = s.EnsureMasterClientCert()
	if err == nil {
		t.Fatal("lost master credential with a persisted pin must not be silently reissued")
	}
	if !strings.Contains(err.Error(), settingNodeMtlsClientPin) {
		t.Fatalf("error must explain the pin/credential conflict, got: %v", err)
	}
}

func TestEnsureMasterClientCert_PersistsCredentialAtomically(t *testing.T) {
	s := setupSettingMtlsDB(t)
	db := database.GetDB()
	trigger := `CREATE TRIGGER fail_master_pin_insert
		BEFORE INSERT ON settings
		WHEN NEW.key = 'nodeMtlsClientCertSha256'
		BEGIN SELECT RAISE(ABORT, 'injected pin failure'); END`
	if err := db.Exec(trigger).Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	if _, err := s.EnsureMasterClientCert(); err == nil {
		t.Fatal("injected persistence failure unexpectedly succeeded")
	}
	for _, key := range []string{settingNodeMtlsClientCert, settingNodeMtlsClientKey, settingNodeMtlsClientPin} {
		got, err := s.getString(key)
		if err != nil {
			t.Fatalf("get %s after rollback: %v", key, err)
		}
		if got != "" {
			t.Fatalf("%s persisted despite transaction rollback", key)
		}
	}

	if err := db.Exec("DROP TRIGGER fail_master_pin_insert").Error; err != nil {
		t.Fatalf("drop failure trigger: %v", err)
	}
	if _, err := s.EnsureMasterClientCert(); err != nil {
		t.Fatalf("retry after rollback: %v", err)
	}
}

func TestNodeMtlsClientCAPool(t *testing.T) {
	s := setupSettingMtlsDB(t)

	pool, err := s.NodeMtlsClientCAPool()
	if err != nil {
		t.Fatalf("NodeMtlsClientCAPool (unset): %v", err)
	}
	if pool != nil {
		t.Fatal("with no trust CA configured, the pool must be nil (mTLS off; listener unchanged)")
	}

	ca, err := s.EnsureNodeMtlsCA()
	if err != nil {
		t.Fatalf("EnsureNodeMtlsCA: %v", err)
	}
	if err := s.setString("nodeMtlsClientCAPem", string(ca.CertPEM)); err != nil {
		t.Fatalf("set trust CA: %v", err)
	}
	pool, err = s.NodeMtlsClientCAPool()
	if err != nil {
		t.Fatalf("NodeMtlsClientCAPool (set): %v", err)
	}
	if pool == nil {
		t.Fatal("with a trust CA configured, the pool must be non-nil")
	}
}

func mustParse(t *testing.T, der []byte) *x509.Certificate {
	t.Helper()
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	return c
}
