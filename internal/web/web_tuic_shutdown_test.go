package web

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/web/job"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestGracefulShutdownPersistsFinalTuicTraffic(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate certificate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal certificate key: %v", err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	const (
		inboundID = 18002
		email     = "shutdown-tuic@x"
	)
	inbound := &model.Inbound{
		Id: inboundID, Tag: "tuic-shutdown-traffic", Protocol: model.TUIC,
		Enable: true, Listen: "127.0.0.1", Port: 0,
		Settings: fmt.Sprintf(`{"certificate":%q,"private_key":%q,"clients":[{"uuid":"a0000000-0000-0000-0000-000000000022","password":"p","email":%q,"enable":true}]}`, certificate, privatePEM, email),
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("create TUIC inbound: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{InboundId: inboundID, Email: email, Enable: true}).Error; err != nil {
		t.Fatalf("create client traffic: %v", err)
	}

	manager := tuic.GetManager()
	manager.StopAll()
	manager.CollectAllTraffic()
	t.Cleanup(manager.StopAll)
	job.NewTuicJob().Run()
	if !manager.AddTestTraffic(inboundID, email, 100, 200) {
		t.Fatal("TUIC listener did not start")
	}

	if err := NewServer().Stop(); err != nil && !strings.Contains(err.Error(), "xray is not running") {
		t.Fatalf("graceful server shutdown: %v", err)
	}

	var got xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&got).Error; err != nil {
		t.Fatalf("load persisted client traffic: %v", err)
	}
	if got.Up != 100 || got.Down != 200 {
		t.Fatalf("shutdown persisted traffic (%d,%d), want (100,200)", got.Up, got.Down)
	}
	_, pending := manager.CollectAllTraffic()
	if len(pending) != 0 {
		t.Fatalf("final TUIC traffic remains only in manager memory: %+v", pending)
	}
}
