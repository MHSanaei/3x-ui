package service

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func addExternalProxyInbound(t *testing.T, tag string, port int, stream string) *model.Inbound {
	t.Helper()
	created, _, err := (&InboundService{}).AddInbound(&model.Inbound{
		UserId: 1, Tag: tag, Enable: true, Port: port, Protocol: model.VLESS,
		StreamSettings: stream,
		Settings:       `{"clients":[{"id":"6df5616b-ebfd-4186-86d5-4bce29fe8805","email":"` + tag + `","subId":"s-` + tag + `","enable":true}],"decryption":"none","encryption":"none"}`,
	})
	if err != nil {
		t.Fatalf("AddInbound %s: %v", tag, err)
	}
	return created
}

func updateStreamSettings(t *testing.T, inbound *model.Inbound, stream string) {
	t.Helper()
	update := *inbound
	update.StreamSettings = stream
	if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
}

func hostsByGroup(t *testing.T, where string, args ...any) map[string]model.Host {
	t.Helper()
	var hosts []model.Host
	if err := database.GetDB().Where(where, args...).Find(&hosts).Error; err != nil {
		t.Fatalf("load hosts: %v", err)
	}
	byGroup := make(map[string]model.Host, len(hosts))
	for _, h := range hosts {
		if _, dup := byGroup[h.GroupId]; dup {
			t.Fatalf("two rows of group %q in %v", h.GroupId, hosts)
		}
		byGroup[h.GroupId] = h
	}
	return byGroup
}

func hostRows(t *testing.T, where string, args ...any) []model.Host {
	t.Helper()
	var hosts []model.Host
	if err := database.GetDB().Where(where, args...).Order("id asc").Find(&hosts).Error; err != nil {
		t.Fatalf("load hosts: %v", err)
	}
	return hosts
}

