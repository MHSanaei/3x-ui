package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

func (s *InboundService) disableInvalidInbounds(tx *gorm.DB) (bool, int64, error) {
	now := time.Now().Unix() * 1000
	needRestart := false

	if p != nil {
		var tags []string
		err := tx.Table("inbounds").
			Select("inbounds.tag").
			Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ? and node_id IS NULL", now, true).
			Scan(&tags).Error
		if err != nil {
			return false, 0, err
		}
		s.xrayApi.Init(p.GetAPIPort())
		for _, tag := range tags {
			err1 := s.xrayApi.DelInbound(tag)
			if err1 == nil {
				logger.Debug("Inbound disabled by api:", tag)
			} else {
				logger.Debug("Error in disabling inbound by api:", err1)
				needRestart = true
			}
		}
		s.xrayApi.Close()
	}

	result := tx.Model(model.Inbound{}).
		Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ? and node_id IS NULL", now, true).
		Update("enable", false)
	err := result.Error
	count := result.RowsAffected
	return needRestart, count, err
}

// depletedClientsCond matches clients that exhausted their quota or expired.
// Besides the local counters it also trips on the cross-panel usage a master
// pushed into client_global_traffics — that's what lets a node cut a client
// whose combined usage exceeds the quota even though the local share doesn't
// (placeholders: now).
const depletedClientsCond = `((total > 0 AND up + down >= total)
	OR (expiry_time > 0 AND expiry_time <= ?)
	OR (total > 0 AND EXISTS (
		SELECT 1 FROM client_global_traffics g
		WHERE g.email = client_traffics.email AND g.up + g.down >= client_traffics.total
	)))`

// depletedClientsCondLocal is depletedClientsCond without the cross-panel
// client_global_traffics check. The EXISTS branch is a correlated subquery that
// turns every traffic poll into a full client_traffics scan; on a panel no
// master pushes to (the common case) client_global_traffics is empty, so the
// branch can never match and is pure CPU cost (#5392).
const depletedClientsCondLocal = `((total > 0 AND up + down >= total)
	OR (expiry_time > 0 AND expiry_time <= ?))`

// depletedCond returns the local-only predicate unless this panel actually
// holds global-traffic rows, in which case the cross-panel EXISTS check is
// needed to enforce combined quota. Both variants take the same single
// expiry_time placeholder, so callers pass identical args either way.
func depletedCond(tx *gorm.DB) string {
	var probe int64
	if err := tx.Model(&model.ClientGlobalTraffic{}).Limit(1).Count(&probe).Error; err == nil && probe > 0 {
		return depletedClientsCond
	}
	return depletedClientsCondLocal
}

func (s *InboundService) disableInvalidClients(tx *gorm.DB) (bool, int64, []int, error) {
	now := time.Now().Unix() * 1000
	needRestart := false
	cond := depletedCond(tx)

	var depletedRows []xray.ClientTraffic
	err := tx.Model(xray.ClientTraffic{}).
		Where(cond+" AND enable = ?", now, true).
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

	var localTargets []target
	localByInbound := make(map[int]map[string]struct{})
	remoteByInbound := make(map[int][]target)
	for _, t := range targets {
		if t.NodeID == nil {
			localTargets = append(localTargets, t)
			if localByInbound[t.InboundID] == nil {
				localByInbound[t.InboundID] = make(map[string]struct{})
			}
			localByInbound[t.InboundID][t.Email] = struct{}{}
		} else {
			remoteByInbound[t.InboundID] = append(remoteByInbound[t.InboundID], t)
		}
	}

	if p != nil && len(localTargets) > 0 {
		s.xrayApi.Init(p.GetAPIPort())
		for _, t := range localTargets {
			err1 := s.xrayApi.RemoveUser(t.Tag, t.Email)
			if err1 == nil {
				logger.Debug("Client disabled by api:", t.Email)
			} else if strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", t.Email)) {
				logger.Debug("User is already disabled. Nothing to do more...")
			} else {
				logger.Debug("Error in disabling client by api:", err1)
				needRestart = true
			}
		}
		s.xrayApi.Close()
	}

	for inboundID, emails := range localByInbound {
		if _, _, mErr := s.markClientsDisabledInSettings(tx, inboundID, emails); mErr != nil {
			logger.Warning("disableInvalidClients: settings.JSON sync failed for inbound", inboundID, ":", mErr)
		}
	}

	result := tx.Model(xray.ClientTraffic{}).
		Where(cond+" AND enable = ?", now, true).
		Update("enable", false)
	err = result.Error
	count := result.RowsAffected
	if err != nil {
		return needRestart, count, nil, err
	}

	if len(depletedEmails) > 0 {
		if err := tx.Model(&model.ClientRecord{}).
			Where("email IN ?", depletedEmails).
			Updates(map[string]any{"enable": false, "updated_at": now}).Error; err != nil {
			logger.Warning("disableInvalidClients update clients.enable:", err)
		}
	}

	disabledNodeIDs := make(map[int]struct{})
	for inboundID, group := range remoteByInbound {
		emails := make(map[string]struct{}, len(group))
		for _, t := range group {
			emails[t.Email] = struct{}{}
		}
		if pushErr := s.disableRemoteClients(tx, inboundID, emails); pushErr != nil {
			logger.Warning("disableInvalidClients: push to remote failed for inbound", inboundID, ":", pushErr)
			needRestart = true
		} else {
			for _, t := range group {
				if t.NodeID != nil {
					disabledNodeIDs[*t.NodeID] = struct{}{}
				}
			}
		}
	}

	nodeIDs := make([]int, 0, len(disabledNodeIDs))
	for nodeID := range disabledNodeIDs {
		nodeIDs = append(nodeIDs, nodeID)
	}

	return needRestart, count, nodeIDs, nil
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
		if cur, _ := entry["enable"].(bool); cur == false {
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

func (s *InboundService) disableRemoteClients(tx *gorm.DB, inboundID int, emails map[string]struct{}) error {
	oldSnapshot, ib, err := s.markClientsDisabledInSettings(tx, inboundID, emails)
	if err != nil {
		return err
	}

	rt, err := s.runtimeFor(ib)
	if err != nil {
		return err
	}
	if err := rt.UpdateInbound(context.Background(), oldSnapshot, ib); err != nil {
		return err
	}
	return nil
}
