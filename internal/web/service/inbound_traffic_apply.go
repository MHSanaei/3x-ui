package service

import (
	"context"
	"slices"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

type trafficLocalApplyAction uint8

const (
	trafficAddUser trafficLocalApplyAction = iota + 1
	trafficRemoveUser
	trafficDisableInbound
)

type trafficLocalApplyPlan struct {
	action  trafficLocalApplyAction
	inbound model.Inbound
	client  map[string]any
	email   string
}

type trafficMutationBatch struct {
	localPlans  []trafficLocalApplyPlan
	remotePlans []trafficInboundUpdatePlan
	nodeIDs     map[int]struct{}
	// renewedEmails get their MTProto sidecar quota zeroed once the tick commits.
	renewedEmails []string
}

type trafficInboundUpdatePlan struct{ oldInbound, newInbound model.Inbound }

func newTrafficMutationBatch() *trafficMutationBatch {
	return &trafficMutationBatch{nodeIDs: make(map[int]struct{})}
}

func (b *trafficMutationBatch) addNode(nodeID int) {
	if nodeID > 0 {
		b.nodeIDs[nodeID] = struct{}{}
	}
}

func (b *trafficMutationBatch) markNodesTx(tx *gorm.DB) error {
	if b == nil {
		return nil
	}
	nodeSvc := NodeService{}
	for nodeID := range b.nodeIDs {
		if err := nodeSvc.MarkNodeDirtyTx(tx, nodeID); err != nil {
			return err
		}
	}
	return nil
}

// applyTrafficRemotePlans is bounded like every per-client node push: the nodes are
// already dirty, so an offline or slow one defers to the reconcile.
func (s *InboundService) applyTrafficRemotePlans(plans []trafficInboundUpdatePlan) bool {
	byNode := make(map[int][]int)
	var nodeIDs []int
	for i := range plans {
		nodeID := 0
		if plans[i].newInbound.NodeID != nil {
			nodeID = *plans[i].newInbound.NodeID
		}
		if _, seen := byNode[nodeID]; !seen {
			nodeIDs = append(nodeIDs, nodeID)
		}
		byNode[nodeID] = append(byNode[nodeID], i)
	}
	failed, panics := fanoutInboundResults(nodeIDs, nodeFanoutConcurrency, func(n int) bool {
		// One budget per node, not per inbound: a node choosing how many inbounds it reports
		// must not choose how long the sync job waits on it before serving other nodes.
		ctx, cancel := nodePushContext()
		defer cancel()
		indexes := byNode[nodeIDs[n]]
		nodeFailed, nodePanics := fanoutInboundResults(indexes, inboundFanoutConcurrency, func(k int) bool {
			plan := &plans[indexes[k]]
			rt, push, _, err := s.nodePushPlan(&plan.newInbound)
			if err == nil && push {
				err = rt.UpdateInbound(ctx, &plan.oldInbound, &plan.newInbound)
			}
			if err != nil {
				logger.Debug("traffic post-commit remote apply failed:", err)
			}
			return err != nil
		})
		return slices.Contains(nodeFailed, true) || slices.ContainsFunc(nodePanics, func(p error) bool { return p != nil })
	})
	needRestart := false
	for i := range failed {
		needRestart = needRestart || failed[i] || panics[i] != nil
	}
	return needRestart
}

func (s *InboundService) applyTrafficMutationBatch(b *trafficMutationBatch) bool {
	if b == nil {
		return false
	}
	needRestart := false
	for i := range b.localPlans {
		plan := &b.localPlans[i]
		if plan.inbound.Protocol == model.MTProto {
			s.applyLocalMtproto(plan.inbound.Id)
			continue
		}
		if plan.inbound.Protocol == model.AmneziaWG {
			s.applyLocalAmneziaWG(plan.inbound.Id)
			continue
		}
		if plan.inbound.Protocol == model.TUIC {
			s.applyLocalTuic(plan.inbound.Id)
			continue
		}
		rt, err := s.runtimeFor(&plan.inbound)
		if err == nil {
			switch plan.action {
			case trafficAddUser:
				err = rt.AddUser(context.Background(), &plan.inbound, plan.client)
			case trafficRemoveUser:
				err = rt.RemoveUser(context.Background(), &plan.inbound, plan.email)
				if err != nil && strings.Contains(err.Error(), "not found") {
					err = nil
				}
			case trafficDisableInbound:
				err = rt.DelInbound(context.Background(), &plan.inbound)
				if xray.IsMissingHandlerErr(err) {
					err = nil
				}
			}
		}
		if err != nil {
			logger.Debug("traffic post-commit runtime apply failed:", err)
			needRestart = true
		}
	}
	return needRestart
}
