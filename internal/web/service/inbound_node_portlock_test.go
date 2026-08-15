package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// The snapshot adopts, replaces and sweeps reservations, so it must hold the
// same per-port lock AddInbound takes; otherwise both commit conflicting claims.
func TestNodeSnapshotHoldsPortReservationLock(t *testing.T) {
	t.Setenv("XUI_DB_TYPE", "sqlite")
	t.Setenv(portReservationsGateEnv, "1")
	setupBulkDB(t)
	nodeID, _ := setupNodeRuntime(t)

	const port = 34567
	snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{{
		Tag: "in-locked", Port: port, Protocol: model.VLESS, Enable: true,
		Settings: `{"clients":[],"decryption":"none","fallbacks":[]}`,
	}}}

	holder := lockPortReservationKeys(portLockKey{nodeScope: nodeID, port: port})
	done := make(chan struct{})
	go func() {
		_, _ = (&InboundService{}).setRemoteTrafficLocked(nodeID, snap, false)
		close(done)
	}()

	select {
	case <-done:
		holder()
		t.Fatal("snapshot processed the node while another writer held the port lock")
	case <-time.After(150 * time.Millisecond):
	}

	holder()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("snapshot never finished after the port lock was released")
	}
}
