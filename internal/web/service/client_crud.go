package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

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
		// Reuse stored credentials when re-adding an existing identity, or
		// fillProtocolDefaults mints a fresh UUID that desyncs other inbounds.
		if client.ID == "" {
			client.ID = existing.UUID
		}
		if client.Password == "" {
			client.Password = existing.Password
		}
		if client.Auth == "" {
			client.Auth = existing.Auth
		}
		if client.Secret == "" {
			client.Secret = existing.Secret
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
	case model.MTProto:
		if c.Secret == "" {
			c.Secret = model.GenerateFakeTLSSecret(mtprotoDomainFromSettings(ib.Settings))
		}
	}
	return nil
}

// defaultMtprotoDomain is the FakeTLS fronting domain used when an mtproto
// inbound carries no fakeTlsDomain of its own; it mirrors the frontend default.
const defaultMtprotoDomain = "www.cloudflare.com"

// mtprotoDomainFromSettings returns the inbound-level FakeTLS domain, falling
// back to the default when unset, so a generated client secret always fronts a
// real hostname.
func mtprotoDomainFromSettings(settings string) string {
	domain := ""
	if settings != "" {
		var m map[string]any
		if err := json.Unmarshal([]byte(settings), &m); err == nil {
			domain, _ = m["fakeTlsDomain"].(string)
		}
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return defaultMtprotoDomain
	}
	return domain
}

