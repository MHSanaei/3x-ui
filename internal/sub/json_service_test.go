package sub

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func hasDirectOutOutbound(svc *SubJsonService) bool {
	for _, raw := range svc.defaultOutbounds {
		var outbound map[string]any
		if err := json.Unmarshal(raw, &outbound); err != nil {
			continue
		}
		if outbound["tag"] == "direct_out" {
			return true
		}
	}
	return false
}

func outboundSettings(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("failed to unmarshal outbound: %v", err)
	}
	settings, _ := parsed["settings"].(map[string]any)
	if settings == nil {
		t.Fatal("outbound has no settings")
	}
	return settings
}

func TestSubJsonServiceInjectsGlobalFinalMask(t *testing.T) {
	finalMask := `{"tcp":[{"type":"fragment","settings":{"packets":"tlshello","length":"100-200","delay":"10-20"}}],"udp":[{"type":"noise","settings":{"noise":[{"type":"base64","packet":"SGVsbG8="}]}}],"quicParams":{"congestion":"bbr"}}`
	svc := NewSubJsonService("", "", finalMask, nil)

	if hasDirectOutOutbound(svc) {
		t.Fatal("direct_out outbound must never be emitted")
	}

	stream := svc.streamData(`{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`)
	if _, ok := stream["sockopt"]; ok {
		t.Fatal("legacy direct_out dialerProxy sockopt must never be set")
	}

	finalmask, _ := stream["finalmask"].(map[string]any)
	if finalmask == nil {
		t.Fatal("streamSettings is missing finalmask")
	}

	tcp, _ := finalmask["tcp"].([]any)
	if len(tcp) != 1 {
		t.Fatalf("tcp masks len = %d, want 1", len(tcp))
	}
	if first, _ := tcp[0].(map[string]any); first["type"] != "fragment" {
		t.Fatalf("tcp[0] type = %v, want fragment", first["type"])
	}

	udp, _ := finalmask["udp"].([]any)
	if len(udp) != 1 {
		t.Fatalf("udp masks len = %d, want 1", len(udp))
	}

	quic, _ := finalmask["quicParams"].(map[string]any)
	if quic == nil || quic["congestion"] != "bbr" {
		t.Fatalf("quicParams missing/wrong: %#v", finalmask["quicParams"])
	}
}

func TestSubJsonServiceMergesWithExistingFinalMask(t *testing.T) {
	finalMask := `{"tcp":[{"type":"fragment","settings":{"packets":"tlshello"}}]}`
	svc := NewSubJsonService("", "", finalMask, nil)

	stream := svc.streamData(`{
		"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}},
		"finalmask":{"tcp":[{"type":"sudoku"}]}
	}`)

	finalmask, _ := stream["finalmask"].(map[string]any)
	tcp, _ := finalmask["tcp"].([]any)
	if len(tcp) != 2 {
		t.Fatalf("tcp masks len = %d, want 2 (existing + global)", len(tcp))
	}
	a, _ := tcp[0].(map[string]any)
	b, _ := tcp[1].(map[string]any)
	if a["type"] != "sudoku" || b["type"] != "fragment" {
		t.Fatalf("tcp masks = %#v, want existing sudoku then global fragment", tcp)
	}
}

func TestSubJsonServiceNoFinalMaskWhenEmpty(t *testing.T) {
	svc := NewSubJsonService("", "", "", nil)
	stream := svc.streamData(`{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`)
	if _, ok := stream["finalmask"]; ok {
		t.Fatal("no finalmask should be emitted when subJsonFinalMask is empty")
	}
	if _, ok := stream["sockopt"]; ok {
		t.Fatal("legacy direct_out sockopt must never be set")
	}
}

// xray-core parses tlsSettings.pinnedPeerCertSha256 as a comma-separated string;
// the JSON subscription must emit that form, not an array, or v2ray clients fail
// to import the config (#5401).
func TestSubJsonServicePinnedCertJoinedToString(t *testing.T) {
	svc := NewSubJsonService("", "", "", nil)
	stream := svc.streamData(`{"network":"tcp","security":"tls","tlsSettings":{"serverName":"a.example.com","settings":{"pinnedPeerCertSha256":["aa11","bb22"]}}}`)

	tls, _ := stream["tlsSettings"].(map[string]any)
	if tls == nil {
		t.Fatalf("tlsSettings missing: %#v", stream)
	}
	if got := tls["pinnedPeerCertSha256"]; got != "aa11,bb22" {
		t.Fatalf("pinnedPeerCertSha256 = %#v, want comma-separated string \"aa11,bb22\"", got)
	}
}

func TestSubJsonServiceVlessFlattened(t *testing.T) {
	inbound := &model.Inbound{Listen: "1.2.3.4", Port: 443, Protocol: model.VLESS, Settings: `{"encryption":"none"}`}
	client := model.Client{ID: "uuid-1", Flow: "xtls-rprx-vision"}

	settings := outboundSettings(t, NewSubJsonService("", "", "", nil).genVless(inbound, nil, client, ""))
	if _, ok := settings["vnext"]; ok {
		t.Fatal("vless outbound must not use vnext")
	}
	if settings["address"] != "1.2.3.4" || settings["id"] != "uuid-1" || settings["encryption"] != "none" || settings["flow"] != "xtls-rprx-vision" {
		t.Fatalf("flat vless settings wrong: %#v", settings)
	}
}

func TestSubJsonServiceVmessFlattened(t *testing.T) {
	inbound := &model.Inbound{Listen: "1.2.3.4", Port: 443, Protocol: model.VMESS, Settings: `{}`}
	client := model.Client{ID: "uuid-2"}

	settings := outboundSettings(t, NewSubJsonService("", "", "", nil).genVnext(inbound, nil, client, ""))
	if _, ok := settings["vnext"]; ok {
		t.Fatal("vmess outbound must not use vnext")
	}
	if settings["id"] != "uuid-2" || settings["security"] != "auto" {
		t.Fatalf("flat vmess settings wrong: %#v", settings)
	}
}

// Shadowsocks/Trojan outbounds must use the standard "servers" array so older
// bundled xray-cores (e.g. v2rayN) parse them; the flat top-level form only
// works on very recent xray-core.
func TestSubJsonServiceServerUsesServersArray(t *testing.T) {
	trojan := &model.Inbound{Listen: "1.2.3.4", Port: 443, Protocol: model.Trojan, Settings: `{}`}
	client := model.Client{Password: "p4ss"}

	settings := outboundSettings(t, NewSubJsonService("", "", "", nil).genServer(trojan, nil, client, ""))
	server := firstServer(settings)
	if server == nil {
		t.Fatalf("trojan outbound must use a servers array, got: %#v", settings)
	}
	if server["password"] != "p4ss" || server["address"] != "1.2.3.4" {
		t.Fatalf("trojan server entry wrong: %#v", server)
	}
	if _, ok := server["method"]; ok {
		t.Fatalf("trojan must not carry method: %#v", server)
	}

	ss := &model.Inbound{Listen: "1.2.3.4", Port: 443, Protocol: model.Shadowsocks, Settings: `{"method":"aes-256-gcm"}`}
	ssSettings := outboundSettings(t, NewSubJsonService("", "", "", nil).genServer(ss, nil, client, ""))
	ssServer := firstServer(ssSettings)
	if ssServer == nil {
		t.Fatalf("shadowsocks outbound must use a servers array, got: %#v", ssSettings)
	}
	if ssServer["method"] != "aes-256-gcm" {
		t.Fatalf("shadowsocks server entry must carry method: %#v", ssServer)
	}
}
