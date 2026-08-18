package sub

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
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
	settingService := service.SettingService{}
	stored, err := settingService.GetTrustedProxyCIDRs()
	if err != nil {
		t.Fatalf("read trustedProxyCIDRs back through SettingService: %v", err)
	}
	if stored != value {
		t.Fatalf("SettingService reads trustedProxyCIDRs as %q, want %q — the key this helper writes has drifted", stored, value)
	}
}

func TestResolveRequest_ForwardedHeaderTrust(t *testing.T) {
	tests := []struct {
		name             string
		stored           *string
		remoteAddr       string
		wantScheme       string
		wantHost         string
		wantHostWithPort string
		wantHostHeader   string
	}{
		{
			name:             "no stored row keeps trusting forwarded headers",
			stored:           nil,
			remoteAddr:       "203.0.113.9:51000",
			wantScheme:       "https",
			wantHost:         "sub.example.net",
			wantHostWithPort: "sub.example.net",
			wantHostHeader:   "sub.example.net",
		},
		{
			name:             "empty stored value keeps trusting forwarded headers",
			stored:           new(""),
			remoteAddr:       "203.0.113.9:51000",
			wantScheme:       "https",
			wantHost:         "sub.example.net",
			wantHostWithPort: "sub.example.net",
			wantHostHeader:   "sub.example.net",
		},
		{
			name:             "stored shipped default keeps trusting forwarded headers",
			stored:           new(service.DefaultTrustedProxyCIDRs),
			remoteAddr:       "203.0.113.9:51000",
			wantScheme:       "https",
			wantHost:         "sub.example.net",
			wantHostWithPort: "sub.example.net",
			wantHostHeader:   "sub.example.net",
		},
		{
			name:             "declared boundary ignores an origin outside it",
			stored:           new("10.0.0.0/8"),
			remoteAddr:       "203.0.113.9:51000",
			wantScheme:       "http",
			wantHost:         "panel.example.com",
			wantHostWithPort: "panel.example.com:2096",
			wantHostHeader:   "panel.example.com",
		},
		{
			name:             "declared boundary trusts an origin inside it",
			stored:           new("10.0.0.0/8"),
			remoteAddr:       "10.1.2.3:44000",
			wantScheme:       "https",
			wantHost:         "sub.example.net",
			wantHostWithPort: "sub.example.net",
			wantHostHeader:   "sub.example.net",
		},
		{
			name:             "declared boundary ignores an unparsable origin",
			stored:           new("10.0.0.0/8"),
			remoteAddr:       "not-an-ip",
			wantScheme:       "http",
			wantHost:         "panel.example.com",
			wantHostWithPort: "panel.example.com:2096",
			wantHostHeader:   "panel.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			initSubDB(t)
			if tc.stored != nil {
				setTrustedProxyCIDRs(t, *tc.stored)
			}
			s := &SubService{}

			c := requestFrom(t, tc.remoteAddr, map[string]string{
				"X-Forwarded-Host":  "sub.example.net",
				"X-Forwarded-Proto": "https",
			})
			scheme, host, hostWithPort, hostHeader := s.ResolveRequest(c)

			if scheme != tc.wantScheme {
				t.Errorf("scheme = %q, want %q", scheme, tc.wantScheme)
			}
			if host != tc.wantHost {
				t.Errorf("host = %q, want %q", host, tc.wantHost)
			}
			if hostWithPort != tc.wantHostWithPort {
				t.Errorf("hostWithPort = %q, want %q", hostWithPort, tc.wantHostWithPort)
			}
			if hostHeader != tc.wantHostHeader {
				t.Errorf("hostHeader = %q, want %q", hostHeader, tc.wantHostHeader)
			}
		})
	}
}

func TestResolveRequest_GatesRealIPFallback(t *testing.T) {
	initSubDB(t)
	setTrustedProxyCIDRs(t, "10.0.0.0/8")
	s := &SubService{}

	c := requestFrom(t, "203.0.113.9:51000", map[string]string{
		"X-Real-IP": "198.51.100.7",
	})
	_, host, _, hostHeader := s.ResolveRequest(c)

	if host != "panel.example.com" {
		t.Errorf("host = %q, want the request host — X-Real-IP from an untrusted origin must be ignored", host)
	}
	if hostHeader != "panel.example.com" {
		t.Errorf("hostHeader = %q, want the request host", hostHeader)
	}
}

func TestHasForwardedHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{name: "no forwarded headers", want: false},
		{name: "forwarded host", headers: map[string]string{"X-Forwarded-Host": "sub.example.net"}, want: true},
		{name: "forwarded proto", headers: map[string]string{"X-Forwarded-Proto": "https"}, want: true},
		{name: "real ip", headers: map[string]string{"X-Real-IP": "10.1.2.3"}, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasForwardedHeaders(requestFrom(t, "10.1.2.3:1234", tc.headers)); got != tc.want {
				t.Errorf("hasForwardedHeaders() = %v, want %v", got, tc.want)
			}
		})
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
		{name: "ipv4 mapped address inside cidr", remoteAddr: "[::ffff:10.1.2.3]:1234", cidrs: "10.0.0.0/8", want: true},
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
