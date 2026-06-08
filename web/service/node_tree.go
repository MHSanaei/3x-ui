package service

import (
	"context"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/web/runtime"
)

// LocalDescendants returns this panel's read-only summaries of the nodes it
// directly manages, so a parent panel can surface them as transitive sub-nodes
// (#4983). Only nodes with a known GUID are included — a stable identity is
// required to attribute them one hop up. Not recursive: each panel reports its
// own direct nodes, and a master walks one level via each direct node's
// endpoint, which covers the Node1 -> Node2 -> Node3 case.
func (s *NodeService) LocalDescendants() ([]model.NodeSummary, error) {
	selfGuid, _ := (&SettingService{}).GetPanelGuid()
	db := database.GetDB()
	var nodes []*model.Node
	if err := db.Model(model.Node{}).Order("id asc").Find(&nodes).Error; err != nil {
		return nil, err
	}
	out := make([]model.NodeSummary, 0, len(nodes))
	for _, n := range nodes {
		if n.Guid == "" {
			continue
		}
		out = append(out, model.NodeSummary{
			Guid:          n.Guid,
			ParentGuid:    selfGuid,
			Name:          n.Name,
			Address:       n.Address,
			Scheme:        n.Scheme,
			Port:          n.Port,
			Status:        n.Status,
			LastHeartbeat: n.LastHeartbeat,
			LatencyMs:     n.LatencyMs,
			PanelVersion:  n.PanelVersion,
			XrayVersion:   n.XrayVersion,
			XrayState:     n.XrayState,
			XrayError:     n.XrayError,
		})
	}
	return out, nil
}

var (
	nodeDescendantsMu    sync.RWMutex
	nodeDescendantsCache = map[int][]model.NodeSummary{}
)

// RefreshDescendants pulls a direct node's published sub-node summaries and
// caches them keyed by node id. Best-effort: a fetch error keeps the last good
// set (the node may be briefly unreachable). Called from the heartbeat job.
func (s *NodeService) RefreshDescendants(ctx context.Context, n *model.Node) {
	if n == nil {
		return
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return
	}
	rt, err := mgr.RemoteFor(n)
	if err != nil {
		return
	}
	summaries, err := rt.GetDescendants(ctx)
	if err != nil {
		return
	}
	nodeDescendantsMu.Lock()
	if len(summaries) == 0 {
		delete(nodeDescendantsCache, n.Id)
	} else {
		nodeDescendantsCache[n.Id] = summaries
	}
	nodeDescendantsMu.Unlock()
}

// ClearDescendants drops a node's cached sub-node summaries (its probe failed).
func (s *NodeService) ClearDescendants(nodeID int) {
	nodeDescendantsMu.Lock()
	delete(nodeDescendantsCache, nodeID)
	nodeDescendantsMu.Unlock()
}

func cachedDescendants() []model.NodeSummary {
	nodeDescendantsMu.RLock()
	defer nodeDescendantsMu.RUnlock()
	out := make([]model.NodeSummary, 0)
	for _, list := range nodeDescendantsCache {
		out = append(out, list...)
	}
	return out
}

