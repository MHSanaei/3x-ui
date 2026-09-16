package sub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// The panel setting must survive the controller wiring, not just the service
// API: sub.go passes it as a controller option.
func TestJsonEndpointServesPanelDnsServers(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4910, 1, dnsTestStream)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	NewSUBController(
		router.Group("/"),
		WithSUBJsonEnabled(true),
		WithSUBJsonAlwaysArray(true),
		WithSUBJsonDns(`["https://dns.google/dns-query", "tls://1.1.1.1"]`),
	)

	req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/json/s1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	docs := parseSubJsonDocs(t, resp.Body.String())
	if len(docs) != 1 {
		t.Fatalf("docs = %d, want 1", len(docs))
	}
	servers, _ := docDnsBlock(t, docs[0])["servers"].([]any)
	if len(servers) != 2 || servers[0] != "https://dns.google/dns-query" || servers[1] != "tls://1.1.1.1" {
		t.Fatalf("dns servers = %v", servers)
	}
	if _, hasTemplate := docDnsBlock(t, docs[0])["tag"]; hasTemplate {
		t.Fatalf("template dns keys leaked: %v", docDnsBlock(t, docs[0]))
	}
	if body := resp.Body.String(); strings.Contains(body, "8.8.8.8") {
		t.Fatalf("template resolver survived the override:\n%s", body)
	}
}
