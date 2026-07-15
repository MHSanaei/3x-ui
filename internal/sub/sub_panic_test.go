package sub

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

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
		{"ws missing wsSettings", `{"network":"ws","security":"none"}`},
		{"httpupgrade missing settings", `{"network":"httpupgrade","security":"none"}`},
		{"tls alpn non-string element", `{"network":"tcp","security":"tls","tlsSettings":{"alpn":[123]}}`},
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

func TestGetJsonToleratesNonStringRealityShortId(t *testing.T) {
	seedSubDB(t)
	stream := `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["sni.example.com"],"shortIds":[42],"settings":{"publicKey":"pk"}}}`
	seedSubInbound(t, "rlty1", "rlty", 46400, 1, stream)

	jsonService := NewSubJsonService("", "", "", NewSubService(""))
	out, _, err := jsonService.GetJson("rlty1", "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if out == "" {
		t.Fatal("GetJson returned empty for a reality inbound with a non-string shortId element")
	}
}

func TestGetClashEmitsPinnedCertSha256(t *testing.T) {
	seedSubDB(t)
	const pin = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	stream := `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"pin.sni","settings":{"pinnedPeerCertSha256":["` + pin + `"]}}}`
	seedSubInbound(t, "pin1", "pin", 46300, 1, stream)

	out, _, err := NewSubClashService(false, "", NewSubService("")).GetClash("pin1", "sub.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if !strings.Contains(out, "pin-sha256") {
		t.Fatalf("Clash proxy dropped the pinned cert sha256:\n%s", out)
	}
}

func TestJsonAndClashTolerateExternalProxyMissingPort(t *testing.T) {
	seedSubDB(t)
	stream := `{"network":"tcp","security":"none","externalProxy":[{"forceTls":"same","dest":"cdn.example.com"}]}`
	seedSubInbound(t, "extp1", "extp", 46500, 1, stream)

	jsonService := NewSubJsonService("", "", "", NewSubService(""))
	jsonOut, _, err := jsonService.GetJson("extp1", "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if jsonOut == "" {
		t.Fatal("GetJson returned empty for an externalProxy entry missing port")
	}

	clashOut, _, err := NewSubClashService(false, "", NewSubService("")).GetClash("extp1", "sub.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if clashOut == "" {
		t.Fatal("GetClash returned empty for an externalProxy entry missing port")
	}
}
