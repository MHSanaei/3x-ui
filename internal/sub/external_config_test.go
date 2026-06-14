package sub

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"github.com/goccy/go-json"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestApplyRemarkToLinkRewritesFragment(t *testing.T) {
	link := "vless://uuid@example.com:443?security=reality&pbk=abc&sid=12#old-name"
	out := applyRemarkToLink(link, "DE-Provider")
	u, err := url.Parse(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Fragment != "DE-Provider" {
		t.Fatalf("fragment = %q, want DE-Provider", u.Fragment)
	}
	// Everything before the fragment must be byte-for-byte preserved.
	if !strings.HasPrefix(out, "vless://uuid@example.com:443?security=reality&pbk=abc&sid=12#") {
		t.Fatalf("link body altered: %s", out)
	}
}

func TestApplyRemarkToLinkVmessSetsPs(t *testing.T) {
	payload := map[string]any{"v": "2", "ps": "old", "add": "1.2.3.4", "port": "443", "id": "uuid"}
	b, _ := json.Marshal(payload)
	link := "vmess://" + base64.StdEncoding.EncodeToString(b)

	out := applyRemarkToLink(link, "NL-Node")
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(out, "vmess://"))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["ps"] != "NL-Node" {
		t.Fatalf("ps = %v, want NL-Node", got["ps"])
	}
	if got["id"] != "uuid" {
		t.Fatalf("credentials lost: %v", got)
	}
}

func TestApplyRemarkEmptyKeepsLinkVerbatim(t *testing.T) {
	link := "trojan://pass@1.2.3.4:8443?security=tls#orig"
	if out := applyRemarkToLink(link, ""); out != link {
		t.Fatalf("empty remark altered link: %s", out)
	}
}

func TestParsedExternalOutboundTagsProxy(t *testing.T) {
	link := "vless://uuid@example.com:443?type=tcp&security=reality&pbk=abc&sid=12&fp=chrome#srv"
	data := parsedExternalOutbound(link)
	if data == nil {
		t.Fatal("expected an outbound, got nil")
	}
	var ob map[string]any
	if err := json.Unmarshal(data, &ob); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ob["tag"] != "proxy" {
		t.Fatalf("tag = %v, want proxy", ob["tag"])
	}
	if ob["protocol"] != "vless" {
		t.Fatalf("protocol = %v, want vless", ob["protocol"])
	}
}

func TestDecodeSubscriptionBodyBase64(t *testing.T) {
	plain := "vless://uuid@a.com:443#one\ntrojan://pw@b.com:8443#two\n"
	body := []byte(base64.StdEncoding.EncodeToString([]byte(plain)))
	links := decodeSubscriptionBody(body)
	if len(links) != 2 || links[0] != "vless://uuid@a.com:443#one" || links[1] != "trojan://pw@b.com:8443#two" {
		t.Fatalf("decoded links = %#v", links)
	}
}

func TestDecodeSubscriptionBodyPlainSkipsComments(t *testing.T) {
	body := []byte("# header\nvmess://abc\n\nnot-a-link\nss://def#x\n")
	links := decodeSubscriptionBody(body)
	if len(links) != 2 || links[0] != "vmess://abc" || links[1] != "ss://def#x" {
		t.Fatalf("decoded links = %#v", links)
	}
}

func TestExpandEntryLinkAppliesRemark(t *testing.T) {
	got := expandEntry(externalLinkEntry{Kind: model.ExternalLinkKindLink, Value: "trojan://pw@b.com:8443#orig", Remark: "DE"})
	if len(got) != 1 || got[0].Name != "DE" {
		t.Fatalf("expandEntry = %#v", got)
	}
}

func TestClashProxyFromExternalTrojanReality(t *testing.T) {
	link := "trojan://provider-pass@37.27.201.56:8443?type=tcp&security=reality&sni=aws.amazon.com&pbk=PBK&sid=298b44&fp=chrome#srv"
	svc := NewSubClashService(false, "", NewSubService(false, "-io"))
	proxy := svc.clashProxyFromExternal(link, "DE-Provider")
	if proxy == nil {
		t.Fatal("expected a clash proxy, got nil")
	}
	if proxy["type"] != "trojan" {
		t.Fatalf("type = %v, want trojan", proxy["type"])
	}
	if proxy["server"] != "37.27.201.56" {
		t.Fatalf("server = %v", proxy["server"])
	}
	if proxy["password"] != "provider-pass" {
		t.Fatalf("password = %v", proxy["password"])
	}
	if proxy["name"] != "DE-Provider" {
		t.Fatalf("name = %v", proxy["name"])
	}
	if proxy["tls"] != true {
		t.Fatalf("expected reality→tls true, got %v", proxy["tls"])
	}
}
