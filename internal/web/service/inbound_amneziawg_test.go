package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestCheckForwardedPortsConflict_EmptySpecNoConflict(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	hit, err := svc.checkForwardedPortsConflict("")
	if err != nil || hit != "" {
		t.Fatalf("an empty spec must never conflict; got hit=%q err=%v", hit, err)
	}
}

func TestCheckForwardedPortsConflict_CollidesWithPanelPort(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	// getString falls back to defaultValueMap's "webPort": "2053" on a fresh
	// DB with no explicit setting row.
	hit, err := svc.checkForwardedPortsConflict("2053")
	if err != nil {
		t.Fatalf("checkForwardedPortsConflict: %v", err)
	}
	if !strings.Contains(hit, "panel") {
		t.Fatalf("expected a collision naming the panel's own port, got %q", hit)
	}
}

func TestCheckForwardedPortsConflict_CollidesWithEnabledInboundPort(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "vless-8080", "0.0.0.0", 8080, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}
	hit, err := svc.checkForwardedPortsConflict("8000-8100")
	if err != nil {
		t.Fatalf("checkForwardedPortsConflict: %v", err)
	}
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
	hit, err := svc.checkForwardedPortsConflict("8080")
	if err != nil || hit != "" {
		t.Fatalf("a disabled inbound's port must not be reserved; got hit=%q err=%v", hit, err)
	}
}

func TestCheckForwardedPortsConflict_NoCollisionWhenPortsDontOverlap(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "vless-8080", "0.0.0.0", 8080, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}
	hit, err := svc.checkForwardedPortsConflict("9000-9100")
	if err != nil || hit != "" {
		t.Fatalf("unrelated ports must not conflict; got hit=%q err=%v", hit, err)
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
