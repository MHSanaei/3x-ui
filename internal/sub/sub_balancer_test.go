package sub

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func seedSubBalancer(t *testing.T, b *model.SubBalancer) *model.SubBalancer {
	t.Helper()
	if err := database.GetDB().Create(b).Error; err != nil {
		t.Fatalf("seed balancer: %v", err)
	}
	return b
}

func parseSubJsonDocs(t *testing.T, out string) []map[string]any {
	t.Helper()
	var docs []map[string]any
	if err := json.Unmarshal([]byte(out), &docs); err != nil {
		t.Fatalf("subscription is not a JSON array: %v\n%s", err, out)
	}
	return docs
}

func docOutboundTags(doc map[string]any) []string {
	outbounds, _ := doc["outbounds"].([]any)
	tags := make([]string, 0, len(outbounds))
	for _, ob := range outbounds {
		if m, ok := ob.(map[string]any); ok {
			tags = append(tags, m["tag"].(string))
		}
	}
	return tags
}

func findDocByRemarks(docs []map[string]any, remarks string) map[string]any {
	for _, doc := range docs {
		if doc["remarks"] == remarks {
			return doc
		}
	}
	return nil
}

// The balancer document mirrors the reference model: members retagged under a
// per-balancer prefix, a balancer selecting that prefix, a burst observatory
// probing it, and proxy rules pointed at the balancer — while the manual
// documents keep their plain "proxy" routing untouched.
func TestSubJson_BalancerDocument(t *testing.T) {
	seedSubDB(t)
	tcp := seedSubInbound(t, "s1", "tcpin", 4701, 1, `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`)
	ws := seedSubInbound(t, "s1", "wsin", 4702, 2, wsTLSStream)
	seedSubBalancer(t, &model.SubBalancer{
		Remark: "auto", Strategy: "leastLoad", InboundIds: []int{tcp.Id, ws.Id}, SortOrder: 1, Enabled: true,
	})

	rules := `[{"type":"field","domain":["geosite:example"],"outboundTag":"proxy"}]`
	js := NewSubJsonService("", rules, "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	if len(docs) != 3 {
		t.Fatalf("docs = %d, want 3 (2 inbounds + 1 balancer):\n%s", len(docs), out)
	}

	balancerDoc := findDocByRemarks(docs, "auto")
	if balancerDoc == nil {
		t.Fatalf("balancer doc missing:\n%s", out)
	}
	if tags := docOutboundTags(balancerDoc); strings.Join(tags, ",") != "bal-1-vless,bal-1-ws,direct,block" {
		t.Fatalf("balancer outbound tags = %v", tags)
	}

	routing, _ := balancerDoc["routing"].(map[string]any)
	balancers, _ := routing["balancers"].([]any)
	if len(balancers) != 1 {
		t.Fatalf("balancers = %d, want 1", len(balancers))
	}
	balancer, _ := balancers[0].(map[string]any)
	if balancer["tag"] != "balancer" {
		t.Fatalf("balancer tag = %v", balancer["tag"])
	}
	if selector, _ := balancer["selector"].([]any); strings.Join(stringify(selector), ",") != "bal-1-" {
		t.Fatalf("selector = %v", selector)
	}
	strategy, _ := balancer["strategy"].(map[string]any)
	if strategy["type"] != "leastLoad" {
		t.Fatalf("strategy = %v", strategy)
	}

	ruleJSON, _ := json.Marshal(routing["rules"])
	if strings.Contains(string(ruleJSON), `"outboundTag":"proxy"`) {
		t.Fatalf("balancer rules must not point at the plain proxy tag: %s", ruleJSON)
	}
	if !strings.Contains(string(ruleJSON), `"balancerTag":"balancer"`) {
		t.Fatalf("balancer catch-all rule missing balancerTag: %s", ruleJSON)
	}
	proxyRules := strings.Count(string(ruleJSON), `"balancerTag"`)
	if proxyRules != 2 { // custom rule + default catch-all
		t.Fatalf("balancerTag rules = %d, want 2: %s", proxyRules, ruleJSON)
	}

	observatory, _ := balancerDoc["burstObservatory"].(map[string]any)
	if selector, _ := observatory["subjectSelector"].([]any); strings.Join(stringify(selector), ",") != "bal-1-" {
		t.Fatalf("subjectSelector = %v", selector)
	}
	ping, _ := observatory["pingConfig"].(map[string]any)
	if ping["destination"] != subBalancerProbeURL {
		t.Fatalf("pingConfig destination = %v", ping["destination"])
	}

	// The routing rewrite must not leak into the manual documents: s.configJson
	// is shared, so a missing clone would corrupt every other doc.
	for _, remarks := range []string{"tcpin-tcpin@e", "wsin-wsin@e"} {
		manual := findDocByRemarks(docs, remarks)
		if manual == nil {
			t.Fatalf("manual doc %q missing:\n%s", remarks, out)
		}
		if tags := docOutboundTags(manual); tags[0] != "proxy" {
			t.Fatalf("manual doc %q first tag = %q, want proxy", remarks, tags[0])
		}
		manualRouting, _ := manual["routing"].(map[string]any)
		manualRules, _ := json.Marshal(manualRouting["rules"])
		if !strings.Contains(string(manualRules), `"outboundTag":"proxy"`) {
			t.Fatalf("manual doc %q lost its proxy rule: %s", remarks, manualRules)
		}
		if _, has := manualRouting["balancers"]; has {
			t.Fatalf("manual doc %q must not carry balancers", remarks)
		}
	}
}

