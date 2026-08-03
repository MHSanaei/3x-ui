package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// otherTunnelAllowedIPs must see across protocols (a WireGuard inbound's
// client address collides with an AmneziaWG one just as easily as two
// AmneziaWG inbounds would), must exclude the inbound doing the asking, and
// must ignore inbounds that aren't WireGuard/AmneziaWG entirely.
func TestOtherTunnelAllowedIPs(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "wg-1", "0.0.0.0", 51820, model.WireGuard, ``, `{"clients":[{"email":"a@wg","allowedIPs":["10.0.0.5/32"]}]}`)
	seedInboundConflict(t, "awg-1", "0.0.0.0", 443, model.AmneziaWG, ``, `{"server":{"subnetIp":"10.8.1.0","subnetCidr":24},"clients":[{"email":"b@awg","allowedIPs":["10.8.1.21/32"]}]}`)
	seedInboundConflict(t, "vless-1", "0.0.0.0", 8443, model.VLESS, `{"network":"tcp"}`, `{"clients":[{"email":"c@vless"}]}`)

	var wgInbound model.Inbound
	if err := database.GetDB().Where("tag = ?", "wg-1").First(&wgInbound).Error; err != nil {
		t.Fatalf("read seeded wg row: %v", err)
	}

	svc := &ClientService{}
	inboundSvc := &InboundService{}
	used, err := svc.otherTunnelAllowedIPs(inboundSvc, wgInbound.Id)
	if err != nil {
		t.Fatalf("otherTunnelAllowedIPs: %v", err)
	}
	if len(used) != 1 {
		t.Fatalf("expected exactly one cross-inbound address (self excluded, vless ignored), got %v", used)
	}
	label, ok := used["10.8.1.21/32"]
	if !ok {
		t.Fatalf("expected the awg inbound's address to be reported as used, got %v", used)
	}
	if label == "" {
		t.Fatal("expected a non-empty description of which inbound holds the address")
	}
}

func TestOtherTunnelAllowedIPsEmptyWhenNoSiblings(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "wg-1", "0.0.0.0", 51820, model.WireGuard, ``, `{"clients":[{"email":"a@wg","allowedIPs":["10.0.0.5/32"]}]}`)

	var wgInbound model.Inbound
	if err := database.GetDB().Where("tag = ?", "wg-1").First(&wgInbound).Error; err != nil {
		t.Fatalf("read seeded wg row: %v", err)
	}

	svc := &ClientService{}
	inboundSvc := &InboundService{}
	used, err := svc.otherTunnelAllowedIPs(inboundSvc, wgInbound.Id)
	if err != nil {
		t.Fatalf("otherTunnelAllowedIPs: %v", err)
	}
	if len(used) != 0 {
		t.Fatalf("expected no cross-inbound addresses with only one tunnel inbound present, got %v", used)
	}
}
