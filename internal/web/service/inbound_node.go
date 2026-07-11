package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var reportedRemoteTagConflict sync.Map

// nodeBulkPushThreshold caps how many per-client RPCs a single operation will
// stream to a remote node. Above it, the panel marks the node dirty instead and
// lets one ReconcileNode push converge the whole inbound — far cheaper than M
// sequential round-trips. Small ops stay on the live per-client path.
const nodeBulkPushThreshold = 32

func (s *InboundService) runtimeFor(ib *model.Inbound) (runtime.Runtime, error) {
	mgr := runtime.GetManager()
	if mgr == nil {
		return nil, fmt.Errorf("runtime manager not initialised")
	}
	return mgr.RuntimeFor(ib.NodeID)
}

func (s *InboundService) nodePushPlan(ib *model.Inbound) (runtime.Runtime, bool, bool, error) {
	if ib.NodeID == nil {
		rt, err := s.runtimeFor(ib)
		if err != nil {
			return nil, false, false, nil
		}
		return rt, true, false, nil
	}
	nodeSvc := NodeService{}
	enabled, status, _, _, err := nodeSvc.NodeSyncState(*ib.NodeID)
	if err != nil {
		return nil, false, false, err
	}
	if !enabled || status == "offline" {
		return nil, false, true, nil
	}
	rt, err := s.runtimeFor(ib)
	if err != nil {
		return nil, false, true, nil
	}
	return rt, true, false, nil
}

func (s *InboundService) NodeIsPending(nodeID *int) bool {
	if nodeID == nil {
		return false
	}
	return (&NodeService{}).IsNodePending(*nodeID)
}

func (s *InboundService) AnyNodePending(inboundIds []int) bool {
	if len(inboundIds) == 0 {
		return false
	}
	nodeSvc := NodeService{}
	for _, id := range inboundIds {
		ib, err := s.GetInbound(id)
		if err != nil || ib.NodeID == nil {
			continue
		}
		if nodeSvc.IsNodePending(*ib.NodeID) {
			return true
		}
	}
	return false
}

// ReconcileNode pushes every inbound and sweeps undesired remote tags even when
// individual operations fail, returning the failures joined: one inbound the
// node rejects (e.g. a legacy protocol failing validation, #5685) must not
// stall the rest of the node's config — or, via syncOne, its traffic sync.
func (s *InboundService) ReconcileNode(ctx context.Context, rt *runtime.Remote, n *model.Node) error {
	if rt == nil || n == nil || n.Id <= 0 {
		return nil
	}
	nodeID := n.Id
	db := database.GetDB()
	var inbounds []*model.Inbound
	if err := db.Model(model.Inbound{}).Where("node_id = ?", nodeID).Find(&inbounds).Error; err != nil {
		return err
	}
	remoteTags, err := rt.ListRemoteTags(ctx)
	if err != nil {
		return err
	}
	remoteTagSet := make(map[string]struct{}, len(remoteTags))
	for _, tag := range remoteTags {
		remoteTagSet[tag] = struct{}{}
	}
	prefix := nodeTagPrefix(&nodeID)
	desiredTags := make(map[string]struct{}, len(inbounds)*2)
	var errs []error
	for _, ib := range inbounds {
		desiredTags[ib.Tag] = struct{}{}
		// existsOnNode: does the node already report this inbound under any of the
		// tag forms it may be stored as? If so, an unchanged push can be skipped.
		_, existsOnNode := remoteTagSet[ib.Tag]
		if prefix != "" {
			if stripped, found := strings.CutPrefix(ib.Tag, prefix); found {
				desiredTags[stripped] = struct{}{}
				if _, ok := remoteTagSet[stripped]; ok {
					existsOnNode = true
				}
			} else {
				desiredTags[prefix+ib.Tag] = struct{}{}
				if _, ok := remoteTagSet[prefix+ib.Tag]; ok {
					existsOnNode = true
				}
			}
		}
		if _, err := rt.ReconcileInbound(ctx, ib, existsOnNode); err != nil {
			errs = append(errs, fmt.Errorf("reconcile inbound %q: %w", ib.Tag, err))
		}
	}
	// Before the first clean sync adopts the node's inbounds, "absent locally"
	// means "not imported yet" — sweeping now would wipe the node at onboarding.
	if n.InboundsAdoptedAt == 0 {
		return errors.Join(errs...)
	}
	// In "selected" sync mode the panel only manages the selected tags: the
	// rest were never imported, so their absence from the local DB must not
	// delete them from the node. Only a selected tag missing locally (the
	// panel deleted it while the node was unreachable) may be swept.
	var selected map[string]struct{}
	if n.InboundSyncMode == "selected" {
		selected = make(map[string]struct{}, len(n.InboundTags))
		for _, tag := range n.InboundTags {
			selected[tag] = struct{}{}
		}
	}
	for _, tag := range remoteTags {
		if _, want := desiredTags[tag]; want {
			continue
		}
		if selected != nil {
			if _, managed := selected[tag]; !managed {
				continue
			}
		}
		if err := rt.DelInbound(ctx, &model.Inbound{Tag: tag}); err != nil {
			errs = append(errs, fmt.Errorf("reconcile delete %q: %w", tag, err))
		}
	}
	return errors.Join(errs...)
}

const resetGracePeriodMs int64 = 30000

// onlineGracePeriodMs must comfortably exceed the 5s traffic-poll interval —
// Xray's stats counters often report a zero delta for an active session across
// a single poll, so a 5s grace would still drop the client on the next tick.
// ~4 polls of slack keeps idle-but-connected clients visible without lingering
// long after a real disconnect.
const onlineGracePeriodMs int64 = 20000

