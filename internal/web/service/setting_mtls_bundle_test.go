package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
)

func mustNodeCAPEM(t *testing.T, name string) string {
	t.Helper()
	ca, err := crypto.GenerateNodeCA(name)
	if err != nil {
		t.Fatalf("GenerateNodeCA(%q): %v", name, err)
	}
	return string(ca.CertPEM)
}

func TestParseCertificateBundlePEM(t *testing.T) {
	first := mustNodeCAPEM(t, "bundle test CA one")
	second := mustNodeCAPEM(t, "bundle test CA two")

	corrupt := strings.Replace(second, "-----BEGIN CERTIFICATE-----\n", "-----BEGIN CERTIFICATE-----\nAA", 1)

	tests := []struct {
		name      string
		bundle    string
		wantCerts int
		wantErr   bool
	}{
		{name: "single certificate", bundle: first, wantCerts: 1},
		{name: "two certificates", bundle: first + second, wantCerts: 2},
		{name: "empty", bundle: "", wantErr: true},
		{name: "whitespace only", bundle: "\n\t  \n", wantErr: true},
		{name: "second certificate corrupt", bundle: first + corrupt, wantErr: true},
		{name: "trailing non-PEM data", bundle: first + "not a certificate\n", wantErr: true},
		{name: "non-certificate block", bundle: first + "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certs, err := parseCertificateBundlePEM([]byte(tt.bundle))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseCertificateBundlePEM() = %d certs, want an error", len(certs))
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCertificateBundlePEM(): %v", err)
			}
			if len(certs) != tt.wantCerts {
				t.Fatalf("parseCertificateBundlePEM() = %d certs, want %d", len(certs), tt.wantCerts)
			}
		})
	}
}

func TestNodeMtlsClientCAPoolRejectsPartiallyValidBundle(t *testing.T) {
	s := setupSettingMtlsDB(t)

	valid := mustNodeCAPEM(t, "pool test CA")
	if err := s.setString("nodeMtlsClientCAPem", valid+"-----BEGIN CERTIFICATE-----\nnot base64\n-----END CERTIFICATE-----\n"); err != nil {
		t.Fatalf("setString: %v", err)
	}

	pool, err := s.NodeMtlsClientCAPool()
	if err == nil {
		t.Fatalf("NodeMtlsClientCAPool() = %v, want an error for a partially valid bundle", pool)
	}
}
