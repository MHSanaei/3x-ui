package sub

import (
	"net/url"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func TestGenWireguardLinkFields(t *testing.T) {
	serverPriv, serverPub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	clientPriv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("client keypair: %v", err)
	}

	inbound := &model.Inbound{
		Listen:   "203.0.113.7",
		Port:     51820,
		Protocol: model.WireGuard,
		Remark:   "wg-sub",
		Settings: `{"secretKey":"` + serverPriv + `","mtu":1420,"clients":[{"email":"user","privateKey":"` + clientPriv + `","allowedIPs":["10.0.0.2/32"],"keepAlive":25}]}`,
	}

	s := &SubService{}
	link := s.genWireguardLink(inbound, "user")

	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("link does not parse: %v\n got: %s", err, link)
	}
	if u.Scheme != "wireguard" {
		t.Fatalf("scheme = %q, want wireguard", u.Scheme)
	}
	if u.Host != "203.0.113.7:51820" {
		t.Fatalf("host = %q, want 203.0.113.7:51820", u.Host)
	}
	if u.User.Username() != clientPriv {
		t.Fatalf("userinfo = %q, want client private key %q", u.User.Username(), clientPriv)
	}
	q := u.Query()
	if q.Get("publickey") != serverPub {
		t.Fatalf("publickey = %q, want server public key %q", q.Get("publickey"), serverPub)
	}
	if q.Get("address") != "10.0.0.2/32" {
		t.Fatalf("address = %q, want 10.0.0.2/32", q.Get("address"))
	}
	if q.Get("mtu") != "1420" {
		t.Fatalf("mtu = %q, want 1420", q.Get("mtu"))
	}
}

func TestGenWireguardLinkWrongProtocol(t *testing.T) {
	s := &SubService{}
	vless := &model.Inbound{Protocol: model.VLESS, Settings: `{"clients":[{"email":"user"}]}`}
	if got := s.genWireguardLink(vless, "user"); got != "" {
		t.Fatalf("wrong protocol should yield empty link, got %q", got)
	}
}

func TestGenWireguardLinkNoKey(t *testing.T) {
	s := &SubService{}
	inbound := &model.Inbound{
		Protocol: model.WireGuard,
		Port:     51820,
		Settings: `{"secretKey":"x","clients":[{"email":"user"}]}`,
	}
	if got := s.genWireguardLink(inbound, "user"); got != "" {
		t.Fatalf("client without private key should yield empty link, got %q", got)
	}
}

func TestGetInboundsBySubIdIncludesWireguard(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()

	in := &model.Inbound{Port: 51820, Protocol: model.WireGuard, Enable: true, Tag: "wg-sub", Settings: `{"secretKey":"x","clients":[]}`}
	if err := db.Create(in).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	rec := &model.ClientRecord{Email: "u@wg", SubID: "subwg", Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: in.Id}).Error; err != nil {
		t.Fatalf("create link: %v", err)
	}

	s := &SubService{}
	inbounds, err := s.getInboundsBySubId("subwg")
	if err != nil {
		t.Fatalf("getInboundsBySubId: %v", err)
	}
	if len(inbounds) != 1 || inbounds[0].Id != in.Id {
		t.Fatalf("wireguard inbound not returned for subId: %+v", inbounds)
	}
}
