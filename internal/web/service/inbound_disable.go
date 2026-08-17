package service

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

func (s *InboundService) disableInvalidInbounds(tx *gorm.DB, mutationBatch *trafficMutationBatch) (bool, int64, error) {
	now := time.Now().Unix() * 1000
	var inbounds []model.Inbound
	if err := tx.Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ? and node_id IS NULL", now, true).
		Find(&inbounds).Error; err != nil {
		return false, 0, err
	}
	for i := range inbounds {
		mutationBatch.localPlans = append(mutationBatch.localPlans, trafficLocalApplyPlan{
			action: trafficDisableInbound, inbound: inbounds[i],
		})
	}

	result := tx.Model(model.Inbound{}).
		Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ? and node_id IS NULL", now, true).
		Update("enable", false)
	err := result.Error
	count := result.RowsAffected
	return false, count, err
}

const globalTrafficFreshWindow = 24 * time.Hour

func globalTrafficFreshSince() int64 {
	return time.Now().Add(-globalTrafficFreshWindow).UnixMilli()
}

// depletedClientsCond matches clients that exhausted their quota or expired.
// Besides the local counters it also trips on the cross-panel usage a master
// pushed into client_global_traffics — that's what lets a node cut a client
// whose combined usage exceeds the quota even though the local share doesn't.
// Only rows a master refreshed recently count (placeholders: now, freshSince).
const depletedClientsCond = `((total > 0 AND up + down >= total)
	OR (expiry_time > 0 AND expiry_time <= ?)
	OR (total > 0 AND EXISTS (
		SELECT 1 FROM client_global_traffics g
		WHERE g.email = client_traffics.email
			AND g.updated_at >= ?
			AND g.up + g.down >= client_traffics.total
	)))`

// depletedClientsCondLocal is depletedClientsCond without the cross-panel
// client_global_traffics check. The EXISTS branch is a correlated subquery that
// turns every traffic poll into a full client_traffics scan; on a panel no
// master pushes to (the common case) client_global_traffics is empty, so the
// branch can never match and is pure CPU cost (#5392). Placeholders: now.
const depletedClientsCondLocal = `((total > 0 AND up + down >= total)
	OR (expiry_time > 0 AND expiry_time <= ?))`

// depletedCond returns the predicate matching depleted clients together with
// the arguments it binds. The local-only variant is used unless this panel
// holds a global-traffic row a master still refreshes, in which case the
// cross-panel EXISTS check is needed to enforce combined quota.
func depletedCond(tx *gorm.DB) (string, []any) {
	now := time.Now().UnixMilli()
	freshSince := globalTrafficFreshSince()
	var probe int64
	err := tx.Model(&model.ClientGlobalTraffic{}).
		Where("updated_at >= ?", freshSince).
		Limit(1).Count(&probe).Error
	if err == nil && probe > 0 {
		return depletedClientsCond, []any{now, freshSince}
	}
	return depletedClientsCondLocal, []any{now}
}

