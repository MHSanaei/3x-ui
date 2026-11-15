package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestDesiredVKTurnInstancesSkipRemoteAndDisabledInbounds(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	node := 9
	settings := `{"vkTurn":{"enabled":true,"callLink":"https://vk.ru/call/join/token","port":56000}}`
	rows := []model.Inbound{
		{Id: 1, UserId: 1, Tag: "wg-active", Protocol: model.WireGuard, Enable: true, Port: 51820, Settings: settings},
		{Id: 2, UserId: 1, Tag: "wg-disabled", Protocol: model.WireGuard, Enable: false, Port: 51821, Settings: settings},
		{Id: 3, UserId: 1, Tag: "wg-remote", Protocol: model.WireGuard, Enable: true, Port: 51822, Settings: settings, NodeID: &node},
		{Id: 4, UserId: 1, Tag: "wg-plain", Protocol: model.WireGuard, Enable: true, Port: 51823, Settings: `{}`},
	}
	for _, row := range rows {
		if err := database.GetDB().Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := &InboundService{}
	desired, err := svc.DesiredVKTurnInstances()
	if err != nil {
		t.Fatal(err)
	}
	if len(desired) != 1 || desired[0].ID != 1 || desired[0].Connect != "127.0.0.1:51820" {
		t.Fatalf("desired = %+v", desired)
	}
	options, err := svc.GetInboundOptions(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 4 || options[0].WgVkTurn == nil || options[0].WgVkTurn.CallLink != "https://vk.ru/call/join/token" || options[3].WgVkTurn != nil {
		t.Fatalf("WireGuard VK hints = %+v", options)
	}
}
