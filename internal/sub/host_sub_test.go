package sub

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func seedSubDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

// seedSubInbound creates a VLESS inbound with one client wired into the
// normalized clients/client_inbounds tables so getInboundsBySubId resolves it.
func seedSubInbound(t *testing.T, subId, tag string, port, subSortIndex int, stream string) *model.Inbound {
	t.Helper()
	db := database.GetDB()
	uuid := "11111111-2222-4333-8444-" + fmt.Sprintf("%012d", port)
	email := tag + "@e"
	settings := fmt.Sprintf(`{"clients":[{"id":%q,"email":%q,"subId":%q,"enable":true}],"decryption":"none"}`, uuid, email, subId)
	ib := &model.Inbound{
		UserId: 1, Tag: tag, Enable: true, Listen: "203.0.113.5", Port: port,
		Protocol: model.VLESS, Remark: tag, Settings: settings, StreamSettings: stream,
		SubSortIndex: subSortIndex,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound %s: %v", tag, err)
	}
	client := &model.ClientRecord{Email: email, SubID: subId, UUID: uuid, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client %s: %v", email, err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound %s: %v", email, err)
	}
	return ib
}

func seedHost(t *testing.T, h *model.Host) *model.Host {
	t.Helper()
	if err := database.GetDB().Create(h).Error; err != nil {
		t.Fatalf("seed host: %v", err)
	}
	return h
}

const wsTLSStream = `{"network":"ws","security":"tls","wsSettings":{"path":"/base","host":"base.host"},"tlsSettings":{"serverName":"base.sni"}}`

// #1 — an inbound with no hosts renders identically to the legacy path: a single
// link from the inbound's own address. Mutation-checks the zero-hosts fallback.
func TestSub_ZeroHosts_IdenticalOutput(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "z", 4431, 1, `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`)
	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("links = %d, want 1", len(links))
	}
	if !strings.Contains(links[0], "203.0.113.5:4431") {
		t.Fatalf("zero-hosts link should use the inbound address: %s", links[0])
	}
	if strings.Contains(links[0], "\n") {
		t.Fatalf("zero-hosts must be a single link: %s", links[0])
	}
}

// #2 — N enabled hosts render N links, ordered by sort_order, each carrying its
// own address/port/sni and host-header/path override.
func TestSub_NHosts_EmitsNLinksOrdered(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "n", 4432, 1, wsTLSStream)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 8443, Security: "tls", Sni: "b.sni", HostHeader: "b.host", Path: "/b"})
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 2096, Security: "tls", Sni: "a.sni", HostHeader: "a.host", Path: "/a"})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	parts := strings.Split(strings.Join(links, "\n"), "\n")
	if len(parts) != 2 {
		t.Fatalf("want 2 host links, got %d: %v", len(parts), parts)
	}
	if !strings.Contains(parts[0], "a.cdn.com:2096") || !strings.Contains(parts[0], "sni=a.sni") ||
		!strings.Contains(parts[0], "host=a.host") || !strings.Contains(parts[0], "path=%2Fa") {
		t.Fatalf("host A link (sort_order 1) wrong: %s", parts[0])
	}
	if !strings.Contains(parts[1], "b.cdn.com:8443") || !strings.Contains(parts[1], "sni=b.sni") ||
		!strings.Contains(parts[1], "host=b.host") || !strings.Contains(parts[1], "path=%2Fb") {
		t.Fatalf("host B link (sort_order 2) wrong: %s", parts[1])
	}
}

// #3 — a disabled host is omitted; the inbound falls back to its legacy link.
func TestSub_DisabledHostSkipped(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "d", 4433, 1, wsTLSStream)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "OFF", Address: "off.cdn.com", Port: 8443, IsDisabled: true})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	if strings.Contains(joined, "off.cdn.com") {
		t.Fatalf("disabled host must not render: %s", joined)
	}
	if !strings.Contains(joined, "203.0.113.5:4433") {
		t.Fatalf("with only a disabled host, the inbound's own link should render: %s", joined)
	}
}

// #4 — when both hosts and a legacy externalProxy are set, hosts win and the
// externalProxy entry is ignored.
func TestSub_HostAndExternalProxy_Precedence(t *testing.T) {
	seedSubDB(t)
	stream := `{"network":"ws","security":"tls","wsSettings":{"path":"/base","host":"base.host"},"tlsSettings":{"serverName":"base.sni"},"externalProxy":[{"forceTls":"tls","dest":"legacy.cdn.com","port":7443,"remark":"L"}]}`
	ib := seedSubInbound(t, "s1", "p", 4434, 1, stream)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "H", Address: "host.cdn.com", Port: 8443, Security: "tls", Sni: "host.sni"})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	if !strings.Contains(joined, "host.cdn.com:8443") {
		t.Fatalf("host should win: %s", joined)
	}
	if strings.Contains(joined, "legacy.cdn.com") {
		t.Fatalf("externalProxy must be ignored when hosts exist: %s", joined)
	}
}

