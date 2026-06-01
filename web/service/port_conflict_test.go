package service

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/op/go-logging"
)

// the panel logger is a process-wide singleton. init it once per test
// binary so a stray warning from gorm doesn't blow up on a nil logger.
var portConflictLoggerOnce sync.Once

// setupConflictDB wires a temp sqlite db so checkPortConflict can read
// real candidates. closes the db before t.TempDir cleans up so windows
// doesn't refuse to remove the file.
func setupConflictDB(t *testing.T) {
	t.Helper()
	portConflictLoggerOnce.Do(func() { xuilogger.InitLogger(logging.ERROR) })

	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := database.CloseDB(); err != nil {
			t.Logf("CloseDB warning: %v", err)
		}
	})
}

func seedInboundConflict(t *testing.T, tag, listen string, port int, protocol model.Protocol, streamSettings, settings string) {
	t.Helper()
	seedInboundConflictNode(t, tag, listen, port, protocol, streamSettings, settings, nil)
}

func seedInboundConflictNode(t *testing.T, tag, listen string, port int, protocol model.Protocol, streamSettings, settings string, nodeID *int) {
	t.Helper()
	in := &model.Inbound{
		Tag:            tag,
		Enable:         true,
		Listen:         listen,
		Port:           port,
		Protocol:       protocol,
		StreamSettings: streamSettings,
		Settings:       settings,
		NodeID:         nodeID,
	}
	if err := database.GetDB().Create(in).Error; err != nil {
		t.Fatalf("seed inbound %s: %v", tag, err)
	}
}

//go:fix inline
func intPtr(v int) *int { return new(v) }

func TestInboundTransports(t *testing.T) {
	cases := []struct {
		name           string
		protocol       model.Protocol
		streamSettings string
		settings       string
		want           transportBits
	}{
		{"vless default tcp", model.VLESS, `{"network":"tcp"}`, ``, transportTCP},
		{"vless ws (still tcp)", model.VLESS, `{"network":"ws"}`, ``, transportTCP},
		{"vless kcp is udp", model.VLESS, `{"network":"kcp"}`, ``, transportUDP},
		{"vless empty stream defaults to tcp", model.VLESS, ``, ``, transportTCP},
		{"vless garbage stream stays tcp", model.VLESS, `not json`, ``, transportTCP},

		{"vmess default tcp", model.VMESS, `{"network":"tcp"}`, ``, transportTCP},
		{"trojan grpc is tcp", model.Trojan, `{"network":"grpc"}`, ``, transportTCP},

		{"hysteria forced udp", model.Hysteria, `{"network":"tcp"}`, ``, transportUDP},
		{"wireguard forced udp", model.WireGuard, ``, ``, transportUDP},

		{"shadowsocks tcp,udp", model.Shadowsocks, ``, `{"network":"tcp,udp"}`, transportTCP | transportUDP},
		{"shadowsocks udp only", model.Shadowsocks, ``, `{"network":"udp"}`, transportUDP},
		{"shadowsocks tcp only", model.Shadowsocks, ``, `{"network":"tcp"}`, transportTCP},
		{"shadowsocks empty network falls back to streamSettings", model.Shadowsocks, `{"network":"tcp"}`, `{}`, transportTCP},

		{"mixed udp on", model.Mixed, `{"network":"tcp"}`, `{"udp":true}`, transportTCP | transportUDP},
		{"mixed udp off", model.Mixed, `{"network":"tcp"}`, `{"udp":false}`, transportTCP},
		{"mixed udp missing", model.Mixed, `{"network":"tcp"}`, `{}`, transportTCP},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := inboundTransports(c.protocol, c.streamSettings, c.settings)
			if got != c.want {
				t.Fatalf("got bits %#b, want %#b", got, c.want)
			}
		})
	}
}