type nodeTrafficCounter struct {
	Up   int64
	Down int64
}

func (s *InboundService) upsertNodeBaseline(tx *gorm.DB, nodeID int, email string, up, down int64) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "node_id"}, {Name: "email"}},
		DoUpdates: clause.AssignmentColumns([]string{"up", "down"}),
	}).Create(&model.NodeClientTraffic{NodeId: nodeID, Email: email, Up: up, Down: down}).Error
}

// mergeActivationExpiry reconciles a node-reported client expiry with the value
// already stored on the master. "Start after first connect" persists a negative
// duration that each node converts to an absolute deadline (now+duration) the
// first time the client connects there. The per-email client_traffics row is
// shared across every node, so a node that has not yet seen a first connection
// keeps reporting the negative duration — which must never reset a deadline
// another node already activated.
//
// A node may legitimately move an already-activated deadline forward (traffic
// reset / auto-renew extends it), so any positive node value is still adopted —
// only an un-activated (<= 0) value is rejected once an absolute deadline
// exists. Kept in lockstep with the SQL CASE in setRemoteTrafficLocked.
func mergeActivationExpiry(existing, node int64) int64 {
	if existing > 0 && node <= 0 {
		return existing
	}
	return node
}

// nodeClientRenewed reports a node-side auto-renew: an absolute deadline moved
// forward while the node's cumulative counter fell below the stored baseline.
func nodeClientRenewed(existing *xray.ClientTraffic, cs xray.ClientTraffic, canon, base nodeTrafficCounter) bool {
	if cs.Reset <= 0 || cs.ExpiryTime <= 0 || existing.ExpiryTime <= 0 {
		return false
	}
	if cs.ExpiryTime <= existing.ExpiryTime {
		return false
	}
	return canon.Up < base.Up || canon.Down < base.Down
}

// liftActivatedClientRecordExpiries copies a node-activated deadline from
// client_traffics onto client records still holding the negative duration (#5714).
func liftActivatedClientRecordExpiries(tx *gorm.DB) error {
	return tx.Exec(
		`UPDATE clients
		 SET expiry_time = (SELECT ct.expiry_time FROM client_traffics ct WHERE ct.email = clients.email AND ct.expiry_time > 0 LIMIT 1)
		 WHERE clients.expiry_time < 0
		   AND EXISTS (SELECT 1 FROM client_traffics ct WHERE ct.email = clients.email AND ct.expiry_time > 0)`,
	).Error
}

// SnapshotHasUnadoptedInbounds reports whether the snapshot carries a tag with
// no central row yet, i.e. the next merge would adopt a new inbound.
func (s *InboundService) SnapshotHasUnadoptedInbounds(nodeID int, snap *runtime.TrafficSnapshot) (bool, error) {
	if snap == nil || len(snap.Inbounds) == 0 {
		return false, nil
	}
	var tags []string
	if err := database.GetDB().Model(model.Inbound{}).
		Where("node_id = ?", nodeID).
		Pluck("tag", &tags).Error; err != nil {
		return false, err
	}
	prefix := nodeTagPrefix(&nodeID)
	known := make(map[string]struct{}, len(tags)*2)
	for _, tag := range tags {
		known[tag] = struct{}{}
		if prefix != "" {
			if stripped, found := strings.CutPrefix(tag, prefix); found {
				known[stripped] = struct{}{}
			} else {
				known[prefix+tag] = struct{}{}
			}
		}
	}
	for _, ib := range snap.Inbounds {
		if ib == nil {
			continue
		}
		if _, ok := known[ib.Tag]; !ok {
			return true, nil
		}
	}
	return false, nil
}

func (s *InboundService) SetRemoteTraffic(nodeID int, snap *runtime.TrafficSnapshot, dirty bool) (bool, error) {
	var structuralChange bool
	err := submitTrafficWrite(func() error {
		var inner error
		structuralChange, inner = s.setRemoteTrafficLocked(nodeID, snap, dirty)
		return inner
	})
	return structuralChange, err
}

// GetNodeInboundTrafficTotals returns the current cumulative up/down for every
// node-hosted inbound, keyed by tag. The node sync diffs successive snapshots of
// this to derive per-inbound speed for the dashboard — node inbounds have no
// local Xray poll to produce live deltas the way local inbounds do.
func (s *InboundService) GetNodeInboundTrafficTotals() (map[string][2]int64, error) {
	var rows []struct {
		Tag  string
		Up   int64
		Down int64
	}
	if err := database.GetDB().Table("inbounds").
		Select("tag, up, down").
		Where("node_id IS NOT NULL").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string][2]int64, len(rows))
	for _, r := range rows {
		out[r.Tag] = [2]int64{r.Up, r.Down}
	}
	return out, nil
}

func adoptedWireChanged(c, snapIb *model.Inbound, adoptedSettings string) bool {
	return c.Settings != adoptedSettings ||
		c.Enable != snapIb.Enable ||
		c.Remark != snapIb.Remark ||
		c.SubSortIndex != normalizeSubSortIndex(snapIb.SubSortIndex) ||
		c.Listen != snapIb.Listen ||
		c.Port != snapIb.Port ||
		c.Protocol != snapIb.Protocol ||
		c.Total != snapIb.Total ||
		c.ExpiryTime != snapIb.ExpiryTime ||
		c.StreamSettings != snapIb.StreamSettings ||
		c.Sniffing != snapIb.Sniffing ||
		c.TrafficReset != snapIb.TrafficReset
}

