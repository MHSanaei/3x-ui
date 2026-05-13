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
