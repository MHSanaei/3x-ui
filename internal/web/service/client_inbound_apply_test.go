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
	used, err := svc.otherTunnelAllowedIPs(inboundSvc, wgInbound.Id, nil)
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

// TestOtherTunnelAllowedIPsExcludesSelfEmail is a regression test for a real
// bug in ClientService.Attach: attaching one identity to multiple
// WireGuard/AmneziaWG inbounds in the same call copies that identity's own
// stored AllowedIPs into every inbound it processes (by design -- the same
// person should get the same tunnel address on every protocol they use).
// Attach's loop calls addInboundClient once per inbound, and each of those
// calls independently computes otherTunnelAllowedIPs -- so by the second
// inbound in the loop, the first inbound's now-successful copy of the
// identity's own address looked like a cross-inbound collision against
// itself, and the attach failed with exactly the error a real user hit:
// "wireguard: allowedIPs entry 10.8.1.21/32 is already used by a client on
// inbound 'awg' (#10)". selfEmails must exclude this identity's own entries
// on sibling inbounds -- safe to do unconditionally because ClientRecord.Email
// is globally unique, so a same-email match can only ever be this identity,
// never a genuine different client.
func TestOtherTunnelAllowedIPsExcludesSelfEmail(t *testing.T) {
	setupConflictDB(t)
	// Both shared@id (to be excluded) and other@awg (a genuinely different
	// client, must still be reported) live on the SAME sibling inbound --
	// otherTunnelAllowedIPs already excludes the asking inbound entirely via
	// excludeID, so putting other@awg there instead would make it invisible
	// to the scan regardless of the selfEmails fix, proving nothing.
	seedInboundConflict(t, "awg-1", "0.0.0.0", 443, model.AmneziaWG, ``, `{"server":{"subnetIp":"10.8.1.0","subnetCidr":24},"clients":[{"email":"shared@id","allowedIPs":["10.8.1.21/32"]},{"email":"other@awg","allowedIPs":["10.8.1.5/32"]}]}`)
	seedInboundConflict(t, "wg-1", "0.0.0.0", 51820, model.WireGuard, ``, `{"clients":[]}`)

	var wgInbound model.Inbound
	if err := database.GetDB().Where("tag = ?", "wg-1").First(&wgInbound).Error; err != nil {
		t.Fatalf("read seeded wg row: %v", err)
	}

	svc := &ClientService{}
	inboundSvc := &InboundService{}
	used, err := svc.otherTunnelAllowedIPs(inboundSvc, wgInbound.Id, map[string]struct{}{"shared@id": {}})
	if err != nil {
		t.Fatalf("otherTunnelAllowedIPs: %v", err)
	}
	if _, stillThere := used["10.8.1.21/32"]; stillThere {
		t.Fatalf("shared@id's own address on the awg inbound must be excluded from used, got %v", used)
	}
	if _, ok := used["10.8.1.5/32"]; !ok {
		t.Fatalf("a genuinely different client's address must still be reported as used, got %v", used)
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
	used, err := svc.otherTunnelAllowedIPs(inboundSvc, wgInbound.Id, nil)
	if err != nil {
		t.Fatalf("otherTunnelAllowedIPs: %v", err)
	}
	if len(used) != 0 {
		t.Fatalf("expected no cross-inbound addresses with only one tunnel inbound present, got %v", used)
	}
}