// #5 — hosts that share a remark but differ in address/port are NOT deduped:
// distinct hosts produce distinct links. Mutation-checks the (absent) dedup.
func TestSub_NHosts_NoDedup(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "dd", 4435, 1, wsTLSStream)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "SAME", Address: "one.cdn.com", Port: 8443, Security: "tls"})
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 2, Remark: "SAME", Address: "two.cdn.com", Port: 8443, Security: "tls"})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	parts := strings.Split(joined, "\n")
	if len(parts) != 2 {
		t.Fatalf("two distinct hosts must yield two links, got %d: %v", len(parts), parts)
	}
	if !strings.Contains(joined, "one.cdn.com") || !strings.Contains(joined, "two.cdn.com") {
		t.Fatalf("both distinct host addresses must appear: %s", joined)
	}
}

// #6 — host sort_order composes with inbound SubSortIndex: inbounds order by
// SubSortIndex, hosts within an inbound by sort_order.
func TestSub_HostSortComposesWithSubSortIndex(t *testing.T) {
	seedSubDB(t)
	// inbound "second" has a higher SubSortIndex so it must come after "first".
	ibFirst := seedSubInbound(t, "s1", "first", 4436, 1, wsTLSStream)
	ibSecond := seedSubInbound(t, "s1", "second", 4437, 2, wsTLSStream)
	seedHost(t, &model.Host{InboundId: ibSecond.Id, SortOrder: 1, Remark: "S", Address: "second-host.com", Port: 8443, Security: "tls"})
	seedHost(t, &model.Host{InboundId: ibFirst.Id, SortOrder: 1, Remark: "F", Address: "first-host.com", Port: 8443, Security: "tls"})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	firstAt := strings.Index(joined, "first-host.com")
	secondAt := strings.Index(joined, "second-host.com")
	if firstAt < 0 || secondAt < 0 {
		t.Fatalf("both inbound hosts should render: %s", joined)
	}
	if firstAt > secondAt {
		t.Fatalf("inbound order must follow SubSortIndex (first before second): %s", joined)
	}
}

// #7 — host overrides apply AFTER projectThroughFallbackMaster: the host's
// address/sni win over the projected master stream.
func TestSub_HostOverFallback(t *testing.T) {
	seedSubDB(t)
	db := database.GetDB()
	master := &model.Inbound{
		UserId: 1, Tag: "master", Enable: true, Listen: "203.0.113.9", Port: 9443,
		Protocol: model.VLESS, Remark: "master",
		Settings:       `{"clients":[],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"master.sni"}}`,
	}
	if err := db.Create(master).Error; err != nil {
		t.Fatalf("seed master: %v", err)
	}
	// child listens internal-only so projection triggers.
	child := seedSubInbound(t, "s1", "child", 4438, 1, `{"network":"tcp","security":"none"}`)
	child.Listen = "127.0.0.1"
	if err := db.Model(&model.Inbound{}).Where("id = ?", child.Id).Update("listen", "127.0.0.1").Error; err != nil {
		t.Fatalf("set child listen: %v", err)
	}
	if err := db.Create(&model.InboundFallback{MasterId: master.Id, ChildId: child.Id}).Error; err != nil {
		t.Fatalf("seed fallback: %v", err)
	}
	seedHost(t, &model.Host{InboundId: child.Id, SortOrder: 1, Remark: "H", Address: "host.cdn.com", Port: 8443, Security: "tls", Sni: "host.sni"})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	if !strings.Contains(joined, "host.cdn.com:8443") || !strings.Contains(joined, "sni=host.sni") {
		t.Fatalf("host override must win over fallback master: %s", joined)
	}
	if strings.Contains(joined, "203.0.113.9") || strings.Contains(joined, "sni=master.sni") {
		t.Fatalf("master endpoint/sni must be overridden by the host: %s", joined)
	}
}

// #8 — a client only gets hosts for inbounds it is actually on (the
// clients ⋈ client_inbounds ⋈ inbounds join), never arbitrary inbounds.
func TestSub_HostsResolveViaClientInbounds(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "mine", 4439, 1, wsTLSStream)           // client on s1
	other := seedSubInbound(t, "s2", "other", 4440, 1, wsTLSStream) // client on s2 only
	seedHost(t, &model.Host{InboundId: other.Id, SortOrder: 1, Remark: "X", Address: "other-host.com", Port: 8443, Security: "tls"})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	if strings.Contains(joined, "other-host.com") {
		t.Fatalf("host on an inbound the client is not on must not appear: %s", joined)
	}
}

// allowInsecure renders as allowInsecure=1 in the raw link and
// skip-cert-verify: true in the Clash proxy.
func TestSub_HostAllowInsecure(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "ai", 4450, 1, wsTLSStream)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 0, Remark: "AI", Address: "ai.cdn.com", Port: 8443, Security: "tls", AllowInsecure: true})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if !strings.Contains(strings.Join(links, "\n"), "allowInsecure=1") {
		t.Fatalf("raw link should carry allowInsecure=1: %s", strings.Join(links, "\n"))
	}

	clash := NewSubClashService(false, "", NewSubService(""))
	yaml, _, err := clash.GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if !strings.Contains(yaml, "skip-cert-verify: true") {
		t.Fatalf("clash proxy should carry skip-cert-verify: true:\n%s", yaml)
	}
}

