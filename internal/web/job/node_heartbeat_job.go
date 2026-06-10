package job

import (
	"context"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
)

const (
	nodeHeartbeatConcurrency    = 32
	nodeHeartbeatRequestTimeout = 4 * time.Second
)

type NodeHeartbeatJob struct {
	nodeService service.NodeService
	running     sync.Mutex
}

func NewNodeHeartbeatJob() *NodeHeartbeatJob {
	return &NodeHeartbeatJob{}
}

func (j *NodeHeartbeatJob) Run() {
	if !j.running.TryLock() {
		return
	}
	defer j.running.Unlock()

	nodes, err := j.nodeService.GetAll()
	if err != nil {
		logger.Warning("node heartbeat: load nodes failed:", err)
		return
	}
	if len(nodes) == 0 {
		return
	}

	sem := make(chan struct{}, nodeHeartbeatConcurrency)
	var wg sync.WaitGroup
	for _, n := range nodes {
		if !n.Enable {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(n *model.Node) {
			defer wg.Done()
			defer func() { <-sem }()
			j.probeOne(n)
		}(n)
	}
	wg.Wait()

	if !websocket.HasClients() {
		return
	}
	updated, err := j.nodeService.GetNodeTree()
	if err != nil {
		logger.Warning("node heartbeat: load nodes for broadcast failed:", err)
		return
	}
	websocket.BroadcastNodes(updated)
}

func (j *NodeHeartbeatJob) probeOne(n *model.Node) {
	ctx, cancel := context.WithTimeout(context.Background(), nodeHeartbeatRequestTimeout)
	defer cancel()
	patch, err := j.nodeService.Probe(ctx, n)
	if err != nil {
		patch.Status = "offline"
	} else {
		patch.Status = "online"
	}
	if updErr := j.nodeService.UpdateHeartbeat(n.Id, patch); updErr != nil {
		logger.Warning("node heartbeat: update node", n.Id, "failed:", updErr)
	}
	// Learn the nodes this node manages so the panel can surface them as
	// transitive sub-nodes (#4983). Fresh context — the probe budget above may
	// be spent. Drop them when the node is unreachable.
	if patch.Status == "online" {
		dctx, dcancel := context.WithTimeout(context.Background(), nodeHeartbeatRequestTimeout)
		j.nodeService.RefreshDescendants(dctx, n)
		dcancel()
	} else {
		j.nodeService.ClearDescendants(n.Id)
	}
}
