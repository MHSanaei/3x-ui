package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestResetTrafficAcrossNodesPushesConcurrently covers the panel's per-client
// traffic reset, which propagated to nodes one round-trip after another.
func TestResetTrafficAcrossNodesPushesConcurrently(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	const nodes = inboundFanoutConcurrency + 1
	const email = "reset@x"
	bar := newApplyBarrier(inboundFanoutConcurrency)
	seedClientAcrossNodes(t, bar, nodes, 46801, email, "88888888-1111-2222-3333-444444444444")

	bar.arm()
	if _, err := (&ClientService{}).ResetTrafficByEmail(&InboundService{}, email); err != nil {
		t.Fatalf("ResetTrafficByEmail across %d node inbounds: %v", nodes, err)
	}
	if got := bar.maxPar.Load(); got != inboundFanoutConcurrency {
		t.Fatalf("peak node pushes in flight = %d, want overlap at the %d cap (barrier timed out: %v)",
			got, inboundFanoutConcurrency, bar.expired.Load())
	}
}

// TestNodePushGivesUpBeforeTheRemoteTimeout pins that an edit gives up on a node
// that hangs on the push at the deadline, leaving it dirty for the reconcile.
func TestNodePushGivesUpBeforeTheRemoteTimeout(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)
	useTestRuntimeManager(t)
	db := database.GetDB()

	const uuid = "77777777-1111-2222-3333-444444444444"
	f := newFakeNodeHTTP(t)
	ib := realNodeInbound(t, f, 53100, []model.Client{{Email: "hung@x", ID: uuid, SubID: "sub-hung", Enable: true}})
	rec := lookupClientRecord(t, "hung@x")
	f.setHold(true)

	start := time.Now()
	if _, err := (&ClientService{}).Update(&InboundService{}, rec.Id, model.Client{
		Email: "hung@x", ID: uuid, SubID: "sub-hung", Enable: true, Comment: "edited",
	}, 0); err != nil {
		t.Fatalf("Update against a hung node: %v", err)
	}
	elapsed := time.Since(start)
	// Both bounds: no push at all would also finish fast and leave the node dirty.
	if got := f.hitCount("/clients/update/"); got != 1 {
		t.Fatalf("clients/update requests reaching the hung node = %d, want exactly 1", got)
	}
	if elapsed < nodeClientPushTimeout-200*time.Millisecond {
		t.Fatalf("edit returned after %v, before the %v push deadline: the push was never awaited", elapsed, nodeClientPushTimeout)
	}
	if elapsed >= 2*nodeClientPushTimeout {
		t.Fatalf("edit took %v against a hung node: it waited out the remote timeout instead of the %v push deadline", elapsed, nodeClientPushTimeout)
	}

	var node model.Node
	if err := db.Where("id = ?", *ib.NodeID).First(&node).Error; err != nil {
		t.Fatalf("read node: %v", err)
	}
	if !node.ConfigDirty {
		t.Fatal("the node whose push timed out must stay dirty, or nothing ever converges it")
	}
}

// TestBulkDeleteStopsPushingToAHungNode pins the batch circuit-break: once one
// push times out, the rest of the batch defers to the reconcile as well.
func TestBulkDeleteStopsPushingToAHungNode(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)
	useTestRuntimeManager(t)

	f := newFakeNodeHTTP(t)
	realNodeInbound(t, f, 53200, []model.Client{
		{Email: "b1@x", ID: "66666666-1111-2222-3333-444444444441", SubID: "sub-b1", Enable: true},
		{Email: "b2@x", ID: "66666666-1111-2222-3333-444444444442", SubID: "sub-b2", Enable: true},
		{Email: "b3@x", ID: "66666666-1111-2222-3333-444444444443", SubID: "sub-b3", Enable: true},
	})
	f.setHold(true)

	start := time.Now()
	if _, _, err := (&ClientService{}).BulkDelete(&InboundService{}, []string{"b1@x", "b2@x", "b3@x"}, false); err != nil {
		t.Fatalf("BulkDelete against a hung node: %v", err)
	}
	elapsed := time.Since(start)
	if got := f.hitCount("/clients/del/"); got != 1 {
		t.Fatalf("clients/del requests sent to the hung node = %d, want 1: the batch kept paying a deadline per client", got)
	}
	if elapsed >= 2*nodeClientPushTimeout {
		t.Fatalf("deleting 3 clients took %v against a hung node, want a single %v deadline, not one per client", elapsed, nodeClientPushTimeout)
	}
}