// A host's Final Mask reaches the raw share link as the fm param, merged with
// any inbound-level mask (#5831).
func TestSub_HostFinalMask_RawLink(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "fmh", 4455, 1,
		`{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"},"finalmask":{"tcp":[{"type":"sudoku"}]}}`)
	seedHost(t, &model.Host{
		InboundId: ib.Id, SortOrder: 0, Remark: "FM", Address: "fm.cdn.com", Port: 8443, Security: "tls",
		FinalMask: `{"tcp":[{"type":"fragment"}]}`,
	})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	wantFm := "fm=" + url.QueryEscape(`{"tcp":[{"type":"sudoku"},{"type":"fragment"}]}`)
	if !strings.Contains(joined, wantFm) {
		t.Fatalf("raw link should merge the host Final Mask into fm.\n got: %s\nwant substring: %s", joined, wantFm)
	}
}

// A host's sockoptParams is injected into the JSON output stream (sockopt is
// stripped from the base stream, re-added per host).
func TestSub_HostSockoptJSON(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "so", 4460, 1,
		`{"network":"xhttp","security":"tls","xhttpSettings":{"path":"/x","mode":"auto"},"tlsSettings":{"serverName":"base.sni"}}`)
	seedHost(t, &model.Host{
		InboundId: ib.Id, SortOrder: 0, Remark: "SO", Address: "so.cdn.com", Port: 8443, Security: "tls",
		SockoptParams: `{"tcpFastOpen":true}`,
	})
	js := NewSubJsonService("", "", "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if !strings.Contains(out, "sockopt") || !strings.Contains(out, "tcpFastOpen") {
		t.Fatalf("json should include the host sockopt:\n%s", out)
	}
}

// A host's muxParams override the JSON outbound's mux.
func TestSub_HostMuxJSON(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "mx", 4470, 1, wsTLSStream)
	seedHost(t, &model.Host{
		InboundId: ib.Id, SortOrder: 0, Remark: "MX", Address: "mx.cdn.com", Port: 8443, Security: "tls",
		MuxParams: `{"enabled":true,"concurrency":8}`,
	})
	js := NewSubJsonService("", "", "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if !strings.Contains(out, "concurrency") {
		t.Fatalf("json should include the host mux override:\n%s", out)
	}
}

// A reality host overrides SNI + fingerprint while inheriting pbk/sid from the
// inbound (reality keys can't be host-supplied).
func TestSub_HostRealitySniOverride(t *testing.T) {
	seedSubDB(t)
	realityStream := `{"network":"tcp","security":"reality","tcpSettings":{"header":{"type":"none"}},"realitySettings":{"serverNames":["base.reality.com"],"shortIds":["abcd"],"settings":{"publicKey":"PBK","fingerprint":"chrome"}}}`
	ib := seedSubInbound(t, "s1", "rl", 4490, 1, realityStream)
	seedHost(t, &model.Host{
		InboundId: ib.Id, SortOrder: 0, Remark: "RL", Address: "rl.cdn.com", Port: 8443,
		Security: "reality", Sni: "host.reality.com", Fingerprint: "firefox",
	})
	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	if !strings.Contains(joined, "rl.cdn.com:8443") || !strings.Contains(joined, "security=reality") {
		t.Fatalf("reality host base wrong: %s", joined)
	}
	if !strings.Contains(joined, "sni=host.reality.com") || !strings.Contains(joined, "fp=firefox") {
		t.Fatalf("reality host sni/fp override not applied: %s", joined)
	}
	if strings.Contains(joined, "sni=base.reality.com") {
		t.Fatalf("base reality sni must be overridden: %s", joined)
	}
	if !strings.Contains(joined, "pbk=PBK") || !strings.Contains(joined, "sid=abcd") {
		t.Fatalf("reality pbk/sid must be inherited from the inbound: %s", joined)
	}
}

// #9 — ExcludeFromSubTypes is honored per format: a host excluded from clash is
// absent from GetClash but present in the raw GetSubs output.
func TestSub_ExcludeFromSubTypes(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "x", 4441, 1, wsTLSStream)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "H", Address: "clashless.cdn.com", Port: 8443, Security: "tls", ExcludeFromSubTypes: []string{"clash"}})

	sub := NewSubService("")
	links, _, _, _, err := sub.GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if !strings.Contains(strings.Join(links, "\n"), "clashless.cdn.com") {
		t.Fatalf("host not excluded from raw should appear in GetSubs")
	}

	clash := NewSubClashService(false, "", NewSubService(""))
	yaml, _, err := clash.GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if strings.Contains(yaml, "clashless.cdn.com") {
		t.Fatalf("host excluded from clash must not appear in GetClash:\n%s", yaml)
	}
}
