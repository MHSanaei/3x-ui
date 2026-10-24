package service

import (
	"reflect"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func TestReorderHostsRejectsInvalidGroupsAtomically(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 46890, model.VLESS, `{"clients":[]}`)
	a := mkHost(t, svc, ib.Id, "a", 0)
	b := mkHost(t, svc, ib.Id, "b", 1)
	for _, invalid := range []struct {
		ids []string
		err string
	}{
		{[]string{b.GroupId, a.GroupId, b.GroupId}, "host group IDs must be nonempty and unique"},
		{[]string{b.GroupId, ""}, "host group IDs must be nonempty and unique"},
		{[]string{b.GroupId, "unknown"}, "host group not found"},
	} {
		if err := svc.ReorderHostGroups(invalid.ids); err == nil || err.Error() != invalid.err+"\n" {
			t.Fatalf("invalid group IDs %v: error = %v, want %q", invalid.ids, err, invalid.err)
		}
		groups, err := svc.GetHosts()
		if err != nil {
			t.Fatal(err)
		}
		if groups[0].GroupId != a.GroupId || groups[0].SortOrder != 0 || groups[1].GroupId != b.GroupId || groups[1].SortOrder != 1 {
			t.Fatalf("invalid request changed ordering: %+v", groups)
		}
	}
	if err := svc.ReorderHostGroups([]string{b.GroupId}); err != nil {
		t.Fatal(err)
	}
	groups, err := svc.GetHosts()
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{groups[0].GroupId, groups[1].GroupId}; !reflect.DeepEqual(got, []string{b.GroupId, a.GroupId}) || groups[1].SortOrder != 1 {
		t.Fatalf("partial reorder lost omitted groups: %+v", groups)
	}
}

func mkHost(t *testing.T, svc *HostService, inboundId int, remark string, order int) *entity.HostGroup {
	t.Helper()
	created, err := svc.AddHostGroup(&entity.HostGroup{
		InboundIds: []int{inboundId},
		Remark:     remark,
		SortOrder:  order,
		Hosts:      []string{remark + ".example.com"},
		Port:       8443,
	})
	if err != nil {
		t.Fatalf("AddHostGroup %s: %v", remark, err)
	}
	g, err := svc.GetHostGroup(created[0].GroupId)
	if err != nil {
		t.Fatalf("GetHostGroup %s: %v", remark, err)
	}
	return g
}

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
	if got[0].GroupId != h2.GroupId || got[1].GroupId != h1.GroupId {
		t.Fatalf("order = [%s,%s], want [%s,%s] (sort_order asc)", got[0].GroupId, got[1].GroupId, h2.GroupId, h1.GroupId)
	}
	if got[0].Hosts[0] != "a.example.com:8443" {
		t.Fatalf("address not persisted: %q", got[0].Hosts[0])
	}
}

func TestAddHost_RejectsUnknownInbound(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	if _, err := svc.AddHostGroup(&entity.HostGroup{InboundIds: []int{99999}, Remark: "x", Hosts: []string{"test.com"}}); err == nil {
		t.Fatalf("expected error adding host to unknown inbound")
	}
}

func TestReorderHosts(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	h1 := mkHost(t, svc, ib.Id, "h1", 0)
	h2 := mkHost(t, svc, ib.Id, "h2", 0)
	h3 := mkHost(t, svc, ib.Id, "h3", 0)

	want := []string{h3.GroupId, h1.GroupId, h2.GroupId}
	if err := svc.ReorderHostGroups(want); err != nil {
		t.Fatalf("ReorderHostGroups: %v", err)
	}
	got, _ := svc.GetHostsByInbound(ib.Id)
	for i, g := range got {
		if g.GroupId != want[i] {
			t.Fatalf("position %d = %s, want %s", i, g.GroupId, want[i])
		}
		if g.SortOrder != i {
			t.Fatalf("host %s sort_order = %d, want %d", g.GroupId, g.SortOrder, i)
		}
	}
}