// GetNodeTree returns the direct nodes plus any transitive sub-nodes learned
// from them, with per-GUID counts so each node shows only the inbounds/online
// it physically hosts (#4983). Direct nodes carry the master's own GUID as
// ParentGuid; a transitive node carries its parent node's GUID. Transitive
// nodes are read-only projections (Id == 0). Used by the Nodes page and the
// heartbeat broadcast — never for probing/syncing, which stay on GetAll.
func (s *NodeService) GetNodeTree() ([]*model.Node, error) {
	nodes, err := s.GetAll()
	if err != nil {
		return nodes, err
	}
	selfGuid, _ := (&SettingService{}).GetPanelGuid()
	directGuids := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		n.ParentGuid = selfGuid
		if n.Guid != "" {
			directGuids[n.Guid] = struct{}{}
		}
	}

	seen := make(map[string]struct{})
	var transitive []*model.Node
	for _, sum := range cachedDescendants() {
		if sum.Guid == "" {
			continue
		}
		if _, ok := directGuids[sum.Guid]; ok {
			continue // already shown as a direct node
		}
		if _, ok := seen[sum.Guid]; ok {
			continue
		}
		seen[sum.Guid] = struct{}{}
		transitive = append(transitive, &model.Node{
			Guid:          sum.Guid,
			ParentGuid:    sum.ParentGuid,
			Name:          sum.Name,
			Address:       sum.Address,
			Scheme:        sum.Scheme,
			Port:          sum.Port,
			Status:        sum.Status,
			LastHeartbeat: sum.LastHeartbeat,
			LatencyMs:     sum.LatencyMs,
			PanelVersion:  sum.PanelVersion,
			XrayVersion:   sum.XrayVersion,
			XrayState:     sum.XrayState,
			XrayError:     sum.XrayError,
			Transitive:    true,
		})
	}
	if len(transitive) == 0 {
		return nodes, nil
	}

	all := make([]*model.Node, 0, len(nodes)+len(transitive))
	all = append(all, nodes...)
	all = append(all, transitive...)
	s.recountByGuid(all, selfGuid)
	return all, nil
}

// recountByGuid recomputes InboundCount/OnlineCount/DepletedCount for every node
// in the tree, keyed by the GUID that physically hosts each inbound, so a direct
// node shows only its own inbounds and each transitive node shows its own
// (#4983). In a flat topology the per-GUID and per-node-id counts coincide, so
// this only changes behaviour once a transitive node exists.
func (s *NodeService) recountByGuid(nodes []*model.Node, selfGuid string) {
	db := database.GetDB()
	type ibRow struct {
		Id             int
		NodeID         *int   `gorm:"column:node_id"`
		OriginNodeGuid string `gorm:"column:origin_node_guid"`
	}
	var ibRows []ibRow
	if err := db.Table("inbounds").Select("id, node_id, origin_node_guid").Scan(&ibRows).Error; err != nil {
		return
	}
	effByInbound := make(map[int]string, len(ibRows))
	inboundCountByGuid := make(map[string]int)
	ids := make([]int, 0, len(ibRows))
	for _, r := range ibRows {
		guid := r.OriginNodeGuid
		if guid == "" {
			if r.NodeID != nil {
				guid = synthNodeGuid(*r.NodeID)
			} else {
				guid = selfGuid
			}
		}
		effByInbound[r.Id] = guid
		inboundCountByGuid[guid]++
		ids = append(ids, r.Id)
	}

	now := time.Now().UnixMilli()
	depletedByGuid := make(map[string]int)
	if len(ids) > 0 {
		type tRow struct {
			InboundID  int `gorm:"column:inbound_id"`
			Enable     bool
			Total      int64
			Up         int64
			Down       int64
			ExpiryTime int64 `gorm:"column:expiry_time"`
		}
		var tRows []tRow
		if err := db.Table("client_traffics").
			Select("inbound_id, enable, total, up, down, expiry_time").
			Where("inbound_id IN ?", ids).Scan(&tRows).Error; err == nil {
			for _, row := range tRows {
				guid, ok := effByInbound[row.InboundID]
				if !ok {
					continue
				}
				expired := row.ExpiryTime > 0 && row.ExpiryTime <= now
				exhausted := row.Total > 0 && row.Up+row.Down >= row.Total
				if expired || exhausted || !row.Enable {
					depletedByGuid[guid]++
				}
			}
		}
	}

	onlineByGuid := s.onlineEmailsByGuid()
	for _, n := range nodes {
		guid := effectiveNodeGuid(n)
		n.InboundCount = inboundCountByGuid[guid]
		n.OnlineCount = len(onlineByGuid[guid])
		n.DepletedCount = depletedByGuid[guid]
	}
}
