package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

func hasForbiddenClientChar(s string) bool {
	for _, r := range s {
		if r == '/' || r == '\\' || r < 0x20 || r == 0x7f || unicode.IsSpace(r) {
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

// Rejected rather than coerced: an unknown cycle would leave the operator with
// a field that reads as configured while no job ever selects the client.
func validateClientTrafficReset(period string, day int) error {
	switch period {
	case "", "never", "hourly", "daily", "weekly", "monthly":
	default:
		return common.NewError("client trafficReset must be never, hourly, daily, weekly or monthly, got:", period)
	}
	if day < 0 || day > 31 {
		return common.NewError("client trafficResetDay must be between 0 and 31, got:", day)
	}
	return nil
}

// Rejected rather than clamped: nextCalendarRenewal would silently move an
// out-of-range day, and a negative one drops out of the renewal query entirely.
func validateClientResetDay(day int) error {
	if day < 0 || day > 31 {
		return common.NewError("client resetDay must be between 0 and 31, got:", day)
	}
	return nil
}

// Rejected rather than coerced: a negative cap reads as "unlimited" to a caller
// but selects nothing, so the client would silently stop renewing.
func validateClientResetMax(resetMax int) error {
	if resetMax < 0 {
		return common.NewError("client resetMax must not be negative, got:", resetMax)
	}
	return nil
}

// normalizeClientTrafficReset stores what the inbound path would store, so the
// day never reaches the DB as a 0 that three layers downstream each clamp to 1.
func normalizeClientTrafficReset(c *model.Client) {
	if c.TrafficReset == "" {
		c.TrafficReset = "never"
	}
	c.TrafficResetDay = normalizeTrafficResetDay(c.TrafficResetDay)
}

// ClientResetCycle is the slice of a client the reset job needs: enough to know
// whether it is due, and whether its disable is the quota's doing or the operator's.
type ClientResetCycle struct {
	Email           string
	TrafficResetDay int
	Enable          bool
	Total           int64
	Used            int64
}

// Depleted reports a client the quota switched off. A reset restores that one;
// a client disabled below its quota was switched off by hand and stays off.
func (c ClientResetCycle) Depleted() bool {
	return c.Total > 0 && c.Used >= c.Total
}

// GetClientsByTrafficReset returns the clients whose own reset cycle matches the
// period, independent of the cycle configured on the inbounds they belong to.
func (s *ClientService) GetClientsByTrafficReset(period string) ([]ClientResetCycle, error) {
	var cycles []ClientResetCycle
	err := database.GetDB().Table("clients c").
		Select("c.email, c.traffic_reset_day, c.enable, COALESCE(ct.total, 0) AS total, COALESCE(ct.up, 0) + COALESCE(ct.down, 0) AS used").
		Joins("LEFT JOIN client_traffics ct ON ct.email = c.email").
		Where("c.traffic_reset = ?", period).
		Scan(&cycles).Error
	if err != nil {
		return nil, err
	}
	return cycles, nil
}

// Create applies the client to every requested inbound: one failing inbound no
// longer aborts the others, so the error can name several and needRestart holds.
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
	if err := validateClientResetDay(client.ResetDay); err != nil {
		return false, err
	}
	if err := validateClientResetMax(client.ResetMax); err != nil {
		return false, err
	}
	if err := validateClientTrafficReset(client.TrafficReset, client.TrafficResetDay); err != nil {
		return false, err
	}
	normalizeClientTrafficReset(&client)
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

	// Prepared before any inbound is written: fillProtocolDefaults mints the
	// shared credentials on the first inbound and every later one reuses them.
	adds := make([]*model.Inbound, 0, len(payload.InboundIds))
	for _, ibId := range payload.InboundIds {
		inbound, getErr := inboundSvc.GetInbound(ibId)
		if getErr != nil {
			return false, fmt.Errorf("inbound %d: %w", ibId, getErr)
		}
		if err := s.fillProtocolDefaults(&client, inbound); err != nil {
			return false, fmt.Errorf("inbound %d: %w", ibId, err)
		}
		clientForInbound := client
		if ips, ok := client.AllowedIPsByInbound[ibId]; ok {
			clientForInbound.AllowedIPs = ips
		} else if !addressesFitAmneziaWGInbound(clientForInbound.AllowedIPs, inbound) {
			// The shared AllowedIPs value (e.g. from a single-field legacy
			// caller) came from a different subnet than this inbound's own --
			// clear it so defaultAmneziaWGClients allocates a fresh, correct
			// address for THIS inbound instead of persisting an unroutable
			// peer. Same reasoning as addressesFitAmneziaWGInbound's own doc
			// comment on the Attach path.
			clientForInbound.AllowedIPs = nil
		}
		settingsPayload, mErr := json.Marshal(map[string][]model.Client{"clients": {clientWithInboundFlow(clientForInbound, inbound)}})
		if mErr != nil {
			return false, fmt.Errorf("inbound %d: %w", ibId, mErr)
		}
		adds = append(adds, &model.Inbound{Id: ibId, Settings: string(settingsPayload)})
	}
	needRestart, fanoutErr := s.fanoutInboundClientAdds(inboundSvc, adds)
	if fanoutErr != nil {
		// Never on a failed create: this retrims the devices of an email that
		// already existed, and a create the panel reported as failed must not.
		return needRestart, fanoutErr
	}
	return needRestart, s.setClientLimitHwidByEmail(nil, client.Email, payload.LimitHwid)
}

// inboundFanoutConcurrency caps how many inbounds one create/attach applies at
// once, so a client spanning many of them can't start an unbounded RPC burst.
const inboundFanoutConcurrency = 4

// fanoutInboundClientAdds applies one payload per inbound with the node pushes
// overlapping; unlike the sequential loop, one failure no longer stops the rest.
func (s *ClientService) fanoutInboundClientAdds(inboundSvc *InboundService, adds []*model.Inbound) (bool, error) {
	var needRestart atomic.Bool
	errs := make([]error, len(adds))
	sem := make(chan struct{}, inboundFanoutConcurrency)
	var wg sync.WaitGroup
	for i := range adds {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			// Off the request goroutine gin's Recovery no longer covers this,
			// so an unrecovered panic here would take the whole panel down.
			defer func() {
				if r := recover(); r != nil {
					// The apply may already have committed, so ask for the
					// restart the lost return value can no longer report.
					needRestart.Store(true)
					errs[i] = fmt.Errorf("inbound %d: panic: %v", adds[i].Id, r)
					logger.Errorf("panic adding client to inbound %d: %v\n%s", adds[i].Id, r, debug.Stack())
				}
			}()
			nr, err := s.AddInboundClient(inboundSvc, adds[i])
			if nr {
				needRestart.Store(true)
			}
			if err != nil {
				errs[i] = fmt.Errorf("inbound %d: %w", adds[i].Id, err)
			}
		}()
	}
	wg.Wait()

	return needRestart.Load(), errors.Join(errs...)
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
	if ib.DisableFlow || !inboundCanEnableTlsFlow(string(ib.Protocol), ib.StreamSettings, ib.Settings) {
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

func (s *ClientService) Update(inboundSvc *InboundService, id int, updated model.Client, limitHwid int, inboundFilter ...int) (bool, error) {
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
	if err := validateClientResetDay(updated.ResetDay); err != nil {
		return false, err
	}
	if err := validateClientResetMax(updated.ResetMax); err != nil {
		return false, err
	}
	if err := validateClientTrafficReset(updated.TrafficReset, updated.TrafficResetDay); err != nil {
		return false, err
	}
	normalizeClientTrafficReset(&updated)
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

	if updated.SubID != existing.SubID {
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
		clientForInbound := updated
		if ips, ok := updated.AllowedIPsByInbound[ibId]; ok {
			clientForInbound.AllowedIPs = ips
		} else if !addressesFitAmneziaWGInbound(clientForInbound.AllowedIPs, inbound) {
			// A single shared AllowedIPs field (the common case for a caller
			// that never sends AllowedIPsByInbound) must never overwrite an
			// inbound it doesn't belong to -- e.g. a client attached to both
			// wg and awg saving its wg-labeled address would otherwise get
			// that same address silently written into the awg peer config
			// too. Clearing it here makes UpdateInboundClient's own
			// empty-AllowedIPs carry-forward (see its WireGuard/AmneziaWG
			// branch) preserve THIS inbound's existing, correct value
			// instead.
			clientForInbound.AllowedIPs = nil
		}
		settingsPayload, mErr := json.Marshal(map[string][]model.Client{"clients": {clientWithInboundFlow(clientForInbound, inbound)}})
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

	if len(inboundIds) == 0 {
		merged := *existing
		applyClientRecordMerge(&merged, updated.ToRecord())
		if err := database.GetDB().Model(&model.ClientRecord{}).
			Where("id = ?", id).
			Updates(map[string]any{
				"sub_id":            merged.SubID,
				"uuid":              merged.UUID,
				"password":          merged.Password,
				"auth":              merged.Auth,
				"secret":            merged.Secret,
				"flow":              merged.Flow,
				"security":          merged.Security,
				"wg_private_key":    merged.PrivateKey,
				"wg_public_key":     merged.PublicKey,
				"wg_allowed_ips":    merged.AllowedIPs,
				"wg_pre_shared_key": merged.PreSharedKey,
				"wg_keep_alive":     merged.KeepAlive,
				"limit_ip":          merged.LimitIP,
				"total_gb":          merged.TotalGB,
				"expiry_time":       merged.ExpiryTime,
				"tg_id":             merged.TgID,
				"comment":           merged.Comment,
				"reset":             merged.Reset,
				"reset_day":         merged.ResetDay,
				"reset_max":         merged.ResetMax,
				"traffic_reset":     merged.TrafficReset,
				"traffic_reset_day": merged.TrafficResetDay,
			}).Error; err != nil {
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

	if err := s.setClientLimitHwidByEmail(nil, updated.Email, limitHwid); err != nil {
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
		withdrawClientTombstones(existing.Email)
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
	// The tombstone lifts with it, or the next node merge finishes the deletion.
	if len(delErrs) > 0 {
		withdrawClientTombstones(existing.Email)
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
		if err := clearClientHwidsBySubIDTx(tx, existing.SubID); err != nil {
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
		withdrawClientTombstones(existing.Email)
		return needRestart, err
	}
	return needRestart, nil
}

// hasTunnelAttachment reports whether any of inboundIds is a currently
// existing WireGuard or AmneziaWG inbound. Inbounds that fail to load are
// skipped rather than treated as an error -- Attach's own loop already
// surfaces a real error for any inbound it can't load when it gets there.
func (s *ClientService) hasTunnelAttachment(inboundSvc *InboundService, inboundIds []int) bool {
	for _, ibId := range inboundIds {
		inbound, err := inboundSvc.GetInbound(ibId)
		if err != nil {
			continue
		}
		if inbound.Protocol == model.WireGuard || inbound.Protocol == model.AmneziaWG {
			return true
		}
	}
	return false
}

// addressesFitAmneziaWGInbound reports whether every entry in addrs falls
// inside ib's own configured subnet(s). AmneziaWG only: its kernel interface
// Address is exactly that subnet, so an address inherited from elsewhere (an
// identity attached to a WireGuard inbound first, say) produces a peer that
// can never connect -- Attach allocates fresh instead.
func addressesFitAmneziaWGInbound(addrs []string, ib *model.Inbound) bool {
	if ib.Protocol != model.AmneziaWG || len(addrs) == 0 {
		return true
	}
	v4Base, v6Base, err := defaultAmneziaWGSubnetBases(ib.Settings)
	if err != nil {
		return false
	}
	bases := make([]netip.Prefix, 0, 2)
	for _, base := range []string{v4Base, v6Base} {
		if base == "" {
			continue
		}
		prefix, pErr := netip.ParsePrefix(base)
		if pErr != nil {
			return false
		}
		bases = append(bases, prefix)
	}
	for _, a := range addrs {
		host := wireguardHostAddr(a)
		if !host.IsValid() {
			return false
		}
		fits := false
		for _, prefix := range bases {
			if prefix.Contains(host) {
				fits = true
				break
			}
		}
		if !fits {
			return false
		}
	}
	return true
}

// Attach applies the client to every requested inbound: one failing inbound no
// longer aborts the others, so the error can name several and needRestart holds.
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

	// If this identity has no CURRENT WireGuard/AmneziaWG attachment,
	// clientWire.AllowedIPs (from the ClientRecord) is a leftover from
	// whenever it last had one -- nothing reserves it anymore. Clear it so
	// attaching to a tunnel inbound now allocates a fresh address instead
	// of resurrecting the old one, which may no longer even be the lowest
	// free slot. Left untouched when the identity already has an active
	// tunnel elsewhere, so extending it to a second protocol still keeps
	// the same address on both.
	if !s.hasTunnelAttachment(inboundSvc, currentIds) {
		clientWire.AllowedIPs = nil
	}

	adds := make([]*model.Inbound, 0, len(inboundIds))
	for _, ibId := range inboundIds {
		if _, attached := have[ibId]; attached {
			continue
		}
		inbound, getErr := inboundSvc.GetInbound(ibId)
		if getErr != nil {
			return false, fmt.Errorf("inbound %d: %w", ibId, getErr)
		}
		copyClient := *clientWire
		if !addressesFitAmneziaWGInbound(copyClient.AllowedIPs, inbound) {
			copyClient.AllowedIPs = nil
		}
		if err := s.fillProtocolDefaults(&copyClient, inbound); err != nil {
			return false, fmt.Errorf("inbound %d: %w", ibId, err)
		}
		settingsPayload, mErr := json.Marshal(map[string][]model.Client{"clients": {clientWithInboundFlow(copyClient, inbound)}})
		if mErr != nil {
			return false, fmt.Errorf("inbound %d: %w", ibId, mErr)
		}
		adds = append(adds, &model.Inbound{Id: ibId, Settings: string(settingsPayload)})
	}
	return s.fanoutInboundClientAdds(inboundSvc, adds)
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

func (s *ClientService) UpdateByEmail(inboundSvc *InboundService, email string, updated model.Client, limitHwid int, inboundFilter ...int) (bool, error) {
	if email == "" {
		return false, common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return false, err
	}
	return s.Update(inboundSvc, rec.Id, updated, limitHwid, inboundFilter...)
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
