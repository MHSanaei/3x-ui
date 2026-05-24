package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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
	case model.Hysteria, model.Hysteria2:
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
			row.UUID = incoming.UUID
			row.Password = incoming.Password
			row.Auth = incoming.Auth
			row.Flow = incoming.Flow
			row.Security = incoming.Security
			row.Reverse = incoming.Reverse
			row.SubID = incoming.SubID
			row.LimitIP = incoming.LimitIP
			row.TotalGB = incoming.TotalGB
			row.ExpiryTime = incoming.ExpiryTime
			row.Enable = incoming.Enable
			row.TgID = incoming.TgID
			row.Comment = incoming.Comment
			row.Reset = incoming.Reset
			if incoming.CreatedAt > 0 && (row.CreatedAt == 0 || incoming.CreatedAt < row.CreatedAt) {
				row.CreatedAt = incoming.CreatedAt
			}
			if incoming.UpdatedAt > row.UpdatedAt {
				row.UpdatedAt = incoming.UpdatedAt
			}
			if err := tx.Save(row).Error; err != nil {
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
		if rows[i].FlowOverride != "" {
			c.Flow = rows[i].FlowOverride
		}
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

func (s *ClientService) Create(inboundSvc *InboundService, payload *ClientCreatePayload) (bool, error) {
	if payload == nil {
		return false, common.NewError("empty payload")
	}
	client := payload.Client
	if strings.TrimSpace(client.Email) == "" {
		return false, common.NewError("client email is required")
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

	needRestart := false
	for _, ibId := range payload.InboundIds {
		inbound, getErr := inboundSvc.GetInbound(ibId)
		if getErr != nil {
			return needRestart, getErr
		}
		if err := s.fillProtocolDefaults(&client, inbound); err != nil {
			return needRestart, err
		}
		settingsPayload, mErr := json.Marshal(map[string][]model.Client{"clients": {client}})
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
	case model.Hysteria, model.Hysteria2:
		if c.Auth == "" {
			c.Auth = strings.ReplaceAll(uuid.NewString(), "-", "")
		}
	}
	return nil
}

// shadowsocksMethodFromSettings pulls the "method" field out of the inbound's
// settings JSON. Returns "" when the field is missing or settings is invalid.
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

// randomShadowsocksClientKey returns a per-client key sized to the cipher.
// The 2022-blake3 ciphers require a base64-encoded key of an exact byte
// length (16 bytes for aes-128-gcm, 32 bytes for aes-256-gcm and
// chacha20-poly1305) — anything else fails with "bad key" on xray start.
// Older ciphers accept arbitrary passwords, so we keep the uuid-style.
func randomShadowsocksClientKey(method string) string {
	if n := shadowsocksKeyBytes(method); n > 0 {
		return random.Base64Bytes(n)
	}
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

// validShadowsocksClientKey reports whether key is acceptable for the cipher.
// For 2022-blake3 it must decode to the exact byte length the cipher needs;
// any other method accepts any non-empty string.
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

// applyShadowsocksClientMethod ensures each client entry carries a "method"
// field for legacy shadowsocks ciphers. xray's multi-user shadowsocks code
// requires a per-client method; an empty/missing field fails with
// "unsupported cipher method:". 2022-blake3 ciphers use the top-level
// method only, so the per-client field must stay absent.
func applyShadowsocksClientMethod(clients []any, settings map[string]any) {
	method, _ := settings["method"].(string)
	if method == "" || strings.HasPrefix(method, "2022-blake3-") {
		return
	}
	for i := range clients {
		cm, ok := clients[i].(map[string]any)
		if !ok {
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

	needRestart := false
	for _, ibId := range inboundIds {
		inbound, getErr := inboundSvc.GetInbound(ibId)
		if getErr != nil {
			return needRestart, getErr
		}
		oldKey := clientKeyForProtocol(inbound.Protocol, existing)
		if oldKey == "" {
			continue
		}
		if err := s.fillProtocolDefaults(&updated, inbound); err != nil {
			return needRestart, err
		}
		settingsPayload, mErr := json.Marshal(map[string][]model.Client{"clients": {updated}})
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
		Update("updated_at", updated.UpdatedAt).Error; err != nil {
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
			return needRestart, getErr
		}
		key := clientKeyForProtocol(inbound.Protocol, existing)
		if key == "" {
			continue
		}
		nr, delErr := s.DelInboundClient(inboundSvc, ibId, key)
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
		settingsPayload, mErr := json.Marshal(map[string][]model.Client{"clients": {copyClient}})
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
	if err != nil {
		return false, err
	}
	return s.Delete(inboundSvc, rec.Id, keepTraffic)
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
	Comment    string              `json:"comment,omitempty"`
	InboundIds []int               `json:"inboundIds"`
	Traffic    *xray.ClientTraffic `json:"traffic,omitempty"`
	CreatedAt  int64               `json:"createdAt"`
	UpdatedAt  int64               `json:"updatedAt"`
}

// ClientPageParams are the query params accepted by /panel/api/clients/list/paged.
// All fields are optional — the empty value means "no filter" / defaults.
type ClientPageParams struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Search   string `form:"search"`
	Filter   string `form:"filter"`
	Protocol string `form:"protocol"`
	Inbound  int    `form:"inbound"`
	Sort     string `form:"sort"`
	Order    string `form:"order"`
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

	var protocolByInbound map[int]string
	if params.Protocol != "" {
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
		if params.Protocol != "" && !clientMatchesProtocol(c, params.Protocol, protocolByInbound) {
			continue
		}
		if params.Inbound > 0 && !clientMatchesInbound(c, params.Inbound) {
			continue
		}
		if params.Filter != "" && !clientMatchesBucket(c, params.Filter, onlineSet, nowMs, expireDiffMs, trafficDiffBytes) {
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

	return &ClientPageResponse{
		Items:    items,
		Total:    total,
		Filtered: filteredCount,
		Page:     page,
		PageSize: pageSize,
		Summary:  summary,
	}, nil
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
	if strings.Contains(strings.ToLower(c.Email), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(c.SubID), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(c.Comment), needle) {
		return true
	}
	return false
}

func clientMatchesProtocol(c ClientWithAttachments, protocol string, byInbound map[int]string) bool {
	if protocol == "" {
		return true
	}
	for _, id := range c.InboundIds {
		if byInbound[id] == protocol {
			return true
		}
	}
	return false
}

func clientMatchesInbound(c ClientWithAttachments, inboundId int) bool {
	if inboundId <= 0 {
		return true
	}
	for _, id := range c.InboundIds {
		if id == inboundId {
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

// BulkAdjust shifts ExpiryTime by addDays (days) and TotalGB by addBytes
// for every email in the list. Clients whose corresponding field is
// unlimited (0) are skipped — bulk extend should not accidentally
// limit an unlimited client. addDays and addBytes may be negative.
func (s *ClientService) BulkAdjust(inboundSvc *InboundService, emails []string, addDays int, addBytes int64) (BulkAdjustResult, bool, error) {
	result := BulkAdjustResult{}
	needRestart := false
	if len(emails) == 0 {
		return result, needRestart, nil
	}
	if addDays == 0 && addBytes == 0 {
		return result, needRestart, common.NewError("no adjustment specified")
	}

	addExpiryMs := int64(addDays) * 24 * 60 * 60 * 1000

	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		rec, err := s.GetRecordByEmail(nil, email)
		if err != nil {
			result.Skipped = append(result.Skipped, BulkAdjustReport{Email: email, Reason: err.Error()})
			continue
		}
		client := rec.ToClient()

		applied := false
		if addDays != 0 {
			switch {
			case rec.ExpiryTime == 0:
				result.Skipped = append(result.Skipped, BulkAdjustReport{Email: email, Reason: "unlimited expiry"})
			case rec.ExpiryTime > 0:
				next := rec.ExpiryTime + addExpiryMs
				if next <= 0 {
					result.Skipped = append(result.Skipped, BulkAdjustReport{Email: email, Reason: "reduction exceeds remaining time"})
				} else {
					client.ExpiryTime = next
					applied = true
				}
			default:
				next := rec.ExpiryTime - addExpiryMs
				if next >= 0 {
					result.Skipped = append(result.Skipped, BulkAdjustReport{Email: email, Reason: "reduction exceeds delay window"})
				} else {
					client.ExpiryTime = next
					applied = true
				}
			}
		}
		if addBytes != 0 {
			if rec.TotalGB == 0 {
				result.Skipped = append(result.Skipped, BulkAdjustReport{Email: email, Reason: "unlimited traffic"})
			} else {
				next := rec.TotalGB + addBytes
				if next < 0 {
					next = 0
				}
				client.TotalGB = next
				applied = true
			}
		}
		if !applied {
			continue
		}

		nr, err := s.Update(inboundSvc, rec.Id, *client)
		if err != nil {
			result.Skipped = append(result.Skipped, BulkAdjustReport{Email: email, Reason: err.Error()})
			continue
		}
		if nr {
			needRestart = true
		}
		result.Adjusted++
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
		nr, delErr := s.DelInboundClient(inboundSvc, ibId, key)
		if delErr != nil {
			return needRestart, delErr
		}
		if nr {
			needRestart = true
		}
	}
	return needRestart, nil
}

func (s *ClientService) checkEmailsExistForClients(inboundSvc *InboundService, clients []model.Client) (string, error) {
	emailSubIDs, err := inboundSvc.getAllEmailSubIDs()
	if err != nil {
		return "", err
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
			interfaceClients[i] = cm
		}
	}
	existEmail, err := s.checkEmailsExistForClients(inboundSvc, clients)
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

	if clientIndex == -1 {
		var rec model.ClientRecord
		var lookupErr error
		switch oldInbound.Protocol {
		case "trojan":
			lookupErr = database.GetDB().Where("password = ?", clientId).First(&rec).Error
		case "shadowsocks":
			lookupErr = database.GetDB().Where("email = ?", clientId).First(&rec).Error
		case "hysteria", "hysteria2":
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
		existEmail, err := s.checkEmailsExistForClients(inboundSvc, clients)
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

func (s *ClientService) DelInboundClient(inboundSvc *InboundService, inboundId int, clientId string) (bool, error) {
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

	if !emailShared {
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
		if !emailShared {
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

func (s *ClientService) DelInboundClientByEmail(inboundSvc *InboundService, inboundId int, email string) (bool, error) {
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

	if !emailShared {
		if err := inboundSvc.DelClientIPs(db, email); err != nil {
			logger.Error("Error in delete client IPs")
			return false, err
		}
	}

	needRestart := false

	if len(email) > 0 && !emailShared {
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
