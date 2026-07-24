package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func makeInboundWithTrafficRatio(tag string, port int, ratio float64) *model.Inbound {
	return &model.Inbound{
		UserId:         1,
		Tag:            tag,
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           port,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
		Settings:       `{"clients":[]}`,
		TrafficRatio:   ratio,
	}
}

func TestUpdateInbound_PersistsTrafficRatio(t *testing.T) {
	setupConflictDB(t)

	ib := makeInboundWithTrafficRatio("in-7101-tcp", 7101, 1)
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	update := *ib
	update.TrafficRatio = 2.5

	svc := &InboundService{}
	got, _, err := svc.UpdateInbound(&update)
	if err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	if got.TrafficRatio != 2.5 {
		t.Fatalf("returned TrafficRatio = %v, want 2.5", got.TrafficRatio)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, ib.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.TrafficRatio != 2.5 {
		t.Fatalf("persisted TrafficRatio = %v, want 2.5", reloaded.TrafficRatio)
	}
}

func TestUpdateInbound_TrafficRatioClampedToOne(t *testing.T) {
	setupConflictDB(t)

	ib := makeInboundWithTrafficRatio("in-7102-tcp", 7102, 3)
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	svc := &InboundService{}
	for _, below := range []float64{0, -2} {
		update := *ib
		update.TrafficRatio = below

		got, _, err := svc.UpdateInbound(&update)
		if err != nil {
			t.Fatalf("UpdateInbound(%v): %v", below, err)
		}
		if got.TrafficRatio != 1 {
			t.Fatalf("returned TrafficRatio = %v for input %v, want 1", got.TrafficRatio, below)
		}

		var reloaded model.Inbound
		if err := database.GetDB().First(&reloaded, ib.Id).Error; err != nil {
			t.Fatalf("reload: %v", err)
		}
		if reloaded.TrafficRatio != 1 {
			t.Fatalf("persisted TrafficRatio = %v for input %v, want 1", reloaded.TrafficRatio, below)
		}
	}
}

func TestAddInbound_TrafficRatioClampedToOne(t *testing.T) {
	setupConflictDB(t)

	svc := &InboundService{}
	ib := makeInboundWithTrafficRatio("in-7103-tcp", 7103, 0)
	got, _, err := svc.AddInbound(ib)
	if err != nil {
		t.Fatalf("AddInbound: %v", err)
	}
	if got.TrafficRatio != 1 {
		t.Fatalf("returned TrafficRatio = %v, want 1", got.TrafficRatio)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, got.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.TrafficRatio != 1 {
		t.Fatalf("persisted TrafficRatio = %v, want 1", reloaded.TrafficRatio)
	}
}
