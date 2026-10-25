package service

import (
	"os"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
)

func initGlobalHeartbeatDB(t *testing.T) *gorm.DB {
	t.Helper()
	cleanup, err := testpg.IsolatePackage("global_heartbeat")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	return initTrafficTestDB(t)
}

func TestGlobalTrafficHeartbeatPostgres(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || os.Getenv("XUI_DB_DSN") == "" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN")
	}
	for _, tc := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"bounds", TestGlobalTrafficHeartbeatBounds},
		{"quota and lifecycle", TestGlobalTrafficHeartbeatQuotaAndLifecycle},
		{"forwarding", TestGlobalTrafficHeartbeatForwarding},
		{"nullable and batches", TestGlobalTrafficHeartbeatNullableAndBatch},
		{"long idle and recovery", TestGlobalTrafficHeartbeatLongIdleAndRecovery},
		{"concurrent pairs", TestGlobalTrafficHeartbeatConcurrentPairs},
	} {
		t.Run(tc.name, tc.run)
	}
}
