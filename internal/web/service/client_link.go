package service

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// applyClientRecordMerge merges incoming client-record fields onto row using the
// same rules everywhere a client record is persisted: scalar quota / lifecycle /
// subscription fields are applied unconditionally (so clearing them takes
// effect), while credentials and identifiers are only overwritten when the
// incoming value is non-empty (so a partial update preserves the stored UUID /
// password / keys). CreatedAt keeps the earliest known value. Email, UpdatedAt,
// and the Id primary key are intentionally not touched here — callers handle
// those separately. Shared by SyncInbound (per-inbound persistence) and Update
// (the no-attached-inbound fallback) so the two paths cannot diverge.
func applyClientRecordMerge(row *model.ClientRecord, incoming *model.ClientRecord) {
	if incoming.UUID != "" {
		row.UUID = incoming.UUID
	}
	if incoming.Password != "" {
		row.Password = incoming.Password
	}
	if incoming.Auth != "" {
		row.Auth = incoming.Auth
	}
	if incoming.Secret != "" {
		row.Secret = incoming.Secret
	}
	if incoming.AdTag != "" {
		row.AdTag = incoming.AdTag
	}
	row.Flow = incoming.Flow
	if incoming.Security != "" {
		row.Security = incoming.Security
	}
	// nil preserves the stored binding, empty clears it via Update's write.
	if ids := model.NormalizeClientHostRuleIds(incoming.ClientHostRuleIds); ids != nil {
		row.ClientHostRuleIds = ids
	}
	if incoming.Reverse != "" {
		row.Reverse = incoming.Reverse
	}
	if incoming.PrivateKey != "" {
		row.PrivateKey = incoming.PrivateKey
	}
	if incoming.PublicKey != "" {
		row.PublicKey = incoming.PublicKey
	}
	if incoming.AllowedIPs != "" {
		row.AllowedIPs = incoming.AllowedIPs
	}
	if incoming.WireguardDevices != nil {
		row.WireguardDevices = model.CloneWireguardDevices(incoming.WireguardDevices)
	}
	if incoming.WgDeviceLimit > 0 {
		row.WgDeviceLimit = incoming.WgDeviceLimit
	}
	row.PreSharedKey = incoming.PreSharedKey
	if incoming.KeepAlive != "" {
		row.KeepAlive = incoming.KeepAlive
	}
	// Guarded like Group: only an AmneziaWG entry is sure to carry the spec, and the record
	// is shared with every other inbound of the client.
	if incoming.ForwardedPorts != "" {
		row.ForwardedPorts = incoming.ForwardedPorts
	}
	row.SubID = incoming.SubID
	row.LimitIP = incoming.LimitIP
	row.TotalGB = incoming.TotalGB
	row.ExpiryTime = incoming.ExpiryTime
	row.Enable = incoming.Enable
	row.TgID = incoming.TgID
	if incoming.Group != "" {
		row.Group = incoming.Group
	}
	row.Comment = incoming.Comment
	row.Reset = incoming.Reset
	row.ResetDay = incoming.ResetDay
	row.ResetWeekday = incoming.ResetWeekday
	row.ResetMax = incoming.ResetMax
	// Guarded like Group and AdTag: a node snapshot rebuilt from settings that
	// predate the cycle would otherwise silently erase it.
	if incoming.TrafficReset != "" {
		row.TrafficReset = incoming.TrafficReset
	}
	if incoming.TrafficResetDay > 0 {
		row.TrafficResetDay = incoming.TrafficResetDay
	}
	if incoming.CreatedAt > 0 && (row.CreatedAt == 0 || incoming.CreatedAt < row.CreatedAt) {
		row.CreatedAt = incoming.CreatedAt
	}
}

// SyncInbound makes the inbound's client records and links match clients
// exactly: links for clients no longer in the set are removed.
func (s *ClientService) SyncInbound(tx *gorm.DB, inboundId int, clients []model.Client) error {
	return s.syncInboundClients(tx, inboundId, clients, nil, true)
}

type inboundClientDelta struct {
	inboundId    int
	protocol     model.Protocol
	nodeID       *int
	hasMeta      bool
	changed      []model.Client
	detachEmails []string
}

// ApplyInboundClientDelta persists only the clients an edit actually changed
// plus the emails it detached, leaving every other link on the inbound alone —
// the whole point being that a one-client edit must not rewrite the inbound's
// entire membership set (#6252).
func (s *ClientService) ApplyInboundClientDelta(tx *gorm.DB, inboundId int, changed []model.Client, detachEmails []string) error {
	return s.syncInboundClients(tx, inboundId, changed, detachEmails, false)
}

