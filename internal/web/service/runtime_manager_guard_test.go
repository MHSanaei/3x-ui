package service

import (
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// loggedEntry returns the first ERROR-level log entry containing substr.
func loggedEntry(substr string) string {
	for _, entry := range logger.GetLogs(500, "ERROR") {
		if strings.Contains(entry, substr) {
			return entry
		}
	}
	return ""
}

// waitForLoggedEntry polls the log buffer for substr, so an asynchronous
// failure is caught instead of racing the assertion.
func waitForLoggedEntry(substr string, within time.Duration) string {
	deadline := time.Now().Add(within)
	for {
		if entry := loggedEntry(substr); entry != "" {
			return entry
		}
		if time.Now().After(deadline) {
			return ""
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A panel without a runtime manager must skip the node restart quietly. The
// lookup used to dereference the nil manager inside the goroutine, where the
// panic was swallowed and logged as "panic in goroutine".
func TestRestartRemoteNodesOnDisableWithoutARuntimeManagerDoesNotPanic(t *testing.T) {
	initTrafficTestDB(t)
	prev := runtime.GetManager()
	runtime.SetManager(nil)
	t.Cleanup(func() { runtime.SetManager(prev) })

	(&InboundService{}).restartRemoteNodesOnDisable([]int{1})

	if entry := waitForLoggedEntry("restart-nodes-on-client-disable", 2*time.Second); entry != "" {
		t.Fatalf("a missing runtime manager must not panic: %s", entry)
	}
}

// The same holds for the reset-all propagation, which runs the node calls in
// fan-out goroutines: a nil manager used to panic in every one of them.
func TestPropagateResetAllTrafficsToNodesWithoutARuntimeManagerDoesNotPanic(t *testing.T) {
	db := initTrafficTestDB(t)
	if err := db.Create(&model.Node{Name: "node-1", Enable: true, Scheme: "https", Address: "node.example", Port: 2053}).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}
	prev := runtime.GetManager()
	runtime.SetManager(nil)
	t.Cleanup(func() { runtime.SetManager(prev) })

	(&InboundService{}).propagateResetAllTrafficsToNodes()

	if entry := loggedEntry("panic applying bulk client change"); entry != "" {
		t.Fatalf("a missing runtime manager must not panic: %s", entry)
	}
}
