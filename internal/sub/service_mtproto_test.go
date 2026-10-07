package sub

import (
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const (
	mtprotoTestSecret        = "ee8196fe6ed8b637d001f91d6952cfcdf07777772e636c6f7564666c6172652e636f6d"
	mtprotoTestSecuredSecret = "dd8196fe6ed8b637d001f91d6952cfcdf0"
)

func TestGenMtprotoLinkFields(t *testing.T) {
	inbound := &model.Inbound{
		Listen:   "203.0.113.7",
		Port:     8443,
		Protocol: model.MTProto,
		Remark:   "mt-sub",
		Settings: `{"fakeTlsDomain":"www.cloudflare.com","clients":[{"email":"user","enable":true,"secret":"` + mtprotoTestSecret + `"}]}`,
	}

	s := &SubService{}
	link := s.genMtprotoLink(inbound, "user")

	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("link does not parse: %v\n got: %s", err, link)
	}
	if u.Scheme != "tg" || u.Host != "proxy" {
		t.Fatalf("link = %q, want a tg://proxy deep link", link)
	}
	q := u.Query()
	if q.Get("server") != "203.0.113.7" {
		t.Fatalf("server = %q, want 203.0.113.7", q.Get("server"))
	}
	if q.Get("port") != "8443" {
		t.Fatalf("port = %q, want 8443", q.Get("port"))
	}
	if q.Get("secret") != mtprotoTestSecret {
		t.Fatalf("secret = %q, want the client's FakeTLS secret", q.Get("secret"))
	}
	if u.Fragment != "" {
		t.Fatalf("link carries a #%s fragment; tg://proxy links must have no remark fragment", u.Fragment)
	}
}

func TestGenMtprotoLinkWrongProtocol(t *testing.T) {
	s := &SubService{}
	vless := &model.Inbound{Protocol: model.VLESS, Settings: `{"clients":[{"email":"user"}]}`}
	if got := s.genMtprotoLink(vless, "user"); got != "" {
		t.Fatalf("wrong protocol should yield empty link, got %q", got)
	}
}

func TestGenMtprotoLinkNoSecret(t *testing.T) {
	s := &SubService{}
	inbound := &model.Inbound{
		Protocol: model.MTProto,
		Port:     8443,
		Settings: `{"fakeTlsDomain":"www.cloudflare.com","clients":[{"email":"user"}]}`,
	}
	if got := s.genMtprotoLink(inbound, "user"); got != "" {
		t.Fatalf("client without secret should yield empty link, got %q", got)
	}
}

func TestGetSubsMtprotoUsesHostEndpoint(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()

	inbound := &model.Inbound{
		Listen:   "127.0.0.1",
		Port:     4060,
		Protocol: model.MTProto,
		Enable:   true,
		Tag:      "mt-public-port",
		Settings: `{"clients":[{"email":"u@mt","enable":true,"subId":"sub-public-port","secret":"` + mtprotoTestSecret + `"}]}`,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := db.Create(&model.Host{
		InboundId: inbound.Id,
		Remark:    "public",
		Address:   "proxy.example.com",
		Port:      443,
		Security:  "same",
	}).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}
	client := &model.ClientRecord{Email: "u@mt", SubID: "sub-public-port", Enable: true, Secret: mtprotoTestSecret}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatalf("attach client: %v", err)
	}

	links, _, _, _, err := NewSubService("").GetSubs(client.SubID, "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("links = %d, want 1: %v", len(links), links)
	}
	u, err := url.Parse(links[0])
	if err != nil {
		t.Fatalf("parse link: %v", err)
	}
	if got := u.Query().Get("server"); got != "proxy.example.com" {
		t.Fatalf("server = %q, want proxy.example.com", got)
	}
	if got := u.Query().Get("port"); got != "443" {
		t.Fatalf("port = %q, want public host port 443", got)
	}
	clientLinks := NewLinkProvider().LinksForClient("sub.example.com", inbound, client.Email)
	if len(clientLinks) != 1 {
		t.Fatalf("client links = %d, want 1: %v", len(clientLinks), clientLinks)
	}
	clientURL, err := url.Parse(clientLinks[0])
	if err != nil {
		t.Fatalf("parse client link: %v", err)
	}
	if got := clientURL.Query().Get("server"); got != "proxy.example.com" {
		t.Fatalf("client link server = %q, want proxy.example.com", got)
	}
	if got := clientURL.Query().Get("port"); got != "443" {
		t.Fatalf("client link port = %q, want 443", got)
	}
}

// Regression: an mtproto inbound must resolve for a subscription id the same way
// every other client-bearing protocol does. It was previously dropped from the
// getInboundsBySubId protocol allowlist, so multi-client MTProto subscriptions
// (and the public sub page) emitted no tg://proxy link at all.
func TestGetInboundsBySubIdIncludesMtproto(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()

	in := &model.Inbound{
		Port:     8443,
		Protocol: model.MTProto,
		Enable:   true,
		Tag:      "mt-sub",
		Settings: `{"fakeTlsDomain":"www.cloudflare.com","clients":[{"email":"u@mt","enable":true,"subId":"submt","secret":"` + mtprotoTestSecret + `"}]}`,
	}
	if err := db.Create(in).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	rec := &model.ClientRecord{Email: "u@mt", SubID: "submt", Enable: true, Secret: mtprotoTestSecret}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: in.Id}).Error; err != nil {
		t.Fatalf("create link: %v", err)
	}

	s := &SubService{}
	inbounds, err := s.getInboundsBySubId("submt")
	if err != nil {
		t.Fatalf("getInboundsBySubId: %v", err)
	}
	if len(inbounds) != 1 || inbounds[0].Id != in.Id {
		t.Fatalf("mtproto inbound not returned for subId: %+v", inbounds)
	}

	links, emails, _, _, err := s.GetSubs("submt", "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 1 || len(emails) != 1 || emails[0] != "u@mt" {
		t.Fatalf("subscription did not emit the mtproto client: links=%v emails=%v", links, emails)
	}
	if !strings.HasPrefix(links[0], "tg://proxy") || !strings.Contains(links[0], "secret="+mtprotoTestSecret) {
		t.Fatalf("subscription link is not a tg://proxy carrying the client secret: %q", links[0])
	}
}

