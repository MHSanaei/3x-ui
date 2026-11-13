package service

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func setupChainDB(t *testing.T) {
	t.Helper()
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	xuilogger.InitLogger(logging.ERROR)
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
}

func mustCreateChainProxy(t *testing.T, svc *ChainProxyService, proxy *model.ChainProxy) *model.ChainProxy {
	t.Helper()
	created, err := svc.Create(proxy)
	if err != nil {
		t.Fatalf("create chain proxy: %v", err)
	}
	return created
}

// The hop's server is dialed as a bare host, so a pasted URL must be reduced to
// one, and credentials must round-trip into the emitted config.
func TestChainProxyServiceCreateNormalizesServer(t *testing.T) {
	setupChainDB(t)
	svc := &ChainProxyService{}

	created := mustCreateChainProxy(t, svc, &model.ChainProxy{
		Remark: "  entry  ", Server: " socks5://User@proxy.example.com:1080/ ",
		Port: 1080, Username: "u", Password: "p", Enabled: true,
	})
	if created.Remark != "entry" {
		t.Fatalf("remark = %q, want trimmed", created.Remark)
	}
	if created.Server != "proxy.example.com" {
		t.Fatalf("server = %q, want the bare host", created.Server)
	}
	if created.Type != "socks" {
		t.Fatalf("type = %q, want the socks default", created.Type)
	}

	stored, err := svc.Get(created.Id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.Username != "u" || stored.Password != "p" {
		t.Fatalf("credentials = %q/%q, want them stored", stored.Username, stored.Password)
	}
}

func TestChainProxyServiceRejectsInvalidRows(t *testing.T) {
	cases := []struct {
		name  string
		proxy model.ChainProxy
		want  string
	}{
		{"no remark", model.ChainProxy{Server: "h.example.com", Port: 1080}, "remark is required"},
		{"unknown type", model.ChainProxy{Remark: "r", Type: "https", Server: "h.example.com", Port: 1080}, "invalid chain proxy type"},
		{"no server", model.ChainProxy{Remark: "r", Type: "socks", Port: 1080}, "server is required"},
		{"space in server", model.ChainProxy{Remark: "r", Type: "socks", Server: "two words", Port: 1080}, "bare host"},
		{"port zero", model.ChainProxy{Remark: "r", Type: "socks", Server: "h.example.com"}, "port must be between"},
		{"port too large", model.ChainProxy{Remark: "r", Type: "socks", Server: "h.example.com", Port: 70000}, "port must be between"},
		{"tls on socks", model.ChainProxy{Remark: "r", Type: "socks", Server: "h.example.com", Port: 1080, TLS: true}, "only supported for http"},
		{"newline in password", model.ChainProxy{Remark: "r", Type: "socks", Server: "h.example.com", Port: 1080, Password: "a\nb"}, "must not contain a newline"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupChainDB(t)
			svc := &ChainProxyService{}
			_, err := svc.Create(&tc.proxy)
			if err == nil {
				t.Fatalf("create accepted %+v", tc.proxy)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// An HTTPS proxy must be an http hop: xray's socks outbound has no TLS, so the
// combination is rejected at save time rather than silently dialed in plaintext.
func TestChainProxyServiceRejectsTLSSocksHop(t *testing.T) {
	setupChainDB(t)
	svc := &ChainProxyService{}
	_, err := svc.Create(&model.ChainProxy{
		Remark: "r", Type: "socks", Server: "h.example.com", Port: 1080, TLS: true,
	})
	if err == nil {
		t.Fatal("a TLS socks hop must be rejected")
	}
	if !strings.Contains(err.Error(), "only supported for http proxies") {
		t.Fatalf("err = %v, want it to name the http-only restriction", err)
	}

	// The same hop as http is accepted.
	created := mustCreateChainProxy(t, svc, &model.ChainProxy{
		Remark: "r", Type: "http", Server: "h.example.com", Port: 8443, TLS: true, Enabled: true,
	})
	if !created.TLS {
		t.Fatal("an https hop must keep its tls flag")
	}
}

// Deleting a hop must not leave references behind: selectors lose the member and
// an inbound assigned it directly falls back to a direct connection.
func TestChainProxyServiceDeleteClearsReferences(t *testing.T) {
	setupChainDB(t)
	proxySvc := &ChainProxyService{}
	selectorSvc := &ChainSelectorService{}
	db := database.GetDB()

	hop := mustCreateChainProxy(t, proxySvc, &model.ChainProxy{Remark: "hop", Type: "socks", Server: "h.example.com", Port: 1080, Enabled: true})
	other := mustCreateChainProxy(t, proxySvc, &model.ChainProxy{Remark: "other", Type: "socks", Server: "o.example.com", Port: 1081, Enabled: true})
	selector, err := selectorSvc.Create(&model.ChainSelector{Remark: "entries", Strategy: "random", ProxyIds: []int{hop.Id, other.Id}, Enabled: true})
	if err != nil {
		t.Fatalf("create selector: %v", err)
	}
	inbound := &model.Inbound{
		UserId: 1, Tag: "assigned", Enable: true, Port: 5201,
		Protocol: model.VLESS, Settings: `{"clients":[]}`, StreamSettings: `{"network":"tcp"}`,
		ChainProxyId: hop.Id,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	if err := proxySvc.Delete(hop.Id); err != nil {
		t.Fatalf("delete hop: %v", err)
	}

	storedSelector, err := selectorSvc.Get(selector.Id)
	if err != nil {
		t.Fatalf("get selector: %v", err)
	}
	if len(storedSelector.ProxyIds) != 1 || storedSelector.ProxyIds[0] != other.Id {
		t.Fatalf("selector members = %v, want only the surviving hop", storedSelector.ProxyIds)
	}
	var storedInbound model.Inbound
	if err := db.First(&storedInbound, inbound.Id).Error; err != nil {
		t.Fatalf("get inbound: %v", err)
	}
	if storedInbound.ChainProxyId != 0 {
		t.Fatalf("inbound ChainProxyId = %d, want 0", storedInbound.ChainProxyId)
	}
}

func TestChainSelectorServiceValidation(t *testing.T) {
	setupChainDB(t)
	proxySvc := &ChainProxyService{}
	selectorSvc := &ChainSelectorService{}
	hop := mustCreateChainProxy(t, proxySvc, &model.ChainProxy{Remark: "hop", Type: "socks", Server: "h.example.com", Port: 1080, Enabled: true})

	cases := []struct {
		name     string
		selector model.ChainSelector
		want     string
	}{
		{"no remark", model.ChainSelector{ProxyIds: []int{hop.Id}}, "remark is required"},
		{"no members", model.ChainSelector{Remark: "s"}, "at least one proxy"},
		{"unknown member", model.ChainSelector{Remark: "s", ProxyIds: []int{hop.Id, 999}}, "unknown chain proxy"},
		{"bad strategy", model.ChainSelector{Remark: "s", Strategy: "weighted", ProxyIds: []int{hop.Id}}, "invalid chain selector strategy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := selectorSvc.Create(&tc.selector)
			if err == nil {
				t.Fatalf("create accepted %+v", tc.selector)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}

	// Duplicates and non-positive ids are dropped, not stored.
	created, err := selectorSvc.Create(&model.ChainSelector{
		Remark: "dedup", Strategy: "", ProxyIds: []int{hop.Id, hop.Id, 0, -3}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("create selector: %v", err)
	}
	if len(created.ProxyIds) != 1 || created.ProxyIds[0] != hop.Id {
		t.Fatalf("members = %v, want the hop once", created.ProxyIds)
	}
	if created.Strategy != "random" {
		t.Fatalf("strategy = %q, want the random default", created.Strategy)
	}
}

// Deleting a selector clears the assignment of every inbound using it, so no
// inbound is left pointing at a row that no longer exists.
func TestChainSelectorServiceDeleteClearsAssignments(t *testing.T) {
	setupChainDB(t)
	proxySvc := &ChainProxyService{}
	selectorSvc := &ChainSelectorService{}
	db := database.GetDB()

	hop := mustCreateChainProxy(t, proxySvc, &model.ChainProxy{Remark: "hop", Type: "socks", Server: "h.example.com", Port: 1080, Enabled: true})
	selector, err := selectorSvc.Create(&model.ChainSelector{Remark: "entries", Strategy: "leastPing", ProxyIds: []int{hop.Id}, Enabled: true})
	if err != nil {
		t.Fatalf("create selector: %v", err)
	}
	inbound := &model.Inbound{
		UserId: 1, Tag: "assigned", Enable: true, Port: 5202,
		Protocol: model.Trojan, Settings: `{"clients":[]}`, StreamSettings: `{"network":"tcp"}`,
		ChainSelectorId: selector.Id,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	if err := selectorSvc.Delete(selector.Id); err != nil {
		t.Fatalf("delete selector: %v", err)
	}
	var storedInbound model.Inbound
	if err := db.First(&storedInbound, inbound.Id).Error; err != nil {
		t.Fatalf("get inbound: %v", err)
	}
	if storedInbound.ChainSelectorId != 0 {
		t.Fatalf("inbound ChainSelectorId = %d, want 0", storedInbound.ChainSelectorId)
	}
}

// An assignment the subscription could never honor is refused at save time:
// one kind only, the row must exist, and the protocol must be chainable.
func TestInboundChainAssignmentValidation(t *testing.T) {
	setupChainDB(t)
	proxySvc := &ChainProxyService{}
	selectorSvc := &ChainSelectorService{}
	hop := mustCreateChainProxy(t, proxySvc, &model.ChainProxy{Remark: "hop", Type: "socks", Server: "h.example.com", Port: 1080, Enabled: true})
	selector, err := selectorSvc.Create(&model.ChainSelector{Remark: "entries", Strategy: "random", ProxyIds: []int{hop.Id}, Enabled: true})
	if err != nil {
		t.Fatalf("create selector: %v", err)
	}

	newInbound := func(protocol model.Protocol) *model.Inbound {
		return &model.Inbound{
			Protocol: protocol, Settings: `{"clients":[]}`,
			StreamSettings: `{"network":"tcp","security":"none"}`,
		}
	}

	cases := []struct {
		name    string
		inbound *model.Inbound
		want    string
	}{
		{
			name: "both kinds",
			inbound: func() *model.Inbound {
				ib := newInbound(model.VLESS)
				ib.ChainProxyId = hop.Id
				ib.ChainSelectorId = selector.Id
				return ib
			}(),
			want: "not both",
		},
		{
			name: "unknown proxy",
			inbound: func() *model.Inbound {
				ib := newInbound(model.VLESS)
				ib.ChainProxyId = 999
				return ib
			}(),
			want: "chain proxy not found",
		},
		{
			name: "unknown selector",
			inbound: func() *model.Inbound {
				ib := newInbound(model.VLESS)
				ib.ChainSelectorId = 999
				return ib
			}(),
			want: "chain selector not found",
		},
		{
			name: "unsupported protocol",
			inbound: func() *model.Inbound {
				ib := newInbound(model.Hysteria)
				ib.ChainProxyId = hop.Id
				return ib
			}(),
			want: "vmess, vless, trojan and shadowsocks",
		},
		{
			name: "negative id",
			inbound: func() *model.Inbound {
				ib := newInbound(model.VLESS)
				ib.ChainProxyId = -1
				return ib
			}(),
			want: "must not be negative",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &InboundService{}
			err := svc.validateInboundChain(tc.inbound)
			if err == nil {
				t.Fatalf("assignment accepted: %+v", tc.inbound)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}

	// The valid combinations pass: a hop, a selector, and no chain at all.
	hopInbound := newInbound(model.Shadowsocks)
	hopInbound.ChainProxyId = hop.Id
	selectorInbound := newInbound(model.Trojan)
	selectorInbound.ChainSelectorId = selector.Id
	for name, ib := range map[string]*model.Inbound{
		"single hop": hopInbound,
		"selector":   selectorInbound,
		"no chain":   newInbound(model.VMESS),
	} {
		if err := (&InboundService{}).validateInboundChain(ib); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
