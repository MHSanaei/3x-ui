package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
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
	hit := svc.checkForwardedPortsConflict(ctx, "8000-8100")
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
	if hit := svc.checkForwardedPortsConflict(ctx, "9000-9100"); hit != "" {
		t.Fatalf("unrelated ports must not conflict; got hit=%q", hit)
	}
}

// A port-forward spec matching a port used only by an inbound hosted on a
// DIFFERENT node must not conflict: that inbound's DNAT/listen socket lives
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

// inboundAmneziaWGServer is pure (no DB), so it needs neither setupConflictDB
// nor CGO/sqlite -- it can run in any Go environment.
func TestInboundAmneziaWGServer_RedactsPrivateKey(t *testing.T) {
	settings := `{"server":{"privateKey":"super-secret","publicKey":"pub","mtu":1420,"headerProtectionKey":"MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivXN18="},"clients":[]}`
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
	// Unlike the private key, the header-protection key is shared with every
	// client config, so the clients page must receive it.
	if got.HeaderProtectionKey != "MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivXN18=" {
		t.Fatalf("HeaderProtectionKey must NOT be redacted, got %q", got.HeaderProtectionKey)
	}
}

func TestNormalizeAmneziaWGSettings_GeneratesFull31Set(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	inbound := &model.Inbound{Protocol: model.AmneziaWG, Port: 51820, Settings: ""}
	if err := svc.normalizeAmneziaWGSettings(inbound); err != nil {
		t.Fatalf("normalize empty settings: %v", err)
	}

	var parsed amneziawg.InboundSettings
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil || parsed.Server == nil {
		t.Fatalf("normalized settings must carry a server block (err=%v): %s", err, inbound.Settings)
	}
	srv := parsed.Server

	key, err := base64.StdEncoding.DecodeString(srv.HeaderProtectionKey)
	if err != nil || len(key) != 32 {
		t.Fatalf("headerProtectionKey = %q, must be base64 of 32 bytes (err=%v)", srv.HeaderProtectionKey, err)
	}
	for field, v := range map[string]string{
		"contentPaddingAddition": srv.ContentPaddingAddition,
		"rekeyAfterTime":         srv.RekeyAfterTime,
		"rekeyTimeout":           srv.RekeyTimeout,
		"rejectAfterTime":        srv.RejectAfterTime,
		"keepaliveTimeout":       srv.KeepaliveTimeout,
		"maxHandshakeAttempts":   srv.MaxHandshakeAttempts,
		"i1":                     srv.I1,
	} {
		if v == "" {
			t.Errorf("fresh server block must fill %s", field)
		}
	}
	if !srv.RandomTrailers || !srv.DisableCookies {
		t.Errorf("fresh server block defaults RandomTrailers/DisableCookies on, got %v/%v", srv.RandomTrailers, srv.DisableCookies)
	}
	if srv.I2 != "" || srv.I3 != "" || srv.I4 != "" || srv.I5 != "" {
		t.Errorf("generated sets must leave I2-I5 empty, got %q/%q/%q/%q", srv.I2, srv.I3, srv.I4, srv.I5)
	}
}

func TestNormalizeAmneziaWGSettings_RejectsBad31Values(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	cases := []struct {
		name    string
		snippet string
	}{
		{"bad headerProtectionKey", `"headerProtectionKey":"short"`},
		{"zero rekeyTimeout", `"rekeyTimeout":"0"`},
		{"rekey overlapping reject", `"rekeyAfterTime":"100-200","rejectAfterTime":"150-300"`},
		{"control chars in i2", `"i2":"<r 64>\nPostUp = evil"`},
	}
	for _, c := range cases {
		inbound := &model.Inbound{
			Protocol: model.AmneziaWG,
			Port:     51820,
			Settings: `{"server":{"privateKey":"x","publicKey":"y","subnetIp":"10.8.1.0","subnetCidr":24,` + c.snippet + `},"clients":[]}`,
		}
		if err := svc.normalizeAmneziaWGSettings(inbound); err == nil {
			t.Errorf("%s must be rejected", c.name)
		}
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
