package runtime

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/op/go-logging"
)

// tlsClientForNode logs a warning on the bad-pin fail-safe path, which panics if
// the global logger backend is nil. Initialize it once for the package's tests.
func TestMain(m *testing.M) {
	xuilogger.InitLogger(logging.ERROR)
	os.Exit(m.Run())
}

func selfSignedLeaf(t *testing.T) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "node.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("createcert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parsecert: %v", err)
	}
	return cert
}

func tlsConfigOf(t *testing.T, c *http.Client) *tls.Config {
	t.Helper()
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport is not *http.Transport")
	}
	return tr.TLSClientConfig
}

// "verify" mode and http nodes must reuse the shared SSRF-guarded client with
// default certificate validation (no custom TLS config).
func TestTLSClientForNode_VerifyAndHTTPReuseShared(t *testing.T) {
	if got := tlsClientForNode(&model.Node{Name: "v", Scheme: "https", TlsVerifyMode: "verify"}); got != remoteHTTPClient {
		t.Errorf("verify mode should reuse the shared client")
	}
	if got := tlsClientForNode(&model.Node{Name: "h", Scheme: "http", TlsVerifyMode: "pin"}); got != remoteHTTPClient {
		t.Errorf("http node should reuse the shared client regardless of mode")
	}
	if got := tlsClientForNode(&model.Node{Name: "d", Scheme: "https"}); got != remoteHTTPClient {
		t.Errorf("empty mode defaults to verify -> shared client")
	}
}

// "skip" mode must disable verification on a dedicated client.
func TestTLSClientForNode_Skip(t *testing.T) {
	c := tlsClientForNode(&model.Node{Name: "s", Scheme: "https", TlsVerifyMode: "skip"})
	if c == remoteHTTPClient {
		t.Fatalf("skip mode must not reuse the shared verifying client")
	}
	cfg := tlsConfigOf(t, c)
	if cfg == nil || !cfg.InsecureSkipVerify {
		t.Errorf("skip mode must set InsecureSkipVerify=true")
	}
	if cfg.VerifyConnection != nil {
		t.Errorf("skip mode must not set a pin VerifyConnection")
	}
}

// "pin" mode must accept the matching leaf and reject a different cert, while
// still bypassing the default chain (InsecureSkipVerify).
func TestTLSClientForNode_PinAcceptsAndRejects(t *testing.T) {
	leaf := selfSignedLeaf(t)
	sum := sha256.Sum256(leaf.Raw)
	pin := base64.StdEncoding.EncodeToString(sum[:])

	c := tlsClientForNode(&model.Node{Name: "p", Scheme: "https", TlsVerifyMode: "pin", PinnedCertSha256: pin})
	cfg := tlsConfigOf(t, c)
	if cfg == nil || !cfg.InsecureSkipVerify || cfg.VerifyConnection == nil {
		t.Fatalf("pin mode must skip default chain and set VerifyConnection")
	}
	if err := cfg.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}}); err != nil {
		t.Errorf("pin should accept the matching leaf: %v", err)
	}
	other := selfSignedLeaf(t)
	if err := cfg.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{other}}); err == nil {
		t.Errorf("pin must reject a non-matching leaf")
	}
	if err := cfg.VerifyConnection(tls.ConnectionState{}); err == nil {
		t.Errorf("pin must reject an empty certificate chain")
	}
}

// A malformed pin must fail safe to the default verifying client, never to skip.
func TestTLSClientForNode_BadPinFailsSafe(t *testing.T) {
	c := tlsClientForNode(&model.Node{Name: "bad", Scheme: "https", TlsVerifyMode: "pin", PinnedCertSha256: "not-a-real-hash"})
	if c != remoteHTTPClient {
		t.Errorf("invalid pin must fall back to the shared verifying client, not skip")
	}
}

func TestDecodeCertPin_HexAndBase64(t *testing.T) {
	raw := sha256.Sum256([]byte("x"))
	for _, s := range []string{
		base64.StdEncoding.EncodeToString(raw[:]),
		base64.RawStdEncoding.EncodeToString(raw[:]),
		hexColons(raw[:]),
	} {
		got, err := decodeCertPin(s)
		if err != nil {
			t.Errorf("decode %q: %v", s, err)
			continue
		}
		if string(got) != string(raw[:]) {
			t.Errorf("decode %q: wrong bytes", s)
		}
	}
	if _, err := decodeCertPin("zzzz"); err == nil {
		t.Errorf("garbage pin must error")
	}
	if _, err := decodeCertPin(""); err == nil {
		t.Errorf("empty pin must error")
	}
}

func hexColons(b []byte) string {
	const hexdig = "0123456789abcdef"
	out := make([]byte, 0, len(b)*3)
	for i, c := range b {
		if i > 0 {
			out = append(out, ':')
		}
		out = append(out, hexdig[c>>4], hexdig[c&0x0f])
	}
	return string(out)
}
