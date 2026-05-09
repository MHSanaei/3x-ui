package job

import (
	"context"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/runtime"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/websocket"
)

// nodeTrafficSyncConcurrency caps how many nodes we sync simultaneously.
// Each sync does three HTTP calls in series, so the wall-clock budget
// per node is the request timeout below — keeping the cap modest avoids
// flooding the network while still getting through dozens of nodes
// inside a 10s tick.
const nodeTrafficSyncConcurrency = 8

// nodeTrafficSyncRequestTimeout bounds the per-node sync. Three probes
// in series at 8s each would blow past the cron interval, so the budget
// here covers the whole snapshot — FetchTrafficSnapshot internally caps
// each HTTP call at the runtime's own 10s ceiling but uses ctx for the
// outer total.
const nodeTrafficSyncRequestTimeout = 8 * time.Second

// NodeTrafficSyncJob pulls absolute traffic + online stats from every
// enabled, currently-online remote node and merges them into the central
// DB. Mirrors NodeHeartbeatJob's structure: TryLock to skip pile-ups,
// errgroup-style fan-out with a concurrency cap, per-node ctx timeout.
//
// Offline nodes are skipped entirely — the heartbeat job already owns
// status tracking, and we'd just waste sockets retrying a node we know
// is unreachable. As soon as heartbeat marks a node online again, the
// next traffic tick picks it up.
type NodeTrafficSyncJob struct {
	nodeService    service.NodeService
	inboundService service.InboundService

	// Coarse mutex prevents two ticks running concurrently if a single
	// sync stalls past the 10s cron interval (rare but possible when
	// many nodes are slow simultaneously).
	running sync.Mutex
}

// NewNodeTrafficSyncJob builds a singleton sync job. Cron hands the same
// instance to every tick so the running mutex is preserved across runs.
func NewNodeTrafficSyncJob() *NodeTrafficSyncJob {
	return &NodeTrafficSyncJob{}
}

func (j *NodeTrafficSyncJob) Run() {
	if !j.running.TryLock() {
		return
	}
	defer j.running.Unlock()

	mgr := runtime.GetManager()
	if mgr == nil {
		// Server still booting — pre-Manager runs are normal during
		// the first few seconds of startup.
		return
	}

	nodes, err := j.nodeService.GetAll()
	if err != nil {
		logger.Warning("node traffic sync: load nodes failed:", err)
		return
	}
	if len(nodes) == 0 {
		return
	}

	sem := make(chan struct{}, nodeTrafficSyncConcurrency)
	var wg sync.WaitGroup
	for _, n := range nodes {
		if !n.Enable || n.Status != "online" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(n *model.Node) {
			defer wg.Done()
			defer func() { <-sem }()
			j.syncOne(mgr, n)
		}(n)
	}
	wg.Wait()

	// One broadcast per tick, batched across all nodes — frontend code
	// is invariant to whether the rows came from local xray or a node,
	// so we reuse the same WebSocket envelope XrayTrafficJob uses.
	if websocket.HasClients() {
		online := j.inboundService.GetOnlineClients()
		if online == nil {
			online = []string{}
		}
		lastOnline, err := j.inboundService.GetClientsLastOnline()
		if err != nil {
			logger.Warning("node traffic sync: get last-online failed:", err)
		}
		if lastOnline == nil {
			lastOnline = map[string]int64{}
		}
		websocket.BroadcastTraffic(map[string]any{
			"onlineClients": online,
			"lastOnlineMap": lastOnline,
		})
	}
}

// syncOne fetches and merges one node's snapshot. Errors are logged
// per-node and don't propagate; one slow node shouldn't keep the rest
// from running.
func (j *NodeTrafficSyncJob) syncOne(mgr *runtime.Manager, n *model.Node) {
	ctx, cancel := context.WithTimeout(context.Background(), nodeTrafficSyncRequestTimeout)
	defer cancel()

	rt, err := mgr.RemoteFor(n)
	if err != nil {
		logger.Warning("node traffic sync: remote lookup failed for", n.Name, ":", err)
		return
	}
	snap, err := rt.FetchTrafficSnapshot(ctx)
	if err != nil {
		logger.Warning("node traffic sync: fetch from", n.Name, "failed:", err)
		// Drop node-online contribution so a hiccup doesn't leave the
		// online filter showing stale clients indefinitely.
		j.inboundService.ClearNodeOnlineClients(n.Id)
		return
	}
	if err := j.inboundService.SetRemoteTraffic(n.Id, snap); err != nil {
		logger.Warning("node traffic sync: merge for", n.Name, "failed:", err)
	}
}
