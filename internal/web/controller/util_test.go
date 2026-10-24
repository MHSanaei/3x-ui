package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetRemoteIpIgnoresForwardedHeadersFromUntrustedRemote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "203.0.113.10:12345"
	c.Request.Header.Set("X-Real-IP", "198.51.100.9")
	c.Request.Header.Set("X-Forwarded-For", "198.51.100.8")

	if got := getRemoteIp(c); got != "203.0.113.10" {
		t.Fatalf("remote IP = %q, want request remote address", got)
	}
}

func TestGetRemoteIpHonorsForwardedHeadersFromTrustedLoopbackProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "127.0.0.1:12345"
	c.Request.Header.Set("X-Forwarded-For", "198.51.100.8, 127.0.0.1")

	if got := getRemoteIp(c); got != "198.51.100.8" {
		t.Fatalf("remote IP = %q, want forwarded client IP", got)
	}
}

// Caddy and HAProxy set X-Forwarded-For but pass the client's own X-Real-IP
// through, so a client could pick its login-limiter bucket per request.
func TestGetRemoteIpDistrustsRealIPThatContradictsForwardedFor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "127.0.0.1:12345"
	c.Request.Header.Set("X-Forwarded-For", "203.0.113.5")
	c.Request.Header.Set("X-Real-IP", "198.51.100.1")

	if got := getRemoteIp(c); got != "127.0.0.1" {
		t.Fatalf("remote IP = %q, want the proxy address when the forwarded headers disagree", got)
	}
}

// An appending proxy keeps whatever X-Forwarded-For the client sent in front of
// the address it saw, so only the hop next to the trusted proxies is real.
func TestGetRemoteIpReadsForwardedForFromTheProxyEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "127.0.0.1:12345"
	c.Request.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.5")

	if got := getRemoteIp(c); got != "203.0.113.5" {
		t.Fatalf("remote IP = %q, want the hop the trusted proxy appended", got)
	}
}

func TestResolveHostPrefersForwardedHostOverRealIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Host = "panel.example.com:2053"
	c.Request.RemoteAddr = "127.0.0.1:12345"
	c.Request.Header.Set("X-Forwarded-Host", "sub.example.net:443")
	c.Request.Header.Set("X-Real-IP", "198.51.100.7")

	if got := resolveHost(c); got != "sub.example.net" {
		t.Fatalf("resolveHost = %q, want X-Forwarded-Host", got)
	}
}

func TestResolveHostIgnoresRealIPFromTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Host = "panel.example.com:2053"
	c.Request.RemoteAddr = "127.0.0.1:12345"
	c.Request.Header.Set("X-Real-IP", "198.51.100.7")

	if got := resolveHost(c); got != "panel.example.com" {
		t.Fatalf("resolveHost = %q, want request host (not X-Real-IP)", got)
	}
}
