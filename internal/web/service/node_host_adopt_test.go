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
	if _, err := svc.setRemoteTrafficLocked(nodeID, snap, false); err != nil {
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
