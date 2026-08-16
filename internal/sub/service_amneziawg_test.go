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
		Settings: `{"server":{"privateKey":"` + serverPriv + `","publicKey":"` + serverPub + `","mtu":1420,"primaryDns":"8.8.8.8",` +
			`"headerProtectionKey":"some-header-protection-key","contentPaddingAddition":"20-40",` +
			`"randomTrailers":true,"disableCookies":true},` +
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
		"HeaderProtectionKey = some-header-protection-key",
		"ContentPaddingAddition = 20-40",
		"RandomTrailers = true",
		"DisableCookies = true",
		"[Peer]",
		"PublicKey = " + serverPub,
		"Endpoint = 203.0.113.7:51820",
		"PersistentKeepalive = 25",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("decoded config missing %q\n got: %s", want, text)
		}
	}
}

// TestGenAmneziaWGLinkRandomTrailersDisableCookiesOmitted covers AmneziaWG
// 3.1's two boolean fields when left at their default (false): the config
// text must omit both lines entirely, matching the frontend's own
// buildAmneziaWGClientConfig -- neither is ever emitted as "= false".
func TestGenAmneziaWGLinkRandomTrailersDisableCookiesOmitted(t *testing.T) {
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
		Settings: `{"server":{"privateKey":"` + serverPriv + `","publicKey":"` + serverPub + `"},` +
			`"clients":[{"email":"user","privateKey":"` + clientPriv + `","allowedIPs":["10.8.1.2/32"]}]}`,
	}

	s := &SubService{}
	link := s.genAmneziaWGLink(inbound, "user")
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(link, "vpn://"))
	if err != nil {
		t.Fatalf("link body does not decode as base64url: %v\n got: %s", err, link)
	}
	text := string(raw)

	for _, unwanted := range []string{"RandomTrailers", "DisableCookies"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("decoded config should not contain %q when unset\n got: %s", unwanted, text)
		}
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
