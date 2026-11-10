package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// A failed node read used to leave the zero-value row behind, so dirty read as
// false and the merge rebuilt client traffic rows from a snapshot the node
// never confirmed. The read failure has to abort the merge.
func TestSetRemoteTraffic_NodeReadFailureAbortsMerge(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	node := &model.Node{Name: "read-error", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true, Status: "online"}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := db.Migrator().DropTable(&model.Node{}); err != nil {
		t.Fatalf("drop nodes table: %v", err)
	}

	snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{{
		Tag:      "read-error-443",
		Enable:   true,
		Port:     443,
		Protocol: model.VLESS,
		Settings: "{\"clients\":[{\"email\":\"a@x\"}]}",
	}}}
	if _, err := (&InboundService{}).setRemoteTrafficLocked(node.Id, snap, false, false); err == nil {
		t.Fatal("setRemoteTrafficLocked ignored a failed node read")
	}
}
