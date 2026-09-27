package service

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func useOnlineTestProcess(t *testing.T) *xray.Process {
	t.Helper()
	previousProcess, previousResult := xrayState.snapshot()
	process := xray.NewTestProcess(nil, "")
	xrayState.replace(process)
	t.Cleanup(func() {
		xrayState.mu.Lock()
		xrayState.process = previousProcess
		xrayState.result = previousResult
		xrayState.mu.Unlock()
	})
	return process
}

// Only a failed snapshot fetch used to clear a node's online set, so a node the
// sync stopped reaching (disabled, marked offline, deleted) kept its clients online.
func TestRetainSyncedNodeOnlineClientsDropsUnsyncedNodes(t *testing.T) {
	setupConflictDB(t)
	process := useOnlineTestProcess(t)
	svc := InboundService{}
	for id := 1; id <= 4; id++ {
		guid := fmt.Sprintf("g%d", id)
		svc.SetNodeOnlineTree(id, map[string][]string{guid: {guid + "@x"}})
		process.SetNodeActiveInboundTree(id, map[string][]string{guid: {"in-" + guid}})
	}

	svc.RetainSyncedNodeOnlineClients([]*model.Node{
		{Id: 1, Enable: true, Status: "online"},
		{Id: 2, Enable: false, Status: "online"},
		{Id: 3, Enable: true, Status: "offline"},
	})

	if got, want := svc.GetOnlineClientsByGuid(), map[string][]string{"g1": {"g1@x"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("online by guid = %v, want %v", got, want)
	}
	if got, want := svc.GetActiveInboundsByGuid(), map[string][]string{"g1": {"in-g1"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("active inbounds by guid = %v, want %v", got, want)
	}
}

func TestSetRemoteTrafficFailureClearsNodeOnlineClients(t *testing.T) {
	setupConflictDB(t)
	useOnlineTestProcess(t)
	svc := InboundService{}
	svc.SetNodeOnlineTree(7, map[string][]string{"g7": {"a@x"}})
	if err := database.GetDB().Exec("DROP TABLE inbounds").Error; err != nil {
		t.Fatalf("drop inbounds: %v", err)
	}

	if _, err := svc.SetRemoteTraffic(7, &runtime.TrafficSnapshot{}, false, false); err == nil {
		t.Fatal("SetRemoteTraffic succeeded without an inbounds table")
	}
	if got := svc.GetOnlineClientsByGuid(); len(got) != 0 {
		t.Errorf("online by guid after a failed merge = %v, want none", got)
	}
}
