package pia

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCatalogClientReturnsExplicitlyVerifiedSnapshot(t *testing.T) {
	payload := []byte(`{"version":6,"groups":{"wg":[]},"regions":[]}`)
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
	signed := append(append(append([]byte{}, payload...), '\n'), []byte(base64.StdEncoding.EncodeToString(signature))...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(signed)
	}))
	defer server.Close()
	client := NewCatalogClient(server.URL+"/v6", pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	snapshot, err := client.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.SignatureVerified || snapshot.SchemaHint != "6" || string(snapshot.Payload) != string(payload) {
		t.Fatalf("unexpected verified snapshot: %+v", snapshot)
	}
}

func TestCatalogClientRejectsUnsafeResponses(t *testing.T) {
	tests := []struct {
		name, contentType, body string
		maxBody                 int64
		wantCode                string
	}{
		{name: "html", contentType: "text/html", body: "<html>maintenance</html>", wantCode: CodeCatalogSchemaUnsupported},
		{name: "oversized", contentType: "application/octet-stream", body: strings.Repeat("x", 65), maxBody: 64, wantCode: CodeCatalogUnavailable},
		{name: "unsigned", contentType: "application/json", body: `{"version":6,"groups":{},"regions":[]}`, wantCode: CodeCatalogSignatureInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			client := NewCatalogClient(server.URL+"/v6", []byte("invalid public key"))
			if test.maxBody > 0 {
				client.MaxBody = test.maxBody
			}
			_, err := client.Fetch(context.Background())
			if CodeOf(err) != test.wantCode {
				t.Fatalf("got %s, want %s: %v", CodeOf(err), test.wantCode, err)
			}
		})
	}

	payload := []byte(`{"version":6,"groups":{"wg":[]},"regions":[]}`)
	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	signature, err := rsa.SignPKCS1v15(rand.Reader, signingKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	signed := append(append(append([]byte{}, payload...), '\n'), []byte(base64.StdEncoding.EncodeToString(signature))...)
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	wrongDER, err := x509.MarshalPKIXPublicKey(&wrongKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("valid signature from unpinned key", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(signed)
		}))
		defer server.Close()
		client := NewCatalogClient(server.URL+"/v6", pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: wrongDER}))
		_, err := client.Fetch(context.Background())
		if CodeOf(err) != CodeCatalogSignatureInvalid {
			t.Fatalf("got %s, want %s: %v", CodeOf(err), CodeCatalogSignatureInvalid, err)
		}
	})
}
