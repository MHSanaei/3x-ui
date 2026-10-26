package sub

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClashExternalHysteriaPreservesStreamOptions(t *testing.T) {
	const pin = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	const fingerprint = "00:01:02:03:04:05:06:07:08:09:0A:0B:0C:0D:0E:0F:10:11:12:13:14:15:16:17:18:19:1A:1B:1C:1D:1E:1F"
	tests := []struct {
		name  string
		query string
		want  map[string]any
	}{
		{
			name:  "plain TLS retains SNI ALPN and client fingerprint",
			query: "sni=tls.example&alpn=h3,h2&fp=chrome",
			want:  map[string]any{"sni": "tls.example", "alpn": []string{"h3", "h2"}, "client-fingerprint": "chrome"},
		},
		{
			name:  "salamander password and hopping range survive conversion",
			query: "obfs=salamander&obfs-password=obfs%2B%26%3D%23&mport=20000-30000",
			want:  map[string]any{"obfs": "salamander", "obfs-password": "obfs+&=#", "ports": "20000-30000"},
		},
		{
			name:  "gecko packet sizes and multiple hopping ranges survive conversion",
			query: "obfs=gecko&obfs-password=pw&minPacketSize=512&maxPacketSize=1200&mport=20000-30000,40000-50000",
			want: map[string]any{
				"obfs": "gecko", "obfs-password": "pw", "obfs-min-packet-size": 512,
				"obfs-max-packet-size": 1200, "ports": "20000-30000,40000-50000",
			},
		},
		{
			name:  "certificate pin remains distinct from client fingerprint",
			query: "fp=firefox&pinSHA256=invalid," + pin,
			want:  map[string]any{"client-fingerprint": "firefox", "fingerprint": fingerprint},
		},
		{
			name:  "base64 certificate pin is normalized",
			query: "pinSHA256=AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8%3D",
			want:  map[string]any{"fingerprint": fingerprint},
		},
		{
			name:  "invalid packet sizes retain salamander password",
			query: "obfs=gecko&obfs-password=pw&minPacketSize=1200&maxPacketSize=512",
			want:  map[string]any{"obfs": "salamander", "obfs-password": "pw"},
		},
		{
			name:  "incomplete packet size range retains salamander password",
			query: "obfs=gecko&obfs-password=pw&minPacketSize=512",
			want:  map[string]any{"obfs": "salamander", "obfs-password": "pw"},
		},
		{
			name:  "obfuscation without password leaves hopping intact",
			query: "obfs=salamander&mport=20000-30000",
			want:  map[string]any{"ports": "20000-30000"},
		},
		{
			name:  "unknown obfuscation and invalid pin are omitted",
			query: "obfs=unknown&obfs-password=pw&pinSHA256=not-a-pin",
		},
		{
			name:  "finalmask obfuscation and hopping take precedence over URI options",
			query: "obfs=salamander&obfs-password=ignored&mport=1-2&fm=" + url.QueryEscape(`{"udp":[{"type":"salamander","settings":{"password":"mask-pw","packetSize":"600-1000"}},{"type":"udphop","settings":{"remotePorts":"30000-40000"}}]}`),
			want: map[string]any{
				"obfs": "gecko", "obfs-password": "mask-pw", "obfs-min-packet-size": 600,
				"obfs-max-packet-size": 1000, "ports": "30000-40000",
			},
		},
		{
			name:  "legacy hopping range survives conversion",
			query: "fm=" + url.QueryEscape(`{"quicParams":{"udpHop":{"ports":" 20000-30000 "}}}`),
			want:  map[string]any{"ports": "20000-30000"},
		},
		{
			name:  "malformed masks are skipped before usable obfuscation and hopping",
			query: "fm=" + url.QueryEscape(`{"udp":[null,42,{"type":"salamander","settings":{"password":42}},{"type":"salamander","settings":{"password":"pw","packetSize":"512-4096"}},{"type":"udphop","settings":null},{"type":"udphop","settings":{"remotePorts":"20000-30000"}}]}`),
			want:  map[string]any{"obfs": "salamander", "obfs-password": "pw", "ports": "20000-30000"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := map[string]any{
				"name": "external", "type": "hysteria2", "server": "192.0.2.1", "port": 443,
				"password": "auth+&=#", "udp": true, "alpn": []string{"h3"},
			}
			for key, value := range tt.want {
				want[key] = value
			}
			got := (&SubClashService{}).clashProxyFromExternal("hy2://auth%2B%26%3D%23@192.0.2.1:443?"+tt.query+"#provider", "external")
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("proxy = %#v, want %#v", got, want)
			}
		})
	}
}

