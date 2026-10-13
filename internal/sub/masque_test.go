package sub

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"testing"

	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/transport/internet"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// MASQUE has no share-link format, so the JSON subscription is the only way a client
// gets one: a masque outbound authenticating with the client's email and password.
func TestSubJsonServiceMasqueClientConfig(t *testing.T) {
	inbound := &model.Inbound{
		Listen: "203.0.113.9", Port: 8443, Protocol: model.MASQUE, Remark: "masque",
		Settings: `{"clients":[{"email":"ivy@example.test","password":"pw-ivy"}],"address":["10.14.0.1/24"]}`,
		StreamSettings: `{"network":"masque","security":"tls",
			"masqueSettings":{"path":"/.well-known/masque/ip/*/*/"},
			"tlsSettings":{"serverName":"masque.example.test","alpn":["h3"],
				"certificates":[{"certificateFile":"/etc/ssl/m.crt","keyFile":"/etc/ssl/m.key"}],
				"settings":{"fingerprint":"chrome"}}}`,
	}
	client := model.Client{Email: "ivy@example.test", Password: "pw-ivy"}

	configs := NewSubJsonService("", "", "", "", nil).getConfig(&SubService{address: "sub.example.com"}, inbound, client, "sub.example.com")
	if len(configs) != 1 {
		t.Fatalf("got %d configs, want one", len(configs))
	}
	var config struct {
		Outbounds []struct {
			Protocol       string         `json:"protocol"`
			Settings       map[string]any `json:"settings"`
			StreamSettings map[string]any `json:"streamSettings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(configs[0], &config); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	proxy := config.Outbounds[0]
	if proxy.Protocol != "masque" {
		t.Fatalf("first outbound protocol = %q, want masque", proxy.Protocol)
	}
	if proxy.Settings["address"] != "203.0.113.9" || proxy.Settings["port"] != float64(8443) {
		t.Fatalf("masque settings = %v, want the inbound address and port", proxy.Settings)
	}
	wantMasque := map[string]any{"path": "/.well-known/masque/ip/*/*/", "user": "ivy@example.test", "pass": "pw-ivy"}
	if got := proxy.StreamSettings["masqueSettings"]; !reflect.DeepEqual(got, wantMasque) {
		t.Fatalf("masqueSettings = %v, want %v", got, wantMasque)
	}
	tls, _ := proxy.StreamSettings["tlsSettings"].(map[string]any)
	if proxy.StreamSettings["network"] != "masque" || proxy.StreamSettings["security"] != "tls" ||
		tls["serverName"] != "masque.example.test" || tls["certificates"] != nil {
		t.Fatalf("stream = %v, want masque over client-side TLS to masque.example.test", proxy.StreamSettings)
	}
}

// The subscription SQL allowlist decides which inbounds a subId can see at all; left
// off it, a MASQUE client's JSON subscription would come back empty.
func TestGetInboundsBySubIdIncludesMasque(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()

	in := &model.Inbound{Port: 8443, Protocol: model.MASQUE, Enable: true, Tag: "masque-sub", Settings: `{"clients":[],"address":["10.14.0.1/24"]}`}
	if err := db.Create(in).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	rec := &model.ClientRecord{Email: "u@masque", SubID: "submasque", Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: in.Id}).Error; err != nil {
		t.Fatalf("create link: %v", err)
	}

	inbounds, err := (&SubService{}).getInboundsBySubId("submasque")
	if err != nil {
		t.Fatalf("getInboundsBySubId: %v", err)
	}
	if len(inbounds) != 1 || inbounds[0].Id != in.Id {
		t.Fatalf("masque inbound not returned for subId: %+v", inbounds)
	}
}

// An inbound keeps its hop range in quicParams.udpHop (udphop refuses to run on a server),
// but the client config must carry the udphop mask: the core ignores the old key since 26.9.9.
func TestSubJsonServiceTurnsInboundHopIntoClientUDPHopMask(t *testing.T) {
	inbound := &model.Inbound{
		Listen: "203.0.113.10", Port: 443, Protocol: model.Hysteria, Remark: "hy",
		Settings: `{"version":2,"clients":[{"email":"ivy@example.test","auth":"a"}]}`,
		StreamSettings: `{"network":"hysteria","security":"tls",
			"hysteriaSettings":{"version":2},
			"tlsSettings":{"serverName":"hy.example.test","alpn":["h3"]},
			"finalmask":{"quicParams":{"udpHop":{"ports":"20000-30000","interval":"5-10"}}}}`,
	}
	client := model.Client{Email: "ivy@example.test", Auth: "a"}

	configs := NewSubJsonService("", "", "", "", nil).getConfig(&SubService{address: "sub.example.com"}, inbound, client, "sub.example.com")
	if len(configs) != 1 {
		t.Fatalf("got %d configs, want one", len(configs))
	}
	var config struct {
		Outbounds []struct {
			Protocol       string         `json:"protocol"`
			StreamSettings map[string]any `json:"streamSettings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(configs[0], &config); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	got, _ := json.Marshal(config.Outbounds[0].StreamSettings["finalmask"])
	const want = `{"udp":[{"settings":{"interval":"5-10","mode":"intervalRemote","remotePorts":"20000-30000"},"type":"udphop"}]}`
	if string(got) != want {
		t.Fatalf("client finalmask\n got: %s\nwant: %s", got, want)
	}
}

// coreDialsUDP dials a client stream's finalmask the way the core does: it reverses the
// UDP masks and refuses a udphop anywhere but the socket end ("incorrect index").
func coreDialsUDP(t *testing.T, stream map[string]any) {
	t.Helper()
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer listener.Close()
	raw, _ := json.Marshal(map[string]any{"finalmask": stream["finalmask"]})
	var sc conf.StreamConfig
	if err := json.Unmarshal(raw, &sc); err != nil {
		t.Fatalf("decode stream: %v", err)
	}
	built, err := sc.Build()
	if err != nil {
		t.Fatalf("the core refuses the stream: %v", err)
	}
	mss, err := internet.ToMemoryStreamConfig(built)
	if err != nil {
		t.Fatalf("memory stream: %v", err)
	}
	port := listener.LocalAddr().(*net.UDPAddr).Port
	conn, err := mss.FinalMask.DialUDP(context.Background(), xnet.UDPDestination(xnet.LocalHostIP, xnet.Port(port)))
	if err != nil {
		t.Fatalf("the core cannot dial through the finalmask: %v", err)
	}
	_ = conn.Close()
}

// A client dials through one hopper and the core takes it only last in udp, so the JSON
// subscription keeps the most specific hop range (host, then inbound, then global) at the end.
func TestSubJsonServiceKeepsOneTrailingUDPHopMask(t *testing.T) {
	const hop = `{"type":"udphop","settings":{"mode":"intervalRemote","remotePorts":"%s","interval":"5-10"}}`
	const salamander = `{"type":"salamander","settings":{"password":"host-pass"}}`
	const inboundHop = `"finalmask":{"quicParams":{"udpHop":{"ports":"20000-30000","interval":"5-10"}}},`
	tests := []struct {
		name, inboundMask, hostMask, global, wantPorts string
	}{
		{
			"a host hop overrides the inbound and the global one", inboundHop,
			`{"udp":[` + salamander + `,` + fmt.Sprintf(hop, "40000-41000") + `]}`, fmt.Sprintf(hop, "30000-31000"), "40000-41000",
		},
		{
			"the inbound hop overrides the global one", inboundHop,
			`{"udp":[` + salamander + `]}`, fmt.Sprintf(hop, "30000-31000"), "20000-30000",
		},
		{
			"the global hop stays last behind host masks", ``,
			`{"udp":[` + salamander + `]}`, fmt.Sprintf(hop, "30000-31000"), "30000-31000",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inbound := &model.Inbound{
				Listen: "203.0.113.10", Port: 443, Protocol: model.Hysteria, Remark: "hy",
				Settings: `{"version":2,"clients":[{"email":"ivy@example.test","auth":"a"}]}`,
				StreamSettings: `{"network":"hysteria","security":"tls","hysteriaSettings":{"version":2},
					"tlsSettings":{"serverName":"hy.example.test","alpn":["h3"]},` + tc.inboundMask + `
					"externalProxy":[{"forceTls":"same","dest":"a.example.test","port":443,"remark":"a",
						"finalMask":` + strconv.Quote(tc.hostMask) + `}]}`,
			}
			client := model.Client{Email: "ivy@example.test", Auth: "a"}
			global := `{"udp":[` + tc.global + `]}`

			configs := NewSubJsonService("", "", global, "", nil).getConfig(&SubService{address: "sub.example.com"}, inbound, client, "sub.example.com")
			if len(configs) != 1 {
				t.Fatalf("got %d configs, want one", len(configs))
			}
			var config struct {
				Outbounds []struct {
					StreamSettings map[string]any `json:"streamSettings"`
				} `json:"outbounds"`
			}
			if err := json.Unmarshal(configs[0], &config); err != nil {
				t.Fatalf("decode config: %v", err)
			}
			stream := config.Outbounds[0].StreamSettings
			finalmask, _ := stream["finalmask"].(map[string]any)
			masks, _ := finalmask["udp"].([]any)
			var hops []string
			for _, entry := range masks {
				mask, _ := entry.(map[string]any)
				if mask["type"] == "udphop" {
					settings, _ := mask["settings"].(map[string]any)
					hops = append(hops, fmt.Sprint(settings["remotePorts"]))
				}
			}
			last, _ := masks[len(masks)-1].(map[string]any)
			if len(hops) != 1 || hops[0] != tc.wantPorts || last["type"] != "udphop" {
				got, _ := json.Marshal(finalmask)
				t.Fatalf("client finalmask = %s, want one trailing udphop on %s", got, tc.wantPorts)
			}
			coreDialsUDP(t, stream)
		})
	}
}

// Every endpoint's config starts from the same parsed inbound finalmask; a host whose own
// masks are merged in first must not use up the hop range the endpoints after it need.
func TestSubJsonServiceKeepsUDPHopMaskOnEveryEndpoint(t *testing.T) {
	inbound := &model.Inbound{
		Listen: "203.0.113.10", Port: 443, Protocol: model.Hysteria, Remark: "hy",
		Settings: `{"version":2,"clients":[{"email":"ivy@example.test","auth":"a"}]}`,
		StreamSettings: `{"network":"hysteria","security":"tls",
			"hysteriaSettings":{"version":2},
			"tlsSettings":{"serverName":"hy.example.test","alpn":["h3"]},
			"finalmask":{"quicParams":{"udpHop":{"ports":"20000-30000","interval":"5-10"}}},
			"externalProxy":[
				{"forceTls":"same","dest":"a.example.test","port":443,"remark":"a",
					"finalMask":"{\"udp\":[{\"type\":\"salamander\",\"settings\":{\"password\":\"p\"}}]}"},
				{"forceTls":"same","dest":"b.example.test","port":443,"remark":"b"}]}`,
	}
	client := model.Client{Email: "ivy@example.test", Auth: "a"}

	configs := NewSubJsonService("", "", "", "", nil).getConfig(&SubService{address: "sub.example.com"}, inbound, client, "sub.example.com")
	if len(configs) != 2 {
		t.Fatalf("got %d configs, want one per endpoint", len(configs))
	}
	const hop = `{"settings":{"interval":"5-10","mode":"intervalRemote","remotePorts":"20000-30000"},"type":"udphop"}`
	wants := []string{
		`{"udp":[{"settings":{"password":"p"},"type":"salamander"},` + hop + `]}`,
		`{"udp":[` + hop + `]}`,
	}
	for i, want := range wants {
		var config struct {
			Outbounds []struct {
				StreamSettings map[string]any `json:"streamSettings"`
			} `json:"outbounds"`
		}
		if err := json.Unmarshal(configs[i], &config); err != nil {
			t.Fatalf("decode config %d: %v", i, err)
		}
		got, _ := json.Marshal(config.Outbounds[0].StreamSettings["finalmask"])
		if string(got) != want {
			t.Fatalf("endpoint %d finalmask\n got: %s\nwant: %s", i, got, want)
		}
	}
}
