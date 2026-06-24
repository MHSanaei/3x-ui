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

// ExportAll returns every client in the same {client, inboundIds} shape that
// /add and /bulkCreate accept, so an exported file round-trips straight back
// through Import. Clients with no inbound attachment are included with an empty
// inboundIds list so an export taken before DeleteOrphans can restore them.
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
	for i := range rows {
		ids = append(ids, rows[i].Id)
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
		})
	}
	return out, nil
}

// ImportClients recreates clients from an exported list. Items that carry
// inboundIds go through the normal BulkCreate path (added to every inbound and
// pushed to xray); items with no inboundIds are restored as bare records so an
// orphan-inclusive export round-trips. Existing emails are never overwritten —
// they are reported in Skipped. The boolean reports whether xray needs a restart.
func (s *ClientService) ImportClients(inboundSvc *InboundService, items []ClientCreatePayload) (BulkCreateResult, bool, error) {
	result := BulkCreateResult{}
	if len(items) == 0 {
		return result, false, nil
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

		if err := db.Create(client.ToRecord()).Error; err != nil {
			skip(email, err.Error())
			continue
		}
		result.Created++
	}

	return result, needRestart, nil
}

// DeleteOrphans removes every client that is not attached to any inbound,
// together with its traffic rows, IP log, and external links. It mirrors the
// cleanup the single-client Delete performs, batched into one transaction.
// Returns the number of clients deleted.
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
	for i := range rows {
		ids = append(ids, rows[i].Id)
		if rows[i].Email != "" {
			emails = append(emails, rows[i].Email)
		}
	}
	tombstoneClientEmails(emails)

	if err := runSerializedTx(func(tx *gorm.DB) error {
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
