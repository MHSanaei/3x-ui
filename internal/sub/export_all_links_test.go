package sub

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// inboundLinks (the "Export all inbound links" path) must render the remark
// template's whole Client token group per client, name-only — the same engine
// the client/QR pages use.
func TestInboundLinks_RemarkTemplateClientTokens(t *testing.T) {
	seedSubDB(t)
	db := database.GetDB()
	settings := `{"clients":[{"id":"11111111-2222-4333-8444-000000000001","email":"john@e","subId":"subABC","comment":"vip","tgId":777,"enable":true}],"decryption":"none"}`
	ib := &model.Inbound{
		UserId: 1, Tag: "t", Enable: true, Listen: "203.0.113.5", Port: 4431,
		Protocol: model.VLESS, Remark: "Germany", Settings: settings,
		StreamSettings: `{"network":"ws","security":"tls","wsSettings":{"path":"/","host":""},"tlsSettings":{"serverName":"sni"}}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	svc := NewSubService("{{INBOUND}}-{{EMAIL}}-{{COMMENT}}-{{SUB_ID}}-{{TELEGRAM_ID}}-{{SHORT_ID}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D")
	svc.PrepareForRequest("req.example.com")
	links := svc.inboundLinks(ib)

	if len(links) != 1 {
		t.Fatalf("links = %d, want 1: %v", len(links), links)
	}
	frag := links[0]
	for _, want := range []string{"Germany-john", "vip", "subABC", "777", "11111111"} {
		if !strings.Contains(frag, want) {
			t.Fatalf("remark missing client token %q: %s", want, frag)
		}
	}
	if strings.Contains(frag, "GB") || strings.ContainsRune(frag, '⏳') {
		t.Fatalf("display mode must drop the traffic/expiry segments: %s", frag)
	}
}
