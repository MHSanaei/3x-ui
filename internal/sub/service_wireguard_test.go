package sub

import (
	"fmt"
	"net/url"
	"strings"
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

func TestGenWireguardLinkMultiAllowedIPs(t *testing.T) {
	serverPriv, _, err := wgutil.GenerateWireguardKeypair()
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
		Settings: `{"secretKey":"` + serverPriv + `","clients":[{"email":"user","privateKey":"` + clientPriv + `","allowedIPs":["10.0.0.2/32","fd00::2/128"]}]}`,
	}

	s := &SubService{}
	link := s.genWireguardLink(inbound, "user")

	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("link does not parse: %v\n got: %s", err, link)
	}
	if got, want := u.Query().Get("address"), "10.0.0.2/32,fd00::2/128"; got != want {
		t.Fatalf("address = %q, want %q (all allowed IPs joined, not just the first)", got, want)
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

func mustWireguardKeypair(t *testing.T) (string, string) {
	t.Helper()
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	return priv, pub
}

func parseWireguardSubLink(t *testing.T, link string) *url.URL {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse wireguard link: %v\n got: %s", err, link)
	}
	if u.Scheme != "wireguard" {
		t.Fatalf("scheme = %q, want wireguard (%s)", u.Scheme, link)
	}
	return u
}

// The shared clients row holds the last sync's tunnel identity. Each wireguard://
// entry must keep its own key and both IPv4 and IPv6 addresses, in either sort order (#6641).
func TestGetSubs_PreservesPerInboundWireGuardIdentity(t *testing.T) {
	serverAPriv, serverAPub := mustWireguardKeypair(t)
	serverBPriv, serverBPub := mustWireguardKeypair(t)
	privA, _ := mustWireguardKeypair(t)
	privB, _ := mustWireguardKeypair(t)
	mergedPriv, _ := mustWireguardKeypair(t)

	const (
		email      = "dual@wg"
		subID      = "sub-wg-identity"
		mergedAddr = "10.9.9.9/32,fd00:9::9/128"
	)
	nodes := []struct {
		tag, listen, priv, serverPriv, serverPub string
		port                                     int
		allowed                                  []string
	}{
		{"wg-a", "203.0.113.10", privA, serverAPriv, serverAPub, 51820, []string{"10.1.0.2/32", "fd00:1::2/128"}},
		{"wg-b", "203.0.113.11", privB, serverBPriv, serverBPub, 51821, []string{"10.2.0.2/32", "fd00:2::2/128"}},
	}

	for _, tc := range []struct {
		name  string
		sort  [2]int
		order [2]int
	}{
		{name: "creation order", sort: [2]int{1, 2}, order: [2]int{0, 1}},
		{name: "reversed subscription sort", sort: [2]int{2, 1}, order: [2]int{1, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initSubDB(t)
			db := database.GetDB()
			inbounds := make([]*model.Inbound, len(nodes))
			for i, n := range nodes {
				settings := fmt.Sprintf(
					`{"secretKey":%q,"mtu":1420,"clients":[{"email":%q,"privateKey":%q,"allowedIPs":[%q,%q],"enable":true}]}`,
					n.serverPriv, email, n.priv, n.allowed[0], n.allowed[1],
				)
				ib := &model.Inbound{
					UserId: 1, Tag: n.tag, Enable: true, Listen: n.listen, Port: n.port,
					Protocol: model.WireGuard, Remark: n.tag, Settings: settings, SubSortIndex: tc.sort[i],
				}
				if err := db.Create(ib).Error; err != nil {
					t.Fatalf("create %s: %v", n.tag, err)
				}
				inbounds[i] = ib
			}
			rec := &model.ClientRecord{
				Email: email, SubID: subID, Enable: true,
				PrivateKey: mergedPriv, AllowedIPs: mergedAddr,
				PreSharedKey: "sharedpsk", KeepAlive: 25,
			}
			if err := db.Create(rec).Error; err != nil {
				t.Fatalf("create client: %v", err)
			}
			for _, ib := range inbounds {
				if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
					t.Fatalf("link %s: %v", ib.Tag, err)
				}
			}

			links, _, _, _, err := NewSubService("").GetSubs(subID, "sub.example.com")
			if err != nil {
				t.Fatalf("GetSubs: %v", err)
			}
			if len(links) != len(nodes) {
				t.Fatalf("links = %d, want %d: %v", len(links), len(nodes), links)
			}
			for outIdx, nodeIdx := range tc.order {
				n := nodes[nodeIdx]
				other := nodes[1-nodeIdx]
				u := parseWireguardSubLink(t, links[outIdx])
				if u.Host != fmt.Sprintf("%s:%d", n.listen, n.port) {
					t.Fatalf("host = %q, want %s:%d", u.Host, n.listen, n.port)
				}
				if u.User.Username() != n.priv {
					t.Fatalf("private key = %q, want inbound key %q", u.User.Username(), n.priv)
				}
				q := u.Query()
				if got, want := q.Get("address"), strings.Join(n.allowed, ","); got != want {
					t.Fatalf("address = %q, want %q", got, want)
				}
				if q.Get("publickey") != n.serverPub {
					t.Fatalf("publickey = %q, want %q", q.Get("publickey"), n.serverPub)
				}
				if q.Get("presharedkey") != "" || q.Get("keepalive") != "" {
					t.Fatalf("optional fields inherited shared values: %s", u.RawQuery)
				}
				if u.User.Username() == mergedPriv || strings.Contains(q.Get("address"), "10.9.9.9") || strings.Contains(q.Get("address"), other.allowed[0]) || strings.Contains(q.Get("address"), other.allowed[1]) {
					t.Fatalf("link borrowed another tunnel identity: %s", links[outIdx])
				}
			}
		})
	}
}

