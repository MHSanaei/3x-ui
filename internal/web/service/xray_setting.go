package service

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"net/netip"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// XraySettingService provides business logic for Xray configuration management.
// It handles validation and storage of Xray template configurations.
type XraySettingService struct {
	SettingService
}

const (
	unencryptedOutboundProhibitedError = "without TLS or other encryption is prohibited unless the server address is a private IP or domain"
	unencryptedOutboundMinimumVersion  = "26.7.11"
	narrowedPrivateIPMinimumVersion    = "26.10.29"
)

// legacyPrivateIPPrefixes is the "private" set xray-core 26.7.11 to 26.10.28 let an unencrypted
// vless/trojan outbound reach; 26.10.29 (#7249) narrowed it to RFC 1918, loopback and link-local.
var legacyPrivateIPPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/3"),
	netip.MustParsePrefix("::/127"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
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
	if synced, err := EnsureDnsServerRouting(newXraySettings); err == nil {
		newXraySettings = synced
	}
	if spelled, changed, err := database.RewriteDNSOutboundQTypeZero(newXraySettings); err == nil && changed {
		newXraySettings = spelled
	}
	return s.saveSetting("xrayTemplateConfig", newXraySettings)
}

func (s *XraySettingService) CheckXrayConfig(XrayTemplateConfig string) error {
	xrayConfig := &xray.Config{}
	err := json.Unmarshal([]byte(XrayTemplateConfig), xrayConfig)
	if err != nil {
		return common.NewError("xray template config invalid:", err)
	}
	if err := checkTemplateLuaScripts(xrayConfig); err != nil {
		return err
	}
	if err := checkTemplateGeodata([]byte(xrayConfig.Geodata)); err != nil {
		return err
	}
	for _, inbound := range xrayConfig.InboundConfigs {
		if err := validateFinalMaskSalamander(string(inbound.StreamSettings)); err != nil {
			return common.NewError("xray template inbound \""+inbound.Tag+"\":", err)
		}
		if err := validateXDriveStorage(string(inbound.StreamSettings)); err != nil {
			return common.NewError("xray template inbound \""+inbound.Tag+"\":", err)
		}
		if err := validateXDriveInbound(inbound.Protocol, string(inbound.Settings), string(inbound.StreamSettings)); err != nil {
			return common.NewError("xray template inbound \""+inbound.Tag+"\":", err)
		}
		if err := validateTunWfpBlockLeak(inbound.Protocol, string(inbound.Settings)); err != nil {
			return common.NewError("xray template inbound \""+inbound.Tag+"\":", err)
		}
	}
	if len(xrayConfig.OutboundConfigs) > 0 {
		var outbounds []json.RawMessage
		if err := json.Unmarshal(xrayConfig.OutboundConfigs, &outbounds); err != nil {
			return common.NewError("xray template config invalid: outbounds is not an array:", err)
		}
		coreVersion := "Unknown"
		if process := currentXrayProcess(); process != nil {
			coreVersion = process.GetXrayVersion()
		}
		for _, outbound := range outbounds {
			// Panel pseudo-protocol: validated panel-side because the core's
			// loader would reject it outright.
			if amneziawg.IsAmneziaWGOutbound(outbound) {
				var probe struct {
					Tag string `json:"tag"`
				}
				if err := json.Unmarshal(outbound, &probe); err != nil {
					return common.NewError("xray template config invalid: amneziawg outbound tag unreadable:", err)
				}
				if err := amneziawg.ValidateAmneziaWGOutbound(probe.Tag, outbound); err != nil {
					return err
				}
				continue
			}
			var probe struct {
				Tag            string          `json:"tag"`
				StreamSettings json.RawMessage `json:"streamSettings"`
			}
			_ = json.Unmarshal(outbound, &probe)
			if err := validateFinalMaskSalamander(string(probe.StreamSettings)); err != nil {
				return common.NewError("xray template outbound \""+probe.Tag+"\":", err)
			}
			if err := validateXDriveStorage(string(probe.StreamSettings)); err != nil {
				return common.NewError("xray template outbound \""+probe.Tag+"\":", err)
			}
			if err := xray.ValidateOutboundConfig(outbound); err != nil {
				if shouldSkipLegacyUnencryptedOutboundRejection(coreVersion, outbound, err) {
					continue
				}
				tagged := struct {
					Tag string `json:"tag"`
				}{}
				_ = json.Unmarshal(outbound, &tagged)
				return common.NewError("xray core rejects outbound \""+tagged.Tag+"\":", err)
			}
		}
	}
	return nil
}

