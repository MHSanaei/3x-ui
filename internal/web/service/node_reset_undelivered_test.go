package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

const (
	resetLostOn  = `{"clients":[{"email":"reset-lost","totalGB":100,"enable":true}]}`
	resetLostOff = `{"clients":[{"email":"reset-lost","totalGB":100,"enable":false}]}`
)

// seedLatchedNodeClient leaves reset-lost depleted and latched off on the
// master by its node's own usage, as a real node sync does.
func seedLatchedNodeClient(t *testing.T, svc *InboundService) (*gorm.DB, *model.Inbound) {
	t.Helper()
	db := initTrafficTestDB(t)
	createNodeInboundWithClient(t, db, 1, "n1-in", 41901, "reset-lost")
	syncNodeWithSettings(t, svc, 1, "n1-in", resetLostOn, xray.ClientTraffic{Email: "reset-lost", Up: 10, Down: 10, Total: 100, Enable: true})
	syncNodeWithSettings(t, svc, 1, "n1-in", resetLostOff, xray.ClientTraffic{Email: "reset-lost", Up: 60, Down: 60, Total: 100, Enable: false})
	if got := readTraffic(t, db, "reset-lost"); got.Enable {
		t.Fatal("setup: the depleted client should be latched off")
	}
	var ib model.Inbound
	if err := db.Where("tag = ?", "n1-in").First(&ib).Error; err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	return db, &ib
}

// A reset the node never received leaves its old counters, so the node keeps
// switching the client off; the master must not adopt that verdict.
func TestNodeResetNotDeliveredDoesNotRedisableClient(t *testing.T) {
	resets := []struct {
		name string
		run  func(svc *InboundService, ib *model.Inbound) error
	}{
		{"single", func(svc *InboundService, ib *model.Inbound) error {
			_, err := svc.ResetClientTraffic(ib.Id, "reset-lost")
			return err
		}},
		{"bulk", func(svc *InboundService, _ *model.Inbound) error {
			_, err := (&ClientService{}).BulkResetTraffic(svc, []string{"reset-lost"})
			return err
		}},
		{"inbound", func(svc *InboundService, ib *model.Inbound) error {
			return (&ClientService{}).ResetAllClientTraffics(svc, ib.Id)
		}},
		{"all", func(*InboundService, *model.Inbound) error {
			_, err := (&ClientService{}).ResetAllTraffics()
			return err
		}},
	}
	for _, reset := range resets {
		t.Run(reset.name, func(t *testing.T) {
			svc := &InboundService{}
			db, ib := seedLatchedNodeClient(t, svc)
			if err := reset.run(svc, ib); err != nil {
				t.Fatalf("reset: %v", err)
			}
			syncNodeWithSettings(t, svc, 1, "n1-in", resetLostOff, xray.ClientTraffic{Email: "reset-lost", Up: 60, Down: 60, Total: 100, Enable: false})
			got := readTraffic(t, db, "reset-lost")
			if !got.Enable || got.Up+got.Down != 0 {
				t.Fatalf("after reset: enable=%v used=%d, want enabled at 0 — the undelivered reset re-disabled it", got.Enable, got.Up+got.Down)
			}
		})
	}
}

// resetRecordingRuntime is a node that accepts per-client resets unless failing.
type resetRecordingRuntime struct {
	fakeNodeRuntime
	mu   sync.Mutex
	fail bool
	got  []string
}

func (r *resetRecordingRuntime) ResetClientTraffic(_ context.Context, _ *model.Inbound, email string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return errors.New("node unreachable")
	}
	r.got = append(r.got, email)
	return nil
}

func (r *resetRecordingRuntime) delivered() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.got)
}

func pendingResetEmails(t *testing.T, nodeID int) []string {
	t.Helper()
	var emails []string
	if err := database.GetDB().Model(&model.NodePendingReset{}).Where("node_id = ?", nodeID).
		Order("email").Pluck("email", &emails).Error; err != nil {
		t.Fatalf("read pending resets: %v", err)
	}
	return emails
}

func setupRecordingNode(t *testing.T, fail bool) (int, *resetRecordingRuntime, *model.Inbound) {
	t.Helper()
	setupBulkDB(t)
	mgr := useTestRuntimeManager(t)
	node := &model.Node{Name: "reset-node", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true, Status: "online"}
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	rec := &resetRecordingRuntime{fail: fail}
	mgr.SetRuntimeOverride(node.Id, rec)
	ib := nodeInbound(t, node.Id, 41911, []model.Client{{Email: "reset-lost", ID: "11111111-1111-1111-1111-1111111111aa", Enable: true}})
	if err := (&InboundService{}).AddClientStat(database.GetDB(), ib.Id, &model.Client{Email: "reset-lost", Enable: true}); err != nil {
		t.Fatalf("AddClientStat: %v", err)
	}
	return node.Id, rec, ib
}

