package sub

import (
	"reflect"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Clash has no shadowsocks tcp/http header, so the inbound path drops the node
// (applyTransport returns false) and the external-link path must drop it too,
// instead of handing mihomo a proxy that connects with the wrong obfuscation.
func TestClashExternalShadowsocksMatchesInboundPath(t *testing.T) {
	const settings = `{"method":"aes-256-gcm","password":"inboundpw","clients":[{"password":"clientpw","email":"user"}]}`
	// base64("aes-256-gcm:clientpw") — the SIP002 userinfo of the links below.
	const userinfo = "YWVzLTI1Ni1nY206Y2xpZW50cHc"

	tests := []struct {
		name        string
		stream      string
		link        string
		wantDropped bool
	}{
		{
			name:   "plain tcp",
			stream: `{"network":"tcp","security":"none"}`,
			link:   "ss://" + userinfo + "@203.0.113.1:8443?type=tcp#ss",
		},
		{
			name:        "tcp http header",
			stream:      `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"http","request":{"path":["/"],"headers":{"Host":["test"]}}}}}`,
			link:        "ss://" + userinfo + "@203.0.113.1:8443?type=tcp&headerType=http&host=test#ss",
			wantDropped: true,
		},
		{
			name:   "tls",
			stream: `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"ss.sni"}}`,
			link:   "ss://" + userinfo + "@203.0.113.1:8443?type=tcp&security=tls&sni=ss.sni#ss",
		},
	}

	svc := NewSubClashService(false, "", &SubService{})
	client := model.Client{Password: "clientpw", Email: "user"}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inbound := &model.Inbound{
				Listen:         "203.0.113.1",
				Port:           8443,
				Protocol:       model.Shadowsocks,
				Remark:         "ss",
				Settings:       settings,
				StreamSettings: tc.stream,
			}

			fromInbound := svc.buildProxy(svc.SubService, inbound, client, svc.streamData(tc.stream), nil)
			fromLink := svc.clashProxyFromExternal(tc.link, "external")

			if tc.wantDropped {
				if fromInbound != nil || fromLink != nil {
					t.Fatalf("not representable in clash, want both dropped: inbound %#v, link %#v", fromInbound, fromLink)
				}
				return
			}
			if fromInbound == nil || fromLink == nil {
				t.Fatalf("want a proxy from both paths: inbound %#v, link %#v", fromInbound, fromLink)
			}
			delete(fromInbound, "name")
			delete(fromLink, "name")
			if !reflect.DeepEqual(fromInbound, fromLink) {
				t.Fatalf("inbound %#v != link %#v", fromInbound, fromLink)
			}
		})
	}
}
