package service

import (
	_ "embed"
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v2/logger"
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
	if stripped, removed, err := StripLegacyReverse(newXraySettings); err == nil && removed {
		newXraySettings = stripped
		logger.Warning("[xray-setting] removed legacy `reverse` block from saved template (xray-core v26+ no longer supports it; see XTLS/Xray-core#5101 for VLESS Reverse Proxy)")
	}
	if err := s.CheckXrayConfig(newXraySettings); err != nil {
		return err
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

// StripLegacyReverse removes the top-level `reverse` block from a
// saved xray template if it carries the deprecated `portals` / `bridges`
// shape that xray-core v26+ refuses to parse.
//
// Why (#4115): xray-core dropped the original reverse-proxy feature in
// favour of "VLESS Reverse Proxy" (XTLS/Xray-core#5101). After upgrading
// xray, any panel template that still contains a `reverse: { portals
// [...], bridges: [...] }` section makes the daemon fail to start with
// `The feature "legacy reverse" has been removed and migrated to "VLESS
// Reverse Proxy"`. The panel UI used to write configs in that exact old
// shape, so a lot of in-the-wild templates carry it.
//
// Stripping it here keeps the rest of the config valid, lets xray come
// back up, and makes the issue self-healing on the next save. The new
// VLESS Reverse Proxy is a per-client field on a VLESS inbound (and a
// per-outbound field on the bridge side), not a top-level block, so
// dropping the legacy block does not collide with the new mechanism.
//
// Returns the (possibly modified) JSON, a flag indicating whether
// anything was actually removed, and a parse error if the input is not
// valid JSON. The caller decides whether to log/notify on `removed`.
func StripLegacyReverse(raw string) (out string, removed bool, err error) {
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return raw, false, err
	}
	rev, ok := cfg["reverse"]
	if !ok || len(rev) == 0 {
		return raw, false, nil
	}
	var revObj map[string]json.RawMessage
	if err := json.Unmarshal(rev, &revObj); err != nil {
		// Not an object (could be `null`, an array, etc). Leave alone.
		return raw, false, nil
	}
	// The legacy shape is identified by the `portals` or `bridges`
	// fields. Anything else under `reverse` (currently nothing valid in
	// xray-core, but be conservative) we leave alone for the user to
	// inspect.
	_, hasPortals := revObj["portals"]
	_, hasBridges := revObj["bridges"]
	if !hasPortals && !hasBridges {
		return raw, false, nil
	}
	delete(cfg, "reverse")
	rebuilt, marshalErr := json.Marshal(cfg)
	if marshalErr != nil {
		return raw, false, marshalErr
	}
	return string(rebuilt), true, nil
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
