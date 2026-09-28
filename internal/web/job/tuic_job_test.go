package job

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func generateTestCertForJob(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return
}

func TestTuicJob_TrafficAccounting(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "tuic_job.db")); err != nil {
		t.Fatalf("database.InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	certPEM, keyPEM := generateTestCertForJob(t)

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := pc.LocalAddr().(*net.UDPAddr).Port
	_ = pc.Close()

	email := "tuic-user@example.com"
	settings := fmt.Sprintf(`{
		"certificate": %q,
		"private_key": %q,
		"congestion_control": "bbr",
		"alpn": ["h3"],
		"udp_relay_mode": "native",
		"zero_rtt_handshake": false,
		"clients": [
			{
				"uuid": "a0000000-0000-0000-0000-000000000001",
				"password": "password123",
				"email": %q,
				"enable": true
			}
		]
	}`, string(certPEM), string(keyPEM), email)

	inbound := &model.Inbound{
		Id:       42,
		Tag:      "tuic-in-42",
		Protocol: model.TUIC,
		Listen:   "127.0.0.1",
		Port:     port,
		Enable:   true,
		Settings: settings,
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("create inbound failed: %v", err)
	}

	clientTraffic := &xray.ClientTraffic{
		InboundId: inbound.Id,
		Email:     email,
		Up:        0,
		Down:      0,
		Enable:    true,
	}
	if err := database.GetDB().Create(clientTraffic).Error; err != nil {
		t.Fatalf("create clientTraffic failed: %v", err)
	}

	mgr := tuic.GetManager()
	t.Cleanup(mgr.StopAll)

	job := NewTuicJob()

	// Initial run reconciles desired instances and starts the server
	job.Run()

	// Add test traffic to the running client
	const wantUp = int64(1024)
	const wantDown = int64(2048)
	if !mgr.AddTestTraffic(inbound.Id, email, wantUp, wantDown) {
		t.Fatalf("failed to add test traffic for %s on inbound %d", email, inbound.Id)
	}

	// Second run collects and writes traffic to database
	job.Run()

	// Verify client traffic in database
	var dbClient xray.ClientTraffic
	if err := database.GetDB().Where("inbound_id = ? AND email = ?", inbound.Id, email).First(&dbClient).Error; err != nil {
		t.Fatalf("find client traffic in DB failed: %v", err)
	}
	if dbClient.Up != wantUp || dbClient.Down != wantDown {
		t.Fatalf("client traffic mismatch: got up=%d down=%d, want up=%d down=%d", dbClient.Up, dbClient.Down, wantUp, wantDown)
	}

	// Verify inbound total traffic in database: TuicJob leaves inbound total
	// accounting to xray_traffic_job (metered on the SOCKS relay tag, matching mtproto),
	// preventing double-counting.
	var dbInbound model.Inbound
	if err := database.GetDB().First(&dbInbound, inbound.Id).Error; err != nil {
		t.Fatalf("find inbound in DB failed: %v", err)
	}
	if dbInbound.Up != 0 || dbInbound.Down != 0 {
		t.Fatalf("expected inbound traffic to remain 0 in TuicJob (metered by Xray bridge), got up=%d down=%d", dbInbound.Up, dbInbound.Down)
	}
}
