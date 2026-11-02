package service

import (
	"encoding/json"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/version"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

// inboundShadowsocksMethod extracts settings.method for Shadowsocks inbounds so
// the client UI can generate a valid PSK (base64 of the method's key length)
// for Shadowsocks 2022 ciphers. Returns "" for non-Shadowsocks inbounds.
func inboundShadowsocksMethod(protocol, settings string) string {
	if protocol != string(model.Shadowsocks) || settings == "" {
		return ""
	}
	var s struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return ""
	}
	return s.Method
}

// inboundCanEnableTlsFlow mirrors the frontend's canEnableTlsFlow: xray-core runs Vision over
// TLS/REALITY on raw TCP, or inside VLESS encryption on any transport but XDRIVE (#7182).
func inboundCanEnableTlsFlow(protocol, streamSettings, settings string) bool {
	if protocol != string(model.VLESS) {
		return false
	}
	var stream struct {
		Network  string `json:"network"`
		Security string `json:"security"`
	}
	_ = json.Unmarshal([]byte(streamSettings), &stream)
	if stream.Network == "xdrive" {
		return false
	}
	if stream.Network == "tcp" && (stream.Security == "tls" || stream.Security == "reality") {
		return true
	}
	return vlessEncryptionEnabled(settings)
}

// nodeEligibleProtocols mirrors the frontend's NODE_ELIGIBLE_PROTOCOLS. A sidecar
// protocol's row is local on the node it is pushed to, so that panel runs it.
var nodeEligibleProtocols = map[model.Protocol]bool{
	model.VLESS:       true,
	model.VMESS:       true,
	model.Trojan:      true,
	model.Shadowsocks: true,
	model.Hysteria:    true,
	model.WireGuard:   true,
	model.MTProto:     true,
	model.AmneziaWG:   true,
	model.TUIC:        true,
	model.MASQUE:      true,
}

// nodeProtocolFirstRelease is the panel release that introduced each protocol
// newer than node support itself; an older node would hand it to Xray as-is.
var nodeProtocolFirstRelease = map[model.Protocol]string{
	model.MTProto:   "v3.5.0",
	model.AmneziaWG: "v3.7.0",
	model.TUIC:      "v3.8.0",
	model.MASQUE:    "v3.9.1",
}

// checkNodeCanHostProtocol refuses assigning protocol to nodeID unless the
// protocol may live on a node and that node's panel is new enough to run it.
func checkNodeCanHostProtocol(db *gorm.DB, nodeID int, protocol model.Protocol) error {
	if !nodeEligibleProtocols[protocol] {
		return common.NewErrorf("%s inbounds cannot be assigned to a node", protocol)
	}
	firstRelease, ok := nodeProtocolFirstRelease[protocol]
	if !ok {
		return nil
	}
	var node model.Node
	if err := db.Select("id", "name", "panel_version").First(&node, nodeID).Error; err != nil {
		return err
	}
	if strings.TrimSpace(node.PanelVersion) == "" {
		return common.NewErrorf("node %q has not reported its panel version yet; %s inbounds need %s or newer",
			node.Name, protocol, firstRelease)
	}
	// A dev build reports "dev+<sha>": it tracks main, which carries every protocol.
	if cmp, ok := version.Compare(node.PanelVersion, firstRelease); ok && cmp < 0 {
		return common.NewErrorf("node %q runs panel %s; %s inbounds need %s or newer",
			node.Name, node.PanelVersion, protocol, firstRelease)
	}
	return nil
}

// vlessEncryptionEnabled reports whether a VLESS inbound has VLESS-level
// encryption (vlessenc / ML-KEM) configured. When enabled these fields hold a
// generated dotted string (e.g. "mlkem768x25519plus.native.0rtt.<key>"); "none"
// or empty means off. The value is never the literal "vlessenc" — that is the
// name of the `xray vlessenc` CLI subcommand, not a stored value.
//
// Both fields are checked: decryption is the authoritative server-side value
// xray-core reads, while encryption is stored by the panel for link generation.
// The ML-KEM/X25519 buttons set both, but accepting either keeps the gate
// working for inbounds configured via the API or raw JSON.
func vlessEncryptionEnabled(settings string) bool {
	if settings == "" {
		return false
	}
	var s struct {
		Encryption string `json:"encryption"`
		Decryption string `json:"decryption"`
	}
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return false
	}
	return vlessEncValueSet(s.Encryption) || vlessEncValueSet(s.Decryption)
}

