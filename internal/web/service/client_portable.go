package service

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

// ClientPortableTraffic is the client_traffics snapshot carried in export/import.
// model.Client only has the limit (totalGB); usage counters live in this table.
type ClientPortableTraffic struct {
	Up           int64 `json:"up"`
	Down         int64 `json:"down"`
	ResetCount   int   `json:"resetCount"`
	LastOnline   int64 `json:"lastOnline,omitempty"`
	LastSubFetch int64 `json:"lastSubFetch,omitempty"`
}

// ExportAll returns every client as {client, inboundIds[, traffic]} for round-trip
// import; orphan clients keep empty inboundIds, and traffic preserves usage (#5858).
func (s *ClientService) ExportAll() ([]ClientCreatePayload, error) {
	db := database.GetDB()
	var rows []model.ClientRecord
	if err := db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ClientCreatePayload, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}

	ids := make([]int, 0, len(rows))
	emails := make([]string, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].Id)
		if rows[i].Email != "" {
			emails = append(emails, rows[i].Email)
		}
	}

	hostAssignments := make(map[int][]string, len(rows))
	for _, batch := range chunkInts(ids, sqlInChunk) {
		var links []model.ClientHost
		if err := db.Where("client_id IN ?", batch).Order("group_id ASC").Find(&links).Error; err != nil {
			return nil, err
		}
		for _, l := range links {
			hostAssignments[l.ClientId] = append(hostAssignments[l.ClientId], l.GroupId)
		}
	}

	attachments := make(map[int][]int, len(rows))
	for _, batch := range chunkInts(ids, sqlInChunk) {
		var links []model.ClientInbound
		if err := db.Where("client_id IN ?", batch).Order("inbound_id ASC").Find(&links).Error; err != nil {
			return nil, err
		}
		for _, l := range links {
			attachments[l.ClientId] = append(attachments[l.ClientId], l.InboundId)
		}
	}

	trafficByEmail := make(map[string]*ClientPortableTraffic, len(emails))
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var traffics []xray.ClientTraffic
		if err := db.Where("email IN ?", batch).Find(&traffics).Error; err != nil {
			return nil, err
		}
		for i := range traffics {
			t := traffics[i]
			trafficByEmail[t.Email] = &ClientPortableTraffic{
				Up:           t.Up,
				Down:         t.Down,
				ResetCount:   t.ResetCount,
				LastOnline:   t.LastOnline,
				LastSubFetch: t.LastSubFetch,
			}
		}
	}

	for i := range rows {
		client := rows[i].ToClient()
		// The per-inbound flow_override is the reliable flow for multi-inbound
		// clients; the canonical column can be left stale by SyncInbound (#4792).
		if flow, err := s.EffectiveFlow(db, rows[i].Id); err == nil && flow != "" {
			client.Flow = flow
		}
		hostGroupIds := hostAssignments[rows[i].Id]
		if hostGroupIds == nil {
			hostGroupIds = []string{}
		}
		out = append(out, ClientCreatePayload{
			Client:       *client,
			InboundIds:   attachments[rows[i].Id],
			HostGroupIds: hostGroupIds,
			LimitHwid:    rows[i].LimitHwid,
			Traffic:      trafficByEmail[rows[i].Email],
		})
	}
	return out, nil
}

