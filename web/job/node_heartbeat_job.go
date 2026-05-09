package job

import (
	"context"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/websocket"
)

// nodeHeartbeatConcurrency caps how many remote panels we probe at once.
// Plenty of headroom for typical deployments (tens of nodes) without
// letting a misconfigured run open thousands of sockets at once.
const nodeHeartbeatConcurrency = 32

// nodeHeartbeatRequestTimeout bounds a single probe. The cron is @every 10s,
// so this needs to stay well under that to avoid run pile-up.
const nodeHeartbeatRequestTimeout = 6 * time.Second

// NodeHeartbeatJob probes every enabled remote node once per cron tick
// and persists the result. Disabled nodes are skipped entirely so a
// long-broken node can be parked without burning sockets every 10s.
type NodeHeartbeatJob struct {
	nodeService service.NodeService

	// Coarse mutex prevents two ticks running concurrently if probes
	// pile up under network failure. The next tick simply skips when
	// the previous one is still draining.
	running sync.Mutex
}

// NewNodeHeartbeatJob constructs a heartbeat job. The robfig/cron
// scheduler will hand the same instance to every tick, so the
// running mutex carries across runs as intended.
func NewNodeHeartbeatJob() *NodeHeartbeatJob {
	return &NodeHeartbeatJob{}
}

func (j *NodeHeartbeatJob) Run() {
	if !j.running.TryLock() {
		// Previous tick still in flight — skip this one.
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

	// Push the fresh list to any open Nodes page over WebSocket so the
	// status / latency / cpu / mem cells update without the user clicking
	// refresh. Skip the DB read entirely when no browser is connected —
	// matches the gating pattern in xray_traffic_job.
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

// probeOne runs a single probe and persists the result. We deliberately
// don't return errors — partial failures across the node set should not
// abort other probes, and the LastError column carries the message for
// the UI to surface.
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
		// A row deleted mid-tick produces "rows affected = 0", which
		// gorm reports as nil — so any error we get here is real.
		logger.Warning("node heartbeat: update node", n.Id, "failed:", updErr)
	}
}