// ApplyMultiInboundClientDelta applies per-inbound deltas across multiple inbounds
// in one batched pass over clients, client_inbounds, and flow settlement.
func (s *ClientService) ApplyMultiInboundClientDelta(tx *gorm.DB, deltas []inboundClientDelta) error {
	if len(deltas) == 0 {
		return nil
	}
	if len(deltas) == 1 {
		return s.syncInboundClients(tx, deltas[0].inboundId, deltas[0].changed, deltas[0].detachEmails, false)
	}
	if tx == nil {
		tx = database.GetDB()
	}

	type inboundMeta struct {
		protocol model.Protocol
		nodeID   *int
	}
	metaByInbound := make(map[int]inboundMeta, len(deltas))
	var missingMetaIDs []int
	groupNames := make(map[string]string)
	for i := range deltas {
		if err := validateClientsRenewal(deltas[i].changed); err != nil {
			return err
		}
		for j := range deltas[i].changed {
			name := strings.TrimSpace(deltas[i].changed[j].Group)
			if name == "" {
				continue
			}
			canonical, ok := groupNames[name]
			if !ok {
				group, err := database.EnsureClientGroup(tx, name)
				if err != nil {
					return err
				}
				canonical = group.Name
				groupNames[name] = canonical
			}
			deltas[i].changed[j].Group = canonical
		}
		if deltas[i].hasMeta {
			metaByInbound[deltas[i].inboundId] = inboundMeta{
				protocol: deltas[i].protocol,
				nodeID:   deltas[i].nodeID,
			}
		} else if _, ok := metaByInbound[deltas[i].inboundId]; !ok {
			missingMetaIDs = append(missingMetaIDs, deltas[i].inboundId)
		}
	}
	if len(missingMetaIDs) > 0 {
		for _, batch := range chunkInts(uniqueInts(missingMetaIDs), sqlInChunk) {
			var rows []struct {
				Id       int
				Protocol model.Protocol
				NodeID   *int
			}
			if err := tx.Model(&model.Inbound{}).Select("id, protocol, node_id").Where("id IN ?", batch).Find(&rows).Error; err != nil {
				return err
			}
			for _, r := range rows {
				metaByInbound[r.Id] = inboundMeta{protocol: r.Protocol, nodeID: r.NodeID}
			}
		}
	}

	anyTUIC := false
	for i := range deltas {
		meta, ok := metaByInbound[deltas[i].inboundId]
		if !ok {
			return gorm.ErrRecordNotFound
		}
		if meta.protocol == model.TUIC {
			anyTUIC = true
		}
		if meta.protocol == model.TUIC || meta.protocol == model.MASQUE {
			if err := s.validateProtocolClients(tx, meta.protocol, deltas[i].inboundId, deltas[i].changed, deltas[i].detachEmails, false); err != nil {
				return err
			}
		}
	}

	changedEmailSet := make(map[string]struct{})
	changedEmails := make([]string, 0)
	detachEmailSet := make(map[string]struct{})
	detachEmailsAll := make([]string, 0)
	for i := range deltas {
		for j := range deltas[i].changed {
			em := strings.TrimSpace(deltas[i].changed[j].Email)
			if em == "" {
				continue
			}
			if _, ok := changedEmailSet[em]; !ok {
				changedEmailSet[em] = struct{}{}
				changedEmails = append(changedEmails, em)
			}
		}
		for _, rawEm := range deltas[i].detachEmails {
			em := strings.TrimSpace(rawEm)
			if em == "" {
				continue
			}
			if _, ok := detachEmailSet[em]; !ok {
				detachEmailSet[em] = struct{}{}
				detachEmailsAll = append(detachEmailsAll, em)
			}
		}
	}

	existing := make(map[string]*model.ClientRecord, len(changedEmails))
	const selectChunk = 400
	for start := 0; start < len(changedEmails); start += selectChunk {
		end := min(start+selectChunk, len(changedEmails))
		var rows []model.ClientRecord
		if err := tx.Where("email IN ?", changedEmails[start:end]).Find(&rows).Error; err != nil {
			return err
		}
		for i := range rows {
			r := rows[i]
			existing[r.Email] = &r
		}
	}

	existingIDs := make([]int, 0, len(existing))
	for _, row := range existing {
		existingIDs = append(existingIDs, row.Id)
	}
	onLowerTunnelByInbound := make(map[int]map[int]struct{})
	onLowerWgByInbound := make(map[int]map[int]struct{})
	if len(existingIDs) > 0 {
		for i := range deltas {
			meta := metaByInbound[deltas[i].inboundId]
			if meta.protocol != model.WireGuard && meta.protocol != model.AmneziaWG {
				continue
			}
			if _, done := onLowerTunnelByInbound[deltas[i].inboundId]; done {
				continue
			}
			m, err := clientsOnLowerTunnel(tx, deltas[i].inboundId, meta.nodeID, existingIDs, []model.Protocol{model.WireGuard, model.AmneziaWG})
			if err != nil {
				return err
			}
			onLowerTunnelByInbound[deltas[i].inboundId] = m
			if meta.protocol == model.WireGuard {
				wm, err := clientsOnLowerTunnel(tx, deltas[i].inboundId, meta.nodeID, existingIDs, []model.Protocol{model.WireGuard})
				if err != nil {
					return err
				}
				onLowerWgByInbound[deltas[i].inboundId] = wm
			}
		}
	}

	beforeByID := make(map[int]model.ClientRecord, len(existing))
	for _, row := range existing {
		beforeByID[row.Id] = *row
	}

	idByEmail := make(map[string]int, len(changedEmails)+len(detachEmailsAll))
	pending := make(map[string]*model.ClientRecord, len(changedEmails))
	toCreate := make([]*model.ClientRecord, 0)
	dirtyRecords := make(map[int]*model.ClientRecord)

	for i := range deltas {
		meta := metaByInbound[deltas[i].inboundId]
		isTunnel := meta.protocol == model.WireGuard || meta.protocol == model.AmneziaWG
		isWG := meta.protocol == model.WireGuard
		onLowerTunnel := onLowerTunnelByInbound[deltas[i].inboundId]
		onLowerWG := onLowerWgByInbound[deltas[i].inboundId]
		for j := range deltas[i].changed {
			email := strings.TrimSpace(deltas[i].changed[j].Email)
			if email == "" {
				continue
			}
			incoming := deltas[i].changed[j].ToRecord()
			incoming.Email = email
			incoming.ClientHostRuleIds = model.NormalizeClientHostRuleIds(incoming.ClientHostRuleIds)
			row, ok := existing[email]
			if !ok {
				if p, exists := pending[email]; !exists {
					pending[email] = incoming
					toCreate = append(toCreate, incoming)
				} else {
					pBefore := *p
					applyClientRecordMerge(p, incoming)
					keepLinkOwnedFields(p, &pBefore, isTunnel, isWG)
					p.UpdatedAt = max(incoming.UpdatedAt, p.UpdatedAt)
				}
				continue
			}
			beforeMerge := *row
			applyClientRecordMerge(row, incoming)
			_, outranked := onLowerTunnel[row.Id]
			_, wgOutranked := onLowerWG[row.Id]
			keepLinkOwnedFields(row, &beforeMerge, isTunnel && !outranked, isWG && !wgOutranked)
			preservedUpdatedAt := max(incoming.UpdatedAt, row.UpdatedAt)
			row.UpdatedAt = preservedUpdatedAt
			idByEmail[email] = row.Id
			if !reflect.DeepEqual(*row, beforeByID[row.Id]) {
				dirtyRecords[row.Id] = row
			}
		}
	}

	for _, row := range dirtyRecords {
		preservedUpdatedAt := row.UpdatedAt
		if err := tx.Save(row).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ClientRecord{}).
			Where("id = ?", row.Id).
			UpdateColumn("updated_at", preservedUpdatedAt).Error; err != nil {
			return err
		}
	}

	if len(toCreate) > 0 {
		wantEnable := make([]bool, len(toCreate))
		for i, rec := range toCreate {
			wantEnable[i] = rec.Enable
		}
		if err := tx.CreateInBatches(toCreate, 200).Error; err != nil {
			return err
		}
		liftTombstonesOnCommit(tx, toCreate)
		disabledIDs := make([]int, 0)
		for i, rec := range toCreate {
			idByEmail[rec.Email] = rec.Id
			if !wantEnable[i] {
				disabledIDs = append(disabledIDs, rec.Id)
			}
		}
		for _, batch := range chunkInts(disabledIDs, sqlInChunk) {
			if err := tx.Model(&model.ClientRecord{}).Where("id IN ?", batch).
				UpdateColumn("enable", false).Error; err != nil {
				return err
			}
		}
	}

	var missingDetach []string
	for _, em := range detachEmailsAll {
		if _, ok := idByEmail[em]; !ok {
			missingDetach = append(missingDetach, em)
		}
	}
	for _, batch := range chunkStrings(missingDetach, sqlInChunk) {
		var rows []struct {
			Id    int
			Email string
		}
		if err := tx.Model(&model.ClientRecord{}).Select("id, email").Where("email IN ?", batch).Find(&rows).Error; err != nil {
			return err
		}
		for _, r := range rows {
			idByEmail[r.Email] = r.Id
		}
	}

	type deltaWanted struct {
		inboundId  int
		wantedFlow map[int]string
		wantedIds  []int
		detachIds  []int
	}
	perDelta := make([]deltaWanted, len(deltas))
	allInboundIDs := make([]int, 0, len(deltas))
	allClientIDSet := make(map[int]struct{})
	allWantedIDs := make([]int, 0)
	seenWantedGlobal := make(map[int]struct{})

	for i := range deltas {
		allInboundIDs = append(allInboundIDs, deltas[i].inboundId)
		wf := make(map[int]string, len(deltas[i].changed))
		wIds := make([]int, 0, len(deltas[i].changed))
		for j := range deltas[i].changed {
			email := strings.TrimSpace(deltas[i].changed[j].Email)
			if email == "" {
				continue
			}
			id, ok := idByEmail[email]
			if !ok {
				continue
			}
			if _, dup := wf[id]; dup {
				continue
			}
			wf[id] = deltas[i].changed[j].Flow
			wIds = append(wIds, id)
			allClientIDSet[id] = struct{}{}
			if _, g := seenWantedGlobal[id]; !g {
				seenWantedGlobal[id] = struct{}{}
				allWantedIDs = append(allWantedIDs, id)
			}
		}
		var dIds []int
		for _, rawEm := range deltas[i].detachEmails {
			em := strings.TrimSpace(rawEm)
			if em == "" {
				continue
			}
			if id, ok := idByEmail[em]; ok {
				if _, keep := wf[id]; !keep {
					dIds = append(dIds, id)
				}
			}
		}
		perDelta[i] = deltaWanted{
			inboundId:  deltas[i].inboundId,
			wantedFlow: wf,
			wantedIds:  wIds,
			detachIds:  dIds,
		}
	}

	uniqueInboundIDs := uniqueInts(allInboundIDs)
	allClientIDs := make([]int, 0, len(allClientIDSet))
	for id := range allClientIDSet {
		allClientIDs = append(allClientIDs, id)
	}

	type linkKey struct {
		inboundId int
		clientId  int
	}
	existingLinks := make(map[linkKey]string, len(allClientIDs)*len(uniqueInboundIDs))
	if len(allClientIDs) > 0 && len(uniqueInboundIDs) > 0 {
		for _, ibBatch := range chunkInts(uniqueInboundIDs, sqlInChunk) {
			clientChunk := max(1, sqlInChunk-len(ibBatch))
			for _, clBatch := range chunkInts(allClientIDs, clientChunk) {
				var rows []model.ClientInbound
				if err := tx.Where("inbound_id IN ? AND client_id IN ?", ibBatch, clBatch).Find(&rows).Error; err != nil {
					return err
				}
				for _, r := range rows {
					existingLinks[linkKey{inboundId: r.InboundId, clientId: r.ClientId}] = r.FlowOverride
				}
			}
		}
	}

	var toInsert []model.ClientInbound
	toUpdateByFlow := make(map[string]map[int][]int)
	toDeleteByInbound := make(map[int][]int)
	relinkedSet := make(map[int]struct{})

	for i := range perDelta {
		dw := &perDelta[i]
		for _, cid := range dw.detachIds {
			toDeleteByInbound[dw.inboundId] = append(toDeleteByInbound[dw.inboundId], cid)
			relinkedSet[cid] = struct{}{}
		}
		for _, cid := range dw.wantedIds {
			flow := dw.wantedFlow[cid]
			curFlow, exists := existingLinks[linkKey{inboundId: dw.inboundId, clientId: cid}]
			if !exists {
				toInsert = append(toInsert, model.ClientInbound{
					ClientId:     cid,
					InboundId:    dw.inboundId,
					FlowOverride: flow,
				})
				existingLinks[linkKey{inboundId: dw.inboundId, clientId: cid}] = flow
				relinkedSet[cid] = struct{}{}
			} else if curFlow != flow {
				byIb := toUpdateByFlow[flow]
				if byIb == nil {
					byIb = make(map[int][]int)
					toUpdateByFlow[flow] = byIb
				}
				byIb[dw.inboundId] = append(byIb[dw.inboundId], cid)
				existingLinks[linkKey{inboundId: dw.inboundId, clientId: cid}] = flow
				relinkedSet[cid] = struct{}{}
			}
		}
	}

	for ibId, cids := range toDeleteByInbound {
		for _, batch := range chunkInts(uniqueInts(cids), sqlInChunk) {
			if err := tx.Where("inbound_id = ? AND client_id IN ?", ibId, batch).
				Delete(&model.ClientInbound{}).Error; err != nil {
				return err
			}
		}
	}

	for flow, byIb := range toUpdateByFlow {
		for ibId, cids := range byIb {
			for _, batch := range chunkInts(uniqueInts(cids), sqlInChunk) {
				if err := tx.Model(&model.ClientInbound{}).
					Where("inbound_id = ? AND client_id IN ?", ibId, batch).
					Update("flow_override", flow).Error; err != nil {
					return err
				}
			}
		}
	}

	if len(toInsert) > 0 {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "client_id"}, {Name: "inbound_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"flow_override"}),
		}).CreateInBatches(toInsert, 200).Error; err != nil {
			return err
		}
		linked := make([]int, 0, len(toInsert))
		for _, link := range toInsert {
			linked = append(linked, link.ClientId)
		}
		for _, batch := range chunkInts(uniqueInts(linked), sqlInChunk) {
			if err := tx.Model(&model.ClientRecord{}).Where("id IN ? AND sync_orphaned_at > 0", batch).
				Updates(map[string]any{"sync_orphaned_at": 0, "sync_orphaned_by": 0}).Error; err != nil {
				return err
			}
		}
	}

	if len(relinkedSet) > 0 {
		relinked := make([]int, 0, len(relinkedSet))
		for cid := range relinkedSet {
			relinked = append(relinked, cid)
		}
		if err := settleRecordFlows(tx, relinked); err != nil {
			return err
		}
	}

	return checkTuicDuplicateUUIDs(tx, uniqueInboundIDs, allWantedIDs, anyTUIC)
}