func (s *InboundService) disableInvalidClients(tx *gorm.DB, mutationBatch *trafficMutationBatch) (bool, int64, []int, error) {
	now := time.Now().UnixMilli()
	cond, condArgs := depletedCond(tx)

	var depletedRows []xray.ClientTraffic
	err := tx.Model(xray.ClientTraffic{}).
		Where(cond+" AND enable = ?", append(condArgs, true)...).
		Find(&depletedRows).Error
	if err != nil {
		return false, 0, nil, err
	}
	if len(depletedRows) == 0 {
		return false, 0, nil, nil
	}

	depletedEmails := make([]string, 0, len(depletedRows))
	for i := range depletedRows {
		if depletedRows[i].Email == "" {
			continue
		}
		depletedEmails = append(depletedEmails, depletedRows[i].Email)
	}

	type target struct {
		InboundID int  `gorm:"column:inbound_id"`
		NodeID    *int `gorm:"column:node_id"`
		Tag       string
		Email     string
	}
	var targets []target
	if len(depletedEmails) > 0 {
		err = tx.Raw(`
			SELECT inbounds.id AS inbound_id, inbounds.node_id AS node_id,
			       inbounds.tag AS tag, clients.email AS email
			FROM clients
			JOIN client_inbounds ON client_inbounds.client_id = clients.id
			JOIN inbounds        ON inbounds.id = client_inbounds.inbound_id
			WHERE clients.email IN ?
		`, depletedEmails).Scan(&targets).Error
		if err != nil {
			return false, 0, nil, err
		}
	}

	byInbound := make(map[int][]target)
	for _, t := range targets {
		byInbound[t.InboundID] = append(byInbound[t.InboundID], t)
	}

	disabledNodeIDs := make(map[int]struct{})
	for inboundID, group := range byInbound {
		emails := make(map[string]struct{}, len(group))
		for _, t := range group {
			emails[t.Email] = struct{}{}
		}
		oldInbound, inbound, mErr := s.markClientsDisabledInSettings(tx, inboundID, emails)
		if mErr != nil {
			return false, 0, nil, mErr
		}
		if inbound.NodeID != nil {
			mutationBatch.remotePlans = append(mutationBatch.remotePlans, trafficInboundUpdatePlan{
				oldInbound: *oldInbound, newInbound: *inbound,
			})
			mutationBatch.addNode(*inbound.NodeID)
			disabledNodeIDs[*inbound.NodeID] = struct{}{}
			continue
		}
		for email := range emails {
			mutationBatch.localPlans = append(mutationBatch.localPlans, trafficLocalApplyPlan{
				action: trafficRemoveUser, inbound: *inbound, email: email,
			})
		}
	}
	// Flip the rows already collected above by primary key instead of
	// re-evaluating the depleted predicate, which was a second full scan of
	// client_traffics on every poll. Sorted ids keep the lock order stable.
	ids := make([]int, 0, len(depletedRows))
	for i := range depletedRows {
		ids = append(ids, depletedRows[i].Id)
	}
	slices.Sort(ids)
	var count int64
	for _, batch := range chunkInts(ids, sqlInChunk) {
		result := tx.Model(xray.ClientTraffic{}).
			Where("id IN ? AND enable = ?", batch, true).
			Update("enable", false)
		if result.Error != nil {
			return false, count, nil, result.Error
		}
		count += result.RowsAffected
	}

	if len(depletedEmails) > 0 {
		if err := tx.Model(&model.ClientRecord{}).
			Where("email IN ?", depletedEmails).
			Updates(map[string]any{"enable": false, "updated_at": now}).Error; err != nil {
			return false, count, nil, err
		}
	}

	nodeIDs := make([]int, 0, len(disabledNodeIDs))
	for nodeID := range disabledNodeIDs {
		nodeIDs = append(nodeIDs, nodeID)
	}

	return false, count, nodeIDs, nil
}

// markClientsDisabledInSettings flips client.enable=false in the inbound's
// stored settings JSON for the given emails and returns both the pre and
// post snapshots so a caller pushing to a remote node has the diff to hand.
func (s *InboundService) markClientsDisabledInSettings(tx *gorm.DB, inboundID int, emails map[string]struct{}) (oldIb, newIb *model.Inbound, err error) {
	var ib model.Inbound
	if err := tx.Model(&model.Inbound{}).Where("id = ?", inboundID).First(&ib).Error; err != nil {
		return nil, nil, err
	}
	snapshot := ib

	settings := map[string]any{}
	if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
		return nil, nil, err
	}
	clients, _ := settings["clients"].([]any)
	now := time.Now().Unix() * 1000
	mutated := false
	for i := range clients {
		entry, ok := clients[i].(map[string]any)
		if !ok {
			continue
		}
		email, _ := entry["email"].(string)
		if _, hit := emails[email]; !hit {
			continue
		}
		if cur, _ := entry["enable"].(bool); !cur {
			continue
		}
		entry["enable"] = false
		entry["updated_at"] = now
		clients[i] = entry
		mutated = true
	}
	if !mutated {
		return &snapshot, &ib, nil
	}
	settings["clients"] = clients
	bs, marshalErr := json.MarshalIndent(settings, "", "  ")
	if marshalErr != nil {
		return nil, nil, marshalErr
	}
	ib.Settings = string(bs)
	if err := tx.Model(&model.Inbound{}).Where("id = ?", inboundID).
		Update("settings", ib.Settings).Error; err != nil {
		return nil, nil, err
	}
	return &snapshot, &ib, nil
}
