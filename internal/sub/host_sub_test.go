package sub

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func seedSubDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))
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

	clash := NewSubClashService(false, "", false, NewSubService(""))
	yaml, _, err := clash.GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if !strings.Contains(yaml, "skip-cert-verify: true") {
		t.Fatalf("clash proxy should carry skip-cert-verify: true:\n%s", yaml)
	}
}

// A host's Host header and path reach the Clash and JSON renderers even when
// the inbound's own ws settings leave them empty (#5944).
func TestSub_HostHeaderReachesClashAndJson(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "hh", 4457, 1,
		`{"network":"ws","security":"tls","wsSettings":{"path":"/"},"tlsSettings":{"serverName":"base.sni"}}`)
	seedHost(t, &model.Host{
		InboundId: ib.Id, SortOrder: 0, Remark: "HH", Address: "hh.cdn.com", Port: 8443, Security: "tls",
		HostHeader: "cdn.example.com", Path: "/ws-path",
	})

	clash := NewSubClashService(false, "", false, NewSubService(""))
	yaml, _, err := clash.GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if !strings.Contains(yaml, "Host: cdn.example.com") {
		t.Fatalf("clash ws-opts should carry the host record's Host header:\n%s", yaml)
	}
	if !strings.Contains(yaml, "path: /ws-path") {
		t.Fatalf("clash ws-opts should carry the host record's path:\n%s", yaml)
	}

	js := NewSubJsonService("", "", "", "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if !strings.Contains(out, `"host": "cdn.example.com"`) && !strings.Contains(out, `"host":"cdn.example.com"`) {
		t.Fatalf("json wsSettings should carry the host record's Host header:\n%s", out)
	}
}

// A host's Final Mask reaches the raw share link as the fm param, merged with
// any inbound-level mask (#5831).
func TestSub_HostFinalMask_RawLink(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "fmh", 4455, 1,
		`{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"},"finalmask":{"tcp":[{"type":"sudoku"}]}}`)
	finalMask := `{"tcp":[{"type":"fragment","settings":{"packets":"tlshello","lengths":["5-10","10-15","15-20","20-25","25-30"],"delays":["10-20","5-20","5-25","15-25","10-30"],"maxSplit":"10-15"}}]}`
	seedHost(t, &model.Host{
		InboundId: ib.Id, SortOrder: 0, Remark: "FM", Address: "fm.cdn.com", Port: 8443, Security: "tls",
		FinalMask: finalMask,
	})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) == 0 {
		t.Fatal("GetSubs returned no links")
	}
	link, err := url.Parse(strings.Split(links[0], "\n")[0])
	if err != nil {
		t.Fatalf("parse raw link: %v", err)
	}
	var finalmask map[string]any
	if err := json.Unmarshal([]byte(link.Query().Get("fm")), &finalmask); err != nil {
		t.Fatalf("unmarshal fm query param: %v", err)
	}
	tcp, _ := finalmask["tcp"].([]any)
	if len(tcp) != 2 {
		t.Fatalf("tcp mask count = %d, want existing + host mask: %#v", len(tcp), finalmask)
	}
	fragment, _ := tcp[1].(map[string]any)
	settings, _ := fragment["settings"].(map[string]any)
	if settings["length"] != "25-30" || settings["delay"] != "10-30" {
		t.Fatalf("legacy ranges = (%v, %v), want last per-segment values", settings["length"], settings["delay"])
	}
	if len(settings["lengths"].([]any)) != 5 || len(settings["delays"].([]any)) != 5 {
		t.Fatalf("per-segment ranges changed: %#v", settings)
	}
}

func TestSub_HostFinalMaskJSONAddsLegacyFragmentRanges(t *testing.T) {
	seedSubDB(t)
	baseStream := `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"},"finalmask":{"tcp":[{"type":"sudoku","settings":{"password":"p"}}]}}`
	ib := seedSubInbound(t, "s1", "fmj", 4456, 1, baseStream)
	finalMask := `{"tcp":[{"type":"fragment","settings":{"packets":"tlshello","lengths":["5-10","10-15","15-20","20-25","25-30"],"delays":["10-20","5-20","5-25","15-25","10-30"],"maxSplit":"10-15"}}]}`
	host := seedHost(t, &model.Host{
		InboundId: ib.Id, SortOrder: 0, Remark: "FM", Address: "fm-json.cdn.com", Port: 8444, Security: "tls",
		FinalMask: finalMask,
	})

	globalFinalMask := `{"tcp":[{"type":"fragment","settings":{"packets":"tlshello","lengths":["31-40"],"delays":[]}}]}`
	out, _, err := NewSubJsonService("", "", globalFinalMask, "", NewSubService("")).GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(out), &config); err != nil {
		t.Fatalf("unmarshal JSON subscription: %v", err)
	}
	outbounds, _ := config["outbounds"].([]any)
	if len(outbounds) == 0 {
		t.Fatalf("JSON subscription has no outbounds: %s", out)
	}
	outbound, _ := outbounds[0].(map[string]any)
	stream, _ := outbound["streamSettings"].(map[string]any)
	finalmask, _ := stream["finalmask"].(map[string]any)
	tcp, _ := finalmask["tcp"].([]any)
	if len(tcp) != 3 {
		t.Fatalf("tcp mask count = %d, want base + global + host masks: %#v", len(tcp), finalmask)
	}
	globalFragment, _ := tcp[1].(map[string]any)
	globalSettings, _ := globalFragment["settings"].(map[string]any)
	if globalSettings["length"] != "31-40" {
		t.Fatalf("global legacy length = %v, want 31-40", globalSettings["length"])
	}
	if _, exists := globalSettings["delay"]; exists {
		t.Fatalf("empty global delays must not emit a fallback: %#v", globalSettings)
	}
	fragment, _ := tcp[2].(map[string]any)
	settings, _ := fragment["settings"].(map[string]any)
	if settings["length"] != "25-30" || settings["delay"] != "10-30" {
		t.Fatalf("legacy ranges = (%v, %v), want the final per-segment ranges", settings["length"], settings["delay"])
	}
	if got := settings["lengths"].([]any); len(got) != 5 || got[4] != "25-30" {
		t.Fatalf("per-segment lengths changed: %#v", settings["lengths"])
	}
	if got := settings["delays"].([]any); len(got) != 5 || got[4] != "10-30" {
		t.Fatalf("per-segment delays changed: %#v", settings["delays"])
	}
	if got := settings["maxSplit"]; got != "10-15" {
		t.Fatalf("maxSplit = %v, want 10-15", got)
	}

	var storedHost model.Host
	if err := database.GetDB().First(&storedHost, host.Id).Error; err != nil {
		t.Fatalf("reload host: %v", err)
	}
	if storedHost.FinalMask != finalMask {
		t.Fatalf("stored host FinalMask changed: %s", storedHost.FinalMask)
	}
	var storedInbound model.Inbound
	if err := database.GetDB().First(&storedInbound, ib.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	if storedInbound.StreamSettings != baseStream {
		t.Fatalf("stored inbound StreamSettings changed: %s", storedInbound.StreamSettings)
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
	js := NewSubJsonService("", "", "", "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", false)
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
	js := NewSubJsonService("", "", "", "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", false)
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

// A reality host's SNI reaches JSON and Clash as serverName only: xray refuses a
// reality client that also carries serverNames (#6690).
func TestSub_HostRealitySniJSONAndClash(t *testing.T) {
	seedSubDB(t)
	realityStream := `{"network":"tcp","security":"reality","tcpSettings":{"header":{"type":"none"}},"realitySettings":{"serverNames":["base.reality.com"],"shortIds":["abcd"],"settings":{"publicKey":"PBK","fingerprint":"chrome"}}}`
	ib := seedSubInbound(t, "s1", "rlj", 4491, 1, realityStream)
	seedHost(t, &model.Host{
		InboundId: ib.Id, SortOrder: 0, Remark: "RLJ", Address: "rl.cdn.com", Port: 8443,
		Security: "reality", Sni: "host.reality.com",
	})

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("a single-config subscription should be one JSON object: %v\n%s", err, out)
	}
	reality := proxyRealitySettings(t, doc)
	if got := reality["serverName"]; got != "host.reality.com" {
		t.Fatalf("json serverName = %v, want the host's SNI host.reality.com", got)
	}
	if names, leaked := reality["serverNames"]; leaked {
		t.Fatalf("server-side serverNames %v leaked into the json reality client:\n%s", names, out)
	}

	yaml, _, err := NewSubClashService(false, "", false, NewSubService("")).GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if !strings.Contains(yaml, "servername: host.reality.com") {
		t.Fatalf("clash proxy should carry the host's SNI:\n%s", yaml)
	}
}

// A reality host's fingerprint must reach JSON and Clash like it reaches the raw
// link; otherwise one host gets fp=firefox in vless:// but chrome in Clash.
func TestSub_HostRealityFingerprintJSONAndClash(t *testing.T) {
	seedSubDB(t)
	realityStream := `{"network":"tcp","security":"reality","tcpSettings":{"header":{"type":"none"}},"realitySettings":{"serverNames":["base.reality.com"],"shortIds":["abcd"],"settings":{"publicKey":"PBK","fingerprint":"chrome"}}}`
	ib := seedSubInbound(t, "s1", "rlf", 4492, 1, realityStream)
	seedHost(t, &model.Host{
		InboundId: ib.Id, SortOrder: 0, Remark: "RLF", Address: "rl.cdn.com", Port: 8443,
		Security: "reality", Sni: "host.reality.com", Fingerprint: "firefox",
	})

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("a single-config subscription should be one JSON object: %v\n%s", err, out)
	}
	if got := proxyRealitySettings(t, doc)["fingerprint"]; got != "firefox" {
		t.Fatalf("json reality fingerprint = %v, want the host's firefox", got)
	}

	yaml, _, err := NewSubClashService(false, "", false, NewSubService("")).GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if !strings.Contains(yaml, "client-fingerprint: firefox") {
		t.Fatalf("clash proxy should carry the host's fingerprint:\n%s", yaml)
	}
}

