package service

import (
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
