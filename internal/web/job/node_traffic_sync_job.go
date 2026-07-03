package job

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
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
	nodeClientIpSyncTimeout       = 6 * time.Second
	nodeGlobalPushInterval        = 30 * time.Second
	// nodeInboundSpeedWindowMs is the poll window node-inbound speed deltas are
	// normalized to; it MUST match the dashboard's TRAFFIC_POLL_INTERVAL_S (5s),
	// the fixed divisor the frontend applies to turn a delta into a rate.
	nodeInboundSpeedWindowMs int64 = 5000
)

// inboundSample is a node inbound's last-seen cumulative up/down and the time
// (unix millis) its counter last changed, used to derive a normalized speed.
type inboundSample struct {
	up, down, at int64
}

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
	// noGuidIpEndpoint tracks nodes (by id) whose client-IP attribution endpoint
	// returned 404, so an old-build node is noted once instead of every cycle.
	noGuidIpEndpoint sync.Map
	// prevInboundTotals holds the previous poll's cumulative up/down (and the time
	// the counter last changed) per node inbound tag, so the next poll can derive
	// a per-inbound speed delta — node inbounds have no local Xray poll. Touched
	// only from Run (serialized).
	prevInboundTotals map[string]inboundSample
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
	var activeMu sync.Mutex
	var activeEmails []string
	for _, n := range nodes {
		if !n.Enable || n.Status != "online" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		n := n
		common.GoRecover("node-traffic-sync:"+n.Name, func() {
			defer wg.Done()
			defer func() { <-sem }()
			if emails := j.syncOne(mgr, n, doIpSync); len(emails) > 0 {
				activeMu.Lock()
				activeEmails = append(activeEmails, emails...)
				activeMu.Unlock()
			}
		})
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

	// Prune stale local-online entries (no local active emails or inbound tags
	// to add here — only the local xray poll feeds those) so a stopped local
	// xray's clients and inbounds still age out between traffic polls.
	j.inboundService.RefreshLocalOnlineClients(nil, nil)

	// Derive per-node-inbound speed every tick (keeps the baseline fresh even
	// with no dashboard open); only broadcast it when someone is watching.
	inboundSpeed := j.nodeInboundSpeed()

	if !websocket.HasClients() {
		return
	}

	// Same snapshot-vs-delta split as the local traffic job: above the
	// threshold a full snapshot would be dropped by the hub's payload cap, so
	// send only the rows for clients online on the synced nodes this tick.
	snapshot := true
	if total, countErr := j.inboundService.CountClientTraffics(); countErr != nil {
		logger.Warning("node traffic sync: count client traffics failed:", countErr)
	} else if total > clientStatsSnapshotMaxClients {
		snapshot = false
	}

	var stats []*xray.ClientTraffic
	var statsErr error
	if snapshot {
		stats, statsErr = j.inboundService.GetAllClientTraffics()
	} else {
		stats, statsErr = j.inboundService.GetActiveClientTraffics(activeEmails)
	}
	if statsErr != nil {
		logger.Warning("node traffic sync: get client traffics for websocket failed:", statsErr)
	}

	var lastOnline map[string]int64
	if snapshot {
		var loErr error
		if lastOnline, loErr = j.inboundService.GetClientsLastOnline(); loErr != nil {
			logger.Warning("node traffic sync: get last-online failed:", loErr)
		}
	} else {
		lastOnline = make(map[string]int64, len(stats))
		for _, ct := range stats {
			if ct != nil {
				lastOnline[ct.Email] = ct.LastOnline
			}
		}
	}
	if lastOnline == nil {
		lastOnline = map[string]int64{}
	}

	online := j.inboundService.GetOnlineClients()
	if online == nil {
		online = []string{}
	}
	trafficPayload := map[string]any{
		"onlineClients":  online,
		"onlineByGuid":   j.inboundService.GetOnlineClientsByGuid(),
		"activeInbounds": j.inboundService.GetActiveInboundsByGuid(),
		"lastOnlineMap":  lastOnline,
	}
	// Always send the key so the dashboard clears node inbounds that went idle
	// this tick. A nil result (query error) marshals to null and is skipped
	// client-side, leaving the last shown value untouched; an empty (non-nil)
	// slice marshals to [] and clears stale speeds.
	trafficPayload["nodeTraffics"] = inboundSpeed
	websocket.BroadcastTraffic(trafficPayload)

	clientStats := map[string]any{"snapshot": snapshot}
	if len(stats) > 0 {
		clientStats["clients"] = stats
	}
	if summary, err := j.inboundService.GetInboundsTrafficSummary(); err != nil {
		logger.Warning("node traffic sync: get inbounds summary for websocket failed:", err)
	} else if len(summary) > 0 {
		clientStats["inbounds"] = summary
	}
	if len(clientStats) > 1 {
		websocket.BroadcastClientStats(clientStats)
	}

	if j.structural.takeAndReset() {
		websocket.BroadcastInvalidate(websocket.MessageTypeInbounds)
		websocket.BroadcastInvalidate(websocket.MessageTypeClients)
	}
}