func checkTuicDuplicateUUIDs(tx *gorm.DB, inboundIDs []int, clientIDs []int, anyTUIC bool) error {
	if !anyTUIC && len(clientIDs) == 0 {
		return nil
	}
	if !anyTUIC {
		var hasTUIC int
		if err := tx.Model(&model.Inbound{}).Select("1").Where("protocol = ?", model.TUIC).Limit(1).Scan(&hasTUIC).Error; err != nil {
			return err
		}
		if hasTUIC != 1 {
			return nil
		}
	}
	affected := make(map[int]struct{}, len(inboundIDs))
	for _, id := range inboundIDs {
		affected[id] = struct{}{}
	}
	for _, ids := range chunkInts(clientIDs, sqlInChunk) {
		var linkedIDs []int
		if err := tx.Model(&model.ClientInbound{}).Distinct("inbound_id").Where("client_id IN ?", ids).Pluck("inbound_id", &linkedIDs).Error; err != nil {
			return err
		}
		for _, id := range linkedIDs {
			affected[id] = struct{}{}
		}
	}
	affectedIDs := make([]int, 0, len(affected))
	for id := range affected {
		affectedIDs = append(affectedIDs, id)
	}
	for _, ids := range chunkInts(affectedIDs, sqlInChunk) {
		var duplicates int64
		err := tx.Raw(`SELECT COUNT(*) FROM (
            SELECT ci.inbound_id, LOWER(c.uuid) FROM clients c
            JOIN client_inbounds ci ON ci.client_id = c.id
            JOIN inbounds i ON i.id = ci.inbound_id
            WHERE i.protocol = ? AND c.uuid <> '' AND ci.inbound_id IN ?
            GROUP BY ci.inbound_id, LOWER(c.uuid) HAVING COUNT(*) > 1
        ) AS duplicate_uuids`, model.TUIC, ids).Scan(&duplicates).Error
		if err != nil {
			return err
		}
		if duplicates > 0 {
			return fmt.Errorf("TUIC: duplicate client UUID within inbound")
		}
	}
	return nil
}

