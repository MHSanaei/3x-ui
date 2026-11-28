package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

func (s *ClientService) GetRecordByEmail(tx *gorm.DB, email string) (*model.ClientRecord, error) {
	if tx == nil {
		tx = database.GetDB()
	}
	row := &model.ClientRecord{}
	err := tx.Where("email = ?", email).First(row).Error
	if err != nil {
		return nil, err
	}
	return row, nil
}

// EffectiveFlow returns the client's flow from the first flow-capable inbound
// it is attached to (lowest inbound_id with a non-empty flow_override). The
// canonical clients.Flow column is unreliable for multi-inbound clients: a
// non-flow inbound (Hysteria, WS, gRPC, …) carries an empty flow and, when its
// SyncInbound runs last, overwrites the column to "" even though a VLESS Reality
// inbound stored a real flow. The per-inbound flow_override is always correct,
// so derive the display flow from it (order-independent). See issue #4792.
func (s *ClientService) EffectiveFlow(tx *gorm.DB, recordId int) (string, error) {
	if tx == nil {
		tx = database.GetDB()
	}
	var flows []string
	err := tx.Model(&model.ClientInbound{}).
		Where("client_id = ? AND flow_override <> ?", recordId, "").
		Order("inbound_id ASC").
		Limit(1).
		Pluck("flow_override", &flows).Error
	if err != nil {
		return "", err
	}
	if len(flows) == 0 {
		return "", nil
	}
	return flows[0], nil
}

// EffectiveFlowsByEmails resolves the intended flow (non-empty flow_override,
// lowest inbound_id first — same rule as EffectiveFlow) for many clients in one
// query, keyed by email. Emails absent from the result carry no flow anywhere.
// Batched so flow restoration on an inbound with many clients is O(1) queries
// instead of O(clients). Used to restore a stripped flow onto an inbound that
// has just become flow-eligible.
func (s *ClientService) EffectiveFlowsByEmails(tx *gorm.DB, emails []string) (map[string]string, error) {
	if tx == nil {
		tx = database.GetDB()
	}
	out := make(map[string]string, len(emails))
	if len(emails) == 0 {
		return out, nil
	}
	type row struct {
		Email string
		Flow  string `gorm:"column:flow_override"`
	}
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var rows []row
		err := tx.Table("client_inbounds").
			Select("clients.email AS email, client_inbounds.flow_override AS flow_override").
			Joins("JOIN clients ON clients.id = client_inbounds.client_id").
			Where("clients.email IN ? AND client_inbounds.flow_override <> ?", batch, "").
			Order("client_inbounds.inbound_id ASC").
			Scan(&rows).Error
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if _, seen := out[r.Email]; !seen { // ordered by inbound_id ASC → first = lowest
				out[r.Email] = r.Flow
			}
		}
	}
	return out, nil
}

