package sub

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// initMutDB spins up a real temp SQLite DB for tests that exercise DB-backed
// query helpers, mirroring the house pattern in service_sharelink/dedup tests.
func initMutDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

// --- json_service.go:40 — rules are merged into routing only when non-empty ---

func TestSubJsonService_CustomRulesPrepended(t *testing.T) {
	rules := `[{"type":"field","domain":["geosite:ads"],"outboundTag":"block"}]`
	svc := NewSubJsonService("", rules, "", nil)

	routing, ok := svc.configJson["routing"].(map[string]any)
	if !ok {
		t.Fatalf("routing missing: %#v", svc.configJson["routing"])
	}
	got, _ := routing["rules"].([]any)
	// default.json ships exactly 1 rule; the custom rule must be prepended.
	if len(got) != 2 {
		t.Fatalf("rules len = %d, want 2 (custom prepended to default)", len(got))
	}
	first, _ := got[0].(map[string]any)
	if domains, _ := first["domain"].([]any); len(domains) != 1 || domains[0] != "geosite:ads" {
		t.Fatalf("custom rule must come first, got %#v", got[0])
	}
}

func TestSubJsonService_EmptyRulesLeavesDefault(t *testing.T) {
	svc := NewSubJsonService("", "", "", nil)
	routing, _ := svc.configJson["routing"].(map[string]any)
	got, _ := routing["rules"].([]any)
	if len(got) != 1 {
		t.Fatalf("rules len = %d, want 1 (no custom rules → default untouched)", len(got))
	}
}

// --- json_service.go:331,356,408 — mux is attached only when configured ---

func TestSubJsonService_MuxAttachedWhenConfigured(t *testing.T) {
	const mux = `{"enabled":true,"concurrency":8}`
	client := model.Client{ID: "uuid-1", Password: "p4ss"}

	cases := []struct {
		name     string
		raw      []byte
		wantMux  bool
		protocol model.Protocol
	}{
		{"vmess mux", NewSubJsonService(mux, "", "", nil).genVnext(&model.Inbound{Protocol: model.VMESS, Settings: `{}`}, nil, client, mux), true, model.VMESS},
		{"vless mux", NewSubJsonService(mux, "", "", nil).genVless(&model.Inbound{Protocol: model.VLESS, Settings: `{}`}, nil, client, mux), true, model.VLESS},
		{"server mux", NewSubJsonService(mux, "", "", nil).genServer(&model.Inbound{Protocol: model.Trojan, Settings: `{}`}, nil, client, mux), true, model.Trojan},
		{"vmess no mux", NewSubJsonService("", "", "", nil).genVnext(&model.Inbound{Protocol: model.VMESS, Settings: `{}`}, nil, client, ""), false, model.VMESS},
		{"vless no mux", NewSubJsonService("", "", "", nil).genVless(&model.Inbound{Protocol: model.VLESS, Settings: `{}`}, nil, client, ""), false, model.VLESS},
		{"server no mux", NewSubJsonService("", "", "", nil).genServer(&model.Inbound{Protocol: model.Trojan, Settings: `{}`}, nil, client, ""), false, model.Trojan},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ob map[string]any
			if err := json.Unmarshal(tc.raw, &ob); err != nil {
				t.Fatalf("unmarshal outbound: %v", err)
			}
			m, has := ob["mux"]
			if tc.wantMux {
				if !has {
					t.Fatalf("mux must be set when configured, outbound = %#v", ob)
				}
				mm, _ := m.(map[string]any)
				if mm["enabled"] != true || mm["concurrency"] != float64(8) {
					t.Fatalf("mux payload wrong: %#v", m)
				}
			} else if has {
				t.Fatalf("mux must be omitted when empty, outbound = %#v", ob)
			}
		})
	}
}

// --- json_service.go:268 — a non-empty finalMask that merges to nothing must
// not add the finalmask key (the `len(merged) > 0` guard). ---

