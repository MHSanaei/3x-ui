package sub

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const dnsTestStream = `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`

func docDnsBlock(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	dns, _ := doc["dns"].(map[string]any)
	if dns == nil {
		t.Fatalf("doc has no dns block: %v", doc["dns"])
	}
	return dns
}

func onlySubJsonDoc(t *testing.T, js *SubJsonService, subId string) map[string]any {
	t.Helper()
	out, _, err := js.GetJson(subId, "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	if len(docs) != 1 {
		t.Fatalf("docs = %d, want 1:\n%s", len(docs), out)
	}
	return docs[0]
}

// A bare servers array must replace the template resolver, not append to it.
func TestSubJsonDns_ArrayReplacesTemplateServers(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4901, 1, dnsTestStream)

	js := NewSubJsonService("", "", "", "", NewSubService(""))
	js.SetDnsConfig(`["https://dns.google/dns-query", {"address": "tls://1.1.1.1", "domains": ["geosite:youtube"]}]`)

	dns := docDnsBlock(t, onlySubJsonDoc(t, js, "s1"))
	servers, _ := dns["servers"].([]any)
	if len(servers) != 2 {
		t.Fatalf("servers = %v, want 2", servers)
	}
	if servers[0] != "https://dns.google/dns-query" {
		t.Fatalf("servers[0] = %v", servers[0])
	}
	second, _ := servers[1].(map[string]any)
	if second["address"] != "tls://1.1.1.1" {
		t.Fatalf("servers[1] = %v", second)
	}
	if domains, _ := second["domains"].([]any); strings.Join(stringify(domains), ",") != "geosite:youtube" {
		t.Fatalf("servers[1].domains = %v", second["domains"])
	}
	if _, hasTemplate := dns["tag"]; hasTemplate {
		t.Fatalf("template dns keys leaked into the override: %v", dns)
	}
}

func TestSubJsonDns_ObjectReplacesWholeBlock(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4902, 1, dnsTestStream)

	js := NewSubJsonService("", "", "", "", NewSubService(""))
	js.SetDnsConfig(`{"tag":"panel_dns","queryStrategy":"UseIPv4","disableCache":true,"hosts":{"example.com":"1.2.3.4"},"servers":[{"address":"1.1.1.1","skipFallback":true}]}`)

	dns := docDnsBlock(t, onlySubJsonDoc(t, js, "s1"))
	if dns["tag"] != "panel_dns" || dns["queryStrategy"] != "UseIPv4" || dns["disableCache"] != true {
		t.Fatalf("dns header = %v", dns)
	}
	hosts, _ := dns["hosts"].(map[string]any)
	if hosts["example.com"] != "1.2.3.4" {
		t.Fatalf("dns hosts = %v", dns["hosts"])
	}
	servers, _ := dns["servers"].([]any)
	if len(servers) != 1 {
		t.Fatalf("servers = %v", servers)
	}
	server, _ := servers[0].(map[string]any)
	if server["address"] != "1.1.1.1" || server["skipFallback"] != true {
		t.Fatalf("server = %v", server)
	}
}

// The explicit panel DNS block wins over the profile's, while the profile keeps
// owning the routing rules.
func TestSubJsonDns_OverridesRoutingProfileDns(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4903, 1, dnsTestStream)

	js := NewSubJsonService("", "", "", bakedRoutingPayload, NewSubService(""))
	js.SetDnsConfig(`["9.9.9.9"]`)

	doc := onlySubJsonDoc(t, js, "s1")
	dns := docDnsBlock(t, doc)
	servers, _ := dns["servers"].([]any)
	if len(servers) != 1 || servers[0] != "9.9.9.9" {
		t.Fatalf("servers = %v, want the panel override only", servers)
	}
	if hosts, _ := dns["hosts"].(map[string]any); len(hosts) != 0 {
		t.Fatalf("profile dns hosts survived the override: %v", hosts)
	}
	want := "domain->block,domain->proxy,domain->direct,ip->direct,network->proxy"
	if got := strings.Join(ruleSignatures(t, doc), ","); got != want {
		t.Fatalf("rules = %v\nwant %v", got, want)
	}
}

