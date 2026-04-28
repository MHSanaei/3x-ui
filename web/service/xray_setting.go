package service

import (
	_ "embed"
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

// XraySettingService provides business logic for Xray configuration management.
// It handles validation and storage of Xray template configurations.
type XraySettingService struct {
	SettingService
}

func (s *XraySettingService) SaveXraySetting(newXraySettings string) error {
	// The frontend round-trips the whole getXraySetting response back
	// through the textarea, so if it has ever received a wrapped
	// payload (see UnwrapXrayTemplateConfig) it sends that same wrapper
	// back here. Strip it before validation/storage, otherwise we save
	// garbage the next read can't recover from without this same call.
	newXraySettings = UnwrapXrayTemplateConfig(newXraySettings)
	if err := s.CheckXrayConfig(newXraySettings); err != nil {
		return err
	}
	if hoisted, err := EnsureStatsRouting(newXraySettings); err == nil {
		newXraySettings = hoisted
	}
	return s.SettingService.saveSetting("xrayTemplateConfig", newXraySettings)
}

func (s *XraySettingService) CheckXrayConfig(XrayTemplateConfig string) error {
	xrayConfig := &xray.Config{}
	err := json.Unmarshal([]byte(XrayTemplateConfig), xrayConfig)
	if err != nil {
		return common.NewError("xray template config invalid:", err)
	}
	return nil
}

// UnwrapXrayTemplateConfig returns the raw xray config JSON from `raw`,
// peeling off any number of `{ "inboundTags": ..., "outboundTestUrl": ...,
// "xraySetting": <real config> }` response-shaped wrappers that may have
// ended up in the database.
//
// How it got there: getXraySetting used to embed the raw DB value as
// `xraySetting` in its response without checking whether the stored
// value was already that exact response shape. If the frontend then
// saved it verbatim (the textarea is a round-trip of the JSON it was
// handed), the wrapper got persisted — and each subsequent save nested
// another layer, producing the blank Xray Settings page reported in
// issue #4059.
//
// If `raw` does not look like a wrapper, it is returned unchanged.
func UnwrapXrayTemplateConfig(raw string) string {
	const maxDepth = 8 // defensive cap against pathological multi-nest values
	for i := 0; i < maxDepth; i++ {
		var top map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &top); err != nil {
			return raw
		}
		inner, ok := top["xraySetting"]
		if !ok {
			return raw
		}
		// Real xray configs never contain a top-level "xraySetting" key,
		// but they do contain things like "inbounds"/"outbounds"/"api".
		// If any of those are present, we're already at the real config
		// and the "xraySetting" field is either user data or coincidence
		// — don't touch it.
		for _, k := range []string{"inbounds", "outbounds", "routing", "api", "dns", "log", "policy", "stats"} {
			if _, hit := top[k]; hit {
				return raw
			}
		}
		// Peel off one layer.
		unwrapped := string(inner)
		// `xraySetting` may be stored either as a JSON object or as a
		// JSON-encoded string of an object. Handle both.
		var asStr string
		if err := json.Unmarshal(inner, &asStr); err == nil {
			unwrapped = asStr
		}
		raw = unwrapped
	}
	return raw
}

// EnsureStatsRouting hoists the `api -> api` routing rule to the front
// of routing.rules so the stats query path is never starved by a
// catch-all rule the admin may have added or reordered above it.
//
// Why this matters (#4113, #2818): an admin who adds a cascade outbound
// (e.g. vless to another server) and a routing rule sending all inbound
// traffic to it ends up sending the internal stats inbound's traffic to
// that cascade too, since rules are evaluated top-to-bottom and the
// catch-all matches first. The panel's gRPC stats query then can't reach
// the running xray instance, GetTraffic returns nothing, and every
// client appears offline with zero traffic even though the actual proxy
// path works fine.
//
// The api inbound is special-cased internal infrastructure for the
// panel, not something the admin should ever route to a real outbound.
// Keeping its rule pinned at index 0 is the only correct configuration.
//
// If the api rule is already at index 0 the input is returned unchanged.
// If it exists somewhere else it is moved. If it is missing entirely a
// default rule (`type=field, inboundTag=[api], outboundTag=api`) is
// inserted at the front. Other routing entries keep their relative order.
func EnsureStatsRouting(raw string) (string, error) {
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return raw, err
	}

	var routing map[string]json.RawMessage
	if r, ok := cfg["routing"]; ok && len(r) > 0 {
		if err := json.Unmarshal(r, &routing); err != nil {
			return raw, err
		}
	}
	if routing == nil {
		routing = make(map[string]json.RawMessage)
	}

	var rules []map[string]any
	if r, ok := routing["rules"]; ok && len(r) > 0 {
		if err := json.Unmarshal(r, &rules); err != nil {
			return raw, err
		}
	}

	apiIdx := findApiRule(rules)
	if apiIdx == 0 {
		return raw, nil // already correct, don't churn the JSON
	}

	var apiRule map[string]any
	if apiIdx > 0 {
		apiRule = rules[apiIdx]
		rules = append(rules[:apiIdx], rules[apiIdx+1:]...)
	} else {
		apiRule = map[string]any{
			"type":        "field",
			"inboundTag":  []string{"api"},
			"outboundTag": "api",
		}
	}
	rules = append([]map[string]any{apiRule}, rules...)

	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return raw, err
	}
	routing["rules"] = rulesJSON

	routingJSON, err := json.Marshal(routing)
	if err != nil {
		return raw, err
	}
	cfg["routing"] = routingJSON

	out, err := json.Marshal(cfg)
	if err != nil {
		return raw, err
	}
	return string(out), nil
}

// findApiRule returns the index of the routing rule that targets the
// internal api inbound (inboundTag contains "api" and outboundTag is
// "api"), or -1 if no such rule exists.
func findApiRule(rules []map[string]any) int {
	for i, rule := range rules {
		if outTag, _ := rule["outboundTag"].(string); outTag != "api" {
			continue
		}
		raw, ok := rule["inboundTag"]
		if !ok {
			continue
		}
		// inboundTag is usually []string but can come as []any from a
		// roundtrip through map[string]any. Accept both shapes.
		switch tags := raw.(type) {
		case []any:
			for _, t := range tags {
				if s, ok := t.(string); ok && s == "api" {
					return i
				}
			}
		case []string:
			for _, s := range tags {
				if s == "api" {
					return i
				}
			}
		case string:
			if tags == "api" {
				return i
			}
		}
	}
	return -1
}
