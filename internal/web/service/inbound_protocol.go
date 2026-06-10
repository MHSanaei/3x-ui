package service

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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

// inboundCanEnableTlsFlow mirrors Inbound.canEnableTlsFlow() from the frontend:
// XTLS Vision is only valid for VLESS on TCP with tls or reality.
func inboundCanEnableTlsFlow(protocol, streamSettings string) bool {
	if protocol != string(model.VLESS) {
		return false
	}
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

// inboundCanHostFallbacks gates the settings.fallbacks injection.
// Xray only honors fallbacks on VLESS and Trojan inbounds carried over
// TCP transport with TLS or Reality security.
func inboundCanHostFallbacks(ib *model.Inbound) bool {
	if ib == nil {
		return false
	}
	if ib.Protocol != model.VLESS && ib.Protocol != model.Trojan {
		return false
	}
	return inboundCanEnableTlsFlow(string(ib.Protocol), ib.StreamSettings) ||
		(ib.Protocol == model.Trojan && trojanStreamSupportsFallbacks(ib.StreamSettings))
}

// trojanStreamSupportsFallbacks mirrors the Trojan side of the same gate
// (Trojan reuses XTLS-Vision capable streams: tcp + tls or reality).
func trojanStreamSupportsFallbacks(streamSettings string) bool {
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
