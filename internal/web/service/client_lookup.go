package service

import (
	"encoding/json"
	"strings"

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

func (s *ClientService) List() ([]ClientWithAttachments, error) {
	db := database.GetDB()
	var rows []model.ClientRecord
	if err := db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []ClientWithAttachments{}, nil
	}

	clientIds := make([]int, 0, len(rows))
	emails := make([]string, 0, len(rows))
	for i := range rows {
		clientIds = append(clientIds, rows[i].Id)
		if rows[i].Email != "" {
			emails = append(emails, rows[i].Email)
		}
	}

	attachments := make(map[int][]int, len(rows))
	for _, batch := range chunkInts(clientIds, sqlInChunk) {
		var links []model.ClientInbound
		if err := db.Where("client_id IN ?", batch).Find(&links).Error; err != nil {
			return nil, err
		}
		for _, l := range links {
			attachments[l.ClientId] = append(attachments[l.ClientId], l.InboundId)
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
		for i := range stats {
			trafficByEmail[stats[i].Email] = &stats[i]
		}
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

// findInboundIdsByClientEmail returns every inbound whose settings.clients[]
// JSON contains an entry with the given email. Driver-portable (no JSON
// operators) by parsing in Go — fine for the rare fallback path.
func (s *ClientService) findInboundIdsByClientEmail(email string) ([]int, error) {
	var inbounds []model.Inbound
	if err := database.GetDB().
		Select("id, settings").
		Where("settings LIKE ?", "%"+email+"%").
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
