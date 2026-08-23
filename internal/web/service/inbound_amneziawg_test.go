package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func TestCheckForwardedPortsConflict_EmptySpecNoConflict(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	ctx, err := svc.loadPortConflictContext(database.GetDB())
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
	ctx, err := svc.loadPortConflictContext(database.GetDB())
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
	ctx, err := svc.loadPortConflictContext(database.GetDB())
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	hit := svc.checkForwardedPortsConflict(ctx, "8075-8085")
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
	ctx, err := svc.loadPortConflictContext(database.GetDB())
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
	ctx, err := svc.loadPortConflictContext(database.GetDB())
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	if hit := svc.checkForwardedPortsConflict(ctx, "9075-9085"); hit != "" {
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
	ctx, err := svc.loadPortConflictContext(database.GetDB())
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
		{"line-wrapped headerProtectionKey", `"headerProtectionKey":"MCPfRGcDGotJ6Tcn\r\nIdDqsemj2cMIiGHnPUHM5ivXN18="`},
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

func TestNormalizeAmneziaWGSettings_CanonicalizesRangeValues(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	inbound := &model.Inbound{
		Protocol: model.AmneziaWG,
		Port:     51820,
		Settings: `{"server":{"privateKey":"x","publicKey":"y","subnetIp":"10.8.1.0","subnetCidr":24,` +
			`"rekeyAfterTime":"110 - 140","rejectAfterTime":"190-250","keepaliveTimeout":"   "},"clients":[]}`,
	}
	if err := svc.normalizeAmneziaWGSettings(inbound); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	var parsed amneziawg.InboundSettings
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil || parsed.Server == nil {
		t.Fatalf("re-parse normalized settings (err=%v): %s", err, inbound.Settings)
	}
	if parsed.Server.RekeyAfterTime != "110-140" {
		t.Errorf("rekeyAfterTime = %q, want canonical \"110-140\"", parsed.Server.RekeyAfterTime)
	}
	// A whitespace-only value must collapse to "feature off", not be stored
	// as a value the server emitter renders into an invalid blank line.
	if parsed.Server.KeepaliveTimeout != "" {
		t.Errorf("keepaliveTimeout = %q, want collapsed to empty", parsed.Server.KeepaliveTimeout)
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

// A newline inside a client's allowedIPs used to reach the rendered .conf,
// where a following "[Interface]\nPostUp = ..." runs as root the moment
// whoever applies that config (client app, or awg-quick directly) does so.
func TestNormalizeAmneziaWGSettings_RejectsInjectedClientAllowedIPs(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	inbound := &model.Inbound{
		Protocol: model.AmneziaWG,
		Port:     51820,
		Settings: `{"server":{"privateKey":"x","publicKey":"y","subnetIp":"10.8.1.0","subnetCidr":24},` +
			`"clients":[{"email":"a@x","enable":true,"publicKey":"pk",` +
			`"allowedIPs":["10.8.1.2/32\n[Interface]\nPostUp = touch /tmp/pwned"]}]}`,
	}
	err := svc.normalizeAmneziaWGSettings(inbound)
	if err == nil {
		t.Fatalf("an allowedIPs entry carrying a config-injection payload must be rejected; settings became:\n%s", inbound.Settings)
	}
	if !strings.Contains(err.Error(), "allowedIPs") {
		t.Errorf("error should name the offending field, got %q", err)
	}
}

func TestNormalizeAmneziaWGSettings_CanonicalizesClientAllowedIPs(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	inbound := &model.Inbound{
		Protocol: model.AmneziaWG,
		Port:     51820,
		Settings: `{"server":{"privateKey":"x","publicKey":"y","subnetIp":"10.8.1.0","subnetCidr":24},` +
			`"clients":[{"email":"a@x","enable":true,"publicKey":"pk","allowedIPs":[" 10.8.1.2 "]}]}`,
	}
	if err := svc.normalizeAmneziaWGSettings(inbound); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	var parsed amneziawg.InboundSettings
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil {
		t.Fatalf("re-parse normalized settings: %v", err)
	}
	if len(parsed.Clients) != 1 || len(parsed.Clients[0].AllowedIPs) != 1 || parsed.Clients[0].AllowedIPs[0] != "10.8.1.2/32" {
		t.Fatalf("allowedIPs = %v, want [\"10.8.1.2/32\"]", parsed.Clients)
	}
}

func TestGetAmneziaWGLogs_ClampsCountAndFiltersEvents(t *testing.T) {
	logger.InitLogger(logging.DEBUG)
	logger.Info("amneziawg: started interface awg1 for inbound 1")
	logger.Info("xray: unrelated line that must never show up here")
	logger.Warning("amneziawgnet: reconcile failed for inbound 2: handshake timeout")

	svc := &ServerService{}
	logs := svc.GetAmneziaWGLogs("not-a-number", "")
	if logs == nil {
		t.Fatal("GetAmneziaWGLogs must never return nil")
	}
	for _, line := range logs.Events {
		if !strings.Contains(strings.ToLower(line), "amneziawg") {
			t.Fatalf("non-AmneziaWG line leaked into the event list: %q", line)
		}
	}
	if len(logs.Events) < 2 {
		t.Fatalf("both AmneziaWG lines should be present, got %v", logs.Events)
	}

	// count caps the event list, so an operator asking for 1 gets 1.
	if one := svc.GetAmneziaWGLogs("1", ""); len(one.Events) != 1 {
		t.Fatalf("count=1 must cap the event list, got %d", len(one.Events))
	}
	// filter narrows further, case-insensitively.
	filtered := svc.GetAmneziaWGLogs("100", "RECONCILE")
	if len(filtered.Events) != 1 || !strings.Contains(filtered.Events[0], "reconcile") {
		t.Fatalf("filter must narrow to the matching line, got %v", filtered.Events)
	}
}

func TestCheckForwardedPortsConflict_RejectsSpecOverCap(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	ctx, err := svc.loadPortConflictContext(database.GetDB())
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	spec := fmt.Sprintf("20000-%d", 20000+amneziawg.MaxForwardedPorts)
	hit := svc.checkForwardedPortsConflict(ctx, spec)
	if !strings.Contains(hit, fmt.Sprintf("%d", amneziawg.MaxForwardedPorts)) {
		t.Fatalf("expected a collision naming the %d-port cap, got %q", amneziawg.MaxForwardedPorts, hit)
	}
}

// A spec covering exactly MaxForwardedPorts ports is AT the cap, not over
// it, and must be accepted -- ExpandForwardedPorts truncates there by
// design, so a naive len(...) >= cap comparison can't tell the two apart.
func TestCheckForwardedPortsConflict_AcceptsSpecExactlyAtCap(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}
	ctx, err := svc.loadPortConflictContext(database.GetDB())
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	spec := fmt.Sprintf("20000-%d", 20000+amneziawg.MaxForwardedPorts-1)
	if hit := svc.checkForwardedPortsConflict(ctx, spec); hit != "" {
		t.Fatalf("a spec covering exactly %d ports must be accepted, got collision %q", amneziawg.MaxForwardedPorts, hit)
	}
}

// The SOCKS5 relay port an enabled AmneziaWG inbound gets (SOCKSPortForInbound)
// is a phantom, non-DB-row port -- ctx.inbounds alone can't see it, so
// checkForwardedPortsConflict must check it explicitly.
func TestCheckForwardedPortsConflict_CollidesWithAmneziawgnetSocksPort(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "awg-1", "0.0.0.0", 51820, model.AmneziaWG, ``, `{}`)

	var awgInbound model.Inbound
	if err := database.GetDB().Where("tag = ?", "awg-1").First(&awgInbound).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}
	relayPort := amneziawgnet.SOCKSPortForInbound(awgInbound.Id)

	svc := &InboundService{}
	ctx, err := svc.loadPortConflictContext(database.GetDB())
	if err != nil {
		t.Fatalf("loadPortConflictContext: %v", err)
	}
	hit := svc.checkForwardedPortsConflict(ctx, fmt.Sprintf("%d", relayPort))
	if !strings.Contains(hit, "SOCKS5") {
		t.Fatalf("expected a collision naming the AmneziaWG inbound's SOCKS5 relay port, got %q", hit)
	}
}
