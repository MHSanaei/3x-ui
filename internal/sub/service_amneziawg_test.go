package sub

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// TestGenAmneziaWGLinkFields covers the real AmneziaVPN app's vpn:// scheme:
// base64url (no padding) of a plain AmneziaWG .conf text, parsed by the real
// app as a flat "Key = Value" bag (confirmed by reading its own source).
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

	if !strings.HasPrefix(link, "vpn://") {
		t.Fatalf("link = %q, want vpn:// prefix", link)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(link, "vpn://"))
	if err != nil {
		t.Fatalf("link body does not decode as base64url: %v\n got: %s", err, link)
	}
	text := string(raw)

	for _, want := range []string{
		"[Interface]",
		"PrivateKey = " + clientPriv,
		"Address = 10.8.1.2/32",
		"MTU = 1420",
		"DNS = 8.8.8.8",
		"[Peer]",
		"PublicKey = " + serverPub,
		"Endpoint = 203.0.113.7:51820",
		"PersistentKeepalive = 25",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("decoded config missing %q\n got: %s", want, text)
		}
	}

	// The server block sets none of the 3.1 fields: none may leak into the
	// client config (a lone HeaderProtectionKey would break the handshake).
	for _, absent := range []string{"HeaderProtectionKey", "RandomTrailers", "DisableCookies", "RekeyAfterTime", "ContentPaddingAddition"} {
		if strings.Contains(text, absent) {
			t.Fatalf("config must omit unset 3.1 field %q\n got: %s", absent, text)
		}
	}
}

// TestGenAmneziaWGLink31Fields pins the AmneziaWG 3.1 [Interface] lines and
// their order in the decoded vpn:// payload — client and server configs must
// carry the identical parameter block for the tunnel to work.
func TestGenAmneziaWGLink31Fields(t *testing.T) {
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
		Remark:   "awg-31",
		Settings: `{"server":{"privateKey":"` + serverPriv + `","publicKey":"` + serverPub + `",` +
			`"jc":4,"jmin":40,"jmax":100,"s1":30,"s2":90,"s3":20,"s4":10,` +
			`"h1":"10-2000","h2":"3000-5000","h3":"6000-8000","h4":"9000-11000",` +
			`"i1":"<r 64>","i2":"<r 80>",` +
			`"headerProtectionKey":"MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivXN18=",` +
			`"contentPaddingAddition":"16-48","rekeyAfterTime":"110-140","rekeyTimeout":"4-8",` +
			`"rejectAfterTime":"190-250","keepaliveTimeout":"9-15","maxHandshakeAttempts":"20-40",` +
			`"randomTrailers":true,"disableCookies":true},` +
			`"clients":[{"email":"user","privateKey":"` + clientPriv + `","allowedIPs":["10.8.1.2/32"]}]}`,
	}

	s := &SubService{}
	link := s.genAmneziaWGLink(inbound, "user")
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(link, "vpn://"))
	if err != nil {
		t.Fatalf("link body does not decode as base64url: %v\n got: %s", err, link)
	}
	text := string(raw)

	want := []string{
		"Jc = 4",
		"H4 = 9000-11000",
		"I1 = <r 64>",
		"I2 = <r 80>",
		"HeaderProtectionKey = MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivXN18=",
		"ContentPaddingAddition = 16-48",
		"RekeyAfterTime = 110-140",
		"RekeyTimeout = 4-8",
		"RejectAfterTime = 190-250",
		"KeepaliveTimeout = 9-15",
		"MaxHandshakeAttempts = 20-40",
		"RandomTrailers = on",
		"DisableCookies = on",
		"[Peer]",
	}
	pos := -1
	for _, w := range want {
		i := strings.Index(text, w)
		if i < 0 {
			t.Fatalf("decoded config missing %q\n got: %s", w, text)
		}
		if i < pos {
			t.Fatalf("%q out of order in decoded config:\n%s", w, text)
		}
		pos = i
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