// Hosts of one inbound render from per-host copies of the same stream; a host
// that leaves an override blank must get the inbound's value, not the previous host's.
func TestSub_HostBlankOverridesDoNotLeakBetweenHosts(t *testing.T) {
	for _, tc := range []struct {
		name      string
		stream    string
		first     model.Host
		clashWant map[string]string
		jsonField func(stream map[string]any) any
		jsonWant  any
	}{
		{
			name:      "reality fingerprint and sni",
			stream:    mihomoRealityStream,
			first:     model.Host{Security: "reality", Sni: "a.reality.com", Fingerprint: "firefox"},
			clashWant: map[string]string{"client-fingerprint": "chrome", "servername": "base.reality.com"},
			jsonField: func(stream map[string]any) any {
				rs, _ := stream["realitySettings"].(map[string]any)
				return rs["fingerprint"]
			},
			jsonWant: "chrome",
		},
		{
			name:      "ws host header and path",
			stream:    wsTLSStream,
			first:     model.Host{Security: "tls", HostHeader: "a.host", Path: "/a"},
			clashWant: map[string]string{"ws-opts.path": "/base", "ws-opts.headers.Host": "base.host"},
			jsonField: func(stream map[string]any) any {
				ws, _ := stream["wsSettings"].(map[string]any)
				return ws["path"]
			},
			jsonWant: "/base",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seedSubDB(t)
			ib := seedSubInbound(t, "s1", "lk", 4494, 1, tc.stream)
			first := tc.first
			first.InboundId, first.SortOrder, first.Remark, first.Address, first.Port = ib.Id, 0, "A", "a.cdn.com", 443
			seedHost(t, &first)
			seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "B", Address: "b.cdn.com", Port: 443, Security: tc.first.Security})

			out, _, err := NewSubClashService(false, "", false, NewSubService("")).GetClash("s1", "req.example.com")
			if err != nil {
				t.Fatalf("GetClash: %v", err)
			}
			second := clashProxyByServer(t, out, "b.cdn.com")
			for field, want := range tc.clashWant {
				if got := lookupPath(second, field); got != want {
					t.Errorf("host B clash %s = %v, want the inbound's %q", field, got, want)
				}
			}

			out, _, err = NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
			if err != nil {
				t.Fatalf("GetJson: %v", err)
			}
			var configs []map[string]any
			if err := json.Unmarshal([]byte(out), &configs); err != nil || len(configs) != 2 {
				t.Fatalf("want one JSON config per host, got err=%v:\n%s", err, out)
			}
			if got := tc.jsonField(proxyStreamSettings(t, configs[1])); got != tc.jsonWant {
				t.Errorf("host B json value = %v, want the inbound's %v", got, tc.jsonWant)
			}
		})
	}
}

func clashProxyByServer(t *testing.T, out, server string) map[string]any {
	t.Helper()
	var config struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(out), &config); err != nil {
		t.Fatalf("decode Clash YAML: %v\n%s", err, out)
	}
	for _, proxy := range config.Proxies {
		if proxy["server"] == server {
			return proxy
		}
	}
	t.Fatalf("no clash proxy for %s in:\n%s", server, out)
	return nil
}

func lookupPath(m map[string]any, path string) any {
	var cur any = m
	for _, key := range strings.Split(path, ".") {
		next, _ := cur.(map[string]any)
		cur = next[key]
	}
	return cur
}

func proxyStreamSettings(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	outbounds, _ := doc["outbounds"].([]any)
	for _, ob := range outbounds {
		outbound, _ := ob.(map[string]any)
		if outbound["tag"] == "proxy" {
			stream, _ := outbound["streamSettings"].(map[string]any)
			return stream
		}
	}
	t.Fatalf("no proxy outbound in %v", doc)
	return nil
}

const mihomoRealityStream = `{"network":"tcp","security":"reality","tcpSettings":{"header":{"type":"none"}},"realitySettings":{"serverNames":["base.reality.com"],"shortIds":["abcd"],"settings":{"publicKey":"PBK","fingerprint":"chrome"}}}`

// seedMihomoX25519Hosts puts a default host and a "Mihomo X25519" host on one
// reality inbound, so each test proves the toggle is per host, not per inbound.
func seedMihomoX25519Hosts(t *testing.T) {
	t.Helper()
	ib := seedSubInbound(t, "s1", "mx", 4493, 1, mihomoRealityStream)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 0, Remark: "PQ", Address: "pq.cdn.com", Port: 443})
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "CLASSIC", Address: "classic.cdn.com", Port: 443, MihomoX25519: true})
}

