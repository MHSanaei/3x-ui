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
	Total        int64 `json:"total"`
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
				Total:        t.Total,
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
		out = append(out, ClientCreatePayload{
			Client:     *client,
			InboundIds: attachments[rows[i].Id],
			LimitHwid:  rows[i].LimitHwid,
			Traffic:    trafficByEmail[rows[i].Email],
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

	existingEmails, err := existingClientEmails(items)
	if err != nil {
		return result, false, err
	}

	attached := make([]ClientCreatePayload, 0, len(items))
	orphans := make([]ClientCreatePayload, 0)
	for i := range items {
		if len(items[i].InboundIds) > 0 {
			attached = append(attached, items[i])
		} else {
			orphans = append(orphans, items[i])
		}
	}

	skip := func(email, reason string) {
		if strings.TrimSpace(email) == "" {
			email = "(missing email)"
		}
		result.Skipped = append(result.Skipped, BulkCreateReport{Email: email, Reason: reason})
	}

	needRestart := false
	if len(attached) > 0 {
		sub, nr, err := s.BulkCreate(inboundSvc, attached)
		if err != nil {
			return result, needRestart, err
		}
		needRestart = needRestart || nr
		result.Created += sub.Created
		result.Skipped = append(result.Skipped, sub.Skipped...)
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
		if verr := validateClientResetDay(client.ResetDay); verr != nil {
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
		if !client.Enable {
			client.Enable = true
		}
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
		result.Created++
	}

	if err := s.applyPortableTraffics(items, existingEmails, result.Skipped); err != nil {
		return result, needRestart, err
	}

	return result, needRestart, nil
}

func existingClientEmails(items []ClientCreatePayload) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	emails := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for i := range items {
		email := strings.TrimSpace(items[i].Client.Email)
		if email == "" {
			continue
		}
		le := strings.ToLower(email)
		if _, ok := seen[le]; ok {
			continue
		}
		seen[le] = struct{}{}
		emails = append(emails, email)
	}
	if len(emails) == 0 {
		return out, nil
	}
	db := database.GetDB()
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var rows []model.ClientRecord
		if err := db.Select("email").Where("email IN ?", batch).Find(&rows).Error; err != nil {
			return nil, err
		}
		for i := range rows {
			out[strings.ToLower(rows[i].Email)] = struct{}{}
		}
	}
	return out, nil
}

func (s *ClientService) applyPortableTraffics(items []ClientCreatePayload, existingEmails map[string]struct{}, skipped []BulkCreateReport) error {
	skippedSet := make(map[string]struct{}, len(skipped))
	for _, rep := range skipped {
		skippedSet[strings.ToLower(rep.Email)] = struct{}{}
	}
	for i := range items {
		snap := items[i].Traffic
		if snap == nil {
			continue
		}
		email := strings.TrimSpace(items[i].Client.Email)
		if email == "" {
			continue
		}
		le := strings.ToLower(email)
		if _, existed := existingEmails[le]; existed {
			continue
		}
		if _, wasSkipped := skippedSet[le]; wasSkipped {
			continue
		}
		if err := applyPortableTraffic(email, snap, items[i].Client); err != nil {
			return err
		}
	}
	return nil
}

// applyPortableTraffic restores exported up/down onto a newly imported email's
// client_traffics row without touching limit/expiry already set by AddClientStat.
func applyPortableTraffic(email string, snap *ClientPortableTraffic, client model.Client) error {
	if snap == nil {
		return nil
	}
	return submitTrafficWrite(func() error {
		db := database.GetDB()
		updates := map[string]any{
			"up":             snap.Up,
			"down":           snap.Down,
			"reset_count":    snap.ResetCount,
			"last_online":    snap.LastOnline,
			"last_sub_fetch": snap.LastSubFetch,
		}
		res := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return nil
		}
		total := client.TotalGB
		if snap.Total > 0 {
			total = snap.Total
		}
		return db.Create(&xray.ClientTraffic{
			Email:        email,
			Enable:       true,
			Up:           snap.Up,
			Down:         snap.Down,
			Total:        total,
			ExpiryTime:   client.ExpiryTime,
			Reset:        client.Reset,
			ResetDay:     client.ResetDay,
			ResetMax:     client.ResetMax,
			ResetCount:   snap.ResetCount,
			LastOnline:   snap.LastOnline,
			LastSubFetch: snap.LastSubFetch,
		}).Error
	})
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
