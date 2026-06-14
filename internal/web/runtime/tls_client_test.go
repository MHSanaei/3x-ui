package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

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
	if _, err := HTTPClientForNode(&model.Node{Scheme: "https", TlsVerifyMode: "pin", PinnedCertSha256: "not-a-pin"}, ""); err == nil {
		t.Fatal("expected error for invalid pin")
	}
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
