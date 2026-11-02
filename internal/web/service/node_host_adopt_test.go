package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func TestSetRemoteTraffic_AdoptsNodeHostRows(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	const nodeID = 6
	if err := db.Create(&model.Node{
		Id:       nodeID,
		Name:     "host-node",
		Address:  "10.0.0.6",
		Port:     2053,
		ApiToken: "t",
		Guid:     "host-node-guid",
	}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	snap := &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{{
			Id:       77,
			Tag:      "host-adopt-443",
			Enable:   true,
			Port:     443,
			Protocol: model.VLESS,
			Settings: `{"clients":[]}`,
		}},
		HostGroups: []*entity.HostGroup{{
			GroupId:     "g-node",
			InboundIds:  []int{77, 99},
			Hosts:       []string{"cdn.example.com:8443"},
			Remark:      "cdn",
			Security:    "tls",
			Sni:         "sni.example.com",
			Fingerprint: "firefox",
		}},
	}

	svc := InboundService{}
	if _, err := svc.setRemoteTrafficLocked(nodeID, snap, false, false); err != nil {
		t.Fatalf("setRemoteTrafficLocked: %v", err)
	}

	var central model.Inbound
	if err := db.Where("tag = ?", "host-adopt-443").First(&central).Error; err != nil {
		t.Fatalf("load adopted inbound: %v", err)
	}
	var hosts []model.Host
	if err := db.Where("inbound_id = ?", central.Id).Find(&hosts).Error; err != nil {
		t.Fatalf("load adopted hosts: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("adopted host rows = %d, want 1", len(hosts))
	}
	h := hosts[0]
	if h.GroupId != "g-node" || h.Address != "cdn.example.com" || h.Port != 8443 ||
		h.Security != "tls" || h.Sni != "sni.example.com" || h.Fingerprint != "firefox" || h.Remark != "cdn" {
		t.Fatalf("adopted host mismatch: %+v", h)
	}

	var total int64
	if err := db.Model(&model.Host{}).Count(&total).Error; err != nil {
		t.Fatalf("count hosts: %v", err)
	}
	if total != 1 {
		t.Fatalf("total host rows = %d, want 1 (group member for un-adopted node inbound 99 must not materialize)", total)
	}
}

func TestSetRemoteTraffic_SweptInboundTakesItsDependentsAlong(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	const nodeID = 6
	if err := db.Create(&model.Node{Id: nodeID, Name: "host-node", Address: "10.0.0.6", Port: 2053, ApiToken: "t", Guid: "host-node-guid"}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	inbound := func(id, port int, tag string) *model.Inbound {
		return &model.Inbound{Id: id, Tag: tag, Enable: true, Port: port, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	}
	group := func(groupId string, inboundId int) *entity.HostGroup {
		return &entity.HostGroup{GroupId: groupId, InboundIds: []int{inboundId}, Hosts: []string{"cdn.example.com:8443"}, Remark: "cdn"}
	}

	svc := InboundService{}
	if _, err := svc.setRemoteTrafficLocked(nodeID, &runtime.TrafficSnapshot{
		Inbounds:   []*model.Inbound{inbound(1, 443, "swept-443"), inbound(2, 444, "kept-444")},
		HostGroups: []*entity.HostGroup{group("g-swept", 1), group("g-kept", 2)},
	}, false, false); err != nil {
		t.Fatalf("adopting merge: %v", err)
	}
	swept, kept := loadInboundByTag(t, "swept-443"), loadInboundByTag(t, "kept-444")
	balancer := &model.SubBalancer{Remark: "both", InboundIds: []int{swept.Id, kept.Id}, Enabled: true}
	if err := db.Create(balancer).Error; err != nil {
		t.Fatalf("create balancer: %v", err)
	}

	if _, err := svc.setRemoteTrafficLocked(nodeID, &runtime.TrafficSnapshot{
		Inbounds: []*model.Inbound{inbound(2, 444, "kept-444")},
	}, false, false); err != nil {
		t.Fatalf("sweeping merge: %v", err)
	}

	var groups []string
	if err := db.Model(&model.Host{}).Order("group_id").Pluck("group_id", &groups).Error; err != nil {
		t.Fatalf("load hosts: %v", err)
	}
	if len(groups) != 1 || groups[0] != "g-kept" {
		t.Fatalf("host groups after the sweep = %v, want [g-kept]", groups)
	}
	if err := db.First(balancer, balancer.Id).Error; err != nil {
		t.Fatalf("reload balancer: %v", err)
	}
	if len(balancer.InboundIds) != 1 || balancer.InboundIds[0] != kept.Id {
		t.Fatalf("balancer members after the sweep = %v, want [%d]", balancer.InboundIds, kept.Id)
	}
}

// floodSnapshot reports inboundCount new inbounds (ids and ports from firstPort)
// and makes every group a member of all of them.
func floodSnapshot(tagPrefix string, firstPort, inboundCount int, groups ...*entity.HostGroup) *runtime.TrafficSnapshot {
	snap := &runtime.TrafficSnapshot{HostGroups: groups}
	ids := make([]int, inboundCount)
	for i := range ids {
		ids[i] = firstPort + i
		snap.Inbounds = append(snap.Inbounds, &model.Inbound{
			Id: firstPort + i, Tag: fmt.Sprintf("%s-%d", tagPrefix, i), Enable: true, Port: firstPort + i, Protocol: model.VLESS, Settings: `{"clients":[]}`,
		})
	}
	for _, g := range groups {
		g.InboundIds = ids
	}
	return snap
}

func countHostRows(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := database.GetDB().Model(&model.Host{}).Where(query, args...).Count(&n).Error; err != nil {
		t.Fatalf("count hosts: %v", err)
	}
	return n
}

// A group is stored once per address and per inbound, so a small hosts/list answer
// must not be able to write an unbounded number of rows or bytes, in one merge or many.
func TestSetRemoteTraffic_BoundsAdoptedHostRows(t *testing.T) {
	setupConflictDB(t)

	const nodeID = 7
	if err := database.GetDB().Create(&model.Node{
		Id: nodeID, Name: "flood-node", Address: "10.0.0.7", Port: 2053, ApiToken: "t", Guid: "flood-node-guid",
	}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	addresses := make([]string, 50)
	for i := range addresses {
		addresses[i] = fmt.Sprintf("h%d.example", i)
	}
	snap := floodSnapshot("flood", 30000, 100,
		&entity.HostGroup{GroupId: "g-big", Hosts: []string{"cdn.example"}, Remark: "big", Sni: strings.Repeat("s", 100_000)},
		&entity.HostGroup{GroupId: "g-many", Hosts: addresses, Remark: "many", Sni: "sni.example"},
	)

	svc := InboundService{}
	if _, err := svc.setRemoteTrafficLocked(nodeID, snap, false, false); err != nil {
		t.Fatalf("setRemoteTrafficLocked: %v", err)
	}
	if big := countHostRows(t, "group_id = ?", "g-big"); big != 0 {
		t.Fatalf("a 100000-byte group was stored %d times", big)
	}
	if total := countHostRows(t, "1 = 1"); total != maxAdoptedHostRows {
		t.Fatalf("one merge adopted %d host rows, want %d", total, maxAdoptedHostRows)
	}

	more := floodSnapshot("more", 31000, 10, &entity.HostGroup{GroupId: "g-more", Hosts: addresses, Remark: "more"})
	more.Inbounds = append(more.Inbounds, snap.Inbounds...)
	if _, err := svc.setRemoteTrafficLocked(nodeID, more, false, false); err != nil {
		t.Fatalf("second setRemoteTrafficLocked: %v", err)
	}
	if total := countHostRows(t, "1 = 1"); total != maxAdoptedHostRows {
		t.Fatalf("a second merge raised the node's host rows to %d, want %d", total, maxAdoptedHostRows)
	}
}