func clientWithInboundFlow(c model.Client, ib *model.Inbound) model.Client {
	if !inboundCanEnableTlsFlow(string(ib.Protocol), ib.StreamSettings, ib.Settings) {
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

// normalizeShadowsocksClientKeys rewrites any Shadowsocks-2022 client password
// whose decoded length no longer matches settings.method, which happens after the
// inbound method is switched between ciphers of different key sizes (e.g.
// aes-256↔aes-128). A wrong-length uPSK makes xray reject the user, so the link
// fails to connect; regenerating restores a valid key (clients must re-fetch).
// Non-Shadowsocks / legacy-SS settings pass through unchanged.
func normalizeShadowsocksClientKeys(settings string) (string, bool) {
	method := shadowsocksMethodFromSettings(settings)
	if shadowsocksKeyBytes(method) == 0 {
		return settings, false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(settings), &m); err != nil {
		return settings, false
	}
	clients, ok := m["clients"].([]any)
	if !ok {
		return settings, false
	}
	changed := false
	for i := range clients {
		c, ok := clients[i].(map[string]any)
		if !ok {
			continue
		}
		if pw, _ := c["password"].(string); validShadowsocksClientKey(method, pw) {
			continue
		}
		c["password"] = randomShadowsocksClientKey(method)
		clients[i] = c
		changed = true
	}
	if !changed {
		return settings, false
	}
	m["clients"] = clients
	bs, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return settings, false
	}
	return string(bs), true
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

func (s *ClientService) Update(inboundSvc *InboundService, id int, updated model.Client, inboundFilter ...int) (bool, error) {
	existing, err := s.GetByID(id)
	if err != nil {
		return false, err
	}
	inboundIds, err := s.GetInboundIdsForRecord(id)
	if err != nil {
		return false, err
	}
	if len(inboundFilter) > 0 {
		allow := make(map[int]struct{}, len(inboundFilter))
		for _, fid := range inboundFilter {
			allow[fid] = struct{}{}
		}
		filtered := inboundIds[:0:0]
		for _, ibId := range inboundIds {
			if _, ok := allow[ibId]; ok {
				filtered = append(filtered, ibId)
			}
		}
		inboundIds = filtered
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

	// Preserve existing credentials when the caller omits them, so a partial
	// update (e.g. only changing traffic/expiry) doesn't silently rotate the
	// client's UUID/password/auth via fillProtocolDefaults. Supplying a new
	// value still rotates it intentionally.
	if updated.ID == "" {
		updated.ID = existing.UUID
	}
	if updated.Password == "" {
		updated.Password = existing.Password
	}
	if updated.Auth == "" {
		updated.Auth = existing.Auth
	}
	if updated.Secret == "" {
		updated.Secret = existing.Secret
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
		if existing.Email == "" {
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
		}, existing.Email)
		if upErr != nil {
			return needRestart, upErr
		}
		if nr {
			needRestart = true
		}
	}

	// UpdateInboundClient renames the record atomically with each inbound's
	// settings JSON; this direct write only covers records with no inbound left.
	if updated.Email != existing.Email {
		if err := database.GetDB().Model(&model.ClientRecord{}).
			Where("id = ? AND email = ?", id, existing.Email).
			Update("email", updated.Email).Error; err != nil {
			return needRestart, err
		}
	}

	reverseStr := ""
	if updated.Reverse != nil && strings.TrimSpace(updated.Reverse.Tag) != "" {
		if b, mErr := json.Marshal(updated.Reverse); mErr == nil {
			reverseStr = string(b)
		}
	}
	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("id = ?", id).
		Update("reverse", reverseStr).Error; err != nil {
		return needRestart, err
	}

	// Persist the group explicitly. SyncInbound deliberately preserves the
	// stored group when the inbound settings carry none — so a node snapshot or a
	// group-less settings rebuild can't wipe it (see SyncInbound + its tests).
	// That guard also meant clearing the group in the client editor never took
	// effect. The editor always round-trips the field, so apply it here,
	// including the empty string that removes the client from its group.
	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("id = ?", id).
		UpdateColumn("group_name", updated.Group).Error; err != nil {
		return needRestart, err
	}

	// Same shape as the group write above: SyncInbound keeps a stored ad-tag
	// when the incoming settings carry none, so clearing the override must be
	// applied here, where the editor always round-trips the field.
	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("id = ?", id).
		UpdateColumn("ad_tag", updated.AdTag).Error; err != nil {
		return needRestart, err
	}

	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("id = ?", id).
		UpdateColumn("enable", updated.Enable).Error; err != nil {
		return needRestart, err
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
	var delErrs []error
	for _, ibId := range inboundIds {
		if _, getErr := inboundSvc.GetInbound(ibId); getErr != nil {
			if errors.Is(getErr, gorm.ErrRecordNotFound) {
				continue
			}
			delErrs = append(delErrs, fmt.Errorf("inbound %d: %w", ibId, getErr))
			continue
		}

		// Always delete by email — the client's stable identity. This removes
		// every matching entry from the inbound's settings even when the stored
		// credential (UUID/password/auth) drifted from the inbound JSON, or a
		// duplicate entry with the same email exists.
		if existing.Email == "" {
			continue
		}
		nr, delErr := s.DelInboundClientByEmail(inboundSvc, ibId, existing.Email, keepTraffic, true)
		if delErr != nil {
			// The client is already absent from this inbound (data drift or a
			// retried delete). Skip it — deletion stays idempotent.
			if errors.Is(delErr, ErrClientNotInInbound) {
				continue
			}
			delErrs = append(delErrs, fmt.Errorf("inbound %d: %w", ibId, delErr))
			continue
		}
		if nr {
			needRestart = true
		}
	}
	// A failed inbound still holds the client in its settings JSON: keep the
	// record so the next delete retries exactly the leftovers, and report it.
	if len(delErrs) > 0 {
		return needRestart, errors.Join(delErrs...)
	}

	db := database.GetDB()
	if err := db.Transaction(func(tx *gorm.DB) error {
		if existing.Email != "" {
			if err := adjustGroupBaselinesForRemovedTraffic(tx, []string{existing.Email}); err != nil {
				return err
			}
		}
		if err := tx.Where("client_id = ?", id).Delete(&model.ClientInbound{}).Error; err != nil {
			return err
		}
		if err := tx.Where("client_id = ?", id).Delete(&model.ClientExternalLink{}).Error; err != nil {
			return err
		}
		if !keepTraffic && existing.Email != "" {
			if err := tx.Where("email = ?", existing.Email).Delete(&xray.ClientTraffic{}).Error; err != nil {
				return err
			}
			if err := clearGlobalTraffic(tx, existing.Email); err != nil {
				return err
			}
			if err := tx.Where("client_email = ?", existing.Email).Delete(&model.InboundClientIps{}).Error; err != nil {
				return err
			}
			if err := tx.Where("email = ?", existing.Email).Delete(&model.NodeClientTraffic{}).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&model.ClientRecord{}, id).Error
	}); err != nil {
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
	flow, ffErr := s.EffectiveFlow(nil, id)
	if ffErr != nil {
		return false, ffErr
	}
	clientWire.Flow = flow
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
	var delErrs []error
	for _, ibId := range inboundIds {
		nr, delErr := s.DelInboundClientByEmail(inboundSvc, ibId, email, keepTraffic, true)
		if delErr != nil {
			if errors.Is(delErr, ErrClientNotInInbound) {
				continue
			}
			delErrs = append(delErrs, fmt.Errorf("inbound %d: %w", ibId, delErr))
			continue
		}
		if nr {
			needRestart = true
		}
	}
	if len(delErrs) > 0 {
		return needRestart, errors.Join(delErrs...)
	}
	if !keepTraffic {
		db := database.GetDB()
		if err := db.Where("email = ?", email).Delete(&xray.ClientTraffic{}).Error; err != nil {
			return needRestart, err
		}
		if err := clearGlobalTraffic(db, email); err != nil {
			return needRestart, err
		}
		if err := db.Where("client_email = ?", email).Delete(&model.InboundClientIps{}).Error; err != nil {
			return needRestart, err
		}
		if err := db.Where("email = ?", email).Delete(&model.NodeClientTraffic{}).Error; err != nil {
			return needRestart, err
		}
	}
	return needRestart, nil
}

func (s *ClientService) UpdateByEmail(inboundSvc *InboundService, email string, updated model.Client, inboundFilter ...int) (bool, error) {
	if email == "" {
		return false, common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return false, err
	}
	return s.Update(inboundSvc, rec.Id, updated, inboundFilter...)
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
		if _, getErr := inboundSvc.GetInbound(ibId); getErr != nil {
			return needRestart, getErr
		}
		// Detach by email — the client's stable identity (see Delete).
		if existing.Email == "" {
			continue
		}
		nr, delErr := s.DelInboundClientByEmail(inboundSvc, ibId, existing.Email, true, false)
		if delErr != nil {
			if errors.Is(delErr, ErrClientNotInInbound) {
				continue
			}
			return needRestart, delErr
		}
		if nr {
			needRestart = true
		}
	}
	return needRestart, nil
}
