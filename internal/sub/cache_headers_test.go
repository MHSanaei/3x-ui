package sub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Subscription bodies carry live traffic/expiry and the Happ/Incy routing
// headers, so a cached copy serves stale data (e.g. old routing rules after an
// admin edit). Every body-serving branch must answer with no-store.
func TestSubscriptionBodiesAreNotCacheable(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "cache", 4482, 1, `{"network":"tcp","security":"none"}`)
	gin.SetMode(gin.TestMode)
	oldDistFS := distFS
	distFS = testDistFS
	t.Cleanup(func() { distFS = oldDistFS })
	router := newSubscriptionTestRouter(subscriptionTestRouterConfig{clashAutoDetect: true, jsonAutoDetect: true})

	tests := []struct {
		name string
		path string
		ua   string
	}{
		{name: "raw base64 body", path: "/sub/s1", ua: "v2rayNG/1.8.5"},
		{name: "auto-detected Clash", path: "/sub/s1", ua: "Clash-Verge/v2.4.2"},
		{name: "auto-detected JSON", path: "/sub/s1", ua: "Happ/2.0"},
		{name: "JSON profile", path: "/json/s1", ua: "Happ/2.0"},
		{name: "JSON raw view", path: "/json/s1?view=raw", ua: "curl/8.0"},
		{name: "Clash profile", path: "/clash/s1", ua: "curl/8.0"},
		{name: "Clash raw view", path: "/clash/s1?view=raw", ua: "curl/8.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://sub.example.com"+tt.path, nil)
			req.Header.Set("User-Agent", tt.ua)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
			}
			if cc := resp.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
				t.Fatalf("Cache-Control = %q, want a no-store directive", cc)
			}
		})
	}
}
