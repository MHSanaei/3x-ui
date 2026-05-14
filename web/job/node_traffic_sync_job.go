package job

import (
	"context"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"github.com/mhsanaei/3x-ui/v3/web/websocket"
)

const (
	nodeTrafficSyncConcurrency    = 8
	nodeTrafficSyncRequestTimeout = 4 * time.Second
)

type NodeTrafficSyncJob struct {
	nodeService    service.NodeService
	inboundService service.InboundService
	running        sync.Mutex
	structural     atomicBool
}

type atomicBool struct {
	mu sync.Mutex
	v  bool
}

func (a *atomicBool) set() {
	a.mu.Lock()
	a.v = true
	a.mu.Unlock()
}

func (a *atomicBool) takeAndReset() bool {
	a.mu.Lock()
	v := a.v
	a.v = false
	a.mu.Unlock()
	return v
}

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

	if !websocket.HasClients() {
		return
	}

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

	clientStats := map[string]any{}
	if stats, err := j.inboundService.GetAllClientTraffics(); err != nil {
		logger.Warning("node traffic sync: get all client traffics for websocket failed:", err)
	} else if len(stats) > 0 {
		clientStats["clients"] = stats
	}
	if summary, err := j.inboundService.GetInboundsTrafficSummary(); err != nil {
		logger.Warning("node traffic sync: get inbounds summary for websocket failed:", err)
	} else if len(summary) > 0 {
		clientStats["inbounds"] = summary
	}
	if len(clientStats) > 0 {
		websocket.BroadcastClientStats(clientStats)
	}

	if j.structural.takeAndReset() {
		websocket.BroadcastInvalidate(websocket.MessageTypeInbounds)
	}
}

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
		j.inboundService.ClearNodeOnlineClients(n.Id)
		return
	}
	changed, err := j.inboundService.SetRemoteTraffic(n.Id, snap)
	if err != nil {
		logger.Warning("node traffic sync: merge for", n.Name, "failed:", err)
		return
	}
	if changed {
		j.structural.set()
	}
}
