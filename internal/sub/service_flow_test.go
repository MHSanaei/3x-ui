package sub

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Issue #5232: a vision flow set on a VLESS+XHTTP+REALITY (vlessenc) client
// must survive into subscription output, not just the inbound JSON.

const testMlkemEncryption = "mlkem768x25519plus.native.0rtt.dGVzdC1rZXk"

func TestVlessFlowAllowed(t *testing.T) {
	enc := map[string]any{"encryption": testMlkemEncryption}
	noEnc := map[string]any{"encryption": "none"}

	tests := []struct {
		name     string
		network  string
		security string
		settings map[string]any
		want     bool
	}{
		{"tcp tls", "tcp", "tls", noEnc, true},
		{"tcp reality", "tcp", "reality", noEnc, true},
		{"tcp none", "tcp", "none", noEnc, false},
		{"tcp none vlessenc", "tcp", "none", enc, false},
		{"xhttp none vlessenc", "xhttp", "none", enc, true},
		{"xhttp reality vlessenc (#5232)", "xhttp", "reality", enc, true},
		{"xhttp tls vlessenc", "xhttp", "tls", enc, true},
		{"xhttp reality no vlessenc", "xhttp", "reality", noEnc, false},
		{"ws tls", "ws", "tls", noEnc, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := vlessFlowAllowed(tc.network, tc.security, tc.settings); got != tc.want {
				t.Fatalf("vlessFlowAllowed(%q, %q, %v) = %v, want %v", tc.network, tc.security, tc.settings, got, tc.want)
			}
		})
	}
}

func flowTestInbound(streamSettings, encryption string) *model.Inbound {
	return &model.Inbound{
		Listen:   "203.0.113.1",
		Port:     443,
		Protocol: model.VLESS,
		Remark:   "flowtest",
		Settings: `{"clients":[{"id":"11111111-2222-4333-8444-555555555555","email":"user","flow":"xtls-rprx-vision"}],` +
			`"decryption":"` + encryption + `","encryption":"` + encryption + `"}`,
		StreamSettings: streamSettings,
	}
}

const xhttpRealityStream = `{
	"network": "xhttp",
	"security": "reality",
	"xhttpSettings": {"path": "/", "mode": "auto"},
	"realitySettings": {
		"serverNames": ["example.com"],
		"shortIds": ["abcd"],
		"settings": {"publicKey": "pub", "fingerprint": "chrome"}
	}
}`

func TestGenVlessLink_FlowXhttpRealityVlessenc(t *testing.T) {
	s := &SubService{remarkModel: "-ieo"}
	link := s.genVlessLink(flowTestInbound(xhttpRealityStream, testMlkemEncryption), "user")
	if !strings.Contains(link, "flow=xtls-rprx-vision") {
		t.Fatalf("xhttp+reality+vlessenc link must carry the vision flow (#5232), got %q", link)
	}
}

func TestGenVlessLink_NoFlowXhttpRealityWithoutVlessenc(t *testing.T) {
	s := &SubService{remarkModel: "-ieo"}
	link := s.genVlessLink(flowTestInbound(xhttpRealityStream, "none"), "user")
	if strings.Contains(link, "flow=") {
		t.Fatalf("xhttp+reality without vlessenc must not carry a flow, got %q", link)
	}
}

func TestGenVlessLink_FlowTcpRealityStillWorks(t *testing.T) {
	stream := `{
		"network": "tcp",
		"security": "reality",
		"tcpSettings": {"header": {"type": "none"}},
		"realitySettings": {
			"serverNames": ["example.com"],
			"shortIds": ["abcd"],
			"settings": {"publicKey": "pub", "fingerprint": "chrome"}
		}
	}`
	s := &SubService{remarkModel: "-ieo"}
	link := s.genVlessLink(flowTestInbound(stream, "none"), "user")
	if !strings.Contains(link, "flow=xtls-rprx-vision") {
		t.Fatalf("tcp+reality link must keep the vision flow, got %q", link)
	}
}