// ImportClients recreates exported clients; existing emails are Skipped.
// Traffic is applied only for newly created emails so live counters stay intact (#5858).
func (s *ClientService) ImportClients(inboundSvc *InboundService, items []ClientCreatePayload) (BulkCreateResult, bool, error) {
	result := BulkCreateResult{}
	if len(items) == 0 {
		return result, false, nil
	}

	skip := func(email, reason string) {
		if strings.TrimSpace(email) == "" {
			email = "(missing email)"
		}
		result.Skipped = append(result.Skipped, BulkCreateReport{Email: email, Reason: reason})
	}

	attached := make([]ClientCreatePayload, 0, len(items))
	attachedSrc := make([]int, 0, len(items))
	orphans := make([]ClientCreatePayload, 0)
	orphanSrc := make([]int, 0, len(items))
	for i := range items {
		if err := (&ClientHostService{}).ValidateGroupIDs(items[i].HostGroupIds); err != nil {
			skip(items[i].Client.Email, err.Error())
			continue
		}
		if len(items[i].InboundIds) > 0 {
			attached = append(attached, items[i])
			attachedSrc = append(attachedSrc, i)
		} else {
			orphans = append(orphans, items[i])
			orphanSrc = append(orphanSrc, i)
		}
	}

	inserted := make([]int, 0, len(items))
	needRestart := false
	if len(attached) > 0 {
		sub, subInserted, nr, err := s.bulkCreate(inboundSvc, attached)
		if err != nil {
			return result, needRestart, err
		}
		needRestart = needRestart || nr
		result.Created += sub.Created
		result.Skipped = append(result.Skipped, sub.Skipped...)
		skippedEmails := make(map[string]struct{}, len(sub.Skipped))
		for _, item := range sub.Skipped {
			skippedEmails[strings.ToLower(strings.TrimSpace(item.Email))] = struct{}{}
		}
		for _, item := range attached {
			if len(item.HostGroupIds) == 0 {
				continue
			}
			email := strings.ToLower(strings.TrimSpace(item.Client.Email))
			if _, skipped := skippedEmails[email]; skipped {
				continue
			}
			if err := (&ClientHostService{}).SetGroupIDsByEmail(item.Client.Email, item.HostGroupIds); err != nil {
				return result, needRestart, err
			}
		}
		for _, j := range subInserted {
			inserted = append(inserted, attachedSrc[j])
		}
	}

	db := database.GetDB()
	for i := range orphans {
		client := orphans[i].Client
		email := strings.TrimSpace(client.Email)
		if email == "" {
			skip("", "client email is required")
			continue
		}
		if verr := validateClientEmail(email); verr != nil {
			skip(email, verr.Error())
			continue
		}
		if verr := validateClientSubID(client.SubID); verr != nil {
			skip(email, verr.Error())
			continue
		}
		if verr := validateClientRenewal(client); verr != nil {
			skip(email, verr.Error())
			continue
		}
		if verr := validateClientResetMax(client.ResetMax); verr != nil {
			skip(email, verr.Error())
			continue
		}
		if verr := validateClientTrafficReset(client.TrafficReset, client.TrafficResetDay); verr != nil {
			skip(email, verr.Error())
			continue
		}

		// An existing record (in the DB or just created from the attached set
		// above) always wins — import never clobbers a live client.
		var taken int64
		if err := db.Model(&model.ClientRecord{}).Where("email = ?", email).Count(&taken).Error; err != nil {
			return result, needRestart, err
		}
		if taken > 0 {
			skip(email, "email already in use: "+email)
			continue
		}

		client.Email = email
		if client.SubID == "" {
			client.SubID = uuid.NewString()
		}
		if client.SubID != "" {
			var subTaken int64
			if err := db.Model(&model.ClientRecord{}).
				Where("sub_id = ? AND email <> ?", client.SubID, email).
				Count(&subTaken).Error; err != nil {
				return result, needRestart, err
			}
			if subTaken > 0 {
				skip(email, "subId already in use: "+client.SubID)
				continue
			}
		}
		// Preserve exported enable so a disabled orphan stays disabled (#6478).
		now := time.Now().UnixMilli()
		if client.CreatedAt == 0 {
			client.CreatedAt = now
		}
		client.UpdatedAt = now

		rec := client.ToRecord()
		rec.LimitHwid = orphans[i].LimitHwid
		if err := db.Create(rec).Error; err != nil {
			skip(email, err.Error())
			continue
		}
		// gorm default:true drops enable=false on Create — restate (#6478).
		if !client.Enable {
			if err := db.Model(&model.ClientRecord{}).Where("id = ?", rec.Id).
				UpdateColumn("enable", false).Error; err != nil {
				return result, needRestart, err
			}
		}
		if len(orphans[i].HostGroupIds) > 0 {
			if err := (&ClientHostService{}).SetGroupIDs(rec.Id, orphans[i].HostGroupIds); err != nil {
				return result, needRestart, err
			}
		}
		result.Created++
		inserted = append(inserted, orphanSrc[i])
	}

	if err := applyPortableTraffics(inboundSvc, items, inserted); err != nil {
		return result, needRestart, err
	}

	return result, needRestart, nil
}

