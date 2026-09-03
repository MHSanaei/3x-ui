package runtime

import (
	"context"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Each test wires SetNeedRestart to t.Fatal so a regression back to it
// (instead of the fast hook) fails loudly -- see ScheduleAmneziaWGRelayResync.

func TestUpdateAmneziaWGInboundSchedulesFastResync(t *testing.T) {
	calls := 0
	l := NewLocal(LocalDeps{
		APIPort:                      func() int { return 0 },
		SetNeedRestart:               func() { t.Fatal("AmneziaWG update must use ScheduleAmneziaWGRelayResync, not the slow SetNeedRestart") },
		ScheduleAmneziaWGRelayResync: func() { calls++ },
	})

	// Enable=false keeps this on the cheap Manager.Remove path (a no-op on
	// an unmanaged id), not the heavy Ensure path no test exercises yet.
	oldIb := &model.Inbound{Id: 1, Protocol: model.AmneziaWG, Enable: false}
	newIb := &model.Inbound{Id: 1, Protocol: model.AmneziaWG, Enable: false}
	if err := l.UpdateInbound(context.Background(), oldIb, newIb); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected ScheduleAmneziaWGRelayResync to be called exactly once, got %d", calls)
	}
}

func TestDelAmneziaWGInboundSchedulesFastResync(t *testing.T) {
	calls := 0
	l := NewLocal(LocalDeps{
		APIPort:                      func() int { return 0 },
		SetNeedRestart:               func() { t.Fatal("AmneziaWG delete must use ScheduleAmneziaWGRelayResync, not the slow SetNeedRestart") },
		ScheduleAmneziaWGRelayResync: func() { calls++ },
	})

	ib := &model.Inbound{Id: 2, Protocol: model.AmneziaWG, Enable: true}
	if err := l.DelInbound(context.Background(), ib); err != nil {
		t.Fatalf("DelInbound: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected ScheduleAmneziaWGRelayResync to be called exactly once, got %d", calls)
	}
}
