package sub

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoonSubscriptionUsesChrome147ForPostQuantumReality(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "loon-profile", "reality", 8443, 1, `{
        "network":"tcp","security":"reality",
        "realitySettings":{"serverNames":["backend.example.test"],"shortIds":["ab12cd"],
          "settings":{"publicKey":"TEST-public-key","fingerprint":"chrome"}}
    }`)
	seedSubInbound(t, "loon-profile", "tls", 9443, 2, `{"network":"tcp","security":"tls"}`)
	gin.SetMode(gin.TestMode)
	router := newSubscriptionTestRouter(subscriptionTestRouterConfig{clashAutoDetect: true})
	fetch := func(query, client string) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.test/sub/loon-profile"+query, nil)
		req.Header.Set("User-Agent", client)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("subscription response status = %d", rec.Code)
		}
		if rec.Header().Get("Subscription-Userinfo") == "" {
			t.Fatal("usage header was lost")
		}
		body, err := base64.StdEncoding.DecodeString(rec.Body.String())
		if err != nil {
			t.Fatalf("decode subscription: %v", err)
		}
		return string(body)
	}
	raw := fetch("", "Generic/1.0")
	if strings.Contains(raw, "tls-profile=") || !strings.Contains(raw, "fp=chrome") {
		t.Fatal("default raw VLESS subscription changed")
	}
	loon := fetch("?format=loon", "Mihomo/1.19")
	if !strings.HasPrefix(loon, "[Proxy]\n") || !strings.Contains(loon, "= VLESS,203.0.113.5,8443,") ||
		!strings.Contains(loon, `public-key="TEST-public-key"`) || !strings.Contains(loon, "short-id=ab12cd") ||
		!strings.Contains(loon, "sni=backend.example.test") || !strings.Contains(loon, "tls-profile=chrome147") ||
		!strings.Contains(loon, "vless://") {
		t.Fatalf("Loon subscription lacks the native REALITY node or the unchanged TLS entry: %s", loon)
	}
}