// applyPortableTraffics restores counters only for items that inserted a record,
// in batched serialized transactions rather than one writer round-trip per client.
func applyPortableTraffics(inboundSvc *InboundService, items []ClientCreatePayload, inserted []int) error {
	const batchSize = 400
	withTraffic := make([]int, 0, len(inserted))
	for _, i := range inserted {
		if items[i].Traffic != nil {
			withTraffic = append(withTraffic, i)
		}
	}
	for start := 0; start < len(withTraffic); start += batchSize {
		batch := withTraffic[start:min(start+batchSize, len(withTraffic))]
		if err := runSerializedTx(func(tx *gorm.DB) error {
			emails := make([]string, 0, len(batch))
			for _, i := range batch {
				if err := applyPortableTraffic(tx, inboundSvc, items[i]); err != nil {
					return err
				}
				emails = append(emails, strings.TrimSpace(items[i].Client.Email))
			}
			return adjustGroupBaselinesForRestoredTraffic(tx, emails)
		}); err != nil {
			return err
		}
	}
	return nil
}

// applyPortableTraffic writes the exported counters. Attached clients got their row
// on create; an orphan's row (new, or kept by a keepTraffic delete) is upserted here.
func applyPortableTraffic(tx *gorm.DB, inboundSvc *InboundService, item ClientCreatePayload) error {
	client := item.Client
	client.Email = strings.TrimSpace(client.Email)
	if len(item.InboundIds) == 0 {
		if err := inboundSvc.AddClientStat(tx, 0, &client); err != nil {
			return err
		}
	}
	return tx.Model(&xray.ClientTraffic{}).Where("email = ?", client.Email).Updates(map[string]any{
		"up":             item.Traffic.Up,
		"down":           item.Traffic.Down,
		"reset_count":    item.Traffic.ResetCount,
		"last_online":    item.Traffic.LastOnline,
		"last_sub_fetch": item.Traffic.LastSubFetch,
	}).Error
}

// DeleteOrphans removes every unattached client plus its traffic, IP log, and
// external links in one transaction; returns how many clients were deleted.
func (s *ClientService) DeleteOrphans() (int, error) {
	db := database.GetDB()
	sub := database.GetDB().Table("client_inbounds").Select("client_id")
	var rows []model.ClientRecord
	if err := db.Where("id NOT IN (?)", sub).Order("id ASC").Find(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	ids := make([]int, 0, len(rows))
	emails := make([]string, 0, len(rows))
	subIDs := make([]string, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].Id)
		if rows[i].Email != "" {
			emails = append(emails, rows[i].Email)
		}
		subIDs = append(subIDs, rows[i].SubID)
	}
	tombstoneClientEmails(emails)

	if err := runSerializedTx(func(tx *gorm.DB) error {
		if e := adjustGroupBaselinesForRemovedTraffic(tx, emails); e != nil {
			return e
		}
		if e := clearClientHwidsBySubIDTx(tx, subIDs...); e != nil {
			return e
		}
		for _, batch := range chunkInts(ids, sqlInChunk) {
			if e := tx.Where("client_id IN ?", batch).Delete(&model.ClientInbound{}).Error; e != nil {
				return e
			}
			if e := tx.Where("client_id IN ?", batch).Delete(&model.ClientHost{}).Error; e != nil {
				return e
			}
			if e := tx.Where("client_id IN ?", batch).Delete(&model.ClientExternalLink{}).Error; e != nil {
				return e
			}
		}
		if len(emails) > 0 {
			for _, batch := range chunkStrings(emails, sqlInChunk) {
				if e := tx.Where("email IN ?", batch).Delete(&xray.ClientTraffic{}).Error; e != nil {
					return e
				}
				if e := tx.Where("client_email IN ?", batch).Delete(&model.InboundClientIps{}).Error; e != nil {
					return e
				}
			}
			if e := clearGlobalTraffic(tx, emails...); e != nil {
				return e
			}
		}
		for _, batch := range chunkInts(ids, sqlInChunk) {
			if e := tx.Where("id IN ?", batch).Delete(&model.ClientRecord{}).Error; e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return len(ids), nil
}
