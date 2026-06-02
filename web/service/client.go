package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/util/common"
	"github.com/mhsanaei/3x-ui/v3/util/random"
	"github.com/mhsanaei/3x-ui/v3/xray"

	"gorm.io/gorm"
)

type ClientWithAttachments struct {
	model.ClientRecord
	InboundIds []int               `json:"inboundIds"`
	Traffic    *xray.ClientTraffic `json:"traffic,omitempty"`
}

// MarshalJSON is required because model.ClientRecord defines its own
// MarshalJSON. Go promotes the embedded method to the outer struct, so without
// this the encoder would call ClientRecord.MarshalJSON for the whole value and
// silently drop InboundIds and Traffic from the API response.
func (c ClientWithAttachments) MarshalJSON() ([]byte, error) {
	rec, err := json.Marshal(c.ClientRecord)
	if err != nil {
		return nil, err
	}
	extras := struct {
		InboundIds []int               `json:"inboundIds"`
		Traffic    *xray.ClientTraffic `json:"traffic,omitempty"`
	}{InboundIds: c.InboundIds, Traffic: c.Traffic}
	extra, err := json.Marshal(extras)
	if err != nil {
		return nil, err
	}
	if len(rec) < 2 || rec[len(rec)-1] != '}' || len(extra) <= 2 {
		return rec, nil
	}
	const maxMarshalSize = 256 << 20
	if len(rec) > maxMarshalSize || len(extra) > maxMarshalSize {
		return rec, nil
	}
	out := make([]byte, 0, len(rec)+len(extra))
	out = append(out, rec[:len(rec)-1]...)
	if len(rec) > 2 {
		out = append(out, ',')
	}
	out = append(out, extra[1:]...)
	return out, nil
}

func clientKeyForProtocol(p model.Protocol, rec *model.ClientRecord) string {
	if rec == nil {
		return ""
	}
	switch p {
	case model.Trojan:
		return rec.Password
	case model.Shadowsocks:
		return rec.Email
	case model.Hysteria:
		return rec.Auth
	default:
		return rec.UUID
	}
}

type ClientService struct{}

// Short-lived tombstone of just-deleted client emails so that a node snapshot
// arriving between delete and node-side processing doesn't resurrect them.
var (
	recentlyDeletedMu sync.Mutex
	recentlyDeleted   = map[string]time.Time{}
)

const deleteTombstoneTTL = 90 * time.Second

var (
	inboundMutationLocksMu sync.Mutex
	inboundMutationLocks   = map[int]*sync.Mutex{}
)

func lockInbound(inboundId int) *sync.Mutex {
	inboundMutationLocksMu.Lock()
	defer inboundMutationLocksMu.Unlock()
	m, ok := inboundMutationLocks[inboundId]
	if !ok {
		m = &sync.Mutex{}
		inboundMutationLocks[inboundId] = m
	}
	m.Lock()
	return m
}

func compactOrphans(db *gorm.DB, clients []any) []any {
	if len(clients) == 0 {
		return clients
	}
	emails := make([]string, 0, len(clients))
	for _, c := range clients {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if e, _ := cm["email"].(string); e != "" {
			emails = append(emails, e)
		}
	}
	if len(emails) == 0 {
		return clients
	}
	var existingEmails []string
	if err := db.Model(&model.ClientRecord{}).Where("email IN ?", emails).Pluck("email", &existingEmails).Error; err != nil {
		logger.Warning("compactOrphans pluck:", err)
		return clients
	}
	if len(existingEmails) == len(emails) {
		return clients
	}
	existing := make(map[string]struct{}, len(existingEmails))
	for _, e := range existingEmails {
		existing[e] = struct{}{}
	}
	out := make([]any, 0, len(existingEmails))
	for _, c := range clients {
		cm, ok := c.(map[string]any)
		if !ok {
			out = append(out, c)
			continue
		}
		e, _ := cm["email"].(string)
		if e == "" {
			out = append(out, c)
			continue
		}
		if _, ok := existing[e]; ok {
			out = append(out, c)
		}
	}
	return out
}

func tombstoneClientEmail(email string) {
	if email == "" {
		return
	}
	recentlyDeletedMu.Lock()
	defer recentlyDeletedMu.Unlock()
	recentlyDeleted[email] = time.Now()
	cutoff := time.Now().Add(-deleteTombstoneTTL)
	for e, ts := range recentlyDeleted {
		if ts.Before(cutoff) {
			delete(recentlyDeleted, e)
		}
	}
}

func isClientEmailTombstoned(email string) bool {
	if email == "" {
		return false
	}
	recentlyDeletedMu.Lock()
	defer recentlyDeletedMu.Unlock()
	ts, ok := recentlyDeleted[email]
	if !ok {
		return false
	}
	if time.Since(ts) > deleteTombstoneTTL {
		delete(recentlyDeleted, email)
		return false
	}
	return true
}