// Servers on Xray-core < 26.9.8 do not need ML-KEM, and a two-segment ML-KEM
// ClientHello is what some DPI drops; the host toggle must turn it off in Clash.
func TestSub_HostMihomoX25519Clash(t *testing.T) {
	seedSubDB(t)
	seedMihomoX25519Hosts(t)

	out, _, err := NewSubClashService(false, "", false, NewSubService("")).GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	var config struct {
		Proxies []struct {
			Server      string `yaml:"server"`
			RealityOpts struct {
				MLKEM *bool `yaml:"support-x25519mlkem768"`
			} `yaml:"reality-opts"`
		} `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(out), &config); err != nil {
		t.Fatalf("decode Clash YAML: %v\n%s", err, out)
	}
	want := map[string]bool{"pq.cdn.com": true, "classic.cdn.com": false}
	for _, proxy := range config.Proxies {
		mlkem, ok := want[proxy.Server]
		if !ok {
			continue
		}
		if got := proxy.RealityOpts.MLKEM; got == nil || *got != mlkem {
			t.Errorf("%s: support-x25519mlkem768 = %s, want %v\n%s", proxy.Server, fmtOptBool(got), mlkem, out)
		}
		delete(want, proxy.Server)
	}
	if len(want) != 0 {
		t.Fatalf("missing host proxies %v in:\n%s", want, out)
	}
}

func fmtOptBool(b *bool) string {
	if b == nil {
		return "absent"
	}
	return fmt.Sprint(*b)
}

// Mihomo also reads the ML-KEM hint from imported vless:// links (#6712), so a
// "Mihomo X25519" host must not carry it there either.
func TestSub_HostMihomoX25519RawLink(t *testing.T) {
	seedSubDB(t)
	seedMihomoX25519Hosts(t)

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	var pq, classic string
	for _, link := range strings.Split(strings.Join(links, "\n"), "\n") {
		switch {
		case strings.Contains(link, "@pq.cdn.com:443"):
			pq = link
		case strings.Contains(link, "@classic.cdn.com:443"):
			classic = link
		}
	}
	if !strings.Contains(pq, "support-x25519mlkem768=true") {
		t.Fatalf("default host must keep the ML-KEM hint: %q", pq)
	}
	if classic == "" || strings.Contains(classic, "support-x25519mlkem768") {
		t.Fatalf("Mihomo X25519 host must drop the ML-KEM hint: %q", classic)
	}
}

func proxyRealitySettings(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	outbounds, _ := doc["outbounds"].([]any)
	for _, ob := range outbounds {
		outbound, _ := ob.(map[string]any)
		if outbound["tag"] != "proxy" {
			continue
		}
		stream, _ := outbound["streamSettings"].(map[string]any)
		reality, ok := stream["realitySettings"].(map[string]any)
		if !ok {
			t.Fatalf("proxy outbound has no realitySettings: %v", outbound)
		}
		return reality
	}
	t.Fatalf("no proxy outbound in %v", doc)
	return nil
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

	clash := NewSubClashService(false, "", false, NewSubService(""))
	yaml, _, err := clash.GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if strings.Contains(yaml, "clashless.cdn.com") {
		t.Fatalf("host excluded from clash must not appear in GetClash:\n%s", yaml)
	}
}

// A host that forces plain TLS over a Reality inbound must not leave the
// Reality identity behind: pbk/sid/spx and the Reality dest sni describe a
// handshake the endpoint no longer performs.
func TestSub_HostTlsOverRealityDropsRealityParams(t *testing.T) {
	seedSubDB(t)
	reality := `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["master-dest.example.com"],"publicKey":"MASTERPBK","shortIds":["ab12"],"fingerprint":"chrome"}}`
	ib := seedSubInbound(t, "s1", "reality-in", 4461, 1, reality)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "H", Address: "edge.example.com", Port: 443, Security: "tls"})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	if !strings.Contains(joined, "security=tls") {
		t.Fatalf("host forces tls, link must say so: %s", joined)
	}
	for _, leaked := range []string{
		"pbk=",
		"sid=",
		"spx=",
		"support-x25519mlkem768=",
		"sni=master-dest.example.com",
	} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("reality parameter %q survived a tls host override: %s", leaked, joined)
		}
	}
}

// A host's cipher suites override the inbound's own in the JSON subscription,
// while a host that leaves the field blank inherits them.
func TestSub_HostCipherSuitesJSON(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "cs", 4462, 1,
		`{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni","cipherSuites":"TLS_CHACHA20_POLY1305_SHA256"}}`)
	seedHost(t, &model.Host{
		InboundId: ib.Id, SortOrder: 0, Remark: "CS", Address: "cs.cdn.com", Port: 8443, Security: "tls",
		CipherSuites: "TLS_AES_256_GCM_SHA384:TLS_AES_128_GCM_SHA256",
	})
	seedHost(t, &model.Host{
		InboundId: ib.Id, SortOrder: 1, Remark: "INHERIT", Address: "inh.cdn.com", Port: 8443, Security: "tls",
	})

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if !strings.Contains(out, `"cipherSuites": "TLS_AES_256_GCM_SHA384:TLS_AES_128_GCM_SHA256"`) &&
		!strings.Contains(out, `"cipherSuites":"TLS_AES_256_GCM_SHA384:TLS_AES_128_GCM_SHA256"`) {
		t.Fatalf("json tlsSettings should carry the host's cipher suites:\n%s", out)
	}
	if !strings.Contains(out, `"cipherSuites": "TLS_CHACHA20_POLY1305_SHA256"`) &&
		!strings.Contains(out, `"cipherSuites":"TLS_CHACHA20_POLY1305_SHA256"`) {
		t.Fatalf("a host with no cipher suites should inherit the inbound's:\n%s", out)
	}
}

// Xray reads a client's TLS verification fields at the top of tlsSettings; a
// nested "settings" map is panel-only shape and xray silently ignores it.
func TestSub_HostTLSVerificationJSONAtXrayLevel(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "ech", 4461, 1,
		`{"network":"xhttp","security":"tls","xhttpSettings":{"path":"/"},"tlsSettings":{"serverName":"base.sni","settings":{"fingerprint":"chrome"}}}`)
	seedHost(t, &model.Host{
		InboundId: ib.Id, SortOrder: 0, Remark: "ECH", Address: "ech.cdn.com", Port: 443, Security: "tls",
		EchConfigList: "cloudflare-ech.com+udp://1.1.1.1", VerifyPeerCertByName: "cert.example.com",
		PinnedPeerCertSha256: []string{"aa11", "bb22"}, AllowInsecure: true,
	})

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(out), &config); err != nil {
		t.Fatalf("unmarshal JSON subscription: %v", err)
	}
	outbounds, _ := config["outbounds"].([]any)
	if len(outbounds) == 0 {
		t.Fatalf("JSON subscription has no outbounds: %s", out)
	}
	outbound, _ := outbounds[0].(map[string]any)
	stream, _ := outbound["streamSettings"].(map[string]any)
	tls, _ := stream["tlsSettings"].(map[string]any)
	want := map[string]any{
		"serverName":           "base.sni",
		"fingerprint":          "chrome",
		"echConfigList":        "cloudflare-ech.com+udp://1.1.1.1",
		"verifyPeerCertByName": "cert.example.com",
		"pinnedPeerCertSha256": "aa11,bb22",
	}
	for key, value := range want {
		if tls[key] != value {
			t.Errorf("tlsSettings.%s = %#v, want %#v", key, tls[key], value)
		}
	}
	for _, key := range []string{"settings", "allowInsecure"} {
		if _, ok := tls[key]; ok {
			t.Errorf("tlsSettings.%s must not reach xray: %#v", key, tls)
		}
	}
}