func TestListenOverlaps(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"", "", true},
		{"0.0.0.0", "", true},
		{"0.0.0.0", "1.2.3.4", true},
		{"::", "1.2.3.4", true},
		{"::0", "fe80::1", true},
		{"1.2.3.4", "1.2.3.4", true},
		{"1.2.3.4", "5.6.7.8", false},
		{"1.2.3.4", "::1", false},
	}
	for _, c := range cases {
		if got := listenOverlaps(c.a, c.b); got != c.want {
			t.Errorf("listenOverlaps(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// the actual case from #4103: tcp/443 vless reality and udp/443
// hysteria must be allowed to coexist on the same port.
func TestCheckPortConflict_TCPandUDPCoexistOnSamePort(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "vless-443-tcp", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}
	hyst2 := &model.Inbound{
		Tag:      "hyst2-443-udp",
		Listen:   "0.0.0.0",
		Port:     443,
		Protocol: model.Hysteria,
	}
	exist, err := svc.checkPortConflict(hyst2, 0)
	if err != nil {
		t.Fatalf("checkPortConflict: %v", err)
	}
	if exist != nil {
		t.Fatalf("vless/tcp and hysteria2/udp on the same port must be allowed to coexist")
	}
}

// two tcp inbounds on the same port still conflict.
func TestCheckPortConflict_TCPCollidesWithTCP(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "vless-443-a", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}
	other := &model.Inbound{
		Tag:            "vless-443-b",
		Listen:         "0.0.0.0",
		Port:           443,
		Protocol:       model.Trojan,
		StreamSettings: `{"network":"ws"}`,
	}
	exist, err := svc.checkPortConflict(other, 0)
	if err != nil {
		t.Fatalf("checkPortConflict: %v", err)
	}
	if exist == nil {
		t.Fatalf("two tcp inbounds on the same port must still conflict")
	}
}

// two udp inbounds (e.g. hysteria2 vs wireguard) on the same port also
// conflict, since they fight for the same socket.
func TestCheckPortConflict_UDPCollidesWithUDP(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "hyst2-443", "0.0.0.0", 443, model.Hysteria, ``, ``)

	svc := &InboundService{}
	wg := &model.Inbound{
		Tag:      "wg-443",
		Listen:   "0.0.0.0",
		Port:     443,
		Protocol: model.WireGuard,
	}
	exist, err := svc.checkPortConflict(wg, 0)
	if err != nil {
		t.Fatalf("checkPortConflict: %v", err)
	}
	if exist == nil {
		t.Fatalf("two udp inbounds on the same port must conflict")
	}
}

// shadowsocks listening on tcp+udp eats the whole port for both
// transports, so neither a tcp nor a udp neighbour is allowed.
func TestCheckPortConflict_ShadowsocksDualListenBlocksBoth(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "ss-443-dual", "0.0.0.0", 443, model.Shadowsocks, ``, `{"network":"tcp,udp"}`)

	svc := &InboundService{}

	tcpClash := &model.Inbound{
		Tag:            "vless-443",
		Listen:         "0.0.0.0",
		Port:           443,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
	}
	if exist, err := svc.checkPortConflict(tcpClash, 0); err != nil || exist == nil {
		t.Fatalf("tcp inbound should clash with shadowsocks tcp,udp; exist=%v err=%v", exist, err)
	}

	udpClash := &model.Inbound{
		Tag:      "hyst2-443",
		Listen:   "0.0.0.0",
		Port:     443,
		Protocol: model.Hysteria,
	}
	if exist, err := svc.checkPortConflict(udpClash, 0); err != nil || exist == nil {
		t.Fatalf("udp inbound should clash with shadowsocks tcp,udp; exist=%v err=%v", exist, err)
	}
}

// different ports never conflict regardless of transport.
func TestCheckPortConflict_DifferentPortNeverConflicts(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "vless-443", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}
	other := &model.Inbound{
		Tag:            "vless-444",
		Listen:         "0.0.0.0",
		Port:           444,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
	}
	if exist, err := svc.checkPortConflict(other, 0); err != nil || exist != nil {
		t.Fatalf("different port must not conflict; exist=%v err=%v", exist, err)
	}
}

// specific listen addresses on the same port don't clash with each other,
// but do clash with any-address on the same port (preserved from the old
// check).
func TestCheckPortConflict_ListenOverlapPreserved(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "vless-1.2.3.4", "1.2.3.4", 443, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}

	// different specific address, same port + transport: no conflict.
	other := &model.Inbound{
		Tag:            "vless-5.6.7.8",
		Listen:         "5.6.7.8",
		Port:           443,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
	}
	if exist, err := svc.checkPortConflict(other, 0); err != nil || exist != nil {
		t.Fatalf("different specific listen must not conflict; exist=%v err=%v", exist, err)
	}

	// any-address vs specific on same transport: conflict (any-addr wins).
	anyAddr := &model.Inbound{
		Tag:            "vless-any",
		Listen:         "0.0.0.0",
		Port:           443,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
	}
	if exist, err := svc.checkPortConflict(anyAddr, 0); err != nil || exist == nil {
		t.Fatalf("any-addr on same port+transport must conflict with specific; exist=%v err=%v", exist, err)
	}
}

