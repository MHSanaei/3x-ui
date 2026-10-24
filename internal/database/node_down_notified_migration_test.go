package database

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestNodeDownNotifiedSeederMarksOfflineNodesAnnounced(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	if err := db.Where("seeder_name = ?", "NodeDownNotified").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}
	offline := model.Node{Name: "offline", Address: "10.0.0.1", Port: 1, Status: "offline"}
	online := model.Node{Name: "online", Address: "10.0.0.2", Port: 1, Status: "online"}
	for _, n := range []*model.Node{&offline, &online} {
		if err := db.Create(n).Error; err != nil {
			t.Fatalf("create node %s: %v", n.Name, err)
		}
	}

	if err := runSeeders(false); err != nil {
		t.Fatalf("runSeeders: %v", err)
	}

	for _, tc := range []struct {
		node *model.Node
		want bool
	}{{&offline, true}, {&online, false}} {
		var got model.Node
		if err := db.First(&got, tc.node.Id).Error; err != nil {
			t.Fatalf("reload node %s: %v", tc.node.Name, err)
		}
		if got.DownNotified != tc.want {
			t.Errorf("node %s DownNotified = %v, want %v", tc.node.Name, got.DownNotified, tc.want)
		}
	}
}
