package runtime

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/wirecodec"
)

// TestRemoteSendsEnvelopeWhenNodeAdvertisesCap: once a node has advertised the
// zstd capability (via a response header on any prior call), a large push is
// sent zstd-compressed with an X-Config-Sha256 of the *uncompressed* body.
func TestRemoteSendsEnvelopeWhenNodeAdvertisesCap(t *testing.T) {
	var capturedEnc, capturedHash string
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(wirecodec.CapsHeader, wirecodec.CapZstd) // advertise on every response
		if r.Method == http.MethodPost {
			capturedEnc = r.Header.Get("Content-Encoding")
			capturedHash = r.Header.Get(wirecodec.HashHeader)
			capturedBody, _ = io.ReadAll(r.Body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)

	// Prime: a prior call learns the cap from the response header.
	if _, err := r.do(context.Background(), http.MethodGet, "ping", nil); err != nil {
		t.Fatalf("prime call: %v", err)
	}

	body := url.Values{}
	body.Set("settings", strings.Repeat("x", 4096))
	if _, err := r.do(context.Background(), http.MethodPost, "panel/api/inbounds/add", body); err != nil {
		t.Fatalf("push: %v", err)
	}

	if capturedEnc != wirecodec.EncodingZstd {
		t.Fatalf("Content-Encoding = %q, want %q", capturedEnc, wirecodec.EncodingZstd)
	}
	if len(capturedHash) != 64 {
		t.Fatalf("missing/short X-Config-Sha256: %q", capturedHash)
	}
	raw, err := wirecodec.Decompress(capturedBody, 1<<20)
	if err != nil {
		t.Fatalf("server could not decompress the body: %v", err)
	}
	if string(raw) != body.Encode() {
		t.Fatalf("decompressed body mismatch: %q != %q", string(raw), body.Encode())
	}
	if wirecodec.Sha256Hex(raw) != capturedHash {
		t.Fatal("X-Config-Sha256 does not match the decompressed body")
	}
}

// TestRemoteSendsPlainWhenNoCap: a node that never advertises the cap (old
// build) receives a plain body — but the integrity hash is still attached
// (harmless to old nodes, verified by new ones).
func TestRemoteSendsPlainWhenNoCap(t *testing.T) {
	var capturedEnc, capturedHash string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			capturedEnc = r.Header.Get("Content-Encoding")
			capturedHash = r.Header.Get(wirecodec.HashHeader)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	body := url.Values{}
	body.Set("settings", strings.Repeat("x", 4096))
	if _, err := r.do(context.Background(), http.MethodPost, "panel/api/inbounds/add", body); err != nil {
		t.Fatalf("push: %v", err)
	}

	if capturedEnc != "" {
		t.Fatalf("a no-cap node must receive a plain body, got Content-Encoding=%q", capturedEnc)
	}
	if len(capturedHash) != 64 {
		t.Fatalf("integrity hash should always be sent, got %q", capturedHash)
	}
}
