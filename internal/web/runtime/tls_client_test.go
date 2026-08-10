package runtime

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
)

type generationProbeTransport struct {
	id     string
	closed atomic.Int32
}

func (t *generationProbeTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     make(http.Header),
		Request:    &http.Request{},
	}, nil
}

func (t *generationProbeTransport) CloseIdleConnections() {
	t.closed.Add(1)
}

func TestCredentialRotatingTransportDropsOldPoolBeforeNextRequest(t *testing.T) {
	var selected atomic.Pointer[generationProbeTransport]
	oldTransport := &generationProbeTransport{id: "old"}
	newTransport := &generationProbeTransport{id: "new"}
	selected.Store(oldTransport)

	rotating, err := newCredentialRotatingTransport(func() (idleClosingRoundTripper, error) {
		return selected.Load(), nil
	})
	if err != nil {
		t.Fatalf("newCredentialRotatingTransport: %v", err)
	}
	rotating.mu.Lock()
	initial := rotating.current
	rotating.mu.Unlock()
	if initial != oldTransport {
		t.Fatalf("initial transport = %p, want old %p", initial, oldTransport)
	}

	selected.Store(newTransport)
	InvalidateMasterClientConnections()

	req := httptest.NewRequest(http.MethodGet, "https://node.example.test/panel/api/server/status", nil)
	resp, err := rotating.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip after credential rotation: %v", err)
	}
	_ = resp.Body.Close()

	rotating.mu.Lock()
	current := rotating.current
	rotating.mu.Unlock()
	if current != newTransport {
		t.Fatalf("transport after invalidation = %p, want new %p", current, newTransport)
	}
	if got := oldTransport.closed.Load(); got != 1 {
		t.Fatalf("old transport CloseIdleConnections calls = %d, want 1", got)
	}
}

func TestReloadMasterClientConnectionsValidatesProviderBeforeInvalidation(t *testing.T) {
	before := masterCertEpoch.Load()
	SetMasterClientCertProvider(func() (tls.Certificate, error) {
		return tls.Certificate{}, context.Canceled
	})
	if err := ReloadMasterClientConnections(); err == nil {
		t.Fatal("reload with an invalid provider unexpectedly succeeded")
	}
	if got := masterCertEpoch.Load(); got != before {
		t.Fatalf("failed reload changed generation from %d to %d", before, got)
	}

	SetMasterClientCertProvider(func() (tls.Certificate, error) {
		return masterCertForTest(t), nil
	})
	t.Cleanup(func() { SetMasterClientCertProvider(nil) })
	if err := ReloadMasterClientConnections(); err != nil {
		t.Fatalf("ReloadMasterClientConnections: %v", err)
	}
	if got := masterCertEpoch.Load(); got != before+1 {
		t.Fatalf("successful reload generation = %d, want %d", got, before+1)
	}
}

func TestCredentialRotatingTransportRejectsBuildAcrossInvalidation(t *testing.T) {
	oldTransport := &generationProbeTransport{id: "old"}
	newTransport := &generationProbeTransport{id: "new"}
	var selected atomic.Pointer[generationProbeTransport]
	selected.Store(oldTransport)

	firstBuildCaptured := make(chan struct{})
	releaseFirstBuild := make(chan struct{})
	var once sync.Once
	build := func() (idleClosingRoundTripper, error) {
		captured := selected.Load()
		once.Do(func() {
			close(firstBuildCaptured)
			<-releaseFirstBuild
		})
		return captured, nil
	}

	type result struct {
		transport *credentialRotatingTransport
		err       error
	}
	resultCh := make(chan result, 1)
	go func() {
		transport, err := newCredentialRotatingTransport(build)
		resultCh <- result{transport: transport, err: err}
	}()

	<-firstBuildCaptured
	selected.Store(newTransport)
	InvalidateMasterClientConnections()
	close(releaseFirstBuild)

	got := <-resultCh
	if got.err != nil {
		t.Fatalf("newCredentialRotatingTransport: %v", got.err)
	}
	got.transport.mu.Lock()
	current := got.transport.current
	got.transport.mu.Unlock()
	if current != newTransport {
		t.Fatalf("transport built across invalidation = %p, want new %p", current, newTransport)
	}
	if calls := oldTransport.closed.Load(); calls != 1 {
		t.Fatalf("stale transport CloseIdleConnections calls = %d, want 1", calls)
	}
}