// adoptedWireInbound is the central inbound as it reads after adopting the
// node-reported wire fields — the payload the reconcile fingerprint must track.
func adoptedWireInbound(c, snapIb *model.Inbound, adoptedSettings string) *model.Inbound {
	a := *c
	a.Enable = snapIb.Enable
	a.Remark = snapIb.Remark
	a.SubSortIndex = normalizeSubSortIndex(snapIb.SubSortIndex)
	a.Listen = snapIb.Listen
	a.Port = snapIb.Port
	a.Protocol = snapIb.Protocol
	a.Total = snapIb.Total
	a.ExpiryTime = snapIb.ExpiryTime
	a.Settings = adoptedSettings
	a.StreamSettings = snapIb.StreamSettings
	a.Sniffing = snapIb.Sniffing
	a.TrafficReset = snapIb.TrafficReset
	return &a
}

func (s *InboundService) setRemoteTrafficLocked(nodeID int, snap *runtime.TrafficSnapshot, dirty bool) (bool, error) {
	if snap == nil || nodeID <= 0 {
		return false, nil
	}
	db := database.GetDB()
	now := time.Now().UnixMilli()

	// originGuidFor attributes a synced inbound to the panel that physically
	// hosts it. A node's OWN inbounds report either an empty origin or — on
	// builds that set it locally — the node's own panelGuid; both resolve to
	// selfKey, which is the node's panelGuid unless that GUID is ambiguous
	// (shared with another node or the master, i.e. a cloned server), in which
	// case it falls back to the node-unique id so #4983 attribution doesn't
	// collapse two physical nodes into one bucket. Only a DIFFERENT, non-empty
	// origin (an inbound the node forwards from its own sub-node) is kept as-is,
	// so a chained Node1->Node2->Node3 still attributes Node3's inbounds to Node3.
	var nodeRow model.Node
	db.Select("guid").Where("id = ?", nodeID).First(&nodeRow)
	selfKey := effectiveNodeKey(&model.Node{Id: nodeID, Guid: nodeRow.Guid})
	guidShared := nodeRow.Guid != "" && selfKey != nodeRow.Guid
	originGuidFor := func(snapIb *model.Inbound) string {
		if snapIb.OriginNodeGuid != "" && snapIb.OriginNodeGuid != nodeRow.Guid {
			return snapIb.OriginNodeGuid
		}
		return selfKey
	}

	var central []model.Inbound
	if err := db.Model(model.Inbound{}).
		Where("node_id = ?", nodeID).
		Find(&central).Error; err != nil {
		return false, err
	}
	// Index under the stored tag and its prefix-flipped form so a snap matches
	// whether the n<id>- prefix lives on the node side, the central side, or
	// neither — a mismatch must never spawn a duplicate central inbound.
	tagToCentral := make(map[string]*model.Inbound, len(central)*2)
	prefix := nodeTagPrefix(&nodeID)
	for i := range central {
		tagToCentral[central[i].Tag] = &central[i]
		if prefix != "" {
			if stripped, found := strings.CutPrefix(central[i].Tag, prefix); found {
				tagToCentral[stripped] = &central[i]
			} else {
				tagToCentral[prefix+central[i].Tag] = &central[i]
			}
		}
	}

	var centralClientStats []xray.ClientTraffic
	if len(central) > 0 {
		ids := make([]int, 0, len(central))
		for i := range central {
			ids = append(ids, central[i].Id)
		}
		if err := db.Model(xray.ClientTraffic{}).
			Where("inbound_id IN ?", ids).
			Find(&centralClientStats).Error; err != nil {
			return false, err
		}
	}
	type csKey struct {
		inboundID int
		email     string
	}
	centralCS := make(map[csKey]*xray.ClientTraffic, len(centralClientStats))
	centralCSByEmail := make(map[string]*xray.ClientTraffic, len(centralClientStats))
	for i := range centralClientStats {
		centralCS[csKey{centralClientStats[i].InboundId, centralClientStats[i].Email}] = &centralClientStats[i]
		centralCSByEmail[centralClientStats[i].Email] = &centralClientStats[i]
	}

	nodeBaselines := make(map[string]nodeTrafficCounter)
	var baselineRows []model.NodeClientTraffic
	if err := db.Model(&model.NodeClientTraffic{}).
		Where("node_id = ?", nodeID).
		Find(&baselineRows).Error; err != nil {
		return false, err
	}
	for i := range baselineRows {
		nodeBaselines[baselineRows[i].Email] = nodeTrafficCounter{Up: baselineRows[i].Up, Down: baselineRows[i].Down}
	}

	var defaultUserId int
	if len(central) > 0 {
		defaultUserId = central[0].UserId
	} else {
		var u model.User
		if err := db.Model(model.User{}).Order("id asc").First(&u).Error; err == nil {
			defaultUserId = u.Id
		} else {
			defaultUserId = 1
		}
	}

	// Union of every email the snapshot still reports, across all inbounds.
	// The (node, email) baseline rows are keyed per node, not per inbound, so
	// the sweeps below must only drop one when the email left the node
	// entirely — an email whose stats moved to (or always lived under) a
	// sibling inbound still needs its baseline for the sibling's delta
	// computation (#5202).
	//
	// Xray counts traffic per email, not per inbound, so a multi-attached
	// client's shared counter is copied onto every inbound it's on. Fold each
	// email to its per-field max (nodeEmailTotals) so divergent copies can't make
	// the reset clamp re-add a lower sibling as fresh traffic (#5274).
	snapEmailsAll := make(map[string]struct{})
	nodeEmailTotals := make(map[string]nodeTrafficCounter)
	for _, snapIb := range snap.Inbounds {
		if snapIb == nil {
			continue
		}
		for i := range snapIb.ClientStats {
			email := snapIb.ClientStats[i].Email
			snapEmailsAll[email] = struct{}{}
			cur := nodeEmailTotals[email]
			if snapIb.ClientStats[i].Up > cur.Up {
				cur.Up = snapIb.ClientStats[i].Up
			}
			if snapIb.ClientStats[i].Down > cur.Down {
				cur.Down = snapIb.ClientStats[i].Down
			}
			nodeEmailTotals[email] = cur
		}
	}

	// Membership set for the rowExists checks below. Only the snapshot's emails
	// are ever probed, so scope the lookup to those instead of plucking the whole
	// client_traffics table (50k+ rows) on every node poll.
	existingEmails := make(map[string]struct{}, len(snapEmailsAll))
	if len(snapEmailsAll) > 0 {
		snapEmailList := make([]string, 0, len(snapEmailsAll))
		for email := range snapEmailsAll {
			snapEmailList = append(snapEmailList, email)
		}
		for _, batch := range chunkStrings(snapEmailList, sqliteMaxVars) {
			var found []string
			if err := db.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Pluck("email", &found).Error; err != nil {
				return false, err
			}
			for _, e := range found {
				existingEmails[e] = struct{}{}
			}
		}
	}

	tx := db.Begin()
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	structuralChange := false

	var adoptedInbounds []*model.Inbound

	newInboundIDs := make(map[int]struct{})

	snapTags := make(map[string]struct{}, len(snap.Inbounds))
	for _, snapIb := range snap.Inbounds {
		if snapIb == nil {
			continue
		}
		snapTags[snapIb.Tag] = struct{}{}
		// Record the prefix-flipped form too so the orphan sweep below keeps a
		// central inbound whether its tag carries the n<id>- prefix or not.
		if prefix != "" {
			if stripped, found := strings.CutPrefix(snapIb.Tag, prefix); found {
				snapTags[stripped] = struct{}{}
			} else {
				snapTags[prefix+snapIb.Tag] = struct{}{}
			}
		}

		c, ok := tagToCentral[snapIb.Tag]
		if !ok {
			if dirty {
				continue
			}
			// Try snap.Tag first; on collision fall back to the n<id>-
			// prefixed form so local+node can both own the same port.
			pickFreeTag := func() (string, error) {
				candidates := []string{snapIb.Tag}
				if prefix != "" && !strings.HasPrefix(snapIb.Tag, prefix) {
					candidates = append(candidates, prefix+snapIb.Tag)
				}
				for _, t := range candidates {
					var owner model.Inbound
					err := tx.Where("tag = ?", t).First(&owner).Error
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return t, nil
					}
					if err != nil {
						return "", err
					}
				}
				return "", nil
			}
			chosenTag, err := pickFreeTag()
			if err != nil {
				logger.Warningf("setRemoteTraffic: check tag %q failed: %v", snapIb.Tag, err)
				continue
			}
			if chosenTag == "" {
				key := fmt.Sprintf("%d:%s", nodeID, snapIb.Tag)
				if _, seen := reportedRemoteTagConflict.LoadOrStore(key, struct{}{}); !seen {
					logger.Warningf(
						"setRemoteTraffic: tag %q from node %d collides with an existing inbound even after the n%d- prefix — skipping (rename one side to remove the duplicate)",
						snapIb.Tag, nodeID, nodeID,
					)
				}
				continue
			}
			reportedRemoteTagConflict.Delete(fmt.Sprintf("%d:%s", nodeID, snapIb.Tag))
			newIb := model.Inbound{
				UserId:               defaultUserId,
				NodeID:               &nodeID,
				OriginNodeGuid:       originGuidFor(snapIb),
				Tag:                  chosenTag,
				Listen:               snapIb.Listen,
				Port:                 snapIb.Port,
				Protocol:             snapIb.Protocol,
				Settings:             snapIb.Settings,
				StreamSettings:       snapIb.StreamSettings,
				Sniffing:             snapIb.Sniffing,
				TrafficReset:         snapIb.TrafficReset,
				LastTrafficResetTime: snapIb.LastTrafficResetTime,
				Enable:               snapIb.Enable,
				Remark:               snapIb.Remark,
				SubSortIndex:         normalizeSubSortIndex(snapIb.SubSortIndex),
				Total:                snapIb.Total,
				ExpiryTime:           snapIb.ExpiryTime,
				Up:                   snapIb.Up,
				Down:                 snapIb.Down,
				ShareAddrStrategy:    "node",
			}
			if err := tx.Create(&newIb).Error; err != nil {
				logger.Warningf("setRemoteTraffic: create central inbound for tag %q failed: %v", snapIb.Tag, err)
				continue
			}
			tagToCentral[snapIb.Tag] = &newIb
			if newIb.Tag != snapIb.Tag {
				tagToCentral[newIb.Tag] = &newIb
			}
			if rows := adoptedHostRows(snap.HostGroups, snapIb.Id, newIb.Id); len(rows) > 0 {
				if err := tx.Create(&rows).Error; err != nil {
					logger.Warningf("setRemoteTraffic: adopt host rows for tag %q failed: %v", newIb.Tag, err)
				}
			}
			newInboundIDs[newIb.Id] = struct{}{}
			structuralChange = true
			continue
		}

		inGrace := c.LastTrafficResetTime > 0 && now-c.LastTrafficResetTime < resetGracePeriodMs

		// Adopting the node's settings verbatim would re-add a client the master
		// deleted moments ago if this snapshot was fetched before the deletion
		// push landed — filter just-deleted emails out while their tombstone lives.
		adoptedSettings := snapIb.Settings
		if stripped, changed := stripTombstonedClients(adoptedSettings); changed {
			adoptedSettings = stripped
		}
		if deduped, changed := dedupeSettingsClients(adoptedSettings); changed {
			adoptedSettings = deduped
		}

		updates := map[string]any{}
		if !dirty {
			updates["enable"] = snapIb.Enable
			updates["remark"] = snapIb.Remark
			updates["sub_sort_index"] = normalizeSubSortIndex(snapIb.SubSortIndex)
			updates["listen"] = snapIb.Listen
			updates["port"] = snapIb.Port
			updates["protocol"] = snapIb.Protocol
			updates["total"] = snapIb.Total
			updates["expiry_time"] = snapIb.ExpiryTime
			updates["settings"] = adoptedSettings
			updates["stream_settings"] = snapIb.StreamSettings
			updates["sniffing"] = snapIb.Sniffing
			updates["traffic_reset"] = snapIb.TrafficReset
			updates["last_traffic_reset_time"] = snapIb.LastTrafficResetTime
			if adoptedWireChanged(c, snapIb, adoptedSettings) {
				adoptedInbounds = append(adoptedInbounds, adoptedWireInbound(c, snapIb, adoptedSettings))
			}
		}
		if !inGrace || (snapIb.Up+snapIb.Down) <= (c.Up+c.Down) {
			updates["up"] = snapIb.Up
			updates["down"] = snapIb.Down
		}
		// Physical-home attribution is independent of config-dirty state, so
		// keep it current even while the node has pending offline edits. Writes
		// once to backfill an existing row, then stays equal (#4983).
		if og := originGuidFor(snapIb); c.OriginNodeGuid != og {
			updates["origin_node_guid"] = og
		}

		if !dirty && (c.Settings != adoptedSettings ||
			c.Remark != snapIb.Remark ||
			c.Listen != snapIb.Listen ||
			c.Port != snapIb.Port ||
			c.Total != snapIb.Total ||
			c.ExpiryTime != snapIb.ExpiryTime ||
			c.Enable != snapIb.Enable) {
			structuralChange = true
		}

		if len(updates) > 0 {
			if err := tx.Model(model.Inbound{}).
				Where("id = ?", c.Id).
				Updates(updates).Error; err != nil {
				return false, err
			}
		}
	}

	for _, c := range central {
		if dirty {
			continue
		}
		if len(snapTags) == 0 {
			// A node mid-restart or with a transient DB error can return an empty
			// inbound list with success=true. Treat "zero inbounds reported" as
			// "nothing to say", not "delete all my inbounds" — otherwise a blip
			// wipes the node's central inbounds and every client on them (and
			// resets traffic history on re-create). A real per-inbound deletion
			// still sweeps, because the node keeps reporting its other inbounds.
			continue
		}
		if _, kept := snapTags[c.Tag]; kept {
			continue
		}
		var goneEmails []string
		if err := tx.Model(xray.ClientTraffic{}).
			Where("inbound_id = ?", c.Id).
			Pluck("email", &goneEmails).Error; err != nil {
			return false, err
		}
		if len(goneEmails) > 0 {
			// Baselines are per (node, email), not per inbound: keep them for
			// emails the snapshot still reports under a sibling inbound (#5202).
			baselineGone := make([]string, 0, len(goneEmails))
			for _, e := range goneEmails {
				if _, still := snapEmailsAll[e]; !still {
					baselineGone = append(baselineGone, e)
				}
			}
			// Chunk to avoid SQLite bind var limit when a node has many clients
			// removed (e.g. after API bulk delete or structural change on node inbound).
			for _, batch := range chunkStrings(baselineGone, sqliteMaxVars) {
				if err := tx.Where("node_id = ? AND email IN ?", nodeID, batch).
					Delete(&model.NodeClientTraffic{}).Error; err != nil {
					return false, err
				}
			}
			// The per-email row is the shared accumulator across every inbound
			// (and node) the email is attached to. Only drop it when this was the
			// email's last inbound — wiping it while a sibling still feeds it
			// loses the summed history, and the next node sync would re-seed the
			// row with that node's counter alone.
			sharedEmails, sErr := s.emailsUsedByOtherInbounds(goneEmails, c.Id)
			if sErr != nil {
				return false, sErr
			}
			delEmails := make([]string, 0, len(goneEmails))
			for _, e := range goneEmails {
				if !sharedEmails[strings.ToLower(strings.TrimSpace(e))] {
					delEmails = append(delEmails, e)
				}
			}
			for _, batch := range chunkStrings(delEmails, sqliteMaxVars) {
				if err := tx.Where("inbound_id = ? AND email IN ?", c.Id, batch).
					Delete(&xray.ClientTraffic{}).Error; err != nil {
					return false, err
				}
			}
		}
		if err := s.clientService.DetachInbound(tx, c.Id); err != nil {
			return false, err
		}
		if err := tx.Where("id = ?", c.Id).
			Delete(&model.Inbound{}).Error; err != nil {
			return false, err
		}
		delete(tagToCentral, c.Tag)
		structuralChange = true
	}

	for _, snapIb := range snap.Inbounds {
		if snapIb == nil {
			continue
		}
		c, ok := tagToCentral[snapIb.Tag]
		if !ok {
			continue
		}
		snapEmails := make(map[string]struct{}, len(snapIb.ClientStats))
		for _, cs := range snapIb.ClientStats {
			snapEmails[cs.Email] = struct{}{}

			// Node-wide total, not this inbound's possibly-stale copy (#5274).
			canon := nodeEmailTotals[cs.Email]

			base, seen := nodeBaselines[cs.Email]
			var deltaUp, deltaDown int64
			if seen {
				if deltaUp = canon.Up - base.Up; deltaUp < 0 {
					deltaUp = 0
				}
				if deltaDown = canon.Down - base.Down; deltaDown < 0 {
					deltaDown = 0
				}
			}

			if _, rowExists := existingEmails[cs.Email]; !rowExists {
				if dirty {
					continue
				}
				_, isNewInbound := newInboundIDs[c.Id]
				// On a known inbound a missing row plus a live tombstone means the
				// master just deleted this client and the snapshot predates the
				// deletion push — recreating the row (at zero) would resurrect the
				// client. A freshly adopted inbound still gets its row (seeded at
				// zero) so adoption semantics stay intact.
				if !isNewInbound && isClientEmailTombstoned(cs.Email) {
					continue
				}
				var seedUp, seedDown int64
				if isNewInbound && !isClientEmailTombstoned(cs.Email) {
					seedUp, seedDown = canon.Up, canon.Down
				}
				row := &xray.ClientTraffic{
					InboundId:  c.Id,
					Email:      cs.Email,
					Enable:     cs.Enable,
					Total:      cs.Total,
					ExpiryTime: cs.ExpiryTime,
					Reset:      cs.Reset,
					Up:         seedUp,
					Down:       seedDown,
					LastOnline: cs.LastOnline,
				}
				if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "email"}}, DoNothing: true}).
					Create(row).Error; err != nil {
					return false, err
				}
				centralCS[csKey{c.Id, cs.Email}] = row
				centralCSByEmail[cs.Email] = row
				existingEmails[cs.Email] = struct{}{}
				structuralChange = true
				if err := s.upsertNodeBaseline(tx, nodeID, cs.Email, canon.Up, canon.Down); err != nil {
					return false, err
				}
				nodeBaselines[cs.Email] = nodeTrafficCounter{Up: canon.Up, Down: canon.Down}
				continue
			}

			existing := centralCSByEmail[cs.Email]
			if existing != nil &&
				(existing.Enable != cs.Enable ||
					existing.Total != cs.Total ||
					existing.ExpiryTime != mergeActivationExpiry(existing.ExpiryTime, cs.ExpiryTime) ||
					existing.Reset != cs.Reset) {
				structuralChange = true
			}

			if seen && existing != nil && nodeClientRenewed(existing, cs, canon, base) {
				// A renewal starts a fresh quota window: adopt the node's counters
				// and enable state, drop stale pushes (mirrors autoRenewClients).
				if err := tx.Exec(
					fmt.Sprintf(
						`UPDATE client_traffics
						 SET up = ?, down = ?, enable = ?, total = ?,
						     expiry_time = ?, reset = ?, last_online = %s
						 WHERE email = ?`,
						database.GreatestExpr("last_online", "?"),
					),
					canon.Up, canon.Down, cs.Enable, cs.Total,
					cs.ExpiryTime, cs.Reset,
					cs.LastOnline, cs.Email,
				).Error; err != nil {
					return false, err
				}
				if err := clearGlobalTraffic(tx, cs.Email); err != nil {
					return false, err
				}
			} else {
				enableExpr := database.ClientTrafficEnableMergeExpr()
				// expiry_time merge mirrors mergeActivationExpiry: a node that has not
				// yet seen the client's first connection keeps reporting the negative
				// "start after first connect" duration, which must never reset the
				// absolute deadline another node already activated. A positive node
				// value is still adopted (e.g. auto-renew moves the deadline forward).
				// CAST(? AS BIGINT): in the `<= 0` comparison Postgres would otherwise
				// infer int4 from the literal and overflow on real expiry values.
				if err := tx.Exec(
					fmt.Sprintf(
						`UPDATE client_traffics
						 SET up = %s, down = %s, enable = %s, total = ?,
						     expiry_time = CASE WHEN expiry_time > 0 AND CAST(? AS BIGINT) <= 0 THEN expiry_time ELSE CAST(? AS BIGINT) END,
						     reset = ?, last_online = %s
						 WHERE email = ?`,
						database.ClampedAddExpr("up"),
						database.ClampedAddExpr("down"),
						enableExpr,
						database.GreatestExpr("last_online", "?"),
					),
					deltaUp, deltaDown, cs.Enable, cs.Total,
					cs.ExpiryTime, cs.ExpiryTime, cs.Reset,
					cs.LastOnline, cs.Email,
				).Error; err != nil {
					return false, err
				}
			}
			if err := s.upsertNodeBaseline(tx, nodeID, cs.Email, canon.Up, canon.Down); err != nil {
				return false, err
			}
			nodeBaselines[cs.Email] = nodeTrafficCounter{Up: canon.Up, Down: canon.Down}
		}

		for k, existing := range centralCS {
			if dirty {
				continue
			}
			if k.inboundID != c.Id {
				continue
			}
			if _, kept := snapEmails[k.email]; kept {
				continue
			}
			// Gone from this inbound's stats but still reported by the node under
			// a sibling inbound: both the shared accumulator row and the (node,
			// email) baseline must survive, or the sibling's next delta would
			// compute against nothing and freeze the counter (#5202).
			if _, still := snapEmailsAll[k.email]; still {
				continue
			}
			if err := tx.Where("node_id = ? AND email = ?", nodeID, existing.Email).
				Delete(&model.NodeClientTraffic{}).Error; err != nil {
				return false, err
			}
			// Same shared-accumulator rule as the inbound-removal sweep above:
			// keep the row while another inbound still references the email.
			stillUsed, uErr := s.emailUsedByOtherInbounds(existing.Email, c.Id)
			if uErr != nil {
				return false, uErr
			}
			if !stillUsed {
				if err := tx.Where("inbound_id = ? AND email = ?", c.Id, existing.Email).
					Delete(&xray.ClientTraffic{}).Error; err != nil {
					return false, err
				}
			}
			structuralChange = true
		}
	}

	type oldSet struct {
		inboundID int
		emails    map[string]struct{}
	}
	var perInboundOld []oldSet
	for _, snapIb := range snap.Inbounds {
		if snapIb == nil {
			continue
		}
		c, ok := tagToCentral[snapIb.Tag]
		if !ok {
			continue
		}
		if dirty {
			continue
		}
		var oldEmailsRows []string
		if err := tx.Table("clients").
			Joins("JOIN client_inbounds ON client_inbounds.client_id = clients.id").
			Where("client_inbounds.inbound_id = ?", c.Id).
			Pluck("email", &oldEmailsRows).Error; err == nil {
			oldEmails := make(map[string]struct{}, len(oldEmailsRows))
			for _, e := range oldEmailsRows {
				if e != "" {
					oldEmails[e] = struct{}{}
				}
			}
			perInboundOld = append(perInboundOld, oldSet{inboundID: c.Id, emails: oldEmails})
		}

		clients, gcErr := s.GetClients(snapIb)
		if gcErr != nil {
			logger.Warningf("setRemoteTraffic: parse clients for tag %q failed: %v", snapIb.Tag, gcErr)
			continue
		}
		csEnableByEmail := make(map[string]bool, len(snapIb.ClientStats))
		for _, cs := range snapIb.ClientStats {
			csEnableByEmail[cs.Email] = cs.Enable
		}
		filtered := clients[:0]
		for i := range clients {
			if isClientEmailTombstoned(clients[i].Email) {
				continue
			}
			if cse, hit := csEnableByEmail[clients[i].Email]; hit && !cse {
				clients[i].Enable = false
			}
			filtered = append(filtered, clients[i])
		}
		localEmails := make([]string, 0, len(filtered))
		for i := range filtered {
			if filtered[i].Email != "" {
				localEmails = append(localEmails, filtered[i].Email)
			}
		}
		if len(localEmails) > 0 {
			var localMeta []struct {
				Email   string
				Comment string `gorm:"column:comment"`
			}
			if err := tx.Table("clients").
				Select("email, comment").
				Where("email IN ?", localEmails).
				Find(&localMeta).Error; err == nil {
				commentByEmail := make(map[string]string, len(localMeta))
				for _, m := range localMeta {
					commentByEmail[m.Email] = m.Comment
				}
				for i := range filtered {
					if cmt, ok := commentByEmail[filtered[i].Email]; ok {
						filtered[i].Comment = cmt
					}
				}
			}
		}
		if err := s.clientService.SyncInbound(tx, c.Id, filtered); err != nil {
			logger.Warningf("setRemoteTraffic: sync clients for tag %q failed: %v", snapIb.Tag, err)
		}
	}

	for _, old := range perInboundOld {
		var stillAttached []string
		if err := tx.Table("clients").
			Joins("JOIN client_inbounds ON client_inbounds.client_id = clients.id").
			Where("client_inbounds.inbound_id = ?", old.inboundID).
			Pluck("email", &stillAttached).Error; err != nil {
			continue
		}
		stillSet := make(map[string]struct{}, len(stillAttached))
		for _, e := range stillAttached {
			stillSet[e] = struct{}{}
		}
		for email := range old.emails {
			if _, kept := stillSet[email]; kept {
				continue
			}
			var attachmentCount int64
			if err := tx.Table("client_inbounds").
				Joins("JOIN clients ON clients.id = client_inbounds.client_id").
				Where("clients.email = ?", email).
				Count(&attachmentCount).Error; err != nil {
				continue
			}
			if attachmentCount > 0 {
				continue
			}
			if err := tx.Where("email = ?", email).Delete(&model.ClientRecord{}).Error; err != nil {
				logger.Warningf("setRemoteTraffic: delete ClientRecord %q failed: %v", email, err)
			}
			if err := tx.Where("email = ?", email).Delete(&xray.ClientTraffic{}).Error; err != nil {
				logger.Warningf("setRemoteTraffic: delete ClientTraffic %q failed: %v", email, err)
			}
			if err := tx.Where("email = ?", email).Delete(&model.NodeClientTraffic{}).Error; err != nil {
				logger.Warningf("setRemoteTraffic: delete NodeClientTraffic %q failed: %v", email, err)
			}
			structuralChange = true
		}
	}

	if err := liftActivatedClientRecordExpiries(tx); err != nil {
		logger.Warning("setRemoteTraffic: lift activated expiries failed:", err)
	}

	if err := tx.Commit().Error; err != nil {
		return false, err
	}
	committed = true

	if len(adoptedInbounds) > 0 {
		if mgr := runtime.GetManager(); mgr != nil {
			if rt, rtErr := mgr.RuntimeFor(&nodeID); rtErr == nil {
				if rem, ok := rt.(*runtime.Remote); ok {
					for _, ib := range adoptedInbounds {
						rem.RecordAdoptedInbound(ib)
					}
				}
			}
		}
	}

	if p != nil {
		tree := snap.OnlineTree
		switch {
		case len(tree) == 0 && len(snap.OnlineEmails) > 0:
			// Old-build node (no GUID tree): key its flat online list under its
			// own effective identity so attribution still works for that branch.
			tree = map[string][]string{selfKey: snap.OnlineEmails}
		case guidShared && len(tree) > 0:
			// Newer cloned node: its own clients arrive keyed under the shared
			// panelGuid. Remap just that entry to the node-unique key so the
			// clones don't merge; descendant subtrees keep their distinct GUIDs.
			if _, ok := tree[nodeRow.Guid]; ok {
				remapped := make(map[string][]string, len(tree))
				for g, emails := range tree {
					if g == nodeRow.Guid {
						g = selfKey
					}
					remapped[g] = emails
				}
				tree = remapped
			}
		}
		p.SetNodeOnlineTree(nodeID, tree)
	}

	return structuralChange, nil
}

