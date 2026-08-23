package pia

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifySignedServerList(t *testing.T) {
	payload := []byte(`{"version":6,"groups":{},"regions":[]}`)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	signed := append(append(append([]byte{}, payload...), '\n', '\n'), []byte(base64.StdEncoding.EncodeToString(signature))...)
	validSigned := append([]byte(nil), signed...)
	verified, err := VerifySignedServerList(signed, publicPEM)
	if err != nil {
		t.Fatal(err)
	}
	if string(verified) != string(payload) {
		t.Fatalf("verified payload changed: %s", verified)
	}
	signed[10] ^= 1
	if _, err := VerifySignedServerList(signed, publicPEM); err == nil {
		t.Fatal("expected tampered payload to fail signature verification")
	}
	for name, input := range map[string][]byte{
		"missing signature": payload,
		"invalid base64":    append(append([]byte{}, payload...), []byte("\nnot-base64!")...),
		"trailing garbage":  append(append([]byte{}, validSigned...), []byte("\nextra")...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := VerifySignedServerList(input, publicPEM); err == nil {
				t.Fatal("expected malformed signed response to be rejected")
			}
		})
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "serverlist", "invalid_signature.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySignedServerList(fixture, publicPEM); err == nil {
		t.Fatal("expected invalid-signature fixture to be rejected")
	}
}
