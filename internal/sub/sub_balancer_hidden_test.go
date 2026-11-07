package sub

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func hideInbound(t *testing.T, ib *model.Inbound) {
	t.Helper()
	if err := database.GetDB().Model(ib).Update("exclude_from_sub", true).Error; err != nil {
		t.Fatalf("hide inbound: %v", err)
	}
}

func docsMentioning(t *testing.T, docs []map[string]any, needle string) []string {
	t.Helper()
	var remarks []string
	for _, doc := range docs {
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("marshal doc: %v", err)
		}
		if strings.Contains(string(raw), needle) {
			remarks = append(remarks, doc["remarks"].(string))
		}
	}
	return remarks
}

// A balancer may select an inbound that is hidden from the subscription: the
// inbound contributes a member outbound to the balancer doc only.
func TestSubJson_BalancerMemberFromHiddenInbound(t *testing.T) {
	for _, mode := range []string{"legacy", "hosts-first"} {
		t.Run(mode, func(t *testing.T) {
			seedSubDB(t)
			setHostSortMode(t, mode)
			shown := seedSubInbound(t, "s1", "shown", 4711, 2, tcpTLSStream)
			hidden := seedSubInbound(t, "s1", "hidden", 4712, 1, wsTLSStream)
			hideInbound(t, hidden)
			seedSubBalancer(t, &model.SubBalancer{
				Remark: "auto", Strategy: "random", InboundIds: []int{shown.Id, hidden.Id}, SortOrder: 1, Enabled: true,
			})

			out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", true)
			if err != nil {
				t.Fatalf("GetJson: %v", err)
			}
			docs := parseSubJsonDocs(t, out)
			if len(docs) != 2 {
				t.Fatalf("docs = %d, want 2 (shown inbound + balancer):\n%s", len(docs), out)
			}
			balancerDoc := findDocByRemarks(docs, "auto")
			if balancerDoc == nil {
				t.Fatalf("balancer doc missing:\n%s", out)
			}
			if tags := strings.Join(docOutboundTags(balancerDoc), ","); tags != "bal-1-vless,bal-1-vless-2,direct,block" {
				t.Fatalf("balancer outbound tags = %s, want both members", tags)
			}
			if holders := docsMentioning(t, docs, "4712"); len(holders) != 1 || holders[0] != "auto" {
				t.Fatalf("hidden inbound must appear only inside the balancer doc, found in %v", holders)
			}
		})
	}
}

// A balancer whose only member is hidden still renders.
func TestSubJson_BalancerOfOnlyHiddenInbound(t *testing.T) {
	seedSubDB(t)
	hidden := seedSubInbound(t, "s1", "hidden", 4715, 1, tcpTLSStream)
	hideInbound(t, hidden)
	seedSubBalancer(t, &model.SubBalancer{
		Remark: "solo", Strategy: "random", InboundIds: []int{hidden.Id}, SortOrder: 1, Enabled: true,
	})

	out, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	docs := parseSubJsonDocs(t, out)
	if len(docs) != 1 || docs[0]["remarks"] != "solo" {
		t.Fatalf("want only the balancer doc, got:\n%s", out)
	}
}

// Rendering a hidden member must not use up the first-link-only remark tokens:
// the first inbound the client actually sees still carries them.
func TestSubJson_HiddenBalancerMemberKeepsFirstLinkRemark(t *testing.T) {
	seedSubDB(t)
	hidden := seedSubInbound(t, "s1", "hidden", 4716, 1, tcpTLSStream)
	seedSubInbound(t, "s1", "shown", 4717, 2, wsTLSStream)
	hideInbound(t, hidden)
	seedSubBalancer(t, &model.SubBalancer{
		Remark: "auto", Strategy: "random", InboundIds: []int{hidden.Id}, SortOrder: 9, Enabled: true,
	})

	svc := NewSubService("{{INBOUND}}-{{EMAIL}}")
	out, _, err := NewSubJsonService("", "", "", "", svc).GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	shownDoc := findDocByRemarks(parseSubJsonDocs(t, out), "shown-shown@e")
	if shownDoc == nil {
		t.Fatalf("shown inbound lost its first-link remark (email token spent on the hidden member):\n%s", out)
	}
}
