package sub

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestHostToExternalProxyMap_VlessRoute(t *testing.T) {
	with := hostToExternalProxyMap(&model.Host{VlessRoute: "443"}, "d.example.com", 443)
	if with["vlessRoute"] != "443" {
		t.Fatalf(`ep["vlessRoute"] = %v, want "443"`, with["vlessRoute"])
	}
	without := hostToExternalProxyMap(&model.Host{}, "d.example.com", 443)
	if _, ok := without["vlessRoute"]; ok {
		t.Fatalf("empty VlessRoute must not add the key: %v", without["vlessRoute"])
	}
}

// seedSubInbound's client UUID is 11111111-2222-4333-8444-<port>, so route 443
// -> 01bb, 53 -> 0035, and a route-less host keeps 4333.
func TestSub_HostVlessRoute_RawMultiHost(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "vr", 4500, 1, wsTLSStream)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 8443, Security: "tls", VlessRoute: "443"})
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 8443, Security: "tls", VlessRoute: "53"})
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 3, Remark: "C", Address: "c.cdn.com", Port: 8443, Security: "tls"})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	parts := strings.Split(strings.Join(links, "\n"), "\n")
	if len(parts) != 3 {
		t.Fatalf("want 3 host links, got %d: %v", len(parts), parts)
	}
	if !strings.Contains(parts[0], "vless://11111111-2222-01bb-8444-") {
		t.Fatalf("host A (route 443) must encode 01bb: %s", parts[0])
	}
	if !strings.Contains(parts[1], "vless://11111111-2222-0035-8444-") {
		t.Fatalf("host B (route 53) must encode 0035: %s", parts[1])
	}
	if !strings.Contains(parts[2], "vless://11111111-2222-4333-8444-") {
		t.Fatalf("host C (no route) must keep the original 3rd group: %s", parts[2])
	}
}

func TestSub_HostVlessRoute_JSON(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "vrj", 4501, 1, wsTLSStream)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "J", Address: "j.cdn.com", Port: 8443, Security: "tls", VlessRoute: "443"})

	js := NewSubJsonService("", "", "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if !strings.Contains(out, "11111111-2222-01bb-8444-") {
		t.Fatalf("json outbound id should encode route 443 (01bb):\n%s", out)
	}
	if strings.Contains(out, "11111111-2222-4333-8444-") {
		t.Fatalf("original id 3rd group must be replaced in json:\n%s", out)
	}
}

func TestSub_HostVlessRoute_Clash(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "vrc", 4502, 1, wsTLSStream)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "C", Address: "c.cdn.com", Port: 8443, Security: "tls", VlessRoute: "443"})

	clash := NewSubClashService(false, "", NewSubService(""))
	yaml, _, err := clash.GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if !strings.Contains(yaml, "11111111-2222-01bb-8444-") {
		t.Fatalf("clash proxy uuid should encode route 443 (01bb):\n%s", yaml)
	}
	if strings.Contains(yaml, "11111111-2222-4333-8444-") {
		t.Fatalf("original uuid 3rd group must be replaced in clash:\n%s", yaml)
	}
}