func (s *ClientService) syncInboundClients(tx *gorm.DB, inboundId int, clients []model.Client, detachEmails []string, prune bool) error {
	if err := validateClientsRenewal(clients); err != nil {
		return err
	}
	if tx == nil {
		tx = database.GetDB()
	}

	groupNames := make(map[string]string)
	for i := range clients {
		name := strings.TrimSpace(clients[i].Group)
		if name == "" {
			continue
		}
		canonical, ok := groupNames[name]
		if !ok {
			group, err := database.EnsureClientGroup(tx, name)
			if err != nil {
				return err
			}
			canonical = group.Name
			groupNames[name] = canonical
		}
		clients[i].Group = canonical
	}

	var inbound model.Inbound
	if err := tx.Select("protocol", "node_id").Where("id = ?", inboundId).Take(&inbound).Error; err != nil {
		return err
	}
	if err := s.validateProtocolClients(tx, inbound.Protocol, inboundId, clients, detachEmails, prune); err != nil {
		return err
	}

	emails := make([]string, 0, len(clients))
	seen := make(map[string]struct{}, len(clients))
	for i := range clients {
		email := strings.TrimSpace(clients[i].Email)
		if email == "" {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		emails = append(emails, email)
	}

	existing := make(map[string]*model.ClientRecord, len(emails))
	const selectChunk = 400
	for start := 0; start < len(emails); start += selectChunk {
		end := min(start+selectChunk, len(emails))
		var rows []model.ClientRecord
		if err := tx.Where("email IN ?", emails[start:end]).Find(&rows).Error; err != nil {
			return err
		}
		for i := range rows {
			r := rows[i]
			existing[r.Email] = &r
		}
	}

	isTunnel := inbound.Protocol == model.WireGuard || inbound.Protocol == model.AmneziaWG
	var onLowerTunnel, onLowerWireguard map[int]struct{}
	if isTunnel && len(existing) > 0 {
		ids := make([]int, 0, len(existing))
		for _, row := range existing {
			ids = append(ids, row.Id)
		}
		var err error
		if onLowerTunnel, err = clientsOnLowerTunnel(tx, inboundId, inbound.NodeID, ids, []model.Protocol{model.WireGuard, model.AmneziaWG}); err != nil {
			return err
		}
		if inbound.Protocol == model.WireGuard {
			if onLowerWireguard, err = clientsOnLowerTunnel(tx, inboundId, inbound.NodeID, ids, []model.Protocol{model.WireGuard}); err != nil {
				return err
			}
		}
	}

	idByEmail := make(map[string]int, len(emails))
	pending := make(map[string]*model.ClientRecord, len(emails))
	toCreate := make([]*model.ClientRecord, 0, len(emails))
	for i := range clients {
		email := strings.TrimSpace(clients[i].Email)
		if email == "" {
			continue
		}

		incoming := clients[i].ToRecord()
		// ToRecord copies the raw email; store the trimmed key this function
		// looks up by, or a padded email is inserted and never found again.
		incoming.Email = email
		incoming.ClientHostRuleIds = model.NormalizeClientHostRuleIds(incoming.ClientHostRuleIds)
		row, ok := existing[email]
		if !ok {
			if _, dup := pending[email]; !dup {
				pending[email] = incoming
				toCreate = append(toCreate, incoming)
			}
			continue
		}

		before := *row
		applyClientRecordMerge(row, incoming)
		_, outranked := onLowerTunnel[row.Id]
		_, wireguardOutranked := onLowerWireguard[row.Id]
		keepLinkOwnedFields(row, &before, isTunnel && !outranked, inbound.Protocol == model.WireGuard && !wireguardOutranked)
		preservedUpdatedAt := max(incoming.UpdatedAt, row.UpdatedAt)
		row.UpdatedAt = preservedUpdatedAt

		idByEmail[email] = row.Id

		// DeepEqual: ClientRecord now carries a slice (ClientHostRuleIds)
		// and is no longer ==-comparable.
		if reflect.DeepEqual(*row, before) {
			continue
		}
		if err := tx.Save(row).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ClientRecord{}).
			Where("id = ?", row.Id).
			UpdateColumn("updated_at", preservedUpdatedAt).Error; err != nil {
			return err
		}
	}

	if len(toCreate) > 0 {
		// Capture enable before Create: gorm default:true drops explicit false (#6478).
		// Restate disabled rows after CreateInBatches.
		wantEnable := make([]bool, len(toCreate))
		for i, rec := range toCreate {
			wantEnable[i] = rec.Enable
		}
		if err := tx.CreateInBatches(toCreate, 200).Error; err != nil {
			return err
		}
		// Whichever API re-created the client, a delete tombstone left standing makes
		// the next node merge prune it and ReapSyncOrphans delete it.
		liftTombstonesOnCommit(tx, toCreate)
		disabledIDs := make([]int, 0)
		for i, rec := range toCreate {
			idByEmail[rec.Email] = rec.Id
			if !wantEnable[i] {
				disabledIDs = append(disabledIDs, rec.Id)
			}
		}
		for _, batch := range chunkInts(disabledIDs, sqlInChunk) {
			if err := tx.Model(&model.ClientRecord{}).Where("id IN ?", batch).
				UpdateColumn("enable", false).Error; err != nil {
				return err
			}
		}
	}

	wantedFlow := make(map[int]string, len(clients))
	wantedIds := make([]int, 0, len(clients))
	for i := range clients {
		email := strings.TrimSpace(clients[i].Email)
		if email == "" {
			continue
		}
		id, ok := idByEmail[email]
		if !ok {
			continue
		}
		if _, dup := wantedFlow[id]; dup {
			continue
		}
		wantedFlow[id] = clients[i].Flow
		wantedIds = append(wantedIds, id)
	}

	relinked, err := s.reconcileInboundLinks(tx, inboundId, wantedFlow, wantedIds, detachEmails, prune)
	if err != nil {
		return err
	}
	if err := settleRecordFlows(tx, relinked); err != nil {
		return err
	}

	return checkTuicDuplicateUUIDs(tx, []int{inboundId}, wantedIds, inbound.Protocol == model.TUIC)
}

