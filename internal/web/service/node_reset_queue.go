package service

import (
	"context"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// nodeBulkResetter is a node runtime that can zero many clients in one call.
type nodeBulkResetter interface {
	ResetClientTraffics(ctx context.Context, emails []string) error
}

type nodeEmail struct {
	NodeId int    `gorm:"column:node_id"`
	Email  string `gorm:"column:email"`
}

// queueNodeResets records a reset for every node hosting one of emails (all
// node-hosted clients when emails is nil) and returns the nodes involved.
func queueNodeResets(tx *gorm.DB, emails []string) ([]int, error) {
	base := func() *gorm.DB {
		return tx.Table("clients").
			Select("DISTINCT inbounds.node_id AS node_id, clients.email AS email").
			Joins("JOIN client_inbounds ON client_inbounds.client_id = clients.id").
			Joins("JOIN inbounds ON inbounds.id = client_inbounds.inbound_id").
			Where("inbounds.node_id IS NOT NULL")
	}
	var pairs []nodeEmail
	if emails == nil {
		if err := base().Scan(&pairs).Error; err != nil {
			return nil, err
		}
	} else {
		for _, batch := range chunkStrings(uniqueNonEmptyStrings(emails), sqlInChunk) {
			var page []nodeEmail
			if err := base().Where("clients.email IN ?", batch).Scan(&page).Error; err != nil {
				return nil, err
			}
			pairs = append(pairs, page...)
		}
	}
	if len(pairs) == 0 {
		return nil, nil
	}
	now := time.Now().UnixNano()
	rows := make([]model.NodePendingReset, 0, len(pairs))
	nodes := make(map[int]struct{})
	for _, p := range pairs {
		rows = append(rows, model.NodePendingReset{NodeId: p.NodeId, Email: p.Email, QueuedAt: now})
		nodes[p.NodeId] = struct{}{}
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "node_id"}, {Name: "email"}},
		DoUpdates: clause.AssignmentColumns([]string{"queued_at"}),
	}).CreateInBatches(rows, 200).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	return ids, nil
}

// pendingNodeResetEmails lists the clients whose reset the node still owes.
func pendingNodeResetEmails(tx *gorm.DB, nodeID int) (map[string]struct{}, error) {
	var emails []string
	if err := tx.Model(&model.NodePendingReset{}).Where("node_id = ?", nodeID).Pluck("email", &emails).Error; err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(emails))
	for _, e := range emails {
		out[e] = struct{}{}
	}
	return out, nil
}

var nodeResetDeliveryLocks sync.Map

// DeliverNodeResets sends the node every reset it has not confirmed. A row is
// dropped only after the node accepted it and only if nothing re-queued it since.
func (s *InboundService) DeliverNodeResets(ctx context.Context, nodeID int, rt runtime.Runtime) error {
	lock, _ := nodeResetDeliveryLocks.LoadOrStore(nodeID, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()
	db := database.GetDB()
	var rows []model.NodePendingReset
	if err := db.Where("node_id = ?", nodeID).Order("queued_at, id").Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	var failed error
	bulk, canBulk := rt.(nodeBulkResetter)
	for start := 0; start < len(rows); start += sqlInChunk {
		batch := rows[start:min(start+sqlInChunk, len(rows))]
		emails := make([]string, len(batch))
		for i := range batch {
			emails[i] = batch[i].Email
		}
		if canBulk && len(batch) > nodeBulkPushThreshold {
			if err := bulk.ResetClientTraffics(ctx, emails); err != nil {
				return err
			}
			for i := range batch {
				if err := settleNodeReset(db, &batch[i]); err != nil {
					return err
				}
			}
			continue
		}
		// Each reset settles on its own: one the node never accepts must neither hold back
		// the rest nor have them re-sent, which would wipe the usage made in between.
		for i := range batch {
			if err := rt.ResetClientTraffic(ctx, nil, batch[i].Email); err != nil {
				failed = err
				// To the back of the queue, so it cannot spend the next attempt's budget first.
				db.Model(&model.NodePendingReset{}).Where("id = ? AND queued_at = ?", batch[i].Id, batch[i].QueuedAt).
					Update("queued_at", time.Now().UnixNano())
				if ctx.Err() != nil {
					return failed
				}
				continue
			}
			if err := settleNodeReset(db, &batch[i]); err != nil {
				return err
			}
		}
	}
	return failed
}

// settleNodeReset drops a delivered row unless it was re-queued while it was being sent.
func settleNodeReset(db *gorm.DB, row *model.NodePendingReset) error {
	return db.Where("id = ? AND queued_at = ?", row.Id, row.QueuedAt).Delete(&model.NodePendingReset{}).Error
}

// deliverNodeResetsNow tries each node once right after a reset commits; what
// fails stays queued for the node sync job.
func (s *InboundService) deliverNodeResetsNow(nodeIDs []int) {
	mgr := runtime.GetManager()
	if mgr == nil || len(nodeIDs) == 0 {
		return
	}
	done := make(chan struct{})
	common.GoRecover("node-reset-delivery", func() {
		defer close(done)
		fanoutInboundResults(nodeIDs, nodeFanoutConcurrency, func(i int) struct{} {
			rt, err := mgr.RuntimeFor(&nodeIDs[i])
			if err != nil {
				return struct{}{}
			}
			ctx, cancel := nodePushContext()
			defer cancel()
			if err := s.DeliverNodeResets(ctx, nodeIDs[i], rt); err != nil {
				logger.Warning("reset delivery to", rt.Name(), "deferred to the next sync:", err)
			}
			return struct{}{}
		})
	})
	// A silent node must not hold the request that queued the reset (a node relaying its
	// master's reset): the master would time out and replay it, wiping usage made since.
	select {
	case <-done:
	case <-time.After(nodeResetInlineWait):
	}
}

const nodeResetInlineWait = time.Second
