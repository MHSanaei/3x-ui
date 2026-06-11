package job

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const (
	nodeTrafficSyncConcurrency    = 8
	nodeTrafficSyncRequestTimeout = 4 * time.Second
	nodeReconcileTimeout          = 30 * time.Second
	nodeClientIpSyncInterval      = 10 * time.Second
	nodeGlobalPushInterval        = 30 * time.Second
)

type NodeTrafficSyncJob struct {
	nodeService    service.NodeService
	inboundService service.InboundService
	settingService service.SettingService
	xrayService    service.XrayService
	running        sync.Mutex
	structural     atomicBool
	ipSyncMu       sync.Mutex
	lastIpSync     int64
	globalPushMu   sync.Mutex
	lastGlobalPush int64
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

	// Decide once per tick whether this run also syncs client IPs, and stamp the
	// clock before the loop so two back-to-back 5s ticks can't both qualify.
	doIpSync := false
	j.ipSyncMu.Lock()
	if now := time.Now().Unix(); now-j.lastIpSync >= int64(nodeClientIpSyncInterval/time.Second) {
		doIpSync = true
		j.lastIpSync = now
	}
	j.ipSyncMu.Unlock()

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
			j.syncOne(mgr, n, doIpSync)
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

	j.maybePushGlobals(mgr, nodes)

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
		"onlineByGuid":   j.inboundService.GetOnlineClientsByGuid(),
		"activeInbounds": j.inboundService.GetActiveInboundsByGuid(),
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

// maybePushGlobals broadcasts this panel's aggregated per-client usage to its
// online nodes so each node can display the client's cross-panel total and
// enforce its quota locally (see InboundService.AcceptGlobalTraffic). Scoped
// per node to the clients that node actually hosts, and throttled — the
// aggregates only need to reach nodes on a human timescale, not every poll.
func (j *NodeTrafficSyncJob) maybePushGlobals(mgr *runtime.Manager, nodes []*model.Node) {
	j.globalPushMu.Lock()
	now := time.Now().Unix()
	if now-j.lastGlobalPush < int64(nodeGlobalPushInterval/time.Second) {
		j.globalPushMu.Unlock()
		return
	}
	j.lastGlobalPush = now
	j.globalPushMu.Unlock()

	masterGuid, err := j.settingService.GetPanelGuid()
	if err != nil || masterGuid == "" {
		return
	}

	sem := make(chan struct{}, nodeTrafficSyncConcurrency)
	var wg sync.WaitGroup
	for _, n := range nodes {
		if !n.Enable || n.Status != "online" {
			continue
		}
		remote, err := mgr.RemoteFor(n)
		if err != nil {
			continue
		}
		traffics, err := j.inboundService.GetNodeClientTraffics(n.Id)
		if err != nil {
			logger.Warning("node traffic sync: load globals for", n.Name, "failed:", err)
			continue
		}
		if len(traffics) == 0 {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(n *model.Node, remote *runtime.Remote, traffics []*xray.ClientTraffic) {
			defer wg.Done()
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), nodeTrafficSyncRequestTimeout)
			defer cancel()
			if err := remote.PushGlobalClientTraffics(ctx, masterGuid, traffics); err != nil {
				// An old-build node without the endpoint answers 404 — not worth a
				// warning every cycle.
				if strings.Contains(err.Error(), "HTTP 404") {
					logger.Debug("node traffic sync: node", n.Name, "has no global-traffic endpoint (old build)")
				} else {
					logger.Warning("node traffic sync: push globals to", n.Name, "failed:", err)
				}
			}
		}(n, remote, traffics)
	}
	wg.Wait()
}

func (j *NodeTrafficSyncJob) syncOne(mgr *runtime.Manager, n *model.Node, doIpSync bool) {
	rt, err := mgr.RemoteFor(n)
	if err != nil {
		logger.Warning("node traffic sync: remote lookup failed for", n.Name, ":", err)
		return
	}

	if n.ConfigDirty {
		reconcileCtx, reconcileCancel := context.WithTimeout(context.Background(), nodeReconcileTimeout)
		reconcileErr := j.inboundService.ReconcileNode(reconcileCtx, rt, n)
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
	service.FilterNodeSnapshot(n, snap)
	_, _, dirty, _, _ := j.nodeService.NodeSyncState(n.Id)
	changed, err := j.inboundService.SetRemoteTraffic(n.Id, snap, dirty)
	if err != nil {
		logger.Warning("node traffic sync: merge for", n.Name, "failed:", err)
		return
	}
	if changed {
		j.structural.set()
	}

	if !doIpSync {
		return
	}

	nodeIps, err := rt.FetchAllClientIps(ctx)
	if err == nil && len(nodeIps) > 0 {
		if err := j.inboundService.MergeInboundClientIps(nodeIps); err != nil {
			logger.Warning("node traffic sync: merge client ips from", n.Name, "failed:", err)
		}
	} else if err != nil {
		logger.Warning("node traffic sync: fetch client ips from", n.Name, "failed:", err)
	}

	masterIps, err := j.inboundService.GetAllInboundClientIps()
	if err != nil {
		logger.Warning("node traffic sync: load client ips for push to", n.Name, "failed:", err)
		return
	}
	if len(masterIps) > 0 {
		if err := rt.PushAllClientIps(ctx, masterIps); err != nil {
			logger.Warning("node traffic sync: push client ips to", n.Name, "failed:", err)
		}
	}
}