func TestSubJsonService_FinalMaskMergingToEmptyNotAdded(t *testing.T) {
	// finalMask is non-empty (passes the len(fm)==0 early return) but its only
	// key is an empty tcp slice, which mergeFinalMask drops → merged is empty,
	// so applyGlobalFinalMask (json_service.go:268) must NOT set finalmask.
	svc := NewSubJsonService("", "", `{"tcp":[]}`, nil)
	stream := svc.streamData(`{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`)
	if _, ok := stream["finalmask"]; ok {
		t.Fatalf("finalMask merging to empty must not add a finalmask key: %#v", stream["finalmask"])
	}

	// Sanity: a finalMask that DOES merge to something still gets set, so the
	// guard is the only distinguishing factor.
	svc2 := NewSubJsonService("", "", `{"tcp":[{"type":"fragment"}]}`, nil)
	stream2 := svc2.streamData(`{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`)
	if _, ok := stream2["finalmask"]; !ok {
		t.Fatal("non-empty finalMask must be set")
	}
}

// --- json_service.go:494 — an empty extra tcp slice must not clobber the base ---

func TestMergeFinalMask_EmptyExtraTcpKeepsBase(t *testing.T) {
	base := map[string]any{"tcp": []any{map[string]any{"type": "keep"}}}
	extra := map[string]any{"tcp": []any{}} // empty → must be ignored
	merged := mergeFinalMask(base, extra)
	tcp, _ := merged["tcp"].([]any)
	if len(tcp) != 1 {
		t.Fatalf("tcp len = %d, want 1 (empty extra must not drop or append)", len(tcp))
	}
	if first, _ := tcp[0].(map[string]any); first["type"] != "keep" {
		t.Fatalf("base tcp mask lost: %#v", tcp)
	}
	// Sanity: a non-empty extra DOES append, so the guard is the only thing
	// distinguishing the two paths.
	extra2 := map[string]any{"tcp": []any{map[string]any{"type": "add"}}}
	merged2 := mergeFinalMask(base, extra2)
	if tcp2, _ := merged2["tcp"].([]any); len(tcp2) != 2 {
		t.Fatalf("non-empty extra must append: len = %d, want 2", len(tcp2))
	}
}

// --- service.go:69-77 — configuredPublicHost priority: subDomain > webDomain > "" ---

func TestConfiguredPublicHost_Priority(t *testing.T) {
	initMutDB(t)
	db := database.GetDB()
	set := func(key, val string) {
		if err := db.Save(&model.Setting{Key: key, Value: val}).Error; err != nil {
			t.Fatalf("save %s: %v", key, err)
		}
	}

	s := &SubService{}

	// Both empty → "".
	if got := s.configuredPublicHost(); got != "" {
		t.Fatalf("no domains configured: got %q, want empty", got)
	}

	// Only webDomain → webDomain wins (exercises the second branch, service.go:73).
	set("webDomain", "web.example.com")
	if got := s.configuredPublicHost(); got != "web.example.com" {
		t.Fatalf("webDomain fallback: got %q, want web.example.com", got)
	}

	// subDomain set → subDomain takes precedence over webDomain (service.go:70).
	set("subDomain", "sub.example.com")
	if got := s.configuredPublicHost(); got != "sub.example.com" {
		t.Fatalf("subDomain priority: got %q, want sub.example.com", got)
	}
}

// --- service.go:248 — AggregateTrafficByEmails tracks the MAX LastOnline ---

func TestAggregateTrafficByEmails_LastOnlineIsMax(t *testing.T) {
	initMutDB(t)
	db := database.GetDB()

	rows := []xray.ClientTraffic{
		{Email: "a@x", Up: 10, Down: 20, LastOnline: 100},
		{Email: "b@x", Up: 1, Down: 2, LastOnline: 500}, // the max
		{Email: "c@x", Up: 3, Down: 4, LastOnline: 300},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed traffic: %v", err)
		}
	}

	s := &SubService{}
	agg, lastOnline := s.AggregateTrafficByEmails([]string{"a@x", "b@x", "c@x"})
	if lastOnline != 500 {
		t.Fatalf("lastOnline = %d, want 500 (max across rows)", lastOnline)
	}
	// Up/Down must still sum so a mutant can't pass by zeroing everything.
	if agg.Up != 14 || agg.Down != 26 {
		t.Fatalf("agg up/down = %d/%d, want 14/26", agg.Up, agg.Down)
	}
}