// even with a stale legacy tag owning "in-443", a new UDP-side
// inbound gets a fully qualified canonical tag and does not collide.
func TestGenerateInboundTag_DisambiguatesByTransportOnSamePort(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "in-443", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}
	udp := &model.Inbound{
		Listen:   "0.0.0.0",
		Port:     443,
		Protocol: model.Hysteria,
	}
	got, err := svc.generateInboundTag(udp, 0)
	if err != nil {
		t.Fatalf("generateInboundTag: %v", err)
	}
	if got != "in-443-udp" {
		t.Fatalf("expected in-443-udp, got %q", got)
	}
}

// when the port is free, the canonical tag carries the transport so
// tcp/8443 and udp/8443 get distinct tags out of the box.
func TestGenerateInboundTag_KeepsBaseTagWhenFree(t *testing.T) {
	setupConflictDB(t)

	svc := &InboundService{}
	in := &model.Inbound{
		Listen:   "0.0.0.0",
		Port:     8443,
		Protocol: model.VLESS,
	}
	got, err := svc.generateInboundTag(in, 0)
	if err != nil {
		t.Fatalf("generateInboundTag: %v", err)
	}
	if got != "in-8443-tcp" {
		t.Fatalf("expected in-8443-tcp, got %q", got)
	}
}