// A peer missing from settings, or settings that do not parse, must not emit the
// shared clients.wg_* identity. A sibling inbound with its own peer still does (#6641).
func TestGetSubs_WireGuardUnavailableSettingsEmitNoSharedConfig(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()
	serverPriv, serverPub := mustWireguardKeypair(t)
	validPriv, _ := mustWireguardKeypair(t)
	otherPriv, _ := mustWireguardKeypair(t)
	mergedPriv, _ := mustWireguardKeypair(t)

	const (
		email      = "dual@wg"
		subID      = "sub-wg-missing"
		mergedAddr = "10.9.9.9/32,fd00:9::9/128"
	)
	validAllowed := []string{"10.4.0.2/32", "fd00:4::2/128"}
	validSettings := fmt.Sprintf(
		`{"secretKey":%q,"clients":[{"email":%q,"privateKey":%q,"allowedIPs":[%q,%q],"enable":true}]}`,
		serverPriv, email, validPriv, validAllowed[0], validAllowed[1],
	)
	absentSettings := fmt.Sprintf(
		`{"secretKey":%q,"clients":[{"email":"someone-else@wg","privateKey":%q,"allowedIPs":["10.8.9.9/32"],"enable":true}]}`,
		serverPriv, otherPriv,
	)
	specs := []struct {
		tag, listen, settings string
		port                  int
	}{
		{"wg-bad-json", "203.0.113.31", `{not-json`, 51831},
		{"wg-absent-peer", "203.0.113.32", absentSettings, 51832},
		{"wg-valid", "203.0.113.33", validSettings, 51833},
	}
	inbounds := make([]*model.Inbound, len(specs))
	for i, sp := range specs {
		ib := &model.Inbound{
			UserId: 1, Tag: sp.tag, Enable: true, Listen: sp.listen, Port: sp.port,
			Protocol: model.WireGuard, Remark: sp.tag, Settings: sp.settings, SubSortIndex: i + 1,
		}
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create %s: %v", sp.tag, err)
		}
		inbounds[i] = ib
	}
	rec := &model.ClientRecord{
		Email: email, SubID: subID, Enable: true,
		PrivateKey: mergedPriv, AllowedIPs: mergedAddr,
		PreSharedKey: "sharedpsk", KeepAlive: 25,
	}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	for _, ib := range inbounds {
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
			t.Fatalf("link %s: %v", ib.Tag, err)
		}
	}

	links, _, _, _, err := NewSubService("").GetSubs(subID, "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("links = %d, want 1 (absent and malformed inbounds must not emit the shared row): %q", len(links), links)
	}
	u := parseWireguardSubLink(t, links[0])
	if u.Host != "203.0.113.33:51833" {
		t.Fatalf("host = %q, want the valid inbound", u.Host)
	}
	if u.User.Username() != validPriv {
		t.Fatalf("private key = %q, want inbound key", u.User.Username())
	}
	if got, want := u.Query().Get("address"), strings.Join(validAllowed, ","); got != want {
		t.Fatalf("address = %q, want %q", got, want)
	}
	if u.Query().Get("publickey") != serverPub || u.Query().Get("presharedkey") != "" || u.Query().Get("keepalive") != "" {
		t.Fatalf("query borrowed shared or foreign tunnel fields: %s", u.RawQuery)
	}
}

// Explicit empty preshared key and keepalive must not inherit the shared row (#6641).
func TestGetSubs_WireGuardEmptyOptionalTunnelFieldsDoNotInheritShared(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()
	serverPriv, serverPub := mustWireguardKeypair(t)
	clientPriv, _ := mustWireguardKeypair(t)
	mergedPriv, _ := mustWireguardKeypair(t)

	const (
		email = "optional@wg"
		subID = "sub-wg-optional"
	)
	allowed := []string{"10.5.0.2/32", "fd00:5::2/128"}
	settings := fmt.Sprintf(
		`{"secretKey":%q,"clients":[{"email":%q,"privateKey":%q,"allowedIPs":[%q,%q],"preSharedKey":"","keepAlive":0,"enable":true}]}`,
		serverPriv, email, clientPriv, allowed[0], allowed[1],
	)
	ib := &model.Inbound{
		UserId: 1, Tag: "wg-optional", Enable: true, Listen: "203.0.113.40", Port: 51840,
		Protocol: model.WireGuard, Remark: "wg-optional", Settings: settings,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	rec := &model.ClientRecord{
		Email: email, SubID: subID, Enable: true,
		PrivateKey: mergedPriv, AllowedIPs: "10.9.9.9/32,fd00:9::9/128",
		PreSharedKey: "sharedpsk", KeepAlive: 25,
	}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("link client: %v", err)
	}

	links, _, _, _, err := NewSubService("").GetSubs(subID, "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("links = %d, want 1: %q", len(links), links)
	}
	u := parseWireguardSubLink(t, links[0])
	if u.User.Username() != clientPriv {
		t.Fatalf("private key = %q, want inbound key", u.User.Username())
	}
	if got, want := u.Query().Get("address"), strings.Join(allowed, ","); got != want {
		t.Fatalf("address = %q, want %q", got, want)
	}
	if u.Query().Get("publickey") != serverPub {
		t.Fatalf("publickey = %q, want %q", u.Query().Get("publickey"), serverPub)
	}
	if u.Query().Get("presharedkey") != "" || u.Query().Get("keepalive") != "" || strings.Contains(u.RawQuery, "sharedpsk") || strings.Contains(u.Query().Get("address"), "10.9.9.9") || strings.Contains(u.Query().Get("address"), "fd00:9::9") {
		t.Fatalf("link inherited shared tunnel fields: %s", links[0])
	}
}
