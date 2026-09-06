package tgbot

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeCert(t *testing.T, commonName string, dnsNames []string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	path := filepath.Join(t.TempDir(), "cert.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	return path
}

// The OS hostname is never reachable from a customer's device, so the domain
// the certificate is valid for is what the subscription URL has to carry.
func TestDomainFromCertificate(t *testing.T) {
	tests := []struct {
		name       string
		commonName string
		dnsNames   []string
		want       string
	}{
		{"concrete name wins", "", []string{"subs.example.com"}, "subs.example.com"},
		{"concrete name beats a wildcard", "", []string{"*.example.com", "panel.example.com"}, "panel.example.com"},
		{"wildcard falls back to its zone", "", []string{"*.playvalorant.cfd"}, "playvalorant.cfd"},
		{"common name when there are no SANs", "legacy.example.com", nil, "legacy.example.com"},
		{"wildcard common name is not a host", "*.example.com", nil, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domainFromCertificate(writeCert(t, tc.commonName, tc.dnsNames))
			if got != tc.want {
				t.Fatalf("domainFromCertificate = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDomainFromCertificateMissingOrJunk(t *testing.T) {
	if got := domainFromCertificate(""); got != "" {
		t.Fatalf("no cert configured should yield %q, got %q", "", got)
	}
	if got := domainFromCertificate(filepath.Join(t.TempDir(), "absent.pem")); got != "" {
		t.Fatalf("an unreadable cert should yield %q, got %q", "", got)
	}
	junk := filepath.Join(t.TempDir(), "junk.pem")
	if err := os.WriteFile(junk, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write junk: %v", err)
	}
	if got := domainFromCertificate(junk); got != "" {
		t.Fatalf("a malformed cert should yield %q, got %q", "", got)
	}
}