func (s *ClientService) SyncInbound(tx *gorm.DB, inboundId int, clients []model.Client) error {
	if tx == nil {
		tx = database.GetDB()
	}

	if err := tx.Where("inbound_id = ?", inboundId).Delete(&model.ClientInbound{}).Error; err != nil {
		return err
	}

	for i := range clients {
		c := clients[i]
		email := strings.TrimSpace(c.Email)
		if email == "" {
			continue
		}

		incoming := c.ToRecord()
		row := &model.ClientRecord{}
		err := tx.Where("email = ?", email).First(row).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(incoming).Error; err != nil {
				return err
			}
			row = incoming
		} else {
			if incoming.UUID != "" {
				row.UUID = incoming.UUID
			}
			if incoming.Password != "" {
				row.Password = incoming.Password
			}
			if incoming.Auth != "" {
				row.Auth = incoming.Auth
			}
			row.Flow = incoming.Flow
			if incoming.Security != "" {
				row.Security = incoming.Security
			}
			if incoming.Reverse != "" {
				row.Reverse = incoming.Reverse
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
			if incoming.CreatedAt > 0 && (row.CreatedAt == 0 || incoming.CreatedAt < row.CreatedAt) {
				row.CreatedAt = incoming.CreatedAt
			}
			preservedUpdatedAt := max(incoming.UpdatedAt, row.UpdatedAt)
			row.UpdatedAt = preservedUpdatedAt
			if err := tx.Save(row).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.ClientRecord{}).
				Where("id = ?", row.Id).
				UpdateColumn("updated_at", preservedUpdatedAt).Error; err != nil {
				return err
			}
		}

		link := model.ClientInbound{
			ClientId:     row.Id,
			InboundId:    inboundId,
			FlowOverride: c.Flow,
		}
		if err := tx.Create(&link).Error; err != nil {
			return err
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

	var links []model.ClientInbound
	if err := db.Where("client_id IN ?", clientIds).Find(&links).Error; err != nil {
		return nil, err
	}
	attachments := make(map[int][]int, len(rows))
	for _, l := range links {
		attachments[l.ClientId] = append(attachments[l.ClientId], l.InboundId)
	}

	trafficByEmail := make(map[string]*xray.ClientTraffic, len(emails))
	if len(emails) > 0 {
		var stats []xray.ClientTraffic
		if err := db.Where("email IN ?", emails).Find(&stats).Error; err != nil {
			return nil, err
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

type ClientCreatePayload struct {
	Client     model.Client `json:"client"`
	InboundIds []int        `json:"inboundIds"`
}

func hasForbiddenClientChar(s string) bool {
	for _, r := range s {
		if r == '/' || r == '\\' || r == ' ' || r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func validateClientEmail(email string) error {
	if hasForbiddenClientChar(email) {
		return common.NewError("client email contains an invalid character:", email)
	}
	return nil
}

func validateClientSubID(subID string) error {
	if hasForbiddenClientChar(subID) {
		return common.NewError("client subId contains an invalid character:", subID)
	}
	return nil
}

func (s *ClientService) Create(inboundSvc *InboundService, payload *ClientCreatePayload) (bool, error) {
	if payload == nil {
		return false, common.NewError("empty payload")
	}
	client := payload.Client
	if strings.TrimSpace(client.Email) == "" {
		return false, common.NewError("client email is required")
	}
	if err := validateClientEmail(client.Email); err != nil {
		return false, err
	}
	if err := validateClientSubID(client.SubID); err != nil {
		return false, err
	}
	if len(payload.InboundIds) == 0 {
		return false, common.NewError("at least one inbound is required")
	}

	if client.SubID == "" {
		client.SubID = uuid.NewString()
	}
	if !client.Enable {
		client.Enable = true
	}
	now := time.Now().UnixMilli()
	if client.CreatedAt == 0 {
		client.CreatedAt = now
	}
	client.UpdatedAt = now

	existing := &model.ClientRecord{}
	err := database.GetDB().Where("email = ?", client.Email).First(existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	emailTaken := !errors.Is(err, gorm.ErrRecordNotFound)
	if emailTaken {
		if existing.SubID == "" || existing.SubID != client.SubID {
			return false, common.NewError("email already in use:", client.Email)
		}
	}

	if client.SubID != "" {
		var subTaken int64
		if err := database.GetDB().Model(&model.ClientRecord{}).
			Where("sub_id = ? AND email <> ?", client.SubID, client.Email).
			Count(&subTaken).Error; err != nil {
			return false, err
		}
		if subTaken > 0 {
			return false, common.NewError("subId already in use:", client.SubID)
		}
	}

	needRestart := false
	for _, ibId := range payload.InboundIds {
		inbound, getErr := inboundSvc.GetInbound(ibId)
		if getErr != nil {
			return needRestart, getErr
		}
		if err := s.fillProtocolDefaults(&client, inbound); err != nil {
			return needRestart, err
		}
		settingsPayload, mErr := json.Marshal(map[string][]model.Client{"clients": {clientWithInboundFlow(client, inbound)}})
		if mErr != nil {
			return needRestart, mErr
		}
		nr, addErr := s.AddInboundClient(inboundSvc, &model.Inbound{
			Id:       ibId,
			Settings: string(settingsPayload),
		})
		if addErr != nil {
			return needRestart, addErr
		}
		if nr {
			needRestart = true
		}
	}
	return needRestart, nil
}

func (s *ClientService) fillProtocolDefaults(c *model.Client, ib *model.Inbound) error {
	switch ib.Protocol {
	case model.VMESS, model.VLESS:
		if c.ID == "" {
			c.ID = uuid.NewString()
		}
	case model.Trojan:
		if c.Password == "" {
			c.Password = strings.ReplaceAll(uuid.NewString(), "-", "")
		}
	case model.Shadowsocks:
		method := shadowsocksMethodFromSettings(ib.Settings)
		if c.Password == "" || !validShadowsocksClientKey(method, c.Password) {
			c.Password = randomShadowsocksClientKey(method)
		}
	case model.Hysteria:
		if c.Auth == "" {
			c.Auth = strings.ReplaceAll(uuid.NewString(), "-", "")
		}
	}
	return nil
}

func clientWithInboundFlow(c model.Client, ib *model.Inbound) model.Client {
	if !inboundCanEnableTlsFlow(string(ib.Protocol), ib.StreamSettings) {
		c.Flow = ""
	}
	return c
}

func shadowsocksMethodFromSettings(settings string) string {
	if settings == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(settings), &m); err != nil {
		return ""
	}
	method, _ := m["method"].(string)
	return method
}

func randomShadowsocksClientKey(method string) string {
	if n := shadowsocksKeyBytes(method); n > 0 {
		return random.Base64Bytes(n)
	}
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func validShadowsocksClientKey(method, key string) bool {
	n := shadowsocksKeyBytes(method)
	if n == 0 {
		return key != ""
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return false
	}
	return len(decoded) == n
}

func shadowsocksKeyBytes(method string) int {
	switch method {
	case "2022-blake3-aes-128-gcm":
		return 16
	case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		return 32
	}
	return 0
}

func applyShadowsocksClientMethod(clients []any, settings map[string]any) {
	method, _ := settings["method"].(string)
	is2022 := strings.HasPrefix(method, "2022-blake3-")
	for i := range clients {
		cm, ok := clients[i].(map[string]any)
		if !ok {
			continue
		}
		if is2022 {
			if _, hasKey := cm["method"]; hasKey {
				delete(cm, "method")
				clients[i] = cm
			}
			continue
		}
		if method == "" {
			continue
		}
		if existing, _ := cm["method"].(string); existing != "" {
			continue
		}
		cm["method"] = method
		clients[i] = cm
	}
}

func (s *ClientService) Update(inboundSvc *InboundService, id int, updated model.Client) (bool, error) {
	existing, err := s.GetByID(id)
	if err != nil {
		return false, err
	}
	inboundIds, err := s.GetInboundIdsForRecord(id)
	if err != nil {
		return false, err
	}

	if strings.TrimSpace(updated.Email) == "" {
		return false, common.NewError("client email is required")
	}
	if err := validateClientEmail(updated.Email); err != nil {
		return false, err
	}
	if err := validateClientSubID(updated.SubID); err != nil {
		return false, err
	}
	if updated.SubID == "" {
		updated.SubID = existing.SubID
	}
	if updated.SubID == "" {
		updated.SubID = uuid.NewString()
	}
	updated.UpdatedAt = time.Now().UnixMilli()
	if updated.CreatedAt == 0 {
		updated.CreatedAt = existing.CreatedAt
	}

	if updated.Email != existing.Email {
		var collisionCount int64
		if err := database.GetDB().Model(&model.ClientRecord{}).
			Where("email = ? AND id <> ?", updated.Email, id).
			Count(&collisionCount).Error; err != nil {
			return false, err
		}
		if collisionCount > 0 {
			return false, common.NewError("Duplicate email:", updated.Email)
		}
		if err := database.GetDB().Model(&model.ClientRecord{}).
			Where("id = ?", id).
			Update("email", updated.Email).Error; err != nil {
			return false, err
		}
	}

	if updated.SubID != "" {
		var subCollision int64
		if err := database.GetDB().Model(&model.ClientRecord{}).
			Where("sub_id = ? AND id <> ?", updated.SubID, id).
			Count(&subCollision).Error; err != nil {
			return false, err
		}
		if subCollision > 0 {
			return false, common.NewError("Duplicate subId:", updated.SubID)
		}
	}

	needRestart := false
	for _, ibId := range inboundIds {
		inbound, getErr := inboundSvc.GetInbound(ibId)
		if getErr != nil {
			if errors.Is(getErr, gorm.ErrRecordNotFound) {
				if err := database.GetDB().
					Where("client_id = ? AND inbound_id = ?", id, ibId).
					Delete(&model.ClientInbound{}).Error; err != nil {
					return needRestart, err
				}
				continue
			}
			return needRestart, getErr
		}
		oldKey := clientKeyForProtocol(inbound.Protocol, existing)
		if oldKey == "" {
			continue
		}
		if err := s.fillProtocolDefaults(&updated, inbound); err != nil {
			return needRestart, err
		}
		settingsPayload, mErr := json.Marshal(map[string][]model.Client{"clients": {clientWithInboundFlow(updated, inbound)}})
		if mErr != nil {
			return needRestart, mErr
		}
		nr, upErr := s.UpdateInboundClient(inboundSvc, &model.Inbound{
			Id:       ibId,
			Settings: string(settingsPayload),
		}, oldKey)
		if upErr != nil {
			return needRestart, upErr
		}
		if nr {
			needRestart = true
		}
	}

	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("id = ?", id).
		UpdateColumn("updated_at", time.Now().UnixMilli()).Error; err != nil {
		return needRestart, err
	}
	return needRestart, nil
}

func (s *ClientService) Delete(inboundSvc *InboundService, id int, keepTraffic bool) (bool, error) {
	existing, err := s.GetByID(id)
	if err != nil {
		return false, err
	}
	tombstoneClientEmail(existing.Email)

	inboundIds, err := s.GetInboundIdsForRecord(id)
	if err != nil {
		return false, err
	}

	needRestart := false
	for _, ibId := range inboundIds {
		inbound, getErr := inboundSvc.GetInbound(ibId)
		if getErr != nil {
			if errors.Is(getErr, gorm.ErrRecordNotFound) {
				continue
			}
			return needRestart, getErr
		}
		key := clientKeyForProtocol(inbound.Protocol, existing)
		if key == "" {
			continue
		}
		nr, delErr := s.DelInboundClient(inboundSvc, ibId, key, false)
		if delErr != nil {
			return needRestart, delErr
		}
		if nr {
			needRestart = true
		}
	}

	db := database.GetDB()
	if err := db.Where("client_id = ?", id).Delete(&model.ClientInbound{}).Error; err != nil {
		return needRestart, err
	}
	if !keepTraffic && existing.Email != "" {
		if err := db.Where("email = ?", existing.Email).Delete(&xray.ClientTraffic{}).Error; err != nil {
			return needRestart, err
		}
		if err := db.Where("client_email = ?", existing.Email).Delete(&model.InboundClientIps{}).Error; err != nil {
			return needRestart, err
		}
	}
	if err := db.Delete(&model.ClientRecord{}, id).Error; err != nil {
		return needRestart, err
	}
	return needRestart, nil
}

func (s *ClientService) Attach(inboundSvc *InboundService, id int, inboundIds []int) (bool, error) {
	existing, err := s.GetByID(id)
	if err != nil {
		return false, err
	}
	currentIds, err := s.GetInboundIdsForRecord(id)
	if err != nil {
		return false, err
	}
	have := make(map[int]struct{}, len(currentIds))
	for _, x := range currentIds {
		have[x] = struct{}{}
	}

	clientWire := existing.ToClient()
	clientWire.UpdatedAt = time.Now().UnixMilli()

	needRestart := false
	for _, ibId := range inboundIds {
		if _, attached := have[ibId]; attached {
			continue
		}
		inbound, getErr := inboundSvc.GetInbound(ibId)
		if getErr != nil {
			return needRestart, getErr
		}
		copyClient := *clientWire
		if err := s.fillProtocolDefaults(&copyClient, inbound); err != nil {
			return needRestart, err
		}
		settingsPayload, mErr := json.Marshal(map[string][]model.Client{"clients": {clientWithInboundFlow(copyClient, inbound)}})
		if mErr != nil {
			return needRestart, mErr
		}
		nr, addErr := s.AddInboundClient(inboundSvc, &model.Inbound{
			Id:       ibId,
			Settings: string(settingsPayload),
		})
		if addErr != nil {
			return needRestart, addErr
		}
		if nr {
			needRestart = true
		}
	}
	return needRestart, nil
}

func (s *ClientService) CreateOne(inboundSvc *InboundService, inboundId int, client model.Client) (bool, error) {
	return s.Create(inboundSvc, &ClientCreatePayload{
		Client:     client,
		InboundIds: []int{inboundId},
	})
}

func (s *ClientService) DetachByEmail(inboundSvc *InboundService, inboundId int, email string) (bool, error) {
	if email == "" {
		return false, common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return false, err
	}
	return s.Detach(inboundSvc, rec.Id, []int{inboundId})
}

func (s *ClientService) AttachByEmail(inboundSvc *InboundService, email string, inboundIds []int) (bool, error) {
	if email == "" {
		return false, common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return false, err
	}
	return s.Attach(inboundSvc, rec.Id, inboundIds)
}

// BulkAttachResult reports the outcome of a bulk attach across target inbounds.
type BulkAttachResult struct {
	Attached []string `json:"attached"`
	Skipped  []string `json:"skipped"`
	Errors   []string `json:"errors"`
}

// BulkAttach attaches the given existing clients (by email) to each target inbound,
// reusing their identity (email/UUID/password/subId) and a shared traffic row. It adds
// all clients to a target in a single AddInboundClient call, and reports clients already
// present on a target as skipped.
func (s *ClientService) BulkAttach(inboundSvc *InboundService, emails []string, inboundIds []int) (*BulkAttachResult, bool, error) {
	result := &BulkAttachResult{}
	if len(emails) == 0 || len(inboundIds) == 0 {
		return result, false, nil
	}

	recordErr := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		result.Errors = append(result.Errors, msg)
		logger.Warningf("[BulkAttach] %s", msg)
	}

	records := make([]*model.ClientRecord, 0, len(emails))
	seenEmail := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		if email == "" {
			continue
		}
		key := strings.ToLower(email)
		if _, ok := seenEmail[key]; ok {
			continue
		}
		seenEmail[key] = struct{}{}
		rec, err := s.GetRecordByEmail(nil, email)
		if err != nil {
			recordErr("%s: %v", email, err)
			continue
		}
		records = append(records, rec)
	}

	emailSubIDs, sidErr := inboundSvc.getAllEmailSubIDs()
	if sidErr != nil {
		emailSubIDs = nil
		logger.Warningf("[BulkAttach] getAllEmailSubIDs: %v", sidErr)
	}

	needRestart := false
	for _, ibId := range inboundIds {
		inbound, err := inboundSvc.GetInbound(ibId)
		if err != nil {
			recordErr("inbound %d: %v", ibId, err)
			continue
		}
		existingClients, err := inboundSvc.GetClients(inbound)
		if err != nil {
			recordErr("inbound %d: %v", ibId, err)
			continue
		}
		have := make(map[string]struct{}, len(existingClients))
		for _, c := range existingClients {
			have[strings.ToLower(c.Email)] = struct{}{}
		}

		clientsToAdd := make([]model.Client, 0, len(records))
		for _, rec := range records {
			if _, attached := have[strings.ToLower(rec.Email)]; attached {
				result.Skipped = append(result.Skipped, rec.Email)
				continue
			}
			client := *rec.ToClient()
			client.UpdatedAt = time.Now().UnixMilli()
			if err := s.fillProtocolDefaults(&client, inbound); err != nil {
				recordErr("%s -> inbound %d: %v", rec.Email, ibId, err)
				continue
			}
			clientsToAdd = append(clientsToAdd, clientWithInboundFlow(client, inbound))
		}

		if len(clientsToAdd) == 0 {
			continue
		}

		payload, err := json.Marshal(map[string][]model.Client{"clients": clientsToAdd})
		if err != nil {
			recordErr("inbound %d: %v", ibId, err)
			continue
		}
		nr, err := s.addInboundClient(inboundSvc, &model.Inbound{Id: ibId, Settings: string(payload)}, emailSubIDs)
		if err != nil {
			recordErr("inbound %d: %v", ibId, err)
			continue
		}
		if nr {
			needRestart = true
		}
		for _, c := range clientsToAdd {
			result.Attached = append(result.Attached, c.Email)
		}
	}

	return result, needRestart, nil
}

// BulkDetachResult reports the outcome of a bulk detach across target inbounds.
type BulkDetachResult struct {
	Detached []string `json:"detached"`
	Skipped  []string `json:"skipped"`
	Errors   []string `json:"errors"`
}

// BulkDetach detaches the given existing clients (by email) from each target inbound.
// (email, inbound) pairs where the client is not currently attached are silently skipped
// at the inbound level; emails that aren't attached to any of the requested inbounds
// are reported under skipped. ClientRecord rows are kept even when they become orphaned
// (matches single-client detach semantics); callers should use bulkDelete for full removal.
func (s *ClientService) BulkDetach(inboundSvc *InboundService, emails []string, inboundIds []int) (*BulkDetachResult, bool, error) {
	result := &BulkDetachResult{}
	if len(emails) == 0 || len(inboundIds) == 0 {
		return result, false, nil
	}

	recordErr := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		result.Errors = append(result.Errors, msg)
		logger.Warningf("[BulkDetach] %s", msg)
	}

	requested := make(map[int]struct{}, len(inboundIds))
	for _, id := range inboundIds {
		requested[id] = struct{}{}
	}

	recsByInbound := make(map[int][]*model.ClientRecord)
	emailOrder := make([]string, 0, len(emails))
	emailRepr := make(map[string]string, len(emails))
	emailFailed := make(map[string]bool, len(emails))
	seenEmail := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		if email == "" {
			continue
		}
		key := strings.ToLower(email)
		if _, ok := seenEmail[key]; ok {
			continue
		}
		seenEmail[key] = struct{}{}

		rec, err := s.GetRecordByEmail(nil, email)
		if err != nil {
			recordErr("%s: %v", email, err)
			continue
		}
		currentIds, err := s.GetInboundIdsForRecord(rec.Id)
		if err != nil {
			recordErr("%s: %v", email, err)
			continue
		}
		matched := false
		for _, id := range currentIds {
			if _, ok := requested[id]; ok {
				recsByInbound[id] = append(recsByInbound[id], rec)
				matched = true
			}
		}
		if !matched {
			result.Skipped = append(result.Skipped, rec.Email)
			continue
		}
		emailOrder = append(emailOrder, key)
		emailRepr[key] = rec.Email
	}

	needRestart := false
	for _, ibId := range inboundIds {
		recs, ok := recsByInbound[ibId]
		if !ok {
			continue
		}
		delete(recsByInbound, ibId)
		nr, err := s.delInboundClients(inboundSvc, ibId, recs, true)
		if err != nil {
			recordErr("inbound %d: %v", ibId, err)
			for _, rec := range recs {
				emailFailed[strings.ToLower(rec.Email)] = true
			}
			continue
		}
		if nr {
			needRestart = true
		}
	}

	for _, key := range emailOrder {
		if emailFailed[key] {
			continue
		}
		result.Detached = append(result.Detached, emailRepr[key])
	}

	return result, needRestart, nil
}

// delInboundClients removes several clients from a single inbound in one pass:
// one settings rewrite, one runtime sweep, one Save and one SyncInbound for the
// whole batch, instead of repeating the full per-client cycle. It mirrors the
// semantics of DelInboundClient for each removed client. needRestart is the OR
// across all removals.
func (s *ClientService) delInboundClients(inboundSvc *InboundService, inboundId int, recs []*model.ClientRecord, keepTraffic bool) (bool, error) {
	if len(recs) == 0 {
		return false, nil
	}
	defer lockInbound(inboundId).Unlock()

	oldInbound, err := inboundSvc.GetInbound(inboundId)
	if err != nil {
		logger.Error("Load Old Data Error")
		return false, err
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(oldInbound.Settings), &settings); err != nil {
		return false, err
	}

	clientKey := "id"
	switch oldInbound.Protocol {
	case "trojan":
		clientKey = "password"
	case "shadowsocks":
		clientKey = "email"
	case "hysteria":
		clientKey = "auth"
	}

	wanted := make(map[string]struct{}, len(recs))
	for _, rec := range recs {
		if k := clientKeyForProtocol(oldInbound.Protocol, rec); k != "" {
			wanted[k] = struct{}{}
		}
	}

	interfaceClients, ok := settings["clients"].([]any)
	if !ok {
		return false, common.NewError("invalid clients format in inbound settings")
	}

	type removedClient struct {
		email      string
		needApiDel bool
	}
	removed := make([]removedClient, 0, len(wanted))
	newClients := make([]any, 0, len(interfaceClients))
	for _, client := range interfaceClients {
		c, ok := client.(map[string]any)
		if !ok {
			newClients = append(newClients, client)
			continue
		}
		cid, _ := c[clientKey].(string)
		if _, hit := wanted[cid]; hit && cid != "" {
			email, _ := c["email"].(string)
			enable, _ := c["enable"].(bool)
			removed = append(removed, removedClient{email: email, needApiDel: enable})
			continue
		}
		newClients = append(newClients, client)
	}

	if len(removed) == 0 {
		return false, nil
	}

	db := database.GetDB()
	newClients = compactOrphans(db, newClients)
	if newClients == nil {
		newClients = []any{}
	}
	settings["clients"] = newClients
	newSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	oldInbound.Settings = string(newSettings)

	needRestart := false
	for _, r := range removed {
		email := r.email
		emailShared, err := inboundSvc.emailUsedByOtherInbounds(email, inboundId)
		if err != nil {
			return needRestart, err
		}
		if !emailShared && !keepTraffic {
			if err := inboundSvc.DelClientIPs(db, email); err != nil {
				logger.Error("Error in delete client IPs")
				return needRestart, err
			}
		}
		if len(email) > 0 {
			var enables []bool
			if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).Limit(1).Pluck("enable", &enables).Error; err != nil {
				logger.Error("Get stats error")
				return needRestart, err
			}
			notDepleted := len(enables) > 0 && enables[0]
			if !emailShared && !keepTraffic {
				if err := inboundSvc.DelClientStat(db, email); err != nil {
					logger.Error("Delete stats Data Error")
					return needRestart, err
				}
			}
			if r.needApiDel && notDepleted && oldInbound.NodeID == nil {
				rt, rterr := inboundSvc.runtimeFor(oldInbound)
				if rterr != nil {
					needRestart = true
				} else if err1 := rt.RemoveUser(context.Background(), oldInbound, email); err1 != nil {
					if !strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", email)) {
						needRestart = true
					}
				}
			}
		}
		if oldInbound.NodeID != nil && len(email) > 0 {
			rt, rterr := inboundSvc.runtimeFor(oldInbound)
			if rterr != nil {
				return needRestart, rterr
			}
			if err1 := rt.DeleteUser(context.Background(), oldInbound, email); err1 != nil {
				return needRestart, err1
			}
		}
	}

	if err := db.Save(oldInbound).Error; err != nil {
		return needRestart, err
	}
	finalClients, gcErr := inboundSvc.GetClients(oldInbound)
	if gcErr != nil {
		return needRestart, gcErr
	}
	if err := s.SyncInbound(db, inboundId, finalClients); err != nil {
		return needRestart, err
	}
	return needRestart, nil
}

func (s *ClientService) DetachByEmailMany(inboundSvc *InboundService, email string, inboundIds []int) (bool, error) {
	if email == "" {
		return false, common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return false, err
	}
	return s.Detach(inboundSvc, rec.Id, inboundIds)
}

func (s *ClientService) DeleteByEmail(inboundSvc *InboundService, email string, keepTraffic bool) (bool, error) {
	if email == "" {
		return false, common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err == nil {
		return s.Delete(inboundSvc, rec.Id, keepTraffic)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	inboundIds, idsErr := s.findInboundIdsByClientEmail(email)
	if idsErr != nil {
		return false, idsErr
	}
	if len(inboundIds) == 0 {
		return false, common.NewError(fmt.Sprintf("client %q not found in any inbound or client record", email))
	}
	needRestart := false
	for _, ibId := range inboundIds {
		nr, delErr := s.DelInboundClientByEmail(inboundSvc, ibId, email, false)
		if delErr != nil {
			return needRestart, delErr
		}
		if nr {
			needRestart = true
		}
	}
	if !keepTraffic {
		db := database.GetDB()
		if err := db.Where("email = ?", email).Delete(&xray.ClientTraffic{}).Error; err != nil {
			return needRestart, err
		}
		if err := db.Where("client_email = ?", email).Delete(&model.InboundClientIps{}).Error; err != nil {
			return needRestart, err
		}
	}
	return needRestart, nil
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

func (s *ClientService) UpdateByEmail(inboundSvc *InboundService, email string, updated model.Client) (bool, error) {
	if email == "" {
		return false, common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return false, err
	}
	return s.Update(inboundSvc, rec.Id, updated)
}

func (s *ClientService) ResetTrafficByEmail(inboundSvc *InboundService, email string) (bool, error) {
	if email == "" {
		return false, common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return false, err
	}
	inboundIds, err := s.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		return false, err
	}
	if len(inboundIds) == 0 {
		if rErr := inboundSvc.ResetClientTrafficByEmail(email); rErr != nil {
			return false, rErr
		}
		return false, nil
	}
	needRestart := false
	for _, ibId := range inboundIds {
		nr, rErr := inboundSvc.ResetClientTraffic(ibId, email)
		if rErr != nil {
			return needRestart, rErr
		}
		if nr {
			needRestart = true
		}
	}
	return needRestart, nil
}

// ClientSlim is the row-shape used by the clients page. It drops fields the
// table never reads (UUID, password, auth, flow, security, reverse, tgId)
// so the list payload stays compact even when the panel manages thousands
// of clients. Modals that need the full record still call /get/:email.
type ClientSlim struct {
	Email      string              `json:"email"`
	SubID      string              `json:"subId"`
	Enable     bool                `json:"enable"`
	TotalGB    int64               `json:"totalGB"`
	ExpiryTime int64               `json:"expiryTime"`
	LimitIP    int                 `json:"limitIp"`
	Reset      int                 `json:"reset"`
	Group      string              `json:"group,omitempty"`
	Comment    string              `json:"comment,omitempty"`
	InboundIds []int               `json:"inboundIds"`
	Traffic    *xray.ClientTraffic `json:"traffic,omitempty"`
	CreatedAt  int64               `json:"createdAt"`
	UpdatedAt  int64               `json:"updatedAt"`
}

// ClientPageParams are the query params accepted by /panel/api/clients/list/paged.
// All fields are optional — the empty value means "no filter" / defaults.
//
// Filter / Protocol / Inbound accept either a single value or a comma-separated
// list; matching is OR within a field and AND across fields. The numeric range
// fields treat 0 as "unset" on the lower bound and 0 (or negative) as
// "unbounded" on the upper bound.
type ClientPageParams struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Search   string `form:"search"`
	Filter   string `form:"filter"`
	Protocol string `form:"protocol"`
	Inbound  string `form:"inbound"`
	Sort     string `form:"sort"`
	Order    string `form:"order"`

	ExpiryFrom int64  `form:"expiryFrom"`
	ExpiryTo   int64  `form:"expiryTo"`
	UsageFrom  int64  `form:"usageFrom"`
	UsageTo    int64  `form:"usageTo"`
	AutoRenew  string `form:"autoRenew"`
	HasTgID    string `form:"hasTgId"`
	HasComment string `form:"hasComment"`
	Group      string `form:"group"`
}

// ClientPageResponse is the shape returned by ListPaged. `Total` is the
// row count in the DB; `Filtered` is the count after Search/Filter/Protocol
// were applied, before pagination. The page contains at most PageSize items.
// Summary is computed across the full DB row set so dashboard counters
// on the clients page stay stable as the user paginates/filters.
type ClientPageResponse struct {
	Items    []ClientSlim   `json:"items"`
	Total    int            `json:"total"`
	Filtered int            `json:"filtered"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Summary  ClientsSummary `json:"summary"`
	Groups   []string       `json:"groups"`
}

// ClientsSummary collects per-bucket counts plus the matching email lists so
// the clients page can render the dashboard stat cards and their hover
// popovers without shipping the full client array.
type ClientsSummary struct {
	Total    int      `json:"total"`
	Active   int      `json:"active"`
	Online   []string `json:"online"`
	Depleted []string `json:"depleted"`
	Expiring []string `json:"expiring"`
	Deactive []string `json:"deactive"`
}

const (
	clientPageDefaultSize = 25
	clientPageMaxSize     = 200
)

// ListPaged loads every client (with traffic + attachments) into memory,
// applies the requested filter / search / protocol predicates, sorts, and
// returns the requested page along with total and filtered counts. The DB
// query itself is unchanged from List(); the win is that the response
// only carries 25-ish slim rows over the wire instead of all 2000 full
// records, which on real panels was the dominant cost.
func (s *ClientService) ListPaged(inboundSvc *InboundService, settingSvc *SettingService, params ClientPageParams) (*ClientPageResponse, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	total := len(all)

	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = clientPageDefaultSize
	}
	if pageSize > clientPageMaxSize {
		pageSize = clientPageMaxSize
	}
	page := params.Page
	if page <= 0 {
		page = 1
	}

	protocols := parseCSVStrings(params.Protocol)
	inboundIDs := parseCSVInts(params.Inbound)
	buckets := parseCSVStrings(params.Filter)

	var protocolByInbound map[int]string
	if len(protocols) > 0 {
		inbounds, err := inboundSvc.GetAllInbounds()
		if err == nil {
			protocolByInbound = make(map[int]string, len(inbounds))
			for _, ib := range inbounds {
				protocolByInbound[ib.Id] = string(ib.Protocol)
			}
		}
	}

	onlines := inboundSvc.GetOnlineClients()
	onlineSet := make(map[string]struct{}, len(onlines))
	for _, e := range onlines {
		onlineSet[e] = struct{}{}
	}

	var expireDiffMs, trafficDiffBytes int64
	if settingSvc != nil {
		if v, err := settingSvc.GetExpireDiff(); err == nil {
			expireDiffMs = int64(v) * 86400000
		}
		if v, err := settingSvc.GetTrafficDiff(); err == nil {
			trafficDiffBytes = int64(v) * 1073741824
		}
	}

	nowMs := time.Now().UnixMilli()
	summary := buildClientsSummary(all, onlineSet, nowMs, expireDiffMs, trafficDiffBytes)

	needle := strings.ToLower(strings.TrimSpace(params.Search))

	filtered := make([]ClientWithAttachments, 0, len(all))
	for _, c := range all {
		if needle != "" && !clientMatchesSearch(c, needle) {
			continue
		}
		if len(protocols) > 0 && !clientMatchesAnyProtocol(c, protocols, protocolByInbound) {
			continue
		}
		if len(inboundIDs) > 0 && !clientMatchesAnyInbound(c, inboundIDs) {
			continue
		}
		if len(buckets) > 0 && !clientMatchesAnyBucket(c, buckets, onlineSet, nowMs, expireDiffMs, trafficDiffBytes) {
			continue
		}
		if !clientMatchesExpiryRange(c, params.ExpiryFrom, params.ExpiryTo) {
			continue
		}
		if !clientMatchesUsageRange(c, params.UsageFrom, params.UsageTo) {
			continue
		}
		if !clientMatchesAutoRenew(c, params.AutoRenew) {
			continue
		}
		if !clientMatchesHasTgID(c, params.HasTgID) {
			continue
		}
		if !clientMatchesHasComment(c, params.HasComment) {
			continue
		}
		if !clientMatchesAnyGroup(c, params.Group) {
			continue
		}
		filtered = append(filtered, c)
	}

	sortClients(filtered, params.Sort, params.Order)

	filteredCount := len(filtered)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > filteredCount {
		start = filteredCount
	}
	if end > filteredCount {
		end = filteredCount
	}
	pageRows := filtered[start:end]

	items := make([]ClientSlim, 0, len(pageRows))
	for _, c := range pageRows {
		items = append(items, toClientSlim(c))
	}

	groupRows, gErr := s.ListGroups()
	if gErr != nil {
		return nil, gErr
	}
	groups := make([]string, 0, len(groupRows))
	for _, g := range groupRows {
		groups = append(groups, g.Name)
	}

	return &ClientPageResponse{
		Items:    items,
		Total:    total,
		Filtered: filteredCount,
		Page:     page,
		PageSize: pageSize,
		Summary:  summary,
		Groups:   groups,
	}, nil
}

type GroupSummary struct {
	Name        string `json:"name"`
	ClientCount int    `json:"clientCount"`
}

func (s *ClientService) ListGroups() ([]GroupSummary, error) {
	db := database.GetDB()
	var derived []GroupSummary
	if err := db.Model(&model.ClientRecord{}).
		Select("group_name AS name, COUNT(*) AS client_count").
		Where("group_name <> ''").
		Group("group_name").
		Scan(&derived).Error; err != nil {
		return nil, err
	}
	var stored []model.ClientGroup
	if err := db.Find(&stored).Error; err != nil {
		return nil, err
	}
	merged := make(map[string]int, len(derived)+len(stored))
	for _, g := range stored {
		merged[g.Name] = 0
	}
	for _, g := range derived {
		merged[g.Name] = g.ClientCount
	}
	out := make([]GroupSummary, 0, len(merged))
	for name, count := range merged {
		out = append(out, GroupSummary{Name: name, ClientCount: count})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func (s *ClientService) EmailsByGroup(name string) ([]string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return []string{}, nil
	}
	db := database.GetDB()
	var emails []string
	if err := db.Model(&model.ClientRecord{}).
		Where("group_name = ?", name).
		Order("email ASC").
		Pluck("email", &emails).Error; err != nil {
		return nil, err
	}
	if emails == nil {
		emails = []string{}
	}
	return emails, nil
}

func (s *ClientService) BulkResetTraffic(inboundSvc *InboundService, emails []string) (int, error) {
	if len(emails) == 0 {
		return 0, nil
	}
	count := 0
	for _, email := range emails {
		if _, err := s.ResetTrafficByEmail(inboundSvc, email); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *ClientService) CreateGroup(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return common.NewError("group name is required")
	}
	db := database.GetDB()
	var count int64
	if err := db.Model(&model.ClientGroup{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return common.NewError("group already exists")
	}
	return db.Create(&model.ClientGroup{Name: name}).Error
}

func (s *ClientService) RenameGroup(oldName, newName string) (int, error) {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" {
		return 0, common.NewError("old group name is required")
	}
	if newName == "" {
		return 0, common.NewError("new group name is required")
	}
	if oldName == newName {
		return 0, nil
	}
	return s.replaceGroupValue(oldName, newName)
}

func (s *ClientService) DeleteGroup(name string) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, common.NewError("group name is required")
	}
	return s.replaceGroupValue(name, "")
}

func (s *ClientService) RemoveFromGroup(emails []string) (int, error) {
	return s.AddToGroup(emails, "")
}

func (s *ClientService) AddToGroup(emails []string, group string) (int, error) {
	group = strings.TrimSpace(group)
	if len(emails) == 0 {
		return 0, nil
	}
	db := database.GetDB()

	if group != "" {
		var exists int64
		if err := db.Model(&model.ClientGroup{}).Where("name = ?", group).Count(&exists).Error; err != nil {
			return 0, err
		}
		if exists == 0 {
			var derived int64
			if err := db.Model(&model.ClientRecord{}).Where("group_name = ?", group).Count(&derived).Error; err != nil {
				return 0, err
			}
			if derived == 0 {
				if err := db.Create(&model.ClientGroup{Name: group}).Error; err != nil {
					return 0, err
				}
			}
		}
	}

	var records []model.ClientRecord
	if err := db.Where("email IN ?", emails).Find(&records).Error; err != nil {
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}
	affectedEmails := make([]string, 0, len(records))
	for _, r := range records {
		affectedEmails = append(affectedEmails, r.Email)
	}

	tx := db.Begin()
	if err := tx.Model(&model.ClientRecord{}).
		Where("email IN ?", affectedEmails).
		UpdateColumn("group_name", group).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	var inboundIDs []int
	if err := tx.Table("client_inbounds").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("clients.email IN ?", affectedEmails).
		Distinct("client_inbounds.inbound_id").
		Pluck("inbound_id", &inboundIDs).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	emailSet := make(map[string]struct{}, len(affectedEmails))
	for _, e := range affectedEmails {
		emailSet[e] = struct{}{}
	}

	for _, ibID := range inboundIDs {
		var ib model.Inbound
		if err := tx.First(&ib, ibID).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
		var settings map[string]any
		if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
			continue
		}
		clients, ok := settings["clients"].([]any)
		if !ok {
			continue
		}
		modified := false
		for i := range clients {
			cm, ok := clients[i].(map[string]any)
			if !ok {
				continue
			}
			email, _ := cm["email"].(string)
			if _, hit := emailSet[email]; !hit {
				continue
			}
			if group == "" {
				delete(cm, "group")
			} else {
				cm["group"] = group
			}
			clients[i] = cm
			modified = true
		}
		if modified {
			settings["clients"] = clients
			newSettings, err := json.Marshal(settings)
			if err != nil {
				continue
			}
			ib.Settings = string(newSettings)
			if err := tx.Save(&ib).Error; err != nil {
				tx.Rollback()
				return 0, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return len(records), nil
}

func (s *ClientService) replaceGroupValue(oldName, newName string) (int, error) {
	db := database.GetDB()
	if newName == "" {
		if err := db.Where("name = ?", oldName).Delete(&model.ClientGroup{}).Error; err != nil {
			return 0, err
		}
	} else {
		if err := db.Model(&model.ClientGroup{}).Where("name = ?", oldName).Update("name", newName).Error; err != nil {
			return 0, err
		}
	}
	var records []model.ClientRecord
	if err := db.Where("group_name = ?", oldName).Find(&records).Error; err != nil {
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}
	affectedEmails := make([]string, 0, len(records))
	for _, r := range records {
		affectedEmails = append(affectedEmails, r.Email)
	}

	tx := db.Begin()
	if err := tx.Model(&model.ClientRecord{}).
		Where("group_name = ?", oldName).
		UpdateColumn("group_name", newName).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	var inboundIDs []int
	if err := tx.Table("client_inbounds").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("clients.email IN ?", affectedEmails).
		Distinct("client_inbounds.inbound_id").
		Pluck("inbound_id", &inboundIDs).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	for _, ibID := range inboundIDs {
		var ib model.Inbound
		if err := tx.First(&ib, ibID).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
		var settings map[string]any
		if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
			continue
		}
		clients, ok := settings["clients"].([]any)
		if !ok {
			continue
		}
		modified := false
		for i := range clients {
			cm, ok := clients[i].(map[string]any)
			if !ok {
				continue
			}
			if g, ok := cm["group"].(string); ok && g == oldName {
				if newName == "" {
					delete(cm, "group")
				} else {
					cm["group"] = newName
				}
				clients[i] = cm
				modified = true
			}
		}
		if modified {
			settings["clients"] = clients
			newSettings, err := json.Marshal(settings)
			if err != nil {
				continue
			}
			ib.Settings = string(newSettings)
			if err := tx.Save(&ib).Error; err != nil {
				tx.Rollback()
				return 0, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return len(records), nil
}

func buildClientsSummary(all []ClientWithAttachments, onlineSet map[string]struct{}, nowMs, expireDiffMs, trafficDiffBytes int64) ClientsSummary {
	s := ClientsSummary{
		Total:    len(all),
		Online:   []string{},
		Depleted: []string{},
		Expiring: []string{},
		Deactive: []string{},
	}
	for _, c := range all {
		used := int64(0)
		if c.Traffic != nil {
			used = c.Traffic.Up + c.Traffic.Down
		}
		exhausted := c.TotalGB > 0 && used >= c.TotalGB
		expired := c.ExpiryTime > 0 && c.ExpiryTime <= nowMs
		if c.Enable {
			if _, ok := onlineSet[c.Email]; ok {
				s.Online = append(s.Online, c.Email)
			}
		}
		if exhausted || expired {
			s.Depleted = append(s.Depleted, c.Email)
			continue
		}
		if !c.Enable {
			s.Deactive = append(s.Deactive, c.Email)
			continue
		}
		nearExpiry := c.ExpiryTime > 0 && c.ExpiryTime-nowMs < expireDiffMs
		nearLimit := c.TotalGB > 0 && c.TotalGB-used < trafficDiffBytes
		if nearExpiry || nearLimit {
			s.Expiring = append(s.Expiring, c.Email)
		} else {
			s.Active++
		}
	}
	return s
}

func toClientSlim(c ClientWithAttachments) ClientSlim {
	return ClientSlim{
		Email:      c.Email,
		SubID:      c.SubID,
		Enable:     c.Enable,
		TotalGB:    c.TotalGB,
		ExpiryTime: c.ExpiryTime,
		LimitIP:    c.LimitIP,
		Reset:      c.Reset,
		Group:      c.Group,
		Comment:    c.Comment,
		InboundIds: c.InboundIds,
		Traffic:    c.Traffic,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

func clientMatchesSearch(c ClientWithAttachments, needle string) bool {
	if needle == "" {
		return true
	}
	candidates := [...]string{c.Email, c.SubID, c.Comment, c.UUID, c.Password, c.Auth}
	for _, v := range candidates {
		if v != "" && strings.Contains(strings.ToLower(v), needle) {
			return true
		}
	}
	return false
}

// parseCSVStrings splits a comma-separated list, trims/lower-cases each item,
// and drops blanks. Returns nil when the input has no usable entries — the
// caller can then skip the predicate entirely.
func parseCSVStrings(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.ToLower(strings.TrimSpace(p))
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseCSVInts is parseCSVStrings for positive integer IDs; non-numeric or
// non-positive entries are silently dropped.
func parseCSVInts(raw string) []int {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func clientMatchesAnyProtocol(c ClientWithAttachments, protocols []string, byInbound map[int]string) bool {
	for _, id := range c.InboundIds {
		p := byInbound[id]
		if p == "" {
			continue
		}
		if slices.Contains(protocols, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func clientMatchesAnyInbound(c ClientWithAttachments, inboundIds []int) bool {
	for _, id := range c.InboundIds {
		if slices.Contains(inboundIds, id) {
			return true
		}
	}
	return false
}

func clientMatchesAnyBucket(c ClientWithAttachments, buckets []string, onlineSet map[string]struct{}, nowMs, expireDiffMs, trafficDiffBytes int64) bool {
	for _, b := range buckets {
		if clientMatchesBucket(c, b, onlineSet, nowMs, expireDiffMs, trafficDiffBytes) {
			return true
		}
	}
	return false
}

func clientMatchesExpiryRange(c ClientWithAttachments, fromMs, toMs int64) bool {
	if fromMs <= 0 && toMs <= 0 {
		return true
	}
	// expiryTime of 0 means "never expires"; treat it as outside any bounded
	// range so users filtering by date see only clients with concrete expiries.
	if c.ExpiryTime == 0 {
		return false
	}
	// Negative expiry is the "delayed start" sentinel; same treatment as never.
	if c.ExpiryTime < 0 {
		return false
	}
	if fromMs > 0 && c.ExpiryTime < fromMs {
		return false
	}
	if toMs > 0 && c.ExpiryTime > toMs {
		return false
	}
	return true
}

func clientMatchesUsageRange(c ClientWithAttachments, fromBytes, toBytes int64) bool {
	if fromBytes <= 0 && toBytes <= 0 {
		return true
	}
	used := int64(0)
	if c.Traffic != nil {
		used = c.Traffic.Up + c.Traffic.Down
	}
	if fromBytes > 0 && used < fromBytes {
		return false
	}
	if toBytes > 0 && used > toBytes {
		return false
	}
	return true
}

func clientMatchesAutoRenew(c ClientWithAttachments, mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "on":
		return c.Reset > 0
	case "off":
		return c.Reset <= 0
	}
	return true
}

func clientMatchesHasTgID(c ClientWithAttachments, mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "yes":
		return c.TgID != 0
	case "no":
		return c.TgID == 0
	}
	return true
}

func clientMatchesHasComment(c ClientWithAttachments, mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "yes":
		return strings.TrimSpace(c.Comment) != ""
	case "no":
		return strings.TrimSpace(c.Comment) == ""
	}
	return true
}

func clientMatchesAnyGroup(c ClientWithAttachments, csv string) bool {
	groups := parseCSVStrings(csv)
	if len(groups) == 0 {
		return true
	}
	current := strings.TrimSpace(c.Group)
	for _, g := range groups {
		if g == "" {
			if current == "" {
				return true
			}
			continue
		}
		if strings.EqualFold(g, current) {
			return true
		}
	}
	return false
}

func clientMatchesBucket(c ClientWithAttachments, bucket string, onlineSet map[string]struct{}, nowMs, expireDiffMs, trafficDiffBytes int64) bool {
	if bucket == "" {
		return true
	}
	used := int64(0)
	if c.Traffic != nil {
		used = c.Traffic.Up + c.Traffic.Down
	}
	exhausted := c.TotalGB > 0 && used >= c.TotalGB
	expired := c.ExpiryTime > 0 && c.ExpiryTime <= nowMs
	switch bucket {
	case "online":
		if onlineSet == nil {
			return false
		}
		_, ok := onlineSet[c.Email]
		return ok && c.Enable
	case "depleted":
		return exhausted || expired
	case "deactive":
		return !c.Enable
	case "active":
		return c.Enable && !exhausted && !expired
	case "expiring":
		if !c.Enable || exhausted || expired {
			return false
		}
		nearExpiry := c.ExpiryTime > 0 && c.ExpiryTime-nowMs < expireDiffMs
		nearLimit := c.TotalGB > 0 && c.TotalGB-used < trafficDiffBytes
		return nearExpiry || nearLimit
	}
	return true
}

func sortClients(rows []ClientWithAttachments, sortKey, order string) {
	if sortKey == "" {
		return
	}
	desc := order == "descend"
	less := func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch sortKey {
		case "enable":
			if a.Enable == b.Enable {
				return false
			}
			return !a.Enable && b.Enable
		case "email":
			return strings.ToLower(a.Email) < strings.ToLower(b.Email)
		case "inboundIds":
			return len(a.InboundIds) < len(b.InboundIds)
		case "traffic":
			ua := int64(0)
			if a.Traffic != nil {
				ua = a.Traffic.Up + a.Traffic.Down
			}
			ub := int64(0)
			if b.Traffic != nil {
				ub = b.Traffic.Up + b.Traffic.Down
			}
			return ua < ub
		case "remaining":
			ra := int64(1<<62 - 1)
			if a.TotalGB > 0 {
				used := int64(0)
				if a.Traffic != nil {
					used = a.Traffic.Up + a.Traffic.Down
				}
				ra = a.TotalGB - used
			}
			rb := int64(1<<62 - 1)
			if b.TotalGB > 0 {
				used := int64(0)
				if b.Traffic != nil {
					used = b.Traffic.Up + b.Traffic.Down
				}
				rb = b.TotalGB - used
			}
			return ra < rb
		case "expiryTime":
			ea := int64(1<<62 - 1)
			if a.ExpiryTime > 0 {
				ea = a.ExpiryTime
			}
			eb := int64(1<<62 - 1)
			if b.ExpiryTime > 0 {
				eb = b.ExpiryTime
			}
			return ea < eb
		case "createdAt":
			if a.CreatedAt == b.CreatedAt {
				return a.Id < b.Id
			}
			return a.CreatedAt < b.CreatedAt
		case "updatedAt":
			if a.UpdatedAt == b.UpdatedAt {
				return a.Id < b.Id
			}
			return a.UpdatedAt < b.UpdatedAt
		case "lastOnline":
			la := int64(0)
			if a.Traffic != nil {
				la = a.Traffic.LastOnline
			}
			lb := int64(0)
			if b.Traffic != nil {
				lb = b.Traffic.LastOnline
			}
			if la == lb {
				return a.Id < b.Id
			}
			return la < lb
		}
		return false
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if desc {
			return less(j, i)
		}
		return less(i, j)
	})
}

// BulkAdjustResult is returned by BulkAdjust to report how many clients were
// successfully updated and which were skipped (typically because the field
// being adjusted was unlimited for that client) or failed.
type BulkAdjustResult struct {
	Adjusted int                `json:"adjusted"`
	Skipped  []BulkAdjustReport `json:"skipped,omitempty"`
}

type BulkAdjustReport struct {
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

type bulkAdjustEntry struct {
	record      *model.ClientRecord
	applyExpiry bool
	newExpiry   int64
	applyTotal  bool
	newTotal    int64
}

// BulkAdjust shifts ExpiryTime by addDays (days) and TotalGB by addBytes
// for every email in the list. Clients whose corresponding field is
// unlimited (0) are skipped — bulk extend should not accidentally
// limit an unlimited client. addDays and addBytes may be negative.
//
// Like BulkDelete, the work is grouped by inbound so each inbound's
// settings JSON is parsed and written exactly once regardless of how
// many target emails it contains.
func (s *ClientService) BulkAdjust(inboundSvc *InboundService, emails []string, addDays int, addBytes int64) (BulkAdjustResult, bool, error) {
	result := BulkAdjustResult{}
	if len(emails) == 0 {
		return result, false, nil
	}
	if addDays == 0 && addBytes == 0 {
		return result, false, common.NewError("no adjustment specified")
	}

	addExpiryMs := int64(addDays) * 24 * 60 * 60 * 1000

	seen := map[string]struct{}{}
	cleanEmails := make([]string, 0, len(emails))
	for _, e := range emails {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		cleanEmails = append(cleanEmails, e)
	}
	if len(cleanEmails) == 0 {
		return result, false, nil
	}

	db := database.GetDB()

	var records []model.ClientRecord
	if err := db.Where("email IN ?", cleanEmails).Find(&records).Error; err != nil {
		return result, false, err
	}
	recordsByEmail := make(map[string]*model.ClientRecord, len(records))
	for i := range records {
		recordsByEmail[records[i].Email] = &records[i]
	}

	skippedReasons := map[string]string{}
	for _, email := range cleanEmails {
		if _, ok := recordsByEmail[email]; !ok {
			skippedReasons[email] = "client not found"
		}
	}

	plan := map[string]*bulkAdjustEntry{}
	for email, rec := range recordsByEmail {
		entry := &bulkAdjustEntry{record: rec}
		if addDays != 0 {
			switch {
			case rec.ExpiryTime == 0:
				if _, exists := skippedReasons[email]; !exists {
					skippedReasons[email] = "unlimited expiry"
				}
			case rec.ExpiryTime > 0:
				next := rec.ExpiryTime + addExpiryMs
				if next <= 0 {
					if _, exists := skippedReasons[email]; !exists {
						skippedReasons[email] = "reduction exceeds remaining time"
					}
				} else {
					entry.applyExpiry = true
					entry.newExpiry = next
				}
			default:
				next := rec.ExpiryTime - addExpiryMs
				if next >= 0 {
					if _, exists := skippedReasons[email]; !exists {
						skippedReasons[email] = "reduction exceeds delay window"
					}
				} else {
					entry.applyExpiry = true
					entry.newExpiry = next
				}
			}
		}
		if addBytes != 0 {
			if rec.TotalGB == 0 {
				if _, exists := skippedReasons[email]; !exists {
					skippedReasons[email] = "unlimited traffic"
				}
			} else {
				next := max(rec.TotalGB+addBytes, 0)
				entry.applyTotal = true
				entry.newTotal = next
			}
		}
		if entry.applyExpiry || entry.applyTotal {
			plan[email] = entry
		}
	}

	if len(plan) == 0 {
		for email, reason := range skippedReasons {
			result.Skipped = append(result.Skipped, BulkAdjustReport{Email: email, Reason: reason})
		}
		return result, false, nil
	}

	plannedIds := make([]int, 0, len(plan))
	recordIdToEmail := make(map[int]string, len(plan))
	for email, entry := range plan {
		plannedIds = append(plannedIds, entry.record.Id)
		recordIdToEmail[entry.record.Id] = email
	}

	var mappings []model.ClientInbound
	if err := db.Where("client_id IN ?", plannedIds).Find(&mappings).Error; err != nil {
		return result, false, err
	}
	emailsByInbound := map[int][]string{}
	for _, m := range mappings {
		email, ok := recordIdToEmail[m.ClientId]
		if !ok {
			continue
		}
		emailsByInbound[m.InboundId] = append(emailsByInbound[m.InboundId], email)
	}

	needRestart := false
	for inboundId, ibEmails := range emailsByInbound {
		ibRes := s.bulkAdjustInboundClients(inboundSvc, inboundId, ibEmails, plan)
		if ibRes.needRestart {
			needRestart = true
		}
		for email, reason := range ibRes.perEmailSkipped {
			if _, already := skippedReasons[email]; !already {
				skippedReasons[email] = reason
			}
		}
	}

	for email, entry := range plan {
		if _, skipped := skippedReasons[email]; skipped {
			continue
		}
		updates := map[string]any{}
		if entry.applyExpiry {
			updates["expiry_time"] = entry.newExpiry
		}
		if entry.applyTotal {
			updates["total"] = entry.newTotal
		}
		if len(updates) == 0 {
			continue
		}
		if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).Updates(updates).Error; err != nil {
			if _, already := skippedReasons[email]; !already {
				skippedReasons[email] = err.Error()
			}
			continue
		}
		result.Adjusted++
	}

	for email, reason := range skippedReasons {
		result.Skipped = append(result.Skipped, BulkAdjustReport{Email: email, Reason: reason})
	}
	return result, needRestart, nil
}

type bulkInboundAdjustResult struct {
	perEmailSkipped map[string]string
	needRestart     bool
}

// bulkAdjustInboundClients applies expiry/total deltas to multiple clients
// inside a single inbound's settings JSON. The xray runtime is updated
// only for remote-node inbounds; local nodes do not need a notification
// because the AddUser payload does not include totalGB/expiryTime —
// changing those fields is identity-preserving and the panel's traffic
// enforcement loop picks up the new limits from ClientTraffic directly.
func (s *ClientService) bulkAdjustInboundClients(
	inboundSvc *InboundService,
	inboundId int,
	emails []string,
	plan map[string]*bulkAdjustEntry,
) bulkInboundAdjustResult {
	res := bulkInboundAdjustResult{perEmailSkipped: map[string]string{}}

	defer lockInbound(inboundId).Unlock()

	oldInbound, err := inboundSvc.GetInbound(inboundId)
	if err != nil {
		logger.Error("Load Old Data Error")
		for _, e := range emails {
			res.perEmailSkipped[e] = err.Error()
		}
		return res
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(oldInbound.Settings), &settings); err != nil {
		for _, e := range emails {
			res.perEmailSkipped[e] = err.Error()
		}
		return res
	}

	clientKey := "id"
	switch oldInbound.Protocol {
	case model.Trojan:
		clientKey = "password"
	case model.Shadowsocks:
		clientKey = "email"
	case model.Hysteria:
		clientKey = "auth"
	}

	keyToEmail := make(map[string]string, len(emails))
	for _, email := range emails {
		entry := plan[email]
		if entry == nil {
			res.perEmailSkipped[email] = "client not found"
			continue
		}
		key := clientKeyForProtocol(oldInbound.Protocol, entry.record)
		if key == "" {
			res.perEmailSkipped[email] = "missing client key for protocol"
			continue
		}
		keyToEmail[key] = email
	}

	interfaceClients, _ := settings["clients"].([]any)
	foundEmails := map[string]bool{}
	nowMs := time.Now().Unix() * 1000
	for i, client := range interfaceClients {
		c, ok := client.(map[string]any)
		if !ok {
			continue
		}
		cKey, _ := c[clientKey].(string)
		targetEmail, found := keyToEmail[cKey]
		if !found {
			continue
		}
		entry := plan[targetEmail]
		if entry.applyExpiry {
			c["expiryTime"] = entry.newExpiry
		}
		if entry.applyTotal {
			c["totalGB"] = entry.newTotal
		}
		c["updated_at"] = nowMs
		interfaceClients[i] = c
		foundEmails[targetEmail] = true
	}

	for _, email := range keyToEmail {
		if !foundEmails[email] {
			res.perEmailSkipped[email] = "Client Not Found In Inbound"
		}
	}

	if len(foundEmails) == 0 {
		return res
	}

	settings["clients"] = interfaceClients
	newSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		for email := range foundEmails {
			res.perEmailSkipped[email] = err.Error()
		}
		return res
	}
	oldInbound.Settings = string(newSettings)

	if oldInbound.NodeID != nil {
		rt, rterr := inboundSvc.runtimeFor(oldInbound)
		if rterr != nil {
			for email := range foundEmails {
				res.perEmailSkipped[email] = rterr.Error()
				delete(foundEmails, email)
			}
		} else {
			for email := range foundEmails {
				entry := plan[email]
				updated := *entry.record.ToClient()
				if entry.applyExpiry {
					updated.ExpiryTime = entry.newExpiry
				}
				if entry.applyTotal {
					updated.TotalGB = entry.newTotal
				}
				updated.UpdatedAt = nowMs
				if err1 := rt.UpdateUser(context.Background(), oldInbound, email, updated); err1 != nil {
					res.perEmailSkipped[email] = err1.Error()
					delete(foundEmails, email)
				}
			}
		}
	}

	db := database.GetDB()
	if err := db.Save(oldInbound).Error; err != nil {
		for email := range foundEmails {
			if _, skip := res.perEmailSkipped[email]; !skip {
				res.perEmailSkipped[email] = err.Error()
			}
		}
		return res
	}

	finalClients, gcErr := inboundSvc.GetClients(oldInbound)
	if gcErr == nil {
		if syncErr := s.SyncInbound(db, inboundId, finalClients); syncErr != nil {
			logger.Warning("bulkAdjust SyncInbound:", syncErr)
		}
	}

	return res
}

// BulkDeleteResult mirrors BulkAdjustResult: total deleted plus per-email
// skip reasons when an email could not be processed.
type BulkDeleteResult struct {
	Deleted int                `json:"deleted"`
	Skipped []BulkDeleteReport `json:"skipped,omitempty"`
}

type BulkDeleteReport struct {
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

// BulkDelete removes every client in the list in one optimized pass.
// Instead of running the full single-delete pipeline N times (which would
// re-read, re-parse, and re-write each inbound's settings JSON for every
// email), it groups emails by inbound and performs a single
// read-modify-write per inbound. Per-row DB cleanups are also batched with
// IN-clause queries at the end. Errors on a particular email are recorded
// in the Skipped list and processing continues for the rest.
func (s *ClientService) BulkDelete(inboundSvc *InboundService, emails []string, keepTraffic bool) (BulkDeleteResult, bool, error) {
	result := BulkDeleteResult{}

	seen := map[string]struct{}{}
	cleanEmails := make([]string, 0, len(emails))
	for _, e := range emails {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		cleanEmails = append(cleanEmails, e)
	}
	if len(cleanEmails) == 0 {
		return result, false, nil
	}

	db := database.GetDB()

	var records []model.ClientRecord
	if err := db.Where("email IN ?", cleanEmails).Find(&records).Error; err != nil {
		return result, false, err
	}
	recordsByEmail := make(map[string]*model.ClientRecord, len(records))
	for i := range records {
		recordsByEmail[records[i].Email] = &records[i]
		tombstoneClientEmail(records[i].Email)
	}

	skippedReasons := map[string]string{}
	for _, email := range cleanEmails {
		if _, ok := recordsByEmail[email]; !ok {
			skippedReasons[email] = "client not found"
		}
	}

	clientIds := make([]int, 0, len(recordsByEmail))
	recordIdToEmail := make(map[int]string, len(recordsByEmail))
	for _, r := range recordsByEmail {
		clientIds = append(clientIds, r.Id)
		recordIdToEmail[r.Id] = r.Email
	}

	emailsByInbound := map[int][]string{}
	if len(clientIds) > 0 {
		var mappings []model.ClientInbound
		if err := db.Where("client_id IN ?", clientIds).Find(&mappings).Error; err != nil {
			return result, false, err
		}
		for _, m := range mappings {
			email, ok := recordIdToEmail[m.ClientId]
			if !ok {
				continue
			}
			emailsByInbound[m.InboundId] = append(emailsByInbound[m.InboundId], email)
		}
	}

	needRestart := false
	for inboundId, ibEmails := range emailsByInbound {
		ibResult := s.bulkDelInboundClients(inboundSvc, inboundId, ibEmails, recordsByEmail, false)
		if ibResult.needRestart {
			needRestart = true
		}
		for email, reason := range ibResult.perEmailSkipped {
			if _, already := skippedReasons[email]; !already {
				skippedReasons[email] = reason
			}
		}
	}

	successEmails := make([]string, 0, len(recordsByEmail))
	successIds := make([]int, 0, len(recordsByEmail))
	for email, rec := range recordsByEmail {
		if _, skipped := skippedReasons[email]; skipped {
			continue
		}
		successEmails = append(successEmails, email)
		successIds = append(successIds, rec.Id)
	}

	if len(successIds) > 0 {
		if err := db.Where("client_id IN ?", successIds).Delete(&model.ClientInbound{}).Error; err != nil {
			return result, needRestart, err
		}
		if !keepTraffic && len(successEmails) > 0 {
			if err := db.Where("email IN ?", successEmails).Delete(&xray.ClientTraffic{}).Error; err != nil {
				return result, needRestart, err
			}
			if err := db.Where("client_email IN ?", successEmails).Delete(&model.InboundClientIps{}).Error; err != nil {
				return result, needRestart, err
			}
		}
		if err := db.Where("id IN ?", successIds).Delete(&model.ClientRecord{}).Error; err != nil {
			return result, needRestart, err
		}
	}

	result.Deleted = len(successEmails)
	for email, reason := range skippedReasons {
		result.Skipped = append(result.Skipped, BulkDeleteReport{Email: email, Reason: reason})
	}
	return result, needRestart, nil
}

type bulkInboundDeleteResult struct {
	perEmailSkipped map[string]string
	needRestart     bool
}

// bulkDelInboundClients removes multiple clients from a single inbound's
// settings JSON in one read-modify-write cycle, runs the xray runtime
// RemoveUser/DeleteUser calls, and persists the inbound. The returned map
// holds per-email failure reasons; emails not present in the map are
// considered successful for this inbound.
func (s *ClientService) bulkDelInboundClients(
	inboundSvc *InboundService,
	inboundId int,
	emails []string,
	records map[string]*model.ClientRecord,
	keepTraffic bool,
) bulkInboundDeleteResult {
	res := bulkInboundDeleteResult{perEmailSkipped: map[string]string{}}

	defer lockInbound(inboundId).Unlock()

	oldInbound, err := inboundSvc.GetInbound(inboundId)
	if err != nil {
		logger.Error("Load Old Data Error")
		for _, e := range emails {
			res.perEmailSkipped[e] = err.Error()
		}
		return res
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(oldInbound.Settings), &settings); err != nil {
		for _, e := range emails {
			res.perEmailSkipped[e] = err.Error()
		}
		return res
	}

	clientKey := "id"
	switch oldInbound.Protocol {
	case model.Trojan:
		clientKey = "password"
	case model.Shadowsocks:
		clientKey = "email"
	case model.Hysteria:
		clientKey = "auth"
	}

	keyToEmail := make(map[string]string, len(emails))
	for _, email := range emails {
		rec := records[email]
		if rec == nil {
			res.perEmailSkipped[email] = "client not found"
			continue
		}
		key := clientKeyForProtocol(oldInbound.Protocol, rec)
		if key == "" {
			res.perEmailSkipped[email] = "missing client key for protocol"
			continue
		}
		keyToEmail[key] = email
	}

	interfaceClients, _ := settings["clients"].([]any)
	newClients := make([]any, 0, len(interfaceClients))
	foundEmails := map[string]bool{}
	enableByEmail := map[string]bool{}
	for _, client := range interfaceClients {
		c, ok := client.(map[string]any)
		if !ok {
			newClients = append(newClients, client)
			continue
		}
		cKey, _ := c[clientKey].(string)
		if targetEmail, found := keyToEmail[cKey]; found {
			foundEmails[targetEmail] = true
			if em, _ := c["email"].(string); em != "" {
				en, _ := c["enable"].(bool)
				enableByEmail[em] = en
			}
			continue
		}
		newClients = append(newClients, client)
	}

	for _, email := range keyToEmail {
		if !foundEmails[email] {
			res.perEmailSkipped[email] = "Client Not Found In Inbound"
		}
	}

	db := database.GetDB()
	newClients = compactOrphans(db, newClients)
	if newClients == nil {
		newClients = []any{}
	}
	settings["clients"] = newClients
	newSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		for email := range foundEmails {
			if _, skip := res.perEmailSkipped[email]; !skip {
				res.perEmailSkipped[email] = err.Error()
			}
		}
		return res
	}
	oldInbound.Settings = string(newSettings)

	foundList := make([]string, 0, len(foundEmails))
	for email := range foundEmails {
		foundList = append(foundList, email)
	}

	notDepletedByEmail := map[string]bool{}
	if len(foundList) > 0 {
		type trafficRow struct {
			Email  string
			Enable bool
		}
		var rows []trafficRow
		if err := db.Model(xray.ClientTraffic{}).
			Where("email IN ?", foundList).
			Select("email, enable").
			Scan(&rows).Error; err == nil {
			for _, r := range rows {
				notDepletedByEmail[r.Email] = r.Enable
			}
		}
	}

	for email := range foundEmails {
		shared, sharedErr := inboundSvc.emailUsedByOtherInbounds(email, inboundId)
		if sharedErr != nil {
			res.perEmailSkipped[email] = sharedErr.Error()
			delete(foundEmails, email)
			continue
		}
		if shared || keepTraffic {
			continue
		}
		if delErr := inboundSvc.DelClientIPs(db, email); delErr != nil {
			logger.Error("Error in delete client IPs")
			res.perEmailSkipped[email] = delErr.Error()
			delete(foundEmails, email)
			continue
		}
		if delErr := inboundSvc.DelClientStat(db, email); delErr != nil {
			logger.Error("Delete stats Data Error")
			res.perEmailSkipped[email] = delErr.Error()
			delete(foundEmails, email)
			continue
		}
	}

	if oldInbound.NodeID == nil {
		rt, rterr := inboundSvc.runtimeFor(oldInbound)
		if rterr != nil {
			res.needRestart = true
		} else {
			for email := range foundEmails {
				if !enableByEmail[email] || !notDepletedByEmail[email] {
					continue
				}
				err1 := rt.RemoveUser(context.Background(), oldInbound, email)
				if err1 == nil {
					logger.Debug("Client deleted on", rt.Name(), ":", email)
				} else if strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", email)) {
					logger.Debug("User is already deleted. Nothing to do more...")
				} else {
					logger.Debug("Error in deleting client on", rt.Name(), ":", err1)
					res.needRestart = true
				}
			}
		}
	} else {
		rt, rterr := inboundSvc.runtimeFor(oldInbound)
		if rterr != nil {
			for email := range foundEmails {
				res.perEmailSkipped[email] = rterr.Error()
				delete(foundEmails, email)
			}
		} else {
			for email := range foundEmails {
				if err1 := rt.DeleteUser(context.Background(), oldInbound, email); err1 != nil {
					res.perEmailSkipped[email] = err1.Error()
					delete(foundEmails, email)
				}
			}
		}
	}

	if err := db.Save(oldInbound).Error; err != nil {
		for email := range foundEmails {
			if _, skip := res.perEmailSkipped[email]; !skip {
				res.perEmailSkipped[email] = err.Error()
			}
		}
		return res
	}

	finalClients, err := inboundSvc.GetClients(oldInbound)
	if err != nil {
		return res
	}
	if err := s.SyncInbound(db, inboundId, finalClients); err != nil {
		return res
	}

	return res
}

// BulkCreateResult mirrors BulkAdjustResult for the create flow.
type BulkCreateResult struct {
	Created int                `json:"created"`
	Skipped []BulkCreateReport `json:"skipped,omitempty"`
}

type BulkCreateReport struct {
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

// BulkCreate iterates payloads sequentially. Each item is the same shape
// the single-create endpoint accepts, so callers can submit a heterogeneous
// list (different inboundIds, plans, etc.) in one round-trip.
func (s *ClientService) BulkCreate(inboundSvc *InboundService, payloads []ClientCreatePayload) (BulkCreateResult, bool, error) {
	result := BulkCreateResult{}
	needRestart := false
	for i := range payloads {
		p := payloads[i]
		email := strings.TrimSpace(p.Client.Email)
		nr, err := s.Create(inboundSvc, &p)
		if err != nil {
			if email == "" {
				email = "(missing email)"
			}
			result.Skipped = append(result.Skipped, BulkCreateReport{Email: email, Reason: err.Error()})
			continue
		}
		if nr {
			needRestart = true
		}
		result.Created++
	}
	return result, needRestart, nil
}

func (s *ClientService) DelDepleted(inboundSvc *InboundService) (int, bool, error) {
	db := database.GetDB()
	now := time.Now().UnixMilli()
	depletedClause := "reset = 0 and ((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?))"

	var rows []xray.ClientTraffic
	if err := db.Where(depletedClause, now).Find(&rows).Error; err != nil {
		return 0, false, err
	}
	if len(rows) == 0 {
		return 0, false, nil
	}

	emails := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		if r.Email != "" {
			emails[r.Email] = struct{}{}
		}
	}

	needRestart := false
	deleted := 0
	for email := range emails {
		var rec model.ClientRecord
		if err := db.Where("email = ?", email).First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return deleted, needRestart, err
		}
		nr, err := s.Delete(inboundSvc, rec.Id, false)
		if err != nil {
			return deleted, needRestart, err
		}
		if nr {
			needRestart = true
		}
		deleted++
	}
	return deleted, needRestart, nil
}

func (s *ClientService) ResetAllClientTraffics(inboundSvc *InboundService, id int) error {
	return submitTrafficWrite(func() error {
		return s.resetAllClientTrafficsLocked(id)
	})
}

func (s *ClientService) resetAllClientTrafficsLocked(id int) error {
	db := database.GetDB()
	now := time.Now().Unix() * 1000

	if err := db.Transaction(func(tx *gorm.DB) error {
		whereText := "inbound_id "
		if id == -1 {
			whereText += " > ?"
		} else {
			whereText += " = ?"
		}

		result := tx.Model(xray.ClientTraffic{}).
			Where(whereText, id).
			Updates(map[string]any{"enable": true, "up": 0, "down": 0})

		if result.Error != nil {
			return result.Error
		}

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
	return nil
}

func (s *ClientService) ResetAllTraffics() (bool, error) {
	res := database.GetDB().Model(&xray.ClientTraffic{}).
		Where("1 = 1").
		Updates(map[string]any{"up": 0, "down": 0})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (s *ClientService) Detach(inboundSvc *InboundService, id int, inboundIds []int) (bool, error) {
	existing, err := s.GetByID(id)
	if err != nil {
		return false, err
	}
	currentIds, err := s.GetInboundIdsForRecord(id)
	if err != nil {
		return false, err
	}
	have := make(map[int]struct{}, len(currentIds))
	for _, x := range currentIds {
		have[x] = struct{}{}
	}

	needRestart := false
	for _, ibId := range inboundIds {
		if _, attached := have[ibId]; !attached {
			continue
		}
		inbound, getErr := inboundSvc.GetInbound(ibId)
		if getErr != nil {
			return needRestart, getErr
		}
		key := clientKeyForProtocol(inbound.Protocol, existing)
		if key == "" {
			continue
		}
		nr, delErr := s.DelInboundClient(inboundSvc, ibId, key, true)
		if delErr != nil {
			return needRestart, delErr
		}
		if nr {
			needRestart = true
		}
	}
	return needRestart, nil
}

func (s *ClientService) checkEmailsExistForClients(inboundSvc *InboundService, clients []model.Client, emailSubIDs map[string]string) (string, error) {
	if emailSubIDs == nil {
		var err error
		emailSubIDs, err = inboundSvc.getAllEmailSubIDs()
		if err != nil {
			return "", err
		}
	}
	seen := make(map[string]string, len(clients))
	for _, client := range clients {
		if client.Email == "" {
			continue
		}
		key := strings.ToLower(client.Email)
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

func (s *ClientService) AddInboundClient(inboundSvc *InboundService, data *model.Inbound) (bool, error) {
	return s.addInboundClient(inboundSvc, data, nil)
}

// addInboundClient is AddInboundClient with an optional precomputed email→subId
// map. Bulk callers pass a single snapshot so the global getAllEmailSubIDs scan
// runs once for the whole batch instead of once per target inbound; a nil map
// makes it compute its own (the single-add path).
func (s *ClientService) addInboundClient(inboundSvc *InboundService, data *model.Inbound, emailSubIDs map[string]string) (bool, error) {
	defer lockInbound(data.Id).Unlock()

	clients, err := inboundSvc.GetClients(data)
	if err != nil {
		return false, err
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(data.Settings), &settings)
	if err != nil {
		return false, err
	}

	interfaceClients := settings["clients"].([]any)
	nowTs := time.Now().Unix() * 1000
	for i := range interfaceClients {
		if cm, ok := interfaceClients[i].(map[string]any); ok {
			if _, ok2 := cm["created_at"]; !ok2 {
				cm["created_at"] = nowTs
			}
			cm["updated_at"] = nowTs
			existingSub, _ := cm["subId"].(string)
			if strings.TrimSpace(existingSub) == "" {
				cm["subId"] = random.NumLower(16)
			}
			interfaceClients[i] = cm
		}
	}
	existEmail, err := s.checkEmailsExistForClients(inboundSvc, clients, emailSubIDs)
	if err != nil {
		return false, err
	}
	if existEmail != "" {
		return false, common.NewError("Duplicate email:", existEmail)
	}

	oldInbound, err := inboundSvc.GetInbound(data.Id)
	if err != nil {
		return false, err
	}

	for _, client := range clients {
		if strings.TrimSpace(client.Email) == "" {
			return false, common.NewError("client email is required")
		}
		switch oldInbound.Protocol {
		case "trojan":
			if client.Password == "" {
				return false, common.NewError("empty client ID")
			}
		case "shadowsocks":
			if client.Email == "" {
				return false, common.NewError("empty client ID")
			}
		case "hysteria":
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

	if oldInbound.Protocol == model.Shadowsocks {
		applyShadowsocksClientMethod(interfaceClients, oldSettings)
	}

	oldClients := oldSettings["clients"].([]any)
	oldClients = compactOrphans(database.GetDB(), oldClients)
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
	rt, rterr := inboundSvc.runtimeFor(oldInbound)
	if rterr != nil {
		if oldInbound.NodeID != nil {
			err = rterr
			return false, err
		}
		needRestart = true
	} else if oldInbound.NodeID == nil {
		for _, client := range clients {
			if len(client.Email) == 0 {
				needRestart = true
				continue
			}
			inboundSvc.AddClientStat(tx, data.Id, &client)
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
		for _, client := range clients {
			if len(client.Email) > 0 {
				inboundSvc.AddClientStat(tx, data.Id, &client)
			}
			if err1 := rt.AddClient(context.Background(), oldInbound, client); err1 != nil {
				err = err1
				return false, err
			}
		}
	}

	if err = tx.Save(oldInbound).Error; err != nil {
		return false, err
	}
	finalClients, gcErr := inboundSvc.GetClients(oldInbound)
	if gcErr != nil {
		err = gcErr
		return false, err
	}
	if err = s.SyncInbound(tx, oldInbound.Id, finalClients); err != nil {
		return false, err
	}
	return needRestart, nil
}

func (s *ClientService) UpdateInboundClient(inboundSvc *InboundService, data *model.Inbound, clientId string) (bool, error) {
	defer lockInbound(data.Id).Unlock()

	clients, err := inboundSvc.GetClients(data)
	if err != nil {
		return false, err
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(data.Settings), &settings)
	if err != nil {
		return false, err
	}

	interfaceClients := settings["clients"].([]any)

	oldInbound, err := inboundSvc.GetInbound(data.Id)
	if err != nil {
		return false, err
	}

	oldClients, err := inboundSvc.GetClients(oldInbound)
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
		case "hysteria":
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

	if clientIndex == -1 {
		var rec model.ClientRecord
		var lookupErr error
		switch oldInbound.Protocol {
		case "trojan":
			lookupErr = database.GetDB().Where("password = ?", clientId).First(&rec).Error
		case "shadowsocks":
			lookupErr = database.GetDB().Where("email = ?", clientId).First(&rec).Error
		case "hysteria":
			lookupErr = database.GetDB().Where("auth = ?", clientId).First(&rec).Error
		default:
			lookupErr = database.GetDB().Where("uuid = ?", clientId).First(&rec).Error
		}
		if lookupErr == nil && rec.Email != "" {
			for index, oldClient := range oldClients {
				if oldClient.Email == rec.Email {
					oldEmail = oldClient.Email
					clientIndex = index
					break
				}
			}
		}
	}

	if newClientId == "" || clientIndex == -1 {
		return false, common.NewError("empty client ID")
	}
	if strings.TrimSpace(clients[0].Email) == "" {
		return false, common.NewError("client email is required")
	}

	if clients[0].Email != oldEmail {
		existEmail, err := s.checkEmailsExistForClients(inboundSvc, clients, nil)
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
	var preservedCreated any
	var preservedSubID string
	if clientIndex >= 0 && clientIndex < len(settingsClients) {
		if oldMap, ok := settingsClients[clientIndex].(map[string]any); ok {
			if v, ok2 := oldMap["created_at"]; ok2 {
				preservedCreated = v
			}
			preservedSubID, _ = oldMap["subId"].(string)
		}
	}
	if len(interfaceClients) > 0 {
		if newMap, ok := interfaceClients[0].(map[string]any); ok {
			if preservedCreated == nil {
				preservedCreated = time.Now().Unix() * 1000
			}
			newMap["created_at"] = preservedCreated
			newMap["updated_at"] = time.Now().Unix() * 1000
			newSub, _ := newMap["subId"].(string)
			if strings.TrimSpace(newSub) == "" {
				if strings.TrimSpace(preservedSubID) != "" {
					newMap["subId"] = preservedSubID
				} else {
					newMap["subId"] = random.NumLower(16)
				}
			}
			interfaceClients[0] = newMap
		}
	}
	if oldInbound.Protocol == model.Shadowsocks {
		applyShadowsocksClientMethod(interfaceClients, oldSettings)
	}
	settingsClients[clientIndex] = interfaceClients[0]
	oldSettings["clients"] = settingsClients

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
			emailUnchanged := strings.EqualFold(oldEmail, clients[0].Email)
			targetExists := int64(0)
			if !emailUnchanged {
				if err = tx.Model(xray.ClientTraffic{}).Where("email = ?", clients[0].Email).Count(&targetExists).Error; err != nil {
					return false, err
				}
			}
			if emailUnchanged || targetExists == 0 {
				err = inboundSvc.UpdateClientStat(tx, oldEmail, &clients[0])
				if err != nil {
					return false, err
				}
				err = inboundSvc.UpdateClientIPs(tx, oldEmail, clients[0].Email)
				if err != nil {
					return false, err
				}
			} else {
				stillUsed, sErr := inboundSvc.emailUsedByOtherInbounds(oldEmail, data.Id)
				if sErr != nil {
					return false, sErr
				}
				if !stillUsed {
					if err = inboundSvc.DelClientStat(tx, oldEmail); err != nil {
						return false, err
					}
					if err = inboundSvc.DelClientIPs(tx, oldEmail); err != nil {
						return false, err
					}
				}
				if err = inboundSvc.UpdateClientStat(tx, clients[0].Email, &clients[0]); err != nil {
					return false, err
				}
			}
		} else {
			inboundSvc.AddClientStat(tx, data.Id, &clients[0])
		}
	} else {
		stillUsed, err := inboundSvc.emailUsedByOtherInbounds(oldEmail, data.Id)
		if err != nil {
			return false, err
		}
		if !stillUsed {
			err = inboundSvc.DelClientStat(tx, oldEmail)
			if err != nil {
				return false, err
			}
			err = inboundSvc.DelClientIPs(tx, oldEmail)
			if err != nil {
				return false, err
			}
		}
	}
	needRestart := false
	if len(oldEmail) > 0 {
		rt, rterr := inboundSvc.runtimeFor(oldInbound)
		if rterr != nil {
			if oldInbound.NodeID != nil {
				err = rterr
				return false, err
			}
			needRestart = true
		} else if oldInbound.NodeID == nil {
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
			if err1 := rt.UpdateUser(context.Background(), oldInbound, oldEmail, clients[0]); err1 != nil {
				err = err1
				return false, err
			}
		}
	} else {
		logger.Debug("Client old email not found")
		needRestart = true
	}
	if err = tx.Save(oldInbound).Error; err != nil {
		return false, err
	}
	finalClients, gcErr := inboundSvc.GetClients(oldInbound)
	if gcErr != nil {
		err = gcErr
		return false, err
	}
	if err = s.SyncInbound(tx, oldInbound.Id, finalClients); err != nil {
		return false, err
	}
	return needRestart, nil
}

func (s *ClientService) DelInboundClient(inboundSvc *InboundService, inboundId int, clientId string, keepTraffic bool) (bool, error) {
	defer lockInbound(inboundId).Unlock()

	oldInbound, err := inboundSvc.GetInbound(inboundId)
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
	case "hysteria":
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

	db := database.GetDB()
	newClients = compactOrphans(db, newClients)
	if newClients == nil {
		newClients = []any{}
	}
	settings["clients"] = newClients
	newSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}

	oldInbound.Settings = string(newSettings)

	emailShared, err := inboundSvc.emailUsedByOtherInbounds(email, inboundId)
	if err != nil {
		return false, err
	}

	if !emailShared && !keepTraffic {
		err = inboundSvc.DelClientIPs(db, email)
		if err != nil {
			logger.Error("Error in delete client IPs")
			return false, err
		}
	}
	needRestart := false

	if len(email) > 0 {
		var enables []bool
		err = db.Model(xray.ClientTraffic{}).Where("email = ?", email).Limit(1).Pluck("enable", &enables).Error
		if err != nil {
			logger.Error("Get stats error")
			return false, err
		}
		notDepleted := len(enables) > 0 && enables[0]
		if !emailShared && !keepTraffic {
			err = inboundSvc.DelClientStat(db, email)
			if err != nil {
				logger.Error("Delete stats Data Error")
				return false, err
			}
		}
		if needApiDel && notDepleted && oldInbound.NodeID == nil {
			rt, rterr := inboundSvc.runtimeFor(oldInbound)
			if rterr != nil {
				needRestart = true
			} else {
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
			}
		}
	}
	if oldInbound.NodeID != nil && len(email) > 0 {
		rt, rterr := inboundSvc.runtimeFor(oldInbound)
		if rterr != nil {
			return false, rterr
		}
		if err1 := rt.DeleteUser(context.Background(), oldInbound, email); err1 != nil {
			return false, err1
		}
	}
	if err := db.Save(oldInbound).Error; err != nil {
		return false, err
	}
	finalClients, gcErr := inboundSvc.GetClients(oldInbound)
	if gcErr != nil {
		return false, gcErr
	}
	if err := s.SyncInbound(db, inboundId, finalClients); err != nil {
		return false, err
	}
	return needRestart, nil
}

func (s *ClientService) DelInboundClientByEmail(inboundSvc *InboundService, inboundId int, email string, keepTraffic bool) (bool, error) {
	defer lockInbound(inboundId).Unlock()

	oldInbound, err := inboundSvc.GetInbound(inboundId)
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
			found = true
			needApiDel, _ = c["enable"].(bool)
		} else {
			newClients = append(newClients, client)
		}
	}

	if !found {
		return false, common.NewError(fmt.Sprintf("client with email %s not found", email))
	}
	db := database.GetDB()
	newClients = compactOrphans(db, newClients)
	if newClients == nil {
		newClients = []any{}
	}
	settings["clients"] = newClients
	newSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}

	oldInbound.Settings = string(newSettings)

	emailShared, err := inboundSvc.emailUsedByOtherInbounds(email, inboundId)
	if err != nil {
		return false, err
	}

	if !emailShared && !keepTraffic {
		if err := inboundSvc.DelClientIPs(db, email); err != nil {
			logger.Error("Error in delete client IPs")
			return false, err
		}
	}

	needRestart := false

	if len(email) > 0 && !emailShared {
		if !keepTraffic {
			traffic, err := inboundSvc.GetClientTrafficByEmail(email)
			if err != nil {
				return false, err
			}
			if traffic != nil {
				if err := inboundSvc.DelClientStat(db, email); err != nil {
					logger.Error("Delete stats Data Error")
					return false, err
				}
			}
		}

		if needApiDel {
			rt, rterr := inboundSvc.runtimeFor(oldInbound)
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
				if err1 := rt.DeleteUser(context.Background(), oldInbound, email); err1 != nil {
					return false, err1
				}
			}
		}
	}

	if err := db.Save(oldInbound).Error; err != nil {
		return false, err
	}
	finalClients, gcErr := inboundSvc.GetClients(oldInbound)
	if gcErr != nil {
		return false, gcErr
	}
	if err := s.SyncInbound(db, inboundId, finalClients); err != nil {
		return false, err
	}
	return needRestart, nil
}

func (s *ClientService) SetClientTelegramUserID(inboundSvc *InboundService, trafficId int, tgId int64) (bool, error) {
	traffic, inbound, err := inboundSvc.GetClientInboundByTrafficID(trafficId)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Traffic ID:", trafficId)
	}

	clientEmail := traffic.Email

	oldClients, err := inboundSvc.GetClients(inbound)
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
	needRestart, err := s.UpdateInboundClient(inboundSvc, inbound, clientId)
	return needRestart, err
}

func (s *ClientService) checkIsEnabledByEmail(inboundSvc *InboundService, clientEmail string) (bool, error) {
	_, inbound, err := inboundSvc.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	clients, err := inboundSvc.GetClients(inbound)
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

func (s *ClientService) ToggleClientEnableByEmail(inboundSvc *InboundService, clientEmail string) (bool, bool, error) {
	_, inbound, err := inboundSvc.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, false, err
	}
	if inbound == nil {
		return false, false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := inboundSvc.GetClients(inbound)
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

	needRestart, err := s.UpdateInboundClient(inboundSvc, inbound, clientId)
	if err != nil {
		return false, needRestart, err
	}

	return !clientOldEnabled, needRestart, nil
}

func (s *ClientService) SetClientEnableByEmail(inboundSvc *InboundService, clientEmail string, enable bool) (bool, bool, error) {
	current, err := s.checkIsEnabledByEmail(inboundSvc, clientEmail)
	if err != nil {
		return false, false, err
	}
	if current == enable {
		return false, false, nil
	}
	newEnabled, needRestart, err := s.ToggleClientEnableByEmail(inboundSvc, clientEmail)
	if err != nil {
		return false, needRestart, err
	}
	return newEnabled == enable, needRestart, nil
}

func (s *ClientService) ResetClientIpLimitByEmail(inboundSvc *InboundService, clientEmail string, count int) (bool, error) {
	_, inbound, err := inboundSvc.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := inboundSvc.GetClients(inbound)
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
	needRestart, err := s.UpdateInboundClient(inboundSvc, inbound, clientId)
	return needRestart, err
}

func (s *ClientService) ResetClientExpiryTimeByEmail(inboundSvc *InboundService, clientEmail string, expiry_time int64) (bool, error) {
	_, inbound, err := inboundSvc.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := inboundSvc.GetClients(inbound)
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
	needRestart, err := s.UpdateInboundClient(inboundSvc, inbound, clientId)
	return needRestart, err
}

func (s *ClientService) ResetClientTrafficLimitByEmail(inboundSvc *InboundService, clientEmail string, totalGB int) (bool, error) {
	if totalGB < 0 {
		return false, common.NewError("totalGB must be >= 0")
	}
	_, inbound, err := inboundSvc.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := inboundSvc.GetClients(inbound)
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
	needRestart, err := s.UpdateInboundClient(inboundSvc, inbound, clientId)
	return needRestart, err
}
