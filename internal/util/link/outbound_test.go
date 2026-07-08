package link

import (
	"net/url"
	"strings"
	"testing"
)

func TestParseVmessLink(t *testing.T) {
	// vmess:// + base64 of:
	// {"v":"2","ps":"test","add":"1.2.3.4","port":443,"id":"uuid","aid":"0","net":"ws","type":"","host":"ex.com","path":"/","tls":"tls"}
	link := "vmess://eyJ2IjoiMiIsInBzIjoidGVzdCIsImFkZCI6IjEuMi4zLjQiLCJwb3J0Ijo0NDMsImlkIjoidXVpZCIsImFpZCI6IjAiLCJuZXQiOiJ3cyIsInR5cGUiOiIiLCJob3N0IjoiZXguY29tIiwicGF0aCI6Ii8iLCJ0bHMiOiJ0bHMifQ=="
	res, err := ParseLink(link)
	if err != nil {
		t.Fatalf("parse vmess: %v", err)
	}
	if res.Outbound["protocol"] != "vmess" {
		t.Errorf("expected vmess protocol, got %v", res.Outbound["protocol"])
	}
	if res.Outbound["tag"] != "test" {
		t.Errorf("expected tag 'test', got %v", res.Outbound["tag"])
	}
}

func TestParseVlessLink(t *testing.T) {
	link := "vless://uuid@1.2.3.4:443?type=ws&security=tls&path=/&host=ex.com#node1"
	res, err := ParseLink(link)
	if err != nil {
		t.Fatalf("parse vless: %v", err)
	}
	if res.Outbound["protocol"] != "vless" {
		t.Fatalf("bad protocol")
	}
	if res.Outbound["tag"] != "node1" {
		t.Errorf("tag mismatch: %v", res.Outbound["tag"])
	}
}

func TestParseVlessLink_FinalMaskQuicParamsSanitized(t *testing.T) {
	fm := url.QueryEscape(`{"mask":"dtls","quicParams":{"keepAlivePeriod":"10s","maxIdleTimeout":"30","initStreamReceiveWindow":524288,"maxIncomingStreams":true,"brutalUp":"100 mbps"}}`)
	res, err := ParseLink("vless://uuid@1.2.3.4:443?type=tcp&security=none&fm=" + fm + "#node1")
	if err != nil {
		t.Fatalf("parse vless with fm: %v", err)
	}
	stream, ok := res.Outbound["streamSettings"].(map[string]any)
	if !ok {
		t.Fatalf("missing streamSettings: %v", res.Outbound)
	}
	finalmask, ok := stream["finalmask"].(map[string]any)
	if !ok {
		t.Fatalf("missing finalmask: %v", stream)
	}
	if finalmask["mask"] != "dtls" {
		t.Errorf("mask changed: %v", finalmask["mask"])
	}
	qp, ok := finalmask["quicParams"].(map[string]any)
	if !ok {
		t.Fatalf("missing quicParams: %v", finalmask)
	}
	if got := qp["keepAlivePeriod"]; got != int64(10) {
		t.Errorf("keepAlivePeriod: expected 10, got %v (%T)", got, got)
	}
	if got := qp["maxIdleTimeout"]; got != int64(30) {
		t.Errorf("maxIdleTimeout: expected 30, got %v (%T)", got, got)
	}
	if got := qp["initStreamReceiveWindow"]; got != int64(524288) {
		t.Errorf("initStreamReceiveWindow: expected 524288, got %v (%T)", got, got)
	}
	if _, exists := qp["maxIncomingStreams"]; exists {
		t.Errorf("maxIncomingStreams should be dropped, got %v", qp["maxIncomingStreams"])
	}
	if got := qp["brutalUp"]; got != "100 mbps" {
		t.Errorf("brutalUp should stay a string, got %v (%T)", got, got)
	}
}

func TestSanitizeFinalMaskQuicParams_ClampsAndRejects(t *testing.T) {
	cases := []struct {
		name string
		key  string
		in   any
		want any
	}{
		{"infinite string dropped", "keepAlivePeriod", "inf", nil},
		{"nan string dropped", "keepAlivePeriod", "NaN", nil},
		{"negative dropped", "maxStreamReceiveWindow", float64(-5), nil},
		{"negative duration dropped", "keepAlivePeriod", "-10s", nil},
		{"absurd magnitude dropped", "initConnectionReceiveWindow", float64(1e30), nil},
		{"keepAlive clamped up", "keepAlivePeriod", "1s", int64(2)},
		{"keepAlive clamped down", "keepAlivePeriod", "90s", int64(60)},
		{"idle clamped up", "maxIdleTimeout", float64(1), int64(4)},
		{"idle clamped down", "maxIdleTimeout", "10m", int64(120)},
		{"streams clamped up", "maxIncomingStreams", float64(4), int64(8)},
		{"zero means unset and survives", "maxIdleTimeout", float64(0), int64(0)},
		{"window passes through", "initStreamReceiveWindow", float64(524288), int64(524288)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			parsed := map[string]any{"quicParams": map[string]any{c.key: c.in}}
			sanitizeFinalMaskQuicParams(parsed)
			qp := parsed["quicParams"].(map[string]any)
			got, exists := qp[c.key]
			if c.want == nil {
				if exists {
					t.Fatalf("%s: expected key dropped, got %v (%T)", c.key, got, got)
				}
				return
			}
			if !exists || got != c.want {
				t.Fatalf("%s: expected %v, got %v (%T)", c.key, c.want, got, got)
			}
		})
	}
}

func TestParseSubscriptionBody_Base64(t *testing.T) {
	// base64 of the two joined links:
	// vless://u@h:443?type=tcp#A\nvless://u2@h2:443?type=tcp#B
	b64 := "dmxlc3M6Ly91QGg6NDQzP3R5cGU9dGNwI0EKdmxlc3M6Ly91MkBoMjo0NDM/dHlwZT10Y3AjQg=="
	obs, ids, err := ParseSubscriptionBody([]byte(b64))
	if err != nil {
		t.Fatalf("parse sub body: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("expected 2 outbounds, got %d", len(obs))
	}
	if !strings.HasPrefix(ids[0], "vless:") || !strings.HasPrefix(ids[1], "vless:") {
		t.Errorf("bad identities: %v", ids)
	}
}

func TestSlugAndSuggest(t *testing.T) {
	if SlugRemark("Hello World!") != "hello-world" {
		t.Errorf("slug failed")
	}
	tag := SuggestTag("hk-", "  SG 01 !! ", 0)
	if tag != "hk-sg-01" {
		t.Errorf("suggest tag got %q", tag)
	}
	// Non-ASCII letters/digits are preserved rather than stripped.
	if got := SlugRemark("Москва 🇷🇺 01"); got != "москва-01" {
		t.Errorf("unicode slug got %q", got)
	}
	if got := SuggestTag("ru-", "Сервер 2", 0); got != "ru-сервер-2" {
		t.Errorf("unicode suggest tag got %q", got)
	}
}