func TestSub_HostCIDR_RawLinks(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "cidr-raw", 4462, 1,
		`{"network":"tcp","security":"tls","tlsSettings":{"serverName":"sni.example.com"}}`)
	seedHost(t, &model.Host{
		InboundId:   ib.Id,
		SortOrder:   0,
		Remark:      "IPv6Node",
		Address:     "2a01:4f8:1c1b:729::/64",
		Port:        8443,
		Security:    "tls",
		RandomCount: 3,
	})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	parts := strings.Split(strings.Join(links, "\n"), "\n")
	if len(parts) != 3 {
		t.Fatalf("len(parts) = %d, want 3", len(parts))
	}

	_, ipNet, _ := net.ParseCIDR("2a01:4f8:1c1b:729::/64")
	seenIPs := make(map[string]struct{}, len(parts))

	for i, link := range parts {
		wantRemark := fmt.Sprintf("IPv6Node%%20-%%20%d", i+1)
		if !strings.Contains(link, wantRemark) {
			t.Errorf("link[%d] does not contain expected remark %q: %s", i, wantRemark, link)
		}

		atIdx := strings.Index(link, "@")
		colonIdx := strings.LastIndex(link, ":8443")
		if atIdx == -1 || colonIdx == -1 || colonIdx <= atIdx {
			t.Fatalf("malformed link host: %s", link)
		}
		rawAddr := strings.Trim(link[atIdx+1:colonIdx], "[]")
		parsed := net.ParseIP(rawAddr)
		if parsed == nil || !ipNet.Contains(parsed) {
			t.Errorf("link[%d] IP %q not inside 2a01:4f8:1c1b:729::/64", i, rawAddr)
		}
		if _, dup := seenIPs[rawAddr]; dup {
			t.Errorf("duplicate IP %q emitted across links", rawAddr)
		}
		seenIPs[rawAddr] = struct{}{}
	}
}

func TestSub_HostCIDR_SingleCountKeepsBaseRemark(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "cidr-single", 4463, 1,
		`{"network":"tcp","security":"none"}`)
	seedHost(t, &model.Host{
		InboundId:   ib.Id,
		SortOrder:   0,
		Remark:      "SingleCIDR",
		Address:     "198.51.100.0/24",
		Port:        8080,
		Security:    "none",
		RandomCount: 1,
	})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	parts := strings.Split(strings.Join(links, "\n"), "\n")
	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
	if strings.Contains(parts[0], "SingleCIDR - 1") {
		t.Errorf("count=1 must not append index suffix: %s", parts[0])
	}
	if !strings.Contains(parts[0], "SingleCIDR") {
		t.Errorf("link does not contain base remark: %s", parts[0])
	}
}

func TestSub_HostCIDR_ClashAndJson(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "cidr-clash", 4464, 1,
		`{"network":"tcp","security":"none"}`)
	seedHost(t, &model.Host{
		InboundId:   ib.Id,
		SortOrder:   0,
		Remark:      "V4Node",
		Address:     "203.0.113.0/24",
		Port:        8080,
		Security:    "none",
		RandomCount: 2,
	})

	clashYaml, _, err := NewSubClashService(false, "", false, NewSubService("")).GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if !strings.Contains(clashYaml, "V4Node - 1") || !strings.Contains(clashYaml, "V4Node - 2") {
		t.Errorf("Clash YAML should contain numbered host proxies: %s", clashYaml)
	}

	jsonDoc, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if !strings.Contains(jsonDoc, "V4Node - 1") || !strings.Contains(jsonDoc, "V4Node - 2") {
		t.Errorf("JSON config should contain numbered host outbounds: %s", jsonDoc)
	}
}

// seedSubClient adds a second client sharing subId to an already-seeded
// inbound, so per-client host bindings can be compared within one sub.
func seedSubClient(t *testing.T, ib *model.Inbound, subId, email string) {
	t.Helper()
	db := database.GetDB()
	uuid := "22222222-3333-4444-8555-" + fmt.Sprintf("%012d", ib.Id)
	client := &model.ClientRecord{Email: email, SubID: subId, UUID: uuid, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client %s: %v", email, err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound %s: %v", email, err)
	}
}

const tcpTLSStream = `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`

// A bound host with blank address and port must use the inbound's own, even after an earlier
// client rewrote the shared inbound's Listen/Port while rendering its endpoints.
func TestSub_JsonBoundHostBlankAddressIgnoresEarlierClient(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "mv", 4491, 1, tcpTLSStream)
	// Host A has a real address and sorts last, so the unbound client (lower
	// id, rendered first) leaves the shared inbound pointing at a.cdn.com:443.
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls"})
	// Host B is override-only: blank address and port, so the bound client's
	// endpoint must inherit the inbound's own address and port.
	hostB := seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 0, Remark: "B", Security: "tls", Sni: "b.sni"})
	seedSubClient(t, ib, "s1", "bound@e")

	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", "bound@e").
		Update("client_host_rule_ids", model.EncodeClientHostRuleIds([]int{hostB.Id})).Error; err != nil {
		t.Fatalf("bind host rule: %v", err)
	}

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	type config struct {
		Remarks   string `json:"remarks"`
		Outbounds []struct {
			Tag      string `json:"tag"`
			Settings struct {
				Address string `json:"address"`
				Port    int    `json:"port"`
			} `json:"settings"`
		} `json:"outbounds"`
	}
	var docs []config
	if err := json.Unmarshal([]byte(out), &docs); err != nil {
		t.Fatalf("unmarshal json sub: %v\n%s", err, out)
	}
	var got struct {
		address string
		port    int
	}
	for _, doc := range docs {
		if !strings.Contains(doc.Remarks, "bound@e") {
			continue
		}
		for _, ob := range doc.Outbounds {
			if ob.Tag == "proxy" {
				got.address, got.port = ob.Settings.Address, ob.Settings.Port
			}
		}
	}
	if got.address == "" {
		t.Fatalf("no proxy outbound found for the bound client:\n%s", out)
	}
	if got.address != ib.Listen || got.port != ib.Port {
		t.Fatalf("override-only host resolved to %s:%d, want the inbound's own %s:%d",
			got.address, got.port, ib.Listen, ib.Port)
	}
}

// A client bound to one host renders only that host's link with its SNI,
// while an unbound client on the same inbound still fans out to every host.
func TestSub_ClientHostRule_SingleBinding(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "cb", 443, 1, tcpTLSStream)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls", Sni: "a.sni"})
	hostB := seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 443, Security: "tls", Sni: "b.sni"})
	seedSubClient(t, ib, "s1", "other@e")

	db := database.GetDB()
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "cb@e").
		Update("client_host_rule_ids", model.EncodeClientHostRuleIds([]int{hostB.Id})).Error; err != nil {
		t.Fatalf("bind host rule: %v", err)
	}

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	parts := strings.Split(strings.Join(links, "\n"), "\n")
	if len(parts) != 3 {
		t.Fatalf("bound client renders 1 link, unbound renders 2, got %d: %v", len(parts), parts)
	}
	bound := 0
	for _, l := range parts {
		if strings.Contains(l, "b.cdn.com:443") {
			bound++
			if !strings.Contains(l, "sni=b.sni") {
				t.Fatalf("bound link must carry the bound host SNI: %s", l)
			}
		}
	}
	if bound != 2 {
		t.Fatalf("want 2 links via b.cdn.com (1 bound + 1 unbound fan-out), got %d: %v", bound, parts)
	}
	aLinks := 0
	for _, l := range parts {
		if strings.Contains(l, "a.cdn.com:443") {
			aLinks++
			if strings.Contains(l, "sni=b.sni") {
				t.Fatalf("unbound host-A link must not carry host-B SNI: %s", l)
			}
		}
	}
	if aLinks != 1 {
		t.Fatalf("want exactly 1 link via a.cdn.com (unbound fan-out), got %d: %v", aLinks, parts)
	}
}