// reconcileInboundLinks writes only the client_inbounds rows that differ and returns
// their clients. prune also removes links absent from wantedFlow, as only a full sync may.
func (s *ClientService) reconcileInboundLinks(tx *gorm.DB, inboundId int, wantedFlow map[int]string, wantedIds []int, detachEmails []string, prune bool) ([]int, error) {
	var current []model.ClientInbound
	if prune {
		if err := tx.Where("inbound_id = ?", inboundId).Find(&current).Error; err != nil {
			return nil, err
		}
	} else {
		for _, batch := range chunkInts(wantedIds, sqlInChunk) {
			var rows []model.ClientInbound
			if err := tx.Where("inbound_id = ? AND client_id IN ?", inboundId, batch).Find(&rows).Error; err != nil {
				return nil, err
			}
			current = append(current, rows...)
		}
	}

	var toDelete []int
	toUpdate := make(map[string][]int)
	have := make(map[int]struct{}, len(current))
	for _, link := range current {
		have[link.ClientId] = struct{}{}
		flow, keep := wantedFlow[link.ClientId]
		if !keep {
			if prune {
				toDelete = append(toDelete, link.ClientId)
			}
			continue
		}
		// Plain compare, not non-empty-wins: clearing a flow must persist "".
		if flow != link.FlowOverride {
			toUpdate[flow] = append(toUpdate[flow], link.ClientId)
		}
	}

	if len(detachEmails) > 0 {
		for _, batch := range chunkStrings(detachEmails, sqlInChunk) {
			var ids []int
			if err := tx.Model(&model.ClientRecord{}).Where("email IN ?", batch).Pluck("id", &ids).Error; err != nil {
				return nil, err
			}
			for _, id := range ids {
				if _, keep := wantedFlow[id]; !keep {
					toDelete = append(toDelete, id)
				}
			}
		}
	}

	toInsert := make([]model.ClientInbound, 0, len(wantedIds))
	for _, id := range wantedIds {
		if _, exists := have[id]; exists {
			continue
		}
		toInsert = append(toInsert, model.ClientInbound{
			ClientId:     id,
			InboundId:    inboundId,
			FlowOverride: wantedFlow[id],
		})
	}

	for _, batch := range chunkInts(toDelete, sqlInChunk) {
		if err := tx.Where("inbound_id = ? AND client_id IN ?", inboundId, batch).
			Delete(&model.ClientInbound{}).Error; err != nil {
			return nil, err
		}
	}
	for flow, ids := range toUpdate {
		for _, batch := range chunkInts(ids, sqlInChunk) {
			if err := tx.Model(&model.ClientInbound{}).
				Where("inbound_id = ? AND client_id IN ?", inboundId, batch).
				Update("flow_override", flow).Error; err != nil {
				return nil, err
			}
		}
	}
	if len(toInsert) > 0 {
		// The delete this replaced also serialized concurrent syncs of one
		// inbound; without the clause a racing node poll aborts its whole tx.
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "client_id"}, {Name: "inbound_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"flow_override"}),
		}).CreateInBatches(toInsert, 200).Error; err != nil {
			return nil, err
		}
		// Not left to the next node merge: there may never be one, and a mark that outlives the
		// attach hands the client to the reaper on its next detach.
		linked := make([]int, 0, len(toInsert))
		for _, link := range toInsert {
			linked = append(linked, link.ClientId)
		}
		for _, batch := range chunkInts(linked, sqlInChunk) {
			if err := tx.Model(&model.ClientRecord{}).Where("id IN ? AND sync_orphaned_at > 0", batch).
				Updates(map[string]any{"sync_orphaned_at": 0, "sync_orphaned_by": 0}).Error; err != nil {
				return nil, err
			}
		}
	}

	relinked := append([]int(nil), toDelete...)
	for _, ids := range toUpdate {
		relinked = append(relinked, ids...)
	}
	for _, link := range toInsert {
		relinked = append(relinked, link.ClientId)
	}
	return relinked, nil
}

