package sub

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func requestFrom(t *testing.T, remoteAddr string, headers map[string]string) *gin.Context {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/sub/abc", nil)
	req.Host = "panel.example.com:2096"
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	return c
}

func setTrustedProxyCIDRs(t *testing.T, value string) {
	t.Helper()
	if err := database.GetDB().Create(&model.Setting{Key: "trustedProxyCIDRs", Value: value}).Error; err != nil {
		t.Fatalf("set trustedProxyCIDRs: %v", err)
	}
}

// An install that never touched trustedProxyCIDRs must keep the legacy
// behavior exactly: the forwarded host still drives the generated URLs, no
// matter where the request came from. This is the compatibility guarantee for
// every deployment whose reverse proxy is not on the loopback default.
func TestResolveRequest_UnconfiguredInstallStillTrustsForwardedHost(t *testing.T) {
	initSubDB(t)
	s := &SubService{}

	c := requestFrom(t, "203.0.113.9:51000", map[string]string{
		"X-Forwarded-Host":  "sub.example.net",
		"X-Forwarded-Proto": "https",
	})
	scheme, host, _, _ := s.ResolveRequest(c)

	if host != "sub.example.net" {
		t.Fatalf("unconfigured install must keep trusting X-Forwarded-Host, got host %q", host)
	}
	if scheme != "https" {
		t.Fatalf("unconfigured install must keep trusting X-Forwarded-Proto, got scheme %q", scheme)
	}
}

// Once the operator declares a trust boundary, a request arriving from outside
// it can no longer point the generated subscription URLs at a host of its
// choosing; the request's own Host stands.
func TestResolveRequest_ConfiguredInstallIgnoresUntrustedForwardedHost(t *testing.T) {
	initSubDB(t)
	setTrustedProxyCIDRs(t, "10.0.0.0/8")
	s := &SubService{}

	c := requestFrom(t, "203.0.113.9:51000", map[string]string{
		"X-Forwarded-Host":  "attacker.example.net",
		"X-Forwarded-Proto": "https",
	})
	scheme, host, hostWithPort, hostHeader := s.ResolveRequest(c)

	if host != "panel.example.com" {
		t.Fatalf("forwarded host from an untrusted origin must be ignored, got %q", host)
	}
	if hostWithPort != "panel.example.com:2096" || hostHeader != "panel.example.com" {
		t.Fatalf("every forwarded-host read must be gated, got hostWithPort=%q hostHeader=%q", hostWithPort, hostHeader)
	}
	if scheme != "http" {
		t.Fatalf("forwarded proto from an untrusted origin must be ignored, got %q", scheme)
	}
}

// The declared boundary is honored in the affirmative direction too: a request
// that really does arrive through the configured proxy keeps working.
func TestResolveRequest_ConfiguredInstallTrustsProxyInsideTheBoundary(t *testing.T) {
	initSubDB(t)
	setTrustedProxyCIDRs(t, "10.0.0.0/8")
	s := &SubService{}

	c := requestFrom(t, "10.1.2.3:44000", map[string]string{
		"X-Forwarded-Host":  "sub.example.net",
		"X-Forwarded-Proto": "https",
	})
	scheme, host, _, _ := s.ResolveRequest(c)

	if host != "sub.example.net" {
		t.Fatalf("a request from inside the declared boundary must still be trusted, got %q", host)
	}
	if scheme != "https" {
		t.Fatalf("scheme from inside the declared boundary must be trusted, got %q", scheme)
	}
}

func TestRemoteAddrInCIDRs(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		cidrs      string
		want       bool
	}{
		{name: "inside cidr", remoteAddr: "10.1.2.3:1234", cidrs: "10.0.0.0/8", want: true},
		{name: "outside cidr", remoteAddr: "203.0.113.9:1234", cidrs: "10.0.0.0/8", want: false},
		{name: "bare address entry", remoteAddr: "192.168.1.5:80", cidrs: "192.168.1.5", want: true},
		{name: "ipv6 loopback", remoteAddr: "[::1]:8080", cidrs: "::1/128", want: true},
		{name: "no port", remoteAddr: "10.1.2.3", cidrs: "10.0.0.0/8", want: true},
		{name: "unparsable origin", remoteAddr: "not-an-ip", cidrs: "10.0.0.0/8", want: false},
		{name: "empty entries skipped", remoteAddr: "10.1.2.3:1", cidrs: " , 10.0.0.0/8 , ", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := remoteAddrInCIDRs(tc.remoteAddr, tc.cidrs); got != tc.want {
				t.Errorf("remoteAddrInCIDRs(%q, %q) = %v, want %v", tc.remoteAddr, tc.cidrs, got, tc.want)
			}
		})
	}
}