// A client bound to several host rules renders one link per bound host,
// and only those hosts — a third inbound host stays out of its links.
func TestSub_ClientHostRule_MultipleBindings(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "mb", 444, 1, tcpTLSStream)
	hostA := seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls", Sni: "a.sni"})
	hostB := seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 443, Security: "tls", Sni: "b.sni"})
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 3, Remark: "C", Address: "c.cdn.com", Port: 443, Security: "tls", Sni: "c.sni"})

	db := database.GetDB()
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "mb@e").
		Update("client_host_rule_ids", model.EncodeClientHostRuleIds([]int{hostA.Id, hostB.Id})).Error; err != nil {
		t.Fatalf("bind host rules: %v", err)
	}

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	parts := strings.Split(joined, "\n")
	if len(parts) != 2 {
		t.Fatalf("two bound hosts must yield two links, got %d: %v", len(parts), parts)
	}
	if !strings.Contains(joined, "sni=a.sni") || !strings.Contains(joined, "sni=b.sni") {
		t.Fatalf("each bound host SNI must appear: %s", joined)
	}
	if strings.Contains(joined, "c.cdn.com") {
		t.Fatalf("unbound host must not render for the bound client: %s", joined)
	}
}

// A deleted binding is skipped; a binding that resolves to no inbound fails closed with zero links
// (no fallback, no dummy link) but stays in traffic accounting.
func TestSub_ClientHostRule_DeletedFallsBack(t *testing.T) {
	t.Run("deleted and valid binding renders only the valid host", func(t *testing.T) {
		seedSubDB(t)
		ib := seedSubInbound(t, "s1", "fb", 445, 1, tcpTLSStream)
		seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls", Sni: "a.sni"})
		hostB := seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 443, Security: "tls", Sni: "b.sni"})

		db := database.GetDB()
		if err := db.Model(&model.ClientRecord{}).Where("email = ?", "fb@e").
			Update("client_host_rule_ids", model.EncodeClientHostRuleIds([]int{9999, hostB.Id})).Error; err != nil {
			t.Fatalf("bind host rules: %v", err)
		}

		links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
		if err != nil {
			t.Fatalf("GetSubs: %v", err)
		}
		parts := strings.Split(strings.Join(links, "\n"), "\n")
		if len(parts) != 1 {
			t.Fatalf("deleted id skipped, want only the valid host's link, got %d: %v", len(parts), parts)
		}
		if !strings.Contains(parts[0], "b.cdn.com:443") || !strings.Contains(parts[0], "sni=b.sni") {
			t.Fatalf("valid bound host must render with its SNI: %s", parts[0])
		}
		if strings.Contains(parts[0], "a.cdn.com") {
			t.Fatalf("unbound host must not render for the bound client: %s", parts[0])
		}
	})

	t.Run("binding that resolves to no inbound fails closed with zero links", func(t *testing.T) {
		seedSubDB(t)
		ib := seedSubInbound(t, "s1", "fb", 445, 1, tcpTLSStream)
		seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls", Sni: "a.sni"})
		seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 443, Security: "tls", Sni: "b.sni"})

		db := database.GetDB()
		if err := db.Model(&model.ClientRecord{}).Where("email = ?", "fb@e").
			Update("client_host_rule_ids", model.EncodeClientHostRuleIds([]int{9999})).Error; err != nil {
			t.Fatalf("bind host rule: %v", err)
		}
		if err := db.Create(&xray.ClientTraffic{InboundId: ib.Id, Email: "fb@e", Up: 100, Down: 200, Enable: true}).Error; err != nil {
			t.Fatalf("seed traffic: %v", err)
		}

		links, _, _, traffic, err := NewSubService("").GetSubs("s1", "req.example.com")
		if err != nil {
			t.Fatalf("GetSubs: %v", err)
		}
		if len(links) != 0 {
			t.Fatalf("deleted-only binding must emit zero links, got %d: %v", len(links), links)
		}
		joined := strings.Join(links, "\n")
		if strings.Contains(joined, "a.cdn.com") || strings.Contains(joined, "b.cdn.com") {
			t.Fatalf("fail-closed must not fall back to inbound hosts: %s", joined)
		}
		if strings.Contains(joined, "socks://") || strings.Contains(joined, "127.0.0.1") {
			t.Fatalf("fail-closed must not emit a dummy link: %s", joined)
		}
		if traffic.Up != 100 || traffic.Down != 200 {
			t.Fatalf("usage = up %d/down %d, want 100/200: closed client stays in accounting", traffic.Up, traffic.Down)
		}
		if !traffic.Enable {
			t.Fatalf("traffic.Enable = false, want true: closed client stays enabled")
		}
	})

	// An unknown host id must not silence an inbound without the client's other bound hosts: the
	// live and deleted hosts the ids resolve to decide which inbounds the binding guards.
	t.Run("deleted id on one inbound does not silence an unrelated inbound", func(t *testing.T) {
		seedSubDB(t)
		ibA := seedSubInbound(t, "s1", "xa", 4471, 1, tcpTLSStream)
		ibB := seedSubInbound(t, "s1", "xb", 4472, 2, tcpTLSStream)

		// Put the client on both inbounds under one email.
		db := database.GetDB()
		var client model.ClientRecord
		if err := db.Where("email = ?", "xb@e").First(&client).Error; err != nil {
			t.Fatalf("load client xb@e: %v", err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ibA.Id}).Error; err != nil {
			t.Fatalf("link client to inbound A: %v", err)
		}
		// Host A lives on ibA; bind it plus a deleted id 9999.
		hostA := seedHost(t, &model.Host{InboundId: ibA.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls", Sni: "a.sni"})
		if err := db.Model(&model.ClientRecord{}).Where("email = ?", "xb@e").
			Update("client_host_rule_ids", model.EncodeClientHostRuleIds([]int{hostA.Id, 9999})).Error; err != nil {
			t.Fatalf("bind host rules: %v", err)
		}

		links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
		if err != nil {
			t.Fatalf("GetSubs: %v", err)
		}
		joined := strings.Join(links, "\n")
		// Inbound B has no bound host and none of the ids map to it, so its
		// legacy fallback link must survive; inbound A still renders host A.
		if !strings.Contains(joined, "a.cdn.com:443") {
			t.Fatalf("inbound A's bound host must render: %s", joined)
		}
		if !strings.Contains(joined, fmt.Sprintf("203.0.113.5:%d", ibB.Port)) {
			t.Fatalf("deleted id must not silence inbound B's fallback link: %s", joined)
		}
	})
}

// Deleting a client's only bound host blanked it on every inbound, since the gone rows no longer
// said which inbound the binding guarded; only the inbound the host served may close.
func TestSub_ClientHostRule_DeletedHostClosesOnlyItsInbound(t *testing.T) {
	for _, tc := range []struct {
		name   string
		subId  string
		portA  int
		remove func(t *testing.T, ibA *model.Inbound, groupId string) error
	}{
		{"by its host group", "dg", 4481, func(_ *testing.T, _ *model.Inbound, groupId string) error {
			return (&service.HostService{}).DeleteHostGroup(groupId)
		}},
		{"with its inbound", "di", 4483, func(t *testing.T, ibA *model.Inbound, _ string) error {
			if err := database.GetDB().Model(ibA).Update("enable", false).Error; err != nil {
				t.Fatalf("disable inbound A: %v", err)
			}
			_, err := (&service.InboundService{}).DelInbound(ibA.Id)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seedSubDB(t)
			ibA := seedSubInbound(t, tc.subId, tc.subId+"a", tc.portA, 1, tcpTLSStream)
			ibB := seedSubInbound(t, tc.subId, tc.subId+"b", tc.portA+1, 2, tcpTLSStream)
			email, groupId := tc.subId+"b@e", tc.subId+"-grp"

			db := database.GetDB()
			var client model.ClientRecord
			if err := db.Where("email = ?", email).First(&client).Error; err != nil {
				t.Fatalf("load client %s: %v", email, err)
			}
			if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ibA.Id}).Error; err != nil {
				t.Fatalf("link client to inbound A: %v", err)
			}
			hostA := seedHost(t, &model.Host{GroupId: groupId, InboundId: ibA.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls", Sni: "a.sni"})
			if err := db.Model(&model.ClientRecord{}).Where("email = ?", email).
				Update("client_host_rule_ids", model.EncodeClientHostRuleIds([]int{hostA.Id})).Error; err != nil {
				t.Fatalf("bind host rule: %v", err)
			}
			if err := tc.remove(t, ibA, groupId); err != nil {
				t.Fatalf("delete the bound host: %v", err)
			}

			links, _, _, _, err := NewSubService("").GetSubs(tc.subId, "req.example.com")
			if err != nil {
				t.Fatalf("GetSubs: %v", err)
			}
			joined := strings.Join(links, "\n")
			uuid := "11111111-2222-4333-8444-" + fmt.Sprintf("%012d", ibB.Port)
			if strings.Contains(joined, "a.cdn.com") || strings.Contains(joined, fmt.Sprintf("%s@203.0.113.5:%d", uuid, ibA.Port)) {
				t.Fatalf("inbound A lost the client's bound host and must render nothing for it: %s", joined)
			}
			if !strings.Contains(joined, fmt.Sprintf("%s@203.0.113.5:%d", uuid, ibB.Port)) {
				t.Fatalf("inbound B never had a binding and must keep the client's link: %s", joined)
			}
		})
	}
}

// A client bound to a disabled host fails closed on every format (no links, outbounds or proxies)
// but stays in traffic accounting, so quota and expiry headers stay correct.
func TestSub_ClientHostRule_DisabledFailsClosed(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "dis", 447, 1, tcpTLSStream)
	host := seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "D", Address: "dis.cdn.com", Port: 443, Security: "tls", Sni: "dis.sni"})

	db := database.GetDB()
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "dis@e").
		Update("client_host_rule_ids", model.EncodeClientHostRuleIds([]int{host.Id})).Error; err != nil {
		t.Fatalf("bind host rules: %v", err)
	}
	if err := db.Model(&model.Host{}).Where("id = ?", host.Id).Update("is_disabled", true).Error; err != nil {
		t.Fatalf("disable host: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: ib.Id, Email: "dis@e", Up: 100, Down: 200, Enable: true}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	t.Run("raw", func(t *testing.T) {
		links, _, _, traffic, err := NewSubService("").GetSubs("s1", "req.example.com")
		if err != nil {
			t.Fatalf("GetSubs: %v", err)
		}
		if len(links) != 0 {
			t.Fatalf("disabled binding must emit zero links, got %d: %v", len(links), links)
		}
		joined := strings.Join(links, "\n")
		if strings.Contains(joined, "dis.cdn.com") || strings.Contains(joined, "203.0.113.5") {
			t.Fatalf("fail-closed must not fall back: %s", joined)
		}
		if strings.Contains(joined, "socks://") || strings.Contains(joined, "127.0.0.1") {
			t.Fatalf("fail-closed must not emit a dummy link: %s", joined)
		}
		if traffic.Up != 100 || traffic.Down != 200 {
			t.Fatalf("usage = up %d/down %d, want 100/200", traffic.Up, traffic.Down)
		}
	})

	t.Run("json", func(t *testing.T) {
		out, header, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
		if err != nil {
			t.Fatalf("GetJson: %v", err)
		}
		if strings.TrimSpace(out) != "" {
			t.Fatalf("disabled binding must emit zero outbounds, got:\n%s", out)
		}
		if !strings.HasPrefix(header, "upload=100; download=200; ") {
			t.Fatalf("header = %q, want usage prefix %q", header, "upload=100; download=200; ")
		}
		if strings.Contains(out, "dis.cdn.com") {
			t.Fatalf("fail-closed json must not contain the disabled host:\n%s", out)
		}
	})

	t.Run("clash", func(t *testing.T) {
		out, header, err := NewSubClashService(false, "", false, NewSubService("")).GetClash("s1", "req.example.com")
		if err != nil {
			t.Fatalf("GetClash: %v", err)
		}
		if strings.TrimSpace(out) != "" {
			t.Fatalf("disabled binding must emit zero proxies, got:\n%s", out)
		}
		if !strings.HasPrefix(header, "upload=100; download=200; ") {
			t.Fatalf("header = %q, want usage prefix %q", header, "upload=100; download=200; ")
		}
		if strings.Contains(out, "dis.cdn.com") {
			t.Fatalf("fail-closed clash must not contain the disabled host:\n%s", out)
		}
	})

	t.Run("panel links", func(t *testing.T) {
		if got := NewLinkProvider().LinksForClient("req.example.com", ib, "dis@e"); len(got) != 0 {
			t.Fatalf("LinksForClient must emit zero links, got %v", got)
		}
		svc := NewSubService("")
		svc.PrepareForRequest("req.example.com")
		if got := svc.inboundLinks(ib); len(got) != 0 {
			t.Fatalf("inboundLinks must emit zero links, got %v", got)
		}
	})
}

