package service

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// fakeNodeRuntime is a runtime.Runtime stub that counts the per-client dispatch
// calls so a test can assert a bulk op does NOT stream one RPC per client.
type fakeNodeRuntime struct {
	addClient  atomic.Int32
	deleteUser atomic.Int32
	updateUser atomic.Int32
}

func (f *fakeNodeRuntime) Name() string { return "fake-node" }

func (f *fakeNodeRuntime) AddInbound(context.Context, *model.Inbound) error { return nil }
func (f *fakeNodeRuntime) DelInbound(context.Context, *model.Inbound) error { return nil }
func (f *fakeNodeRuntime) UpdateInbound(context.Context, *model.Inbound, *model.Inbound) error {
	return nil
}
func (f *fakeNodeRuntime) AddUser(context.Context, *model.Inbound, map[string]any) error { return nil }
func (f *fakeNodeRuntime) RemoveUser(context.Context, *model.Inbound, string) error      { return nil }
func (f *fakeNodeRuntime) UpdateUser(context.Context, *model.Inbound, string, model.Client) error {
	f.updateUser.Add(1)
	return nil
}

func (f *fakeNodeRuntime) DeleteUser(context.Context, *model.Inbound, string) error {
	f.deleteUser.Add(1)
	return nil
}

func (f *fakeNodeRuntime) AddClient(context.Context, *model.Inbound, model.Client) error {
	f.addClient.Add(1)
	return nil
}
func (f *fakeNodeRuntime) RestartXray(context.Context) error { return nil }
func (f *fakeNodeRuntime) ResetClientTraffic(context.Context, *model.Inbound, string) error {
	return nil
}
func (f *fakeNodeRuntime) ResetInboundTraffic(context.Context, *model.Inbound) error { return nil }
func (f *fakeNodeRuntime) ResetAllTraffics(context.Context) error                    { return nil }

// setupNodeRuntime wires an online node + a fake runtime override and returns the
// node id and the fake so a test can drive the service node-dispatch path without
// a network node.
func setupNodeRuntime(t *testing.T) (int, *fakeNodeRuntime) {
	t.Helper()
	prev := runtime.GetManager()
	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}})
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(prev) })

	node := &model.Node{Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true, Status: "online"}
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	fake := &fakeNodeRuntime{}
	mgr.SetRuntimeOverride(node.Id, fake)
	return node.Id, fake
}

func nodeInbound(t *testing.T, nodeID, port int, clients []model.Client) *model.Inbound {
	t.Helper()
	if clients == nil {
		clients = []model.Client{}
	}
	ib := &model.Inbound{
		UserId: 1, NodeID: &nodeID, Tag: fmt.Sprintf("in-%d", port), Enable: true,
		Port: port, Protocol: model.VLESS, Settings: clientsSettings(t, clients),
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create node inbound: %v", err)
	}
	if err := (&ClientService{}).SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("seed SyncInbound: %v", err)
	}
	return ib
}

func makeNodeClients(n int) []model.Client {
	out := make([]model.Client, n)
	for i := range n {
		out[i] = model.Client{ID: uuid.NewString(), Email: fmt.Sprintf("nu-%05d@x", i), Enable: true}
	}
	return out
}

// TestNodeBulk_LargeAddFoldsToDirty: adding more than the threshold of clients to
// an online node inbound must NOT stream one AddClient RPC per client; it marks
// the node dirty so a single reconcile push converges it instead.
func TestNodeBulk_LargeAddFoldsToDirty(t *testing.T) {
	setupBulkDB(t)
	nodeID, fake := setupNodeRuntime(t)
	ib := nodeInbound(t, nodeID, 30001, nil)

	svc := &ClientService{}
	inboundSvc := &InboundService{}

	add := makeNodeClients(nodeBulkPushThreshold + 10)
	if _, err := svc.AddInboundClient(inboundSvc, &model.Inbound{Id: ib.Id, Protocol: model.VLESS, Settings: clientsSettings(t, add)}); err != nil {
		t.Fatalf("AddInboundClient: %v", err)
	}

	if got := fake.addClient.Load(); got != 0 {
		t.Fatalf("large add streamed %d AddClient RPCs, want 0 (should fold to dirty)", got)
	}
	if _, _, dirty, _, err := (&NodeService{}).NodeSyncState(nodeID); err != nil {
		t.Fatalf("NodeSyncState: %v", err)
	} else if !dirty {
		t.Fatal("large add must mark the node dirty")
	}
}

// TestNodeBulk_SmallAddPushesLive: a small add stays on the live per-client path.
func TestNodeBulk_SmallAddPushesLive(t *testing.T) {
	setupBulkDB(t)
	nodeID, fake := setupNodeRuntime(t)
	ib := nodeInbound(t, nodeID, 30002, nil)

	svc := &ClientService{}
	inboundSvc := &InboundService{}

	const small = 3
	add := makeNodeClients(small)
	if _, err := svc.AddInboundClient(inboundSvc, &model.Inbound{Id: ib.Id, Protocol: model.VLESS, Settings: clientsSettings(t, add)}); err != nil {
		t.Fatalf("AddInboundClient: %v", err)
	}
	if got := fake.addClient.Load(); got != int32(small) {
		t.Fatalf("small add streamed %d AddClient RPCs, want %d", got, small)
	}
}

// TestNodeBulk_LargeDeleteFoldsToDirty: deleting more than the threshold from an
// online node inbound must fold into a reconcile rather than per-client deletes.
func TestNodeBulk_LargeDeleteFoldsToDirty(t *testing.T) {
	setupBulkDB(t)
	nodeID, fake := setupNodeRuntime(t)

	seed := makeNodeClients(nodeBulkPushThreshold + 10)
	nodeInbound(t, nodeID, 30003, seed)

	svc := &ClientService{}
	inboundSvc := &InboundService{}
	emails := make([]string, len(seed))
	for i := range seed {
		emails[i] = seed[i].Email
	}
	if _, _, err := svc.BulkDelete(inboundSvc, emails, false); err != nil {
		t.Fatalf("BulkDelete: %v", err)
	}

	if got := fake.deleteUser.Load(); got != 0 {
		t.Fatalf("large delete streamed %d DeleteUser RPCs, want 0 (should fold to dirty)", got)
	}
	if _, _, dirty, _, err := (&NodeService{}).NodeSyncState(nodeID); err != nil {
		t.Fatalf("NodeSyncState: %v", err)
	} else if !dirty {
		t.Fatal("large delete must mark the node dirty")
	}
}