// nodeInboundSpeed derives a per-node-inbound speed delta by diffing the current
// cumulative up/down against the previous poll's, keyed by the central tag the
// dashboard matches. The node's counter keeps climbing while the master can't
// reach it, so the first delta after a gap (node outage, skipped poll, slow
// node) spans more than one poll window; it is normalized to the fixed
// nodeInboundSpeedWindowMs using the real elapsed time so the dashboard's fixed
// divisor yields the true average rate over the gap instead of an impossible
// one-tick spike. The change timestamp only advances when the value actually
// moves, so an idle stretch is averaged correctly when traffic resumes. A reset
// rebaselines to the lower value; a first-seen tag yields no delta until the
// next poll.
func (j *NodeTrafficSyncJob) nodeInboundSpeed() []*xray.Traffic {
	totals, err := j.inboundService.GetNodeInboundTrafficTotals()
	if err != nil {
		return nil
	}
	now := time.Now().UnixMilli()
	deltas := make([]*xray.Traffic, 0, len(totals))
	next := make(map[string]inboundSample, len(totals))
	for tag, cur := range totals {
		prev, ok := j.prevInboundTotals[tag]
		if !ok {
			next[tag] = inboundSample{up: cur[0], down: cur[1], at: now}
			continue
		}
		dUp := cur[0] - prev.up
		dDown := cur[1] - prev.down
		if dUp <= 0 && dDown <= 0 {
			// No movement, or a counter reset: hold the change timestamp so a
			// later jump is averaged over the real elapsed window, not shown as a
			// spike. Adopt the lower value on a reset.
			if cur[0] < prev.up || cur[1] < prev.down {
				next[tag] = inboundSample{up: cur[0], down: cur[1], at: now}
			} else {
				next[tag] = prev
			}
			continue
		}
		if dUp < 0 {
			dUp = 0
		}
		if dDown < 0 {
			dDown = 0
		}
		elapsed := max(now-prev.at, nodeInboundSpeedWindowMs)
		up := dUp * nodeInboundSpeedWindowMs / elapsed
		down := dDown * nodeInboundSpeedWindowMs / elapsed
		if up > 0 || down > 0 {
			deltas = append(deltas, &xray.Traffic{Tag: tag, IsInbound: true, Up: up, Down: down})
		}
		next[tag] = inboundSample{up: cur[0], down: cur[1], at: now}
	}
	j.prevInboundTotals = next
	return deltas
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
			logger.Warningf("node traffic sync: load globals for %s failed: %v", n.Name, err)
			continue
		}
		if len(traffics) == 0 {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		n, remote, traffics := n, remote, traffics
		common.GoRecover("node-global-push:"+n.Name, func() {
			defer wg.Done()
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), nodeTrafficSyncRequestTimeout)
			defer cancel()
			if err := remote.PushGlobalClientTraffics(ctx, masterGuid, traffics); err != nil {
				// An old-build node without the endpoint answers 404 — not worth a
				// warning every cycle.
				if strings.Contains(err.Error(), "HTTP 404") {
					logger.Debugf("node traffic sync: node %s has no global-traffic endpoint (old build)", n.Name)
				} else {
					logger.Warningf("node traffic sync: push globals to %s failed: %v", n.Name, err)
				}
			}
		})
	}
	wg.Wait()
}