// A host excluded from one format fails closed only there: that format emits nothing but keeps
// accounting, while the other formats still render the host.
func TestSub_ClientHostRule_FormatExcludedFailsClosed(t *testing.T) {
	formats := []string{"raw", "json", "clash"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			seedSubDB(t)
			ib := seedSubInbound(t, "s1", "exc", 448, 1, tcpTLSStream)
			host := seedHost(t, &model.Host{
				InboundId: ib.Id, SortOrder: 1, Remark: "X", Address: "exc.cdn.com", Port: 443,
				Security: "tls", Sni: "exc.sni", ExcludeFromSubTypes: []string{format},
			})

			db := database.GetDB()
			if err := db.Model(&model.ClientRecord{}).Where("email = ?", "exc@e").
				Update("client_host_rule_ids", model.EncodeClientHostRuleIds([]int{host.Id})).Error; err != nil {
				t.Fatalf("bind host rules: %v", err)
			}
			if err := db.Create(&xray.ClientTraffic{InboundId: ib.Id, Email: "exc@e", Up: 300, Down: 400, Enable: true}).Error; err != nil {
				t.Fatalf("seed traffic: %v", err)
			}

			switch format {
			case "raw":
				links, _, _, traffic, err := NewSubService("").GetSubs("s1", "req.example.com")
				if err != nil {
					t.Fatalf("GetSubs: %v", err)
				}
				if len(links) != 0 {
					t.Fatalf("format-excluded binding must emit zero links, got %d: %v", len(links), links)
				}
				if traffic.Up != 300 || traffic.Down != 400 {
					t.Fatalf("usage = up %d/down %d, want 300/400", traffic.Up, traffic.Down)
				}
				out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
				if err != nil {
					t.Fatalf("GetJson: %v", err)
				}
				if !strings.Contains(out, "exc.cdn.com") {
					t.Fatalf("json (not excluded) must still render the host:\n%s", out)
				}
				yaml, _, err := NewSubClashService(false, "", false, NewSubService("")).GetClash("s1", "req.example.com")
				if err != nil {
					t.Fatalf("GetClash: %v", err)
				}
				if !strings.Contains(yaml, "exc.cdn.com") {
					t.Fatalf("clash (not excluded) must still render the host:\n%s", yaml)
				}
			case "json":
				out, header, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
				if err != nil {
					t.Fatalf("GetJson: %v", err)
				}
				if strings.TrimSpace(out) != "" {
					t.Fatalf("format-excluded binding must emit zero outbounds, got:\n%s", out)
				}
				if !strings.HasPrefix(header, "upload=300; download=400; ") {
					t.Fatalf("header = %q, want usage prefix", header)
				}
				links, _, _, traffic, err := NewSubService("").GetSubs("s1", "req.example.com")
				if err != nil {
					t.Fatalf("GetSubs: %v", err)
				}
				if len(links) != 1 || !strings.Contains(links[0], "exc.cdn.com") {
					t.Fatalf("raw (not excluded) must still render the host: %v", links)
				}
				if traffic.Up != 300 || traffic.Down != 400 {
					t.Fatalf("usage = up %d/down %d, want 300/400", traffic.Up, traffic.Down)
				}
			case "clash":
				yaml, header, err := NewSubClashService(false, "", false, NewSubService("")).GetClash("s1", "req.example.com")
				if err != nil {
					t.Fatalf("GetClash: %v", err)
				}
				if strings.TrimSpace(yaml) != "" {
					t.Fatalf("format-excluded binding must emit zero proxies, got:\n%s", yaml)
				}
				if !strings.HasPrefix(header, "upload=300; download=400; ") {
					t.Fatalf("header = %q, want usage prefix", header)
				}
				links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
				if err != nil {
					t.Fatalf("GetSubs: %v", err)
				}
				if len(links) != 1 || !strings.Contains(links[0], "exc.cdn.com") {
					t.Fatalf("raw (not excluded) must still render the host: %v", links)
				}
			}
		})
	}
}

