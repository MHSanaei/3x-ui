package crypto

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
)

// parseOneCert decodes a single CERTIFICATE PEM block.
func parseOneCert(t *testing.T, pemBytes []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("expected a CERTIFICATE PEM block, got %+v", block)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	return cert
}

func TestGenerateNodeCA(t *testing.T) {
	ca, err := GenerateNodeCA("3x-ui node CA")
	if err != nil {
		t.Fatalf("GenerateNodeCA: %v", err)
	}
	cert := parseOneCert(t, ca.CertPEM)
	if !cert.IsCA {
		t.Fatal("CA certificate must have IsCA=true")
	}
	if cert.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Fatal("CA certificate must allow KeyUsageCertSign")
	}
	if _, _, err := LoadCAFromPEM(ca); err != nil {
		t.Fatalf("LoadCAFromPEM on a freshly generated CA: %v", err)
	}
}

func TestIssueClientCert_VerifiesAgainstCA(t *testing.T) {
	ca, err := GenerateNodeCA("3x-ui node CA")
	if err != nil {
		t.Fatalf("GenerateNodeCA: %v", err)
	}
	leaf, err := IssueClientCert(ca, "central-panel")
	if err != nil {
		t.Fatalf("IssueClientCert: %v", err)
	}
	cert := parseOneCert(t, leaf.CertPEM)

	hasClientAuth := false
	for _, u := range cert.ExtKeyUsage {
		if u == x509.ExtKeyUsageClientAuth {
			hasClientAuth = true
		}
	}
	if !hasClientAuth {
		t.Fatal("client leaf must carry ExtKeyUsageClientAuth")
	}

	roots := x509.NewCertPool()
	roots.AddCert(parseOneCert(t, ca.CertPEM))
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Fatalf("client leaf must verify against the issuing CA: %v", err)
	}
}