// keepLinkOwnedFields undoes the merge of the record fields one inbound cannot decide
// alone, or syncing a client's inbounds in turn rewrites the record back and forth.
func keepLinkOwnedFields(row, before *model.ClientRecord, ownsTunnelPeer, ownsWireguardDevices bool) {
	row.Flow = before.Flow
	if !ownsWireguardDevices {
		row.WireguardDevices = model.CloneWireguardDevices(before.WireguardDevices)
		row.WgDeviceLimit = before.WgDeviceLimit
	}
	if ownsTunnelPeer {
		return
	}
	row.PrivateKey = before.PrivateKey
	row.PublicKey = before.PublicKey
	row.AllowedIPs = before.AllowedIPs
	row.PreSharedKey = before.PreSharedKey
	row.KeepAlive = before.KeepAlive
}

// clientsOnLowerTunnel returns the clients also attached to a higher-ranked WireGuard/AmneziaWG
// inbound, which owns their record's peer fields: a master-local one first, then the lowest id.
func clientsOnLowerTunnel(tx *gorm.DB, inboundId int, nodeID *int, clientIds []int, protocols []model.Protocol) (map[int]struct{}, error) {
	outranks := "inbounds.node_id IS NULL AND client_inbounds.inbound_id < ?"
	if nodeID != nil {
		outranks = "(inbounds.node_id IS NULL OR client_inbounds.inbound_id < ?)"
	}
	out := make(map[int]struct{})
	for _, batch := range chunkInts(clientIds, sqlInChunk) {
		var ids []int
		if err := tx.Table("client_inbounds").
			Joins("JOIN inbounds ON inbounds.id = client_inbounds.inbound_id").
			Where("client_inbounds.client_id IN ? AND inbounds.protocol IN ?",
				batch, protocols).
			Where(outranks, inboundId).
			Distinct().Pluck("client_inbounds.client_id", &ids).Error; err != nil {
			return nil, err
		}
		for _, id := range ids {
			out[id] = struct{}{}
		}
	}
	return out, nil
}

