package service

import (
	"context"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

type blockingResetRuntime struct {
	fakeNodeRuntime
	reached chan struct{}
	release chan struct{}
}

func (b *blockingResetRuntime) ResetAllTraffics(context.Context) error {
	close(b.reached)
	<-b.release
	return nil
}

func TestResetAllTrafficsDoesNotBlockWriterOnNodeCall(t *testing.T) {
	db := initTrafficTestDB(t)
	resetTrafficWriterForTest(t)
	StartTrafficWriter()

	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })

	node := &model.Node{Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true, Status: "online"}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	fake := &blockingResetRuntime{reached: make(chan struct{}), release: make(chan struct{})}
	mgr.SetRuntimeOverride(node.Id, fake)

	done := make(chan error, 1)
	go func() { done <- (&InboundService{}).ResetAllTraffics() }()

	select {
	case <-fake.reached:
	case <-time.After(3 * time.Second):
		close(fake.release)
		t.Fatal("node ResetAllTraffics was never reached")
	}

	writerFree := make(chan error, 1)
	go func() { writerFree <- submitTrafficWrite(func() error { return nil }) }()
	select {
	case err := <-writerFree:
		if err != nil {
			close(fake.release)
			t.Fatalf("concurrent writer submit failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		close(fake.release)
		<-done
		t.Fatal("the serial traffic writer was blocked by a node reset HTTP call")
	}

	close(fake.release)
	if err := <-done; err != nil {
		t.Fatalf("ResetAllTraffics: %v", err)
	}
}