func onlyHost(t *testing.T, inboundId int) model.Host {
	t.Helper()
	var hosts []model.Host
	if err := database.GetDB().Where("inbound_id = ?", inboundId).Find(&hosts).Error; err != nil {
		t.Fatalf("load hosts: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("hosts = %+v, want exactly one", hosts)
	}
	return hosts[0]
}

// Subscriptions build links from hosts, so a host imported from the old
// externalProxy kept handing out its port and security after the update.
func TestUpdateInbound_ExternalProxyChangeRetargetsImportedHost(t *testing.T) {
	setupConflictDB(t)
	created := addExternalProxyInbound(t, "edge-tls", 2053, `{"network":"ws","security":"tls",
		"tlsSettings":{"certificates":[{"certificateFile":"/etc/ssl/edge.pem","keyFile":"/etc/ssl/edge.key"}]},
		"wsSettings":{"path":"/ws"},
		"externalProxy":[{"forceTls":"same","dest":"edge.example","port":2053}]}`)
	before := onlyHost(t, created.Id)

	updateStreamSettings(t, created, `{"network":"ws","security":"none","wsSettings":{"path":"/ws"},
		"externalProxy":[{"forceTls":"tls","dest":"edge.example","port":443,"sni":"edge.example","alpn":["http/1.1"],"fingerprint":"chrome"}]}`)

	after := onlyHost(t, created.Id)
	if after.Id != before.Id || after.GroupId != before.GroupId {
		t.Fatalf("host replaced (id %d group %q -> id %d group %q), want it updated in place so its group id stays valid",
			before.Id, before.GroupId, after.Id, after.GroupId)
	}
	if after.Address != "edge.example" || after.Port != 443 || after.Security != "tls" ||
		after.Sni != "edge.example" || after.Fingerprint != "chrome" || !slices.Equal(after.Alpn, []string{"http/1.1"}) {
		t.Fatalf("host = %+v, want edge.example:443 tls, sni edge.example, alpn [http/1.1], fingerprint chrome", after)
	}
}

// Only hosts still exactly as imported follow externalProxy; an imported host
// the admin edited and a host the admin created are theirs and stay as they are.
func TestUpdateInbound_ExternalProxyChangeKeepsAdminHosts(t *testing.T) {
	setupConflictDB(t)
	hostSvc := &HostService{}
	created := addExternalProxyInbound(t, "edge-mixed", 8080, `{"network":"ws","security":"none","wsSettings":{"path":"/ws"},
		"externalProxy":[
			{"forceTls":"tls","dest":"a.cdn.example","port":8443,"remark":"A"},
			{"forceTls":"tls","dest":"b.cdn.example","port":8443,"remark":"B"},
			{"forceTls":"none","dest":"c.cdn.example","port":80,"remark":"C"}
		]}`)
	imported := map[string]model.Host{}
	for _, h := range hostsByGroup(t, "inbound_id = ?", created.Id) {
		imported[h.Remark] = h
	}
	if len(imported) != 3 {
		t.Fatalf("imported hosts = %+v, want A, B and C", imported)
	}

	edited, err := hostSvc.GetHostGroup(imported["B"].GroupId)
	if err != nil {
		t.Fatalf("GetHostGroup B: %v", err)
	}
	edited.Sni = "edited.example"
	if _, err := hostSvc.UpdateHostGroup(edited.GroupId, edited); err != nil {
		t.Fatalf("UpdateHostGroup B: %v", err)
	}
	manual, err := hostSvc.AddHostGroup(&entity.HostGroup{
		InboundIds: []int{created.Id}, Remark: "manual", Hosts: []string{"manual.example"}, Port: 443, Security: "tls",
	})
	if err != nil {
		t.Fatalf("AddHostGroup manual: %v", err)
	}
	beforeUpdate := hostsByGroup(t, "inbound_id = ?", created.Id)

	updateStreamSettings(t, created, `{"network":"ws","security":"none","wsSettings":{"path":"/ws"},
		"externalProxy":[{"forceTls":"tls","dest":"a2.cdn.example","port":443,"remark":"A2"}]}`)

	got := hostsByGroup(t, "inbound_id = ?", created.Id)
	if len(got) != 3 {
		t.Fatalf("hosts = %+v, want A retargeted, edited B and the manual host (C removed)", got)
	}
	a := got[imported["A"].GroupId]
	if a.Address != "a2.cdn.example" || a.Port != 443 || a.Remark != "A2" {
		t.Fatalf("host A = %+v, want it retargeted to a2.cdn.example:443", a)
	}
	if _, ok := got[imported["C"].GroupId]; ok {
		t.Fatal("untouched import C survived although its externalProxy entry was removed")
	}
	for _, groupId := range []string{edited.GroupId, manual[0].GroupId} {
		want := beforeUpdate[groupId]
		if h := got[groupId]; h.Id != want.Id || h.Address != want.Address || h.Port != want.Port || h.Sni != want.Sni || h.Remark != want.Remark {
			t.Fatalf("admin host %q = %+v, want it untouched: %+v", groupId, h, want)
		}
	}
}

// Changing one entry must not re-import another that stayed the same: the admin's
// disable, delete, edit or share of that entry's host would come back as a duplicate.
func TestUpdateInbound_ExternalProxyChangeLeavesUnchangedEntriesAlone(t *testing.T) {
	const before = `{"network":"ws","security":"none","wsSettings":{"path":"/ws"},"externalProxy":[
		{"forceTls":"tls","dest":"a.cdn.example","port":8443,"remark":"A"},
		{"forceTls":"tls","dest":"b.cdn.example","port":8443,"remark":"B"}]}`
	const after = `{"network":"ws","security":"none","wsSettings":{"path":"/ws"},"externalProxy":[
		{"forceTls":"tls","dest":"a.cdn.example","port":8443,"remark":"A"},
		{"forceTls":"tls","dest":"b.cdn.example","port":443,"remark":"B"}]}`
	cases := []struct {
		name  string
		takeA func(t *testing.T, svc *HostService, a *entity.HostGroup, otherInbound int)
		wantA int
	}{
		{"disabled", func(t *testing.T, svc *HostService, a *entity.HostGroup, _ int) {
			if err := svc.SetHostGroupEnable(a.GroupId, false); err != nil {
				t.Fatalf("SetHostGroupEnable: %v", err)
			}
		}, 1},
		{"deleted", func(t *testing.T, svc *HostService, a *entity.HostGroup, _ int) {
			if err := svc.DeleteHostGroup(a.GroupId); err != nil {
				t.Fatalf("DeleteHostGroup: %v", err)
			}
		}, 0},
		{"edited", func(t *testing.T, svc *HostService, a *entity.HostGroup, _ int) {
			a.Remark = "renamed"
			if _, err := svc.UpdateHostGroup(a.GroupId, a); err != nil {
				t.Fatalf("UpdateHostGroup: %v", err)
			}
		}, 1},
		{"shared", func(t *testing.T, svc *HostService, a *entity.HostGroup, otherInbound int) {
			a.InboundIds = append(a.InboundIds, otherInbound)
			if _, err := svc.UpdateHostGroup(a.GroupId, a); err != nil {
				t.Fatalf("UpdateHostGroup: %v", err)
			}
		}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setupConflictDB(t)
			hostSvc := &HostService{}
			created := addExternalProxyInbound(t, "edge-pair", 8083, before)
			other := addExternalProxyInbound(t, "edge-pair-other", 8084, `{"network":"ws","security":"none","wsSettings":{"path":"/other"}}`)
			imported := map[string]model.Host{}
			for _, h := range hostsByGroup(t, "inbound_id = ?", created.Id) {
				imported[h.Remark] = h
			}
			a, err := hostSvc.GetHostGroup(imported["A"].GroupId)
			if err != nil {
				t.Fatalf("GetHostGroup A: %v", err)
			}
			c.takeA(t, hostSvc, a, other.Id)
			kept := hostRows(t, "address = ?", "a.cdn.example")

			updateStreamSettings(t, created, after)

			got := hostsByGroup(t, "inbound_id = ?", created.Id)
			if len(got) != 1+c.wantA {
				t.Fatalf("hosts = %+v, want B plus %d admin-owned A row(s), and no re-imported A", got, c.wantA)
			}
			if b := got[imported["B"].GroupId]; b.Address != "b.cdn.example" || b.Port != 443 {
				t.Fatalf("host B = %+v, want it retargeted to b.cdn.example:443", b)
			}
			if rows := hostRows(t, "address = ?", "a.cdn.example"); !reflect.DeepEqual(rows, kept) {
				t.Fatalf("A rows = %+v, want the admin's rows untouched: %+v", rows, kept)
			}
		})
	}
}

