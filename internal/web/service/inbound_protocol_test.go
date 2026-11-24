package service

import (
	"strings"
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

		// xray-core runs Vision inside VLESS encryption on any transport (encryption.CommonConn)
		// but XDRIVE, which refuses a flow since 26.10.17 (#7182).
		{"vless raw vlessenc", string(model.VLESS), `{"network":"tcp","security":"none"}`, `{"decryption":"mlkem768x25519plus.native.600s.mMFxPe7lz5xoq2qBk22cQYefu5fpc_2dGR8lMOKem0E","encryption":"mlkem768x25519plus.native.0rtt.hT4AY_tPWY9NVuKR3BIXxXq6zx9DqN2X86QPYW09XEM"}`, true},
		{"vless ws vlessenc", string(model.VLESS), `{"network":"ws","security":"none"}`, `{"encryption":"` + vlessEncValue + `"}`, true},
		{"vless xdrive vlessenc", string(model.VLESS), `{"network":"xdrive","security":"none"}`, `{"decryption":"` + vlessEncValue + `"}`, false},
		{"vless xdrive no encryption", string(model.VLESS), `{"network":"xdrive","security":"none"}`, `{"decryption":"none"}`, false},

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

// xray-core 26.10.31 (#7288) refuses an unknown autoSystemWfpBlockLeak value when the core starts.
func TestAddInboundRefusesTunWfpBlockLeakTheCoreRefuses(t *testing.T) {
	tests := []struct{ name, leaks, want string }{
		{"port zero", `["tcp/0"]`, "unknown autoSystemWfpBlockLeak value: tcp/0"},
		{"unknown protocol", `["icmp/1"]`, "unknown autoSystemWfpBlockLeak value: icmp/1"},
		{"named values and ports", `["DNS","windnsclient","misconfigtun","udp/853","tcp/7215"]`, ""},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			_, _, err := (&InboundService{}).AddInbound(&model.Inbound{
				Enable: true, Port: 18520 + i, Protocol: model.Protocol("tun"),
				Settings: `{"name":"xray0","autoSystemWfpBlockLeak":` + tc.leaks + `}`,
			})
			if tc.want == "" {
				if err != nil {
					t.Fatalf("AddInbound = %v, want values the core accepts saved", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("AddInbound = %v, want one containing %q", err, tc.want)
			}
		})
	}
}
