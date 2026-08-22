package pia

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testPubKey() string {
	raw := make([]byte, 32)
	raw[0] = 1
	return base64.StdEncoding.EncodeToString(raw)
}

func TestParseRegistrationFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "addkey", "success.json"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := parseRegistration(raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.PeerIP.String() != "10.42.0.2/32" || result.ServerPort != 51820 || result.ServerIP.String() != "198.51.100.10" || len(result.DNSServers) != 2 {
		t.Fatalf("unexpected registration result: %+v", result)
	}
	prefixed := []byte(`{"status":"OK","peer_ip":"10.42.0.3/32","server_key":"AgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","server_ip":"198.51.100.10","server_port":51820,"dns_servers":["10.0.0.242"]}`)
	prefixedResult, err := parseRegistration(prefixed)
	if err != nil || prefixedResult.PeerIP.String() != "10.42.0.3/32" {
		t.Fatalf("expected an explicit /32 peer address to remain supported: result=%+v err=%v", prefixedResult, err)
	}
	missing, err := os.ReadFile(filepath.Join("testdata", "addkey", "missing_port.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseRegistration(missing); err == nil || CodeOf(err) != CodeRegistrationInvalid {
		t.Fatalf("expected missing server port to be rejected: %v", err)
	}

	dnsRaw, err := os.ReadFile(filepath.Join("testdata", "addkey", "invalid_dns.json"))
	if err != nil {
		t.Fatal(err)
	}
	dnsResult, err := parseRegistration(dnsRaw)
	if err != nil || len(dnsResult.DNSServers) != 0 {
		t.Fatalf("invalid dns_servers must be ignored: result=%+v err=%v", dnsResult, err)
	}

	invalidFixtures := []struct{ file, wantCode string }{
		{"status_error.json", CodeRegistrationRejected},
		{"invalid_peer_ip.json", CodeRegistrationInvalid},
		{"invalid_peer_prefix.json", CodeRegistrationInvalid},
		{"invalid_server_key.json", CodeRegistrationInvalid},
		{"invalid_server_ip.json", CodeRegistrationInvalid},
		{"invalid_port.json", CodeRegistrationInvalid},
	}
	for _, test := range invalidFixtures {
		t.Run(test.file, func(t *testing.T) {
			raw, readErr := os.ReadFile(filepath.Join("testdata", "addkey", test.file))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if _, parseErr := parseRegistration(raw); CodeOf(parseErr) != test.wantCode {
				t.Fatalf("expected %s, got %s: %v", test.wantCode, CodeOf(parseErr), parseErr)
			}
		})
	}
}

func TestRegistrationTLSHostnameAndCA(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "addkey", "success.json"))
	if err != nil {
		t.Fatal(err)
	}
	const token = "test-token-value-that-is-long-enough"
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pt") != token || r.URL.Query().Get("pubkey") == "" {
			t.Errorf("registration query is missing required values")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	server.StartTLS()
	defer server.Close()
	certificate := server.Certificate()
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})
	host, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := net.LookupPort("tcp", portText)
	if err != nil {
		t.Fatal(err)
	}
	client := NewRegistrationClient(caPEM)
	client.Port = uint16(port)
	key := testPubKey()
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	_, err = client.RegisterKey(context.Background(), WireGuardServer{Hostname: "example.com", IP: netip.MustParseAddr(host)}, token, key)
	if err != nil {
		t.Fatalf("expected TLS registration to succeed: %v", err)
	}
	_, err = client.RegisterKey(context.Background(), WireGuardServer{Hostname: "example.com", IP: netip.MustParseAddr(host)}, token, "")
	if CodeOf(err) != CodeInvalidInput {
		t.Fatalf("zero public key returned %s, want %s", CodeOf(err), CodeInvalidInput)
	}
	_, err = client.RegisterKey(context.Background(), WireGuardServer{Hostname: "wrong.example", IP: netip.MustParseAddr(host)}, token, key)
	if CodeOf(err) != CodeTLSValidation {
		t.Fatalf("wrong hostname returned %s, want %s: %v", CodeOf(err), CodeTLSValidation, err)
	}
	client = NewRegistrationClient([]byte("-----BEGIN CERTIFICATE-----\ninvalid\n-----END CERTIFICATE-----"))
	client.Port = uint16(port)
	_, err = client.RegisterKey(context.Background(), WireGuardServer{Hostname: "example.com", IP: netip.MustParseAddr(host)}, token, key)
	if CodeOf(err) != CodeTLSValidation {
		t.Fatalf("wrong CA returned %s, want %s", CodeOf(err), CodeTLSValidation)
	}

	expiredServer, expiredCA := newExpiredTLSServer(t, fixture)
	defer expiredServer.Close()
	expiredHost, expiredPortText, err := net.SplitHostPort(expiredServer.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	expiredPort, err := net.LookupPort("tcp", expiredPortText)
	if err != nil {
		t.Fatal(err)
	}
	client = NewRegistrationClient(expiredCA)
	client.Port = uint16(expiredPort)
	_, err = client.RegisterKey(context.Background(), WireGuardServer{Hostname: "example.com", IP: netip.MustParseAddr(expiredHost)}, token, key)
	if CodeOf(err) != CodeTLSValidation {
		t.Fatalf("expired certificate returned %s, want %s: %v", CodeOf(err), CodeTLSValidation, err)
	}
}

