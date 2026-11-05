package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/util/wirecodec"
)

func envelopeTestEngine(t *testing.T, onHandler func()) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(ConfigEnvelopeMiddleware())
	engine.POST("/echo", func(c *gin.Context) {
		if onHandler != nil {
			onHandler()
		}
		b, _ := io.ReadAll(c.Request.Body)
		c.String(http.StatusOK, "%s", string(b))
	})
	return engine
}

func TestConfigEnvelope_DecompressesAndVerifies(t *testing.T) {
	engine := envelopeTestEngine(t, nil)

	orig := []byte(strings.Repeat("payload-", 200))
	packed := wirecodec.Compress(orig)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(packed))
	req.Header.Set("Content-Encoding", wirecodec.EncodingZstd)
	req.Header.Set(wirecodec.HashHeader, wirecodec.Sha256Hex(orig))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != string(orig) {
		t.Fatal("handler did not receive the decompressed body")
	}
	if !strings.Contains(w.Header().Get(wirecodec.CapsHeader), wirecodec.CapZstd) {
		t.Fatal("response must advertise the zstd capability")
	}
}

func TestConfigEnvelope_RejectsHashMismatch(t *testing.T) {
	called := false
	engine := envelopeTestEngine(t, func() { called = true })

	body := []byte("the-real-config")
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(body))
	// Header claims a hash of a *different* body — a tampered/corrupted push.
	req.Header.Set(wirecodec.HashHeader, wirecodec.Sha256Hex([]byte("a-different-config")))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("a hash mismatch must be rejected 4xx, got %d", w.Code)
	}
	if called {
		t.Fatal("the handler must NOT be invoked on a hash mismatch")
	}
}

func TestConfigEnvelope_PlainPassesThrough(t *testing.T) {
	engine := envelopeTestEngine(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("plain=body"))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK || w.Body.String() != "plain=body" {
		t.Fatalf("a request with no envelope headers must pass through unchanged: code=%d body=%q", w.Code, w.Body.String())
	}
}

// A read-only (monitor) token reaches GET routes only; an envelope never rides on one, so
// it must not make the panel buffer and decode a body there.
func TestConfigEnvelope_RefusesAnEnvelopeOnASafeMethod(t *testing.T) {
	called := false
	engine := envelopeTestEngine(t, nil)
	engine.GET("/status", func(c *gin.Context) { called = true })
	body := bytes.Repeat([]byte("A"), 1<<20)
	req := httptest.NewRequest(http.MethodGet, "/status", bytes.NewReader(wirecodec.Compress(body)))
	req.Header.Set("Content-Encoding", wirecodec.EncodingZstd)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || called {
		t.Fatalf("GET with an envelope = %d (handler called %v), want 400 before the handler", w.Code, called)
	}
}

// The router-wide body cap does not cover every route, so the envelope reads at most
// what it would accept once decoded.
func TestConfigEnvelope_CapsTheRawBody(t *testing.T) {
	called := false
	engine := envelopeTestEngine(t, func() { called = true })
	body := bytes.Repeat([]byte("A"), maxDecodedConfigBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(body))
	req.Header.Set(wirecodec.HashHeader, wirecodec.Sha256Hex(body))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge || called {
		t.Fatalf("oversized envelope body = %d (handler called %v), want 413", w.Code, called)
	}
}
