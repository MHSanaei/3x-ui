package runtime

import (
	"context"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestUpdateTuicInboundResyncsBridgeOnTagRename(t *testing.T) {
	resyncs := 0
	local := NewLocal(LocalDeps{RequestReconcile: func() { resyncs++ }})
	oldInbound := &model.Inbound{Id: 940001, Tag: "old-tuic-tag", Protocol: model.TUIC, Enable: true, Settings: `{"clients":[]}`}
	newInbound := *oldInbound
	newInbound.Tag = "new-tuic-tag"

	if err := local.updateTuicInbound(context.Background(), oldInbound, &newInbound); err != nil {
		t.Fatalf("updateTuicInbound: %v", err)
	}
	if resyncs != 1 {
		t.Fatalf("Xray bridge resync count = %d, want 1", resyncs)
	}
}

// applyLocalTuic reads the row after commit, so old/new payloads may already match.
func TestUpdateTuicInboundResyncsAccountsWithIdenticalPayloads(t *testing.T) {
	resyncs := 0
	local := NewLocal(LocalDeps{RequestReconcile: func() { resyncs++ }})
	inbound := &model.Inbound{Id: 940002, Tag: "tuic-tag", Protocol: model.TUIC, Enable: true, Settings: `{"clients":[]}`}
	if err := local.UpdateInbound(context.Background(), inbound, inbound); err != nil {
		t.Fatal(err)
	}
	if resyncs != 1 {
		t.Fatalf("relay account resync requests = %d, want 1", resyncs)
	}
}

func TestTuicBulkUserOperationsResyncRelayAccounts(t *testing.T) {
	for _, operation := range []string{"add", "remove"} {
		t.Run(operation, func(t *testing.T) {
			resyncs := 0
			local := NewLocal(LocalDeps{RequestReconcile: func() { resyncs++ }})
			inbound := &model.Inbound{Id: 940003, Protocol: model.TUIC, Enable: true}
			var err error
			if operation == "add" {
				err = local.AddUser(context.Background(), inbound, map[string]any{"email": "bulk@tuic"})
			} else {
				err = local.RemoveUser(context.Background(), inbound, "bulk@tuic")
			}
			if err != nil {
				t.Fatal(err)
			}
			if resyncs != 1 {
				t.Fatalf("%s requested %d relay reconciliations, want 1", operation, resyncs)
			}
		})
	}
}