func TestSetHostEnableAndBulk(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	h1 := mkHost(t, svc, ib.Id, "h1", 0)
	h2 := mkHost(t, svc, ib.Id, "h2", 1)

	if err := svc.SetHostGroupEnable(h1.GroupId, false); err != nil {
		t.Fatalf("SetHostGroupEnable: %v", err)
	}
	if g, _ := svc.GetHostGroup(h1.GroupId); g == nil || !g.IsDisabled {
		t.Fatalf("h1 should be disabled after SetHostGroupEnable(false)")
	}

	if err := svc.SetHostsGroupEnable([]string{h1.GroupId, h2.GroupId}, true); err != nil {
		t.Fatalf("SetHostsGroupEnable(true): %v", err)
	}
	for _, gid := range []string{h1.GroupId, h2.GroupId} {
		if g, _ := svc.GetHostGroup(gid); g == nil || g.IsDisabled {
			t.Fatalf("host %s should be enabled", gid)
		}
	}
	if err := svc.SetHostsGroupEnable([]string{h1.GroupId, h2.GroupId}, false); err != nil {
		t.Fatalf("SetHostsGroupEnable(false): %v", err)
	}
	for _, gid := range []string{h1.GroupId, h2.GroupId} {
		if g, _ := svc.GetHostGroup(gid); g == nil || !g.IsDisabled {
			t.Fatalf("host %s should be disabled", gid)
		}
	}
}

func TestDeleteHosts(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	h1 := mkHost(t, svc, ib.Id, "h1", 0)
	h2 := mkHost(t, svc, ib.Id, "h2", 1)
	h3 := mkHost(t, svc, ib.Id, "h3", 2)

	if err := svc.DeleteHostsGroup([]string{h1.GroupId, h3.GroupId}); err != nil {
		t.Fatalf("DeleteHostsGroup: %v", err)
	}
	got, _ := svc.GetHostsByInbound(ib.Id)
	if len(got) != 1 || got[0].GroupId != h2.GroupId {
		t.Fatalf("remaining = %v, want only h2 (%s)", got, h2.GroupId)
	}
}

