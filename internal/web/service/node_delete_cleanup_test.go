package service

import (
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Deleting a node dropped only its cpu and mem series, while the heartbeat also
// records netUp and netDown, so each deleted node leaked two histories for good.
func TestDeleteNodeDropsEveryMetricSeries(t *testing.T) {
	setupConflictDB(t)
	node := &model.Node{Id: 9101, Name: "gone", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true, Status: "online"}
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	ns := NodeService{}
	if err := ns.UpdateHeartbeat(node.Id, HeartbeatPatch{
		Status: "online", LastHeartbeat: time.Now().Unix(), CpuPct: 1, MemPct: 2, NetUp: 3, NetDown: 4,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat: %v", err)
	}

	if err := ns.Delete(node.Id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	prefix := nodeMetricKey(node.Id, "")
	var left []string
	nodeMetrics.mu.Lock()
	for key := range nodeMetrics.series {
		if strings.HasPrefix(key, prefix) {
			left = append(left, key)
		}
	}
	nodeMetrics.mu.Unlock()
	if len(left) != 0 {
		t.Fatalf("metric series left after deleting the node: %v", left)
	}
}
