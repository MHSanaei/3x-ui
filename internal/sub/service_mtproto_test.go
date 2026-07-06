package sub

import (
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const mtprotoTestSecret = "ee8196fe6ed8b637d001f91d6952cfcdf07777772e636c6f7564666c6172652e636f6d"

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
