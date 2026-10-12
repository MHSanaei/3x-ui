package sub

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func seedTunnelSubInbound(t *testing.T, protocol model.Protocol, tag, subID, email, settings string, port int) *model.Inbound {
	t.Helper()
	db := database.GetDB()
	ib := &model.Inbound{
		UserId: 1, Tag: tag, Enable: true, Listen: "203.0.113.5", Port: port,
		Protocol: protocol, Remark: tag, Settings: settings,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create %s: %v", tag, err)
	}
	rec := &model.ClientRecord{Email: email, SubID: subID, Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("link client: %v", err)
	}
	return ib
}

// A Host on a WireGuard inbound must replace the advertised endpoint; the raw
// generator used to ignore it and always emit the panel's own address.
func TestGetSubs_WireGuardAdvertisesHostEndpoints(t *testing.T) {
	initSubDB(t)
	serverPriv, _ := mustWireguardKeypair(t)
	clientPriv, _ := mustWireguardKeypair(t)
	const email, subID = "alice@wg", "sub-wg-hosts"
	settings := fmt.Sprintf(`{"secretKey":%q,"clients":[{"email":%q,"privateKey":%q,"allowedIPs":["10.0.0.2/32"],"enable":true}]}`,
		serverPriv, email, clientPriv)
	ib := seedTunnelSubInbound(t, model.WireGuard, "wg-in", subID, email, settings, 51820)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 2, Remark: "CDN-B", Address: "wg2.example.com"})
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "CDN-A", Address: "wg.example.com", Port: 443})

	links, _, _, _, err := NewSubService("").GetSubs(subID, "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	parts := splitLinkLines(strings.Join(links, "\n"))
	want := []struct{ host, remark string }{
		{"wg.example.com:443", "wg-in-CDN-A-" + email},
		{"wg2.example.com:51820", "wg-in-CDN-B-" + email},
	}
	if len(parts) != len(want) {
		t.Fatalf("links = %d, want %d: %v", len(parts), len(want), parts)
	}
	for i, w := range want {
		u := parseWireguardSubLink(t, parts[i])
		if u.Host != w.host || u.Fragment != w.remark {
			t.Fatalf("link %d = %s#%s, want %s#%s", i, u.Host, u.Fragment, w.host, w.remark)
		}
		if u.User.Username() != clientPriv {
			t.Fatalf("link %d private key = %q, want the client's", i, u.User.Username())
		}
	}
}

// The AmneziaWG vpn:// payload carries the endpoint inside its .conf text, so a
// Host must reach the Endpoint line and the remark comment, not only the URL.
func TestGetSubs_AmneziaWGAdvertisesHostEndpoint(t *testing.T) {
	initSubDB(t)
	serverPriv, serverPub := mustWireguardKeypair(t)
	clientPriv, _ := mustWireguardKeypair(t)
	const email, subID = "alice@awg", "sub-awg-hosts"
	settings := fmt.Sprintf(`{"server":{"privateKey":%q,"publicKey":%q,"mtu":1420},"clients":[{"email":%q,"privateKey":%q,"allowedIPs":["10.8.0.2/32"],"enable":true}]}`,
		serverPriv, serverPub, email, clientPriv)
	ib := seedTunnelSubInbound(t, model.AmneziaWG, "awg-in", subID, email, settings, 51821)
	seedHost(t, &model.Host{InboundId: ib.Id, SortOrder: 1, Remark: "CDN", Address: "awg.example.com", Port: 8443})

	links, _, _, _, err := NewSubService("").GetSubs(subID, "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	parts := splitLinkLines(strings.Join(links, "\n"))
	if len(parts) != 1 {
		t.Fatalf("links = %d, want 1: %v", len(parts), parts)
	}
	conf := decodeAmneziaWGSubLink(t, parts[0])
	for _, line := range []string{"Endpoint = awg.example.com:8443", "# awg-in-CDN-" + email, "PrivateKey = " + clientPriv} {
		if !strings.Contains(conf, line) {
			t.Fatalf("config missing %q\n%s", line, conf)
		}
	}
	if strings.Contains(conf, "203.0.113.5") {
		t.Fatalf("config still advertises the inbound address\n%s", conf)
	}
}