// Leftover entries pair up by position only after unchanged ones are set aside; otherwise
// removing or inserting an entry ahead of an admin-owned host shifts every pairing after it.
func TestUpdateInbound_ExternalProxyShiftLeavesAdminHostAlone(t *testing.T) {
	const entryA = `{"forceTls":"tls","dest":"a.cdn.example","port":8443,"remark":"A"}`
	const entryB = `{"forceTls":"tls","dest":"b.cdn.example","port":8443,"remark":"B"}`
	const entryC = `{"forceTls":"tls","dest":"c.cdn.example","port":8443,"remark":"C"}`
	stream := func(entries ...string) string {
		return `{"network":"ws","security":"none","wsSettings":{"path":"/ws"},"externalProxy":[` + strings.Join(entries, ",") + `]}`
	}
	cases := []struct {
		name  string
		after string
		want  []string
	}{
		{"removed ahead", stream(entryB), []string{"B edited"}},
		{"inserted ahead", stream(entryC, entryA, entryB), []string{"A", "B edited", "C"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setupConflictDB(t)
			hostSvc := &HostService{}
			created := addExternalProxyInbound(t, "edge-shift", 8086, stream(entryA, entryB))
			imported := map[string]model.Host{}
			for _, h := range hostsByGroup(t, "inbound_id = ?", created.Id) {
				imported[h.Remark] = h
			}
			edited, err := hostSvc.GetHostGroup(imported["B"].GroupId)
			if err != nil {
				t.Fatalf("GetHostGroup B: %v", err)
			}
			edited.Remark = "B edited"
			if _, err := hostSvc.UpdateHostGroup(edited.GroupId, edited); err != nil {
				t.Fatalf("UpdateHostGroup B: %v", err)
			}
			kept := hostRows(t, "group_id = ?", edited.GroupId)

			updateStreamSettings(t, created, c.after)

			var remarks []string
			for _, h := range hostsByGroup(t, "inbound_id = ?", created.Id) {
				remarks = append(remarks, h.Remark)
			}
			slices.Sort(remarks)
			if !slices.Equal(remarks, c.want) {
				t.Fatalf("host remarks = %q, want %q: B once as the admin left it, no import moved onto another entry's host", remarks, c.want)
			}
			if rows := hostRows(t, "group_id = ?", edited.GroupId); !reflect.DeepEqual(rows, kept) {
				t.Fatalf("edited B rows = %+v, want them untouched: %+v", rows, kept)
			}
		})
	}
}

