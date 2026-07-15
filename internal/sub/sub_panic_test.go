package sub

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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

// The JSON subscription generator for a Hysteria inbound whose StreamSettings
// omit the hysteriaSettings key must not panic (which would 500 the whole JSON
// subscription); the raw generator already tolerates this shape.
func TestGetJsonToleratesHysteriaWithoutHysteriaSettings(t *testing.T) {
	seedSubDB(t)
	db := database.GetDB()

	const subId = "hy1"
	const email = "hy@e"
	ib := &model.Inbound{
		UserId: 1, Tag: "hy", Enable: true, Listen: "203.0.113.5", Port: 46200,
		Protocol:       model.Hysteria,
		Remark:         "hy",
		Settings:       fmt.Sprintf(`{"version":2,"clients":[{"auth":"hyauth","email":%q,"subId":%q,"enable":true}]}`, email, subId),
		StreamSettings: `{"security":"tls","tlsSettings":{"serverName":"hy.sni"}}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := &model.ClientRecord{Email: email, SubID: subId, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound: %v", err)
	}

	jsonService := NewSubJsonService("", "", "", NewSubService(""))
	out, _, err := jsonService.GetJson(subId, "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if out == "" {
		t.Fatal("GetJson returned empty for a hysteria inbound without hysteriaSettings")
	}
}