// --- service.go:329 — projectThroughFallbackMaster returns false for nil ---

func TestProjectThroughFallbackMaster_Nil(t *testing.T) {
	s := &SubService{}
	if s.projectThroughFallbackMaster(nil) {
		t.Fatal("nil inbound must yield false (no projection, no DB hit)")
	}
}

// --- service.go:555 — empty client flow must not emit a flow param even when allowed ---

func TestGenVlessLink_NoFlowWhenClientFlowEmpty(t *testing.T) {
	// tcp+reality is a flow-allowed combo; with an empty client flow the
	// len(...)>0 guard (service.go:555) must keep `flow` out of the link.
	stream := `{
		"network":"tcp","security":"reality",
		"tcpSettings":{"header":{"type":"none"}},
		"realitySettings":{"serverNames":["r.example.com"],"shortIds":["ab"],"settings":{"publicKey":"PBK","fingerprint":"chrome"}}
	}`
	inbound := &model.Inbound{
		Listen:         "203.0.113.1",
		Port:           443,
		Protocol:       model.VLESS,
		Remark:         "noflow",
		Settings:       `{"clients":[{"id":"11111111-2222-4333-8444-555555555555","email":"user"}],"encryption":"none"}`,
		StreamSettings: stream,
	}
	s := &SubService{}
	if link := s.genVlessLink(inbound, "user"); strings.Contains(link, "flow=") {
		t.Fatalf("empty client flow must not produce a flow param, got %q", link)
	}
}

// --- service.go:906-913 — applyPathAndHostParams host source ---

func TestApplyPathAndHostParams(t *testing.T) {
	// Direct host wins (service.go:908 true branch).
	params := map[string]string{}
	applyPathAndHostParams(map[string]any{"path": "/p", "host": "direct.example.com"}, params)
	if params["path"] != "/p" {
		t.Fatalf("path = %q, want /p", params["path"])
	}
	if params["host"] != "direct.example.com" {
		t.Fatalf("direct host = %q, want direct.example.com", params["host"])
	}

	// No direct host → fall back to headers.Host (service.go:908 false branch).
	params = map[string]string{}
	applyPathAndHostParams(map[string]any{
		"path":    "/p",
		"headers": map[string]any{"Host": "via-header.example.com"},
	}, params)
	if params["host"] != "via-header.example.com" {
		t.Fatalf("header host fallback = %q, want via-header.example.com", params["host"])
	}

	// Empty-string host must NOT shadow the header fallback (len(host) > 0 guard).
	params = map[string]string{}
	applyPathAndHostParams(map[string]any{
		"path":    "/p",
		"host":    "",
		"headers": map[string]any{"Host": "via-header.example.com"},
	}, params)
	if params["host"] != "via-header.example.com" {
		t.Fatalf("empty host must defer to headers, got %q", params["host"])
	}
}

// --- external_config.go:39,42,55,58 — getClientExternalLinksBySubId ---