func TestHTTPClientForNodeMTLSRebuildsTLSConfigAfterCredentialInvalidation(t *testing.T) {
	oldCert := masterCertForTest(t)
	newCert := masterCertForTest(t)
	selected := oldCert
	SetMasterClientCertProvider(func() (tls.Certificate, error) { return selected, nil })
	t.Cleanup(func() { SetMasterClientCertProvider(nil) })

	client, err := HTTPClientForNode(&model.Node{
		Scheme:        "https",
		Address:       "node.example.test",
		Port:          443,
		TlsVerifyMode: "mtls",
	}, "")
	if err != nil {
		t.Fatalf("HTTPClientForNode: %v", err)
	}
	rotating, ok := client.Transport.(*credentialRotatingTransport)
	if !ok {
		t.Fatalf("transport = %T, want *credentialRotatingTransport", client.Transport)
	}
	leaf := func() []byte {
		rotating.mu.Lock()
		defer rotating.mu.Unlock()
		transport, ok := rotating.current.(*http.Transport)
		if !ok {
			t.Fatalf("current transport = %T, want *http.Transport", rotating.current)
		}
		return transport.TLSClientConfig.Certificates[0].Certificate[0]
	}
	if got := leaf(); string(got) != string(oldCert.Certificate[0]) {
		t.Fatal("initial TLS config does not contain the old credential")
	}

	selected = newCert
	InvalidateMasterClientConnections()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://node.example.test/", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	if _, err := client.Do(req); err == nil {
		t.Fatal("canceled request unexpectedly succeeded")
	}
	if got := leaf(); string(got) != string(newCert.Certificate[0]) {
		t.Fatal("TLS config retained the old credential after invalidation")
	}
}

// masterCertForTest builds a real CA-signed client certificate for mtls tests.
func masterCertForTest(t *testing.T) tls.Certificate {
	t.Helper()
	ca, err := crypto.GenerateNodeCA("test ca")
	if err != nil {
		t.Fatalf("GenerateNodeCA: %v", err)
	}
	client, err := crypto.IssueClientCert(ca, "master")
	if err != nil {
		t.Fatalf("IssueClientCert: %v", err)
	}
	cert, err := tls.X509KeyPair(client.CertPEM, client.KeyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	return cert
}

// TestTLSConfigForNode_MTLS_PresentsClientCert asserts the mtls branch presents
// the master client cert and verifies the node's server cert against system
// roots (no InsecureSkipVerify, no custom RootCAs).
func TestTLSConfigForNode_MTLS_PresentsClientCert(t *testing.T) {
	cert := masterCertForTest(t)
	SetMasterClientCertProvider(func() (tls.Certificate, error) { return cert, nil })
	t.Cleanup(func() { SetMasterClientCertProvider(nil) })

	cfg, err := tlsConfigForNode(&model.Node{TlsVerifyMode: "mtls"})
	if err != nil {
		t.Fatalf("tlsConfigForNode(mtls): %v", err)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("mtls config must present exactly one client certificate, got %d", len(cfg.Certificates))
	}
	if cfg.InsecureSkipVerify {
		t.Fatal("mtls must NOT skip server verification")
	}
	if cfg.RootCAs != nil {
		t.Fatal("mtls verifies the node server against system roots (RootCAs must be nil)")
	}
}

// TestTLSConfigForNode_MTLS_NoProviderFailsClosed asserts mtls fails closed when
// no master client certificate is available, rather than silently dropping auth.
func TestTLSConfigForNode_MTLS_NoProviderFailsClosed(t *testing.T) {
	SetMasterClientCertProvider(nil)
	if _, err := tlsConfigForNode(&model.Node{TlsVerifyMode: "mtls"}); err == nil {
		t.Fatal("mtls without a configured client cert provider must fail closed")
	}
}

// nodeForServer builds a node pointing at a loopback test server (loopback is
// SSRF-blocked, so AllowPrivateAddress is set for the guarded dialer).
func nodeForServer(t *testing.T, srv *httptest.Server, mode, pin string) *model.Node {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}
	return &model.Node{
		Id:                  1,
		Name:                "n1",
		Scheme:              "https",
		Address:             u.Hostname(),
		Port:                port,
		BasePath:            "/",
		ApiToken:            "token",
		Enable:              true,
		AllowPrivateAddress: true,
		TlsVerifyMode:       mode,
		PinnedCertSha256:    pin,
	}
}

