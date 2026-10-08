package job

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
)

const (
	nodeHeartbeatConcurrency    = 32
	nodeHeartbeatRequestTimeout = 4 * time.Second
	// Past this many same-direction transitions in one tick, one summary event goes out:
	// per-node events overflow the notifier queues and every chat's rate limit.
	nodeTransitionBurst      = 5
	nodeTransitionBurstNames = 10
)

type NodeHeartbeatJob struct {
	nodeService    service.NodeService
	settingService service.SettingService
	running        sync.Mutex
	health         map[int]nodeHealth
}

type nodeHealth struct {
	failStreak int
	notified   bool
}

type nodeProbe struct {
	node  *model.Node
	patch service.HeartbeatPatch
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
	j.nodeService.RetainEnabledNodeDescendants(nodes)
	if len(nodes) == 0 {
		return
	}

	sem := make(chan struct{}, nodeHeartbeatConcurrency)
	var wg sync.WaitGroup
	var probesMu sync.Mutex
	var probes []nodeProbe
	for _, n := range nodes {
		if !n.Enable {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		n := n
		common.GoRecover("node-heartbeat:"+n.Name, func() {
			defer wg.Done()
			defer func() { <-sem }()
			patch := j.probeOne(n)
			probesMu.Lock()
			probes = append(probes, nodeProbe{node: n, patch: patch})
			probesMu.Unlock()
		})
	}
	wg.Wait()
	publishNodeTransitions(j.healthTransitions(probes))

	if !websocket.HasClients() {
		return
	}
	updated, err := j.nodeService.GetNodeTreeView()
	if err != nil {
		logger.Warning("node heartbeat: load nodes for broadcast failed:", err)
		return
	}
	websocket.BroadcastNodes(updated)
}

func (j *NodeHeartbeatJob) probeOne(n *model.Node) service.HeartbeatPatch {
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
	return patch
}

func (j *NodeHeartbeatJob) healthTransitions(probes []nodeProbe) []eventbus.Event {
	threshold := 1
	if v, err := j.settingService.GetNodeDownThreshold(); err == nil && v > 0 {
		threshold = v
	}
	prev := j.health
	j.health = make(map[int]nodeHealth, len(probes))
	var events []eventbus.Event
	for _, p := range probes {
		h := prev[p.node.Id]
		if p.patch.Status == "online" {
			if h.notified {
				events = append(events, nodeHealthEvent(eventbus.EventNodeUp, p.node, p.patch))
			}
			h = nodeHealth{}
		} else {
			h.failStreak++
			if h.failStreak >= threshold && !h.notified {
				events = append(events, nodeHealthEvent(eventbus.EventNodeDown, p.node, p.patch))
				h.notified = true
			}
		}
		j.health[p.node.Id] = h
	}
	return events
}

func nodeHealthEvent(eventType eventbus.EventType, n *model.Node, patch service.HeartbeatPatch) eventbus.Event {
	source := n.Name
	if source == "" {
		source = "node-" + strconv.Itoa(n.Id)
	}
	return eventbus.Event{
		Type:   eventType,
		Source: source,
		Data: &eventbus.NodeHealthData{
			NodeId:    n.Id,
			LatencyMs: patch.LatencyMs,
			CpuPct:    patch.CpuPct,
			MemPct:    patch.MemPct,
			XrayState: patch.XrayState,
			XrayError: patch.XrayError,
		},
	}
}

// publishNodeTransitions sends one tick's transitions, folding a same-direction burst
// (a master-side blip flips every node at once) into one event naming the nodes.
func publishNodeTransitions(events []eventbus.Event) {
	if EventBus == nil {
		return
	}
	namesByType := make(map[eventbus.EventType][]string)
	for _, e := range events {
		namesByType[e.Type] = append(namesByType[e.Type], e.Source)
	}
	for _, e := range events {
		if len(namesByType[e.Type]) <= nodeTransitionBurst {
			EventBus.Publish(e)
		}
	}
	for _, eventType := range []eventbus.EventType{eventbus.EventNodeDown, eventbus.EventNodeUp} {
		names := namesByType[eventType]
		if len(names) <= nodeTransitionBurst {
			continue
		}
		sort.Strings(names)
		source := strings.Join(names[:min(len(names), nodeTransitionBurstNames)], ", ")
		if extra := len(names) - nodeTransitionBurstNames; extra > 0 {
			source += fmt.Sprintf(" (+%d)", extra)
		}
		EventBus.Publish(eventbus.Event{Type: eventType, Source: source})
	}
}
