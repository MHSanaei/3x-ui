// Package service provides business logic services for the 3x-ui web panel,
// including inbound/outbound management, user administration, settings, and Xray integration.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/web/runtime"
	"github.com/mhsanaei/3x-ui/v2/xray"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// InboundService provides business logic for managing Xray inbound configurations.
// It handles CRUD operations for inbounds, client management, traffic monitoring,
// and integration with the Xray API for real-time updates.
type InboundService struct {
	// xrayApi is retained for backwards compatibility with bulk paths
	// that still talk to the local engine directly (e.g. traffic-reset
	// jobs that scope to NodeID IS NULL inbounds anyway). New code paths
	// route through runtimeFor() instead so they can target remote nodes.
	xrayApi xray.XrayAPI
}

// runtimeFor returns the Runtime adapter for an inbound's destination
// engine. Returns the local runtime when the inbound has no NodeID
// (legacy/local inbounds); otherwise the cached Remote for that node.
//
// nil is returned only when the runtime Manager hasn't been wired yet
// (extremely early bootstrap). Callers treat nil as a transient error
// and either fall back to needRestart=true or surface "panel still
// starting" upstream.
func (s *InboundService) runtimeFor(ib *model.Inbound) (runtime.Runtime, error) {
	mgr := runtime.GetManager()
	if mgr == nil {
		return nil, fmt.Errorf("runtime manager not initialised")
	}
	return mgr.RuntimeFor(ib.NodeID)
}

type CopyClientsResult struct {
	Added   []string `json:"added"`
	Skipped []string `json:"skipped"`
	Errors  []string `json:"errors"`
}

// enrichClientStats parses each inbound's clients once, fills in the
// UUID/SubId fields on the preloaded ClientStats, and tops up rows owned by
// a sibling inbound (shared-email mode — the row is keyed on email so it
// only preloads on its owning inbound).
func (s *InboundService) enrichClientStats(db *gorm.DB, inbounds []*model.Inbound) {
	if len(inbounds) == 0 {
		return
	}
	clientsByInbound := make([][]model.Client, len(inbounds))
	seenByInbound := make([]map[string]struct{}, len(inbounds))
	missing := make(map[string]struct{})
	for i, inbound := range inbounds {
		clients, _ := s.GetClients(inbound)
		clientsByInbound[i] = clients
		seen := make(map[string]struct{}, len(inbound.ClientStats))
		for _, st := range inbound.ClientStats {
			if st.Email != "" {
				seen[strings.ToLower(st.Email)] = struct{}{}
			}
		}
		seenByInbound[i] = seen
		for _, c := range clients {
			if c.Email == "" {
				continue
			}
			if _, ok := seen[strings.ToLower(c.Email)]; !ok {
				missing[c.Email] = struct{}{}
			}
		}
	}
	if len(missing) > 0 {
		emails := make([]string, 0, len(missing))
		for e := range missing {
			emails = append(emails, e)
		}
		var extra []xray.ClientTraffic
		if err := db.Model(xray.ClientTraffic{}).Where("email IN ?", emails).Find(&extra).Error; err != nil {
			logger.Warning("enrichClientStats:", err)
		} else {
			byEmail := make(map[string]xray.ClientTraffic, len(extra))
			for _, st := range extra {
				byEmail[strings.ToLower(st.Email)] = st
			}
			for i, inbound := range inbounds {
				for _, c := range clientsByInbound[i] {
					if c.Email == "" {
						continue
					}
					key := strings.ToLower(c.Email)
					if _, ok := seenByInbound[i][key]; ok {
						continue
					}
					if st, ok := byEmail[key]; ok {
						inbound.ClientStats = append(inbound.ClientStats, st)
						seenByInbound[i][key] = struct{}{}
					}
				}
			}
		}
	}
	for i, inbound := range inbounds {
		clients := clientsByInbound[i]
		if len(clients) == 0 || len(inbound.ClientStats) == 0 {
			continue
		}
		cMap := make(map[string]model.Client, len(clients))
		for _, c := range clients {
			cMap[strings.ToLower(c.Email)] = c
		}
		for j := range inbound.ClientStats {
			email := strings.ToLower(inbound.ClientStats[j].Email)
			if c, ok := cMap[email]; ok {
				inbound.ClientStats[j].UUID = c.ID
				inbound.ClientStats[j].SubId = c.SubID
			}
		}
	}
}

// GetInbounds retrieves all inbounds for a specific user with client stats.
func (s *InboundService) GetInbounds(userId int) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Where("user_id = ?", userId).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	s.enrichClientStats(db, inbounds)
	return inbounds, nil
}

// GetAllInbounds retrieves all inbounds with client stats.
func (s *InboundService) GetAllInbounds() ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	s.enrichClientStats(db, inbounds)
	return inbounds, nil
}

func (s *InboundService) GetInboundsByTrafficReset(period string) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Where("traffic_reset = ?", period).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) GetClients(inbound *model.Inbound) ([]model.Client, error) {
	settings := map[string][]model.Client{}
	json.Unmarshal([]byte(inbound.Settings), &settings)
	if settings == nil {
		return nil, fmt.Errorf("setting is null")
	}

	clients := settings["clients"]
	if clients == nil {
		return nil, nil
	}
	return clients, nil
}

func (s *InboundService) getAllEmails() ([]string, error) {
	db := database.GetDB()
	var emails []string
	err := db.Raw(`
		SELECT DISTINCT JSON_EXTRACT(client.value, '$.email')
		FROM inbounds,
			JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		`).Scan(&emails).Error
	if err != nil {
		return nil, err
	}
	return emails, nil
}