// The form drops externalProxy when it switches to a protocol with its own stream
// defaults; that must not delete hosts the operator can no longer see the source of.
func TestUpdateInbound_StreamWithoutExternalProxyKeepsImportedHosts(t *testing.T) {
	setupConflictDB(t)
	created := addExternalProxyInbound(t, "edge-switch", 8085, `{"network":"ws","security":"none","wsSettings":{"path":"/ws"},
		"externalProxy":[{"forceTls":"tls","dest":"a.cdn.example","port":8443,"remark":"A"}]}`)
	before := onlyHost(t, created.Id)

	updateStreamSettings(t, created, `{"network":"tcp","security":"none","tcpSettings":{}}`)

	if after := onlyHost(t, created.Id); !reflect.DeepEqual(after, before) {
		t.Fatalf("host = %+v, want it untouched by a stream with no externalProxy key: %+v", after, before)
	}
}

// A host group the admin spread over a second inbound is theirs even when its
// fields still match the import: retargeting one row would split the group.
func TestUpdateInbound_ExternalProxyChangeKeepsSharedHostGroup(t *testing.T) {
	setupConflictDB(t)
	hostSvc := &HostService{}
	first := addExternalProxyInbound(t, "edge-shared", 8081, `{"network":"ws","security":"none","wsSettings":{"path":"/ws"},
		"externalProxy":[{"forceTls":"tls","dest":"a.cdn.example","port":8443,"remark":"A"}]}`)
	second := addExternalProxyInbound(t, "edge-other", 8082, `{"network":"ws","security":"none","wsSettings":{"path":"/other"}}`)
	group, err := hostSvc.GetHostGroup(onlyHost(t, first.Id).GroupId)
	if err != nil {
		t.Fatalf("GetHostGroup: %v", err)
	}
	group.InboundIds = []int{first.Id, second.Id}
	if _, err := hostSvc.UpdateHostGroup(group.GroupId, group); err != nil {
		t.Fatalf("UpdateHostGroup: %v", err)
	}

	updateStreamSettings(t, first, `{"network":"ws","security":"none","wsSettings":{"path":"/ws"},
		"externalProxy":[{"forceTls":"tls","dest":"a2.cdn.example","port":443,"remark":"A2"}]}`)

	for _, inboundId := range []int{first.Id, second.Id} {
		if h := onlyHost(t, inboundId); h.GroupId != group.GroupId || h.Address != "a.cdn.example" || h.Port != 8443 {
			t.Fatalf("inbound %d host = %+v, want the shared group %q left on a.cdn.example:8443", inboundId, h, group.GroupId)
		}
	}
}
