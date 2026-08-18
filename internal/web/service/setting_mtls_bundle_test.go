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
		wantErr   string
	}{
		{name: "single certificate", bundle: first, wantCerts: 1},
		{name: "two certificates", bundle: first + second, wantCerts: 2},
		{name: "empty", bundle: "", wantErr: "certificate bundle is empty"},
		{name: "whitespace only", bundle: "\n\t  \n", wantErr: "certificate bundle is empty"},
		{name: "leading non-PEM data", bundle: "junk\n" + first, wantErr: "certificate bundle contains malformed or non-PEM data"},
		{name: "interstitial non-PEM data", bundle: first + "junk\n" + second, wantErr: "certificate bundle contains malformed or non-PEM data"},
		{name: "second certificate corrupt", bundle: first + corrupt, wantErr: "certificate bundle contains malformed or non-PEM data"},
		{name: "trailing non-PEM data", bundle: first + "not a certificate\n", wantErr: "certificate bundle contains malformed or non-PEM data"},
		{name: "non-certificate block", bundle: first + "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n", wantErr: "certificate bundle contains malformed or non-PEM data"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certs, err := parseCertificateBundlePEM([]byte(tt.bundle))
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("parseCertificateBundlePEM() error = %v, want %q", err, tt.wantErr)
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
	want := "nodeMtlsClientCAPem is not a valid certificate bundle: certificate bundle contains malformed or non-PEM data"
	if err == nil || err.Error() != want {
		t.Fatalf("NodeMtlsClientCAPool() = %v, error = %v, want %q", pool, err, want)
	}
}
