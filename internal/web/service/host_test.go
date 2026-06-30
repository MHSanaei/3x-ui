package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func mkHost(t *testing.T, svc *HostService, inboundId int, remark string, order int) *model.Host {
	t.Helper()
	h, err := svc.AddHost(&model.Host{
		InboundId: inboundId,
		Remark:    remark,
		SortOrder: order,
		Address:   remark + ".example.com",
		Port:      8443,
	})
	if err != nil {
		t.Fatalf("AddHost %s: %v", remark, err)
	}
	return h
}

// TestAddHost_GetHostsByInbound: create persists; query returns by inbound,
// ordered by sort_order then id.
func TestAddHost_GetHostsByInbound(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	h1 := mkHost(t, svc, ib.Id, "b", 2)
	h2 := mkHost(t, svc, ib.Id, "a", 1)

	got, err := svc.GetHostsByInbound(ib.Id)
	if err != nil {
		t.Fatalf("GetHostsByInbound: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Id != h2.Id || got[1].Id != h1.Id {
		t.Fatalf("order = [%d,%d], want [%d,%d] (sort_order asc)", got[0].Id, got[1].Id, h2.Id, h1.Id)
	}
	if got[0].Address != "a.example.com" {
		t.Fatalf("address not persisted: %q", got[0].Address)
	}
}

// TestAddHost_RejectsUnknownInbound: a host whose inbound does not exist is refused.
func TestAddHost_RejectsUnknownInbound(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	if _, err := svc.AddHost(&model.Host{InboundId: 99999, Remark: "x"}); err == nil {
		t.Fatalf("expected error adding host to unknown inbound")
	}
}

// TestReorderHosts: reorder updates sort_order and re-query reflects new order.
func TestReorderHosts(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	h1 := mkHost(t, svc, ib.Id, "h1", 0)
	h2 := mkHost(t, svc, ib.Id, "h2", 0)
	h3 := mkHost(t, svc, ib.Id, "h3", 0)

	want := []int{h3.Id, h1.Id, h2.Id}
	if err := svc.ReorderHosts(want); err != nil {
		t.Fatalf("ReorderHosts: %v", err)
	}
	got, _ := svc.GetHostsByInbound(ib.Id)
	for i, h := range got {
		if h.Id != want[i] {
			t.Fatalf("position %d = %d, want %d", i, h.Id, want[i])
		}
		if h.SortOrder != i {
			t.Fatalf("host %d sort_order = %d, want %d", h.Id, h.SortOrder, i)
		}
	}
}

// TestSetHostEnableAndBulk: per-row and bulk enable/disable toggles persist.
func TestSetHostEnableAndBulk(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	h1 := mkHost(t, svc, ib.Id, "h1", 0)
	h2 := mkHost(t, svc, ib.Id, "h2", 1)

	if err := svc.SetHostEnable(h1.Id, false); err != nil {
		t.Fatalf("SetHostEnable: %v", err)
	}
	if g, _ := svc.GetHost(h1.Id); g == nil || !g.IsDisabled {
		t.Fatalf("h1 should be disabled after SetHostEnable(false)")
	}

	if err := svc.SetHostsEnable([]int{h1.Id, h2.Id}, true); err != nil {
		t.Fatalf("SetHostsEnable(true): %v", err)
	}
	for _, id := range []int{h1.Id, h2.Id} {
		if g, _ := svc.GetHost(id); g == nil || g.IsDisabled {
			t.Fatalf("host %d should be enabled", id)
		}
	}
	if err := svc.SetHostsEnable([]int{h1.Id, h2.Id}, false); err != nil {
		t.Fatalf("SetHostsEnable(false): %v", err)
	}
	for _, id := range []int{h1.Id, h2.Id} {
		if g, _ := svc.GetHost(id); g == nil || !g.IsDisabled {
			t.Fatalf("host %d should be disabled", id)
		}
	}
}

// TestDeleteHosts: bulk delete removes exactly the named rows.
func TestDeleteHosts(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	h1 := mkHost(t, svc, ib.Id, "h1", 0)
	h2 := mkHost(t, svc, ib.Id, "h2", 1)
	h3 := mkHost(t, svc, ib.Id, "h3", 2)

	if err := svc.DeleteHosts([]int{h1.Id, h3.Id}); err != nil {
		t.Fatalf("DeleteHosts: %v", err)
	}
	got, _ := svc.GetHostsByInbound(ib.Id)
	if len(got) != 1 || got[0].Id != h2.Id {
		t.Fatalf("remaining = %v, want only h2 (%d)", got, h2.Id)
	}
}

// TestDeleteInboundCascadesHosts: deleting an inbound deletes its hosts.
func TestDeleteInboundCascadesHosts(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	inboundSvc := &InboundService{}
	// Disabled local inbound so DelInbound skips the runtime push.
	ib := &model.Inbound{Tag: "casc", Enable: false, Port: 4443, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	mkHost(t, svc, ib.Id, "h1", 0)
	mkHost(t, svc, ib.Id, "h2", 1)

	if _, err := inboundSvc.DelInbound(ib.Id); err != nil {
		t.Fatalf("DelInbound: %v", err)
	}
	got, _ := svc.GetHostsByInbound(ib.Id)
	if len(got) != 0 {
		t.Fatalf("hosts not cascaded on inbound delete, len = %d", len(got))
	}
}

// TestGetAllTags: distinct, sorted tags across all hosts.
func TestGetAllTags(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	if _, err := svc.AddHost(&model.Host{InboundId: ib.Id, Remark: "h1", Tags: []string{"EU", "CDN"}}); err != nil {
		t.Fatalf("AddHost: %v", err)
	}
	if _, err := svc.AddHost(&model.Host{InboundId: ib.Id, Remark: "h2", Tags: []string{"CDN", "FAST"}}); err != nil {
		t.Fatalf("AddHost: %v", err)
	}
	tags, err := svc.GetAllTags()
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}
	want := []string{"CDN", "EU", "FAST"}
	if len(tags) != len(want) {
		t.Fatalf("tags = %v, want %v", tags, want)
	}
	for i := range want {
		if tags[i] != want[i] {
			t.Fatalf("tags = %v, want %v", tags, want)
		}
	}
}

func TestAddHostsBulk(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib1 := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	ib2 := mkInbound(t, 80, model.VLESS, `{"clients":[]}`)

	req := &entity.BulkAddHostReq{
		InboundIds: []int{ib1.Id, ib2.Id},
		Hosts:      []string{"h1.com", "h2.com:443", "[2001:db8::1]:80"},
		Remark:     "BulkRemark",
		Port:       8443,
		Security:   "same",
	}

	created, err := svc.AddHostsBulk(req)
	if err != nil {
		t.Fatalf("AddHostsBulk: %v", err)
	}

	if len(created) != 6 {
		t.Fatalf("expected 6 created hosts, got %d", len(created))
	}

	got1, _ := svc.GetHostsByInbound(ib1.Id)
	if len(got1) != 3 {
		t.Fatalf("expected 3 hosts for inbound 1, got %d", len(got1))
	}

	var foundH2Port443 bool
	var foundIPv6Port80 bool
	var foundH1DefaultPort8443 bool

	for _, h := range got1 {
		if h.Remark != "BulkRemark" {
			t.Errorf("expected remark BulkRemark, got %s", h.Remark)
		}
		if h.Address == "h2.com" && h.Port == 443 {
			foundH2Port443 = true
		}
		if h.Address == "2001:db8::1" && h.Port == 80 {
			foundIPv6Port80 = true
		}
		if h.Address == "h1.com" && h.Port == 8443 {
			foundH1DefaultPort8443 = true
		}
	}

	if !foundH2Port443 {
		t.Error("missing custom port override host h2.com:443")
	}
	if !foundIPv6Port80 {
		t.Error("missing IPv6 host with port override [2001:db8::1]:80")
	}
	if !foundH1DefaultPort8443 {
		t.Error("missing default port fallback host h1.com:8443")
	}
}

func TestParseHostAndPort_IPv6EdgeCases(t *testing.T) {
	tests := []struct {
		input       string
		defaultPort int
		wantAddr    string
		wantPort    int
	}{
		{"2001:db8::1", 8443, "2001:db8::1", 8443},
		{"[2001:db8::1]:80", 8443, "2001:db8::1", 80},
		{"h1.com:443", 8443, "h1.com", 443},
		{"h1.com", 8443, "h1.com", 8443},
	}

	for _, tc := range tests {
		addr, port := parseHostAndPort(tc.input, tc.defaultPort)
		if addr != tc.wantAddr || port != tc.wantPort {
			t.Errorf("parseHostAndPort(%q, %d) = (%q, %d); want (%q, %d)",
				tc.input, tc.defaultPort, addr, port, tc.wantAddr, tc.wantPort)
		}
	}
}

func TestParseHostAndPort_AdversarialStressCases(t *testing.T) {
	tests := []struct {
		input       string
		defaultPort int
		wantAddr    string
		wantPort    int
	}{
		{"", 8443, "", 8443},
		{" ", 8443, "", 8443},
		{"h1.com: ", 8443, "h1.com:", 8443},
		{"h1.com: -1", 8443, "h1.com: -1", 8443},
		{"h1.com:-1", 8443, "h1.com:-1", 8443},
		{"h1.com:0", 8443, "h1.com", 0},
		{"h1.com:65535", 8443, "h1.com", 65535},
		{"h1.com:65536", 8443, "h1.com:65536", 8443},
		{"h1.com:80a", 8443, "h1.com:80a", 8443},
		{"h1.com:123:456", 8443, "h1.com:123:456", 8443},
		{"[2001:db8::1]", 8443, "2001:db8::1", 8443},
		{"[2001:db8::1]:80", 8443, "2001:db8::1", 80},
		{"2001:db8::1", 8443, "2001:db8::1", 8443},
		{"[2001:db8::1]:65536", 8443, "[2001:db8::1]:65536", 8443},
		{"[]:80", 8443, "", 80},
		{"[:]::80", 8443, "[:]:", 80},
		{"h1.com:", 8443, "h1.com:", 8443},
		{"h1.com:123:", 8443, "h1.com:123:", 8443},
		{" h1.com : 80 ", 8443, "h1.com : 80", 8443},
		{" [2001:db8::1]:80 ", 8443, "2001:db8::1", 80},
		{"[2001:db8::1]:+80", 8443, "2001:db8::1", 80},
		{"[2001:db8::1]:080", 8443, "2001:db8::1", 80},
		{"[2001:db8::1]80", 8443, "[2001:db8::1]80", 8443},
		{"[::1]", 8443, "::1", 8443},
		{"[2001:db8::1", 8443, "[2001:db8:", 1},
		{"[2001:db8::1]:-80", 8443, "[2001:db8::1]:-80", 8443},
		{"h1.com:443:80", 8443, "h1.com:443:80", 8443},
		{"[2001:db8::1]::80", 8443, "[2001:db8::1]:", 80},
	}

	for _, tc := range tests {
		addr, port := parseHostAndPort(tc.input, tc.defaultPort)
		if addr != tc.wantAddr || port != tc.wantPort {
			t.Errorf("parseHostAndPort(%q, %d) = (%q, %d); want (%q, %d)",
				tc.input, tc.defaultPort, addr, port, tc.wantAddr, tc.wantPort)
		}
	}
}
