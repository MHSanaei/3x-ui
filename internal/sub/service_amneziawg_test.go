package sub

import (
	"net/url"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func TestGenAmneziaWGLinkFields(t *testing.T) {
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
		Protocol: model.AmneziaWG,
		Remark:   "awg-sub",
		Settings: `{"server":{"privateKey":"` + serverPriv + `","publicKey":"` + serverPub + `","mtu":1420,"primaryDns":"8.8.8.8"},` +
			`"clients":[{"email":"user","privateKey":"` + clientPriv + `","allowedIPs":["10.8.1.2/32"],"keepAlive":25}]}`,
	}

	s := &SubService{}
	link := s.genAmneziaWGLink(inbound, "user")

	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("link does not parse: %v\n got: %s", err, link)
	}
	if u.Scheme != "amneziawg" {
		t.Fatalf("scheme = %q, want amneziawg", u.Scheme)
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
	if q.Get("address") != "10.8.1.2/32" {
		t.Fatalf("address = %q, want 10.8.1.2/32", q.Get("address"))
	}
	if q.Get("mtu") != "1420" {
		t.Fatalf("mtu = %q, want 1420", q.Get("mtu"))
	}
}

func TestGenAmneziaWGLinkWrongProtocol(t *testing.T) {
	s := &SubService{}
	vless := &model.Inbound{Protocol: model.VLESS, Settings: `{"clients":[{"email":"user"}]}`}
	if got := s.genAmneziaWGLink(vless, "user"); got != "" {
		t.Fatalf("wrong protocol should yield empty link, got %q", got)
	}
}

func TestGenAmneziaWGLinkNoKey(t *testing.T) {
	s := &SubService{}
	inbound := &model.Inbound{
		Protocol: model.AmneziaWG,
		Port:     51820,
		Settings: `{"server":{"privateKey":"x","publicKey":"y"},"clients":[{"email":"user"}]}`,
	}
	if got := s.genAmneziaWGLink(inbound, "user"); got != "" {
		t.Fatalf("client without private key should yield empty link, got %q", got)
	}
}

// Regression test for the bug where getInboundsBySubId's SQL allowlist was
// missing 'amneziawg', silently excluding every AmneziaWG client from
// subscriptions (plain/individual links, JSON, Clash) even though
// genAmneziaWGLink itself was already fully implemented and wired into
// GetLink's dispatch switch.
func TestGetInboundsBySubIdIncludesAmneziaWG(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()

	in := &model.Inbound{Port: 51820, Protocol: model.AmneziaWG, Enable: true, Tag: "awg-sub", Settings: `{"server":{"privateKey":"x","publicKey":"y"},"clients":[]}`}
	if err := db.Create(in).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	rec := &model.ClientRecord{Email: "u@awg", SubID: "subawg", Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: in.Id}).Error; err != nil {
		t.Fatalf("create link: %v", err)
	}

	s := &SubService{}
	inbounds, err := s.getInboundsBySubId("subawg")
	if err != nil {
		t.Fatalf("getInboundsBySubId: %v", err)
	}
	if len(inbounds) != 1 || inbounds[0].Id != in.Id {
		t.Fatalf("amneziawg inbound not returned for subId: %+v", inbounds)
	}
}