func TestGenMtprotoLinkSecured(t *testing.T) {
	for _, tc := range []struct {
		name    string
		secured string
		want    []string
	}{
		{"secured", `"secured":true,`, []string{mtprotoTestSecret, mtprotoTestSecuredSecret}},
		{"off", `"secured":false,`, []string{mtprotoTestSecret}},
		{"absent", ``, []string{mtprotoTestSecret}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inbound := &model.Inbound{
				Listen:   "203.0.113.7",
				Port:     8443,
				Protocol: model.MTProto,
				Settings: `{` + tc.secured + `"clients":[{"email":"user","enable":true,"secret":"` + mtprotoTestSecret + `"}]}`,
			}
			lines := splitLinkLines((&SubService{}).genMtprotoLink(inbound, "user"))
			if len(lines) != len(tc.want) {
				t.Fatalf("links = %v, want %d", lines, len(tc.want))
			}
			for i, line := range lines {
				u, err := url.Parse(line)
				if err != nil {
					t.Fatalf("link %q does not parse: %v", line, err)
				}
				q := u.Query()
				if q.Get("secret") != tc.want[i] || q.Get("server") != "203.0.113.7" || q.Get("port") != "8443" {
					t.Fatalf("link %d = %q, want secret %q on 203.0.113.7:8443", i, line, tc.want[i])
				}
			}
		})
	}
}

// A secured inbound behind a managed host must give the dd link the host's
// address too, on the client page as well as in the subscription.
func TestSecuredMtprotoLinksFollowHostEndpoint(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()

	inbound := &model.Inbound{
		Listen:   "127.0.0.1",
		Port:     4061,
		Protocol: model.MTProto,
		Enable:   true,
		Tag:      "mt-secured-host",
		Settings: `{"secured":true,"clients":[{"email":"dd@mt","enable":true,"subId":"sub-secured","secret":"` + mtprotoTestSecret + `"}]}`,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := db.Create(&model.Host{InboundId: inbound.Id, Remark: "public", Address: "proxy.example.com", Port: 443, Security: "same"}).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}
	client := &model.ClientRecord{Email: "dd@mt", SubID: "sub-secured", Enable: true, Secret: mtprotoTestSecret}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatalf("attach client: %v", err)
	}

	subLinks, err := NewLinkProvider().SubLinksForSubId("sub.example.com", client.SubID)
	if err != nil {
		t.Fatalf("SubLinksForSubId: %v", err)
	}
	clientLinks := NewLinkProvider().LinksForClient("sub.example.com", inbound, client.Email)
	for name, links := range map[string][]string{"subscription": subLinks, "client": clientLinks} {
		if len(links) != 2 {
			t.Fatalf("%s links = %v, want the ee and the dd link", name, links)
		}
		for i, wantSecret := range []string{mtprotoTestSecret, mtprotoTestSecuredSecret} {
			u, err := url.Parse(links[i])
			if err != nil {
				t.Fatalf("%s link %q does not parse: %v", name, links[i], err)
			}
			q := u.Query()
			if q.Get("server") != "proxy.example.com" || q.Get("port") != "443" || q.Get("secret") != wantSecret {
				t.Fatalf("%s link %d = %q, want secret %q on proxy.example.com:443", name, i, links[i], wantSecret)
			}
		}
	}
}

// A WEB-enabled inbound adds one tg://webproxy link to the WEB domain after the
// regular ones; the key keeps or drops the dd prefix by secret mode, and an
// invalid or incomplete WEB section yields no link because mtg would not serve it.
func TestGenMtprotoLinkWeb(t *testing.T) {
	const plainKey = "8196fe6ed8b637d001f91d6952cfcdf0"
	for _, tc := range []struct {
		name, web string
		want      string
	}{
		{"ddDefault", `"web":{"bindTo":"127.0.0.1:18080","host":"web.example.com"},`, "tg://webproxy?secret=" + mtprotoTestSecuredSecret + "&server=web.example.com"},
		{"plain", `"web":{"bindTo":"127.0.0.1:18080","host":"web.example.com","secretMode":"plain"},`, "tg://webproxy?secret=" + plainKey + "&server=web.example.com"},
		{"noBind", `"web":{"host":"web.example.com"},`, ""},
		{"publicBind", `"web":{"bindTo":"0.0.0.0:18080","host":"web.example.com"},`, ""},
		{"absent", ``, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inbound := &model.Inbound{
				Listen:   "203.0.113.7",
				Port:     8443,
				Protocol: model.MTProto,
				Settings: `{` + tc.web + `"clients":[{"email":"user","enable":true,"secret":"` + mtprotoTestSecret + `"}]}`,
			}
			lines := splitLinkLines((&SubService{}).genMtprotoLink(inbound, "user"))
			if tc.want == "" {
				if len(lines) != 1 || strings.Contains(lines[0], "webproxy") {
					t.Fatalf("links = %v, want only the FakeTLS link", lines)
				}
				return
			}
			if len(lines) != 2 || lines[1] != tc.want {
				t.Fatalf("links = %v, want the FakeTLS link and %q", lines, tc.want)
			}
		})
	}
}