// updating an inbound on its own port must not flag its own tag as taken;
// that's what ignoreId is for.
func TestGenerateInboundTag_IgnoresSelfOnUpdate(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "in-443-tcp", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`)

	var existing model.Inbound
	if err := database.GetDB().Where("tag = ?", "in-443-tcp").First(&existing).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}

	svc := &InboundService{}
	got, err := svc.generateInboundTag(&existing, existing.Id)
	if err != nil {
		t.Fatalf("generateInboundTag: %v", err)
	}
	if got != "in-443-tcp" {
		t.Fatalf("self-update must keep base tag, got %q", got)
	}
}

// the listen address never appears in the tag; the transport suffix still
// keeps a udp inbound distinct from a tcp one on the same port.
func TestGenerateInboundTag_ListenIgnoredTransportDisambiguates(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "in-443-tcp", "1.2.3.4", 443, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}
	udp := &model.Inbound{
		Listen:   "1.2.3.4",
		Port:     443,
		Protocol: model.Hysteria,
	}
	got, err := svc.generateInboundTag(udp, 0)
	if err != nil {
		t.Fatalf("generateInboundTag: %v", err)
	}
	if got != "in-443-udp" {
		t.Fatalf("expected in-443-udp, got %q", got)
	}
}

// inbounds bound to different nodes run on different physical machines,
// so the same port + transport must be allowed across nodes. covers
// local-vs-remote, remote-A-vs-remote-B, and the still-clashing
// same-node case.
func TestCheckPortConflict_NodeScope(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflictNode(t, "local-443-tcp", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`, nil)
	seedInboundConflictNode(t, "node1-443-tcp", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`, new(1))

	svc := &InboundService{}

	cases := []struct {
		name   string
		nodeID *int
		want   bool
	}{
		{"new local same port + tcp clashes with local", nil, true},
		{"new remote on different node from local is fine", new(2), false},
		{"new remote on existing node 1 clashes", new(1), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			candidate := &model.Inbound{
				Listen:         "0.0.0.0",
				Port:           443,
				Protocol:       model.VLESS,
				StreamSettings: `{"network":"tcp"}`,
				NodeID:         c.nodeID,
			}
			got, err := svc.checkPortConflict(candidate, 0)
			if err != nil {
				t.Fatalf("checkPortConflict: %v", err)
			}
			if (got != nil) != c.want {
				t.Fatalf("got conflict=%v, want %v", got != nil, c.want)
			}
		})
	}
}

// when the caller passes an explicit non-empty Tag that doesn't collide,
// resolveInboundTag returns it verbatim. this is the cross-panel path:
// the central panel picks a tag, pushes the inbound to a node, and the
// node must keep that exact tag so the eventual traffic sync-back can
// match the row by tag. previously the node regenerated and the two
// panels diverged, causing a UNIQUE constraint failure on sync.
func TestResolveInboundTag_RespectsCallerTagWhenFree(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflictNode(t, "in-5000-tcp", "0.0.0.0", 5000, model.VLESS, `{"network":"tcp"}`, `{}`, nil)
	seedInboundConflictNode(t, "in-5000-udp", "0.0.0.0", 5000, model.Hysteria, ``, ``, nil)

	svc := &InboundService{}
	pushed := &model.Inbound{
		Tag:            "custom-pushed-tag",
		Listen:         "0.0.0.0",
		Port:           5000,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
		NodeID:         intPtr(1),
	}
	got, err := svc.resolveInboundTag(pushed, 0)
	if err != nil {
		t.Fatalf("resolveInboundTag: %v", err)
	}
	if got != "custom-pushed-tag" {
		t.Fatalf("caller tag must be preserved when free, got %q", got)
	}
}

// when the caller leaves Tag empty (the local UI path) resolveInboundTag
// falls back to generateInboundTag, which emits the canonical
// "in-<port>-<transport>" shape.
func TestResolveInboundTag_GeneratesWhenTagEmpty(t *testing.T) {
	setupConflictDB(t)

	svc := &InboundService{}
	in := &model.Inbound{
		Listen:   "0.0.0.0",
		Port:     8443,
		Protocol: model.VLESS,
	}
	got, err := svc.resolveInboundTag(in, 0)
	if err != nil {
		t.Fatalf("resolveInboundTag: %v", err)
	}
	if got != "in-8443-tcp" {
		t.Fatalf("expected generated in-8443-tcp, got %q", got)
	}
}

// when the caller's Tag collides (e.g. a node that was used standalone
// happens to already own the tag the central panel picked),
// resolveInboundTag falls back to generateInboundTag rather than
// failing — the inbound still lands, just under a slightly different
// tag that the central will pick up via the AddInbound response.
func TestResolveInboundTag_RegeneratesOnCollision(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflictNode(t, "in-5000-tcp", "0.0.0.0", 5000, model.VLESS, `{"network":"tcp"}`, `{}`, nil)

	svc := &InboundService{}
	pushed := &model.Inbound{
		Tag:            "in-5000-tcp",
		Listen:         "0.0.0.0",
		Port:           5000,
		Protocol:       model.Hysteria,
		StreamSettings: ``,
		Settings:       ``,
	}
	got, err := svc.resolveInboundTag(pushed, 0)
	if err != nil {
		t.Fatalf("resolveInboundTag: %v", err)
	}
	if got == "in-5000-tcp" {
		t.Fatalf("colliding caller tag must be replaced, but resolver kept %q", got)
	}
}

// inbounds bound to a remote node get the canonical tag prefixed with
// "n<id>-" so the same listen+port+transport can live on the central
// panel and on the node simultaneously without bumping the global
// UNIQUE(inbounds.tag) constraint.
func TestGenerateInboundTag_NodePrefix(t *testing.T) {
	setupConflictDB(t)

	svc := &InboundService{}
	in := &model.Inbound{
		Listen:   "0.0.0.0",
		Port:     443,
		Protocol: model.VLESS,
		NodeID:   intPtr(1),
	}
	got, err := svc.generateInboundTag(in, 0)
	if err != nil {
		t.Fatalf("generateInboundTag: %v", err)
	}
	if got != "n1-in-443-tcp" {
		t.Fatalf("expected n1-in-443-tcp, got %q", got)
	}
}

// a node-prefixed inbound shouldn't collide with a same-port local one:
// the prefix scopes the tag to that specific node.
func TestGenerateInboundTag_NodePrefixedDoesNotCollideWithLocal(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "in-443-tcp", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}
	in := &model.Inbound{
		Listen:   "0.0.0.0",
		Port:     443,
		Protocol: model.VLESS,
		NodeID:   intPtr(1),
	}
	got, err := svc.generateInboundTag(in, 0)
	if err != nil {
		t.Fatalf("generateInboundTag: %v", err)
	}
	if got != "n1-in-443-tcp" {
		t.Fatalf("expected n1-in-443-tcp, got %q", got)
	}
}

// updating an inbound must not see itself as a conflict, that's what
// ignoreId is for.
func TestCheckPortConflict_IgnoreSelfOnUpdate(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "vless-443", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`)

	var existing model.Inbound
	if err := database.GetDB().Where("tag = ?", "vless-443").First(&existing).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}

	svc := &InboundService{}
	if exist, err := svc.checkPortConflict(&existing, existing.Id); err != nil || exist != nil {
		t.Fatalf("self-update must not be flagged as conflict; exist=%v err=%v", exist, err)
	}
}

// streamSettings.network=quic rides on UDP at L4, so a QUIC inbound must
// conflict with a UDP-only neighbour (hysteria) on the same port but not
// with a TCP-only one. covers the gap left by the original kcp-only check.
func TestCheckPortConflict_QUICTreatedAsUDP(t *testing.T) {
	quic := &model.Inbound{
		Tag:            "vless-quic-443",
		Listen:         "0.0.0.0",
		Port:           443,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"quic"}`,
	}

	t.Run("conflicts with hysteria/udp", func(t *testing.T) {
		setupConflictDB(t)
		seedInboundConflict(t, "hyst-443", "0.0.0.0", 443, model.Hysteria, ``, ``)
		svc := &InboundService{}
		if exist, err := svc.checkPortConflict(quic, 0); err != nil || exist == nil {
			t.Fatalf("quic on same port as hysteria must conflict; exist=%v err=%v", exist, err)
		}
	})

	t.Run("coexists with vless/tcp", func(t *testing.T) {
		setupConflictDB(t)
		seedInboundConflict(t, "vless-tcp-443", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`)
		svc := &InboundService{}
		if exist, err := svc.checkPortConflict(quic, 0); err != nil || exist != nil {
			t.Fatalf("quic and tcp on same port must coexist; exist=%v err=%v", exist, err)
		}
	})
}

