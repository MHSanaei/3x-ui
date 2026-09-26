package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type hangingUpdateRuntime struct {
	fakeNodeRuntime
	entered chan struct{}
	release chan struct{}
}

func (h *hangingUpdateRuntime) UpdateInbound(ctx context.Context, _, _ *model.Inbound) error {
	h.updateInbound.Add(1)
	select {
	case h.entered <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-h.release:
		return nil
	}
}

func seedDepletedNodeClient(t *testing.T, nodeID, port int) {
	t.Helper()
	client := model.Client{Email: fmt.Sprintf("spent-%d", port), Enable: true}
	ib := nodeInbound(t, nodeID, port, []model.Client{client})
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: client.Email, Enable: true, Up: 100, Total: 100,
	}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}
}

// A depletion wave used to push every node inbound on the serial writer, one by
// one with no deadline, so a hanging node froze traffic accounting and client edits.
func TestTrafficDisableNodePushLeavesWriterFreeAndGivesUp(t *testing.T) {
	setupConflictDB(t)
	StartTrafficWriter()
	t.Cleanup(StopTrafficWriter)
	nodeID, _ := setupNodeRuntime(t)
	hanging := &hangingUpdateRuntime{entered: make(chan struct{}, 1), release: make(chan struct{})}
	runtime.GetManager().SetRuntimeOverride(nodeID, hanging)
	t.Cleanup(func() { close(hanging.release) })
	seedDepletedNodeClient(t, nodeID, 46311)
	seedDepletedNodeClient(t, nodeID, 46313)

	returned := make(chan error, 1)
	go func() {
		_, _, err := (&InboundService{}).AddTraffic(nil, nil)
		returned <- err
	}()
	select {
	case <-hanging.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("depleted node client was never pushed to its node")
	}

	writerFree := make(chan error, 1)
	go func() { writerFree <- submitTrafficWrite(func() error { return nil }) }()
	select {
	case err := <-writerFree:
		if err != nil {
			t.Fatalf("traffic write while node push hangs: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("traffic writer stayed held while a node push hung")
	}

	// Two hanging pushes: one at a time they would take twice the push timeout.
	select {
	case err := <-returned:
		if err != nil {
			t.Fatalf("AddTraffic: %v", err)
		}
	case <-time.After(nodeClientPushTimeout + 2*time.Second):
		t.Fatal("AddTraffic kept waiting on hanging node pushes past one push timeout")
	}
}

func TestTrafficDisableSkipsOfflineNodePushButMarksDirty(t *testing.T) {
	setupConflictDB(t)
	nodeID, fake := setupNodeRuntime(t)
	if err := database.GetDB().Model(&model.Node{}).Where("id = ?", nodeID).Update("status", "offline").Error; err != nil {
		t.Fatalf("mark node offline: %v", err)
	}
	seedDepletedNodeClient(t, nodeID, 46312)

	if _, _, err := (&InboundService{}).AddTraffic(nil, nil); err != nil {
		t.Fatalf("AddTraffic: %v", err)
	}
	if got := fake.updateInbound.Load(); got != 0 {
		t.Fatalf("UpdateInbound calls to an offline node = %d, want 0", got)
	}
	if _, _, dirty, _, err := (&NodeService{}).NodeSyncState(nodeID); err != nil || !dirty {
		t.Fatalf("node dirty = %v (err %v), want true so reconcile applies the disable", dirty, err)
	}
}

type hangingRestartRuntime struct {
	fakeNodeRuntime
	entered chan struct{}
	release chan struct{}
}

func (h *hangingRestartRuntime) RestartXray(ctx context.Context) error {
	select {
	case h.entered <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-h.release:
		return nil
	}
}

// The opt-in restart is best-effort and never replayed, so a hanging node must
// not hold the traffic poll that disabled its client.
func TestTrafficDisableNodeRestartDoesNotBlockTrafficPoll(t *testing.T) {
	setupConflictDB(t)
	setRestartOnClientDisable(t, true)
	nodeID, _ := setupNodeRuntime(t)
	hanging := &hangingRestartRuntime{entered: make(chan struct{}, 1), release: make(chan struct{})}
	runtime.GetManager().SetRuntimeOverride(nodeID, hanging)
	t.Cleanup(func() { close(hanging.release) })
	seedDepletedNodeClient(t, nodeID, 46314)

	returned := make(chan error, 1)
	go func() {
		_, _, err := (&InboundService{}).AddTraffic(nil, nil)
		returned <- err
	}()
	select {
	case <-hanging.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("node Xray was never restarted after its client was disabled")
	}
	select {
	case err := <-returned:
		if err != nil {
			t.Fatalf("AddTraffic: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("AddTraffic waited on a hanging node restart")
	}
}
