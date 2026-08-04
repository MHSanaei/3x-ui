package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestHasTunnelAttachmentDetectsWireguardOrAmneziaWG backs the fix for a
// real production bug: Attach copies an identity's stored AllowedIPs into
// every inbound it processes (so the same person keeps the same tunnel
// address across protocols), but when an identity has been fully detached
// from every WireGuard/AmneziaWG inbound, that stored address is a leftover
// nothing reserves anymore -- reusing it can skip past address space that's
// genuinely free (a real user's own case: address .21 resurrected instead
// of the actually-free .3). hasTunnelAttachment is what Attach checks to
// decide whether to clear the stored address before its loop, so it needs
// to correctly tell "still has an active tunnel elsewhere" (preserve) apart
// from "no tunnel attachment at all" (clear, allocate fresh).
func TestHasTunnelAttachmentDetectsWireguardOrAmneziaWG(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "awg-1", "0.0.0.0", 443, model.AmneziaWG, ``, `{"server":{"subnetIp":"10.8.1.0","subnetCidr":24},"clients":[]}`)
	seedInboundConflict(t, "wg-1", "0.0.0.0", 51820, model.WireGuard, ``, `{"clients":[]}`)
	seedInboundConflict(t, "vless-1", "0.0.0.0", 8443, model.VLESS, `{"network":"tcp"}`, `{"clients":[]}`)

	var awgInbound, wgInbound, vlessInbound model.Inbound
	if err := database.GetDB().Where("tag = ?", "awg-1").First(&awgInbound).Error; err != nil {
		t.Fatalf("read seeded awg row: %v", err)
	}
	if err := database.GetDB().Where("tag = ?", "wg-1").First(&wgInbound).Error; err != nil {
		t.Fatalf("read seeded wg row: %v", err)
	}
	if err := database.GetDB().Where("tag = ?", "vless-1").First(&vlessInbound).Error; err != nil {
		t.Fatalf("read seeded vless row: %v", err)
	}

	s := &ClientService{}
	inboundSvc := &InboundService{}

	if s.hasTunnelAttachment(inboundSvc, nil) {
		t.Error("empty inboundIds must report no tunnel attachment")
	}
	if s.hasTunnelAttachment(inboundSvc, []int{vlessInbound.Id}) {
		t.Error("a VLESS-only attachment must not count as a tunnel attachment")
	}
	if s.hasTunnelAttachment(inboundSvc, []int{99999}) {
		t.Error("a nonexistent inbound id must not count as a tunnel attachment")
	}
	if !s.hasTunnelAttachment(inboundSvc, []int{vlessInbound.Id, wgInbound.Id}) {
		t.Error("a WireGuard inbound among others must count as a tunnel attachment")
	}
	if !s.hasTunnelAttachment(inboundSvc, []int{awgInbound.Id}) {
		t.Error("an AmneziaWG inbound must count as a tunnel attachment")
	}
}

// TestAddressesFitAmneziaWGInbound is a regression test for a real
// production bug: hasTunnelAttachment only asked "does this identity have
// ANY tunnel attachment", not "is the address it would inherit actually
// valid for THIS inbound" -- so an identity whose stored address came from
// WireGuard's own fallback subnet (10.0.0.0/24, used when that inbound has
// no other clients to infer a base from) got that exact address silently
// carried over onto a second, AmneziaWG inbound configured for a completely
// different subnet (10.8.1.0/24). defaultAmneziaWGClients's already-set
// branch only checks for collisions, not subnet membership, so the mismatch
// was accepted with no error -- producing a peer that can never actually
// connect (an AmneziaWG address must fall inside the kernel interface's own
// configured subnet to be routable at all). addressesFitAmneziaWGInbound is
// the check Attach now runs per inbound before deciding whether to keep an
// inherited address or force a fresh allocation.
func TestAddressesFitAmneziaWGInbound(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "awg-1", "0.0.0.0", 443, model.AmneziaWG, ``, `{"server":{"subnetIp":"10.8.1.0","subnetCidr":24},"clients":[]}`)
	seedInboundConflict(t, "wg-1", "0.0.0.0", 51820, model.WireGuard, ``, `{"clients":[]}`)

	var awgInbound, wgInbound model.Inbound
	if err := database.GetDB().Where("tag = ?", "awg-1").First(&awgInbound).Error; err != nil {
		t.Fatalf("read seeded awg row: %v", err)
	}
	if err := database.GetDB().Where("tag = ?", "wg-1").First(&wgInbound).Error; err != nil {
		t.Fatalf("read seeded wg row: %v", err)
	}

	if !addressesFitAmneziaWGInbound(nil, &awgInbound) {
		t.Error("no addresses at all must trivially fit (Attach's own fresh-allocate path)")
	}
	if !addressesFitAmneziaWGInbound([]string{"10.0.0.2/32"}, &wgInbound) {
		t.Error("WireGuard has no strict subnet requirement -- must never be rejected here")
	}
	if addressesFitAmneziaWGInbound([]string{"10.0.0.2/32"}, &awgInbound) {
		t.Fatal("the real bug: a WireGuard-fallback-subnet address must NOT be accepted as fitting an AmneziaWG inbound configured for a different subnet")
	}
	if !addressesFitAmneziaWGInbound([]string{"10.8.1.21/32"}, &awgInbound) {
		t.Error("an address genuinely inside the awg inbound's own configured subnet must fit")
	}
}