// tunnel (dokodemo-door) carries its L4 transport list in
// settings.allowedNetwork, not settings.network. verify the predicate
// picks the right field for each protocol.
func TestCheckPortConflict_TunnelAllowedNetwork(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "tunnel-udp-443", "0.0.0.0", 443, model.Tunnel, ``, `{"allowedNetwork":"udp"}`)

	svc := &InboundService{}

	// tcp inbound on same port should coexist with udp-only tunnel.
	tcpNeighbour := &model.Inbound{
		Tag:            "vless-443",
		Listen:         "0.0.0.0",
		Port:           443,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
	}
	if exist, err := svc.checkPortConflict(tcpNeighbour, 0); err != nil || exist != nil {
		t.Fatalf("tunnel/udp and vless/tcp on same port must coexist; exist=%v err=%v", exist, err)
	}

	// udp neighbour (hysteria) on same port must conflict.
	udpNeighbour := &model.Inbound{
		Tag:      "hyst-443",
		Listen:   "0.0.0.0",
		Port:     443,
		Protocol: model.Hysteria,
	}
	if exist, err := svc.checkPortConflict(udpNeighbour, 0); err != nil || exist == nil {
		t.Fatalf("tunnel/udp and hysteria on same port must conflict; exist=%v err=%v", exist, err)
	}
}

// the rich conflict detail surfaced to the user must name the offending
// inbound (by remark when available) and the shared L4 transport(s).
func TestCheckPortConflict_DetailMessage(t *testing.T) {
	setupConflictDB(t)
	seeded := &model.Inbound{
		Tag:            "vless-443",
		Remark:         "my-vless",
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           443,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
		Settings:       `{}`,
	}
	if err := database.GetDB().Create(seeded).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	svc := &InboundService{}
	candidate := &model.Inbound{
		Tag:            "trojan-443",
		Listen:         "0.0.0.0",
		Port:           443,
		Protocol:       model.Trojan,
		StreamSettings: `{"network":"ws"}`,
	}
	got, err := svc.checkPortConflict(candidate, 0)
	if err != nil || got == nil {
		t.Fatalf("expected conflict, got=%v err=%v", got, err)
	}
	msg := got.String()
	if !strings.Contains(msg, "my-vless") {
		t.Fatalf("message should mention the conflicting inbound's remark; got %q", msg)
	}
	if !strings.Contains(msg, "tcp") {
		t.Fatalf("message should mention the shared L4 transport; got %q", msg)
	}
	if !strings.Contains(msg, "443") {
		t.Fatalf("message should mention the port; got %q", msg)
	}
}

// isAutoGeneratedTag must recognise the tags generateInboundTag emits (so an
// edit that changes port/transport re-derives them) while leaving user-typed
// or cross-panel tags untouched.
func TestIsAutoGeneratedTag(t *testing.T) {
	tcp := transportTCP
	cases := []struct {
		name   string
		tag    string
		port   int
		nodeID *int
		bits   transportBits
		want   bool
	}{
		{"canonical", "in-443-tcp", 443, nil, tcp, true},
		{"canonical udp", "in-443-udp", 443, nil, transportUDP, true},
		{"dedup suffix", "in-443-tcp-2", 443, nil, tcp, true},
		{"node prefixed", "n1-in-443-tcp", 443, intPtr(1), tcp, true},
		{"legacy listen-scoped is now custom", "in-127.0.0.1:443-tcp", 443, nil, tcp, false},
		{"custom tag", "my-cool-tag", 443, nil, tcp, false},
		{"stale port", "in-443-tcp", 8443, nil, tcp, false},
		{"stale transport", "in-443-tcp", 443, nil, transportUDP, false},
		{"non-numeric suffix", "in-443-tcp-x", 443, nil, tcp, false},
		{"empty suffix", "in-443-tcp-", 443, nil, tcp, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isAutoGeneratedTag(c.tag, c.port, c.nodeID, c.bits); got != c.want {
				t.Fatalf("isAutoGeneratedTag(%q) = %v, want %v", c.tag, got, c.want)
			}
		})
	}
}
