package service

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ClientPortable is one entry of the export file: the {client, inboundIds} payload
// /add accepts plus the links the client owns, which are panel-local.
type ClientPortable struct {
	Client        model.Client        `json:"client"`
	InboundIds    []int               `json:"inboundIds"`
	LimitHwid     int                 `json:"limitHwid,omitempty"`
	ExternalLinks []ExternalLinkInput `json:"externalLinks,omitempty"`
}

func (p ClientPortable) payload() ClientCreatePayload {
	return ClientCreatePayload{Client: p.Client, InboundIds: p.InboundIds, LimitHwid: p.LimitHwid}
}

// ExportAll returns every client in the same {client, inboundIds} shape that
// /add and /bulkCreate accept, so an exported file round-trips straight back
// through Import. Clients with no inbound attachment are included with an empty
// inboundIds list so an export taken before DeleteOrphans can restore them.
func (s *ClientService) ExportAll() ([]ClientPortable, error) {
	db := database.GetDB()
	var rows []model.ClientRecord
	if err := db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ClientPortable, 0, len(rows))
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
		out = append(out, ClientPortable{
			Client:     *client,
			InboundIds: attachments[rows[i].Id],
			LimitHwid:  rows[i].LimitHwid,
		})
	}
	links, err := clientOwnExternalLinks(db, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].ExternalLinks = links[rows[i].Id]
	}
	return out, nil
}

// ImportClients recreates clients from an exported list. Items that carry
// inboundIds go through the normal BulkCreate path (added to every inbound and
// pushed to xray); items with no inboundIds are restored as bare records so an
// orphan-inclusive export round-trips. Existing emails are never overwritten —
// they are reported in Skipped. The boolean reports whether xray needs a restart.
func (s *ClientService) ImportClients(inboundSvc *InboundService, items []ClientPortable) (BulkCreateResult, bool, error) {
	result := BulkCreateResult{}
	if len(items) == 0 {
		return result, false, nil
	}

	attached := make([]ClientCreatePayload, 0, len(items))
	orphans := make([]ClientPortable, 0)
	for i := range items {
		if len(items[i].InboundIds) > 0 {
			attached = append(attached, items[i].payload())
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
		result.Created++
	}

	if err := applyImportedClientExternalLinks(db, items, result.Skipped); err != nil {
		return result, needRestart, err
	}
	return result, needRestart, nil
}

// clientOwnExternalLinks returns the links each client owns directly, in the
// order the operator set them, for the export file.
func clientOwnExternalLinks(db *gorm.DB, clientIds []int) (map[int][]ExternalLinkInput, error) {
	out := map[int][]ExternalLinkInput{}
	for _, batch := range chunkInts(clientIds, sqlInChunk) {
		var rows []externalLinkAssignmentRow
		if err := db.Model(&model.ExternalLinkAssignment{}).
			Select(externalLinkAssignmentColumns).
			Joins("JOIN external_links ON external_links.id = external_link_assignments.link_id").
			Where("external_link_assignments.target_type = ? AND external_link_assignments.target_id IN ?",
				model.ExternalLinkTargetClient, batch).
			Order("external_link_assignments.target_id ASC, external_link_assignments.sort_index ASC, external_link_assignments.id ASC").
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			link := row.toClientExternalLink(row.TargetId)
			out[row.TargetId] = append(out[row.TargetId], ExternalLinkInput{
				Kind:       link.Kind,
				Value:      link.Value,
				Remark:     link.Remark,
				Enable:     link.Enable,
				ExpiryTime: link.ExpiryTime,
				NamePrefix: link.NamePrefix,
			})
		}
	}
	return out, nil
}

// applyImportedClientExternalLinks restores the links an export carried, reading
// the library once and creating missing entries through the client-form helper.
func applyImportedClientExternalLinks(db *gorm.DB, items []ClientPortable, skipped []BulkCreateReport) error {
	byEmail := map[string][]ExternalLinkInput{}
	emails := []string{}
	for i := range items {
		email := strings.TrimSpace(items[i].Client.Email)
		if email == "" || len(items[i].ExternalLinks) == 0 {
			continue
		}
		if _, seen := byEmail[email]; seen {
			continue
		}
		byEmail[email] = items[i].ExternalLinks
		emails = append(emails, email)
	}
	if len(emails) == 0 {
		return nil
	}

	// A skipped email is a client the import refused; its links stay out too.
	// Emails are unique, so a client claimed by an earlier entry is skipped.
	skippedEmails := make(map[string]struct{}, len(skipped))
	for _, report := range skipped {
		skippedEmails[report.Email] = struct{}{}
	}

	idByEmail := map[string]int{}
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var rows []model.ClientRecord
		if err := db.Select("id", "email").Where("email IN ?", batch).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			idByEmail[row.Email] = row.Id
		}
	}

	var library []model.ExternalLink
	if err := db.Select("id", "kind", "value").Find(&library).Error; err != nil {
		return err
	}
	linkIdByValue := make(map[string]int, len(library))
	for _, link := range library {
		linkIdByValue[model.ExternalLinkIdentity(link.Kind, link.Value)] = link.Id
	}

	assignments := []model.ExternalLinkAssignment{}
	for _, email := range emails {
		if _, refused := skippedEmails[email]; refused {
			continue
		}
		clientId, ok := idByEmail[email]
		if !ok {
			continue
		}
		rows, err := normalizeExternalLinks(byEmail[email])
		if err != nil {
			return err
		}
		for index := range rows {
			key := model.ExternalLinkIdentity(rows[index].Kind, rows[index].Value)
			linkId, ok := linkIdByValue[key]
			if !ok {
				linkRow, err := upsertExternalLinkTx(db, rows[index])
				if err != nil {
					return err
				}
				linkId = linkRow.Id
				linkIdByValue[key] = linkId
			}
			assignments = append(assignments, model.ExternalLinkAssignment{
				LinkId:     linkId,
				TargetType: model.ExternalLinkTargetClient,
				TargetId:   clientId,
				Enable:     rows[index].Enable,
				// An imported 0 means "never" in the export it came from.
				ExpiryTime: model.AssignmentExpiry(rows[index].ExpiryTime),
				Remark:     rows[index].Remark,
				NamePrefix: rows[index].NamePrefix,
				SortIndex:  rows[index].SortIndex,
				Origin:     model.ExternalLinkOriginPanel,
			})
		}
	}
	if len(assignments) == 0 {
		return nil
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(assignments, 200).Error
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
			if e := dropExternalLinkAssignmentsTx(tx, model.ExternalLinkTargetClient, batch...); e != nil {
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
