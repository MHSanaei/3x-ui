package pia

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"
)

func TestEmbeddedTrustAnchorsAreUsable(t *testing.T) {
	t.Run("server list public key", func(t *testing.T) {
		block, rest := pem.Decode(EmbeddedServerListPublicKey)
		if block == nil {
			t.Fatal("serverlist_public_key.pem does not decode as PEM")
		}
		if block.Type != "PUBLIC KEY" {
			t.Fatalf("PEM block type = %q, want PUBLIC KEY", block.Type)
		}
		if len(bytes.TrimSpace(rest)) != 0 {
			t.Fatalf("trailing data after the public key: %q", rest)
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			t.Fatalf("ParsePKIXPublicKey: %v", err)
		}
		if _, ok := parsed.(*rsa.PublicKey); !ok {
			t.Fatalf("public key type = %T, want *rsa.PublicKey", parsed)
		}
	})

	t.Run("addKey certificate authority", func(t *testing.T) {
		if !x509.NewCertPool().AppendCertsFromPEM(EmbeddedPIACA) {
			t.Fatal("ca.rsa.4096.crt was rejected by AppendCertsFromPEM")
		}
		block, _ := pem.Decode(EmbeddedPIACA)
		if block == nil {
			t.Fatal("ca.rsa.4096.crt does not decode as PEM")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatalf("ParseCertificate: %v", err)
		}
		if !cert.IsCA {
			t.Fatal("the embedded PIA certificate is not a CA")
		}
		if !cert.NotAfter.After(time.Now()) {
			t.Fatalf("the embedded PIA CA expired on %s", cert.NotAfter)
		}
	})
}