func (s *InboundService) GetOnlineClients() []string {
	if p == nil {
		return []string{}
	}
	return p.GetOnlineClients()
}

// GetOnlineClientsByGuid returns online emails keyed by the panelGuid of the
// node that physically hosts each set: this panel's own clients under its own
// GUID, plus every node in the tree under its GUID (#4983). Replaces the old
// node-id keying so a client three hops down is attributed to its real node,
// not the intermediate one it was synced through.
func (s *InboundService) GetOnlineClientsByGuid() map[string][]string {
	if p == nil {
		return map[string][]string{}
	}
	out := p.GetMergedNodeTrees()
	if local := p.GetLocalOnlineClients(); len(local) > 0 {
		if guid := s.panelGuid(); guid != "" {
			out[guid] = mergeEmails(out[guid], local)
		}
	}
	return out
}

// GetActiveInboundsByGuid returns the inbound tags that carried traffic within
// the grace window for THIS panel, under its own GUID. Remote nodes don't
// report per-inbound activity, so a GUID missing from the map means "don't
// gate" for that node's inbounds.
func (s *InboundService) GetActiveInboundsByGuid() map[string][]string {
	if p == nil {
		return map[string][]string{}
	}
	active := p.GetLocalActiveInbounds()
	if len(active) == 0 {
		return map[string][]string{}
	}
	guid := s.panelGuid()
	if guid == "" {
		return map[string][]string{}
	}
	return map[string][]string{guid: active}
}

