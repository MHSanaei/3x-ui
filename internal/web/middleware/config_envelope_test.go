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
