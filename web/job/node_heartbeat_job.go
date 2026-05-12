package job

import (
	"context"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"github.com/mhsanaei/3x-ui/v3/web/websocket"
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
	updated, err := j.nodeService.GetAll()
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
}