// settleRecordFlows sets the record flow of each still-linked client to its
// effective flow (EffectiveFlow's rule); an unlinked record keeps its own.
func settleRecordFlows(tx *gorm.DB, clientIds []int) error {
	for _, batch := range chunkInts(clientIds, sqlInChunk) {
		var links []model.ClientInbound
		if err := tx.Select("client_id", "flow_override").
			Where("client_id IN ?", batch).
			Order("inbound_id ASC").
			Find(&links).Error; err != nil {
			return err
		}
		effective := make(map[int]string, len(batch))
		for _, link := range links {
			if flow, seen := effective[link.ClientId]; !seen || (flow == "" && link.FlowOverride != "") {
				effective[link.ClientId] = link.FlowOverride
			}
		}
		var records []model.ClientRecord
		if err := tx.Select("id", "flow").Where("id IN ?", batch).Find(&records).Error; err != nil {
			return err
		}
		idsByFlow := make(map[string][]int)
		for _, rec := range records {
			if flow, linked := effective[rec.Id]; linked && flow != rec.Flow {
				idsByFlow[flow] = append(idsByFlow[flow], rec.Id)
			}
		}
		for flow, ids := range idsByFlow {
			if err := tx.Model(&model.ClientRecord{}).Where("id IN ?", ids).
				UpdateColumn("flow", flow).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ClientService) DetachInbound(tx *gorm.DB, inboundId int) error {
	if tx == nil {
		tx = database.GetDB()
	}
	return tx.Where("inbound_id = ?", inboundId).Delete(&model.ClientInbound{}).Error
}

type clientStatsIdentity struct {
	UUID  string
	Email string
	SubID string
}

func (s *ClientService) listStatsIdentitiesForInbounds(tx *gorm.DB, inboundIDs []int) (map[int][]clientStatsIdentity, error) {
	if tx == nil {
		tx = database.GetDB()
	}
	out := make(map[int][]clientStatsIdentity)
	for _, batch := range chunkInts(uniqueInts(inboundIDs), sqlInChunk) {
		var rows []struct {
			InboundID int    `gorm:"column:inbound_id"`
			UUID      string `gorm:"column:uuid"`
			Email     string `gorm:"column:email"`
			SubID     string `gorm:"column:sub_id"`
		}
		err := tx.Table("clients").
			Select("client_inbounds.inbound_id, clients.uuid, clients.email, clients.sub_id").
			Joins("JOIN client_inbounds ON client_inbounds.client_id = clients.id").
			Where("client_inbounds.inbound_id IN ?", batch).
			Order("client_inbounds.inbound_id ASC, clients.id ASC").
			Find(&rows).Error
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out[row.InboundID] = append(out[row.InboundID], clientStatsIdentity{
				UUID: row.UUID, Email: row.Email, SubID: row.SubID,
			})
		}
	}
	return out, nil
}

func (s *ClientService) ListForInbound(tx *gorm.DB, inboundId int) ([]model.Client, error) {
	if tx == nil {
		tx = database.GetDB()
	}
	type joinedRow struct {
		model.ClientRecord
		FlowOverride string
	}
	var rows []joinedRow
	err := tx.Table("clients").
		Select("clients.*, client_inbounds.flow_override AS flow_override").
		Joins("JOIN client_inbounds ON client_inbounds.client_id = clients.id").
		Where("client_inbounds.inbound_id = ?", inboundId).
		Order("clients.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]model.Client, 0, len(rows))
	for i := range rows {
		c := rows[i].ToClient()
		c.Flow = rows[i].FlowOverride
		out = append(out, *c)
	}
	return out, nil
}

// ListForInboundBySubId is ListForInbound narrowed to one subscription id —
// both filter columns are indexed, so the subscription server resolves a
// subscriber's clients without touching the inbound's settings JSON.
func (s *ClientService) ListForInboundBySubId(tx *gorm.DB, inboundId int, subId string) ([]model.Client, error) {
	if tx == nil {
		tx = database.GetDB()
	}
	type joinedRow struct {
		model.ClientRecord
		FlowOverride string
	}
	var rows []joinedRow
	err := tx.Table("clients").
		Select("clients.*, client_inbounds.flow_override AS flow_override").
		Joins("JOIN client_inbounds ON client_inbounds.client_id = clients.id").
		Where("client_inbounds.inbound_id = ? AND clients.sub_id = ?", inboundId, subId).
		Order("clients.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]model.Client, 0, len(rows))
	for i := range rows {
		c := rows[i].ToClient()
		c.Flow = rows[i].FlowOverride
		out = append(out, *c)
	}
	return out, nil
}

// validateProtocolClients runs inside the write every path linking clients to an inbound
// shares, so a client the protocol's core config would refuse never reaches it.
func (s *ClientService) validateProtocolClients(tx *gorm.DB, protocol model.Protocol, inboundID int, changed []model.Client, detached []string, prune bool) error {
	switch protocol {
	case model.TUIC:
		return s.validateTuicIdentities(tx, inboundID, changed, detached, prune)
	case model.MASQUE:
		return validateMasqueEmails(changed)
	}
	return nil
}

// MASQUE sends the email as the Basic-auth user, so xray-core refuses the whole inbound,
// and with it the Xray start, over one email holding ":".
func validateMasqueEmails(clients []model.Client) error {
	for _, client := range clients {
		if strings.Contains(client.Email, ":") {
			return fmt.Errorf("MASQUE: client email %q cannot contain \":\"", client.Email)
		}
	}
	return nil
}

func (s *ClientService) validateTuicIdentities(tx *gorm.DB, inboundID int, changed []model.Client, detached []string, prune bool) error {
	candidates := append([]model.Client(nil), changed...)
	if !prune {
		current, err := s.ListForInbound(tx, inboundID)
		if err != nil {
			return err
		}
		excluded := make(map[string]bool)
		for _, client := range changed {
			excluded[client.Email] = true
		}
		for _, email := range detached {
			excluded[email] = true
		}
		for _, client := range current {
			if !excluded[client.Email] {
				candidates = append(candidates, client)
			}
		}
	}
	seen := make(map[uuid.UUID]string)
	for _, client := range candidates {
		if client.ID == "" {
			continue
		}
		id, err := uuid.Parse(client.ID)
		if err != nil {
			return fmt.Errorf("TUIC: invalid client UUID")
		}
		if email, exists := seen[id]; exists && email != client.Email {
			return fmt.Errorf("TUIC: duplicate client UUID within inbound")
		}
		seen[id] = client.Email
	}
	return nil
}
