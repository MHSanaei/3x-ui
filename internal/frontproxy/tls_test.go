package frontproxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caddyserver/certmagic"
)

// writeTestCert generates a self-signed cert/key pair and writes both as PEM
// files in a fresh temp dir, returning their paths and the cert's NotAfter so
// a test can assert the status recorded from it matches.
func writeTestCert(t *testing.T) (certFile, keyFile string, notAfter time.Time) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	notAfter = time.Now().Add(90 * 24 * time.Hour).Truncate(time.Second)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "frontproxy-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile, notAfter
}

func TestManualTLSConfigRecordsObtainedStatus(t *testing.T) {
	certFile, keyFile, notAfter := writeTestCert(t)
	if _, err := manualTLSConfig(TLSSettings{CertFile: certFile, KeyFile: keyFile}); err != nil {
		t.Fatalf("manualTLSConfig: %v", err)
	}
	status := CurrentCertStatus()
	if status.State != CertStateObtained {
		t.Fatalf("state = %q, want %q", status.State, CertStateObtained)
	}
	if status.NotAfter == nil || !status.NotAfter.Equal(notAfter) {
		t.Errorf("NotAfter = %v, want %v", status.NotAfter, notAfter)
	}
}

func TestManualTLSConfigRecordsFailedStatus(t *testing.T) {
	if _, err := manualTLSConfig(TLSSettings{}); err == nil {
		t.Fatal("expected an error with no cert/key configured")
	}
	status := CurrentCertStatus()
	if status.State != CertStateFailed {
		t.Fatalf("state = %q, want %q", status.State, CertStateFailed)
	}
	if status.Error == "" {
		t.Error("expected a non-empty error message")
	}
}

func TestCertOnEventTracksObtaining(t *testing.T) {
	resetCertStatus()
	hook := certOnEvent(certmagic.NewDefault(), "example.test")
	if err := hook(t.Context(), "cert_obtaining", map[string]any{"identifier": "example.test"}); err != nil {
		t.Fatal(err)
	}
	status := CurrentCertStatus()
	if status.State != CertStateObtaining || status.Domain != "example.test" {
		t.Errorf("status = %+v, want obtaining/example.test", status)
	}
}

func TestCertOnEventTracksFailed(t *testing.T) {
	resetCertStatus()
	hook := certOnEvent(certmagic.NewDefault(), "example.test")
	failure := errors.New("no HTTP-01 response from example.test: connection refused")
	if err := hook(t.Context(), "cert_failed", map[string]any{"identifier": "example.test", "error": failure}); err != nil {
		t.Fatal(err)
	}
	status := CurrentCertStatus()
	if status.State != CertStateFailed || status.Error != failure.Error() {
		t.Errorf("status = %+v, want failed with %q", status, failure.Error())
	}
}

// cert_obtained must not panic or leave a wrong state when nothing was ever
// saved to storage for this domain -- currentCertNotAfter's lookup fails, and
// the handler must still land on Obtained (a certificate really was just
// obtained per the event) just with an unknown expiry rather than crashing.
func TestCertOnEventObtainedWithoutStorageStillSetsState(t *testing.T) {
	resetCertStatus()
	cfg := certmagic.NewDefault()
	cfg.Storage = &certmagic.FileStorage{Path: t.TempDir()}
	hook := certOnEvent(cfg, "example.test")
	if err := hook(t.Context(), "cert_obtained", map[string]any{"identifier": "example.test"}); err != nil {
		t.Fatal(err)
	}
	status := CurrentCertStatus()
	if status.State != CertStateObtained {
		t.Errorf("state = %q, want %q", status.State, CertStateObtained)
	}
	if status.NotAfter != nil {
		t.Errorf("NotAfter = %v, want nil (nothing in storage)", status.NotAfter)
	}
}

func TestResetCertStatusClearsState(t *testing.T) {
	setCertStatus(CertStatus{State: CertStateFailed, Domain: "example.test", Error: "boom"})
	resetCertStatus()
	if got := CurrentCertStatus(); got != (CertStatus{}) {
		t.Errorf("status after reset = %+v, want zero value", got)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// Stopping the reverse proxy must clear the certificate status along with
// everything else -- otherwise a manual-mode "obtained" reading (or a stuck
// "obtaining" one) would linger in the UI for a door that is no longer up.
func TestStopClearsCertStatus(t *testing.T) {
	certFile, keyFile, _ := writeTestCert(t)
	m := &Manager{}
	opts := Options{
		Listen:  "127.0.0.1",
		Port:    freePort(t),
		Routing: Config{PanelBasePath: "/p/", PanelPort: 2053},
		Decoy:   DecoyConfig{Mode: DecoyTemplate, Template: "maintenance"},
		TLS:     TLSSettings{Mode: CertManual, CertFile: certFile, KeyFile: keyFile},
	}
	if err := m.Start(opts); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if CurrentCertStatus().State != CertStateObtained {
		t.Fatalf("status after Start = %+v, want obtained", CurrentCertStatus())
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := CurrentCertStatus(); got != (CertStatus{}) {
		t.Errorf("status after Stop = %+v, want zero value", got)
	}
}