func (s *InboundService) SetNodeOnlineTree(nodeID int, tree map[string][]string) {
	if p != nil {
		p.SetNodeOnlineTree(nodeID, tree)
	}
}

func (s *InboundService) ClearNodeOnlineClients(nodeID int) {
	if p != nil {
		p.ClearNodeOnlineClients(nodeID)
	}
}

// panelGuid returns this panel's stable self-identifier, used to key the local
// panel's own clients in the per-node online maps (#4983).
func (s *InboundService) panelGuid() string {
	guid, _ := (&SettingService{}).GetPanelGuid()
	return guid
}

// synthNodeGuid is the stable per-node fallback identity for a directly-attached
// node whose panel hasn't reported a panelGuid yet (old build). Node ids are
// master-local, so this only composes for direct nodes — exactly the pre-#4983
// flat-topology case where an old-build node appears.
func synthNodeGuid(nodeID int) string {
	return fmt.Sprintf("node:%d", nodeID)
}

// mergeEmails returns the deduped union of two email slices.
func mergeEmails(a, b []string) []string {
	if len(a) == 0 {
		return b
	}
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, e := range a {
		if _, ok := seen[e]; !ok {
			seen[e] = struct{}{}
			out = append(out, e)
		}
	}
	for _, e := range b {
		if _, ok := seen[e]; !ok {
			seen[e] = struct{}{}
			out = append(out, e)
		}
	}
	return out
}

