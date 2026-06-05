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
	nodeReconcileTimeout          = 30 * time.Second
)

type NodeTrafficSyncJob struct {
	nodeService    service.NodeService
	inboundService service.InboundService
	settingService service.SettingService
	xrayService    service.XrayService
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

	_, clientsDisabled, err := j.inboundService.AddTraffic(nil, nil)
	if err != nil {
		logger.Warning("node traffic sync: depletion check failed:", err)
	}
	if clientsDisabled {
		if restartOnDisable, settingErr := j.settingService.GetRestartXrayOnClientDisable(); settingErr == nil && restartOnDisable {
			if err := j.xrayService.RestartXray(true); err != nil {
				logger.Warning("node traffic sync: restart xray after disabling clients failed:", err)
				j.xrayService.SetToNeedRestart()
			}
		} else if settingErr != nil {
			logger.Warning("node traffic sync: get RestartXrayOnClientDisable failed:", settingErr)
		}
		j.structural.set()
	}

	lastOnline, err := j.inboundService.GetClientsLastOnline()
	if err != nil {
		logger.Warning("node traffic sync: get last-online failed:", err)
	}
	if lastOnline == nil {
		lastOnline = map[string]int64{}
	}

	// Prune stale local-online entries (no local active emails or inbound tags
	// to add here — only the local xray poll feeds those) so a stopped local
	// xray's clients and inbounds still age out between traffic polls.
	j.inboundService.RefreshLocalOnlineClients(nil, nil)

	if !websocket.HasClients() {
		return
	}

	online := j.inboundService.GetOnlineClients()
	if online == nil {
		online = []string{}
	}
	websocket.BroadcastTraffic(map[string]any{
		"onlineClients":  online,
		"onlineByNode":   j.inboundService.GetOnlineClientsByNode(),
		"activeInbounds": j.inboundService.GetActiveInboundsByNode(),
		"lastOnlineMap":  lastOnline,
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
		websocket.BroadcastInvalidate(websocket.MessageTypeClients)
	}
}

func (j *NodeTrafficSyncJob) syncOne(mgr *runtime.Manager, n *model.Node) {
	rt, err := mgr.RemoteFor(n)
	if err != nil {
		logger.Warning("node traffic sync: remote lookup failed for", n.Name, ":", err)
		return
	}

	if n.ConfigDirty {
		reconcileCtx, reconcileCancel := context.WithTimeout(context.Background(), nodeReconcileTimeout)
		reconcileErr := j.inboundService.ReconcileNode(reconcileCtx, rt, n.Id)
		reconcileCancel()
		if reconcileErr != nil {
			logger.Warning("node traffic sync: reconcile for", n.Name, "failed:", reconcileErr)
			return
		}
		if clearErr := j.nodeService.ClearNodeDirty(n.Id, n.ConfigDirtyAt); clearErr != nil {
			logger.Warning("node traffic sync: clear dirty for", n.Name, "failed:", clearErr)
		}
		j.structural.set()
	}

	ctx, cancel := context.WithTimeout(context.Background(), nodeTrafficSyncRequestTimeout)
	defer cancel()

	snap, err := rt.FetchTrafficSnapshot(ctx)
	if err != nil {
		logger.Warning("node traffic sync: fetch from", n.Name, "failed:", err)
		j.inboundService.ClearNodeOnlineClients(n.Id)
		return
	}
	_, _, dirty, _, _ := j.nodeService.NodeSyncState(n.Id)
	changed, err := j.inboundService.SetRemoteTraffic(n.Id, snap, dirty)
	if err != nil {
		logger.Warning("node traffic sync: merge for", n.Name, "failed:", err)
		return
	}
	if changed {
		j.structural.set()
	}
}
