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

// inboundCanEnableTlsFlow mirrors canEnableTlsFlow() from the frontend
// (frontend/src/lib/xray/protocol-capabilities.ts). XTLS Vision is valid for
// VLESS on TCP with tls or reality (classic), and on XHTTP when VLESS encryption
// (vlessenc / ML-KEM) is enabled — there the post-quantum, VLESS-level
// encryption stands in for the transport TLS that Vision relies on. settings is
// the inbound's raw settings JSON, which carries the encryption value
// (streamSettings does not).
func inboundCanEnableTlsFlow(protocol, streamSettings, settings string) bool {
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
	switch stream.Network {
	case "tcp":
		return stream.Security == "tls" || stream.Security == "reality"
	case "xhttp":
		return vlessEncryptionEnabled(settings)
	default:
		return false
	}
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
// than inboundCanEnableTlsFlow (which also accepts XHTTP+vlessenc): fallbacks
// are a raw-TCP-only feature.
func inboundCanHostFallbacks(ib *model.Inbound) bool {
	if ib == nil {
		return false
	}
	if ib.Protocol != model.VLESS && ib.Protocol != model.Trojan {
		return false
	}
	return streamSupportsFallbacks(ib.StreamSettings)
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
