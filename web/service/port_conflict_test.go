package service

import (
	"path/filepath"
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
	if err := database.InitDB(filepath.Join(dbDir, "3x-ui.db")); err != nil {
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

func intPtr(v int) *int { return &v }

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
		{"hysteria2 forced udp", model.Hysteria2, ``, ``, transportUDP},
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
// hysteria2 must be allowed to coexist on the same port.
func TestCheckPortConflict_TCPandUDPCoexistOnSamePort(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "vless-443-tcp", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}
	hyst2 := &model.Inbound{
		Tag:      "hyst2-443-udp",
		Listen:   "0.0.0.0",
		Port:     443,
		Protocol: model.Hysteria2,
	}
	exist, err := svc.checkPortConflict(hyst2, 0)
	if err != nil {
		t.Fatalf("checkPortConflict: %v", err)
	}
	if exist {
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
	if !exist {
		t.Fatalf("two tcp inbounds on the same port must still conflict")
	}
}

// two udp inbounds (e.g. hysteria2 vs wireguard) on the same port also
// conflict, since they fight for the same socket.
func TestCheckPortConflict_UDPCollidesWithUDP(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "hyst2-443", "0.0.0.0", 443, model.Hysteria2, ``, ``)

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
	if !exist {
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
	if exist, err := svc.checkPortConflict(tcpClash, 0); err != nil || !exist {
		t.Fatalf("tcp inbound should clash with shadowsocks tcp,udp; exist=%v err=%v", exist, err)
	}

	udpClash := &model.Inbound{
		Tag:      "hyst2-443",
		Listen:   "0.0.0.0",
		Port:     443,
		Protocol: model.Hysteria2,
	}
	if exist, err := svc.checkPortConflict(udpClash, 0); err != nil || !exist {
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
	if exist, err := svc.checkPortConflict(other, 0); err != nil || exist {
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
	if exist, err := svc.checkPortConflict(other, 0); err != nil || exist {
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
	if exist, err := svc.checkPortConflict(anyAddr, 0); err != nil || !exist {
		t.Fatalf("any-addr on same port+transport must conflict with specific; exist=%v err=%v", exist, err)
	}
}

// when the base "inbound-<port>" tag is already taken on a coexisting
// transport, generateInboundTag must disambiguate with a transport
// suffix so the unique-tag DB constraint stays satisfied.
func TestGenerateInboundTag_DisambiguatesByTransportOnSamePort(t *testing.T) {
	setupConflictDB(t)
	// existing tcp inbound owns "inbound-443".
	seedInboundConflict(t, "inbound-443", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}
	udp := &model.Inbound{
		Listen:   "0.0.0.0",
		Port:     443,
		Protocol: model.Hysteria2,
	}
	got, err := svc.generateInboundTag(udp, 0)
	if err != nil {
		t.Fatalf("generateInboundTag: %v", err)
	}
	if got != "inbound-443-udp" {
		t.Fatalf("expected disambiguated tag inbound-443-udp, got %q", got)
	}
}

// when the port is free, the historical "inbound-<port>" shape is kept
// so existing routing rules don't change shape on upgrade.
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
	if got != "inbound-8443" {
		t.Fatalf("expected inbound-8443, got %q", got)
	}
}

// updating an inbound on its own port must not flag its own tag as
// taken, that's what ignoreId is for.
func TestGenerateInboundTag_IgnoresSelfOnUpdate(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "inbound-443", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`)

	var existing model.Inbound
	if err := database.GetDB().Where("tag = ?", "inbound-443").First(&existing).Error; err != nil {
		t.Fatalf("read seeded row: %v", err)
	}

	svc := &InboundService{}
	got, err := svc.generateInboundTag(&existing, existing.Id)
	if err != nil {
		t.Fatalf("generateInboundTag: %v", err)
	}
	if got != "inbound-443" {
		t.Fatalf("self-update must keep base tag, got %q", got)
	}
}

// specific listen address gets the listen-prefixed shape and same
// disambiguation rules.
func TestGenerateInboundTag_SpecificListenSameDisambiguation(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "inbound-1.2.3.4:443", "1.2.3.4", 443, model.VLESS, `{"network":"tcp"}`, `{}`)

	svc := &InboundService{}
	udp := &model.Inbound{
		Listen:   "1.2.3.4",
		Port:     443,
		Protocol: model.Hysteria2,
	}
	got, err := svc.generateInboundTag(udp, 0)
	if err != nil {
		t.Fatalf("generateInboundTag: %v", err)
	}
	if got != "inbound-1.2.3.4:443-udp" {
		t.Fatalf("expected inbound-1.2.3.4:443-udp, got %q", got)
	}
}

// inbounds bound to different nodes run on different physical machines,
// so the same port + transport must be allowed across nodes. covers
// local-vs-remote, remote-A-vs-remote-B, and the still-clashing
// same-node case.
func TestCheckPortConflict_NodeScope(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflictNode(t, "local-443-tcp", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`, nil)
	seedInboundConflictNode(t, "node1-443-tcp", "0.0.0.0", 443, model.VLESS, `{"network":"tcp"}`, `{}`, intPtr(1))

	svc := &InboundService{}

	cases := []struct {
		name   string
		nodeID *int
		want   bool
	}{
		{"new local same port + tcp clashes with local", nil, true},
		{"new remote on different node from local is fine", intPtr(2), false},
		{"new remote on existing node 1 clashes", intPtr(1), true},
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
			if got != c.want {
				t.Fatalf("got conflict=%v, want %v", got, c.want)
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
	seedInboundConflictNode(t, "inbound-5000", "0.0.0.0", 5000, model.VLESS, `{"network":"tcp"}`, `{}`, nil)
	seedInboundConflictNode(t, "inbound-5000-udp", "0.0.0.0", 5000, model.Hysteria2, ``, ``, nil)

	svc := &InboundService{}
	pushed := &model.Inbound{
		Tag:            "inbound-5000-tcp",
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
	if got != "inbound-5000-tcp" {
		t.Fatalf("caller tag must be preserved when free, got %q", got)
	}
}

// when the caller leaves Tag empty (the local UI path) resolveInboundTag
// falls back to generateInboundTag, which keeps the historical
// "inbound-<port>" shape so existing routing rules don't change.
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
	if got != "inbound-8443" {
		t.Fatalf("expected generated inbound-8443, got %q", got)
	}
}

// when the caller's Tag collides (e.g. a node that was used standalone
// happens to already own the tag the central panel picked),
// resolveInboundTag falls back to generateInboundTag rather than
// failing — the inbound still lands, just under a slightly different
// tag that the central will pick up via the AddInbound response.
func TestResolveInboundTag_RegeneratesOnCollision(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflictNode(t, "inbound-5000-tcp", "0.0.0.0", 5000, model.VLESS, `{"network":"tcp"}`, `{}`, nil)

	svc := &InboundService{}
	pushed := &model.Inbound{
		Tag:            "inbound-5000-tcp",
		Listen:         "0.0.0.0",
		Port:           5000,
		Protocol:       model.Hysteria2,
		StreamSettings: ``,
		Settings:       ``,
	}
	got, err := svc.resolveInboundTag(pushed, 0)
	if err != nil {
		t.Fatalf("resolveInboundTag: %v", err)
	}
	if got == "inbound-5000-tcp" {
		t.Fatalf("colliding caller tag must be replaced, but resolver kept %q", got)
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
	if exist, err := svc.checkPortConflict(&existing, existing.Id); err != nil || exist {
		t.Fatalf("self-update must not be flagged as conflict; exist=%v err=%v", exist, err)
	}
}