// vlessEncValueSet reports whether a VLESS encryption/decryption field holds a
// real (generated) value rather than the "none"/empty sentinel.
func vlessEncValueSet(v string) bool {
	return v != "" && v != "none"
}

// inboundCanHostFallbacks gates the settings.fallbacks injection.
// Xray only honors fallbacks on VLESS and Trojan inbounds carried over
// TCP transport with TLS or Reality security. This is intentionally stricter
// than inboundCanEnableTlsFlow (which also accepts any vlessenc stream): fallbacks
// are a raw-TCP-only feature. VLESS inbounds with VLESS-level decryption configured
// cannot host fallbacks (Xray-core refuses fallbacks together with decryption, #7001).
func inboundCanHostFallbacks(ib *model.Inbound) bool {
	if ib == nil {
		return false
	}
	if ib.Protocol != model.VLESS && ib.Protocol != model.Trojan {
		return false
	}
	if ib.Protocol == model.VLESS && vlessDecryptionEnabled(ib.Settings) {
		return false
	}
	return streamSupportsFallbacks(ib.StreamSettings)
}

// vlessDecryptionEnabled reports whether a VLESS inbound has a real server-side
// decryption value configured (vlessenc / ML-KEM).
func vlessDecryptionEnabled(settings string) bool {
	if settings == "" {
		return false
	}
	var s struct {
		Decryption string `json:"decryption"`
	}
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return false
	}
	return vlessEncValueSet(s.Decryption)
}

// normalizeVlessSettings drops empty fallbacks from VLESS settings (matching the UI's
// dropLegacyOptionalEmpties) and refuses non-empty fallbacks when VLESS decryption is
// configured. Xray-core's infra/conf/vless.go refuses fallbacks together with decryption
// and checks Fallbacks != nil; an empty array [] saved via API would otherwise cause
// Xray-core to crash with exit status 23 on startup (#7001).
func normalizeVlessSettings(inbound *model.Inbound) error {
	if inbound == nil || inbound.Protocol != model.VLESS || strings.TrimSpace(inbound.Settings) == "" {
		return nil
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return nil
	}
	fbVal, hasFallbacks := settings["fallbacks"]
	if !hasFallbacks {
		return nil
	}

	decVal, _ := settings["decryption"].(string)
	hasDecryption := vlessEncValueSet(decVal)

	switch fb := fbVal.(type) {
	case nil:
		delete(settings, "fallbacks")
	case []any:
		if len(fb) == 0 {
			delete(settings, "fallbacks")
		} else if hasDecryption {
			return common.NewError("VLESS settings: \"fallbacks\" can not be used together with \"decryption\"")
		}
	default:
		if hasDecryption {
			return common.NewError("VLESS settings: \"fallbacks\" can not be used together with \"decryption\"")
		}
	}

	modified, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	inbound.Settings = string(modified)
	return nil
}

// streamSupportsFallbacks reports whether the stream is raw TCP carried over
// TLS or REALITY — the only transport Xray honors inbound fallbacks on (and the
// classic requirement for XTLS Vision before vlessenc).
func streamSupportsFallbacks(streamSettings string) bool {
	if streamSettings == "" {
		return false
	}
	var stream struct {
		Network  string `json:"network"`
		Security string `json:"security"`
	}
	if err := json.Unmarshal([]byte(streamSettings), &stream); err != nil {
		return false
	}
	if stream.Network != "tcp" {
		return false
	}
	return stream.Security == "tls" || stream.Security == "reality"
}

// validateTunWfpBlockLeak refuses at save the autoSystemWfpBlockLeak values xray-core 26.10.31
// (#7288) refuses when the core starts, which stops every inbound with it.
func validateTunWfpBlockLeak(protocol, settings string) error {
	if protocol != "tun" {
		return nil
	}
	var tunSettings struct {
		AutoSystemWfpBlockLeak []string `json:"autoSystemWfpBlockLeak"`
	}
	if json.Unmarshal([]byte(settings), &tunSettings) != nil {
		return nil
	}
	if err := xray.ValidateTunWfpBlockLeak(tunSettings.AutoSystemWfpBlockLeak); err != nil {
		return common.NewError("TUN autoSystemWfpBlockLeak: ", err)
	}
	return nil
}
