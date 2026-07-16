package service

import (
	"encoding/json"
)

var routingMatcherKeys = []string{
	"domain", "domains", "ip", "port", "sourcePort", "localPort", "network",
	"source", "sourceIP", "localIP", "user", "vlessRoute", "protocol", "attrs", "process",
}

func readInboundTags(raw any) []string {
	switch tags := raw.(type) {
	case []string:
		return append([]string(nil), tags...)
	case string:
		if tags == "" {
			return nil
		}
		return []string{tags}
	case []any:
		out := make([]string, 0, len(tags))
		for _, item := range tags {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func writeInboundTags(rule map[string]any, tags []string) {
	if len(tags) == 0 {
		delete(rule, "inboundTag")
		return
	}
	rule["inboundTag"] = tags
}

func ruleHasNonInboundMatchers(rule map[string]any) bool {
	for _, key := range routingMatcherKeys {
		if hasRoutingMatcherValue(rule[key]) {
			return true
		}
	}
	return false
}

func hasRoutingMatcherValue(raw any) bool {
	switch v := raw.(type) {
	case nil:
		return false
	case string:
		return v != ""
	case float64, int, int64, bool:
		return true
	case []string:
		return len(v) > 0
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return true
	}
}

func replaceInboundTagInRules(rules []map[string]any, oldTag, newTag string) bool {
	changed := false
	for _, rule := range rules {
		if replaceInboundTagInRule(rule, oldTag, newTag) {
			changed = true
		}
	}
	return changed
}

func replaceInboundTagInRule(rule map[string]any, oldTag, newTag string) bool {
	tags := readInboundTags(rule["inboundTag"])
	if len(tags) == 0 {
		return false
	}
	updated := false
	for i, tag := range tags {
		if tag == oldTag {
			tags[i] = newTag
			updated = true
		}
	}
	if updated {
		writeInboundTags(rule, tags)
	}
	return updated
}

func removeInboundTagFromRules(rules []map[string]any, deletedTag string) ([]map[string]any, bool) {
	if deletedTag == "" {
		return rules, false
	}
	changed := false
	out := make([]map[string]any, 0, len(rules))
	for _, rule := range rules {
		tags := readInboundTags(rule["inboundTag"])
		if len(tags) == 0 {
			out = append(out, rule)
			continue
		}
		nextTags := make([]string, 0, len(tags))
		hadDeleted := false
		for _, tag := range tags {
			if tag == deletedTag {
				hadDeleted = true
				continue
			}
			nextTags = append(nextTags, tag)
		}
		if !hadDeleted {
			out = append(out, rule)
			continue
		}
		changed = true
		if len(nextTags) == 0 && !ruleHasNonInboundMatchers(rule) {
			continue
		}
		if len(nextTags) == 0 {
			delete(rule, "inboundTag")
		} else {
			writeInboundTags(rule, nextTags)
		}
		out = append(out, rule)
	}
	return out, changed
}

func replaceInboundTagInOutbounds(outbounds []any, oldTag, newTag string) bool {
	changed := false
	for _, outIface := range outbounds {
		out, ok := outIface.(map[string]any)
		if !ok {
			continue
		}
		proto, _ := out["protocol"].(string)
		if proto != "loopback" {
			continue
		}
		settings, ok := out["settings"].(map[string]any)
		if !ok {
			continue
		}
		tag, _ := settings["inboundTag"].(string)
		if tag != oldTag {
			continue
		}
		settings["inboundTag"] = newTag
		changed = true
	}
	return changed
}

func removeInboundTagFromOutbounds(outbounds []any, deletedTag string) bool {
	changed := false
	for _, outIface := range outbounds {
		out, ok := outIface.(map[string]any)
		if !ok {
			continue
		}
		proto, _ := out["protocol"].(string)
		if proto != "loopback" {
			continue
		}
		settings, ok := out["settings"].(map[string]any)
		if !ok {
			continue
		}
		tag, _ := settings["inboundTag"].(string)
		if tag != deletedTag {
			continue
		}
		delete(settings, "inboundTag")
		changed = true
	}
	return changed
}

func mutateXrayTemplateRouting(raw string, mutate func(cfg map[string]any) bool) (string, bool, error) {
	raw = UnwrapXrayTemplateConfig(raw)
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return raw, false, err
	}
	if !mutate(cfg) {
		return raw, false, nil
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return raw, false, err
	}
	return string(out), true, nil
}

func routingRulesFromCfg(cfg map[string]any) []map[string]any {
	routing, _ := cfg["routing"].(map[string]any)
	if routing == nil {
		return nil
	}
	rawRules, ok := routing["rules"].([]any)
	if !ok {
		return nil
	}
	rules := make([]map[string]any, 0, len(rawRules))
	for _, item := range rawRules {
		rule, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rules = append(rules, rule)
	}
	return rules
}

func setRoutingRulesInCfg(cfg map[string]any, rules []map[string]any) {
	routing, _ := cfg["routing"].(map[string]any)
	if routing == nil {
		routing = map[string]any{}
		cfg["routing"] = routing
	}
	items := make([]any, len(rules))
	for i, rule := range rules {
		items[i] = rule
	}
	routing["rules"] = items
}

func outboundsFromCfg(cfg map[string]any) []any {
	outbounds, _ := cfg["outbounds"].([]any)
	return outbounds
}

// PropagateInboundTagRename rewrites routing rules and loopback outbound
// references when a panel inbound tag changes.
func (s *XraySettingService) PropagateInboundTagRename(oldTag, newTag string) (bool, error) {
	if oldTag == "" || newTag == "" || oldTag == newTag {
		return false, nil
	}
	template, err := s.GetXrayConfigTemplate()
	if err != nil {
		return false, err
	}
	updated, changed, err := mutateXrayTemplateRouting(template, func(cfg map[string]any) bool {
		mutated := false
		rules := routingRulesFromCfg(cfg)
		if len(rules) > 0 {
			if replaceInboundTagInRules(rules, oldTag, newTag) {
				setRoutingRulesInCfg(cfg, rules)
				mutated = true
			}
		}
		outbounds := outboundsFromCfg(cfg)
		if len(outbounds) > 0 && replaceInboundTagInOutbounds(outbounds, oldTag, newTag) {
			cfg["outbounds"] = outbounds
			mutated = true
		}
		return mutated
	})
	if err != nil || !changed {
		return false, err
	}
	if err := s.SaveXraySetting(updated); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveInboundTagReferences drops a deleted inbound tag from routing rules.
// Rules that only matched that inbound are removed; rules with additional
// matchers keep the rule and only lose the inboundTag entry.
func (s *XraySettingService) RemoveInboundTagReferences(deletedTag string) (bool, error) {
	if deletedTag == "" {
		return false, nil
	}
	template, err := s.GetXrayConfigTemplate()
	if err != nil {
		return false, err
	}
	updated, changed, err := mutateXrayTemplateRouting(template, func(cfg map[string]any) bool {
		mutated := false
		rules := routingRulesFromCfg(cfg)
		if len(rules) > 0 {
			nextRules, rulesChanged := removeInboundTagFromRules(rules, deletedTag)
			if rulesChanged {
				setRoutingRulesInCfg(cfg, nextRules)
				mutated = true
			}
		}
		outbounds := outboundsFromCfg(cfg)
		if len(outbounds) > 0 && removeInboundTagFromOutbounds(outbounds, deletedTag) {
			cfg["outbounds"] = outbounds
			mutated = true
		}
		return mutated
	})
	if err != nil || !changed {
		return false, err
	}
	if err := s.SaveXraySetting(updated); err != nil {
		return false, err
	}
	return true, nil
}
