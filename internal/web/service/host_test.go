package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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
