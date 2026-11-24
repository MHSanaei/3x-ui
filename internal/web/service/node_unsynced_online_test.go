package service

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

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

// A node may report only the clients linked to its own inbounds, under its own GUID: an
// unscoped report marked any master client online and grew without bound.
func TestSetRemoteTrafficKeepsOnlyTheNodesOwnClientsOnline(t *testing.T) {
	db := initTrafficTestDB(t)
	useOnlineTestProcess(t)
	svc := &InboundService{}
	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true, Guid: "node-guid"})
	createNodeInboundWithClient(t, db, 1, "keep", 41401, "kept@x")
	settings := `{"clients":[{"email":"kept@x","enable":true}]}`
	snap := snapshotWithClients(t, "keep", settings, xray.ClientTraffic{Email: "kept@x", Enable: true})
	if _, err := svc.setRemoteTrafficLocked(1, snap, false, false); err != nil {
		t.Fatalf("seed sync: %v", err)
	}
	snap = snapshotWithClients(t, "keep", settings, xray.ClientTraffic{Email: "kept@x", Enable: true})
	snap.OnlineTree = map[string][]string{"node-guid": {"kept@x", "master-only@x", "kept@x"}, "someone-else": {"kept@x"}}
	if _, err := svc.setRemoteTrafficLocked(1, snap, false, false); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if got, want := svc.GetOnlineClientsByGuid(), map[string][]string{"node-guid": {"kept@x"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("online by guid = %v, want %v", got, want)
	}
}

// The client-IP reports feed the IP limit and fail2ban; a node must not plant rows for
// clients it does not serve, under other panels' GUIDs, or dated into the future.
func TestNodeClientIpReportsAreScopedToTheNode(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	node := &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true, Guid: "node-guid"}
	seedNodeRow(t, db, node)
	createNodeInboundWithClient(t, db, 1, "keep", 41402, "kept@x")
	settings := `{"clients":[{"email":"kept@x","enable":true}]}`
	if _, err := svc.setRemoteTrafficLocked(1, snapshotWithClients(t, "keep", settings, xray.ClientTraffic{Email: "kept@x", Enable: true}), false, false); err != nil {
		t.Fatalf("seed sync: %v", err)
	}
	future := time.Now().Add(240 * time.Hour).Unix()
	entries := func(ip string) string { return fmt.Sprintf(`[{"ip":%q,"timestamp":%d}]`, ip, future) }
	if err := svc.MergeNodeClientIps(1, []model.InboundClientIps{
		{ClientEmail: "kept@x", Ips: entries("198.51.100.1")},
		{ClientEmail: "master-only@x", Ips: entries("198.51.100.2")},
	}); err != nil {
		t.Fatalf("MergeNodeClientIps: %v", err)
	}
	var flat []model.InboundClientIps
	db.Find(&flat)
	if len(flat) != 1 || flat[0].ClientEmail != "kept@x" || strings.Contains(flat[0].Ips, strconv.FormatInt(future, 10)) {
		t.Fatalf("stored client IPs = %+v, want only kept@x, dated no later than now", flat)
	}
	tree := map[string]map[string][]model.ClientIpEntry{
		"node-guid":    {"kept@x": {{IP: "198.51.100.3", Timestamp: future}}, "master-only@x": {{IP: "198.51.100.4", Timestamp: future}}},
		"someone-else": {"kept@x": {{IP: "198.51.100.5", Timestamp: future}}},
	}
	if err := svc.MergeClientIpsByGuid(node, tree); err != nil {
		t.Fatalf("MergeClientIpsByGuid: %v", err)
	}
	var rows []model.NodeClientIp
	db.Find(&rows)
	if len(rows) != 1 || rows[0].NodeGuid != "node-guid" || rows[0].Email != "kept@x" || strings.Contains(rows[0].Ips, strconv.FormatInt(future, 10)) {
		t.Fatalf("stored per-node IPs = %+v, want only kept@x under node-guid, dated no later than now", rows)
	}
}
