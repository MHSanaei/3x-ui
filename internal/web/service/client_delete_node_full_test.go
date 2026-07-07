package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// A full client delete must reach the node as the full-delete RPC so the node
// drops its own client record too — the detach RPC leaves an orphaned record
// that keeps showing in the node's client list (#5797).
func TestDelete_NodeClientDispatchesFullDeleteRPC(t *testing.T) {
	setupBulkDB(t)
	nodeID, fake := setupNodeRuntime(t)

	clients := []model.Client{{ID: uuid.NewString(), Email: "full-del@x", Enable: true}}
	nodeInbound(t, nodeID, 32001, clients)

	svc := &ClientService{}
	inboundSvc := &InboundService{}

	rec, err := svc.GetRecordByEmail(nil, "full-del@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if _, err := svc.Delete(inboundSvc, rec.Id, false); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if got := fake.deleteClient.Load(); got != 1 {
		t.Fatalf("full delete dispatched %d DeleteClient RPCs, want 1", got)
	}
	if got := fake.deleteUser.Load(); got != 0 {
		t.Fatalf("full delete dispatched %d DeleteUser (detach) RPCs, want 0", got)
	}
}

// A plain detach must stay scoped to the one inbound via the detach RPC and
// never escalate to the node-wide full delete.
func TestDetach_NodeClientStaysOnDetachRPC(t *testing.T) {
	setupBulkDB(t)
	nodeID, fake := setupNodeRuntime(t)

	clients := []model.Client{{ID: uuid.NewString(), Email: "detach-me@x", Enable: true}}
	ib := nodeInbound(t, nodeID, 32002, clients)

	svc := &ClientService{}
	inboundSvc := &InboundService{}

	rec, err := svc.GetRecordByEmail(nil, "detach-me@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if _, err := svc.Detach(inboundSvc, rec.Id, []int{ib.Id}); err != nil {
		t.Fatalf("Detach: %v", err)
	}

	if got := fake.deleteUser.Load(); got != 1 {
		t.Fatalf("detach dispatched %d DeleteUser RPCs, want 1", got)
	}
	if got := fake.deleteClient.Load(); got != 0 {
		t.Fatalf("detach dispatched %d DeleteClient RPCs, want 0", got)
	}
}
