package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

// rulesOf unmarshals a router config and returns its rules for assertions.
func rulesOf(t *testing.T, raw json_util.RawMessage) []map[string]any {
	t.Helper()
	var parsed struct {
		Rules []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return parsed.Rules
}

func TestStripDisabledRules(t *testing.T) {
	t.Run("empty config is returned untouched", func(t *testing.T) {
		if got := stripDisabledRules(nil); got != nil {
			t.Fatalf("expected nil passthrough, got %s", got)
		}
	})

	t.Run("missing or empty rules is a no-op", func(t *testing.T) {
		in := json_util.RawMessage(`{"domainStrategy":"AsIs"}`)
		if got := stripDisabledRules(in); string(got) != string(in) {
			t.Fatalf("config without rules was modified: %s", got)
		}
	})

	t.Run("drops disabled rules and strips the enabled key from the rest", func(t *testing.T) {
		in := json_util.RawMessage(`{"rules":[
			{"outboundTag":"direct","domain":["a.com"],"enabled":true},
			{"outboundTag":"block","domain":["b.com"],"enabled":false},
			{"outboundTag":"proxy","domain":["c.com"]}
		]}`)
		rules := rulesOf(t, stripDisabledRules(in))
		if len(rules) != 2 {
			t.Fatalf("expected 2 active rules, got %d: %v", len(rules), rules)
		}
		for _, r := range rules {
			if _, ok := r["enabled"]; ok {
				t.Fatalf("enabled key must not survive into the runtime config: %v", r)
			}
		}
		if rules[0]["outboundTag"] != "direct" || rules[1]["outboundTag"] != "proxy" {
			t.Fatalf("kept rules or their order are wrong: %v", rules)
		}
	})

	t.Run("never drops the api rule even when marked disabled", func(t *testing.T) {
		in := json_util.RawMessage(`{"rules":[
			{"inboundTag":["api"],"outboundTag":"api","enabled":false},
			{"outboundTag":"block","domain":["b.com"],"enabled":false}
		]}`)
		rules := rulesOf(t, stripDisabledRules(in))
		if len(rules) != 1 {
			t.Fatalf("expected only the api rule to survive, got %d: %v", len(rules), rules)
		}
		if rules[0]["outboundTag"] != "api" {
			t.Fatalf("api rule was dropped: %v", rules)
		}
		if _, ok := rules[0]["enabled"]; ok {
			t.Fatalf("enabled key must be stripped from the api rule too: %v", rules[0])
		}
	})

	t.Run("non-object rules pass through, disabled object is dropped", func(t *testing.T) {
		in := json_util.RawMessage(`{"rules":["weird",{"outboundTag":"block","enabled":false}]}`)
		var parsed struct {
			Rules []any `json:"rules"`
		}
		if err := json.Unmarshal(stripDisabledRules(in), &parsed); err != nil {
			t.Fatal(err)
		}
		if len(parsed.Rules) != 1 {
			t.Fatalf("expected 1 surviving rule (the string), got %v", parsed.Rules)
		}
		if s, _ := parsed.Rules[0].(string); s != "weird" {
			t.Fatalf("non-object rule should be preserved, got %v", parsed.Rules[0])
		}
	})
}
