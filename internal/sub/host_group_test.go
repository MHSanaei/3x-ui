package sub

import (
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func seedClientGroup(t *testing.T, name string) *model.ClientGroup {
	t.Helper()
	group := &model.ClientGroup{Name: name}
	if err := database.GetDB().Create(group).Error; err != nil {
		t.Fatalf("seed client group %s: %v", name, err)
	}
	return group
}

func setClientGroup(t *testing.T, email, name string) {
	t.Helper()
	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("email = ?", email).UpdateColumn("group_name", name).Error; err != nil {
		t.Fatalf("set client group %s: %v", email, err)
	}
}

func bindHostGroup(t *testing.T, hostGroupID string, clientGroupID int) {
	t.Helper()
	if err := database.GetDB().Create(&model.HostGroupClientGroup{
		HostGroupId: hostGroupID, ClientGroupId: clientGroupID,
	}).Error; err != nil {
		t.Fatalf("bind host group %s: %v", hostGroupID, err)
	}
}

func rawEndpointSet(t *testing.T, links []string) []string {
	t.Helper()
	var out []string
	for _, link := range links {
		for _, line := range strings.Split(link, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parsed, err := url.Parse(line)
			if err != nil || parsed.Hostname() == "" || parsed.Port() == "" {
				continue
			}
			out = append(out, parsed.Hostname()+":"+parsed.Port())
		}
	}
	slices.Sort(out)
	return out
}

func jsonEndpointSet(t *testing.T, body string) []string {
	t.Helper()
	docs := parseSubJsonDocs(t, body)
	var out []string
	for _, doc := range docs {
		outbounds, _ := doc["outbounds"].([]any)
		for _, raw := range outbounds {
			ob, _ := raw.(map[string]any)
			settings, _ := ob["settings"].(map[string]any)
			if address, _ := settings["address"].(string); address != "" {
				if port, _ := settings["port"].(float64); port > 0 {
					out = append(out, fmt.Sprintf("%s:%d", address, int(port)))
				}
			}
			for _, key := range []string{"vnext", "servers"} {
				list, _ := settings[key].([]any)
				for _, peerRaw := range list {
					peer, _ := peerRaw.(map[string]any)
					address, _ := peer["address"].(string)
					port, _ := peer["port"].(float64)
					if address != "" && port > 0 {
						out = append(out, fmt.Sprintf("%s:%d", address, int(port)))
					}
				}
			}
		}
	}
	slices.Sort(out)
	return out
}

func clashEndpointSet(t *testing.T, body string) []string {
	t.Helper()
	var doc struct {
		Proxies []struct {
			Server string `yaml:"server"`
			Port   int    `yaml:"port"`
		} `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("clash subscription: %v", err)
	}
	var out []string
	for _, proxy := range doc.Proxies {
		if proxy.Server != "" && proxy.Port > 0 {
			out = append(out, fmt.Sprintf("%s:%d", proxy.Server, proxy.Port))
		}
	}
	slices.Sort(out)
	return out
}

// A client in a bound group gets only that group's hosts; every other client
// keeps the inbound's full default pool (binding is a subset, not a scope).
func TestSub_ClientGroup_ReplaceDefaultPool(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "cg", 4501, 1, tcpTLSStream)
	seedSubClient(t, ib, "s1", "other@e")
	group := seedClientGroup(t, "VIP")
	setClientGroup(t, "cg@e", group.Name)
	seedHost(t, &model.Host{GroupId: "ga", InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls"})
	seedHost(t, &model.Host{GroupId: "gb", InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 443, Security: "tls"})
	bindHostGroup(t, "gb", group.Id)

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	if got := strings.Count(joined, "a.cdn.com:443"); got != 1 {
		t.Fatalf("non-member must get host A once, got %d:\n%s", got, joined)
	}
	if got := strings.Count(joined, "b.cdn.com:443"); got != 2 {
		t.Fatalf("member and non-member must both get host B, got %d:\n%s", got, joined)
	}
}

// A bound group whose only host is disabled fails closed on that inbound; it
// must not fall through to the default pool.
func TestSub_ClientGroup_DisabledHostFailsClosed(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "cg", 4502, 1, tcpTLSStream)
	group := seedClientGroup(t, "VIP")
	setClientGroup(t, "cg@e", group.Name)
	seedHost(t, &model.Host{GroupId: "ga", InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls", IsDisabled: true})
	seedHost(t, &model.Host{GroupId: "gb", InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 443, Security: "tls"})
	bindHostGroup(t, "ga", group.Id)

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("closed group tier must emit no link, got: %v", links)
	}
	jsonBody, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if strings.TrimSpace(jsonBody) != "" {
		t.Fatalf("closed group tier must emit no JSON outbound, got: %s", jsonBody)
	}
	clashBody, _, err := NewSubClashService(false, "", false, NewSubService("")).GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if strings.TrimSpace(clashBody) != "" {
		t.Fatalf("closed group tier must emit no Clash proxy, got: %s", clashBody)
	}
	if panel := NewLinkProvider().LinksForClient("req.example.com", ib, "cg@e"); len(panel) != 0 {
		t.Fatalf("panel links must fail closed too, got: %v", panel)
	}
}

// Format exclusion is applied after tier selection: an excluded group host
// closes the tier instead of falling through to an unbound host.
func TestSub_ClientGroup_FormatExclusionFailsClosed(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "cg", 4503, 1, tcpTLSStream)
	group := seedClientGroup(t, "VIP")
	setClientGroup(t, "cg@e", group.Name)
	seedHost(t, &model.Host{GroupId: "ga", InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls", ExcludeFromSubTypes: []string{"raw"}})
	seedHost(t, &model.Host{GroupId: "gb", InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 443, Security: "tls"})
	bindHostGroup(t, "ga", group.Id)

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("excluded group tier must emit no raw link, got: %v", links)
	}
	jsonBody, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if !strings.Contains(jsonBody, "a.cdn.com") || strings.Contains(jsonBody, "b.cdn.com") {
		t.Fatalf("JSON must keep the group tier, got: %s", jsonBody)
	}
}

// A group binding on inbound A must not change the member's default pool on
// inbound B.
func TestSub_ClientGroup_PerInbound(t *testing.T) {
	seedSubDB(t)
	group := seedClientGroup(t, "VIP")
	first := seedSubInbound(t, "s1", "a", 4504, 1, tcpTLSStream)
	setClientGroup(t, "a@e", group.Name)
	seedHost(t, &model.Host{GroupId: "ga", InboundId: first.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls"})
	bindHostGroup(t, "ga", group.Id)

	second := seedSubInbound(t, "s2", "b", 4505, 1, tcpTLSStream)
	setClientGroup(t, "b@e", group.Name)
	seedHost(t, &model.Host{GroupId: "gb", InboundId: second.Id, SortOrder: 1, Remark: "B", Address: "b.cdn.com", Port: 443, Security: "tls"})

	links, _, _, _, err := NewSubService("").GetSubs("s2", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	if !strings.Contains(joined, "b.cdn.com:443") || strings.Contains(joined, "a.cdn.com:443") {
		t.Fatalf("inbound B must keep its default pool, got: %s", joined)
	}
}

// Existing per-client bindings stay above group bindings.
func TestSub_ClientGroup_ClientBindingWins(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "cg", 4506, 1, tcpTLSStream)
	group := seedClientGroup(t, "VIP")
	setClientGroup(t, "cg@e", group.Name)
	hostA := seedHost(t, &model.Host{GroupId: "ga", InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls"})
	seedHost(t, &model.Host{GroupId: "gb", InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 443, Security: "tls"})
	bindHostGroup(t, "gb", group.Id)
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", "cg@e").
		Update("client_host_rule_ids", model.EncodeClientHostRuleIds([]int{hostA.Id})).Error; err != nil {
		t.Fatalf("bind client host: %v", err)
	}

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	if !strings.Contains(joined, "a.cdn.com:443") || strings.Contains(joined, "b.cdn.com:443") {
		t.Fatalf("client binding must beat the group binding, got: %s", joined)
	}
}

// Group binding is by host group, so another row added to the same group later
// is part of the member's pool without a rebind.
func TestSub_ClientGroup_RowAddedLaterInheritsBinding(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "cg", 4507, 1, tcpTLSStream)
	group := seedClientGroup(t, "VIP")
	setClientGroup(t, "cg@e", group.Name)
	seedHost(t, &model.Host{GroupId: "ga", InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls"})
	seedHost(t, &model.Host{GroupId: "ga", InboundId: ib.Id, SortOrder: 2, Remark: "A2", Address: "a2.cdn.com", Port: 443, Security: "tls"})
	bindHostGroup(t, "ga", group.Id)

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	if !strings.Contains(joined, "a.cdn.com:443") || !strings.Contains(joined, "a2.cdn.com:443") {
		t.Fatalf("both rows of the bound host group must render, got: %s", joined)
	}
}

// Hosts-first mode narrows a group member to the bound variants and leaves the
// non-member on the full default pool.
func TestSub_ClientGroup_HostsFirst(t *testing.T) {
	seedSubDB(t)
	setHostSortMode(t, "hosts-first")
	ib := seedSubInbound(t, "s1", "cg", 4508, 1, tcpTLSStream)
	seedSubClient(t, ib, "s1", "other@e")
	group := seedClientGroup(t, "VIP")
	setClientGroup(t, "cg@e", group.Name)
	seedHost(t, &model.Host{GroupId: "ga", InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls"})
	seedHost(t, &model.Host{GroupId: "gb", InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 443, Security: "tls"})
	bindHostGroup(t, "gb", group.Id)

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	if got := strings.Count(joined, "a.cdn.com:443"); got != 1 {
		t.Fatalf("non-member must get A once, got %d:\n%s", got, joined)
	}
	if got := strings.Count(joined, "b.cdn.com:443"); got != 2 {
		t.Fatalf("member and non-member must both get B, got %d:\n%s", got, joined)
	}
}

// Raw, JSON and Clash must expose the same endpoint set for each subscriber.
func TestSub_ClientGroup_FormatParity(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "member-sub", "cg", 4509, 1, tcpTLSStream)
	seedSubClient(t, ib, "other-sub", "other@e")
	group := seedClientGroup(t, "VIP")
	setClientGroup(t, "cg@e", group.Name)
	seedHost(t, &model.Host{GroupId: "ga", InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls"})
	seedHost(t, &model.Host{GroupId: "gb", InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 443, Security: "tls"})
	bindHostGroup(t, "gb", group.Id)

	for _, subID := range []string{"member-sub", "other-sub"} {
		t.Run(subID, func(t *testing.T) {
			base := NewSubService("")
			links, _, _, _, err := base.GetSubs(subID, "req.example.com")
			if err != nil {
				t.Fatalf("GetSubs: %v", err)
			}
			raw := rawEndpointSet(t, links)
			jsonBody, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson(subID, "req.example.com", true)
			if err != nil {
				t.Fatalf("GetJson: %v", err)
			}
			js := jsonEndpointSet(t, jsonBody)
			clashBody, _, err := NewSubClashService(false, "", false, NewSubService("")).GetClash(subID, "req.example.com")
			if err != nil {
				t.Fatalf("GetClash: %v", err)
			}
			clash := clashEndpointSet(t, clashBody)
			if !reflect.DeepEqual(raw, js) || !reflect.DeepEqual(raw, clash) {
				t.Fatalf("format parity: raw=%v json=%v clash=%v", raw, js, clash)
			}
		})
	}
}

// A group binding whose host rows live only on another inbound leaves this
// inbound on the legacy path, not on an empty group tier.
func TestSub_ClientGroup_LegacyFallbackPerInbound(t *testing.T) {
	seedSubDB(t)
	group := seedClientGroup(t, "VIP")
	other := seedSubInbound(t, "s2", "other", 4511, 1, tcpTLSStream)
	seedHost(t, &model.Host{GroupId: "ga", InboundId: other.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls"})
	bindHostGroup(t, "ga", group.Id)

	seedSubInbound(t, "s1", "target", 4512, 1, tcpTLSStream)
	setClientGroup(t, "target@e", group.Name)

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 1 || !strings.Contains(links[0], "203.0.113.5:4512") {
		t.Fatalf("expected the inbound's legacy link, got: %v", links)
	}
}

// Bindings on a case-duplicate group row still reach members of the canonical
// group: the resolver folds both ids onto the first row loaded.
func TestSub_ClientGroup_CaseDuplicateBinding(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "cg", 4513, 1, tcpTLSStream)
	canonical := seedClientGroup(t, "VIP")
	duplicate := seedClientGroup(t, "vip")
	setClientGroup(t, "cg@e", canonical.Name)
	seedHost(t, &model.Host{GroupId: "ga", InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls"})
	seedHost(t, &model.Host{GroupId: "gb", InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 443, Security: "tls"})
	bindHostGroup(t, "gb", duplicate.Id)

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	if strings.Contains(joined, "a.cdn.com:443") || !strings.Contains(joined, "b.cdn.com:443") {
		t.Fatalf("case-duplicate binding must be canonicalized, got: %s", joined)
	}
}

// A group row without a binding must leave the default pool untouched.
func TestSub_ClientGroup_NoBindingKeepsDefaultPool(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "cg", 4510, 1, tcpTLSStream)
	group := seedClientGroup(t, "VIP")
	setClientGroup(t, "cg@e", group.Name)
	seedHost(t, &model.Host{GroupId: "ga", InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443, Security: "tls"})
	seedHost(t, &model.Host{GroupId: "gb", InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 443, Security: "tls"})

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	joined := strings.Join(links, "\n")
	if !strings.Contains(joined, "a.cdn.com:443") || !strings.Contains(joined, "b.cdn.com:443") {
		t.Fatalf("unbound group must keep the default pool, got: %s", joined)
	}
}