// getAllEmailSubIDs returns email→subId. An email seen with two different
// non-empty subIds is locked (mapped to "") so neither identity can claim it.
func (s *InboundService) getAllEmailSubIDs() (map[string]string, error) {
	db := database.GetDB()
	var rows []struct {
		Email string
		SubID string
	}
	err := db.Raw(`
		SELECT JSON_EXTRACT(client.value, '$.email')  AS email,
		       JSON_EXTRACT(client.value, '$.subId')  AS sub_id
		FROM inbounds,
			JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(rows))
	for _, r := range rows {
		email := strings.ToLower(strings.Trim(r.Email, "\""))
		if email == "" {
			continue
		}
		subID := strings.Trim(r.SubID, "\"")
		if existing, ok := result[email]; ok {
			if existing != subID {
				result[email] = ""
			}
			continue
		}
		result[email] = subID
	}
	return result, nil
}

func lowerAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = strings.ToLower(s)
	}
	return out
}

// emailUsedByOtherInbounds reports whether email lives in any inbound other
// than exceptInboundId. Empty email returns false.
func (s *InboundService) emailUsedByOtherInbounds(email string, exceptInboundId int) (bool, error) {
	if email == "" {
		return false, nil
	}
	db := database.GetDB()
	var count int64
	err := db.Raw(`
		SELECT COUNT(*)
		FROM inbounds,
			JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		WHERE inbounds.id != ?
		  AND LOWER(JSON_EXTRACT(client.value, '$.email')) = LOWER(?)
		`, exceptInboundId, email).Scan(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// checkEmailsExistForClients validates a batch of incoming clients. An email
// collides only when the existing holder has a different (or empty) subId —
// matching non-empty subIds let multiple inbounds share one identity.
func (s *InboundService) checkEmailsExistForClients(clients []model.Client) (string, error) {
	emailSubIDs, err := s.getAllEmailSubIDs()
	if err != nil {
		return "", err
	}
	seen := make(map[string]string, len(clients))
	for _, client := range clients {
		if client.Email == "" {
			continue
		}
		key := strings.ToLower(client.Email)
		// Within the same payload, the same email must carry the same subId;
		// otherwise we would silently merge two distinct identities.
		if prev, ok := seen[key]; ok {
			if prev != client.SubID || client.SubID == "" {
				return client.Email, nil
			}
			continue
		}
		seen[key] = client.SubID
		if existingSub, ok := emailSubIDs[key]; ok {
			if client.SubID == "" || existingSub == "" || existingSub != client.SubID {
				return client.Email, nil
			}
		}
	}
	return "", nil
}

// AddInbound creates a new inbound configuration.
// It validates port uniqueness, client email uniqueness, and required fields,
// then saves the inbound to the database and optionally adds it to the running Xray instance.
// Returns the created inbound, whether Xray needs restart, and any error.
func (s *InboundService) AddInbound(inbound *model.Inbound) (*model.Inbound, bool, error) {
	exist, err := s.checkPortConflict(inbound, 0)
	if err != nil {
		return inbound, false, err
	}
	if exist {
		return inbound, false, common.NewError("Port already exists:", inbound.Port)
	}

	// pick a tag that won't collide with an existing row. for the common
	// case this is the same "inbound-<port>" string the controller already
	// set; only when this port already has another inbound on a different
	// transport (now possible after the transport-aware port check) does
	// this disambiguate with a -tcp/-udp suffix. see #4103.
	inbound.Tag, err = s.generateInboundTag(inbound, 0)
	if err != nil {
		return inbound, false, err
	}

	clients, err := s.GetClients(inbound)
	if err != nil {
		return inbound, false, err
	}
	existEmail, err := s.checkEmailsExistForClients(clients)
	if err != nil {
		return inbound, false, err
	}
	if existEmail != "" {
		return inbound, false, common.NewError("Duplicate email:", existEmail)
	}

	// Ensure created_at and updated_at on clients in settings
	if len(clients) > 0 {
		var settings map[string]any
		if err2 := json.Unmarshal([]byte(inbound.Settings), &settings); err2 == nil && settings != nil {
			now := time.Now().Unix() * 1000
			updatedClients := make([]model.Client, 0, len(clients))
			for _, c := range clients {
				if c.CreatedAt == 0 {
					c.CreatedAt = now
				}
				c.UpdatedAt = now
				updatedClients = append(updatedClients, c)
			}
			settings["clients"] = updatedClients
			if bs, err3 := json.MarshalIndent(settings, "", "  "); err3 == nil {
				inbound.Settings = string(bs)
			} else {
				logger.Debug("Unable to marshal inbound settings with timestamps:", err3)
			}
		} else if err2 != nil {
			logger.Debug("Unable to parse inbound settings for timestamps:", err2)
		}
	}

	// Secure client ID
	for _, client := range clients {
		switch inbound.Protocol {
		case "trojan":
			if client.Password == "" {
				return inbound, false, common.NewError("empty client ID")
			}
		case "shadowsocks":
			if client.Email == "" {
				return inbound, false, common.NewError("empty client ID")
			}
		case "hysteria", "hysteria2":
			if client.Auth == "" {
				return inbound, false, common.NewError("empty client ID")
			}
		default:
			if client.ID == "" {
				return inbound, false, common.NewError("empty client ID")
			}
		}
	}

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	err = tx.Save(inbound).Error
	if err == nil {
		if len(inbound.ClientStats) == 0 {
			for _, client := range clients {
				s.AddClientStat(tx, inbound.Id, &client)
			}
		}
	} else {
		return inbound, false, err
	}

	needRestart := false
	if inbound.Enable {
		rt, rterr := s.runtimeFor(inbound)
		if rterr != nil {
			// Fail-fast on remote routing errors. Assign to the named
			// `err` so the deferred tx handler rolls back the central
			// DB row that tx.Save just inserted — otherwise we'd leave
			// an orphan that the user sees succeed despite the toast.
			err = rterr
			return inbound, false, err
		}
		if err1 := rt.AddInbound(context.Background(), inbound); err1 == nil {
			logger.Debug("New inbound added on", rt.Name(), ":", inbound.Tag)
		} else {
			logger.Debug("Unable to add inbound on", rt.Name(), ":", err1)
			if inbound.NodeID != nil {
				// Remote add failed — roll back so central + node stay
				// in sync (no row on either side).
				err = err1
				return inbound, false, err
			}
			// Local: keep the existing fall-through-to-restart behaviour.
			needRestart = true
		}
	}

	return inbound, needRestart, err
}

// DelInbound deletes an inbound configuration by ID.
// It removes the inbound from the database and the running Xray instance if active.
// Returns whether Xray needs restart and any error.
func (s *InboundService) DelInbound(id int) (bool, error) {
	db := database.GetDB()

	needRestart := false
	// Load the full inbound (not just the tag) so we know its NodeID and
	// can route the runtime call to the right engine. Skip-on-not-found
	// preserves the old "no-op when DB row doesn't exist" behaviour.
	var ib model.Inbound
	loadErr := db.Model(model.Inbound{}).Where("id = ? and enable = ?", id, true).First(&ib).Error
	if loadErr == nil {
		// Delete is best-effort on the runtime side: the user's intent is
		// to get rid of the inbound, so a missing node row, an offline
		// node, or a remote-side "already gone" should NEVER block the
		// central DB cleanup. Worst case the remote keeps an orphan that
		// the user can clean up manually — far less painful than the row
		// being stuck on central.
		rt, rterr := s.runtimeFor(&ib)
		if rterr != nil {
			logger.Warning("DelInbound: runtime lookup failed, deleting central row anyway:", rterr)
			if ib.NodeID == nil {
				needRestart = true
			}
		} else if err1 := rt.DelInbound(context.Background(), &ib); err1 == nil {
			logger.Debug("Inbound deleted on", rt.Name(), ":", ib.Tag)
		} else {
			logger.Warning("DelInbound on", rt.Name(), "failed, deleting central row anyway:", err1)
			if ib.NodeID == nil {
				needRestart = true
			}
		}
	} else {
		logger.Debug("No enabled inbound found to remove by api, id:", id)
	}

	// Delete client traffics of inbounds
	err := db.Where("inbound_id = ?", id).Delete(xray.ClientTraffic{}).Error
	if err != nil {
		return false, err
	}
	inbound, err := s.GetInbound(id)
	if err != nil {
		return false, err
	}
	clients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}
	// Bulk-delete client IPs for every email in this inbound. The previous
	// per-client loop fired one DELETE per row — at 7k+ clients that meant
	// thousands of synchronous SQL roundtrips and a multi-second freeze.
	// Chunked to stay under SQLite's bind-variable limit on huge inbounds.
	if len(clients) > 0 {
		emails := make([]string, 0, len(clients))
		for i := range clients {
			if clients[i].Email != "" {
				emails = append(emails, clients[i].Email)
			}
		}
		for _, batch := range chunkStrings(uniqueNonEmptyStrings(emails), sqliteMaxVars) {
			if err := db.Where("client_email IN ?", batch).
				Delete(model.InboundClientIps{}).Error; err != nil {
				return false, err
			}
		}
	}

	return needRestart, db.Delete(model.Inbound{}, id).Error
}

func (s *InboundService) GetInbound(id int) (*model.Inbound, error) {
	db := database.GetDB()
	inbound := &model.Inbound{}
	err := db.Model(model.Inbound{}).First(inbound, id).Error
	if err != nil {
		return nil, err
	}
	return inbound, nil
}

// SetInboundEnable toggles only the enable flag of an inbound, without
// rewriting the (potentially multi-MB) settings JSON. Used by the UI's
// per-row enable switch — for inbounds with thousands of clients the full
// UpdateInbound path is an order of magnitude too slow for an interactive
// toggle (parses + reserialises every client, runs O(N) traffic diff).
//
// Returns (needRestart, error). needRestart is true when the xray runtime
// could not be re-synced from the cached config and a full restart is
// required to pick up the change.
func (s *InboundService) SetInboundEnable(id int, enable bool) (bool, error) {
	inbound, err := s.GetInbound(id)
	if err != nil {
		return false, err
	}
	if inbound.Enable == enable {
		return false, nil
	}

	db := database.GetDB()
	if err := db.Model(model.Inbound{}).Where("id = ?", id).
		Update("enable", enable).Error; err != nil {
		return false, err
	}
	inbound.Enable = enable

	// Sync xray runtime via the Runtime adapter. For local inbounds we
	// also rebuild the runtime config (drops clients flagged as disabled
	// in ClientTraffic) so the live xray sees the same filtered view it
	// did pre-refactor. Remote runtimes ship the unfiltered inbound —
	// the remote panel does its own filtering before pushing to its xray.
	needRestart := false
	rt, rterr := s.runtimeFor(inbound)
	if rterr != nil {
		if inbound.NodeID != nil {
			return false, rterr
		}
		return true, nil
	}

	if err := rt.DelInbound(context.Background(), inbound); err != nil &&
		!strings.Contains(err.Error(), "not found") {
		logger.Debug("SetInboundEnable: DelInbound on", rt.Name(), "failed:", err)
		needRestart = true
	}
	if !enable {
		return needRestart, nil
	}

	addTarget := inbound
	if inbound.NodeID == nil {
		runtimeInbound, err := s.buildRuntimeInboundForAPI(db, inbound)
		if err != nil {
			logger.Debug("SetInboundEnable: build runtime config failed:", err)
			return true, nil
		}
		addTarget = runtimeInbound
	}
	if err := rt.AddInbound(context.Background(), addTarget); err != nil {
		logger.Debug("SetInboundEnable: AddInbound on", rt.Name(), "failed:", err)
		if inbound.NodeID != nil {
			return false, err
		}
		needRestart = true
	}
	return needRestart, nil
}

// UpdateInbound modifies an existing inbound configuration.
// It validates changes, updates the database, and syncs with the running Xray instance.
// Returns the updated inbound, whether Xray needs restart, and any error.
func (s *InboundService) UpdateInbound(inbound *model.Inbound) (*model.Inbound, bool, error) {
	exist, err := s.checkPortConflict(inbound, inbound.Id)
	if err != nil {
		return inbound, false, err
	}
	if exist {
		return inbound, false, common.NewError("Port already exists:", inbound.Port)
	}

	oldInbound, err := s.GetInbound(inbound.Id)
	if err != nil {
		return inbound, false, err
	}

	tag := oldInbound.Tag

	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	err = s.updateClientTraffics(tx, oldInbound, inbound)
	if err != nil {
		return inbound, false, err
	}

	// Ensure created_at and updated_at exist in inbound.Settings clients
	{
		var oldSettings map[string]any
		_ = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
		emailToCreated := map[string]int64{}
		emailToUpdated := map[string]int64{}
		if oldSettings != nil {
			if oc, ok := oldSettings["clients"].([]any); ok {
				for _, it := range oc {
					if m, ok2 := it.(map[string]any); ok2 {
						if email, ok3 := m["email"].(string); ok3 {
							switch v := m["created_at"].(type) {
							case float64:
								emailToCreated[email] = int64(v)
							case int64:
								emailToCreated[email] = v
							}
							switch v := m["updated_at"].(type) {
							case float64:
								emailToUpdated[email] = int64(v)
							case int64:
								emailToUpdated[email] = v
							}
						}
					}
				}
			}
		}
		var newSettings map[string]any
		if err2 := json.Unmarshal([]byte(inbound.Settings), &newSettings); err2 == nil && newSettings != nil {
			now := time.Now().Unix() * 1000
			if nSlice, ok := newSettings["clients"].([]any); ok {
				for i := range nSlice {
					if m, ok2 := nSlice[i].(map[string]any); ok2 {
						email, _ := m["email"].(string)
						if _, ok3 := m["created_at"]; !ok3 {
							if v, ok4 := emailToCreated[email]; ok4 && v > 0 {
								m["created_at"] = v
							} else {
								m["created_at"] = now
							}
						}
						// Preserve client's updated_at if present; do not bump on parent inbound update
						if _, hasUpdated := m["updated_at"]; !hasUpdated {
							if v, ok4 := emailToUpdated[email]; ok4 && v > 0 {
								m["updated_at"] = v
							}
						}
						nSlice[i] = m
					}
				}
				newSettings["clients"] = nSlice
				if bs, err3 := json.MarshalIndent(newSettings, "", "  "); err3 == nil {
					inbound.Settings = string(bs)
				}
			}
		}
	}

	oldInbound.Up = inbound.Up
	oldInbound.Down = inbound.Down
	oldInbound.Total = inbound.Total
	oldInbound.Remark = inbound.Remark
	oldInbound.Enable = inbound.Enable
	oldInbound.ExpiryTime = inbound.ExpiryTime
	oldInbound.TrafficReset = inbound.TrafficReset
	oldInbound.Listen = inbound.Listen
	oldInbound.Port = inbound.Port
	oldInbound.Protocol = inbound.Protocol
	oldInbound.Settings = inbound.Settings
	oldInbound.StreamSettings = inbound.StreamSettings
	oldInbound.Sniffing = inbound.Sniffing
	// regenerate tag with collision-aware logic. for this row we pass
	// inbound.Id as ignoreId so it doesn't see its own old tag in the db.
	oldInbound.Tag, err = s.generateInboundTag(inbound, inbound.Id)
	if err != nil {
		return inbound, false, err
	}

	needRestart := false
	rt, rterr := s.runtimeFor(oldInbound)
	if rterr != nil {
		if oldInbound.NodeID != nil {
			err = rterr
			return inbound, false, err
		}
		needRestart = true
	} else {
		// Use a snapshot of the OLD tag so the remote can resolve its
		// remote-id even when the new tag has changed (port/listen edit).
		oldSnapshot := *oldInbound
		oldSnapshot.Tag = tag
		if oldInbound.NodeID == nil {
			// Local: keep the old del-then-add-filtered behaviour to
			// preserve runtime client filtering.
			if err2 := rt.DelInbound(context.Background(), &oldSnapshot); err2 == nil {
				logger.Debug("Old inbound deleted on", rt.Name(), ":", tag)
			}
			if inbound.Enable {
				runtimeInbound, err2 := s.buildRuntimeInboundForAPI(tx, oldInbound)
				if err2 != nil {
					logger.Debug("Unable to prepare runtime inbound config:", err2)
					needRestart = true
				} else if err2 := rt.AddInbound(context.Background(), runtimeInbound); err2 == nil {
					logger.Debug("Updated inbound added on", rt.Name(), ":", oldInbound.Tag)
				} else {
					logger.Debug("Unable to update inbound on", rt.Name(), ":", err2)
					needRestart = true
				}
			}
		} else {
			// Remote: a single UpdateInbound call (the Remote adapter
			// resolves remote-id by old tag, then POSTs /update/{id}).
			// Assign to the outer `err` on failure so the deferred tx
			// handler rolls back the central DB write.
			if !inbound.Enable {
				if err2 := rt.DelInbound(context.Background(), &oldSnapshot); err2 != nil {
					err = err2
					return inbound, false, err
				}
			} else if err2 := rt.UpdateInbound(context.Background(), &oldSnapshot, oldInbound); err2 != nil {
				err = err2
				return inbound, false, err
			}
		}
	}

	return inbound, needRestart, tx.Save(oldInbound).Error
}

func (s *InboundService) buildRuntimeInboundForAPI(tx *gorm.DB, inbound *model.Inbound) (*model.Inbound, error) {
	if inbound == nil {
		return nil, fmt.Errorf("inbound is nil")
	}

	runtimeInbound := *inbound
	settings := map[string]any{}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return nil, err
	}

	clients, ok := settings["clients"].([]any)
	if !ok {
		return &runtimeInbound, nil
	}

	var clientStats []xray.ClientTraffic
	err := tx.Model(xray.ClientTraffic{}).
		Where("inbound_id = ?", inbound.Id).
		Select("email", "enable").
		Find(&clientStats).Error
	if err != nil {
		return nil, err
	}

	enableMap := make(map[string]bool, len(clientStats))
	for _, clientTraffic := range clientStats {
		enableMap[clientTraffic.Email] = clientTraffic.Enable
	}

	finalClients := make([]any, 0, len(clients))
	for _, client := range clients {
		c, ok := client.(map[string]any)
		if !ok {
			continue
		}

		email, _ := c["email"].(string)
		if enable, exists := enableMap[email]; exists && !enable {
			continue
		}

		if manualEnable, ok := c["enable"].(bool); ok && !manualEnable {
			continue
		}

		finalClients = append(finalClients, c)
	}

	settings["clients"] = finalClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	runtimeInbound.Settings = string(modifiedSettings)

	return &runtimeInbound, nil
}

// updateClientTraffics syncs the ClientTraffic rows with the inbound's clients
// list: removes rows for emails that disappeared, inserts rows for newly-added
// emails. Uses sets for O(N) lookup — the previous nested-loop implementation
// was O(N²) and degraded into multi-second pauses on inbounds with thousands
// of clients (toggling, saving, or deleting any such inbound felt frozen).
func (s *InboundService) updateClientTraffics(tx *gorm.DB, oldInbound *model.Inbound, newInbound *model.Inbound) error {
	oldClients, err := s.GetClients(oldInbound)
	if err != nil {
		return err
	}
	newClients, err := s.GetClients(newInbound)
	if err != nil {
		return err
	}

	// Email is the unique key for ClientTraffic rows. Clients without an
	// email have no stats row to sync — skip them on both sides instead of
	// risking a unique-constraint hit or accidental delete of an unrelated row.
	oldEmails := make(map[string]struct{}, len(oldClients))
	for i := range oldClients {
		if oldClients[i].Email == "" {
			continue
		}
		oldEmails[oldClients[i].Email] = struct{}{}
	}
	newEmails := make(map[string]struct{}, len(newClients))
	for i := range newClients {
		if newClients[i].Email == "" {
			continue
		}
		newEmails[newClients[i].Email] = struct{}{}
	}

	// Drop stats rows for removed emails — but not when a sibling inbound
	// still references the email, since the row is the shared accumulator.
	for i := range oldClients {
		email := oldClients[i].Email
		if email == "" {
			continue
		}
		if _, kept := newEmails[email]; kept {
			continue
		}
		stillUsed, err := s.emailUsedByOtherInbounds(email, oldInbound.Id)
		if err != nil {
			return err
		}
		if stillUsed {
			continue
		}
		if err := s.DelClientStat(tx, email); err != nil {
			return err
		}
	}
	// Added clients — create their stats rows.
	for i := range newClients {
		email := newClients[i].Email
		if email == "" {
			continue
		}
		if _, existed := oldEmails[email]; existed {
			continue
		}
		if err := s.AddClientStat(tx, oldInbound.Id, &newClients[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *InboundService) AddInboundClient(data *model.Inbound) (bool, error) {
	clients, err := s.GetClients(data)
	if err != nil {
		return false, err
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(data.Settings), &settings)
	if err != nil {
		return false, err
	}

	interfaceClients := settings["clients"].([]any)
	// Add timestamps for new clients being appended
	nowTs := time.Now().Unix() * 1000
	for i := range interfaceClients {
		if cm, ok := interfaceClients[i].(map[string]any); ok {
			if _, ok2 := cm["created_at"]; !ok2 {
				cm["created_at"] = nowTs
			}
			cm["updated_at"] = nowTs
			interfaceClients[i] = cm
		}
	}
	existEmail, err := s.checkEmailsExistForClients(clients)
	if err != nil {
		return false, err
	}
	if existEmail != "" {
		return false, common.NewError("Duplicate email:", existEmail)
	}

	oldInbound, err := s.GetInbound(data.Id)
	if err != nil {
		return false, err
	}

	// Secure client ID
	for _, client := range clients {
		switch oldInbound.Protocol {
		case "trojan":
			if client.Password == "" {
				return false, common.NewError("empty client ID")
			}
		case "shadowsocks":
			if client.Email == "" {
				return false, common.NewError("empty client ID")
			}
		case "hysteria", "hysteria2":
			if client.Auth == "" {
				return false, common.NewError("empty client ID")
			}
		default:
			if client.ID == "" {
				return false, common.NewError("empty client ID")
			}
		}
	}

	var oldSettings map[string]any
	err = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
	if err != nil {
		return false, err
	}

	oldClients := oldSettings["clients"].([]any)
	oldClients = append(oldClients, interfaceClients...)

	oldSettings["clients"] = oldClients

	newSettings, err := json.MarshalIndent(oldSettings, "", "  ")
	if err != nil {
		return false, err
	}

	oldInbound.Settings = string(newSettings)

	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	needRestart := false
	rt, rterr := s.runtimeFor(oldInbound)
	if rterr != nil {
		if oldInbound.NodeID != nil {
			err = rterr
			return false, err
		}
		needRestart = true
	} else if oldInbound.NodeID == nil {
		// Local: per-client AddUser keeps existing connections alive
		// (incremental hot-add). Walk every new client; on any failure
		// fall back to needRestart so cron rebuilds from scratch.
		for _, client := range clients {
			if len(client.Email) == 0 {
				needRestart = true
				continue
			}
			s.AddClientStat(tx, data.Id, &client)
			if !client.Enable {
				continue
			}
			cipher := ""
			if oldInbound.Protocol == "shadowsocks" {
				cipher = oldSettings["method"].(string)
			}
			err1 := rt.AddUser(context.Background(), oldInbound, map[string]any{
				"email":    client.Email,
				"id":       client.ID,
				"auth":     client.Auth,
				"security": client.Security,
				"flow":     client.Flow,
				"password": client.Password,
				"cipher":   cipher,
			})
			if err1 == nil {
				logger.Debug("Client added on", rt.Name(), ":", client.Email)
			} else {
				logger.Debug("Error in adding client on", rt.Name(), ":", err1)
				needRestart = true
			}
		}
	} else {
		// Remote: a single UpdateInbound ships the new clients in one
		// HTTP round-trip rather than N. Settings are already mutated
		// in-memory (oldInbound.Settings) so the remote sees the final
		// state. Per-client ClientStat rows still need the central DB
		// update so the loop runs that branch first.
		for _, client := range clients {
			if len(client.Email) > 0 {
				s.AddClientStat(tx, data.Id, &client)
			}
		}
		if err1 := rt.UpdateInbound(context.Background(), oldInbound, oldInbound); err1 != nil {
			err = err1
			return false, err
		}
	}

	return needRestart, tx.Save(oldInbound).Error
}

func (s *InboundService) getClientPrimaryKey(protocol model.Protocol, client model.Client) string {
	switch protocol {
	case model.Trojan:
		return client.Password
	case model.Shadowsocks:
		return client.Email
	case model.Hysteria:
		return client.Auth
	default:
		return client.ID
	}
}

func (s *InboundService) writeBackClientSubID(sourceInboundID int, sourceProtocol model.Protocol, client model.Client, subID string) (bool, error) {
	client.SubID = subID
	client.UpdatedAt = time.Now().UnixMilli()
	clientID := s.getClientPrimaryKey(sourceProtocol, client)
	if clientID == "" {
		return false, common.NewError("empty client ID")
	}

	settingsBytes, err := json.Marshal(map[string][]model.Client{
		"clients": {client},
	})
	if err != nil {
		return false, err
	}

	updatePayload := &model.Inbound{
		Id:       sourceInboundID,
		Settings: string(settingsBytes),
	}
	return s.UpdateInboundClient(updatePayload, clientID)
}

func (s *InboundService) generateRandomCredential(targetProtocol model.Protocol) string {
	switch targetProtocol {
	case model.VMESS, model.VLESS:
		return uuid.NewString()
	default:
		return strings.ReplaceAll(uuid.NewString(), "-", "")
	}
}

func (s *InboundService) buildTargetClientFromSource(source model.Client, targetProtocol model.Protocol, email string, flow string) (model.Client, error) {
	nowTs := time.Now().UnixMilli()
	target := source
	target.Email = email
	target.CreatedAt = nowTs
	target.UpdatedAt = nowTs

	target.ID = ""
	target.Password = ""
	target.Auth = ""
	target.Flow = ""

	switch targetProtocol {
	case model.VMESS:
		target.ID = s.generateRandomCredential(targetProtocol)
	case model.VLESS:
		target.ID = s.generateRandomCredential(targetProtocol)
		if flow == "xtls-rprx-vision" || flow == "xtls-rprx-vision-udp443" {
			target.Flow = flow
		}
	case model.Trojan, model.Shadowsocks:
		target.Password = s.generateRandomCredential(targetProtocol)
	case model.Hysteria:
		target.Auth = s.generateRandomCredential(targetProtocol)
	default:
		target.ID = s.generateRandomCredential(targetProtocol)
	}

	return target, nil
}

func (s *InboundService) nextAvailableCopiedEmail(originalEmail string, targetID int, occupied map[string]struct{}) string {
	base := fmt.Sprintf("%s_%d", originalEmail, targetID)
	candidate := base
	suffix := 0
	for {
		if _, exists := occupied[strings.ToLower(candidate)]; !exists {
			occupied[strings.ToLower(candidate)] = struct{}{}
			return candidate
		}
		suffix++
		candidate = fmt.Sprintf("%s_%d", base, suffix)
	}
}

func (s *InboundService) CopyInboundClients(targetInboundID int, sourceInboundID int, clientEmails []string, flow string) (*CopyClientsResult, bool, error) {
	result := &CopyClientsResult{
		Added:   []string{},
		Skipped: []string{},
		Errors:  []string{},
	}
	if targetInboundID == sourceInboundID {
		return result, false, common.NewError("source and target inbounds must be different")
	}

	targetInbound, err := s.GetInbound(targetInboundID)
	if err != nil {
		return result, false, err
	}
	sourceInbound, err := s.GetInbound(sourceInboundID)
	if err != nil {
		return result, false, err
	}

	sourceClients, err := s.GetClients(sourceInbound)
	if err != nil {
		return result, false, err
	}
	if len(sourceClients) == 0 {
		return result, false, nil
	}

	allowedEmails := map[string]struct{}{}
	if len(clientEmails) > 0 {
		for _, email := range clientEmails {
			allowedEmails[strings.ToLower(strings.TrimSpace(email))] = struct{}{}
		}
	}

	occupiedEmails := map[string]struct{}{}
	allEmails, err := s.getAllEmails()
	if err != nil {
		return result, false, err
	}
	for _, email := range allEmails {
		clean := strings.Trim(email, "\"")
		if clean != "" {
			occupiedEmails[strings.ToLower(clean)] = struct{}{}
		}
	}

	newClients := make([]model.Client, 0)
	needRestart := false
	for _, sourceClient := range sourceClients {
		originalEmail := strings.TrimSpace(sourceClient.Email)
		if originalEmail == "" {
			continue
		}
		if len(allowedEmails) > 0 {
			if _, ok := allowedEmails[strings.ToLower(originalEmail)]; !ok {
				continue
			}
		}

		if sourceClient.SubID == "" {
			newSubID := uuid.NewString()
			subNeedRestart, subErr := s.writeBackClientSubID(sourceInbound.Id, sourceInbound.Protocol, sourceClient, newSubID)
			if subErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to write source subId: %v", originalEmail, subErr))
				continue
			}
			if subNeedRestart {
				needRestart = true
			}
			sourceClient.SubID = newSubID
		}

		targetEmail := s.nextAvailableCopiedEmail(originalEmail, targetInboundID, occupiedEmails)
		targetClient, buildErr := s.buildTargetClientFromSource(sourceClient, targetInbound.Protocol, targetEmail, flow)
		if buildErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", originalEmail, buildErr))
			continue
		}
		newClients = append(newClients, targetClient)
		result.Added = append(result.Added, targetEmail)
	}

	if len(newClients) == 0 {
		return result, needRestart, nil
	}

	settingsPayload, err := json.Marshal(map[string][]model.Client{
		"clients": newClients,
	})
	if err != nil {
		return result, needRestart, err
	}

	addNeedRestart, err := s.AddInboundClient(&model.Inbound{
		Id:       targetInboundID,
		Settings: string(settingsPayload),
	})
	if err != nil {
		return result, needRestart, err
	}
	if addNeedRestart {
		needRestart = true
	}

	return result, needRestart, nil
}

func (s *InboundService) DelInboundClient(inboundId int, clientId string) (bool, error) {
	oldInbound, err := s.GetInbound(inboundId)
	if err != nil {
		logger.Error("Load Old Data Error")
		return false, err
	}
	var settings map[string]any
	err = json.Unmarshal([]byte(oldInbound.Settings), &settings)
	if err != nil {
		return false, err
	}

	email := ""
	client_key := "id"
	switch oldInbound.Protocol {
	case "trojan":
		client_key = "password"
	case "shadowsocks":
		client_key = "email"
	case "hysteria", "hysteria2":
		client_key = "auth"
	}

	interfaceClients := settings["clients"].([]any)
	var newClients []any
	needApiDel := false
	clientFound := false
	for _, client := range interfaceClients {
		c := client.(map[string]any)
		c_id := c[client_key].(string)
		if c_id == clientId {
			clientFound = true
			email, _ = c["email"].(string)
			needApiDel, _ = c["enable"].(bool)
		} else {
			newClients = append(newClients, client)
		}
	}

	if !clientFound {
		return false, common.NewError("Client Not Found In Inbound For ID:", clientId)
	}

	if len(newClients) == 0 {
		return false, common.NewError("no client remained in Inbound")
	}

	settings["clients"] = newClients
	newSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}

	oldInbound.Settings = string(newSettings)

	db := database.GetDB()

	// Keep the client_traffics row and IPs alive when another inbound still
	// references this email — siblings depend on the shared accounting state.
	emailShared, err := s.emailUsedByOtherInbounds(email, inboundId)
	if err != nil {
		return false, err
	}

	if !emailShared {
		err = s.DelClientIPs(db, email)
		if err != nil {
			logger.Error("Error in delete client IPs")
			return false, err
		}
	}
	needRestart := false

	if len(email) > 0 {
		notDepleted := true
		err = db.Model(xray.ClientTraffic{}).Select("enable").Where("email = ?", email).First(&notDepleted).Error
		if err != nil {
			logger.Error("Get stats error")
			return false, err
		}
		if !emailShared {
			err = s.DelClientStat(db, email)
			if err != nil {
				logger.Error("Delete stats Data Error")
				return false, err
			}
		}
		if needApiDel && notDepleted {
			rt, rterr := s.runtimeFor(oldInbound)
			if rterr != nil {
				if oldInbound.NodeID != nil {
					return false, rterr
				}
				needRestart = true
			} else if oldInbound.NodeID == nil {
				err1 := rt.RemoveUser(context.Background(), oldInbound, email)
				if err1 == nil {
					logger.Debug("Client deleted on", rt.Name(), ":", email)
					needRestart = false
				} else if strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", email)) {
					logger.Debug("User is already deleted. Nothing to do more...")
				} else {
					logger.Debug("Error in deleting client on", rt.Name(), ":", err1)
					needRestart = true
				}
			} else {
				// Remote: settings already mutated above; one UpdateInbound
				// ships the post-deletion state to the node.
				if err1 := rt.UpdateInbound(context.Background(), oldInbound, oldInbound); err1 != nil {
					return false, err1
				}
			}
		}
	}
	return needRestart, db.Save(oldInbound).Error
}

func (s *InboundService) UpdateInboundClient(data *model.Inbound, clientId string) (bool, error) {
	// TODO: check if TrafficReset field is updating
	clients, err := s.GetClients(data)
	if err != nil {
		return false, err
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(data.Settings), &settings)
	if err != nil {
		return false, err
	}

	interfaceClients := settings["clients"].([]any)

	oldInbound, err := s.GetInbound(data.Id)
	if err != nil {
		return false, err
	}

	oldClients, err := s.GetClients(oldInbound)
	if err != nil {
		return false, err
	}

	oldEmail := ""
	newClientId := ""
	clientIndex := -1
	for index, oldClient := range oldClients {
		oldClientId := ""
		switch oldInbound.Protocol {
		case "trojan":
			oldClientId = oldClient.Password
			newClientId = clients[0].Password
		case "shadowsocks":
			oldClientId = oldClient.Email
			newClientId = clients[0].Email
		case "hysteria", "hysteria2":
			oldClientId = oldClient.Auth
			newClientId = clients[0].Auth
		default:
			oldClientId = oldClient.ID
			newClientId = clients[0].ID
		}
		if clientId == oldClientId {
			oldEmail = oldClient.Email
			clientIndex = index
			break
		}
	}

	// Validate new client ID
	if newClientId == "" || clientIndex == -1 {
		return false, common.NewError("empty client ID")
	}

	if len(clients[0].Email) > 0 && clients[0].Email != oldEmail {
		existEmail, err := s.checkEmailsExistForClients(clients)
		if err != nil {
			return false, err
		}
		if existEmail != "" {
			return false, common.NewError("Duplicate email:", existEmail)
		}
	}

	var oldSettings map[string]any
	err = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
	if err != nil {
		return false, err
	}
	settingsClients := oldSettings["clients"].([]any)
	// Preserve created_at and set updated_at for the replacing client
	var preservedCreated any
	if clientIndex >= 0 && clientIndex < len(settingsClients) {
		if oldMap, ok := settingsClients[clientIndex].(map[string]any); ok {
			if v, ok2 := oldMap["created_at"]; ok2 {
				preservedCreated = v
			}
		}
	}
	if len(interfaceClients) > 0 {
		if newMap, ok := interfaceClients[0].(map[string]any); ok {
			if preservedCreated == nil {
				preservedCreated = time.Now().Unix() * 1000
			}
			newMap["created_at"] = preservedCreated
			newMap["updated_at"] = time.Now().Unix() * 1000
			interfaceClients[0] = newMap
		}
	}
	settingsClients[clientIndex] = interfaceClients[0]
	oldSettings["clients"] = settingsClients

	// testseed is only meaningful when at least one VLESS client uses the exact
	// xtls-rprx-vision flow. The client-edit path only rewrites a single client,
	// so re-check the flow set here and strip a stale testseed when nothing in the
	// inbound still warrants it. The full-inbound update path already handles this
	// on the JS side via VLESSSettings.toJson().
	if oldInbound.Protocol == model.VLESS {
		hasVisionFlow := false
		for _, c := range settingsClients {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if flow, _ := cm["flow"].(string); flow == "xtls-rprx-vision" {
				hasVisionFlow = true
				break
			}
		}
		if !hasVisionFlow {
			delete(oldSettings, "testseed")
		}
	}

	newSettings, err := json.MarshalIndent(oldSettings, "", "  ")
	if err != nil {
		return false, err
	}

	oldInbound.Settings = string(newSettings)
	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	if len(clients[0].Email) > 0 {
		if len(oldEmail) > 0 {
			// Repointing onto an email that already has a row would collide on
			// the unique constraint, so retire the donor and let the surviving
			// row carry the merged identity.
			emailUnchanged := strings.EqualFold(oldEmail, clients[0].Email)
			targetExists := int64(0)
			if !emailUnchanged {
				if err = tx.Model(xray.ClientTraffic{}).Where("email = ?", clients[0].Email).Count(&targetExists).Error; err != nil {
					return false, err
				}
			}
			if emailUnchanged || targetExists == 0 {
				err = s.UpdateClientStat(tx, oldEmail, &clients[0])
				if err != nil {
					return false, err
				}
				err = s.UpdateClientIPs(tx, oldEmail, clients[0].Email)
				if err != nil {
					return false, err
				}
			} else {
				stillUsed, sErr := s.emailUsedByOtherInbounds(oldEmail, data.Id)
				if sErr != nil {
					return false, sErr
				}
				if !stillUsed {
					if err = s.DelClientStat(tx, oldEmail); err != nil {
						return false, err
					}
					if err = s.DelClientIPs(tx, oldEmail); err != nil {
						return false, err
					}
				}
				// Refresh the surviving row with the new client's limits/expiry.
				if err = s.UpdateClientStat(tx, clients[0].Email, &clients[0]); err != nil {
					return false, err
				}
			}
		} else {
			s.AddClientStat(tx, data.Id, &clients[0])
		}
	} else {
		stillUsed, err := s.emailUsedByOtherInbounds(oldEmail, data.Id)
		if err != nil {
			return false, err
		}
		if !stillUsed {
			err = s.DelClientStat(tx, oldEmail)
			if err != nil {
				return false, err
			}
			err = s.DelClientIPs(tx, oldEmail)
			if err != nil {
				return false, err
			}
		}
	}
	needRestart := false
	if len(oldEmail) > 0 {
		rt, rterr := s.runtimeFor(oldInbound)
		if rterr != nil {
			if oldInbound.NodeID != nil {
				err = rterr
				return false, err
			}
			needRestart = true
		} else if oldInbound.NodeID == nil {
			// Local: paired Remove+Add on the live xray, keeping other
			// clients online (full-restart fallback on partial failure).
			if oldClients[clientIndex].Enable {
				err1 := rt.RemoveUser(context.Background(), oldInbound, oldEmail)
				if err1 == nil {
					logger.Debug("Old client deleted on", rt.Name(), ":", oldEmail)
				} else if strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", oldEmail)) {
					logger.Debug("User is already deleted. Nothing to do more...")
				} else {
					logger.Debug("Error in deleting client on", rt.Name(), ":", err1)
					needRestart = true
				}
			}
			if clients[0].Enable {
				cipher := ""
				if oldInbound.Protocol == "shadowsocks" {
					cipher = oldSettings["method"].(string)
				}
				err1 := rt.AddUser(context.Background(), oldInbound, map[string]any{
					"email":    clients[0].Email,
					"id":       clients[0].ID,
					"security": clients[0].Security,
					"flow":     clients[0].Flow,
					"auth":     clients[0].Auth,
					"password": clients[0].Password,
					"cipher":   cipher,
				})
				if err1 == nil {
					logger.Debug("Client edited on", rt.Name(), ":", clients[0].Email)
				} else {
					logger.Debug("Error in adding client on", rt.Name(), ":", err1)
					needRestart = true
				}
			}
		} else {
			// Remote: settings already mutated; one UpdateInbound suffices.
			if err1 := rt.UpdateInbound(context.Background(), oldInbound, oldInbound); err1 != nil {
				err = err1
				return false, err
			}
		}
	} else {
		logger.Debug("Client old email not found")
		needRestart = true
	}
	return needRestart, tx.Save(oldInbound).Error
}

// resetGracePeriodMs is the window after a reset during which incoming
// traffic snapshots from the node are ignored if they would resurrect
// non-zero counters. Three sync ticks (10s each) is enough headroom for
// the central → node reset HTTP call to land before the next pull.
const resetGracePeriodMs int64 = 30000

// SetRemoteTraffic merges absolute counters from a remote node into the
// central DB. Unlike AddTraffic, which adds deltas pulled from the local
// xray gRPC stats endpoint, this SETs the values — the node already has
// the canonical absolute value and we just mirror it.
//
// Rows in the post-reset grace window are skipped if the snapshot would
// regress them, so a user-initiated reset survives until the propagation
// HTTP call has completed on the node. After the grace window expires
// the snapshot wins regardless (the node is authoritative for the
// inbounds it hosts).
func (s *InboundService) SetRemoteTraffic(nodeID int, snap *runtime.TrafficSnapshot) error {
	if snap == nil || nodeID <= 0 {
		return nil
	}
	db := database.GetDB()
	now := time.Now().UnixMilli()

	// Load central inbounds for this node so we can resolve tag→id and
	// honour the per-inbound grace window. One query covers every row
	// touched in this tick.
	var central []model.Inbound
	if err := db.Model(model.Inbound{}).
		Where("node_id = ?", nodeID).
		Find(&central).Error; err != nil {
		return err
	}
	tagToCentral := make(map[string]*model.Inbound, len(central))
	for i := range central {
		tagToCentral[central[i].Tag] = &central[i]
	}

	tx := db.Begin()
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	// Per-inbound counter merge. Skip rows whose central allTime is
	// suspiciously lower than the snapshot AND we're inside the grace
	// window — that's the "reset hit central but not the node yet"
	// pattern we want to defer until next tick.
	for _, snapIb := range snap.Inbounds {
		if snapIb == nil {
			continue
		}
		c, ok := tagToCentral[snapIb.Tag]
		if !ok {
			continue // node has an inbound the central doesn't know about — ignore
		}
		snapAllTime := snapIb.AllTime
		if snapAllTime == 0 {
			snapAllTime = snapIb.Up + snapIb.Down
		}
		inGrace := c.LastTrafficResetTime > 0 && now-c.LastTrafficResetTime < resetGracePeriodMs
		if inGrace && snapAllTime > c.AllTime {
			logger.Debug("SetRemoteTraffic: skipping inbound", c.Id, "in reset grace window")
			continue
		}
		if err := tx.Model(model.Inbound{}).
			Where("id = ?", c.Id).
			Updates(map[string]any{
				"up":       snapIb.Up,
				"down":     snapIb.Down,
				"all_time": snapAllTime,
			}).Error; err != nil {
			return err
		}
	}

	// Per-client merge. The snapshot's ClientStats are nested under
	// each Inbound, so flatten before walking. Each client_traffics row
	// is keyed by (inbound_id, email) — we resolve inbound_id from the
	// central inbound row matched above.
	for _, snapIb := range snap.Inbounds {
		if snapIb == nil {
			continue
		}
		c, ok := tagToCentral[snapIb.Tag]
		if !ok {
			continue
		}
		// Honour the same grace window for client rows: if the parent
		// inbound was just reset, leave its clients alone too.
		inGrace := c.LastTrafficResetTime > 0 && now-c.LastTrafficResetTime < resetGracePeriodMs
		for _, cs := range snapIb.ClientStats {
			snapAllTime := cs.AllTime
			if snapAllTime == 0 {
				snapAllTime = cs.Up + cs.Down
			}
			if inGrace {
				// Skip client rows whose snapshot would push counters
				// back up; allow rows that are zero on the node side
				// (those are normal — node was reset alongside central).
				if snapAllTime > 0 {
					continue
				}
			}
			// MAX(last_online, ?) so a momentary clock skew on the node
			// can't regress the central row's last-seen timestamp.
			if err := tx.Exec(
				`UPDATE client_traffics
				 SET up = ?, down = ?, all_time = ?, last_online = MAX(last_online, ?)
				 WHERE inbound_id = ? AND email = ?`,
				cs.Up, cs.Down, snapAllTime, cs.LastOnline, c.Id, cs.Email,
			).Error; err != nil {
				return err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}
	committed = true

	// Push the node's online-clients contribution into xray.Process so
	// GetOnlineClients returns the union of local + every node. Empty
	// list still calls Set so a node that just had everyone disconnect
	// updates promptly.
	if p != nil {
		p.SetNodeOnlineClients(nodeID, snap.OnlineEmails)
	}

	return nil
}

func (s *InboundService) AddTraffic(inboundTraffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) (bool, bool, error) {
	var err error
	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	err = s.addInboundTraffic(tx, inboundTraffics)
	if err != nil {
		return false, false, err
	}
	err = s.addClientTraffic(tx, clientTraffics)
	if err != nil {
		return false, false, err
	}

	needRestart0, count, err := s.autoRenewClients(tx)
	if err != nil {
		logger.Warning("Error in renew clients:", err)
	} else if count > 0 {
		logger.Debugf("%v clients renewed", count)
	}

	disabledClientsCount := int64(0)
	needRestart1, count, err := s.disableInvalidClients(tx)
	if err != nil {
		logger.Warning("Error in disabling invalid clients:", err)
	} else if count > 0 {
		logger.Debugf("%v clients disabled", count)
		disabledClientsCount = count
	}

	needRestart2, count, err := s.disableInvalidInbounds(tx)
	if err != nil {
		logger.Warning("Error in disabling invalid inbounds:", err)
	} else if count > 0 {
		logger.Debugf("%v inbounds disabled", count)
	}
	return needRestart0 || needRestart1 || needRestart2, disabledClientsCount > 0, nil
}

func (s *InboundService) addInboundTraffic(tx *gorm.DB, traffics []*xray.Traffic) error {
	if len(traffics) == 0 {
		return nil
	}

	var err error

	for _, traffic := range traffics {
		if traffic.IsInbound {
			err = tx.Model(&model.Inbound{}).Where("tag = ?", traffic.Tag).
				Updates(map[string]any{
					"up":       gorm.Expr("up + ?", traffic.Up),
					"down":     gorm.Expr("down + ?", traffic.Down),
					"all_time": gorm.Expr("COALESCE(all_time, 0) + ?", traffic.Up+traffic.Down),
				}).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *InboundService) addClientTraffic(tx *gorm.DB, traffics []*xray.ClientTraffic) (err error) {
	if len(traffics) == 0 {
		// Empty onlineUsers
		if p != nil {
			p.SetOnlineClients(make([]string, 0))
		}
		return nil
	}

	onlineClients := make([]string, 0)

	emails := make([]string, 0, len(traffics))
	for _, traffic := range traffics {
		emails = append(emails, traffic.Email)
	}
	dbClientTraffics := make([]*xray.ClientTraffic, 0, len(traffics))
	err = tx.Model(xray.ClientTraffic{}).Where("email IN (?)", emails).Find(&dbClientTraffics).Error
	if err != nil {
		return err
	}

	// Avoid empty slice error
	if len(dbClientTraffics) == 0 {
		return nil
	}

	dbClientTraffics, err = s.adjustTraffics(tx, dbClientTraffics)
	if err != nil {
		return err
	}

	// Index by email for O(N) merge — the previous nested loop was O(N²)
	// and dominated each cron tick on inbounds with thousands of active
	// clients (7500 × 7500 = 56M string comparisons every 10 seconds).
	trafficByEmail := make(map[string]*xray.ClientTraffic, len(traffics))
	for i := range traffics {
		if traffics[i] != nil {
			trafficByEmail[traffics[i].Email] = traffics[i]
		}
	}
	now := time.Now().UnixMilli()
	for dbTraffic_index := range dbClientTraffics {
		t, ok := trafficByEmail[dbClientTraffics[dbTraffic_index].Email]
		if !ok {
			continue
		}
		dbClientTraffics[dbTraffic_index].Up += t.Up
		dbClientTraffics[dbTraffic_index].Down += t.Down
		dbClientTraffics[dbTraffic_index].AllTime += t.Up + t.Down
		if t.Up+t.Down > 0 {
			onlineClients = append(onlineClients, t.Email)
			dbClientTraffics[dbTraffic_index].LastOnline = now
		}
	}

	// Set onlineUsers
	p.SetOnlineClients(onlineClients)

	err = tx.Save(dbClientTraffics).Error
	if err != nil {
		logger.Warning("AddClientTraffic update data ", err)
	}

	return nil
}

func (s *InboundService) adjustTraffics(tx *gorm.DB, dbClientTraffics []*xray.ClientTraffic) ([]*xray.ClientTraffic, error) {
	inboundIds := make([]int, 0, len(dbClientTraffics))
	for _, dbClientTraffic := range dbClientTraffics {
		if dbClientTraffic.ExpiryTime < 0 {
			inboundIds = append(inboundIds, dbClientTraffic.InboundId)
		}
	}

	if len(inboundIds) > 0 {
		var inbounds []*model.Inbound
		err := tx.Model(model.Inbound{}).Where("id IN (?)", inboundIds).Find(&inbounds).Error
		if err != nil {
			return nil, err
		}
		for inbound_index := range inbounds {
			settings := map[string]any{}
			json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
			clients, ok := settings["clients"].([]any)
			if ok {
				var newClients []any
				for client_index := range clients {
					c := clients[client_index].(map[string]any)
					for traffic_index := range dbClientTraffics {
						if dbClientTraffics[traffic_index].ExpiryTime < 0 && c["email"] == dbClientTraffics[traffic_index].Email {
							oldExpiryTime := c["expiryTime"].(float64)
							newExpiryTime := (time.Now().Unix() * 1000) - int64(oldExpiryTime)
							c["expiryTime"] = newExpiryTime
							c["updated_at"] = time.Now().Unix() * 1000
							dbClientTraffics[traffic_index].ExpiryTime = newExpiryTime
							break
						}
					}
					// Backfill created_at and updated_at
					if _, ok := c["created_at"]; !ok {
						c["created_at"] = time.Now().Unix() * 1000
					}
					c["updated_at"] = time.Now().Unix() * 1000
					newClients = append(newClients, any(c))
				}
				settings["clients"] = newClients
				modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
				if err != nil {
					return nil, err
				}

				inbounds[inbound_index].Settings = string(modifiedSettings)
			}
		}
		err = tx.Save(inbounds).Error
		if err != nil {
			logger.Warning("AddClientTraffic update inbounds ", err)
			logger.Error(inbounds)
		}
	}

	return dbClientTraffics, nil
}

func (s *InboundService) autoRenewClients(tx *gorm.DB) (bool, int64, error) {
	// check for time expired
	var traffics []*xray.ClientTraffic
	now := time.Now().Unix() * 1000
	var err, err1 error

	err = tx.Model(xray.ClientTraffic{}).Where("reset > 0 and expiry_time > 0 and expiry_time <= ?", now).Find(&traffics).Error
	if err != nil {
		return false, 0, err
	}
	// return if there is no client to renew
	if len(traffics) == 0 {
		return false, 0, nil
	}

	var inbound_ids []int
	var inbounds []*model.Inbound
	needRestart := false
	var clientsToAdd []struct {
		protocol string
		tag      string
		client   map[string]any
	}

	for _, traffic := range traffics {
		inbound_ids = append(inbound_ids, traffic.InboundId)
	}
	// Dedupe so an inbound hosting N expired clients is fetched and saved once
	// per tick instead of N times across chunk boundaries.
	inbound_ids = uniqueInts(inbound_ids)
	// Chunked to stay under SQLite's bind-variable limit when many inbounds
	// are touched in a single tick.
	for _, batch := range chunkInts(inbound_ids, sqliteMaxVars) {
		var page []*model.Inbound
		if err = tx.Model(model.Inbound{}).Where("id IN ?", batch).Find(&page).Error; err != nil {
			return false, 0, err
		}
		inbounds = append(inbounds, page...)
	}
	for inbound_index := range inbounds {
		settings := map[string]any{}
		json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
		clients := settings["clients"].([]any)
		for client_index := range clients {
			c := clients[client_index].(map[string]any)
			for traffic_index, traffic := range traffics {
				if traffic.Email == c["email"].(string) {
					newExpiryTime := traffic.ExpiryTime
					for newExpiryTime < now {
						newExpiryTime += (int64(traffic.Reset) * 86400000)
					}
					c["expiryTime"] = newExpiryTime
					traffics[traffic_index].ExpiryTime = newExpiryTime
					traffics[traffic_index].Down = 0
					traffics[traffic_index].Up = 0
					if !traffic.Enable {
						traffics[traffic_index].Enable = true
						clientsToAdd = append(clientsToAdd,
							struct {
								protocol string
								tag      string
								client   map[string]any
							}{
								protocol: string(inbounds[inbound_index].Protocol),
								tag:      inbounds[inbound_index].Tag,
								client:   c,
							})
					}
					clients[client_index] = any(c)
					break
				}
			}
		}
		settings["clients"] = clients
		newSettings, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return false, 0, err
		}
		inbounds[inbound_index].Settings = string(newSettings)
	}
	err = tx.Save(inbounds).Error
	if err != nil {
		return false, 0, err
	}
	err = tx.Save(traffics).Error
	if err != nil {
		return false, 0, err
	}
	if p != nil {
		err1 = s.xrayApi.Init(p.GetAPIPort())
		if err1 != nil {
			return true, int64(len(traffics)), nil
		}
		for _, clientToAdd := range clientsToAdd {
			err1 = s.xrayApi.AddUser(clientToAdd.protocol, clientToAdd.tag, clientToAdd.client)
			if err1 != nil {
				needRestart = true
			}
		}
		s.xrayApi.Close()
	}
	return needRestart, int64(len(traffics)), nil
}

func (s *InboundService) disableInvalidInbounds(tx *gorm.DB) (bool, int64, error) {
	now := time.Now().Unix() * 1000
	needRestart := false

	if p != nil {
		var tags []string
		err := tx.Table("inbounds").
			Select("inbounds.tag").
			Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ?", now, true).
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
		Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ?", now, true).
		Update("enable", false)
	err := result.Error
	count := result.RowsAffected
	return needRestart, count, err
}

func (s *InboundService) disableInvalidClients(tx *gorm.DB) (bool, int64, error) {
	now := time.Now().Unix() * 1000
	needRestart := false

	var depletedRows []xray.ClientTraffic
	err := tx.Model(xray.ClientTraffic{}).
		Where("((total > 0 AND up + down >= total) OR (expiry_time > 0 AND expiry_time <= ?)) AND enable = ?", now, true).
		Find(&depletedRows).Error
	if err != nil {
		return false, 0, err
	}
	if len(depletedRows) == 0 {
		return false, 0, nil
	}

	rowByEmail := make(map[string]*xray.ClientTraffic, len(depletedRows))
	depletedEmails := make([]string, 0, len(depletedRows))
	for i := range depletedRows {
		if depletedRows[i].Email == "" {
			continue
		}
		rowByEmail[strings.ToLower(depletedRows[i].Email)] = &depletedRows[i]
		depletedEmails = append(depletedEmails, depletedRows[i].Email)
	}

	// Resolve inbound membership only for the depleted emails — pushing the
	// filter into SQLite avoids dragging every panel client through Go for
	// the common case where most clients are healthy.
	var memberships []struct {
		InboundId int
		Tag       string
		Email     string
		SubID     string `gorm:"column:sub_id"`
	}
	if len(depletedEmails) > 0 {
		err = tx.Raw(`
			SELECT inbounds.id  AS inbound_id,
			       inbounds.tag AS tag,
			       JSON_EXTRACT(client.value, '$.email') AS email,
			       JSON_EXTRACT(client.value, '$.subId') AS sub_id
			FROM inbounds,
				JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
			WHERE LOWER(JSON_EXTRACT(client.value, '$.email')) IN ?
			`, lowerAll(depletedEmails)).Scan(&memberships).Error
		if err != nil {
			return false, 0, err
		}
	}

	// Discover the row holder's subId per email. Only siblings sharing it
	// get cascaded; legacy data where two identities reuse the same email
	// stays isolated to the row owner.
	holderSub := make(map[string]string, len(rowByEmail))
	for _, m := range memberships {
		email := strings.ToLower(strings.Trim(m.Email, "\""))
		row, ok := rowByEmail[email]
		if !ok || m.InboundId != row.InboundId {
			continue
		}
		holderSub[email] = strings.Trim(m.SubID, "\"")
	}

	type target struct {
		InboundId int
		Tag       string
		Email     string
	}
	var targets []target
	for _, m := range memberships {
		email := strings.ToLower(strings.Trim(m.Email, "\""))
		row, ok := rowByEmail[email]
		if !ok {
			continue
		}
		expected, hasSub := holderSub[email]
		mSub := strings.Trim(m.SubID, "\"")
		switch {
		case !hasSub || expected == "":
			if m.InboundId != row.InboundId {
				continue
			}
		case mSub != expected:
			continue
		}
		targets = append(targets, target{
			InboundId: m.InboundId,
			Tag:       m.Tag,
			Email:     strings.Trim(m.Email, "\""),
		})
	}

	if p != nil && len(targets) > 0 {
		s.xrayApi.Init(p.GetAPIPort())
		for _, t := range targets {
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

	result := tx.Model(xray.ClientTraffic{}).
		Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ?", now, true).
		Update("enable", false)
	err = result.Error
	count := result.RowsAffected
	if err != nil {
		return needRestart, count, err
	}

	if len(targets) == 0 {
		return needRestart, count, nil
	}

	// Mirror enable=false + the row's authoritative quota/expiry into every
	// (inbound, email) we just removed via the API.
	inboundEmailMap := make(map[int]map[string]struct{})
	for _, t := range targets {
		if inboundEmailMap[t.InboundId] == nil {
			inboundEmailMap[t.InboundId] = make(map[string]struct{})
		}
		inboundEmailMap[t.InboundId][t.Email] = struct{}{}
	}
	inboundIds := make([]int, 0, len(inboundEmailMap))
	for id := range inboundEmailMap {
		inboundIds = append(inboundIds, id)
	}
	var inbounds []*model.Inbound
	if err = tx.Model(model.Inbound{}).Where("id IN ?", inboundIds).Find(&inbounds).Error; err != nil {
		logger.Warning("disableInvalidClients fetch inbounds:", err)
		return needRestart, count, nil
	}
	dirty := make([]*model.Inbound, 0, len(inbounds))
	for _, inbound := range inbounds {
		settings := map[string]any{}
		if jsonErr := json.Unmarshal([]byte(inbound.Settings), &settings); jsonErr != nil {
			continue
		}
		clientsRaw, ok := settings["clients"].([]any)
		if !ok {
			continue
		}
		emailSet := inboundEmailMap[inbound.Id]
		changed := false
		for i := range clientsRaw {
			c, ok := clientsRaw[i].(map[string]any)
			if !ok {
				continue
			}
			email, _ := c["email"].(string)
			if _, shouldDisable := emailSet[email]; !shouldDisable {
				continue
			}
			c["enable"] = false
			if row, ok := rowByEmail[strings.ToLower(email)]; ok {
				c["totalGB"] = row.Total
				c["expiryTime"] = row.ExpiryTime
			}
			c["updated_at"] = now
			clientsRaw[i] = c
			changed = true
		}
		if !changed {
			continue
		}
		settings["clients"] = clientsRaw
		modifiedSettings, jsonErr := json.MarshalIndent(settings, "", "  ")
		if jsonErr != nil {
			continue
		}
		inbound.Settings = string(modifiedSettings)
		dirty = append(dirty, inbound)
	}
	if len(dirty) > 0 {
		if err = tx.Save(dirty).Error; err != nil {
			logger.Warning("disableInvalidClients update inbound settings:", err)
		}
	}

	return needRestart, count, nil
}

func (s *InboundService) GetInboundTags() (string, error) {
	db := database.GetDB()
	var inboundTags []string
	err := db.Model(model.Inbound{}).Select("tag").Find(&inboundTags).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return "", err
	}
	tags, _ := json.Marshal(inboundTags)
	return string(tags), nil
}

func (s *InboundService) GetClientReverseTags() (string, error) {
	db := database.GetDB()
	var inbounds []model.Inbound
	err := db.Model(model.Inbound{}).Select("settings").Where("protocol = ?", "vless").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return "[]", err
	}

	tagSet := make(map[string]struct{})
	for _, inbound := range inbounds {
		var settings map[string]any
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			continue
		}
		clients, ok := settings["clients"].([]any)
		if !ok {
			continue
		}
		for _, client := range clients {
			clientMap, ok := client.(map[string]any)
			if !ok {
				continue
			}
			reverse, ok := clientMap["reverse"].(map[string]any)
			if !ok {
				continue
			}
			tag, _ := reverse["tag"].(string)
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tagSet[tag] = struct{}{}
			}
		}
	}

	rawTags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		rawTags = append(rawTags, tag)
	}
	sort.Strings(rawTags)

	result, _ := json.Marshal(rawTags)
	return string(result), nil
}

func (s *InboundService) MigrationRemoveOrphanedTraffics() {
	db := database.GetDB()
	db.Exec(`
		DELETE FROM client_traffics
		WHERE email NOT IN (
			SELECT JSON_EXTRACT(client.value, '$.email')
			FROM inbounds,
				JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		)
	`)
}

// AddClientStat inserts a per-client accounting row, no-op on email
// conflict. Xray reports traffic per email, so the surviving row acts as
// the shared accumulator for inbounds that re-use the same identity.
func (s *InboundService) AddClientStat(tx *gorm.DB, inboundId int, client *model.Client) error {
	clientTraffic := xray.ClientTraffic{
		InboundId:  inboundId,
		Email:      client.Email,
		Total:      client.TotalGB,
		ExpiryTime: client.ExpiryTime,
		Enable:     client.Enable,
		Reset:      client.Reset,
	}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "email"}}, DoNothing: true}).
		Create(&clientTraffic).Error
}

func (s *InboundService) UpdateClientStat(tx *gorm.DB, email string, client *model.Client) error {
	result := tx.Model(xray.ClientTraffic{}).
		Where("email = ?", email).
		Updates(map[string]any{
			"enable":      client.Enable,
			"email":       client.Email,
			"total":       client.TotalGB,
			"expiry_time": client.ExpiryTime,
			"reset":       client.Reset,
		})
	err := result.Error
	return err
}

func (s *InboundService) UpdateClientIPs(tx *gorm.DB, oldEmail string, newEmail string) error {
	return tx.Model(model.InboundClientIps{}).Where("client_email = ?", oldEmail).Update("client_email", newEmail).Error
}

func (s *InboundService) DelClientStat(tx *gorm.DB, email string) error {
	return tx.Where("email = ?", email).Delete(xray.ClientTraffic{}).Error
}

func (s *InboundService) DelClientIPs(tx *gorm.DB, email string) error {
	return tx.Where("client_email = ?", email).Delete(model.InboundClientIps{}).Error
}

func (s *InboundService) GetClientInboundByTrafficID(trafficId int) (traffic *xray.ClientTraffic, inbound *model.Inbound, err error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).Where("id = ?", trafficId).Find(&traffics).Error
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with trafficId %d: %v", trafficId, err)
		return nil, nil, err
	}
	if len(traffics) > 0 {
		inbound, err = s.GetInbound(traffics[0].InboundId)
		return traffics[0], inbound, err
	}
	return nil, nil, nil
}

func (s *InboundService) GetClientInboundByEmail(email string) (traffic *xray.ClientTraffic, inbound *model.Inbound, err error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).Where("email = ?", email).Find(&traffics).Error
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with email %s: %v", email, err)
		return nil, nil, err
	}
	if len(traffics) > 0 {
		inbound, err = s.GetInbound(traffics[0].InboundId)
		return traffics[0], inbound, err
	}
	return nil, nil, nil
}

func (s *InboundService) GetClientByEmail(clientEmail string) (*xray.ClientTraffic, *model.Client, error) {
	traffic, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return nil, nil, err
	}
	if inbound == nil {
		return nil, nil, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	clients, err := s.GetClients(inbound)
	if err != nil {
		return nil, nil, err
	}

	for _, client := range clients {
		if client.Email == clientEmail {
			return traffic, &client, nil
		}
	}

	return nil, nil, common.NewError("Client Not Found In Inbound For Email:", clientEmail)
}

func (s *InboundService) SetClientTelegramUserID(trafficId int, tgId int64) (bool, error) {
	traffic, inbound, err := s.GetClientInboundByTrafficID(trafficId)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Traffic ID:", trafficId)
	}

	clientEmail := traffic.Email

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	clientId := ""

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			break
		}
	}

	if len(clientId) == 0 {
		return false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["tgId"] = tgId
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(modifiedSettings)
	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	return needRestart, err
}

func (s *InboundService) checkIsEnabledByEmail(clientEmail string) (bool, error) {
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	clients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	isEnable := false

	for _, client := range clients {
		if client.Email == clientEmail {
			isEnable = client.Enable
			break
		}
	}

	return isEnable, err
}

func (s *InboundService) ToggleClientEnableByEmail(clientEmail string) (bool, bool, error) {
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, false, err
	}
	if inbound == nil {
		return false, false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, false, err
	}

	clientId := ""
	clientOldEnabled := false

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			clientOldEnabled = oldClient.Enable
			break
		}
	}

	if len(clientId) == 0 {
		return false, false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["enable"] = !clientOldEnabled
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, false, err
	}
	inbound.Settings = string(modifiedSettings)

	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	if err != nil {
		return false, needRestart, err
	}

	return !clientOldEnabled, needRestart, nil
}

// SetClientEnableByEmail sets client enable state to desired value; returns (changed, needRestart, error)
func (s *InboundService) SetClientEnableByEmail(clientEmail string, enable bool) (bool, bool, error) {
	current, err := s.checkIsEnabledByEmail(clientEmail)
	if err != nil {
		return false, false, err
	}
	if current == enable {
		return false, false, nil
	}
	newEnabled, needRestart, err := s.ToggleClientEnableByEmail(clientEmail)
	if err != nil {
		return false, needRestart, err
	}
	return newEnabled == enable, needRestart, nil
}

func (s *InboundService) ResetClientIpLimitByEmail(clientEmail string, count int) (bool, error) {
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	clientId := ""

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			break
		}
	}

	if len(clientId) == 0 {
		return false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["limitIp"] = count
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(modifiedSettings)
	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	return needRestart, err
}

func (s *InboundService) ResetClientExpiryTimeByEmail(clientEmail string, expiry_time int64) (bool, error) {
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	clientId := ""

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			break
		}
	}

	if len(clientId) == 0 {
		return false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["expiryTime"] = expiry_time
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(modifiedSettings)
	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	return needRestart, err
}

func (s *InboundService) ResetClientTrafficLimitByEmail(clientEmail string, totalGB int) (bool, error) {
	if totalGB < 0 {
		return false, common.NewError("totalGB must be >= 0")
	}
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	clientId := ""

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			break
		}
	}

	if len(clientId) == 0 {
		return false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["totalGB"] = totalGB * 1024 * 1024 * 1024
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(modifiedSettings)
	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	return needRestart, err
}

func (s *InboundService) ResetClientTrafficByEmail(clientEmail string) error {
	db := database.GetDB()

	// Reset traffic stats in ClientTraffic table
	result := db.Model(xray.ClientTraffic{}).
		Where("email = ?", clientEmail).
		Updates(map[string]any{"enable": true, "up": 0, "down": 0})

	err := result.Error
	if err != nil {
		return err
	}

	return nil
}

func (s *InboundService) ResetClientTraffic(id int, clientEmail string) (bool, error) {
	needRestart := false

	traffic, err := s.GetClientTrafficByEmail(clientEmail)
	if err != nil {
		return false, err
	}

	if !traffic.Enable {
		inbound, err := s.GetInbound(id)
		if err != nil {
			return false, err
		}
		clients, err := s.GetClients(inbound)
		if err != nil {
			return false, err
		}
		for _, client := range clients {
			if client.Email == clientEmail && client.Enable {
				rt, rterr := s.runtimeFor(inbound)
				if rterr != nil {
					if inbound.NodeID != nil {
						return false, rterr
					}
					needRestart = true
					break
				}
				cipher := ""
				if string(inbound.Protocol) == "shadowsocks" {
					var oldSettings map[string]any
					err = json.Unmarshal([]byte(inbound.Settings), &oldSettings)
					if err != nil {
						return false, err
					}
					cipher = oldSettings["method"].(string)
				}
				err1 := rt.AddUser(context.Background(), inbound, map[string]any{
					"email":    client.Email,
					"id":       client.ID,
					"auth":     client.Auth,
					"security": client.Security,
					"flow":     client.Flow,
					"password": client.Password,
					"cipher":   cipher,
				})
				if err1 == nil {
					logger.Debug("Client enabled on", rt.Name(), "due to reset traffic:", clientEmail)
				} else {
					logger.Debug("Error in enabling client on", rt.Name(), ":", err1)
					needRestart = true
				}
				break
			}
		}
	}

	traffic.Up = 0
	traffic.Down = 0
	traffic.Enable = true

	db := database.GetDB()
	err = db.Save(traffic).Error
	if err != nil {
		return false, err
	}

	// Stamp last_traffic_reset_time on the parent inbound so the next
	// NodeTrafficSyncJob tick honours the grace window and doesn't pull
	// the pre-reset absolute back from the node.
	now := time.Now().UnixMilli()
	_ = db.Model(model.Inbound{}).
		Where("id = ?", id).
		Update("last_traffic_reset_time", now).Error

	// Propagate to the remote node if this inbound is node-managed.
	// Best-effort: an offline node shouldn't block a user-driven reset
	// — the central DB is already zeroed and the next successful sync
	// (within the grace window) will re-pull whatever the node has.
	inbound, err := s.GetInbound(id)
	if err == nil && inbound != nil && inbound.NodeID != nil {
		if rt, rterr := s.runtimeFor(inbound); rterr == nil {
			if e := rt.ResetClientTraffic(context.Background(), inbound, clientEmail); e != nil {
				logger.Warning("ResetClientTraffic: remote propagation to", rt.Name(), "failed:", e)
			}
		} else {
			logger.Warning("ResetClientTraffic: runtime lookup failed:", rterr)
		}
	}

	return needRestart, nil
}

func (s *InboundService) ResetAllClientTraffics(id int) error {
	db := database.GetDB()
	now := time.Now().Unix() * 1000

	if err := db.Transaction(func(tx *gorm.DB) error {
		whereText := "inbound_id "
		if id == -1 {
			whereText += " > ?"
		} else {
			whereText += " = ?"
		}

		// Reset client traffics
		result := tx.Model(xray.ClientTraffic{}).
			Where(whereText, id).
			Updates(map[string]any{"enable": true, "up": 0, "down": 0})

		if result.Error != nil {
			return result.Error
		}

		// Update lastTrafficResetTime for the inbound(s)
		inboundWhereText := "id "
		if id == -1 {
			inboundWhereText += " > ?"
		} else {
			inboundWhereText += " = ?"
		}

		result = tx.Model(model.Inbound{}).
			Where(inboundWhereText, id).
			Update("last_traffic_reset_time", now)

		return result.Error
	}); err != nil {
		return err
	}

	// Propagate to remote nodes after the central DB is settled. Single
	// inbound: one rt.ResetInboundClientTraffics call. id == -1 (all
	// inbounds across panel): walk every node-managed inbound and call
	// the per-inbound endpoint — there's no panel-wide endpoint that
	// only resets clients without zeroing inbound counters.
	var inbounds []model.Inbound
	q := db.Model(model.Inbound{}).Where("node_id IS NOT NULL")
	if id != -1 {
		q = q.Where("id = ?", id)
	}
	if err := q.Find(&inbounds).Error; err != nil {
		// Failed to discover which inbounds to propagate to — central
		// DB is already correct, log and move on.
		logger.Warning("ResetAllClientTraffics: discover node inbounds failed:", err)
		return nil
	}
	for i := range inbounds {
		ib := &inbounds[i]
		rt, rterr := s.runtimeFor(ib)
		if rterr != nil {
			logger.Warning("ResetAllClientTraffics: runtime lookup for inbound", ib.Id, "failed:", rterr)
			continue
		}
		if e := rt.ResetInboundClientTraffics(context.Background(), ib); e != nil {
			logger.Warning("ResetAllClientTraffics: remote propagation to", rt.Name(), "failed:", e)
		}
	}
	return nil
}

func (s *InboundService) ResetAllTraffics() error {
	db := database.GetDB()
	now := time.Now().UnixMilli()

	if err := db.Model(model.Inbound{}).
		Where("user_id > ?", 0).
		Updates(map[string]any{
			"up":                      0,
			"down":                    0,
			"last_traffic_reset_time": now,
		}).Error; err != nil {
		return err
	}

	// Propagate to every node that has at least one inbound on this
	// panel. We can't blanket-call rt.ResetAllTraffics because that
	// would also zero traffic for inbounds the node hosts but the
	// central panel doesn't know about — instead reset per inbound.
	var inbounds []model.Inbound
	if err := db.Model(model.Inbound{}).
		Where("node_id IS NOT NULL").
		Find(&inbounds).Error; err != nil {
		logger.Warning("ResetAllTraffics: discover node inbounds failed:", err)
		return nil
	}
	for i := range inbounds {
		ib := &inbounds[i]
		rt, rterr := s.runtimeFor(ib)
		if rterr != nil {
			logger.Warning("ResetAllTraffics: runtime lookup for inbound", ib.Id, "failed:", rterr)
			continue
		}
		if e := rt.ResetInboundClientTraffics(context.Background(), ib); e != nil {
			logger.Warning("ResetAllTraffics: remote propagation to", rt.Name(), "failed:", e)
		}
	}
	return nil
}

func (s *InboundService) ResetInboundTraffic(id int) error {
	db := database.GetDB()

	result := db.Model(model.Inbound{}).
		Where("id = ?", id).
		Updates(map[string]any{"up": 0, "down": 0})

	return result.Error
}

func (s *InboundService) DelDepletedClients(id int) (err error) {
	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	// Collect depleted emails globally — a shared-email row owned by one
	// inbound depletes every sibling that lists the email.
	now := time.Now().Unix() * 1000
	depletedClause := "reset = 0 and ((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?))"
	var depletedRows []xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).
		Where(depletedClause, now).
		Find(&depletedRows).Error
	if err != nil {
		return err
	}
	if len(depletedRows) == 0 {
		return nil
	}

	depletedEmails := make(map[string]struct{}, len(depletedRows))
	for _, r := range depletedRows {
		if r.Email == "" {
			continue
		}
		depletedEmails[strings.ToLower(r.Email)] = struct{}{}
	}
	if len(depletedEmails) == 0 {
		return nil
	}

	var inbounds []*model.Inbound
	inboundQuery := db.Model(model.Inbound{})
	if id >= 0 {
		inboundQuery = inboundQuery.Where("id = ?", id)
	}
	if err = inboundQuery.Find(&inbounds).Error; err != nil {
		return err
	}

	for _, inbound := range inbounds {
		var settings map[string]any
		if err = json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			return err
		}
		rawClients, ok := settings["clients"].([]any)
		if !ok {
			continue
		}
		newClients := make([]any, 0, len(rawClients))
		removed := 0
		for _, client := range rawClients {
			c, ok := client.(map[string]any)
			if !ok {
				newClients = append(newClients, client)
				continue
			}
			email, _ := c["email"].(string)
			if _, isDepleted := depletedEmails[strings.ToLower(email)]; isDepleted {
				removed++
				continue
			}
			newClients = append(newClients, client)
		}
		if removed == 0 {
			continue
		}
		if len(newClients) == 0 {
			s.DelInbound(inbound.Id)
			continue
		}
		settings["clients"] = newClients
		ns, mErr := json.MarshalIndent(settings, "", "  ")
		if mErr != nil {
			return mErr
		}
		inbound.Settings = string(ns)
		if err = tx.Save(inbound).Error; err != nil {
			return err
		}
	}

	// Drop now-orphaned rows. With id >= 0, a row is safe to drop only when
	// no out-of-scope inbound still references the email.
	if id < 0 {
		err = tx.Where(depletedClause, now).Delete(xray.ClientTraffic{}).Error
		return err
	}
	emails := make([]string, 0, len(depletedEmails))
	for e := range depletedEmails {
		emails = append(emails, e)
	}
	var stillReferenced []string
	if err = tx.Raw(`
		SELECT DISTINCT LOWER(JSON_EXTRACT(client.value, '$.email'))
		FROM inbounds,
			JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		WHERE LOWER(JSON_EXTRACT(client.value, '$.email')) IN ?
		`, emails).Scan(&stillReferenced).Error; err != nil {
		return err
	}
	stillSet := make(map[string]struct{}, len(stillReferenced))
	for _, e := range stillReferenced {
		stillSet[e] = struct{}{}
	}
	toDelete := make([]string, 0, len(emails))
	for _, e := range emails {
		if _, kept := stillSet[e]; !kept {
			toDelete = append(toDelete, e)
		}
	}
	if len(toDelete) > 0 {
		if err = tx.Where("LOWER(email) IN ?", toDelete).Delete(xray.ClientTraffic{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *InboundService) GetClientTrafficTgBot(tgId int64) ([]*xray.ClientTraffic, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound

	// Retrieve inbounds where settings contain the given tgId
	err := db.Model(model.Inbound{}).Where("settings LIKE ?", fmt.Sprintf(`%%"tgId": %d%%`, tgId)).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		logger.Errorf("Error retrieving inbounds with tgId %d: %v", tgId, err)
		return nil, err
	}

	var emails []string
	for _, inbound := range inbounds {
		clients, err := s.GetClients(inbound)
		if err != nil {
			logger.Errorf("Error retrieving clients for inbound %d: %v", inbound.Id, err)
			continue
		}
		for _, client := range clients {
			if client.TgID == tgId {
				emails = append(emails, client.Email)
			}
		}
	}

	// Chunked to stay under SQLite's bind-variable limit when a single Telegram
	// account owns thousands of clients across inbounds.
	uniqEmails := uniqueNonEmptyStrings(emails)
	traffics := make([]*xray.ClientTraffic, 0, len(uniqEmails))
	for _, batch := range chunkStrings(uniqEmails, sqliteMaxVars) {
		var page []*xray.ClientTraffic
		if err = db.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Find(&page).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			logger.Errorf("Error retrieving ClientTraffic for emails %v: %v", batch, err)
			return nil, err
		}
		traffics = append(traffics, page...)
	}
	if len(traffics) == 0 {
		logger.Warning("No ClientTraffic records found for emails:", emails)
		return nil, nil
	}

	// Populate UUID and other client data for each traffic record
	for i := range traffics {
		if ct, client, e := s.GetClientByEmail(traffics[i].Email); e == nil && ct != nil && client != nil {
			traffics[i].Enable = client.Enable
			traffics[i].UUID = client.ID
			traffics[i].SubId = client.SubID
		}
	}

	return traffics, nil
}

// sqliteMaxVars is a safe ceiling for the number of bind parameters in a
// single SQL statement. SQLite's SQLITE_MAX_VARIABLE_NUMBER is 999 on builds
// before 3.32 and 32766 after; staying under 999 keeps queries portable
// across forks/old binaries and also bounds per-query memory on truly large
// installs (>32k clients) where even modern SQLite would refuse a single IN.
const sqliteMaxVars = 900

// uniqueNonEmptyStrings returns a deduplicated copy of in with empty strings
// removed, preserving the order of first occurrence.
func uniqueNonEmptyStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// uniqueInts returns a deduplicated copy of in, preserving order of first occurrence.
func uniqueInts(in []int) []int {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(in))
	out := make([]int, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// chunkStrings splits s into consecutive sub-slices of at most size elements.
// Returns nil for an empty input or non-positive size.
func chunkStrings(s []string, size int) [][]string {
	if size <= 0 || len(s) == 0 {
		return nil
	}
	out := make([][]string, 0, (len(s)+size-1)/size)
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[i:end])
	}
	return out
}

// chunkInts splits s into consecutive sub-slices of at most size elements.
// Returns nil for an empty input or non-positive size.
func chunkInts(s []int, size int) [][]int {
	if size <= 0 || len(s) == 0 {
		return nil
	}
	out := make([][]int, 0, (len(s)+size-1)/size)
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[i:end])
	}
	return out
}

// GetActiveClientTraffics returns the absolute ClientTraffic rows for the given
// emails. Used by the WebSocket delta path to push per-client absolute
// counters without re-serializing the full inbound list. The query is chunked
// to stay under SQLite's bind-variable limit on very large active sets.
// Empty input returns (nil, nil).
func (s *InboundService) GetActiveClientTraffics(emails []string) ([]*xray.ClientTraffic, error) {
	uniq := uniqueNonEmptyStrings(emails)
	if len(uniq) == 0 {
		return nil, nil
	}
	db := database.GetDB()
	traffics := make([]*xray.ClientTraffic, 0, len(uniq))
	for _, batch := range chunkStrings(uniq, sqliteMaxVars) {
		var page []*xray.ClientTraffic
		if err := db.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Find(&page).Error; err != nil {
			return nil, err
		}
		traffics = append(traffics, page...)
	}
	return traffics, nil
}

// InboundTrafficSummary is the minimal projection of an inbound's traffic
// counters used by the WebSocket delta path. Excludes Settings/StreamSettings
// blobs so the broadcast stays compact even with many inbounds.
type InboundTrafficSummary struct {
	Id      int   `json:"id"`
	Up      int64 `json:"up"`
	Down    int64 `json:"down"`
	Total   int64 `json:"total"`
	AllTime int64 `json:"allTime"`
	Enable  bool  `json:"enable"`
}

// GetInboundsTrafficSummary returns inbound-level absolute traffic counters
// (no per-client expansion). Companion to GetActiveClientTraffics — together
// they replace the heavy "full inbound list" broadcast on each cron tick.
func (s *InboundService) GetInboundsTrafficSummary() ([]InboundTrafficSummary, error) {
	db := database.GetDB()
	var summaries []InboundTrafficSummary
	if err := db.Model(&model.Inbound{}).
		Select("id, up, down, total, all_time, enable").
		Find(&summaries).Error; err != nil {
		return nil, err
	}
	return summaries, nil
}

func (s *InboundService) GetClientTrafficByEmail(email string) (traffic *xray.ClientTraffic, err error) {
	// Prefer retrieving along with client to reflect actual enabled state from inbound settings
	t, client, err := s.GetClientByEmail(email)
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with email %s: %v", email, err)
		return nil, err
	}
	if t != nil && client != nil {
		t.UUID = client.ID
		t.SubId = client.SubID
		return t, nil
	}
	return nil, nil
}

func (s *InboundService) UpdateClientTrafficByEmail(email string, upload int64, download int64) error {
	db := database.GetDB()

	// Keep all_time monotonic: it represents historical cumulative usage and
	// must never be less than the currently-tracked up+down. Without this,
	// the UI showed "Общий трафик" (allTime) below the live consumed value
	// after admins manually edited a client's counters.
	result := db.Model(xray.ClientTraffic{}).
		Where("email = ?", email).
		Updates(map[string]any{
			"up":       upload,
			"down":     download,
			"all_time": gorm.Expr("CASE WHEN COALESCE(all_time, 0) < ? THEN ? ELSE all_time END", upload+download, upload+download),
		})

	err := result.Error
	if err != nil {
		logger.Warningf("Error updating ClientTraffic with email %s: %v", email, err)
		return err
	}
	return nil
}

func (s *InboundService) GetClientTrafficByID(id string) ([]xray.ClientTraffic, error) {
	db := database.GetDB()
	var traffics []xray.ClientTraffic

	err := db.Model(xray.ClientTraffic{}).Where(`email IN(
		SELECT JSON_EXTRACT(client.value, '$.email') as email
		FROM inbounds,
	  	JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		WHERE
	  	JSON_EXTRACT(client.value, '$.id') in (?)
		)`, id).Find(&traffics).Error

	if err != nil {
		logger.Debug(err)
		return nil, err
	}
	// Reconcile enable flag with client settings per email to avoid stale DB value
	for i := range traffics {
		if ct, client, e := s.GetClientByEmail(traffics[i].Email); e == nil && ct != nil && client != nil {
			traffics[i].Enable = client.Enable
			traffics[i].UUID = client.ID
			traffics[i].SubId = client.SubID
		}
	}
	return traffics, err
}

func (s *InboundService) SearchClientTraffic(query string) (traffic *xray.ClientTraffic, err error) {
	db := database.GetDB()
	inbound := &model.Inbound{}
	traffic = &xray.ClientTraffic{}

	// Search for inbound settings that contain the query
	err = db.Model(model.Inbound{}).Where("settings LIKE ?", "%\""+query+"\"%").First(inbound).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warningf("Inbound settings containing query %s not found: %v", query, err)
			return nil, err
		}
		logger.Errorf("Error searching for inbound settings with query %s: %v", query, err)
		return nil, err
	}

	traffic.InboundId = inbound.Id

	// Unmarshal settings to get clients
	settings := map[string][]model.Client{}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		logger.Errorf("Error unmarshalling inbound settings for inbound ID %d: %v", inbound.Id, err)
		return nil, err
	}

	clients := settings["clients"]
	for _, client := range clients {
		if (client.ID == query || client.Password == query) && client.Email != "" {
			traffic.Email = client.Email
			break
		}
	}

	if traffic.Email == "" {
		logger.Warningf("No client found with query %s in inbound ID %d", query, inbound.Id)
		return nil, gorm.ErrRecordNotFound
	}

	// Retrieve ClientTraffic based on the found email
	err = db.Model(xray.ClientTraffic{}).Where("email = ?", traffic.Email).First(traffic).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warningf("ClientTraffic for email %s not found: %v", traffic.Email, err)
			return nil, err
		}
		logger.Errorf("Error retrieving ClientTraffic for email %s: %v", traffic.Email, err)
		return nil, err
	}

	return traffic, nil
}

func (s *InboundService) GetInboundClientIps(clientEmail string) (string, error) {
	db := database.GetDB()
	InboundClientIps := &model.InboundClientIps{}
	err := db.Model(model.InboundClientIps{}).Where("client_email = ?", clientEmail).First(InboundClientIps).Error
	if err != nil {
		return "", err
	}

	if InboundClientIps.Ips == "" {
		return "", nil
	}

	// Try to parse as new format (with timestamps)
	type IPWithTimestamp struct {
		IP        string `json:"ip"`
		Timestamp int64  `json:"timestamp"`
	}

	var ipsWithTime []IPWithTimestamp
	err = json.Unmarshal([]byte(InboundClientIps.Ips), &ipsWithTime)

	// If successfully parsed as new format, return with timestamps
	if err == nil && len(ipsWithTime) > 0 {
		return InboundClientIps.Ips, nil
	}

	// Otherwise, assume it's old format (simple string array)
	// Try to parse as simple array and convert to new format
	var oldIps []string
	err = json.Unmarshal([]byte(InboundClientIps.Ips), &oldIps)
	if err == nil && len(oldIps) > 0 {
		// Convert old format to new format with current timestamp
		newIpsWithTime := make([]IPWithTimestamp, len(oldIps))
		for i, ip := range oldIps {
			newIpsWithTime[i] = IPWithTimestamp{
				IP:        ip,
				Timestamp: time.Now().Unix(),
			}
		}
		result, _ := json.Marshal(newIpsWithTime)
		return string(result), nil
	}

	// Return as-is if parsing fails
	return InboundClientIps.Ips, nil
}

func (s *InboundService) ClearClientIps(clientEmail string) error {
	db := database.GetDB()

	result := db.Model(model.InboundClientIps{}).
		Where("client_email = ?", clientEmail).
		Update("ips", "")
	err := result.Error
	if err != nil {
		return err
	}
	return nil
}

func (s *InboundService) SearchInbounds(query string) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Where("remark like ?", "%"+query+"%").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) MigrationRequirements() {
	db := database.GetDB()
	tx := db.Begin()
	var err error
	defer func() {
		if err == nil {
			tx.Commit()
			if dbErr := db.Exec(`VACUUM "main"`).Error; dbErr != nil {
				logger.Warningf("VACUUM failed: %v", dbErr)
			}
		} else {
			tx.Rollback()
		}
	}()

	// Calculate and backfill all_time from up+down for inbounds and clients
	err = tx.Exec(`
		UPDATE inbounds
		SET all_time = IFNULL(up, 0) + IFNULL(down, 0)
		WHERE IFNULL(all_time, 0) = 0 AND (IFNULL(up, 0) + IFNULL(down, 0)) > 0
	`).Error
	if err != nil {
		return
	}
	err = tx.Exec(`
		UPDATE client_traffics
		SET all_time = IFNULL(up, 0) + IFNULL(down, 0)
		WHERE IFNULL(all_time, 0) = 0 AND (IFNULL(up, 0) + IFNULL(down, 0)) > 0
	`).Error

	if err != nil {
		return
	}

	// Fix inbounds based problems
	var inbounds []*model.Inbound
	err = tx.Model(model.Inbound{}).Where("protocol IN (?)", []string{"vmess", "vless", "trojan"}).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return
	}
	for inbound_index := range inbounds {
		settings := map[string]any{}
		json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
		clients, ok := settings["clients"].([]any)
		if ok {
			// Fix Client configuration problems
			var newClients []any
			hasVisionFlow := false
			for client_index := range clients {
				c := clients[client_index].(map[string]any)

				// Add email='' if it is not exists
				if _, ok := c["email"]; !ok {
					c["email"] = ""
				}

				// Convert string tgId to int64
				if _, ok := c["tgId"]; ok {
					var tgId any = c["tgId"]
					if tgIdStr, ok2 := tgId.(string); ok2 {
						tgIdInt64, err := strconv.ParseInt(strings.ReplaceAll(tgIdStr, " ", ""), 10, 64)
						if err == nil {
							c["tgId"] = tgIdInt64
						}
					}
				}

				// Remove "flow": "xtls-rprx-direct"
				if _, ok := c["flow"]; ok {
					if c["flow"] == "xtls-rprx-direct" {
						c["flow"] = ""
					}
				}
				if flow, _ := c["flow"].(string); flow == "xtls-rprx-vision" {
					hasVisionFlow = true
				}
				// Backfill created_at and updated_at
				if _, ok := c["created_at"]; !ok {
					c["created_at"] = time.Now().Unix() * 1000
				}
				c["updated_at"] = time.Now().Unix() * 1000
				newClients = append(newClients, any(c))
			}
			settings["clients"] = newClients

			// Drop orphaned testseed: VLESS-only field, only meaningful when at least
			// one client uses the exact xtls-rprx-vision flow. Older versions saved it
			// for any non-empty flow (including the UDP variant) or kept it after the
			// flow was cleared from the client modal — clean those up here.
			if inbounds[inbound_index].Protocol == model.VLESS && !hasVisionFlow {
				delete(settings, "testseed")
			}

			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return
			}

			inbounds[inbound_index].Settings = string(modifiedSettings)
		}

		// Add client traffic row for all clients which has email
		modelClients, err := s.GetClients(inbounds[inbound_index])
		if err != nil {
			return
		}
		for _, modelClient := range modelClients {
			if len(modelClient.Email) > 0 {
				var count int64
				tx.Model(xray.ClientTraffic{}).Where("email = ?", modelClient.Email).Count(&count)
				if count == 0 {
					s.AddClientStat(tx, inbounds[inbound_index].Id, &modelClient)
				}
			}
		}
	}
	tx.Save(inbounds)

	// Remove orphaned traffics
	tx.Where("inbound_id = 0").Delete(xray.ClientTraffic{})

	// Migrate old MultiDomain to External Proxy
	var externalProxy []struct {
		Id             int
		Port           int
		StreamSettings []byte
	}
	err = tx.Raw(`select id, port, stream_settings
	from inbounds
	WHERE protocol in ('vmess','vless','trojan')
	  AND json_extract(stream_settings, '$.security') = 'tls'
	  AND json_extract(stream_settings, '$.tlsSettings.settings.domains') IS NOT NULL`).Scan(&externalProxy).Error
	if err != nil || len(externalProxy) == 0 {
		return
	}

	for _, ep := range externalProxy {
		var reverses any
		var stream map[string]any
		json.Unmarshal(ep.StreamSettings, &stream)
		if tlsSettings, ok := stream["tlsSettings"].(map[string]any); ok {
			if settings, ok := tlsSettings["settings"].(map[string]any); ok {
				if domains, ok := settings["domains"].([]any); ok {
					for _, domain := range domains {
						if domainMap, ok := domain.(map[string]any); ok {
							domainMap["forceTls"] = "same"
							domainMap["port"] = ep.Port
							domainMap["dest"] = domainMap["domain"].(string)
							delete(domainMap, "domain")
						}
					}
				}
				reverses = settings["domains"]
				delete(settings, "domains")
			}
		}
		stream["externalProxy"] = reverses
		newStream, _ := json.MarshalIndent(stream, " ", "  ")
		tx.Model(model.Inbound{}).Where("id = ?", ep.Id).Update("stream_settings", newStream)
	}

	err = tx.Raw(`UPDATE inbounds
	SET tag = REPLACE(tag, '0.0.0.0:', '')
	WHERE INSTR(tag, '0.0.0.0:') > 0;`).Error
	if err != nil {
		return
	}
}

func (s *InboundService) MigrateDB() {
	s.MigrationRequirements()
	s.MigrationRemoveOrphanedTraffics()
}

func (s *InboundService) GetOnlineClients() []string {
	return p.GetOnlineClients()
}

// SetNodeOnlineClients records a remote node's online-clients list on
// the panel-wide xray.Process so GetOnlineClients returns the union of
// local + every node's contribution. Called by NodeTrafficSyncJob.
func (s *InboundService) SetNodeOnlineClients(nodeID int, emails []string) {
	if p != nil {
		p.SetNodeOnlineClients(nodeID, emails)
	}
}

// ClearNodeOnlineClients drops one node's contribution to the online
// set. Used when the per-node sync probe fails so a downed node
// doesn't keep its clients listed as online forever.
func (s *InboundService) ClearNodeOnlineClients(nodeID int) {
	if p != nil {
		p.ClearNodeOnlineClients(nodeID)
	}
}

func (s *InboundService) GetClientsLastOnline() (map[string]int64, error) {
	db := database.GetDB()
	var rows []xray.ClientTraffic
	err := db.Model(&xray.ClientTraffic{}).Select("email, last_online").Find(&rows).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, r := range rows {
		result[r.Email] = r.LastOnline
	}
	return result, nil
}

func (s *InboundService) FilterAndSortClientEmails(emails []string) ([]string, []string, error) {
	db := database.GetDB()

	// Step 1: Get ClientTraffic records for emails in the input list.
	// Chunked to stay under SQLite's bind-variable limit on huge inputs.
	uniqEmails := uniqueNonEmptyStrings(emails)
	clients := make([]xray.ClientTraffic, 0, len(uniqEmails))
	for _, batch := range chunkStrings(uniqEmails, sqliteMaxVars) {
		var page []xray.ClientTraffic
		if err := db.Where("email IN ?", batch).Find(&page).Error; err != nil && err != gorm.ErrRecordNotFound {
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
func (s *InboundService) DelInboundClientByEmail(inboundId int, email string) (bool, error) {
	oldInbound, err := s.GetInbound(inboundId)
	if err != nil {
		logger.Error("Load Old Data Error")
		return false, err
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(oldInbound.Settings), &settings); err != nil {
		return false, err
	}

	interfaceClients, ok := settings["clients"].([]any)
	if !ok {
		return false, common.NewError("invalid clients format in inbound settings")
	}

	var newClients []any
	needApiDel := false
	found := false

	for _, client := range interfaceClients {
		c, ok := client.(map[string]any)
		if !ok {
			continue
		}
		if cEmail, ok := c["email"].(string); ok && cEmail == email {
			// matched client, drop it
			found = true
			needApiDel, _ = c["enable"].(bool)
		} else {
			newClients = append(newClients, client)
		}
	}

	if !found {
		return false, common.NewError(fmt.Sprintf("client with email %s not found", email))
	}
	if len(newClients) == 0 {
		return false, common.NewError("no client remained in Inbound")
	}

	settings["clients"] = newClients
	newSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}

	oldInbound.Settings = string(newSettings)

	db := database.GetDB()

	// Drop the row and IPs only when this was the last inbound referencing
	// the email — siblings still need the shared accounting state.
	emailShared, err := s.emailUsedByOtherInbounds(email, inboundId)
	if err != nil {
		return false, err
	}

	if !emailShared {
		if err := s.DelClientIPs(db, email); err != nil {
			logger.Error("Error in delete client IPs")
			return false, err
		}
	}

	needRestart := false

	// remove stats too
	if len(email) > 0 && !emailShared {
		traffic, err := s.GetClientTrafficByEmail(email)
		if err != nil {
			return false, err
		}
		if traffic != nil {
			if err := s.DelClientStat(db, email); err != nil {
				logger.Error("Delete stats Data Error")
				return false, err
			}
		}

		if needApiDel {
			rt, rterr := s.runtimeFor(oldInbound)
			if rterr != nil {
				if oldInbound.NodeID != nil {
					return false, rterr
				}
				needRestart = true
			} else if oldInbound.NodeID == nil {
				if err1 := rt.RemoveUser(context.Background(), oldInbound, email); err1 == nil {
					logger.Debug("Client deleted on", rt.Name(), ":", email)
					needRestart = false
				} else if strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", email)) {
					logger.Debug("User is already deleted. Nothing to do more...")
				} else {
					logger.Debug("Error in deleting client on", rt.Name(), ":", err1)
					needRestart = true
				}
			} else {
				if err1 := rt.UpdateInbound(context.Background(), oldInbound, oldInbound); err1 != nil {
					return false, err1
				}
			}
		}
	}

	return needRestart, db.Save(oldInbound).Error
}
