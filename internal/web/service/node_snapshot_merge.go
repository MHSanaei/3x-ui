package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"sync"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

var nodeSnapshotMergeMu sync.Mutex

type nodeMergeCache struct {
	db                *gorm.DB
	revision          string
	lifecycleRevision string
	central           []model.Inbound
	attached          map[int]map[string]struct{}
	inboundIDs        map[int]int
}

func snapshotInboundMapping(snap *runtime.TrafficSnapshot, central map[string]*model.Inbound) (map[int]int, bool) {
	ids := make(map[int]int, len(snap.Inbounds))
	seen := make(map[int]bool, len(snap.Inbounds))
	for _, ib := range snap.Inbounds {
		if ib == nil || ib.Id <= 0 || ids[ib.Id] != 0 {
			return nil, false
		}
		c := central[ib.Tag]
		if c == nil || seen[c.Id] {
			return nil, false
		}
		ids[ib.Id], seen[c.Id] = c.Id, true
	}
	return ids, true
}

// Resolve the whole association before using any cached counters; tags may be deployed aliases.
func (c *nodeMergeCache) restoreInboundAliases(nodeID int, snap *runtime.TrafficSnapshot, central []model.Inbound) (map[string]*model.Inbound, bool) {
	if len(c.inboundIDs) != len(snap.Inbounds) || len(c.attached) != len(snap.Inbounds) {
		return nil, false
	}
	byID := make(map[int]*model.Inbound, len(central))
	for i := range central {
		byID[central[i].Id] = &central[i]
	}
	aliases := make(map[string]*model.Inbound, len(snap.Inbounds))
	seenIDs, seenCentral := map[int]bool{}, map[int]bool{}
	for _, ib := range snap.Inbounds {
		if ib == nil || ib.Id <= 0 || seenIDs[ib.Id] {
			return nil, false
		}
		stored := byID[c.inboundIDs[ib.Id]]
		if stored == nil || stored.NodeID == nil || *stored.NodeID != nodeID || seenCentral[stored.Id] {
			return nil, false
		}
		if _, known := c.attached[ib.Id]; !known {
			return nil, false
		}
		if _, duplicate := aliases[ib.Tag]; duplicate {
			return nil, false
		}
		if stored.Port != ib.Port || stored.Protocol != ib.Protocol || strings.TrimSpace(stored.Listen) != strings.TrimSpace(ib.Listen) {
			return nil, false
		}
		aliases[ib.Tag], seenIDs[ib.Id], seenCentral[stored.Id] = stored, true, true
	}
	return aliases, true
}

// Database revisions validate both sides before a structural cache can be reused.
func masterSnapshotRevision(db *gorm.DB, nodeID int, snap *runtime.TrafficSnapshot, node *model.Node, selfKey string) (string, []snapshotInboundVersion, error) {
	if snap.StructuralRevision == "" {
		return "", nil, nil
	}
	var versions []snapshotInboundVersion
	if err := db.Table("inbounds").Select("id, snapshot_revision, up, down").Where("node_id = ?", nodeID).Order("id").Scan(&versions).Error; err != nil {
		return "", nil, err
	}
	var links []snapshotMembershipVersion
	if err := db.Table("client_inbounds ci").Select("ci.*, c.snapshot_revision").Joins("JOIN clients c ON c.id = ci.client_id").Joins("JOIN inbounds ib ON ib.id = ci.inbound_id").Where("ib.node_id = ?", nodeID).Order("ci.inbound_id, ci.client_id").Scan(&links).Error; err != nil {
		return "", nil, err
	}
	hash := sha256.New()
	enc := json.NewEncoder(hash)
	ids := make([]int, 0, len(snap.Inbounds))
	for _, ib := range snap.Inbounds {
		if ib != nil {
			ids = append(ids, ib.Id)
		}
	}
	structural := append([]snapshotInboundVersion(nil), versions...)
	for i := range structural {
		structural[i].Up = 0
		structural[i].Down = 0
	}
	aliases := append([]string(nil), snap.ManagedAliases...)
	slices.Sort(aliases)
	for _, v := range []any{snap.StructuralRevision, node.Guid, node.InboundSyncMode, node.InboundTags, selfKey, ids, aliases, structural, links} {
		if err := enc.Encode(v); err != nil {
			return "", nil, err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), versions, nil
}

func snapshotLifecycleRevision(rows []xray.ClientTraffic) string {
	hash := sha256.New()
	enc := json.NewEncoder(hash)
	for _, cs := range rows {
		cs.Up = 0
		cs.Down = 0
		cs.LastOnline = 0
		cs.LastSubFetch = 0
		_ = enc.Encode(cs)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func snapshotMembershipMatches(db *gorm.DB, nodeID int, snap *runtime.TrafficSnapshot, central map[string]*model.Inbound, attached map[int]map[string]struct{}) bool {
	var rows []struct {
		InboundID int
		Email     string
	}
	if err := db.Table("client_inbounds ci").Select("ci.inbound_id, c.email").Joins("JOIN clients c ON c.id = ci.client_id").Joins("JOIN inbounds ib ON ib.id = ci.inbound_id").Where("ib.node_id = ?", nodeID).Scan(&rows).Error; err != nil {
		return false
	}
	byInbound := make(map[int]map[string]struct{})
	for _, row := range rows {
		if byInbound[row.InboundID] == nil {
			byInbound[row.InboundID] = make(map[string]struct{})
		}
		byInbound[row.InboundID][strings.ToLower(row.Email)] = struct{}{}
	}
	for _, ib := range snap.Inbounds {
		if ib == nil {
			continue
		}
		c := central[ib.Tag]
		if c == nil {
			return false
		}
		want := attached[ib.Id]
		got := byInbound[c.Id]
		if len(want) != len(got) {
			return false
		}
		for email := range want {
			if _, ok := got[email]; !ok {
				return false
			}
		}
	}
	return true
}

func (s *InboundService) RetainNodeSnapshotCaches(nodes []*model.Node) error {
	active := make(map[int]bool, len(nodes))
	for _, node := range nodes {
		active[node.Id] = node.Enable && node.Status == "online"
	}
	return submitTrafficWrite(func() error {
		nodeSnapshotMergeMu.Lock()
		defer nodeSnapshotMergeMu.Unlock()
		for id := range s.snapshotMerges {
			if !active[id] {
				delete(s.snapshotMerges, id)
			}
		}
		return nil
	})
}