func TestClashExternalHysteriaRejectsInvalidLinks(t *testing.T) {
	for _, rawLink := range []string{
		"hysteria2://@192.0.2.1:443?obfs=salamander&obfs-password=pw",
		"hysteria2://pass@[::1",
		"hysteria2://pass%zz@192.0.2.1:443",
	} {
		t.Run(rawLink, func(t *testing.T) {
			if got := (&SubClashService{}).clashProxyFromExternal(rawLink, "external"); got != nil {
				t.Fatalf("invalid link produced proxy %#v, want nil", got)
			}
		})
	}
}

func TestClashHysteriaStreamOptionsMatchInbound(t *testing.T) {
	const pin = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	const fingerprint = "00:01:02:03:04:05:06:07:08:09:0A:0B:0C:0D:0E:0F:10:11:12:13:14:15:16:17:18:19:1A:1B:1C:1D:1E:1F"
	tests := []struct {
		name string
		mask string
		want map[string]any
	}{
		{
			name: "salamander with modern hopping preferred over legacy range",
			mask: `{"udp":[{"type":"salamander","settings":{"password":"pw"}},{"type":"udphop","settings":{"remotePorts":"20000-30000"}}],"quicParams":{"udpHop":{"ports":"1-2"}}}`,
			want: map[string]any{"obfs": "salamander", "obfs-password": "pw", "ports": "20000-30000"},
		},
		{
			name: "gecko with legacy hopping",
			mask: `{"udp":[{"type":"salamander","settings":{"password":"pw","packetSize":"512-1200"}}],"quicParams":{"udpHop":{"ports":"20000-30000"}}}`,
			want: map[string]any{
				"obfs": "gecko", "obfs-password": "pw", "obfs-min-packet-size": 512,
				"obfs-max-packet-size": 1200, "ports": "20000-30000",
			},
		},
		{
			name: "malformed optional settings do not add proxy fields",
			mask: `{"udp":[null,{"type":"salamander","settings":null},{"type":"udphop","settings":{"remotePorts":42}}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inbound := &model.Inbound{
				Protocol: model.Hysteria, Listen: "192.0.2.1", Port: 443, Settings: `{"version":2}`,
				StreamSettings: `{"tlsSettings":{"serverName":"tls.example","alpn":["h3"],"settings":{"fingerprint":"chrome","pinnedPeerCertSha256":["` + pin + `"]}},"finalmask":` + tt.mask + `}`,
			}
			svc := &SubClashService{}
			fromInbound := svc.buildHysteriaProxy(&SubService{}, inbound, model.Client{Email: "user", Auth: "secret"}, nil)
			fromExternal := svc.clashProxyFromExternal("hysteria2://secret@192.0.2.1:443?sni=tls.example&fp=chrome&pinSHA256="+pin+"&fm="+url.QueryEscape(tt.mask), "external")
			want := map[string]any{
				"type": "hysteria2", "server": "192.0.2.1", "port": 443, "password": "secret", "udp": true,
				"sni": "tls.example", "alpn": []string{"h3"}, "client-fingerprint": "chrome", "fingerprint": fingerprint,
			}
			for key, value := range tt.want {
				want[key] = value
			}
			for path, proxy := range map[string]map[string]any{"inbound": fromInbound, "external": fromExternal} {
				delete(proxy, "name")
				if !reflect.DeepEqual(proxy, want) {
					t.Errorf("%s proxy = %#v, want %#v", path, proxy, want)
				}
			}
		})
	}
}

func TestClashHysteriaV1RetainsAuthAndInsecureOverride(t *testing.T) {
	inbound := &model.Inbound{
		Protocol: model.Hysteria, Listen: "192.0.2.1", Port: 443, Settings: `{"version":1}`,
		StreamSettings: `{"tlsSettings":{"serverName":"tls.example","alpn":["h3"],"settings":{"allowInsecure":true,"fingerprint":"chrome"}}}`,
	}
	svc := &SubClashService{}
	for _, ep := range []map[string]any{nil, {"allowInsecure": false}} {
		got := svc.buildHysteriaProxy(&SubService{}, inbound, model.Client{Email: "user", Auth: "secret"}, ep)
		delete(got, "name")
		want := map[string]any{
			"type": "hysteria", "server": "192.0.2.1", "port": 443, "auth-str": "secret", "udp": true,
			"sni": "tls.example", "alpn": []string{"h3"}, "skip-cert-verify": true, "client-fingerprint": "chrome",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("proxy with endpoint %#v = %#v, want %#v", ep, got, want)
		}
	}
}