func TestGetClientExternalLinksBySubId(t *testing.T) {
	initMutDB(t)
	db := database.GetDB()
	s := &SubService{}

	// No client rows for the subId → nil, no error (service.go path :42).
	out, err := s.getClientExternalLinksBySubId("missing")
	if err != nil {
		t.Fatalf("missing subId err = %v, want nil", err)
	}
	if out != nil {
		t.Fatalf("missing subId = %#v, want nil", out)
	}

	// A client with NO external-link rows → nil (the rows-empty guard :58).
	bare := &model.ClientRecord{Email: "bare@x", SubID: "sub-bare", UUID: "u", Enable: true}
	if err := db.Create(bare).Error; err != nil {
		t.Fatalf("seed bare client: %v", err)
	}
	out, err = s.getClientExternalLinksBySubId("sub-bare")
	if err != nil {
		t.Fatalf("bare subId err = %v", err)
	}
	if out != nil {
		t.Fatalf("client with no links = %#v, want nil", out)
	}

	// A client with two link rows: ordering by sort_index and email/enable
	// attribution from the owning client (the loop copies rec.Email/rec.Enable).
	rec := &model.ClientRecord{Email: "owner@x", SubID: "sub-ok", UUID: "u2", Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientExternalLink{ClientId: rec.Id, Kind: model.ExternalLinkKindLink, Value: "trojan://b", Remark: "second", SortIndex: 5}).Error; err != nil {
		t.Fatalf("seed link b: %v", err)
	}
	if err := db.Create(&model.ClientExternalLink{ClientId: rec.Id, Kind: model.ExternalLinkKindLink, Value: "trojan://a", Remark: "first", SortIndex: 1}).Error; err != nil {
		t.Fatalf("seed link a: %v", err)
	}

	out, err = s.getClientExternalLinksBySubId("sub-ok")
	if err != nil {
		t.Fatalf("ok subId err = %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("entries = %d, want 2", len(out))
	}
	// sort_index ASC: the SortIndex=1 row comes first.
	if out[0].Value != "trojan://a" || out[1].Value != "trojan://b" {
		t.Fatalf("ordering wrong: %#v", out)
	}
	// Email + Enable must be copied from the owning client, not the link row
	// (which carries neither field). The enabled owner → Enable true.
	if out[0].Email != "owner@x" || out[0].Enable != true {
		t.Fatalf("attribution wrong: email=%q enable=%v", out[0].Email, out[0].Enable)
	}

	// A DISABLED client must produce entries with Enable=false, proving the
	// value is read from the client row (Enable has a gorm default:true, so
	// flip it with a raw UPDATE that bypasses the default).
	dis := &model.ClientRecord{Email: "off@x", SubID: "sub-off", UUID: "u3", Enable: true}
	if err := db.Create(dis).Error; err != nil {
		t.Fatalf("seed disabled client: %v", err)
	}
	if err := db.Model(&model.ClientRecord{}).Where("id = ?", dis.Id).Update("enable", false).Error; err != nil {
		t.Fatalf("disable client: %v", err)
	}
	if err := db.Create(&model.ClientExternalLink{ClientId: dis.Id, Kind: model.ExternalLinkKindLink, Value: "trojan://c", SortIndex: 1}).Error; err != nil {
		t.Fatalf("seed link c: %v", err)
	}
	offOut, err := s.getClientExternalLinksBySubId("sub-off")
	if err != nil {
		t.Fatalf("off subId err = %v", err)
	}
	if len(offOut) != 1 {
		t.Fatalf("disabled client entries = %d, want 1", len(offOut))
	}
	if offOut[0].Email != "off@x" || offOut[0].Enable != false {
		t.Fatalf("disabled attribution wrong: email=%q enable=%v", offOut[0].Email, offOut[0].Enable)
	}
}

// --- external_config.go:102 — applyRemarkToLink appends a fragment when none exists ---

func TestApplyRemarkToLink_NoFragmentAppends(t *testing.T) {
	link := "trojan://pw@1.2.3.4:8443?security=tls"
	out := applyRemarkToLink(link, "DE-Node")
	if out != link+"#DE-Node" {
		t.Fatalf("no-fragment link must get the remark appended, got %q", out)
	}
}

// --- external_config.go:111 — applyVmessRemark falls back to RawURLEncoding ---

func TestApplyVmessRemark_RawURLEncodingFallback(t *testing.T) {
	// The "aa?" ps forces a URL-safe char (_) in the RawURL encoding, so
	// base64.StdEncoding.DecodeString fails and the RawURLEncoding fallback
	// path (external_config.go:111) must take over. (ps is overwritten below,
	// so its value is irrelevant to the assertions.)
	payload := map[string]any{"v": "2", "ps": "aa?", "add": "1.2.3.4", "port": "443", "id": "uuid"}
	b, _ := json.Marshal(payload)
	link := "vmess://" + base64.RawURLEncoding.EncodeToString(b)
	// Guard the premise: this link must NOT be std-decodable, else the fallback
	// branch is never reached and the test is meaningless.
	if _, err := base64.StdEncoding.DecodeString(padBase64Sub(strings.TrimPrefix(link, "vmess://"))); err == nil {
		t.Fatal("test premise broken: link is std-base64 decodable, fallback not exercised")
	}

	out := applyRemarkToLink(link, "NL-Node")
	if out == link {
		t.Fatalf("raw-url-encoded vmess remark was not applied (fallback decode broken): %q", out)
	}
	// The result re-encodes with StdEncoding; decode and verify ps + credentials.
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(out, "vmess://"))
	if err != nil {
		t.Fatalf("decode out: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["ps"] != "NL-Node" {
		t.Fatalf("ps = %v, want NL-Node", got["ps"])
	}
	if got["id"] != "uuid" {
		t.Fatalf("credentials lost via fallback path: %#v", got)
	}
}

// --- external_config.go:130 — padBase64Sub pads to a multiple of 4 ---

func TestPadBase64Sub(t *testing.T) {
	cases := map[string]string{
		"":     "",
		"a":    "a===",
		"ab":   "ab==",
		"abc":  "abc=",
		"abcd": "abcd",
	}
	for in, want := range cases {
		if got := padBase64Sub(in); got != want {
			t.Fatalf("padBase64Sub(%q) = %q, want %q", in, got, want)
		}
		if len(padBase64Sub(in))%4 != 0 {
			t.Fatalf("padBase64Sub(%q) length not a multiple of 4", in)
		}
	}
}

// --- external_subscription.go:122 — base64 body decode strips embedded whitespace ---

func TestDecodeSubscriptionBody_StripsWhitespaceInBase64(t *testing.T) {
	plain := "vless://uuid@a.com:443#one\ntrojan://pw@b.com:8443#two\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(plain))
	// Inject whitespace into the base64 token; tryDecodeBase64Body must strip it
	// (external_subscription.go:122) so decoding still succeeds.
	half := len(encoded) / 2
	dirty := encoded[:half] + "\n \t" + encoded[half:]

	links := decodeSubscriptionBody([]byte(dirty))
	if len(links) != 2 || links[0] != "vless://uuid@a.com:443#one" || links[1] != "trojan://pw@b.com:8443#two" {
		t.Fatalf("whitespace-laden base64 body decoded wrong: %#v", links)
	}
}

// --- clash_service.go:123 — duplicate proxy names disambiguate as base-N ---

func TestEnsureUniqueProxyNames_SuffixSequence(t *testing.T) {
	proxies := []map[string]any{
		{"name": "node"},
		{"name": "node"},
		{"name": "node"},
	}
	ensureUniqueProxyNames(proxies)
	if proxies[0]["name"] != "node" {
		t.Fatalf("first occurrence must keep base name, got %v", proxies[0]["name"])
	}
	if proxies[1]["name"] != "node-2" {
		t.Fatalf("second duplicate = %v, want node-2", proxies[1]["name"])
	}
	if proxies[2]["name"] != "node-3" {
		t.Fatalf("third duplicate = %v, want node-3", proxies[2]["name"])
	}
}

// --- clash_service.go:422,447 — empty transport opts must NOT add the *-opts key ---

func TestApplyTransport_EmptyOptsOmitted(t *testing.T) {
	svc := &SubClashService{}

	// httpupgrade with no path/host → opts empty → no http-upgrade-opts key (clash:422).
	huProxy := map[string]any{}
	if !svc.applyTransport(huProxy, "httpupgrade", map[string]any{"httpupgradeSettings": map[string]any{}}) {
		t.Fatal("httpupgrade must still be buildable")
	}
	if huProxy["network"] != "httpupgrade" {
		t.Fatalf("network = %v, want httpupgrade", huProxy["network"])
	}
	if _, ok := huProxy["http-upgrade-opts"]; ok {
		t.Fatalf("empty opts must not set http-upgrade-opts: %#v", huProxy["http-upgrade-opts"])
	}

	// xhttp with no path/host/mode → opts empty → no xhttp-opts key (clash:447).
	xhProxy := map[string]any{}
	if !svc.applyTransport(xhProxy, "xhttp", map[string]any{"xhttpSettings": map[string]any{}}) {
		t.Fatal("xhttp must still be buildable")
	}
	if xhProxy["network"] != "xhttp" {
		t.Fatalf("network = %v, want xhttp", xhProxy["network"])
	}
	if _, ok := xhProxy["xhttp-opts"]; ok {
		t.Fatalf("empty opts must not set xhttp-opts: %#v", xhProxy["xhttp-opts"])
	}
}
