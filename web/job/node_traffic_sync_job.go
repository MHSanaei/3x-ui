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

type emailSet struct {
	mu sync.Mutex
	m  map[string]struct{}
}

func newEmailSet() *emailSet { return &emailSet{m: make(map[string]struct{})} }

func (s *emailSet) addAll(emails []string) {
	if len(emails) == 0 {
		return
	}
	s.mu.Lock()
	for _, e := range emails {
		if e != "" {
			s.m[e] = struct{}{}
		}
	}
	s.mu.Unlock()
}

func (s *emailSet) slice() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.m))
	for e := range s.m {
		out = append(out, e)
	}
	return out
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

	touched := newEmailSet()
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
			j.syncOne(mgr, n, touched)
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
	if emails := touched.slice(); len(emails) > 0 {
		if stats, err := j.inboundService.GetActiveClientTraffics(emails); err != nil {
			logger.Warning("node traffic sync: get client traffics for websocket failed:", err)
		} else if len(stats) > 0 {
			clientStats["clients"] = stats
		}
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

func (j *NodeTrafficSyncJob) syncOne(mgr *runtime.Manager, n *model.Node, touched *emailSet) {
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
	for _, ib := range snap.Inbounds {
		if ib == nil {
			continue
		}
		emails := make([]string, 0, len(ib.ClientStats))
		for _, cs := range ib.ClientStats {
			if cs.Email != "" {
				emails = append(emails, cs.Email)
			}
		}
		touched.addAll(emails)
	}
}
