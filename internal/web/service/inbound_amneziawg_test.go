package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestCheckForwardedPortsConflict_EmptySpecNoConflict(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	ctx, err := svc.loadPortConflictContext()
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	if hit := svc.checkForwardedPortsConflict(ctx, ""); hit != "" {
		t.Fatalf("an empty spec must never conflict; got hit=%q", hit)
	}
}

func TestCheckForwardedPortsConflict_CollidesWithPanelPort(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	ctx, err := svc.loadPortConflictContext()
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	// getString falls back to defaultValueMap's "webPort": "2053" on a fresh
	// DB with no explicit setting row.
	hit := svc.checkForwardedPortsConflict(ctx, "2053")
	if !strings.Contains(hit, "panel") {
		t.Fatalf("expected a collision naming the panel's own port, got %q", hit)
	}
}

func TestCheckForwardedPortsConflict_CollidesWithEnabledInboundPort(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "vless-8080", "0.0.0.0", 8080, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}
	ctx, err := svc.loadPortConflictContext()
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	hit := svc.checkForwardedPortsConflict(ctx, "8060-8100")
	if !strings.Contains(hit, "vless-8080") {
		t.Fatalf("expected a collision naming the colliding inbound, got %q", hit)
	}
}

func TestCheckForwardedPortsConflict_IgnoresDisabledInboundPort(t *testing.T) {
	setupConflictDB(t)
	disabled := &model.Inbound{Tag: "vless-8080-off", Enable: false, Listen: "0.0.0.0", Port: 8080, Protocol: model.VLESS, StreamSettings: `{"network":"tcp"}`}
	if err := database.GetDB().Create(disabled).Error; err != nil {
		t.Fatalf("seed disabled inbound: %v", err)
	}

	svc := &InboundService{}
	ctx, err := svc.loadPortConflictContext()
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	if hit := svc.checkForwardedPortsConflict(ctx, "8080"); hit != "" {
		t.Fatalf("a disabled inbound's port must not be reserved; got hit=%q", hit)
	}
}

func TestCheckForwardedPortsConflict_NoCollisionWhenPortsDontOverlap(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "vless-8080", "0.0.0.0", 8080, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}
	ctx, err := svc.loadPortConflictContext()
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	if hit := svc.checkForwardedPortsConflict(ctx, "9000-9050"); hit != "" {
		t.Fatalf("unrelated ports must not conflict; got hit=%q", hit)
	}
}

// A port-forward spec matching a port used only by an inbound hosted on a
// DIFFERENT node must not conflict: that inbound's own listen socket lives
// on the node's own host, never on this panel's, so there is nothing here
// for the forwarded port to actually collide with. Mirrors
// TestCheckPortConflict_NodeScope's own reasoning for the general port-
// conflict check.
func TestCheckForwardedPortsConflict_IgnoresPortOnDifferentNode(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflictNode(t, "node1-8080", "0.0.0.0", 8080, model.VLESS, `{"network":"tcp"}`, `{}`, new(1))

	svc := &InboundService{}
	ctx, err := svc.loadPortConflictContext()
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	if hit := svc.checkForwardedPortsConflict(ctx, "8080"); hit != "" {
		t.Fatalf("a port used only on a different node must not conflict; got hit=%q", hit)
	}
}

func TestCheckForwardedPortsConflict_RejectsSpecOverCap(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	ctx, err := svc.loadPortConflictContext()
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	spec := fmt.Sprintf("20000-%d", 20000+amneziawg.MaxForwardedPorts)
	hit := svc.checkForwardedPortsConflict(ctx, spec)
	if !strings.Contains(hit, fmt.Sprintf("%d", amneziawg.MaxForwardedPorts)) {
		t.Fatalf("expected a collision naming the %d-port cap, got %q", amneziawg.MaxForwardedPorts, hit)
	}
}

// The SOCKS5 relay port an enabled AmneziaWG inbound gets (SOCKSPortForInbound)
// is a phantom, non-DB-row port -- ctx.inbounds alone can't see it, so
// checkForwardedPortsConflict must check it explicitly. Mirrors
// TestCheckPortConflict_AmneziawgnetSocksRelayBlockedLocal's reasoning for the
// opposite direction (a real inbound's own port colliding with the relay).
func TestCheckForwardedPortsConflict_CollidesWithAmneziawgnetSocksPort(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "awg-1", "0.0.0.0", 51820, model.AmneziaWG, ``, `{}`)

	var awgInbound model.Inbound
	if err := database.GetDB().Where("tag = ?", "awg-1").First(&awgInbound).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}
	relayPort := amneziawgnet.SOCKSPortForInbound(awgInbound.Id)

	svc := &InboundService{}
	ctx, err := svc.loadPortConflictContext()
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	hit := svc.checkForwardedPortsConflict(ctx, fmt.Sprintf("%d", relayPort))
	if !strings.Contains(hit, "SOCKS5") {
		t.Fatalf("expected a collision naming the AmneziaWG inbound's SOCKS5 relay port, got %q", hit)
	}
}

// inboundAmneziaWGServer is pure (no DB), so it needs neither setupConflictDB
// nor CGO/sqlite -- it can run in any Go environment.
func TestInboundAmneziaWGServer_RedactsPrivateKey(t *testing.T) {
	settings := `{"server":{"privateKey":"super-secret","publicKey":"pub","mtu":1420},"clients":[]}`
	got := inboundAmneziaWGServer(string(model.AmneziaWG), settings)
	if got == nil {
		t.Fatal("expected a non-nil server block")
	}
	if got.PrivateKey != "" {
		t.Fatalf("PrivateKey must be redacted, got %q", got.PrivateKey)
	}
	if got.PublicKey != "pub" || got.MTU != 1420 {
		t.Fatalf("non-secret fields must still come through unchanged, got %+v", got)
	}
}

func TestInboundAmneziaWGServer_NonAmneziaWGReturnsNil(t *testing.T) {
	if got := inboundAmneziaWGServer(string(model.VLESS), `{"server":{"privateKey":"x"}}`); got != nil {
		t.Fatalf("a non-AmneziaWG protocol must return nil, got %+v", got)
	}
}

func TestInboundAmneziaWGServer_MissingServerBlockReturnsNil(t *testing.T) {
	if got := inboundAmneziaWGServer(string(model.AmneziaWG), `{"clients":[]}`); got != nil {
		t.Fatalf("settings with no server block must return nil, got %+v", got)
	}
}