func stringify(values []any) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, v.(string))
	}
	return out
}

// The balancer interleaves with inbounds by the same 1-based number and, on a
// tie, follows the inbound group with that number.
func TestSubJson_BalancerOrderInterleavesWithInbounds(t *testing.T) {
	seedSubDB(t)
	later := seedSubInbound(t, "s1", "later", 4711, 2, wsTLSStream)
	first := seedSubInbound(t, "s1", "first", 4712, 1, wsTLSStream)
	seedSubBalancer(t, &model.SubBalancer{
		Remark: "bal", Strategy: "roundRobin", InboundIds: []int{later.Id, first.Id}, SortOrder: 1, Enabled: true,
	})

	js := NewSubJsonService("", "", "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	var remarks []string
	for _, doc := range docs {
		remarks = append(remarks, doc["remarks"].(string))
	}
	if strings.Join(remarks, ",") != "first-first@e,bal,later-later@e" {
		t.Fatalf("doc order = %v, want [first bal later]", remarks)
	}
	balancerDoc := findDocByRemarks(docs, "bal")
	routing, _ := balancerDoc["routing"].(map[string]any)
	balancers, _ := routing["balancers"].([]any)
	strategy, _ := balancers[0].(map[string]any)["strategy"].(map[string]any)
	if strategy["type"] != "roundRobin" {
		t.Fatalf("strategy = %v, want roundRobin", strategy["type"])
	}
}

// A disabled balancer is not emitted; an enabled one whose selected inbounds
// have no configs for this subscriber is skipped rather than emitted empty.
func TestSubJson_BalancerDisabledAndEmptySkipped(t *testing.T) {
	seedSubDB(t)
	inbound := seedSubInbound(t, "s1", "only", 4721, 1, wsTLSStream)
	seedSubBalancer(t, &model.SubBalancer{
		Remark: "off", Strategy: "random", InboundIds: []int{inbound.Id}, SortOrder: 1, Enabled: false,
	})
	seedSubBalancer(t, &model.SubBalancer{
		Remark: "nomembers", Strategy: "random", InboundIds: []int{inbound.Id + 100}, SortOrder: 1, Enabled: true,
	})

	js := NewSubJsonService("", "", "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	if len(docs) != 1 {
		t.Fatalf("docs = %d, want 1:\n%s", len(docs), out)
	}
	if docs[0]["remarks"] != "only-only@e" {
		t.Fatalf("remaining doc = %v", docs[0]["remarks"])
	}
}

// Two members sharing a transport get deduplicated tags (…-2 suffix), matching
// the reference makeTag convention.
func TestSubJson_BalancerTagDedup(t *testing.T) {
	seedSubDB(t)
	a := seedSubInbound(t, "s1", "wsa", 4731, 1, wsTLSStream)
	b := seedSubInbound(t, "s1", "wsb", 4732, 2, wsTLSStream)
	seedSubBalancer(t, &model.SubBalancer{
		Remark: "dedup", Strategy: "leastPing", InboundIds: []int{a.Id, b.Id}, SortOrder: 1, Enabled: true,
	})

	js := NewSubJsonService("", "", "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	balancerDoc := findDocByRemarks(docs, "dedup")
	if balancerDoc == nil {
		t.Fatalf("balancer doc missing:\n%s", out)
	}
	if tags := docOutboundTags(balancerDoc); strings.Join(tags, ",") != "bal-1-ws,bal-1-ws-2,direct,block" {
		t.Fatalf("balancer outbound tags = %v", tags)
	}
}

// random/roundRobin have no fallback so they emit no observatory; leastPing
// carries one, with the panel-wide ping config overriding the defaults.
func TestSubJson_BalancerObservatoryConditional(t *testing.T) {
	seedSubDB(t)
	rr := seedSubInbound(t, "s1", "rr", 4741, 1, wsTLSStream)
	lp := seedSubInbound(t, "s1", "lp", 4742, 2, wsTLSStream)
	seedSubBalancer(t, &model.SubBalancer{
		Remark: "rnd", Strategy: "random", InboundIds: []int{rr.Id}, SortOrder: 1, Enabled: true,
	})
	seedSubBalancer(t, &model.SubBalancer{
		Remark: "pinger", Strategy: "leastPing", InboundIds: []int{lp.Id}, SortOrder: 2, Enabled: true,
	})

	js := NewSubJsonService("", "", "", NewSubService(""))
	js.SetObservatoryConfig(`{"destination":"https://probe.example/204","httpMethod":"GET","sampling":5}`)
	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)

	rnd := findDocByRemarks(docs, "rnd")
	if _, has := rnd["burstObservatory"]; has {
		t.Fatalf("random balancer must not emit burstObservatory: %v", rnd["burstObservatory"])
	}

	pinger := findDocByRemarks(docs, "pinger")
	obs, _ := pinger["burstObservatory"].(map[string]any)
	if obs == nil {
		t.Fatalf("leastPing balancer must emit burstObservatory:\n%s", out)
	}
	ping, _ := obs["pingConfig"].(map[string]any)
	if ping["destination"] != "https://probe.example/204" {
		t.Fatalf("destination = %v, want custom probe URL", ping["destination"])
	}
	if ping["httpMethod"] != "GET" {
		t.Fatalf("httpMethod = %v, want GET", ping["httpMethod"])
	}
	if ping["sampling"] != float64(5) {
		t.Fatalf("sampling = %v, want 5", ping["sampling"])
	}
	if ping["interval"] != "1m" {
		t.Fatalf("interval = %v, want default 1m", ping["interval"])
	}
}