func (s *ClientService) GetInboundIdsForEmail(tx *gorm.DB, email string) ([]int, error) {
	if tx == nil {
		tx = database.GetDB()
	}
	var ids []int
	err := tx.Table("client_inbounds").
		Select("client_inbounds.inbound_id").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("clients.email = ?", email).
		Scan(&ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// sub_id carries a plain index, not a unique one: one subscription can cover
// several clients, so callers acting on a subId must handle all of them.
func (s *ClientService) GetRecordsBySubID(subId string) ([]*model.ClientRecord, error) {
	if subId == "" {
		return nil, errors.New("sub_id must not be empty")
	}
	var rows []*model.ClientRecord
	err := database.GetDB().Where("sub_id = ?", subId).Order("id ASC").Find(&rows).Error
	return rows, err
}

func (s *ClientService) GetRecordsByTgID(tgId int64) ([]*model.ClientRecord, error) {
	if tgId <= 0 {
		return nil, errors.New("tg_id must be a positive integer")
	}
	var rows []*model.ClientRecord
	err := database.GetDB().Where("tg_id = ?", tgId).Find(&rows).Error
	return rows, err
}

func (s *ClientService) GetByID(id int) (*model.ClientRecord, error) {
	row := &model.ClientRecord{}
	if err := database.GetDB().Where("id = ?", id).First(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

func (s *ClientService) GetInboundIdsForRecord(id int) ([]int, error) {
	var ids []int
	err := database.GetDB().Table("client_inbounds").
		Where("client_id = ?", id).
		Order("inbound_id ASC").
		Pluck("inbound_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// TunnelAllowedIPsByInbound returns each WireGuard/AmneziaWG inbound's real
// AllowedIPs for email without reading settings blobs of non-tunnel inbounds.
func (s *ClientService) TunnelAllowedIPsByInbound(inboundSvc *InboundService, email string, inboundIds []int) (map[int]string, error) {
	result := make(map[int]string)
	if len(inboundIds) == 0 {
		return result, nil
	}
	metas, metaErrs := inboundSvc.GetInboundMetaBatch(inboundIds)
	var tunnelIDs []int
	for _, ibId := range inboundIds {
		if mErr := metaErrs[ibId]; mErr != nil {
			if errors.Is(mErr, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, mErr
		}
		if m := metas[ibId]; m != nil && (m.Protocol == model.WireGuard || m.Protocol == model.AmneziaWG) {
			tunnelIDs = append(tunnelIDs, ibId)
		}
	}
	if len(tunnelIDs) == 0 {
		return result, nil
	}
	tunnels, err := inboundSvc.GetInboundsByIDs(nil, tunnelIDs)
	if err != nil {
		return nil, err
	}
	for _, ibId := range tunnelIDs {
		inbound := tunnels[ibId]
		if inbound == nil {
			continue
		}
		clients, err := inboundSvc.GetClients(inbound)
		if err != nil {
			return nil, err
		}
		for i := range clients {
			if strings.EqualFold(clients[i].Email, email) {
				result[ibId] = strings.Join(clients[i].AllowedIPs, ",")
				break
			}
		}
	}
	return result, nil
}

// ClientDetailPayload matches the JSON shape returned by /clients/get endpoints.
type ClientDetailPayload struct {
	Client           *model.ClientRecord        `json:"client"`
	InboundIds       []int                      `json:"inboundIds"`
	ExternalLinks    []model.ClientExternalLink `json:"externalLinks"`
	UsedTraffic      int64                      `json:"usedTraffic"`
	TunnelAllowedIPs map[int]string             `json:"tunnelAllowedIPs"`
}

// BuildClientPayloads builds detail payloads for one or more ClientRecords
// using O(1) batched queries instead of per-client N+1 lookups.
func (s *ClientService) BuildClientPayloads(inboundSvc *InboundService, records []*model.ClientRecord) ([]ClientDetailPayload, error) {
	if len(records) == 0 {
		return []ClientDetailPayload{}, nil
	}
	db := database.GetDB()
	clientIDs := make([]int, 0, len(records))
	emails := make([]string, 0, len(records))
	seenEmail := make(map[string]struct{}, len(records))
	for _, r := range records {
		if r == nil {
			continue
		}
		clientIDs = append(clientIDs, r.Id)
		if r.Email != "" {
			if _, ok := seenEmail[r.Email]; !ok {
				seenEmail[r.Email] = struct{}{}
				emails = append(emails, r.Email)
			}
		}
	}

	inboundIdsByClient := make(map[int][]int, len(clientIDs))
	flowByClient := make(map[int]string, len(clientIDs))
	allInboundSet := make(map[int]struct{})
	for _, batch := range chunkInts(clientIDs, sqlInChunk) {
		var links []struct {
			ClientId     int
			InboundId    int
			FlowOverride string
		}
		if err := db.Table("client_inbounds").
			Select("client_id, inbound_id, flow_override").
			Where("client_id IN ?", batch).
			Order("inbound_id ASC").
			Scan(&links).Error; err != nil {
			return nil, err
		}
		for _, l := range links {
			inboundIdsByClient[l.ClientId] = append(inboundIdsByClient[l.ClientId], l.InboundId)
			allInboundSet[l.InboundId] = struct{}{}
			if l.FlowOverride != "" && flowByClient[l.ClientId] == "" {
				flowByClient[l.ClientId] = l.FlowOverride
			}
		}
	}

	extLinksByClient := make(map[int][]model.ClientExternalLink, len(clientIDs))
	for _, batch := range chunkInts(clientIDs, sqlInChunk) {
		var rows []model.ClientExternalLink
		if err := db.Where("client_id IN ?", batch).
			Order("sort_index ASC, id ASC").
			Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			extLinksByClient[row.ClientId] = append(extLinksByClient[row.ClientId], row)
		}
	}

	trafficByEmail := make(map[string]*xray.ClientTraffic, len(emails))
	if len(emails) > 0 {
		var stats []xray.ClientTraffic
		for _, batch := range chunkStrings(emails, sqlInChunk) {
			var batchStats []xray.ClientTraffic
			if err := db.Where("email IN ?", batch).Find(&batchStats).Error; err != nil {
				return nil, err
			}
			stats = append(stats, batchStats...)
		}
		overlayGlobalTrafficValues(db, stats)
		for i := range stats {
			trafficByEmail[stats[i].Email] = &stats[i]
		}
	}

	tunnelAllowedByInbound := make(map[int]map[string]string)
	if len(allInboundSet) > 0 {
		allInboundIDs := make([]int, 0, len(allInboundSet))
		for id := range allInboundSet {
			allInboundIDs = append(allInboundIDs, id)
		}
		metas, metaErrs := inboundSvc.GetInboundMetaBatch(allInboundIDs)
		var tunnelIDs []int
		for _, id := range allInboundIDs {
			if mErr := metaErrs[id]; mErr != nil {
				if errors.Is(mErr, gorm.ErrRecordNotFound) {
					continue
				}
				return nil, mErr
			}
			if m := metas[id]; m != nil && (m.Protocol == model.WireGuard || m.Protocol == model.AmneziaWG) {
				tunnelIDs = append(tunnelIDs, id)
			}
		}
		if len(tunnelIDs) > 0 {
			tunnels, err := inboundSvc.GetInboundsByIDs(nil, tunnelIDs)
			if err != nil {
				return nil, err
			}
			for _, ib := range tunnels {
				clients, err := inboundSvc.GetClients(ib)
				if err != nil {
					return nil, err
				}
				byEmail := make(map[string]string, len(clients))
				for i := range clients {
					if clients[i].Email != "" {
						byEmail[strings.ToLower(clients[i].Email)] = strings.Join(clients[i].AllowedIPs, ",")
					}
				}
				tunnelAllowedByInbound[ib.Id] = byEmail
			}
		}
	}

	out := make([]ClientDetailPayload, 0, len(records))
	for _, r := range records {
		if r == nil {
			continue
		}
		recCopy := *r
		recCopy.Flow = flowByClient[r.Id]

		ibIds := inboundIdsByClient[r.Id]
		if ibIds == nil {
			ibIds = []int{}
		}
		extLinks := extLinksByClient[r.Id]
		if extLinks == nil {
			extLinks = []model.ClientExternalLink{}
		}
		var usedTraffic int64
		if t := trafficByEmail[r.Email]; t != nil {
			usedTraffic = t.Up + t.Down
		}
		tunnelAllowedIPs := make(map[int]string)
		if len(tunnelAllowedByInbound) > 0 && r.Email != "" {
			emailKey := strings.ToLower(r.Email)
			for _, ibId := range ibIds {
				if byEmail, ok := tunnelAllowedByInbound[ibId]; ok {
					if allowed, exists := byEmail[emailKey]; exists {
						tunnelAllowedIPs[ibId] = allowed
					}
				}
			}
		}
		out = append(out, ClientDetailPayload{
			Client:           &recCopy,
			InboundIds:       ibIds,
			ExternalLinks:    extLinks,
			UsedTraffic:      usedTraffic,
			TunnelAllowedIPs: tunnelAllowedIPs,
		})
	}
	return out, nil
}

func clientInboundAggregationSQL(dialect string) string {
	if dialect == database.DialectPostgres {
		return `SELECT ci.client_id, string_agg(ci.inbound_id::text, ',' ORDER BY ci.ctid) AS inbound_ids FROM client_inbounds ci GROUP BY ci.client_id`
	}
	return `SELECT client_id, GROUP_CONCAT(inbound_id) AS inbound_ids FROM (SELECT client_id, inbound_id FROM client_inbounds ORDER BY rowid ASC LIMIT -1) GROUP BY client_id`
}

func loadAllClientInboundAggregated(db *gorm.DB, into map[int][]int) error {
	rows, err := db.Raw(clientInboundAggregationSQL(db.Name())).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var clientID int
		var csv sql.NullString
		if err := rows.Scan(&clientID, &csv); err != nil {
			return err
		}
		if !csv.Valid || csv.String == "" {
			continue
		}
		ids := make([]int, 0, strings.Count(csv.String, ",")+1)
		s := csv.String
		for {
			part, rest, found := strings.Cut(s, ",")
			if id, convErr := strconv.Atoi(part); convErr == nil {
				ids = append(ids, id)
			}
			if !found {
				break
			}
			s = rest
		}
		into[clientID] = ids
	}
	return rows.Err()
}

func (s *ClientService) List() ([]ClientWithAttachments, error) {
	db := database.GetDB()
	var rows []model.ClientRecord
	if err := db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []ClientWithAttachments{}, nil
	}

	attachments := make(map[int][]int, len(rows))
	if err := loadAllClientInboundAggregated(db, attachments); err != nil {
		return nil, err
	}

	var stats []xray.ClientTraffic
	if err := db.Find(&stats).Error; err != nil {
		return nil, err
	}
	overlayGlobalTrafficValues(db, stats)
	trafficByEmail := make(map[string]*xray.ClientTraffic, len(stats))
	for i := range stats {
		trafficByEmail[stats[i].Email] = &stats[i]
	}

	out := make([]ClientWithAttachments, 0, len(rows))
	for i := range rows {
		out = append(out, ClientWithAttachments{
			ClientRecord: rows[i],
			InboundIds:   attachments[rows[i].Id],
			Traffic:      trafficByEmail[rows[i].Email],
		})
	}
	return out, nil
}

func (s *ClientService) HasPendingNode(inboundSvc *InboundService, email string) bool {
	if strings.TrimSpace(email) == "" {
		return false
	}
	ids, err := s.GetInboundIdsForEmail(nil, email)
	if err != nil {
		return false
	}
	return inboundSvc.AnyNodePending(ids)
}

const settingsProbeBytes = 64

// findInboundIdsByClientEmail returns every inbound whose settings.clients[]
// JSON contains an entry with the given email. Driver-portable (no JSON
// operators) by parsing in Go — fine for the rare fallback path.
func (s *ClientService) findInboundIdsByClientEmail(email string) ([]int, error) {
	// A literal, bounded probe: the email comes from a request, and LIKE costs the settings
	// length times the pattern length; the exact match below decides.
	probe := email
	if len(probe) > settingsProbeBytes {
		cut := settingsProbeBytes
		for cut > 0 && !utf8.RuneStart(probe[cut]) {
			cut--
		}
		probe = probe[:cut]
	}
	var inbounds []model.Inbound
	if err := database.GetDB().
		Select("id, settings").
		Where("settings LIKE ? ESCAPE '\\'", "%"+escapeLikeLiteral(probe)+"%").
		Find(&inbounds).Error; err != nil {
		return nil, err
	}
	out := make([]int, 0, len(inbounds))
	for _, ib := range inbounds {
		var settings map[string]any
		if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
			continue
		}
		clients, ok := settings["clients"].([]any)
		if !ok {
			continue
		}
		for _, c := range clients {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if cEmail, _ := cm["email"].(string); cEmail == email {
				out = append(out, ib.Id)
				break
			}
		}
	}
	return out, nil
}

// clientRecordsByEmail batch-loads client rows for emails, keyed by email.
// Callers pass an already-deduplicated list; absent addresses are simply
// missing from the map.
func clientRecordsByEmail(tx *gorm.DB, emails []string) (map[string]*model.ClientRecord, error) {
	if tx == nil {
		tx = database.GetDB()
	}
	var records []model.ClientRecord
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var rows []model.ClientRecord
		// Scoped transactions can reuse their Statement; clone each chunk so
		// WHERE clauses from earlier chunks cannot accumulate.
		if err := tx.Session(&gorm.Session{}).Where("email IN ?", batch).Find(&rows).Error; err != nil {
			return nil, err
		}
		records = append(records, rows...)
	}
	byEmail := make(map[string]*model.ClientRecord, len(records))
	for i := range records {
		byEmail[records[i].Email] = &records[i]
	}
	return byEmail, nil
}