// syncOne pulls one node's traffic snapshot and merges it. It returns the
// emails online on that node this tick, feeding the delta broadcast above the
// snapshot threshold; nil on any failure path.
func (j *NodeTrafficSyncJob) syncOne(mgr *runtime.Manager, n *model.Node, doIpSync bool) []string {
	rt, err := mgr.RemoteFor(n)
	if err != nil {
		logger.Warningf("node traffic sync: remote lookup failed for %s: %v", n.Name, err)
		return nil
	}

	if n.ConfigDirty {
		reconcileCtx, reconcileCancel := context.WithTimeout(context.Background(), nodeReconcileTimeout)
		reconcileErr := j.inboundService.ReconcileNode(reconcileCtx, rt, n)
		reconcileCancel()
		if reconcileErr != nil {
			// The dirty flag stays set so reconcile retries next tick, but traffic
			// accounting must keep flowing: one rejected inbound used to starve the
			// whole node's traffic/online sync forever (#5685).
			logger.Warningf("node traffic sync: reconcile for %s failed, continuing with traffic pull: %v", n.Name, reconcileErr)
		} else {
			if clearErr := j.nodeService.ClearNodeDirty(n.Id, n.ConfigDirtyAt); clearErr != nil {
				logger.Warningf("node traffic sync: clear dirty for %s failed: %v", n.Name, clearErr)
			}
			j.structural.set()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), nodeTrafficSyncRequestTimeout)
	defer cancel()

	snap, err := rt.FetchTrafficSnapshot(ctx)
	if err != nil {
		logger.Warningf("node traffic sync: fetch from %s failed: %v", n.Name, err)
		j.inboundService.ClearNodeOnlineClients(n.Id)
		return nil
	}
	service.FilterNodeSnapshot(n, snap)
	_, _, dirty, _, _ := j.nodeService.NodeSyncState(n.Id)
	changed, err := j.inboundService.SetRemoteTraffic(n.Id, snap, dirty)
	if err != nil {
		logger.Warningf("node traffic sync: merge for %s failed: %v", n.Name, err)
		return nil
	}
	if changed {
		j.structural.set()
	}

	active := make([]string, 0, len(snap.OnlineEmails))
	active = append(active, snap.OnlineEmails...)
	for _, emails := range snap.OnlineTree {
		active = append(active, emails...)
	}

	if !doIpSync {
		return active
	}

	ipCtx, ipCancel := context.WithTimeout(context.Background(), nodeClientIpSyncTimeout)
	defer ipCancel()

	nodeIps, err := rt.FetchAllClientIps(ipCtx)
	if err == nil && len(nodeIps) > 0 {
		if err := j.inboundService.MergeInboundClientIps(nodeIps); err != nil {
			logger.Warningf("node traffic sync: merge client ips from %s failed: %v", n.Name, err)
		}
	} else if err != nil {
		logger.Warningf("node traffic sync: fetch client ips from %s failed: %v", n.Name, err)
	}

	masterIps, err := j.inboundService.GetAllInboundClientIps()
	if err != nil {
		logger.Warningf("node traffic sync: load client ips for push to %s failed: %v", n.Name, err)
		return active
	}
	if len(masterIps) > 0 {
		if err := rt.PushAllClientIps(ipCtx, masterIps); err != nil {
			logger.Warningf("node traffic sync: push client ips to %s failed: %v", n.Name, err)
		}
	}

	// Per-node IP attribution: pull the node's guid-keyed subtree (its own
	// observations plus any descendants) so the master can tell which node each
	// IP is on. Old nodes without the endpoint return HTTP 404 every cycle — note
	// it once per node (re-armed on recovery) instead of flooding the log.
	if guidTrees, err := rt.FetchClientIpsByGuid(ipCtx); err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			if _, seen := j.noGuidIpEndpoint.LoadOrStore(n.Id, true); !seen {
				logger.Debugf("node traffic sync: node %s has no client-IP attribution endpoint (old build)", n.Name)
			}
		} else {
			logger.Debugf("node traffic sync: fetch client ip attribution from %s failed: %v", n.Name, err)
		}
	} else {
		j.noGuidIpEndpoint.Delete(n.Id)
		if len(guidTrees) > 0 {
			if err := j.inboundService.MergeClientIpsByGuid(n, guidTrees); err != nil {
				logger.Warningf("node traffic sync: merge client ip attribution from %s failed: %v", n.Name, err)
			}
		}
	}
	return active
}
