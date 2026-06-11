package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// A representative vlessenc/ML-KEM encryption value as produced by `xray
// vlessenc` — a dotted string, never the literal "vlessenc".
const vlessEncValue = "mlkem768x25519plus.native.0rtt.G3cdPSd1-NnlpTbWNSM5vHsT5VNzWfFzYSKwbUMnV1Y"

func TestInboundCanEnableTlsFlow(t *testing.T) {
	cases := []struct {
		name           string
		protocol       string
		streamSettings string
		settings       string
		want           bool
	}{
		{"vless tcp tls", string(model.VLESS), `{"network":"tcp","security":"tls"}`, "", true},
		{"vless tcp reality", string(model.VLESS), `{"network":"tcp","security":"reality"}`, "", true},
		{"vless tcp none no enc", string(model.VLESS), `{"network":"tcp","security":"none"}`, "", false},
		{"vless ws tls", string(model.VLESS), `{"network":"ws","security":"tls"}`, "", false},
		{"vless grpc reality", string(model.VLESS), `{"network":"grpc","security":"reality"}`, "", false},
		{"vmess tcp tls", string(model.VMESS), `{"network":"tcp","security":"tls"}`, "", false},
		{"empty stream", string(model.VLESS), "", "", false},

		// vlessenc is gated to XHTTP only. TCP without tls/reality is NOT
		// Vision-capable even with vlessenc set — the combination only works on
		// XHTTP in practice.
		{"vless tcp vlessenc not capable", string(model.VLESS), `{"network":"tcp","security":"none"}`, `{"decryption":"mlkem768x25519plus.native.600s.mMFxPe7lz5xoq2qBk22cQYefu5fpc_2dGR8lMOKem0E","encryption":"mlkem768x25519plus.native.0rtt.hT4AY_tPWY9NVuKR3BIXxXq6zx9DqN2X86QPYW09XEM"}`, false},
		// ws is a framed transport — vlessenc never enables Vision there.
		{"vless ws vlessenc still off", string(model.VLESS), `{"network":"ws","security":"none"}`, `{"encryption":"` + vlessEncValue + `"}`, false},

		// XHTTP + VLESS encryption (the #5157 case).
		{"vless xhttp vlessenc", string(model.VLESS), `{"network":"xhttp","security":"none"}`, `{"encryption":"` + vlessEncValue + `"}`, true},
		{"vless xhttp encryption none", string(model.VLESS), `{"network":"xhttp","security":"none"}`, `{"encryption":"none"}`, false},
		{"vless xhttp no settings", string(model.VLESS), `{"network":"xhttp","security":"none"}`, "", false},
		// Regression for PR #5185: the gate is "any non-none encryption", NOT an
		// equality check against the literal "vlessenc" (which the buggy PR used
		// and which never matches a real, generated encryption value). An x25519
		// auth value must enable it just like the ML-KEM value above.
		{"vless xhttp x25519 enc", string(model.VLESS), `{"network":"xhttp","security":"none"}`, `{"encryption":"native.0rtt.121s-180s.xRMUYYjQctqYO1pSyffM-w"}`, true},
		// Server-side configs (API/JSON) may carry only decryption; that alone
		// must also enable the flow gate.
		{"vless xhttp decryption only", string(model.VLESS), `{"network":"xhttp","security":"none"}`, `{"decryption":"` + vlessEncValue + `","encryption":"none"}`, true},
		// XHTTP without encryption stays off even with tls (Vision over XHTTP is
		// gated on vlessenc, not transport security).
		{"vless xhttp tls no encryption", string(model.VLESS), `{"network":"xhttp","security":"tls"}`, `{"encryption":"none"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := inboundCanEnableTlsFlow(tc.protocol, tc.streamSettings, tc.settings)
			if got != tc.want {
				t.Errorf("inboundCanEnableTlsFlow(%q, %q, %q) = %v, want %v",
					tc.protocol, tc.streamSettings, tc.settings, got, tc.want)
			}
		})
	}
}

// Fallbacks must remain raw-TCP-only and must NOT follow the broadened flow gate
// onto XHTTP+vlessenc.
func TestInboundCanHostFallbacks_StaysTcpOnly(t *testing.T) {
	cases := []struct {
		name           string
		protocol       model.Protocol
		streamSettings string
		settings       string
		want           bool
	}{
		{"vless tcp tls", model.VLESS, `{"network":"tcp","security":"tls"}`, "", true},
		{"trojan tcp reality", model.Trojan, `{"network":"tcp","security":"reality"}`, "", true},
		{"vless xhttp vlessenc not fallback-capable", model.VLESS, `{"network":"xhttp","security":"none"}`, `{"encryption":"` + vlessEncValue + `"}`, false},
		{"vmess tcp tls not fallback-capable", model.VMESS, `{"network":"tcp","security":"tls"}`, "", false},
		{"nil-ish empty stream", model.VLESS, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ib := &model.Inbound{Protocol: tc.protocol, StreamSettings: tc.streamSettings, Settings: tc.settings}
			if got := inboundCanHostFallbacks(ib); got != tc.want {
				t.Errorf("inboundCanHostFallbacks = %v, want %v", got, tc.want)
			}
		})
	}
	if inboundCanHostFallbacks(nil) {
		t.Errorf("inboundCanHostFallbacks(nil) = true, want false")
	}
}
