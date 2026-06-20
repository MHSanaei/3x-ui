package web

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
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

// TestApplyNodeMtls exercises the listener policy applied by web.go: a nil pool
// leaves the listener unchanged (no client auth, browsers work); a set pool is
// request-but-don't-require, so no-cert clients still handshake while a
// CA-signed client cert is verified and a foreign cert is rejected.
func TestApplyNodeMtls(t *testing.T) {
	ca, err := crypto.GenerateNodeCA("test ca")
	if err != nil {
		t.Fatalf("GenerateNodeCA: %v", err)
	}
	clientPEM, err := crypto.IssueClientCert(ca, "master")
	if err != nil {
		t.Fatalf("IssueClientCert: %v", err)
	}
	clientCert, err := tls.X509KeyPair(clientPEM.CertPEM, clientPEM.KeyPEM)
	if err != nil {
		t.Fatalf("client X509KeyPair: %v", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(ca.CertPEM) {
		t.Fatal("append CA to pool")
	}

	otherCA, err := crypto.GenerateNodeCA("other ca")
	if err != nil {
		t.Fatalf("GenerateNodeCA(other): %v", err)
	}
	foreignPEM, err := crypto.IssueClientCert(otherCA, "intruder")
	if err != nil {
		t.Fatalf("IssueClientCert(foreign): %v", err)
	}
	foreignCert, err := tls.X509KeyPair(foreignPEM.CertPEM, foreignPEM.KeyPEM)
	if err != nil {
		t.Fatalf("foreign X509KeyPair: %v", err)
	}

	newServer := func(pool *x509.CertPool) *httptest.Server {
		srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := 0
			if r.TLS != nil {
				n = len(r.TLS.VerifiedChains)
			}
			w.Header().Set("X-Verified-Chains", strconv.Itoa(n))
			w.WriteHeader(http.StatusOK)
		}))
		srv.TLS = &tls.Config{}
		applyNodeMtls(srv.TLS, pool)
		srv.StartTLS()
		return srv
	}
	// clientFor forces the client to present cert via GetClientCertificate so the
	// server's verification is what's under test (the default Certificates path
	// would let the Go client silently withhold a cert whose CA the server didn't
	// advertise, masking the reject behavior).
	clientFor := func(srv *httptest.Server, cert *tls.Certificate) *http.Client {
		roots := x509.NewCertPool()
		roots.AddCert(srv.Certificate())
		cfg := &tls.Config{RootCAs: roots}
		if cert != nil {
			c := *cert
			cfg.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
				return &c, nil
			}
		}
		return &http.Client{Transport: &http.Transport{TLSClientConfig: cfg}}
	}

	t.Run("nil pool leaves the listener without client auth", func(t *testing.T) {
		srv := newServer(nil)
		defer srv.Close()
		if srv.TLS.ClientAuth != tls.NoClientCert {
			t.Fatalf("nil pool must not set ClientAuth, got %v", srv.TLS.ClientAuth)
		}
		resp, err := clientFor(srv, nil).Get(srv.URL)
		if err != nil {
			t.Fatalf("no-cert client failed: %v", err)
		}
		resp.Body.Close()
	})

	t.Run("pool set still accepts a no-cert client", func(t *testing.T) {
		srv := newServer(caPool)
		defer srv.Close()
		resp, err := clientFor(srv, nil).Get(srv.URL)
		if err != nil {
			t.Fatalf("no-cert client must still handshake under VerifyClientCertIfGiven: %v", err)
		}
		defer resp.Body.Close()
		if got := resp.Header.Get("X-Verified-Chains"); got != "0" {
			t.Fatalf("no-cert client verified chains = %s, want 0", got)
		}
	})

	t.Run("pool set verifies the master client cert", func(t *testing.T) {
		srv := newServer(caPool)
		defer srv.Close()
		resp, err := clientFor(srv, &clientCert).Get(srv.URL)
		if err != nil {
			t.Fatalf("master client cert must be accepted: %v", err)
		}
		defer resp.Body.Close()
		if got := resp.Header.Get("X-Verified-Chains"); got != "1" {
			t.Fatalf("master cert verified chains = %s, want 1 (cert was not verified)", got)
		}
	})

	t.Run("pool set rejects a foreign-CA client cert", func(t *testing.T) {
		srv := newServer(caPool)
		defer srv.Close()
		if _, err := clientFor(srv, &foreignCert).Get(srv.URL); err == nil {
			t.Fatal("a client cert from an untrusted CA must fail the handshake")
		}
	})
}