// checkTemplateLuaScripts refuses the Lua scripts xray-core 26.10.17 refuses (#7136): any routing
// "script", since SaveXraySetting always keeps the api rule, and a dns "script" beside refused settings.
func checkTemplateLuaScripts(cfg *xray.Config) error {
	var routing map[string]any
	if json.Unmarshal(cfg.RouterConfig, &routing) == nil {
		if script, _ := routing["script"].(string); script != "" {
			return common.NewError(`xray template routing: "script" cannot be used, xray-core refuses it beside routing rules and the panel always keeps its api rule.`)
		}
	}
	var dns map[string]any
	if json.Unmarshal(cfg.DNSConfig, &dns) == nil && dnsScriptRefused(dns) {
		return common.NewError(`xray template dns: "script" cannot be used with disableFallback, disableFallbackIfMatch, enableParallelQuery, or a server's domains, skipFallback or finalQuery.`)
	}
	return nil
}

// checkTemplateGeodata applies the checks xray-core 26.10.29 (#7267) runs on a scheduled geodata
// section only when the core starts, where a refusal stops every inbound.
func checkTemplateGeodata(raw []byte) error {
	var geodata struct {
		Cron   string `json:"cron"`
		Assets []struct {
			File string `json:"file"`
		} `json:"assets"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &geodata) != nil || geodata.Cron == "" {
		return nil
	}
	if len(geodata.Assets) > 100 {
		return common.NewError("xray template geodata: xray-core supports at most 100 assets.")
	}
	for _, asset := range geodata.Assets {
		if !strings.EqualFold(filepath.Ext(asset.File), ".dat") {
			return common.NewError(`xray template geodata: file extension must be .dat, not "` + asset.File + `".`)
		}
	}
	schedule, err := cron.ParseStandard(geodata.Cron)
	if err != nil {
		return common.NewError("xray template geodata: invalid geodata cron:", err)
	}
	if interval, ok := schedule.(cron.ConstantDelaySchedule); ok && interval.Delay < time.Minute {
		return common.NewError("xray template geodata: the cron interval must be at least 1 minute.")
	}
	return nil
}

// shouldSkipLegacyUnencryptedOutboundRejection keeps an unencrypted outbound the running core still
// allows: any address before 26.7.11, a legacy-private IP before 26.10.29; unknown versions stay strict.
func shouldSkipLegacyUnencryptedOutboundRejection(coreVersion string, outbound []byte, err error) bool {
	if err == nil || !strings.Contains(err.Error(), unencryptedOutboundProhibitedError) {
		return false
	}
	if comparison, ok := compareXrayCoreVersions(coreVersion, unencryptedOutboundMinimumVersion); !ok {
		return false
	} else if comparison < 0 {
		return true
	}
	comparison, _ := compareXrayCoreVersions(coreVersion, narrowedPrivateIPMinimumVersion)
	if comparison >= 0 {
		return false
	}
	addr, parseErr := netip.ParseAddr(unencryptedOutboundServerAddress(outbound))
	if parseErr != nil {
		return false
	}
	addr = addr.Unmap()
	return slices.ContainsFunc(legacyPrivateIPPrefixes, func(p netip.Prefix) bool { return p.Contains(addr) })
}

// unencryptedOutboundServerAddress reads the server address of a vless or trojan outbound in
// any of the shapes the core loader accepts: flat, vless "vnext" or trojan "servers".
func unencryptedOutboundServerAddress(outbound []byte) string {
	type server struct {
		Address string `json:"address"`
	}
	var probe struct {
		Settings struct {
			Address string   `json:"address"`
			Vnext   []server `json:"vnext"`
			Servers []server `json:"servers"`
		} `json:"settings"`
	}
	if json.Unmarshal(outbound, &probe) != nil {
		return ""
	}
	switch {
	case probe.Settings.Address != "":
		return probe.Settings.Address
	case len(probe.Settings.Vnext) > 0:
		return probe.Settings.Vnext[0].Address
	case len(probe.Settings.Servers) > 0:
		return probe.Settings.Servers[0].Address
	}
	return ""
}

func compareXrayCoreVersions(a, b string) (int, bool) {
	aParts, okA := parseXrayCoreVersionParts(a)
	bParts, okB := parseXrayCoreVersionParts(b)
	if !okA || !okB {
		return 0, false
	}
	for i := range len(aParts) {
		if aParts[i] > bParts[i] {
			return 1, true
		}
		if aParts[i] < bParts[i] {
			return -1, true
		}
	}
	return 0, true
}

func parseXrayCoreVersionParts(version string) ([3]int, bool) {
	var result [3]int
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(version), "v"), ".")
	if len(parts) != len(result) {
		return result, false
	}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return result, false
		}
		result[i] = n
	}
	return result, true
}

func (s *XraySettingService) UpdateWarpXraySetting(warpData map[string]string, warpConfig map[string]any) error {
	template, err := s.GetXrayConfigTemplate()
	if err != nil {
		return err
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(template), &cfg); err != nil {
		return err
	}

	outbounds, ok := cfg["outbounds"].([]any)
	if !ok {
		return nil
	}

	updated := false
	for _, outIface := range outbounds {
		out, ok := outIface.(map[string]any)
		if !ok {
			continue
		}
		if tag, ok := out["tag"].(string); ok && tag == "warp" {
			settings, ok := out["settings"].(map[string]any)
			if !ok {
				continue
			}

			settings["secretKey"] = warpData["private_key"]

			if conf, ok := warpConfig["config"].(map[string]any); ok {
				if iface, ok := conf["interface"].(map[string]any); ok {
					if addrs, ok := iface["addresses"].(map[string]any); ok {
						var addrList []string
						if v4, ok := addrs["v4"].(string); ok && v4 != "" {
							addrList = append(addrList, v4+"/32")
						}
						if v6, ok := addrs["v6"].(string); ok && v6 != "" {
							addrList = append(addrList, v6+"/128")
						}
						settings["address"] = addrList
					}
				}

				var clientId string
				if id, ok := conf["client_id"].(string); ok {
					clientId = id
				} else if id, ok := warpData["client_id"]; ok {
					clientId = id
				}
				if clientId != "" {
					decoded, _ := base64.StdEncoding.DecodeString(clientId)
					var res []int
					for _, b := range decoded {
						res = append(res, int(b))
					}
					settings["reserved"] = res
				}

				if peers, ok := conf["peers"].([]any); ok && len(peers) > 0 {
					if peer, ok := peers[0].(map[string]any); ok {
						if pSettings, ok := settings["peers"].([]any); ok && len(pSettings) > 0 {
							if pSet, ok := pSettings[0].(map[string]any); ok {
								pSet["publicKey"] = peer["public_key"]
								if endpoint, ok := peer["endpoint"].(map[string]any); ok {
									pSet["endpoint"] = endpoint["host"]
								}
							}
						}
					}
				}
			}
			updated = true
			break
		}
	}

	if updated {
		outJSON, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		return s.SaveXraySetting(string(outJSON))
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
	for range maxDepth {
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
	delete(apiRule, "enabled")
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

// isApiRule reports whether a routing rule targets the internal api inbound
// (inboundTag contains "api" and outboundTag is "api").
func isApiRule(rule map[string]any) bool {
	if outTag, _ := rule["outboundTag"].(string); outTag != "api" {
		return false
	}
	raw, ok := rule["inboundTag"]
	if !ok {
		return false
	}
	// inboundTag is usually []string but can come as []any from a
	// roundtrip through map[string]any. Accept both shapes.
	switch tags := raw.(type) {
	case []any:
		for _, t := range tags {
			if s, ok := t.(string); ok && s == "api" {
				return true
			}
		}
	case []string:
		if slices.Contains(tags, "api") {
			return true
		}
	case string:
		if tags == "api" {
			return true
		}
	}
	return false
}

// findApiRule returns the index of the routing rule that targets the
// internal api inbound, or -1 if no such rule exists.
func findApiRule(rules []map[string]any) int {
	for i, rule := range rules {
		if isApiRule(rule) {
			return i
		}
	}
	return -1
}