func TestDeleteInboundCascadesHosts(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	inboundSvc := &InboundService{}
	ib := &model.Inbound{Tag: "casc", Enable: false, Port: 4443, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	h1 := mkHost(t, svc, ib.Id, "h1", 0)

	if _, err := inboundSvc.DelInbound(ib.Id); err != nil {
		t.Fatalf("DelInbound: %v", err)
	}
	got, _ := svc.GetHostsByInbound(ib.Id)
	if len(got) != 0 {
		t.Fatalf("hosts not cascaded on inbound delete, len = %d", len(got))
	}
	if _, err := svc.GetHostGroup(h1.GroupId); err == nil {
		t.Fatalf("expected group to be deleted after cascading")
	}
}

func TestGetAllTags(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	if _, err := svc.AddHostGroup(&entity.HostGroup{InboundIds: []int{ib.Id}, Remark: "h1", Hosts: []string{"h1.com"}, Tags: []string{"EU", "CDN"}}); err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}
	if _, err := svc.AddHostGroup(&entity.HostGroup{InboundIds: []int{ib.Id}, Remark: "h2", Hosts: []string{"h2.com"}, Tags: []string{"CDN", "FAST"}}); err != nil {
		t.Fatalf("AddHostGroup: %v", err)
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

func TestAddHostsGroup(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib1 := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	ib2 := mkInbound(t, 80, model.VLESS, `{"clients":[]}`)

	req := &entity.HostGroup{
		InboundIds: []int{ib1.Id, ib2.Id},
		Hosts:      []string{"h1.com", "h2.com:443", "[2001:db8::1]:80"},
		Remark:     "BulkRemark",
		Port:       8443,
		Security:   "same",
	}

	created, err := svc.AddHostGroup(req)
	if err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}

	if len(created) != 6 {
		t.Fatalf("expected 6 created hosts, got %d", len(created))
	}

	got1, _ := svc.GetHostsByInbound(ib1.Id)
	if len(got1) != 1 {
		t.Fatalf("expected 1 group for inbound 1, got %d", len(got1))
	}

	g := got1[0]
	if g.Remark != "BulkRemark" {
		t.Errorf("expected remark BulkRemark, got %s", g.Remark)
	}

	var foundH2Port443 bool
	var foundIPv6Port80 bool
	var foundH1DefaultPort8443 bool

	for _, hostStr := range g.Hosts {
		if hostStr == "h2.com:443" {
			foundH2Port443 = true
		}
		if hostStr == "[2001:db8::1]:80" {
			foundIPv6Port80 = true
		}
		if hostStr == "h1.com:8443" {
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

func TestAddHostGroup_OptionalAddress(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)

	created, err := svc.AddHostGroup(&entity.HostGroup{
		InboundIds: []int{ib.Id},
		Remark:     "OptionalAddressHost",
		Hosts:      nil,
		Port:       8443,
	})
	if err != nil {
		t.Fatalf("AddHostGroup with nil Hosts failed: %v", err)
	}

	if len(created) != 1 {
		t.Fatalf("expected 1 host created, got %d", len(created))
	}

	g, err := svc.GetHostGroup(created[0].GroupId)
	if err != nil {
		t.Fatalf("GetHostGroup failed: %v", err)
	}

	if len(g.Hosts) != 1 || g.Hosts[0] != ":8443" {
		t.Fatalf("expected Hosts list to contain default port fallback ':8443', got %v", g.Hosts)
	}
}

func TestUpdateHostGroup_ValidateBeforeDelete(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	h1 := mkHost(t, svc, ib.Id, "h1", 0)

	req := &entity.HostGroup{
		InboundIds: []int{99999},
		Remark:     "h1-updated",
		Hosts:      []string{"h1.com"},
	}
	if _, err := svc.UpdateHostGroup(h1.GroupId, req); err == nil {
		t.Fatalf("expected error updating host group with invalid inbound")
	}

	got, err := svc.GetHostGroup(h1.GroupId)
	if err != nil {
		t.Fatalf("original host group should not be deleted: %v", err)
	}
	if got.Remark != "h1" {
		t.Fatalf("original host group remark changed: %s", got.Remark)
	}

	req.InboundIds = []int{ib.Id}
	if _, err := svc.UpdateHostGroup(h1.GroupId, req); err != nil {
		t.Fatalf("valid update failed: %v", err)
	}
	got2, _ := svc.GetHostGroup(h1.GroupId)
	if got2.Remark != "h1-updated" {
		t.Fatalf("remark not updated: %s", got2.Remark)
	}
}

// An address/port edit must keep the row id: per-client host rule bindings
// reference ids, so a deleted row orphans every bound client.
func TestUpdateHostGroup_AddressEditKeepsHostId(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	created, err := svc.AddHostGroup(&entity.HostGroup{
		InboundIds: []int{ib.Id},
		Remark:     "cdn",
		Hosts:      []string{"a.cdn.com"},
		Port:       8443,
	})
	if err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}
	wantId := created[0].Id

	if _, err := svc.UpdateHostGroup(created[0].GroupId, &entity.HostGroup{
		InboundIds: []int{ib.Id},
		Remark:     "cdn",
		Hosts:      []string{"a2.cdn.com"},
		Port:       8443,
	}); err != nil {
		t.Fatalf("UpdateHostGroup: %v", err)
	}

	raws, err := svc.GetRawHosts()
	if err != nil {
		t.Fatalf("GetRawHosts: %v", err)
	}
	var row *model.Host
	for _, h := range raws {
		if h.GroupId == created[0].GroupId {
			if row != nil {
				t.Fatalf("expected 1 row after edit, got extra %v:%d", h.Address, h.Port)
			}
			row = h
		}
	}
	if row == nil {
		t.Fatalf("host row vanished after edit")
	}
	if row.Id != wantId {
		t.Fatalf("address edit replaced row id %d with %d; bound clients lose all links", wantId, row.Id)
	}
	if row.Address != "a2.cdn.com" || row.Port != 8443 {
		t.Fatalf("row = %s:%d, want a2.cdn.com:8443", row.Address, row.Port)
	}
}

// A host moved to another inbound has no leftover row to pair with on the
// target: the old row must be deleted and the new one inserted.
func TestUpdateHostGroup_MovedRowsDeletedAndInserted(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib1 := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	ib2 := mkInbound(t, 8443, model.VLESS, `{"clients":[]}`)
	created, err := svc.AddHostGroup(&entity.HostGroup{
		InboundIds: []int{ib1.Id},
		Remark:     "move",
		Hosts:      []string{"old.example.com"},
		Port:       8080,
	})
	if err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}

	if _, err := svc.UpdateHostGroup(created[0].GroupId, &entity.HostGroup{
		InboundIds: []int{ib2.Id},
		Remark:     "move",
		Hosts:      []string{"new.example.com"},
		Port:       9090,
	}); err != nil {
		t.Fatalf("UpdateHostGroup: %v", err)
	}

	raws, err := svc.GetRawHosts()
	if err != nil {
		t.Fatalf("GetRawHosts: %v", err)
	}
	rowsOn := func(inboundId int) []*model.Host {
		var out []*model.Host
		for _, h := range raws {
			if h.GroupId == created[0].GroupId && h.InboundId == inboundId {
				out = append(out, h)
			}
		}
		return out
	}
	if rows := rowsOn(ib1.Id); len(rows) != 0 {
		t.Fatalf("old inbound kept %d rows, want 0", len(rows))
	}
	rows := rowsOn(ib2.Id)
	if len(rows) != 1 || rows[0].Address != "new.example.com" || rows[0].Port != 9090 {
		t.Fatalf("moved row = %+v, want 1x new.example.com:9090", rows)
	}
}

// Host fields are copied by hand in buildHostRows and newHostGroup; a missed
// copy on either side silently blanks the value on the next edit-and-save.
func TestHostGroup_CipherSuitesRoundTrip(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	const suites = "TLS_AES_256_GCM_SHA384:TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"

	created, err := svc.AddHostGroup(&entity.HostGroup{
		InboundIds: []int{ib.Id}, Remark: "cs", Hosts: []string{"cs.example.com"},
		Security: "tls", CipherSuites: suites,
	})
	if err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}
	g, err := svc.GetHostGroup(created[0].GroupId)
	if err != nil {
		t.Fatalf("GetHostGroup: %v", err)
	}
	if g.CipherSuites != suites {
		t.Fatalf("CipherSuites = %q, want %q", g.CipherSuites, suites)
	}
}