func (s *InboundService) GetClientsLastOnline() (map[string]int64, error) {
	db := database.GetDB()
	var rows []xray.ClientTraffic
	err := db.Model(&xray.ClientTraffic{}).Select("email, last_online").Find(&rows).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, r := range rows {
		result[r.Email] = r.LastOnline
	}
	return result, nil
}

// RefreshLocalOnlineClients folds the emails and inbound tags active on this
// panel's own xray this poll into the local online/active sets, applying the
// grace window and pruning stale entries. Pass nil to only prune. See
// xray.Process for why the local sets are kept separate from the shared
// last_online column.
func (s *InboundService) RefreshLocalOnlineClients(activeEmails, activeInboundTags []string) {
	if p != nil {
		p.RefreshLocalOnline(activeEmails, activeInboundTags, time.Now().UnixMilli(), onlineGracePeriodMs)
	}
}

func (s *InboundService) FilterAndSortClientEmails(emails []string) ([]string, []string, error) {
	db := database.GetDB()

	// Step 1: Get ClientTraffic records for emails in the input list.
	// Chunked to stay under SQLite's bind-variable limit on huge inputs.
	uniqEmails := uniqueNonEmptyStrings(emails)
	clients := make([]xray.ClientTraffic, 0, len(uniqEmails))
	for _, batch := range chunkStrings(uniqEmails, sqliteMaxVars) {
		var page []xray.ClientTraffic
		if err := db.Where("email IN ?", batch).Find(&page).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, err
		}
		clients = append(clients, page...)
	}

	// Step 2: Sort clients by (Up + Down) descending
	sort.Slice(clients, func(i, j int) bool {
		return (clients[i].Up + clients[i].Down) > (clients[j].Up + clients[j].Down)
	})

	// Step 3: Extract sorted valid emails and track found ones
	validEmails := make([]string, 0, len(clients))
	found := make(map[string]bool)
	for _, client := range clients {
		validEmails = append(validEmails, client.Email)
		found[client.Email] = true
	}

	// Step 4: Identify emails that were not found in the database
	extraEmails := make([]string, 0)
	for _, email := range emails {
		if !found[email] {
			extraEmails = append(extraEmails, email)
		}
	}

	return validEmails, extraEmails, nil
}
