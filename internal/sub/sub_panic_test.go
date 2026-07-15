package sub

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
)

// A subscription is built by iterating every client's share link with no
// recover(), so any panic in the link generators 500s the whole subscription
// for every client. Valid-but-unusual stream settings (an empty Reality
// shortIds/serverNames array, a tcp-http header with no request, a grpc block
// missing its keys) must therefore produce a link, not a panic.
func TestGetSubsToleratesUnusualStreamSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name   string
		stream string
	}{
		{"reality empty arrays", `{"network":"tcp","security":"reality","realitySettings":{"serverNames":[],"shortIds":[],"settings":{"publicKey":"pk"}}}`},
		{"tcp http missing request", `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"http","response":{"headers":{}}}}}`},
		{"tcp http empty path", `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"http","request":{"path":[]}}}}`},
		{"grpc missing keys", `{"network":"grpc","security":"none","grpcSettings":{}}`},
		{"empty stream settings", `{}`},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seedSubDB(t)
			subId := fmt.Sprintf("s%d", i)
			seedSubInbound(t, subId, fmt.Sprintf("t%d", i), 46000+i, 1, tc.stream)

			links, _, _, _, err := NewSubService("").GetSubs(subId, "req.example.com")
			if err != nil {
				t.Fatalf("GetSubs errored: %v", err)
			}
			if len(links) != 1 {
				t.Fatalf("expected 1 share link, got %d", len(links))
			}
		})
	}
}