func TestHostGroup_MultiSNIRoundTrip(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)

	snis := []string{"sni1.example.com", "sni2.example.com"}
	created, err := svc.AddHostGroup(&entity.HostGroup{
		InboundIds: []int{ib.Id},
		Remark:     "multi-sni",
		Hosts:      []string{"h1.example.com", "h2.example.com"},
		Security:   "tls",
		Snis:       snis,
	})
	if err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("created %d rows, want 2", len(created))
	}
	for i, row := range created {
		if !reflect.DeepEqual(row.Snis, snis) {
			t.Fatalf("row[%d].Snis = %v, want %v", i, row.Snis, snis)
		}
		if row.Sni != snis[0] {
			t.Fatalf("row[%d].Sni = %q, want primary %q", i, row.Sni, snis[0])
		}
		if row.SniSharedRandom {
			t.Fatalf("row[%d] shares its SNI by default", i)
		}
	}

	g, err := svc.GetHostGroup(created[0].GroupId)
	if err != nil {
		t.Fatalf("GetHostGroup: %v", err)
	}
	if g.Security != "tls" {
		t.Fatalf("g.Security = %q, want %q", g.Security, "tls")
	}
	if !reflect.DeepEqual(g.Snis, snis) {
		t.Fatalf("g.Snis = %v, want %v", g.Snis, snis)
	}
	if g.SniSharedRandom {
		t.Fatalf("g shares its SNI by default")
	}

	// Now update to SniSharedRandom and verify Security and mode preservation
	g.SniSharedRandom = true
	updated, err := svc.UpdateHostGroup(g.GroupId, g)
	if err != nil {
		t.Fatalf("UpdateHostGroup: %v", err)
	}
	for i, row := range updated {
		if row.Security != "tls" {
			t.Fatalf("updated row[%d].Security = %q, want %q", i, row.Security, "tls")
		}
		if !row.SniSharedRandom {
			t.Fatalf("updated row[%d] lost SniSharedRandom", i)
		}
	}

	// Verify REST script updating req.Sni independently is honored and synchronized
	g.Sni = "rest-updated.example.com"
	updatedSni, err := svc.UpdateHostGroup(g.GroupId, g)
	if err != nil {
		t.Fatalf("UpdateHostGroup with updated Sni: %v", err)
	}
	for i, row := range updatedSni {
		if row.Sni != "rest-updated.example.com" {
			t.Fatalf("row[%d].Sni = %q, want %q", i, row.Sni, "rest-updated.example.com")
		}
		if len(row.Snis) == 0 || row.Snis[0] != "rest-updated.example.com" {
			t.Fatalf("row[%d].Snis[0] = %v, want %q", i, row.Snis, "rest-updated.example.com")
		}
	}
}
