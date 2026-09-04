package sub

import (
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestGenTuicLinkBasic(t *testing.T) {
	inbound := &model.Inbound{
		Listen:   "198.51.100.1",
		Port:     8443,
		Protocol: model.TUIC,
		Remark:   "tuic-test",
		Settings: `{"server":{"certificate":"/path/cert","private_key":"/path/key"},"clients":[{"uuid":"11111111-1111-1111-1111-111111111111","password":"testpassword","email":"user@test"}]}`,
	}

	s := &SubService{}
	link := s.genTuicLink(inbound, "user@test")

	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("link does not parse: %v\ngot: %s", err, link)
	}
	if u.Scheme != "tuic" {
		t.Fatalf("scheme = %q, want tuic", u.Scheme)
	}
	if u.Host != "198.51.100.1:8443" {
		t.Fatalf("host = %q, want 198.51.100.1:8443", u.Host)
	}
	if u.User.Username() != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("username = %q, want uuid", u.User.Username())
	}
	pass, ok := u.User.Password()
	if !ok || pass != "testpassword" {
		t.Fatalf("password = %q, want testpassword", pass)
	}
	q := u.Query()
	if q.Get("congestion_control") != "bbr" {
		t.Fatalf("congestion_control = %q, want bbr", q.Get("congestion_control"))
	}
	if q.Get("allow_insecure") != "0" {
		t.Fatalf("allow_insecure = %q, want 0", q.Get("allow_insecure"))
	}
	if q.Get("alpn") != "h3,spdy/3.1" {
		t.Fatalf("alpn = %q, want h3,spdy/3.1", q.Get("alpn"))
	}
	if q.Get("udp_relay_mode") != "native" {
		t.Fatalf("udp_relay_mode = %q, want native", q.Get("udp_relay_mode"))
	}
	if u.Fragment != "tuic-test-user@test" {
		t.Fatalf("fragment = %q, want tuic-test-user@test", u.Fragment)
	}
}

func TestGenTuicLinkExplicitParams(t *testing.T) {
	inbound := &model.Inbound{
		Listen:   "198.51.100.1",
		Port:     8443,
		Protocol: model.TUIC,
		Remark:   "tuic-test",
		Settings: `{"server":{"certificate":"/path/cert","private_key":"/path/key","congestion_control":"cubic","alpn":["h3"],"sni":"tuic.example.com","udp_relay_mode":"quic"},"clients":[{"uuid":"11111111-1111-1111-1111-111111111111","password":"testpassword","email":"user@test"}]}`,
	}

	s := &SubService{}
	link := s.genTuicLink(inbound, "user@test")

	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("link does not parse: %v\ngot: %s", err, link)
	}
	q := u.Query()
	if q.Get("congestion_control") != "cubic" {
		t.Fatalf("congestion_control = %q, want cubic", q.Get("congestion_control"))
	}
	if q.Get("alpn") != "h3" {
		t.Fatalf("alpn = %q, want h3", q.Get("alpn"))
	}
	if q.Get("sni") != "tuic.example.com" {
		t.Fatalf("sni = %q, want tuic.example.com", q.Get("sni"))
	}
	if q.Get("udp_relay_mode") != "quic" {
		t.Fatalf("udp_relay_mode = %q, want quic", q.Get("udp_relay_mode"))
	}
}

func TestGenTuicLinkExternalProxyFanOut(t *testing.T) {
	inbound := &model.Inbound{
		Listen:         "0.0.0.0",
		Port:           8443,
		Protocol:       model.TUIC,
		Remark:         "tuic-base",
		Settings:       `{"server":{"certificate":"/path/cert","private_key":"/path/key"},"clients":[{"uuid":"11111111-1111-1111-1111-111111111111","password":"testpassword","email":"user@test"}]}`,
		StreamSettings: `{"externalProxy":[{"dest":"host1.example.com","port":9443,"remark":"US"},{"dest":"host2.example.com","port":10443,"remark":"EU","allowInsecure":true}]}`,
	}

	s := &SubService{}
	links := s.genTuicLink(inbound, "user@test")
	lines := strings.Split(strings.TrimSpace(links), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 links for externalProxy fan-out, got %d:\n%s", len(lines), links)
	}

	u1, err := url.Parse(lines[0])
	if err != nil {
		t.Fatalf("first link does not parse: %v", err)
	}
	if u1.Host != "host1.example.com:9443" {
		t.Fatalf("first host = %q, want host1.example.com:9443", u1.Host)
	}
	if !strings.Contains(u1.Fragment, "US") {
		t.Fatalf("first fragment = %q, want to contain US", u1.Fragment)
	}
	if u1.Query().Get("allow_insecure") != "0" {
		t.Fatalf("first allow_insecure = %q, want 0", u1.Query().Get("allow_insecure"))
	}
	if u1.Query().Has("allowInsecure") {
		t.Fatalf("first link should not have allowInsecure")
	}

	u2, err := url.Parse(lines[1])
	if err != nil {
		t.Fatalf("second link does not parse: %v", err)
	}
	if u2.Host != "host2.example.com:10443" {
		t.Fatalf("second host = %q, want host2.example.com:10443", u2.Host)
	}
	if !strings.Contains(u2.Fragment, "EU") {
		t.Fatalf("second fragment = %q, want to contain EU", u2.Fragment)
	}
	if u2.Query().Get("allow_insecure") != "1" {
		t.Fatalf("second allow_insecure = %q, want 1", u2.Query().Get("allow_insecure"))
	}
	if u2.Query().Has("allowInsecure") {
		t.Fatalf("second link should not have allowInsecure")
	}
}