func TestSubJsonDns_InvalidSettingKeepsTemplateDns(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"malformed JSON", `{"servers": [`},
		{"bare string", `"8.8.8.8"`},
		{"servers not an array", `{"servers": "8.8.8.8"}`},
		{"entry without address", `[{"domains": ["geosite:youtube"]}]`},
		{"non-string entry", `[53]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seedSubDB(t)
			seedSubInbound(t, "s1", "tcpin", 4904, 1, dnsTestStream)

			js := NewSubJsonService("", "", "", "", NewSubService(""))
			js.SetDnsConfig(tc.value)

			dns := docDnsBlock(t, onlySubJsonDoc(t, js, "s1"))
			if dns["tag"] != "dns_out" || dns["queryStrategy"] != "UseIP" {
				t.Fatalf("template dns header = %v", dns)
			}
			servers, _ := dns["servers"].([]any)
			if len(servers) != 1 {
				t.Fatalf("template servers = %v", servers)
			}
			first, _ := servers[0].(map[string]any)
			if first["address"] != "8.8.8.8" {
				t.Fatalf("template server = %v", first)
			}
		})
	}
}

func TestSubJsonDns_BlankKeepsTemplateDns(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4905, 1, dnsTestStream)

	js := NewSubJsonService("", "", "", "", NewSubService(""))
	js.SetDnsConfig("   ")

	dns := docDnsBlock(t, onlySubJsonDoc(t, js, "s1"))
	servers, _ := dns["servers"].([]any)
	first, _ := servers[0].(map[string]any)
	if first["address"] != "8.8.8.8" {
		t.Fatalf("template server = %v", first)
	}
}

// Balancer documents are built from the same template, so they carry the
// override too.
func TestSubJsonDns_AppliesToBalancerDocuments(t *testing.T) {
	seedSubDB(t)
	tcp := seedSubInbound(t, "s1", "tcpin", 4906, 1, dnsTestStream)
	seedSubBalancer(t, &model.SubBalancer{
		Remark: "auto", Strategy: "random", InboundIds: []int{tcp.Id}, SortOrder: 1, Enabled: true,
	})

	js := NewSubJsonService("", "", "", "", NewSubService(""))
	js.SetDnsConfig(`["https://dns.google/dns-query"]`)

	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	balancerDoc := findDocByRemarks(parseSubJsonDocs(t, out), "auto")
	if balancerDoc == nil {
		t.Fatalf("balancer doc missing:\n%s", out)
	}
	servers, _ := docDnsBlock(t, balancerDoc)["servers"].([]any)
	if len(servers) != 1 || servers[0] != "https://dns.google/dns-query" {
		t.Fatalf("balancer dns servers = %v", servers)
	}
}

// The validator rejects a block whose types xray cannot decode, even when the
// servers list itself looks fine.
func TestSubJsonDns_BrokenBlockKeepsTemplateDns(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4907, 1, dnsTestStream)

	js := NewSubJsonService("", "", "", "", NewSubService(""))
	js.SetDnsConfig(`{"servers": ["1.1.1.1"], "hosts": 5}`)

	dns := docDnsBlock(t, onlySubJsonDoc(t, js, "s1"))
	servers, _ := dns["servers"].([]any)
	first, _ := servers[0].(map[string]any)
	if len(servers) != 1 || first["address"] != "8.8.8.8" {
		t.Fatalf("template dns = %v", dns)
	}
}

// An unusable routing profile degrades to an empty spec; the DNS override must
// still reach the document.
func TestSubJsonDns_AppliesWhenRoutingProfileUnusable(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "tcpin", 4908, 1, dnsTestStream)

	js := NewSubJsonService("", "", "", "not-a-routing-payload", NewSubService(""))
	js.SetDnsConfig(`["9.9.9.9"]`)

	doc := onlySubJsonDoc(t, js, "s1")
	servers, _ := docDnsBlock(t, doc)["servers"].([]any)
	if len(servers) != 1 || servers[0] != "9.9.9.9" {
		t.Fatalf("dns servers = %v", servers)
	}
	want := "network->proxy"
	if got := strings.Join(ruleSignatures(t, doc), ","); got != want {
		t.Fatalf("rules = %v, want the plain template rule %v", got, want)
	}
}

// The dummy info node is emitted as a document too, so it carries the panel DNS.
func TestSubJsonDns_AppliesToInfoNodeDocument(t *testing.T) {
	setupInfoNodeTestDB(t)
	db := database.GetDB()

	ib := &model.Inbound{
		Id: 1, UserId: 1, Remark: "Germany-VLESS", Enable: true, Port: 443,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[{"id":"c1-uuid","email":"user1@test.com","subId":"sub-json","enable":true,"totalGB":10737418240}]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientRecord{Id: 1, Email: "user1@test.com", SubID: "sub-json", UUID: "c1-uuid", Enable: true, TotalGB: 10737418240}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{InboundId: 1, ClientId: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: 1, Email: "user1@test.com", Up: 1073741824, Down: 1073741824, Total: 10737418240, Enable: true}).Error; err != nil {
		t.Fatal(err)
	}

	sub := NewSubService("{{EMAIL}}|📊{{TRAFFIC_LEFT}}")
	sub.subInfoNodeEnable = true
	js := NewSubJsonService("", "", "", "", sub)
	js.SetDnsConfig(`["https://dns.google/dns-query"]`)

	out, _, err := js.GetJson("sub-json", "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	if len(docs) != 2 {
		t.Fatalf("docs = %d, want info node + inbound:\n%s", len(docs), out)
	}
	servers, _ := docDnsBlock(t, docs[0])["servers"].([]any)
	if len(servers) != 1 || servers[0] != "https://dns.google/dns-query" {
		t.Fatalf("info node dns servers = %v", servers)
	}
}