// A reachable node gets the reset right after the master commits it.
func TestNodeResetDeliveredRightAway(t *testing.T) {
	resets := []struct {
		name string
		run  func(svc *InboundService, ib *model.Inbound) error
	}{
		{"single", func(svc *InboundService, ib *model.Inbound) error {
			_, err := svc.ResetClientTraffic(ib.Id, "reset-lost")
			return err
		}},
		{"bulk", func(svc *InboundService, _ *model.Inbound) error {
			_, err := (&ClientService{}).BulkResetTraffic(svc, []string{"reset-lost"})
			return err
		}},
		{"inbound", func(svc *InboundService, ib *model.Inbound) error {
			return (&ClientService{}).ResetAllClientTraffics(svc, ib.Id)
		}},
		{"all", func(*InboundService, *model.Inbound) error {
			_, err := (&ClientService{}).ResetAllTraffics()
			return err
		}},
	}
	for _, reset := range resets {
		t.Run(reset.name, func(t *testing.T) {
			nodeID, rec, ib := setupRecordingNode(t, false)
			if err := reset.run(&InboundService{}, ib); err != nil {
				t.Fatalf("reset: %v", err)
			}
			if got := rec.delivered(); !slices.Equal(got, []string{"reset-lost"}) {
				t.Fatalf("node received resets %v, want [reset-lost]", got)
			}
			if left := pendingResetEmails(t, nodeID); len(left) != 0 {
				t.Fatalf("delivered reset still queued: %v", left)
			}
		})
	}
}

// bulkResetRuntime also takes a batch in one call.
type bulkResetRuntime struct {
	resetRecordingRuntime
	batches [][]string
}

func (b *bulkResetRuntime) ResetClientTraffics(_ context.Context, emails []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.batches = append(b.batches, slices.Clone(emails))
	return nil
}

// Above the per-client push threshold a backlog goes out as one bulk request,
// not one round-trip per client.
func TestNodeResetBacklogUsesBulkRequest(t *testing.T) {
	setupBulkDB(t)
	const nodeID = 7
	rows := make([]model.NodePendingReset, nodeBulkPushThreshold+1)
	for i := range rows {
		rows[i] = model.NodePendingReset{NodeId: nodeID, Email: fmt.Sprintf("owed-%02d", i), QueuedAt: 1}
	}
	if err := database.GetDB().Create(&rows).Error; err != nil {
		t.Fatalf("seed pending resets: %v", err)
	}
	rt := &bulkResetRuntime{}
	if err := (&InboundService{}).DeliverNodeResets(context.Background(), nodeID, rt); err != nil {
		t.Fatalf("DeliverNodeResets: %v", err)
	}
	if len(rt.batches) != 1 || len(rt.batches[0]) != len(rows) || len(rt.delivered()) != 0 {
		t.Fatalf("bulk batches %d (first %d emails), per-client calls %d; want one batch of %d",
			len(rt.batches), len(rt.batches[0]), len(rt.delivered()), len(rows))
	}
	if left := pendingResetEmails(t, nodeID); len(left) != 0 {
		t.Fatalf("delivered backlog still queued: %d rows", len(left))
	}
}

// An unreachable node keeps the reset queued until a later delivery lands.
func TestNodeResetReplayedAfterFailure(t *testing.T) {
	nodeID, rec, ib := setupRecordingNode(t, true)
	if _, err := (&InboundService{}).ResetClientTraffic(ib.Id, "reset-lost"); err != nil {
		t.Fatalf("ResetClientTraffic: %v", err)
	}
	if left := pendingResetEmails(t, nodeID); !slices.Equal(left, []string{"reset-lost"}) {
		t.Fatalf("pending after failed delivery = %v, want [reset-lost]", left)
	}

	rec.mu.Lock()
	rec.fail = false
	rec.mu.Unlock()
	if err := (&InboundService{}).DeliverNodeResets(context.Background(), nodeID, rec); err != nil {
		t.Fatalf("DeliverNodeResets: %v", err)
	}
	if got := rec.delivered(); !slices.Equal(got, []string{"reset-lost"}) {
		t.Fatalf("node received resets %v, want [reset-lost]", got)
	}
	if left := pendingResetEmails(t, nodeID); len(left) != 0 {
		t.Fatalf("delivered reset still queued: %v", left)
	}
}

// slowResetRuntime holds each reset until a second one arrives or a short
// timeout passes, so two unserialized deliveries both reach the node.
type slowResetRuntime struct {
	resetRecordingRuntime
	calls atomic.Int32
	both  chan struct{}
}

func (r *slowResetRuntime) ResetClientTraffic(ctx context.Context, ib *model.Inbound, email string) error {
	if r.calls.Add(1) == 2 {
		close(r.both)
	}
	select {
	case <-r.both:
	case <-time.After(300 * time.Millisecond):
	}
	return r.resetRecordingRuntime.ResetClientTraffic(ctx, ib, email)
}

// The sync job and a reset's own delivery can run at once; the node must still
// get each owed reset once, or usage made in between is wiped a second time.
func TestConcurrentNodeResetDeliveriesSendOnce(t *testing.T) {
	setupBulkDB(t)
	const nodeID = 9
	if err := database.GetDB().Create(&model.NodePendingReset{NodeId: nodeID, Email: "once", QueuedAt: 1}).Error; err != nil {
		t.Fatalf("seed pending reset: %v", err)
	}
	rt := &slowResetRuntime{both: make(chan struct{})}
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := (&InboundService{}).DeliverNodeResets(context.Background(), nodeID, rt); err != nil {
				t.Errorf("DeliverNodeResets: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := rt.delivered(); !slices.Equal(got, []string{"once"}) {
		t.Fatalf("node received resets %v, want exactly [once]", got)
	}
}
