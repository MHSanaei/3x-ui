package service

import (
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestNormalizeKeepsMtls(t *testing.T) {
	s := &NodeService{}
	cases := []struct {
		name     string
		in       model.Node
		wantMode string
		wantErr  bool
	}{
		{"mtls over https preserved", model.Node{Name: "n", Address: "node.example.com", Port: 2053, Scheme: "https", TlsVerifyMode: "mtls"}, "mtls", false},
		{"mtls over http rejected", model.Node{Name: "n", Address: "node.example.com", Port: 2053, Scheme: "http", TlsVerifyMode: "mtls"}, "", true},
		{"unknown mode clamped to verify", model.Node{Name: "n", Address: "node.example.com", Port: 2053, Scheme: "https", TlsVerifyMode: "bogus"}, "verify", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := c.in
			err := s.normalize(&n)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalize: %v", err)
			}
			if n.TlsVerifyMode != c.wantMode {
				t.Fatalf("TlsVerifyMode = %q, want %q", n.TlsVerifyMode, c.wantMode)
			}
		})
	}
}

func TestNodeTlsVerifyModeValidatorAcceptsMtls(t *testing.T) {
	v := validator.New(validator.WithRequiredStructEnabled())
	base := model.Node{Name: "n", Address: "node.example.com", Port: 2053, Scheme: "https", ApiToken: "t"}

	for _, m := range []string{"verify", "skip", "pin", "mtls"} {
		n := base
		n.TlsVerifyMode = m
		if err := v.Struct(n); err != nil {
			t.Fatalf("validator rejected valid TlsVerifyMode %q: %v", m, err)
		}
	}
	bad := base
	bad.TlsVerifyMode = "bogus"
	if err := v.Struct(bad); err == nil {
		t.Fatal("validator must reject an unknown TlsVerifyMode")
	}
}

func TestNodeMtlsCaCert(t *testing.T) {
	_ = setupSettingMtlsDB(t)

	got, err := (&NodeService{}).NodeMtlsCaCert()
	if err != nil {
		t.Fatalf("NodeMtlsCaCert: %v", err)
	}
	block, _ := pem.Decode([]byte(got))
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("NodeMtlsCaCert must return a CERTIFICATE PEM, got %q", got)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse returned cert: %v", err)
	}
	if !cert.IsCA {
		t.Fatal("NodeMtlsCaCert must return the CA certificate (IsCA)")
	}
}

func TestSetNodeMtlsTrustCA(t *testing.T) {
	_ = setupSettingMtlsDB(t)
	ns := &NodeService{}
	settings := SettingService{}

	ca, err := settings.EnsureNodeMtlsCA()
	if err != nil {
		t.Fatalf("EnsureNodeMtlsCA: %v", err)
	}

	if err := ns.SetNodeMtlsTrustCA(string(ca.CertPEM)); err != nil {
		t.Fatalf("SetNodeMtlsTrustCA(valid): %v", err)
	}
	pool, err := settings.NodeMtlsClientCAPool()
	if err != nil || pool == nil {
		t.Fatalf("valid trust CA must persist + build a pool: pool=%v err=%v", pool, err)
	}

	if err := ns.SetNodeMtlsTrustCA("not a certificate"); err == nil {
		t.Fatal("invalid PEM must be rejected (fail closed)")
	}

	if err := ns.SetNodeMtlsTrustCA(""); err != nil {
		t.Fatalf("clearing the trust CA must be allowed: %v", err)
	}
	pool, _ = settings.NodeMtlsClientCAPool()
	if pool != nil {
		t.Fatal("cleared trust CA must yield a nil pool (mTLS off)")
	}
}
