package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// applyBarrierRuntime holds every armed node push until fanout of them are
// inside it at once; a sequential caller only ever reaches one and times out.
// It stays pass-through until arm() so a test can seed its clients first.
type applyBarrierRuntime struct {
	fakeNodeRuntime
	fanout   int32
	armed    atomic.Bool
	inFlight atomic.Int32
	maxPar   atomic.Int32
	release  chan struct{}
	freed    atomic.Bool
	expired  atomic.Bool
}

func newApplyBarrier(fanout int32) *applyBarrierRuntime {
	return &applyBarrierRuntime{fanout: fanout, release: make(chan struct{})}
}

func (b *applyBarrierRuntime) arm() { b.armed.Store(true) }

func (b *applyBarrierRuntime) free() {
	if b.freed.CompareAndSwap(false, true) {
		close(b.release)
	}
}

func (b *applyBarrierRuntime) wait() {
	if !b.armed.Load() {
		return
	}
	n := b.inFlight.Add(1)
	for {
		peak := b.maxPar.Load()
		if n <= peak || b.maxPar.CompareAndSwap(peak, n) {
			break
		}
	}
	if n == b.fanout {
		b.free()
	}
	select {
	case <-b.release:
	case <-time.After(5 * time.Second):
		// Release everyone on the first timeout so a sequential regression
		// fails once instead of stalling for fanout x the wait.
		b.expired.Store(true)
		b.free()
	}
	b.inFlight.Add(-1)
}

func (b *applyBarrierRuntime) UpdateUser(ctx context.Context, ib *model.Inbound, oldEmail string, c model.Client) error {
	b.wait()
	return b.fakeNodeRuntime.UpdateUser(ctx, ib, oldEmail, c)
}

func (b *applyBarrierRuntime) DeleteClient(ctx context.Context, email string) error {
	b.wait()
	return b.fakeNodeRuntime.DeleteClient(ctx, email)
}

func (b *applyBarrierRuntime) DeleteUser(ctx context.Context, ib *model.Inbound, email string) error {
	b.wait()
	return b.fakeNodeRuntime.DeleteUser(ctx, ib, email)
}

// seedClientAcrossNodes creates one client on nodes separate node inbounds and
// returns its record id, with the barrier still disarmed.
func seedClientAcrossNodes(t *testing.T, bar *applyBarrierRuntime, nodes int, basePort int, email, uuid string) int {
	t.Helper()
	mgr := useTestRuntimeManager(t)
	ids := fanoutNodeInbounds(t, mgr, bar, nodes, basePort)
	if _, err := (&ClientService{}).Create(&InboundService{}, &ClientCreatePayload{
		Client:     model.Client{Email: email, ID: uuid, SubID: "sub-" + email, Enable: true},
		InboundIds: ids,
	}); err != nil {
		t.Fatalf("seed Create across %d node inbounds: %v", nodes, err)
	}
	return lookupClientRecord(t, email).Id
}

// TestUpdateAcrossNodesPushesConcurrently pins that editing a client attached to
// several node inbounds pushes to them at once. Sequentially the per-node
// round-trips add up, so an edit on a multi-node master cost one RPC per node.
func TestUpdateAcrossNodesPushesConcurrently(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	const nodes = inboundFanoutConcurrency + 1
	const uuid = "aaaaaaaa-1111-2222-3333-444444444444"
	bar := newApplyBarrier(inboundFanoutConcurrency)
	recID := seedClientAcrossNodes(t, bar, nodes, 45101, "upfan@x", uuid)

	bar.arm()
	if _, err := (&ClientService{}).Update(&InboundService{}, recID, model.Client{
		Email: "upfan@x", ID: uuid, SubID: "sub-upfan@x", Enable: true, Comment: "edited",
	}, 0); err != nil {
		t.Fatalf("Update across %d node inbounds: %v", nodes, err)
	}

	if got := bar.updateUser.Load(); got != nodes {
		t.Fatalf("UpdateUser pushes = %d, want %d", got, nodes)
	}
	if got := bar.maxPar.Load(); got != inboundFanoutConcurrency {
		t.Fatalf("peak node pushes in flight = %d, want overlap at the %d cap (barrier timed out: %v)",
			got, inboundFanoutConcurrency, bar.expired.Load())
	}
}

// TestDeleteAcrossNodesPushesConcurrently is the delete-side twin of the update
// test above: removing a client must not cost one node round-trip per node.
func TestDeleteAcrossNodesPushesConcurrently(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	const nodes = inboundFanoutConcurrency + 1
	const uuid = "bbbbbbbb-1111-2222-3333-444444444444"
	bar := newApplyBarrier(inboundFanoutConcurrency)
	recID := seedClientAcrossNodes(t, bar, nodes, 45201, "delfan@x", uuid)

	bar.arm()
	if _, err := (&ClientService{}).Delete(&InboundService{}, recID, false); err != nil {
		t.Fatalf("Delete across %d node inbounds: %v", nodes, err)
	}

	if got := bar.deleteClient.Load(); got != nodes {
		t.Fatalf("DeleteClient pushes = %d, want %d", got, nodes)
	}
	if got := bar.maxPar.Load(); got != inboundFanoutConcurrency {
		t.Fatalf("peak node pushes in flight = %d, want overlap at the %d cap (barrier timed out: %v)",
			got, inboundFanoutConcurrency, bar.expired.Load())
	}
}

// TestDetachAcrossNodesPushesConcurrently covers the third sequential loop: a
// bulk detach walks the same per-inbound node push as update and delete.
func TestDetachAcrossNodesPushesConcurrently(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	const nodes = inboundFanoutConcurrency + 1
	const uuid = "cccccccc-1111-2222-3333-444444444444"
	bar := newApplyBarrier(inboundFanoutConcurrency)
	recID := seedClientAcrossNodes(t, bar, nodes, 45301, "detfan@x", uuid)
	ids, err := (&ClientService{}).GetInboundIdsForRecord(recID)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}

	bar.arm()
	if _, err := (&ClientService{}).Detach(&InboundService{}, recID, ids); err != nil {
		t.Fatalf("Detach across %d node inbounds: %v", nodes, err)
	}

	if got := bar.deleteUser.Load(); got != nodes {
		t.Fatalf("DeleteUser pushes = %d, want %d", got, nodes)
	}
	if got := bar.maxPar.Load(); got != inboundFanoutConcurrency {
		t.Fatalf("peak node pushes in flight = %d, want overlap at the %d cap (barrier timed out: %v)",
			got, inboundFanoutConcurrency, bar.expired.Load())
	}
}