func leafPinBase64(srv *httptest.Server) string {
	sum := sha256.Sum256(srv.Certificate().Raw)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// A self-signed node must be reachable by Remote ops under skip/pin and
// rejected under verify — the split issue #5264 reported.
func TestRemoteHonorsTLSVerifyMode(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"obj":[]}`))
	}))
	defer srv.Close()

	goodPin := leafPinBase64(srv)
	wrongPin := base64.StdEncoding.EncodeToString(make([]byte, sha256.Size))

	cases := []struct {
		name    string
		mode    string
		pin     string
		wantErr bool
	}{
		{"verify rejects self-signed", "verify", "", true},
		{"skip accepts self-signed", "skip", "", false},
		{"pin accepts matching cert", "pin", goodPin, false},
		{"pin rejects mismatched cert", "pin", wrongPin, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NewRemote(nodeForServer(t, srv, c.mode, c.pin), nil)
			_, err := r.ListInboundOptions(context.Background())
			if c.wantErr && err == nil {
				t.Fatalf("mode %q: expected error, got nil", c.mode)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("mode %q: unexpected error: %v", c.mode, err)
			}
		})
	}
}

// The lazily-built client is cached for the Remote's lifetime so repeated
// operations reuse one pooled transport rather than rebuilding TLS each call.
func TestRemoteClientCached(t *testing.T) {
	r := NewRemote(&model.Node{Scheme: "https", TlsVerifyMode: "skip"}, nil)
	c1, err1 := r.httpClient()
	c2, err2 := r.httpClient()
	if err1 != nil || err2 != nil {
		t.Fatalf("httpClient errors: %v %v", err1, err2)
	}
	if c1 != c2 {
		t.Fatal("expected the same cached client across calls")
	}
}

func TestHTTPClientForNodeVerifyShared(t *testing.T) {
	// verify mode and plain http both reuse the shared default client.
	for _, n := range []*model.Node{
		{Scheme: "https", TlsVerifyMode: "verify"},
		{Scheme: "https", TlsVerifyMode: ""},
		{Scheme: "http", TlsVerifyMode: "skip"},
	} {
		c, err := HTTPClientForNode(n, "")
		if err != nil {
			t.Fatalf("HTTPClientForNode(%+v): %v", n, err)
		}
		if c != defaultNodeHTTPClient {
			t.Fatalf("HTTPClientForNode(%+v) = %p, want shared default %p", n, c, defaultNodeHTTPClient)
		}
	}
}

func TestHTTPClientForNodePinInvalid(t *testing.T) {
	// pin mode must fail closed, and with a specific error per cause — not merely
	// "some error" (which a bug anywhere in the build path would also satisfy).
	cases := []struct {
		name    string
		pin     string
		wantErr string
	}{
		{"garbage pin", "not-a-pin", "must be a SHA-256 hash"},
		{"empty pin", "", "certificate pin is empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := HTTPClientForNode(&model.Node{Scheme: "https", TlsVerifyMode: "pin", PinnedCertSha256: c.pin}, "")
			if err == nil {
				t.Fatalf("expected error for pin %q", c.pin)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), c.wantErr)
			}
		})
	}
}

// TestHTTPClientForNode_ProxyPinPreservesPinEnforcement covers the proxy+pin branch
// (tls_client.go:43-52): when a node uses a proxy AND pin mode, the proxy client's
// transport must carry the pinning tls.Config (the `transport.TLSClientConfig = tlsCfg`
// line). Dropping it would silently disable certificate pinning whenever a proxy is set.
func TestHTTPClientForNode_ProxyPinPreservesPinEnforcement(t *testing.T) {
	pin := base64.StdEncoding.EncodeToString(make([]byte, sha256.Size))
	n := &model.Node{Scheme: "https", TlsVerifyMode: "pin", PinnedCertSha256: pin}

	c, err := HTTPClientForNode(n, "socks5://127.0.0.1:1080")
	if err != nil {
		t.Fatalf("HTTPClientForNode: %v", err)
	}
	if c == defaultNodeHTTPClient {
		t.Fatal("proxy client must not be the shared default client")
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport is %T, want *http.Transport", c.Transport)
	}
	if tr.TLSClientConfig == nil || tr.TLSClientConfig.VerifyConnection == nil {
		t.Fatal("pin mode over a proxy must install a pinning tls.Config (VerifyConnection); pin enforcement was dropped")
	}
}

// TestHTTPClientForNode_ProxyVerifyNoPin covers the proxy+verify branch
// (tls_client.go:40-42): verify mode over a proxy returns the proxy client as-is,
// using system-CA verification and NOT a pin VerifyConnection.
func TestHTTPClientForNode_ProxyVerifyNoPin(t *testing.T) {
	n := &model.Node{Scheme: "https", TlsVerifyMode: "verify"}
	c, err := HTTPClientForNode(n, "socks5://127.0.0.1:1080")
	if err != nil {
		t.Fatalf("HTTPClientForNode: %v", err)
	}
	if c == defaultNodeHTTPClient {
		t.Fatal("proxy client must not be the shared default client")
	}
	if tr, ok := c.Transport.(*http.Transport); ok && tr.TLSClientConfig != nil && tr.TLSClientConfig.VerifyConnection != nil {
		t.Fatal("verify mode must not install a pin VerifyConnection")
	}
}

// TestTLSConfigForNode_CurrentContract locks the pre-mTLS behavior of
// tlsConfigForNode so the "mtls" branch added later cannot silently regress the
// existing skip/pin modes (characterization — passes on unchanged code).
func TestTLSConfigForNode_CurrentContract(t *testing.T) {
	t.Run("skip disables verification with no VerifyConnection", func(t *testing.T) {
		cfg, err := tlsConfigForNode(&model.Node{TlsVerifyMode: "skip"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.InsecureSkipVerify {
			t.Fatal("skip mode must set InsecureSkipVerify")
		}
		if cfg.VerifyConnection != nil {
			t.Fatal("skip mode must not install a VerifyConnection")
		}
	})
	t.Run("pin installs a VerifyConnection", func(t *testing.T) {
		pin := base64.StdEncoding.EncodeToString(make([]byte, sha256.Size))
		cfg, err := tlsConfigForNode(&model.Node{TlsVerifyMode: "pin", PinnedCertSha256: pin})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.VerifyConnection == nil {
			t.Fatal("pin mode must install a VerifyConnection")
		}
	})
}

func TestDecodeCertPin(t *testing.T) {
	raw := sha256.Sum256([]byte("cert"))
	hexColon := strings.ToUpper(hex.EncodeToString(raw[:]))
	// reinsert colons in openssl -fingerprint style
	var withColons strings.Builder
	for i := 0; i < len(hexColon); i += 2 {
		if i > 0 {
			withColons.WriteByte(':')
		}
		withColons.WriteString(hexColon[i : i+2])
	}

	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"base64 std", base64.StdEncoding.EncodeToString(raw[:]), false},
		{"base64 raw url", base64.RawURLEncoding.EncodeToString(raw[:]), false},
		{"hex bare", hex.EncodeToString(raw[:]), false},
		{"hex colon openssl", withColons.String(), false},
		{"empty", "", true},
		{"garbage", "not-a-pin", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := DecodeCertPin(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", c.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", c.in, err)
			}
			if string(got) != string(raw[:]) {
				t.Fatalf("decoded bytes mismatch for %q", c.in)
			}
		})
	}
}
