package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestAddInbound_ImportConvertsExternalProxyToHosts reproduces the panel report:
// an inbound exported from a build that predated the hosts table carries its
// external proxies inline in streamSettings.externalProxy. The one-time startup
// migration that converts those to host rows is gated off after first run, so a
// freshly imported inbound used to land with zero hosts (its external proxies
// silently lost). AddInbound must convert them on import.
func TestAddInbound_ImportConvertsExternalProxyToHosts(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}

	stream := `{
		"network":"ws",
		"wsSettings":{"path":"/req3","host":"astr.khafanha.ir"},
		"security":"none",
		"externalProxy":[
			{"forceTls":"same","dest":"snapp.ir","port":8080,"remark":"","sni":"","alpn":[],"pinnedPeerCertSha256":[],"echConfigList":""},
			{"forceTls":"tls","dest":"cdn.example.com","port":8443,"remark":"front","sni":"sni.example.com","fingerprint":"chrome","alpn":["h2","h3"],"pinnedPeerCertSha256":["AAAA"],"echConfigList":"ECHV"}
		]
	}`
	settings := `{"clients":[{"id":"6df5616b-ebfd-4186-86d5-4bce29fe8805","email":"imp_user","subId":"s-imp","enable":true}],"decryption":"none","encryption":"none"}`

	in := &model.Inbound{
		UserId:         1,
		Tag:            "in-8080-tcp",
		Enable:         true,
		Listen:         "",
		Port:           8080,
		Protocol:       model.VLESS,
		StreamSettings: stream,
		Settings:       settings,
	}
	created, _, err := svc.AddInbound(in)
	if err != nil {
		t.Fatalf("import inbound: %v", err)
	}

	var hosts []model.Host
	if err := database.GetDB().Where("inbound_id = ?", created.Id).Order("sort_order asc").Find(&hosts).Error; err != nil {
		t.Fatalf("load hosts: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("hosts = %d, want 2 (one per externalProxy entry)", len(hosts))
	}

	a := hosts[0]
	if a.SortOrder != 0 || a.Security != "same" || a.Address != "snapp.ir" || a.Port != 8080 {
		t.Fatalf("host A mapping wrong: %+v", a)
	}
	if a.Remark == "" {
		t.Fatalf("host A remark must be backfilled for a blank externalProxy remark, got empty")
	}

	b := hosts[1]
	if b.SortOrder != 1 || b.Security != "tls" || b.Address != "cdn.example.com" || b.Port != 8443 ||
		b.Remark != "front" || b.Sni != "sni.example.com" || b.Fingerprint != "chrome" || b.EchConfigList != "ECHV" {
		t.Fatalf("host B mapping wrong: %+v", b)
	}
	if len(b.Alpn) != 2 || b.Alpn[0] != "h2" || b.Alpn[1] != "h3" {
		t.Fatalf("host B alpn = %v, want [h2 h3]", b.Alpn)
	}
	if len(b.PinnedPeerCertSha256) != 1 || b.PinnedPeerCertSha256[0] != "AAAA" {
		t.Fatalf("host B pins = %v, want [AAAA]", b.PinnedPeerCertSha256)
	}
}

// TestAddInbound_NoExternalProxyCreatesNoHosts guards the no-op path: an inbound
// built by the current UI (no externalProxy) must not gain phantom host rows.
func TestAddInbound_NoExternalProxyCreatesNoHosts(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}

	in := &model.Inbound{
		UserId:         1,
		Tag:            "in-9201-tcp",
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           9201,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp","security":"none"}`,
		Settings:       `{"clients":[{"id":"77777777-7777-7777-7777-777777777777","email":"plain","subId":"s-plain","enable":true}],"decryption":"none","encryption":"none"}`,
	}
	created, _, err := svc.AddInbound(in)
	if err != nil {
		t.Fatalf("add inbound: %v", err)
	}

	var count int64
	if err := database.GetDB().Model(&model.Host{}).Where("inbound_id = ?", created.Id).Count(&count).Error; err != nil {
		t.Fatalf("count hosts: %v", err)
	}
	if count != 0 {
		t.Fatalf("host count = %d, want 0", count)
	}
}