func TestRegistrationResponseGuardsAndRedirect(t *testing.T) {
	var destinationHits atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		destinationHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	tests := []struct {
		name, contentType, body, redirect string
		maxBody                           int64
		delay                             time.Duration
		wantCode                          string
	}{
		{name: "HTML", contentType: "text/html", body: "<html>maintenance</html>", wantCode: CodeRegistrationInvalid},
		{name: "oversized", contentType: "application/json", body: strings.Repeat("x", 65), maxBody: 64, wantCode: CodeRegistrationInvalid},
		{name: "redirect", contentType: "application/json", redirect: destination.URL, wantCode: CodeNetworkUnavailable},
		{name: "timeout", contentType: "application/json", body: `{"status":"OK"}`, delay: 100 * time.Millisecond, wantCode: CodeTimeout},
	}
	const token = "test-token-value-that-is-long-enough"
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.delay > 0 {
					time.Sleep(test.delay)
				}
				if test.redirect != "" {
					http.Redirect(w, r, test.redirect, http.StatusTemporaryRedirect)
					return
				}
				w.Header().Set("Content-Type", test.contentType)
				_, _ = w.Write([]byte(test.body))
			}))
			server.StartTLS()
			defer server.Close()
			caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
			host, portText, err := net.SplitHostPort(server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			port, err := net.LookupPort("tcp", portText)
			if err != nil {
				t.Fatal(err)
			}
			client := NewRegistrationClient(caPEM)
			client.Port = uint16(port)
			if test.maxBody > 0 {
				client.MaxBody = test.maxBody
			}
			if test.delay > 0 {
				client.Timeout = 25 * time.Millisecond
			}
			_, err = client.RegisterKey(
				context.Background(),
				WireGuardServer{Hostname: "example.com", IP: netip.MustParseAddr(host)},
				token,
				testPubKey(),
			)
			if err == nil {
				t.Fatal("expected registration error")
			}
			if CodeOf(err) != test.wantCode {
				t.Fatalf("got %s, want %s: %v", CodeOf(err), test.wantCode, err)
			}
			if containsSecret(err.Error(), token) {
				t.Fatalf("token leaked in error: %v", err)
			}
		})
	}
	if destinationHits.Load() != 0 {
		t.Fatal("registration request followed a redirect and exposed secrets")
	}
}

func newExpiredTLSServer(t *testing.T, response []byte) (*httptest.Server, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "example.com"},
		DNSNames:              []string{"example.com"},
		NotBefore:             time.Now().Add(-48 * time.Hour),
		NotAfter:              time.Now().Add(-24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	return server, certPEM
}
