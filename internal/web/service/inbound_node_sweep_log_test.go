package service

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"

	"github.com/op/go-logging"
)

// Removing a central inbound also drops its clients' traffic history, so the
// sweep must name what it removed — a silent drop is undiagnosable in the field.
func TestNodeSnapshotSweepLogsRemovedInbound(t *testing.T) {
	logger.InitLogger(logging.WARNING)
	setupBulkDB(t)
	nodeID, _ := setupNodeRuntime(t)

	gone := nodeInbound(t, nodeID, 31777, nil)
	survivor := nodeInbound(t, nodeID, 31778, nil)

	// The snapshot reports only the survivor, so the other row is swept.
	snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{{
		Tag: survivor.Tag, Port: survivor.Port, Protocol: model.VLESS, Enable: true,
		Settings: survivor.Settings,
	}}}
	if _, err := (&InboundService{}).setRemoteTrafficLocked(nodeID, snap, false); err != nil {
		t.Fatalf("setRemoteTrafficLocked: %v", err)
	}

	var remaining int64
	if err := database.GetDB().Model(model.Inbound{}).Where("id = ?", gone.Id).Count(&remaining).Error; err != nil {
		t.Fatalf("count swept inbound: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("fixture did not sweep inbound %d; the log assertion below would be meaningless", gone.Id)
	}

	want := strconv.Itoa(gone.Id)
	for _, line := range logger.GetLogs(200, "warning") {
		if strings.Contains(line, gone.Tag) && strings.Contains(line, want) {
			return
		}
	}
	t.Fatalf("sweep removed inbound %q (id %d) without naming it in the log", gone.Tag, gone.Id)
}
