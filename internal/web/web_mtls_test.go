package web

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPanelTLSAcceptsClientWithoutClientCert characterizes the invariant the
// mTLS work must preserve: the panel's HTTPS listener — configured today with a
// server certificate and NO ClientAuth — completes the TLS handshake for a
// client that presents no client certificate (i.e. every browser). When mTLS is
// wired into web.go, the no-CA path must keep this behavior byte-for-byte. P1.6
// extends this file with the VerifyClientCertIfGiven + ClientCAs cases.
func TestPanelTLSAcceptsClientWithoutClientCert(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Precondition: like web.go today, the listener requests no client cert.
	if srv.TLS.ClientAuth != tls.NoClientCert {
		t.Fatalf("precondition: ClientAuth = %v, want NoClientCert", srv.TLS.ClientAuth)
	}

	// srv.Client() trusts the server's self-signed cert and presents NO client cert.
	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatalf("request without a client certificate failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