// The bound host's SNI wins in the JSON subscription too, and unbound
// inbound hosts stay out of the bound client's configs.
func TestSub_ClientHostRule_JSONSNI(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "jb", 446, 1, tcpTLSStream)
	host := seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "J", Address: "j.cdn.com", Port: 443, Security: "tls", Sni: "bound.sni"})
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 2, Remark: "O", Address: "o.cdn.com", Port: 443, Security: "tls", Sni: "other.sni"})

	db := database.GetDB()
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "jb@e").
		Update("client_host_rule_ids", model.EncodeClientHostRuleIds([]int{host.Id})).Error; err != nil {
		t.Fatalf("bind host rules: %v", err)
	}

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if !strings.Contains(out, "bound.sni") {
		t.Fatalf("json config must carry the bound host SNI:\n%s", out)
	}
	if !strings.Contains(out, "j.cdn.com") {
		t.Fatalf("json config must dial the bound host address:\n%s", out)
	}
	if strings.Contains(out, "other.sni") || strings.Contains(out, "o.cdn.com") {
		t.Fatalf("json config must not contain the unbound host:\n%s", out)
	}
}

// A host bound on inbound A must not leak onto inbound B for a client on
// both: A renders only the bound host, B keeps its own endpoint.
func TestSub_ClientHostRule_ScopedToInbound(t *testing.T) {
	seedSubDB(t)
	db := database.GetDB()
	ibA := seedSubInbound(t, "s1", "scope-a", 443, 1, tcpTLSStream)
	var rec model.ClientRecord
	if err := db.Where("email = ?", "scope-a@e").First(&rec).Error; err != nil {
		t.Fatalf("load client record: %v", err)
	}
	ibB := &model.Inbound{
		UserId: 1, Tag: "scope-b", Enable: true, Listen: "203.0.113.5", Port: 8443,
		Protocol: model.VLESS, Remark: "scope-b", Settings: ibA.Settings,
		StreamSettings: tcpTLSStream, SubSortIndex: 2,
	}
	if err := db.Create(ibB).Error; err != nil {
		t.Fatalf("seed inbound B: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ibB.Id}).Error; err != nil {
		t.Fatalf("attach client to inbound B: %v", err)
	}
	hostA := seedHost(t, &model.Host{InboundId: ibA.Id, SortOrder: 1, Remark: "HA", Address: "host-a.cdn.com", Port: 9443, Security: "tls", Sni: "host-a.sni"})
	seedHost(t, &model.Host{InboundId: ibA.Id, SortOrder: 2, Remark: "HA2", Address: "host-a2.cdn.com", Port: 9443, Security: "tls", Sni: "host-a2.sni"})
	if err := db.Model(&model.ClientRecord{}).Where("id = ?", rec.Id).
		Update("client_host_rule_ids", model.EncodeClientHostRuleIds([]int{hostA.Id})).Error; err != nil {
		t.Fatalf("bind host rule: %v", err)
	}

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	parts := strings.Split(strings.Join(links, "\n"), "\n")
	if len(parts) != 2 {
		t.Fatalf("one link per inbound, got %d: %v", len(parts), parts)
	}
	joined := strings.Join(parts, "\n")
	if got := strings.Count(joined, "host-a.cdn.com"); got != 1 {
		t.Fatalf("host-a address appears %d times, want once (inbound A only):\n%s", got, joined)
	}
	if got := strings.Count(joined, "host-a.sni"); got != 1 {
		t.Fatalf("host-a SNI appears %d times, want once (inbound A only):\n%s", got, joined)
	}
	if strings.Contains(joined, "host-a2.cdn.com") || strings.Contains(joined, "host-a2.sni") {
		t.Fatalf("unbound host-a2 must not render for the bound client:\n%s", joined)
	}
	plain := ""
	for _, l := range parts {
		if strings.Contains(l, "203.0.113.5:8443") {
			plain = l
		}
	}
	if plain == "" {
		t.Fatalf("inbound B must keep its own endpoint 203.0.113.5:8443:\n%s", joined)
	}
	if strings.Contains(plain, "host-a.cdn.com") || strings.Contains(plain, "host-a.sni") {
		t.Fatalf("inbound B link leaked host A:\n%s", plain)
	}

	jsOut, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if got := strings.Count(jsOut, "host-a.cdn.com"); got != 1 {
		t.Fatalf("json host-a address appears %d times, want once:\n%s", got, jsOut)
	}
	if got := strings.Count(jsOut, "host-a.sni"); got != 1 {
		t.Fatalf("json host-a SNI appears %d times, want once:\n%s", got, jsOut)
	}
	if strings.Contains(jsOut, "host-a2.cdn.com") || strings.Contains(jsOut, "host-a2.sni") {
		t.Fatalf("json must not contain the unbound host-a2:\n%s", jsOut)
	}
	if !strings.Contains(jsOut, "203.0.113.5") {
		t.Fatalf("json must keep inbound B on its own address:\n%s", jsOut)
	}

	yaml, _, err := NewSubClashService(false, "", false, NewSubService("")).GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if got := strings.Count(yaml, "host-a.cdn.com"); got != 1 {
		t.Fatalf("clash host-a server appears %d times, want once:\n%s", got, yaml)
	}
	if strings.Contains(yaml, "host-a2.cdn.com") {
		t.Fatalf("clash must not contain the unbound host-a2:\n%s", yaml)
	}
	if !strings.Contains(yaml, "203.0.113.5") {
		t.Fatalf("clash must keep inbound B on its own server:\n%s", yaml)
	}
}

// Binding survives a host-rule edit: UpdateHostGroup keeps row ids, so
// raw/JSON/Clash still render only the bound host with the new SNI.
func TestSub_ClientHostRule_SurvivesGroupEdit(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "edit", 4451, 1, tcpTLSStream)
	hostSvc := &service.HostService{}
	created, err := hostSvc.AddHostGroup(&entity.HostGroup{
		InboundIds: []int{ib.Id},
		Hosts:      []string{"a.cdn.com:443", "b.cdn.com:443"},
		Remark:     "orig",
		Port:       443,
		Security:   "tls",
		Sni:        "old.sni",
	})
	if err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("created = %d rows, want 2", len(created))
	}
	boundId := 0
	for _, h := range created {
		if h.Address == "b.cdn.com" {
			boundId = h.Id
		}
	}
	if boundId == 0 {
		t.Fatalf("bound host row not found in %v", created)
	}

	db := database.GetDB()
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "edit@e").
		Update("client_host_rule_ids", model.EncodeClientHostRuleIds([]int{boundId})).Error; err != nil {
		t.Fatalf("bind host rule: %v", err)
	}

	g, err := hostSvc.GetHostGroup(created[0].GroupId)
	if err != nil {
		t.Fatalf("GetHostGroup: %v", err)
	}
	g.Remark = "renamed"
	g.Sni = "new.sni"
	if _, err := hostSvc.UpdateHostGroup(g.GroupId, g); err != nil {
		t.Fatalf("UpdateHostGroup: %v", err)
	}

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	parts := strings.Split(strings.Join(links, "\n"), "\n")
	if len(parts) != 1 {
		t.Fatalf("edited binding must still render 1 link, got %d: %v", len(parts), parts)
	}
	if !strings.Contains(parts[0], "b.cdn.com:443") || !strings.Contains(parts[0], "sni=new.sni") {
		t.Fatalf("bound link must carry the edited host with new SNI: %s", parts[0])
	}
	if strings.Contains(parts[0], "a.cdn.com") || strings.Contains(parts[0], "sni=old.sni") {
		t.Fatalf("edited binding must not fall back to the unbound host: %s", parts[0])
	}

	jsOut, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if !strings.Contains(jsOut, "b.cdn.com") || !strings.Contains(jsOut, "new.sni") {
		t.Fatalf("json must carry the edited bound host with new SNI:\n%s", jsOut)
	}
	if strings.Contains(jsOut, "a.cdn.com") || strings.Contains(jsOut, "old.sni") {
		t.Fatalf("json must not contain the unbound host or old SNI:\n%s", jsOut)
	}

	yaml, _, err := NewSubClashService(false, "", false, NewSubService("")).GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if got := strings.Count(yaml, "b.cdn.com"); got != 1 {
		t.Fatalf("clash bound host appears %d times, want once:\n%s", got, yaml)
	}
	if !strings.Contains(yaml, "new.sni") {
		t.Fatalf("clash must carry the new SNI:\n%s", yaml)
	}
	if strings.Contains(yaml, "a.cdn.com") || strings.Contains(yaml, "old.sni") {
		t.Fatalf("clash must not contain the unbound host or old SNI:\n%s", yaml)
	}
}

func TestSub_HostMultiSNI_RandomizePerConfig(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "sni-rand", 4465, 1,
		`{"network":"tcp","security":"tls"}`)
	snis := []string{"sni1.example.com", "sni2.example.com"}
	seedHost(t, &model.Host{
		InboundId:   ib.Id,
		SortOrder:   0,
		Remark:      "MultiSNI-CIDR",
		Address:     "198.51.100.0/24",
		Port:        8443,
		Security:    "tls",
		RandomCount: 30,
		Snis:        snis,
	})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	parts := strings.Split(strings.Join(links, "\n"), "\n")
	if len(parts) != 30 {
		t.Fatalf("len(parts) = %d, want 30", len(parts))
	}

	seenSNIs := make(map[string]int)
	for _, l := range parts {
		if strings.Contains(l, "sni=sni1.example.com") {
			seenSNIs["sni1.example.com"]++
		} else if strings.Contains(l, "sni=sni2.example.com") {
			seenSNIs["sni2.example.com"]++
		} else {
			t.Fatalf("link carries unexpected SNI: %s", l)
		}
	}
	if len(seenSNIs) < 2 {
		t.Fatalf("expected both SNIs to be picked across 30 randomized configs, got: %v", seenSNIs)
	}
}

func TestSub_HostMultiSNI_SharedRandom(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "sni-shared", 4466, 1,
		`{"network":"tcp","security":"tls"}`)
	snis := []string{"sniA.example.com", "sniB.example.com"}
	groupId := "grp_multi_sni_shared"
	seedHost(t, &model.Host{
		GroupId:         groupId,
		InboundId:       ib.Id,
		SortOrder:       1,
		Remark:          "Host1",
		Address:         "h1.example.com",
		Port:            8443,
		Security:        "tls",
		Snis:            snis,
		SniSharedRandom: true,
	})
	seedHost(t, &model.Host{
		GroupId:         groupId,
		InboundId:       ib.Id,
		SortOrder:       2,
		Remark:          "Host2",
		Address:         "h2.example.com",
		Port:            8443,
		Security:        "tls",
		Snis:            snis,
		SniSharedRandom: true,
	})
	seedHost(t, &model.Host{
		GroupId:         groupId,
		InboundId:       ib.Id,
		SortOrder:       3,
		Remark:          "Host3",
		Address:         "h3.example.com",
		Port:            8443,
		Security:        "tls",
		Snis:            snis,
		SniSharedRandom: true,
	})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	parts := strings.Split(strings.Join(links, "\n"), "\n")
	if len(parts) != 3 {
		t.Fatalf("len(parts) = %d, want 3", len(parts))
	}

	var firstSNI string
	for i, l := range parts {
		var sni string
		if strings.Contains(l, "sni=sniA.example.com") {
			sni = "sniA.example.com"
		} else if strings.Contains(l, "sni=sniB.example.com") {
			sni = "sniB.example.com"
		} else {
			t.Fatalf("link[%d] missing expected SNI: %s", i, l)
		}
		if firstSNI == "" {
			firstSNI = sni
		} else if sni != firstSNI {
			t.Fatalf("shared random mode mismatch: link[%d] had %s, expected %s across all configs in same group", i, sni, firstSNI)
		}
	}

	// Verify that across independent subscription fetches, both pool SNIs are selected
	seenAcrossFetches := make(map[string]bool)
	seenAcrossFetches[firstSNI] = true
	for i := 0; i < 25; i++ {
		l, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
		if err == nil && len(l) > 0 {
			if strings.Contains(l[0], "sni=sniA.example.com") {
				seenAcrossFetches["sniA.example.com"] = true
			} else if strings.Contains(l[0], "sni=sniB.example.com") {
				seenAcrossFetches["sniB.example.com"] = true
			}
		}
	}
	if len(seenAcrossFetches) < 2 {
		t.Fatalf("expected both SNIs to be selected across independent fetches, got: %v", seenAcrossFetches)
	}
}

func TestSub_HostMultiSNI_Overrides(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "sni-overrides", 4467, 1,
		`{"network":"tcp","security":"tls"}`)
	snis := []string{"sni1.example.com", "sni2.example.com"}

	// When OverrideSniFromAddress is true, address is used as SNI.
	seedHost(t, &model.Host{
		InboundId:              ib.Id,
		SortOrder:              1,
		Remark:                 "AddrAsSni",
		Address:                "addr.example.com",
		Port:                   8443,
		Security:               "tls",
		Snis:                   snis,
		OverrideSniFromAddress: true,
	})

	// When KeepSniBlank is true, SNI is omitted.
	seedHost(t, &model.Host{
		InboundId:    ib.Id,
		SortOrder:    2,
		Remark:       "BlankSni",
		Address:      "blank.example.com",
		Port:         8443,
		Security:     "tls",
		Snis:         snis,
		KeepSniBlank: true,
	})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	parts := strings.Split(strings.Join(links, "\n"), "\n")
	if len(parts) != 2 {
		t.Fatalf("len(parts) = %d, want 2", len(parts))
	}
	if !strings.Contains(parts[0], "sni=addr.example.com") {
		t.Errorf("expected address as sni: %s", parts[0])
	}
	if strings.Contains(parts[1], "sni=") {
		t.Errorf("expected blank sni: %s", parts[1])
	}
}
