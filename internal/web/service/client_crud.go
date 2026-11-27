package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
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
	if email == "." || email == ".." {
		return common.NewError("client email must not be '.' or '..'")
	}
	if hasForbiddenClientChar(email) {
		return common.NewError("client email contains an invalid character:", email)
	}
	return nil
}

func validateClientsNames(clients []model.Client) error {
	for _, client := range clients {
		if err := validateClientEmail(client.Email); err != nil {
			return err
		}
		if err := validateClientSubID(client.SubID); err != nil {
			return err
		}
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

func validateClientRenewal(client model.Client) error {
	if err := validateClientResetDay(client.ResetDay); err != nil {
		return err
	}
	if client.Reset > maxClientResetDays {
		return common.NewError("client reset must not exceed", maxClientResetDays, "days")
	}
	if client.ResetWeekday < 0 || client.ResetWeekday > 7 {
		return common.NewError("client resetWeekday must be between 0 and 7, got:", client.ResetWeekday)
	}
	if client.ResetWeekday > 0 && (client.Reset > 0 || client.ResetDay > 0) {
		return common.NewError("client weekly renewal cannot be combined with reset or resetDay")
	}
	return nil
}

func validateClientsRenewal(clients []model.Client) error {
	for _, client := range clients {
		if err := validateClientRenewal(client); err != nil {
			return err
		}
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
	Up              int64
	Down            int64
}

// Depleted reports a client the quota switched off. A reset restores that one;
// a client disabled below its quota was switched off by hand and stays off.
func (c ClientResetCycle) Depleted() bool {
	return c.Total > 0 && clampedTrafficAdd(c.Up, c.Down) >= c.Total
}

func clientResetCycles(tx *gorm.DB) *gorm.DB {
	return tx.Table("clients c").
		Select("c.email, c.traffic_reset_day, c.enable, COALESCE(ct.total, 0) AS total, COALESCE(ct.up, 0) AS up, COALESCE(ct.down, 0) AS down").
		Joins("LEFT JOIN client_traffics ct ON ct.email = c.email")
}

// GetClientsByTrafficReset returns the clients whose own reset cycle matches the
// period, independent of the cycle configured on the inbounds they belong to.
func (s *ClientService) GetClientsByTrafficReset(period string) ([]ClientResetCycle, error) {
	var cycles []ClientResetCycle
	err := clientResetCycles(database.GetDB()).
		Where("c.traffic_reset = ?", period).
		Scan(&cycles).Error
	if err != nil {
		return nil, err
	}
	return cycles, nil
}

// quotaDisabledEmails lists the disabled clients whose usage reached their quota,
// only those in within unless it is nil. Read it before zeroing the counters,
// which hides why a client is off.
func quotaDisabledEmails(tx *gorm.DB, within []string) ([]string, error) {
	var cycles []ClientResetCycle
	if err := clientResetCycles(tx).Where("c.enable = ?", false).Scan(&cycles).Error; err != nil {
		return nil, err
	}
	// Cross-panel usage depletes too: a client cut off on pushed usage was the quota's doing.
	rows := make([]xray.ClientTraffic, len(cycles))
	for i, c := range cycles {
		rows[i] = xray.ClientTraffic{Email: c.Email, Up: c.Up, Down: c.Down, Total: c.Total}
	}
	overlayGlobalTrafficValues(tx, rows)
	for i := range cycles {
		cycles[i].Up, cycles[i].Down = rows[i].Up, rows[i].Down
	}
	var in map[string]bool
	if within != nil {
		in = make(map[string]bool, len(within))
		for _, e := range within {
			in[e] = true
		}
	}
	var emails []string
	for _, c := range cycles {
		if c.Depleted() && (in == nil || in[c.Email]) {
			emails = append(emails, c.Email)
		}
	}
	return emails, nil
}

var errFallbackSlowPath = errors.New("fallback to per-inbound path")

// Create applies the client to every requested inbound: one failing inbound no
// longer aborts the others, so the error can name several and needRestart holds.
func (s *ClientService) Create(inboundSvc *InboundService, payload *ClientCreatePayload) (bool, error) {
	if payload == nil {
		return false, common.NewError("empty payload")
	}
	// A repeated id adds nothing but another copy of the client held through the fan-out.
	payload.InboundIds = uniqueInts(payload.InboundIds)
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
	if err := validateClientRenewal(client); err != nil {
		return false, err
	}
	if client.TotalGB < 0 {
		return false, common.NewError("totalGB must be >= 0")
	}
	if err := validateClientResetMax(client.ResetMax); err != nil {
		return false, err
	}
	if err := validateClientTrafficReset(client.TrafficReset, client.TrafficResetDay); err != nil {
		return false, err
	}
	normalizeClientTrafficReset(&client)
	// Nothing to withdraw on a new client, so a blank spec is no spec.
	if client.ForwardedPortsSpec() == "" {
		client.ForwardedPorts = nil
	}
	client.ClientHostRuleIds = model.NormalizeClientHostRuleIds(client.ClientHostRuleIds)
	if len(payload.InboundIds) == 0 {
		return false, common.NewError("at least one inbound is required")
	}

	if client.SubID == "" {
		client.SubID = uuid.NewString()
	}
	// Enable: omit defaults true via ClientCreatePayload.UnmarshalJSON; explicit false kept.
	now := time.Now().UnixMilli()
	if client.CreatedAt == 0 {
		client.CreatedAt = now
	}
	client.UpdatedAt = now

	if needRestart, handled, err := s.createBatchedNonTunnel(inboundSvc, client, payload.InboundIds, payload.LimitHwid, now); handled {
		return needRestart, err
	}

	existing := &model.ClientRecord{}
	err := database.GetDB().Where("LOWER(email) = LOWER(?)", client.Email).First(existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	emailTaken := !errors.Is(err, gorm.ErrRecordNotFound)
	if emailTaken {
		if existing.SubID == "" || existing.SubID != client.SubID {
			return false, common.NewError("email already in use:", client.Email)
		}
		client.Email = existing.Email
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
			Where("sub_id = ? AND LOWER(email) <> LOWER(?)", client.SubID, client.Email).
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
			clientForInbound.AllowedIPs = nil
		}
		if err := validateAmneziaWGClientWrite(inbound, []model.Client{clientForInbound}, ""); err != nil {
			return false, fmt.Errorf("inbound %d: %w", ibId, err)
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
	// nil means omitted and preserves the stored binding.
	if client.ClientHostRuleIds != nil {
		if err := database.GetDB().Model(&model.ClientRecord{}).
			Where("email = ?", client.Email).
			Update("client_host_rule_ids", model.EncodeClientHostRuleIds(client.ClientHostRuleIds)).Error; err != nil {
			return needRestart, err
		}
	}
	if payload.LimitHwid > 0 || (emailTaken && existing.LimitHwid != payload.LimitHwid) {
		return needRestart, s.setClientLimitHwidByEmail(client.Email, payload.LimitHwid)
	}
	return needRestart, nil
}

type createWritePlan struct {
	inbound      *model.Inbound
	prevSettings string
	newSettings  string
	client       model.Client
	ssMethod     string
}

func (s *ClientService) createBatchedNonTunnel(inboundSvc *InboundService, client model.Client, uniqueIDs []int, limitHwid int, now int64) (bool, bool, error) {
	defer InvalidateSubData()
	unlock := lockInboundsSorted(uniqueIDs)
	defer unlock()

	var plans []createWritePlan
	var errs []error
	txErr := runSerializedTx(func(tx *gorm.DB) error {
		existing := &model.ClientRecord{}
		err := tx.Where("LOWER(email) = LOWER(?)", client.Email).First(existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		emailTaken := !errors.Is(err, gorm.ErrRecordNotFound)
		if emailTaken {
			if existing.SubID == "" || existing.SubID != client.SubID {
				return common.NewError("email already in use:", client.Email)
			}
			client.Email = existing.Email
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
			if err := tx.Model(&model.ClientRecord{}).
				Where("sub_id = ? AND LOWER(email) <> LOWER(?)", client.SubID, client.Email).
				Count(&subTaken).Error; err != nil {
				return err
			}
			if subTaken > 0 {
				return common.NewError("subId already in use:", client.SubID)
			}
		}

		inboundsByID, idxByID, err := loadInboundsForClientWrite(tx, uniqueIDs)
		if err != nil {
			return err
		}
		for _, ibId := range uniqueIDs {
			ib := inboundsByID[ibId]
			if ib == nil {
				return fmt.Errorf("inbound %d: %w", ibId, gorm.ErrRecordNotFound)
			}
			if ib.Protocol == model.WireGuard || ib.Protocol == model.AmneziaWG {
				return errFallbackSlowPath
			}
		}

		for _, ibId := range uniqueIDs {
			if err := s.fillProtocolDefaultsWithIdx(&client, inboundsByID[ibId], idxByID[ibId]); err != nil {
				return fmt.Errorf("inbound %d: %w", ibId, err)
			}
		}

		settingsUpdates := make([]inboundSettingsUpdate, 0, len(uniqueIDs))
		deltas := make([]inboundClientDelta, 0, len(uniqueIDs))
		for _, ibId := range uniqueIDs {
			ib := inboundsByID[ibId]
			idx := idxByID[ibId]
			if err := validateTuicClientEmails(ib.Protocol, []model.Client{client}); err != nil {
				return fmt.Errorf("inbound %d: %w", ibId, err)
			}
			finalClient := clientWithInboundFlowIdx(client, ib, idx)
			if err := validateClientRequiredCredentials(ib.Protocol, finalClient); err != nil {
				errs = append(errs, fmt.Errorf("inbound %d: %w", ibId, err))
				continue
			}

			if idx != nil && idx.canSplice() {
				if idx.hasFoldEmail(finalClient.Email) {
					continue
				}
				ssMethod := ""
				if ib.Protocol == model.Shadowsocks {
					ssMethod = idx.topMethod
				}
				encoded, sp, mErr := marshalClientWire(finalClient, ssMethod, now)
				if mErr != nil {
					errs = append(errs, fmt.Errorf("inbound %d: %w", ibId, mErr))
					continue
				}
				newSettings, newIdx := idx.spliceAppend([][]byte{encoded}, []clientSpan{sp})
				prevSettings := ib.Settings
				ibCopy := *ib
				ibCopy.Settings = newSettings
				plans = append(plans, createWritePlan{
					inbound:      &ibCopy,
					prevSettings: prevSettings,
					newSettings:  newSettings,
					client:       finalClient,
					ssMethod:     ssMethod,
				})
				settingsUpdates = append(settingsUpdates, inboundSettingsUpdate{
					inboundId:   ib.Id,
					newSettings: newSettings,
					newIdx:      newIdx,
				})
				deltas = append(deltas, inboundClientDelta{
					inboundId: ib.Id,
					protocol:  ib.Protocol,
					nodeID:    ib.NodeID,
					hasMeta:   true,
					changed:   []model.Client{finalClient},
				})
				continue
			}

			doc, dErr := parseRawSettingsDoc(ib.Settings)
			if dErr != nil {
				errs = append(errs, fmt.Errorf("inbound %d: %w", ibId, dErr))
				continue
			}
			dupOnInbound := false
			for i := range doc.rawClients {
				if em := doc.clientHeaderAt(i).Email; em != "" && strings.EqualFold(em, finalClient.Email) {
					dupOnInbound = true
					break
				}
			}
			if dupOnInbound {
				continue
			}
			ssMethod := ""
			if ib.Protocol == model.Shadowsocks {
				ssMethod = doc.topString("method")
			}
			encoded, _, mErr := marshalClientWire(finalClient, ssMethod, now)
			if mErr != nil {
				errs = append(errs, fmt.Errorf("inbound %d: %w", ibId, mErr))
				continue
			}
			doc.hasClients = true
			doc.rawClients = append(doc.rawClients, json.RawMessage(encoded))
			newSettings, eErr := doc.encodeCompact()
			if eErr != nil {
				errs = append(errs, fmt.Errorf("inbound %d: %w", ibId, eErr))
				continue
			}
			prevSettings := ib.Settings
			ibCopy := *ib
			ibCopy.Settings = newSettings
			plans = append(plans, createWritePlan{
				inbound:      &ibCopy,
				prevSettings: prevSettings,
				newSettings:  newSettings,
				client:       finalClient,
				ssMethod:     ssMethod,
			})
			settingsUpdates = append(settingsUpdates, inboundSettingsUpdate{
				inboundId:   ib.Id,
				newSettings: newSettings,
			})
			deltas = append(deltas, inboundClientDelta{
				inboundId: ib.Id,
				protocol:  ib.Protocol,
				nodeID:    ib.NodeID,
				hasMeta:   true,
				changed:   []model.Client{finalClient},
			})
		}

		if len(plans) > 0 {
			if err := inboundSvc.AddClientStat(tx, plans[0].inbound.Id, &plans[0].client); err != nil {
				return err
			}
			if err := commitInboundsSettingsBatch(tx, settingsUpdates); err != nil {
				return err
			}
			if err := s.ApplyMultiInboundClientDelta(tx, deltas); err != nil {
				return err
			}
			dirtyNodes := make(map[int]struct{})
			for _, wp := range plans {
				if wp.inbound.NodeID != nil {
					dirtyNodes[*wp.inbound.NodeID] = struct{}{}
				}
			}
			if len(dirtyNodes) > 0 {
				nodeSvc := &NodeService{}
				for nodeID := range dirtyNodes {
					if err := nodeSvc.MarkNodeDirtyTx(tx, nodeID); err != nil {
						return err
					}
				}
			}
		}

		if len(errs) == 0 {
			if client.ClientHostRuleIds != nil {
				if err := tx.Model(&model.ClientRecord{}).
					Where("email = ?", client.Email).
					Update("client_host_rule_ids", model.EncodeClientHostRuleIds(client.ClientHostRuleIds)).Error; err != nil {
					return err
				}
			}
			if limitHwid > 0 || (emailTaken && existing.LimitHwid != limitHwid) {
				if err := s.setClientLimitHwidByEmailTx(tx, client.Email, limitHwid); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if errors.Is(txErr, errFallbackSlowPath) {
		return false, false, nil
	}
	if txErr != nil {
		return false, true, txErr
	}
	unlock()

	needRestart := false
	if len(plans) > 0 {
		hasRemote := false
		planIDs := make([]int, len(plans))
		for i := range plans {
			planIDs[i] = plans[i].inbound.Id
			if plans[i].inbound.NodeID != nil {
				hasRemote = true
			}
		}
		outcomes, panics := fanoutInboundResults(planIDs, runtimeFanoutLimit(hasRemote), func(i int) inboundApplyOutcome {
			wp := plans[i]
			rt, push, _, perr := inboundSvc.nodePushPlan(wp.inbound)
			if perr != nil {
				return inboundApplyOutcome{err: fmt.Errorf("inbound %d: %w", wp.inbound.Id, perr)}
			}
			var out inboundApplyOutcome
			if wp.inbound.NodeID == nil {
				if !push {
					out.needRestart = true
				} else if wp.inbound.Protocol == model.MTProto {
					inboundSvc.applyLocalMtproto(wp.inbound.Id)
				} else if wp.inbound.Protocol == model.TUIC {
					inboundSvc.applyLocalTuic(wp.inbound.Id)
				} else if wp.client.Enable {
					op := runtime.UserAddOp{
						User:    coreUserFromClient(&wp.client, wp.ssMethod),
						Replace: false,
					}
					if localBatchAddUsers(rt, wp.inbound, []runtime.UserAddOp{op}) {
						out.needRestart = true
					}
				}
				return out
			}
			if push && wp.client.Enable {
				ctx, cancel := nodePushContext()
				err1 := rt.AddClient(ctx, wp.inbound, wp.client)
				cancel()
				if err1 != nil {
					logger.Warning("Error in adding client on", rt.Name(), ":", err1)
				} else {
					advancePushedInbound(rt, wp.prevSettings, wp.newSettings, wp.inbound)
				}
			}
			return out
		})
		for i, out := range outcomes {
			if panics[i] != nil {
				needRestart = true
				errs = append(errs, panics[i])
				continue
			}
			if out.needRestart {
				needRestart = true
			}
			if out.err != nil {
				errs = append(errs, out.err)
			}
		}
	}
	return needRestart, true, errors.Join(errs...)
}

// inboundFanoutConcurrency caps how many inbounds one client op applies at
// once, so a client spanning many of them can't start an unbounded RPC burst.
const inboundFanoutConcurrency = 4

// localFanoutConcurrency caps concurrent local-only HTTP/2 gRPC pushes across inbounds.
const localFanoutConcurrency = 16

func runtimeFanoutLimit(hasRemote bool) int {
	if hasRemote {
		return inboundFanoutConcurrency
	}
	return localFanoutConcurrency
}

// inboundApply is one inbound's share of a client op, ready to run.
type inboundApply struct {
	id  int
	run func() (bool, error)
}

// fanoutInboundApplies runs the applies with the node pushes overlapping, so a
// client spanning several nodes no longer costs one RPC round-trip per node.
func fanoutInboundApplies(applies []inboundApply) (bool, error) {
	var needRestart atomic.Bool
	errs := make([]error, len(applies))
	sem := make(chan struct{}, inboundFanoutConcurrency)
	var wg sync.WaitGroup
	for i := range applies {
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
					errs[i] = fmt.Errorf("inbound %d: panic: %v", applies[i].id, r)
					logger.Errorf("panic applying client change to inbound %d: %v\n%s", applies[i].id, r, debug.Stack())
				}
			}()
			nr, err := applies[i].run()
			if nr {
				needRestart.Store(true)
			}
			if err != nil {
				errs[i] = fmt.Errorf("inbound %d: %w", applies[i].id, err)
			}
		}()
	}
	wg.Wait()

	return needRestart.Load(), errors.Join(errs...)
}

// fanoutInboundClientAdds applies one payload per inbound.
func (s *ClientService) fanoutInboundClientAdds(inboundSvc *InboundService, adds []*model.Inbound) (bool, error) {
	applies := make([]inboundApply, 0, len(adds))
	for _, add := range adds {
		applies = append(applies, inboundApply{id: add.Id, run: func() (bool, error) {
			return s.AddInboundClient(inboundSvc, add)
		}})
	}
	return fanoutInboundApplies(applies)
}

// fanoutInboundResults runs one job per inbound with the node pushes
// overlapping, so a bulk op costs one RPC round-trip instead of one per node.
// limit is the caller's own cap: an op that allocates tunnel addresses passes 1,
// because allocation reads a cross-inbound used-set before it writes.
func fanoutInboundResults[T any](inboundIds []int, limit int, run func(i int) T) ([]T, []error) {
	if limit < 1 {
		limit = 1
	}
	out := make([]T, len(inboundIds))
	errs := make([]error, len(inboundIds))
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := range inboundIds {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			// Off the request goroutine gin's Recovery no longer covers this,
			// so an unrecovered panic here would take the whole panel down.
			defer func() {
				if r := recover(); r != nil {
					errs[i] = fmt.Errorf("inbound %d: panic: %v", inboundIds[i], r)
					logger.Errorf("panic applying bulk client change to inbound %d: %v\n%s", inboundIds[i], r, debug.Stack())
				}
			}()
			out[i] = run(i)
		}()
	}
	wg.Wait()
	return out, errs
}

// addFanoutLimit serializes an add that touches a tunnel inbound. WireGuard and
// AmneziaWG pick a free peer address by reading every inbound's used-set first,
// so two overlapping allocations hand out the same one and the second is refused.
func addFanoutLimit(anyTunnel bool) int {
	if anyTunnel {
		return 1
	}
	return inboundFanoutConcurrency
}

// sortedInboundIds gives the fanout a stable order, so which inbound wins a
// per-email report no longer depends on Go's map iteration order.
func sortedInboundIds[V any](byInbound map[int]V) []int {
	ids := make([]int, 0, len(byInbound))
	for id := range byInbound {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// markInboundNodesDirty makes a half-applied client edit unobservable to a node
// snapshot merge, which skips a node whose config is already flagged dirty.
func markInboundNodesDirty(inboundIds []int) error {
	if len(inboundIds) == 0 {
		return nil
	}
	var nodeIDs []int
	for _, batch := range chunkInts(inboundIds, sqlInChunk) {
		var ids []int
		if err := database.GetDB().Model(&model.Inbound{}).
			Where("id IN ? AND node_id IS NOT NULL", batch).
			Distinct().Pluck("node_id", &ids).Error; err != nil {
			return err
		}
		nodeIDs = append(nodeIDs, ids...)
	}
	if len(nodeIDs) == 0 {
		return nil
	}
	return runSerializedTx(func(tx *gorm.DB) error {
		svc := &NodeService{}
		for _, id := range nodeIDs {
			if err := svc.MarkNodeDirtyTx(tx, id); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *ClientService) fillProtocolDefaults(c *model.Client, ib *model.Inbound) error {
	return s.fillProtocolDefaultsAgainstExisting(c, ib, nil)
}

func (s *ClientService) fillProtocolDefaultsWithIdx(c *model.Client, ib *model.Inbound, idx *fastSettingsIndex) error {
	if idx != nil && ib.Protocol == model.Shadowsocks {
		if c.Password == "" || !validShadowsocksClientKey(idx.topMethod, c.Password) {
			c.Password = randomShadowsocksClientKey(idx.topMethod)
		}
		return nil
	}
	return s.fillProtocolDefaults(c, ib)
}

func (s *ClientService) fillProtocolDefaultsAgainstExisting(c *model.Client, ib *model.Inbound, existing []model.Client, identityEmail ...string) error {
	if err := validateClientKeepAliveAgainstExisting(ib.Protocol, *c, existing, identityEmail...); err != nil {
		return err
	}
	switch ib.Protocol {
	case model.WireGuard:
		c.WireguardDevices = model.CloneWireguardDevices(c.WireguardDevices)
		for i := range c.WireguardDevices {
			if err := defaultWireguardDeviceIdentity(&c.WireguardDevices[i]); err != nil {
				return err
			}
		}
	case model.VMESS, model.VLESS:
		if c.ID == "" {
			c.ID = uuid.NewString()
		}
	case model.Trojan, model.MASQUE:
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
	case model.TUIC:
		if err := tuic.ValidateRelayEmail(c.Email); err != nil {
			return err
		}
		if c.ID == "" {
			c.ID = uuid.NewString()
		}
		if c.Password == "" {
			c.Password = strings.ReplaceAll(uuid.NewString(), "-", "")
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
		if v, _, ok := scanTopLevelStringFields(settings, "fakeTlsDomain", ""); ok {
			domain = v
		} else {
			var m map[string]any
			if err := json.Unmarshal([]byte(settings), &m); err == nil {
				domain, _ = m["fakeTlsDomain"].(string)
			}
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
	if v, _, ok := scanTopLevelStringFields(settings, "method", ""); ok {
		return v
	}
	method, _ := settingsField(settings, "method").(string)
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

// validateShadowsocks2022Inbound refuses a method or server key xray-core cannot
// build an inbound from: the core then does not start at all.
func validateShadowsocks2022Inbound(protocol model.Protocol, settings string) error {
	if protocol != model.Shadowsocks {
		return nil
	}
	var parsed struct {
		Method   string `json:"method"`
		Password string `json:"password"`
	}
	if json.Unmarshal([]byte(settings), &parsed) != nil {
		return nil
	}
	n := xray.Shadowsocks2022KeyBytes(parsed.Method)
	if n == 0 {
		return nil
	}
	if xray.Shadowsocks2022NoMultiUser(parsed.Method) {
		return common.NewErrorf("shadowsocks inbound method %s does not support multiple users, use a 2022-blake3-aes method", parsed.Method)
	}
	if !xray.ValidShadowsocks2022Key(parsed.Method, parsed.Password) {
		return common.NewErrorf("shadowsocks inbound password: %s needs a %d-byte key, base64 encoded", parsed.Method, n)
	}
	return nil
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
	return rewriteSettingsClients(settings, func(clients []any) bool {
		return regenerateMismatchedShadowsocksKeys(method, clients)
	})
}

func regenerateMismatchedShadowsocksKeys(method string, clients []any) bool {
	if shadowsocksKeyBytes(method) == 0 {
		return false
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
		changed = true
	}
	return changed
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
	return s.update(inboundSvc, id, updated, limitHwid, false, inboundFilter)
}

// keepInboundFlows: the caller sent no flow, so every inbound keeps its own and an
// unlinked record keeps its stored one.
func (s *ClientService) update(inboundSvc *InboundService, id int, updated model.Client, limitHwid int, keepInboundFlows bool, inboundFilter []int) (bool, error) {
	existing, err := s.GetByID(id)
	if err != nil {
		return false, err
	}
	defer watchClientChanges(clientChangeSourcePanel, existing.Email).record()
	inboundIds, err := s.GetInboundIdsForRecord(id)
	if err != nil {
		return false, err
	}
	// The rename rewrites the one shared client record, so every node holding
	// this client goes stale — not just the ones an inboundIds filter applies.
	attachedIds := append([]int(nil), inboundIds...)
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
	if emailErr := tuic.ValidateRelayEmail(updated.Email); emailErr != nil {
		var attached int64
		if err := database.GetDB().Model(&model.Inbound{}).
			Joins("JOIN client_inbounds ci ON ci.inbound_id = inbounds.id").
			Where("ci.client_id = ? AND inbounds.protocol = ?", id, model.TUIC).Count(&attached).Error; err != nil {
			return false, err
		}
		if attached > 0 {
			return false, emailErr
		}
	}
	if err := validateClientSubID(updated.SubID); err != nil {
		return false, err
	}
	if err := validateClientRenewal(updated); err != nil {
		return false, err
	}
	if err := validateClientResetMax(updated.ResetMax); err != nil {
		return false, err
	}
	if err := validateClientTrafficReset(updated.TrafficReset, updated.TrafficResetDay); err != nil {
		return false, err
	}
	existingClientsByInbound := make(map[int][]model.Client, len(attachedIds))
	if updated.KeepAlive != nil {
		for _, inboundID := range attachedIds {
			inbound, getErr := inboundSvc.GetInbound(inboundID)
			if getErr != nil {
				return false, fmt.Errorf("inbound %d: %w", inboundID, getErr)
			}
			clients, getErr := inboundSvc.GetClients(inbound)
			if getErr != nil {
				return false, fmt.Errorf("inbound %d: %w", inboundID, getErr)
			}
			existingClientsByInbound[inboundID] = clients
			if err := validateClientKeepAliveAgainstExisting(inbound.Protocol, updated, clients, existing.Email); err != nil {
				return false, fmt.Errorf("inbound %d: %w", inboundID, err)
			}
		}
	}
	normalizeClientTrafficReset(&updated)
	updated.ClientHostRuleIds = model.NormalizeClientHostRuleIds(updated.ClientHostRuleIds)
	if groupName := strings.TrimSpace(updated.Group); groupName != "" {
		group, err := database.EnsureClientGroup(database.GetDB(), groupName)
		if err != nil {
			return false, err
		}
		updated.Group = group.Name
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

	// A client the quota loop switched off comes back on when this update
	// restores a usable window, as BulkAdjust already does on a renewal (#5619).
	if !updated.Enable && s.renewalReenables(existing.Email, updated.TotalGB, updated.ExpiryTime) {
		updated.Enable = true
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

	var inboundFlows map[int]string
	if keepInboundFlows {
		updated.Flow = existing.Flow
		if inboundFlows, err = inboundFlowOverrides(id); err != nil {
			return false, err
		}
	}

	tunnelCount, tcErr := tunnelInboundCount(inboundIds)
	if tcErr != nil {
		return false, tcErr
	}

	if tunnelCount == 0 && len(inboundIds) > 0 && existing.Email != "" && updated.Email == existing.Email {
		if nr, handled, bErr := s.updateBatchedNonTunnel(inboundSvc, id, existing, updated, limitHwid, keepInboundFlows, inboundFlows, inboundIds, attachedIds); handled {
			return nr, bErr
		}
	}

	// Snapshot traffic-disabled state before fanning out so inbound #1 flipping
	// client_traffics.enable cannot mask runtime re-add on sibling inbounds.
	forceRuntimeSync := false
	if updated.Enable && existing.Email != "" {
		if disabled, tErr := isClientTrafficDisabled(database.GetDB(), existing.Email); tErr == nil && disabled {
			forceRuntimeSync = true
		}
	}

	// Built before any inbound is written, as in Create: fillProtocolDefaults
	// mints the shared credentials on the first inbound, later ones reuse them.
	applies := make([]inboundApply, 0, len(inboundIds))
	for _, ibId := range inboundIds {
		inbound, getErr := inboundSvc.GetInbound(ibId)
		if getErr != nil {
			if errors.Is(getErr, gorm.ErrRecordNotFound) {
				if err := database.GetDB().
					Where("client_id = ? AND inbound_id = ?", id, ibId).
					Delete(&model.ClientInbound{}).Error; err != nil {
					return false, err
				}
				continue
			}
			return false, getErr
		}
		if existing.Email == "" {
			continue
		}
		if err := s.fillProtocolDefaultsAgainstExisting(&updated, inbound, existingClientsByInbound[ibId], existing.Email); err != nil {
			return false, err
		}
		clientForInbound := updated
		if inbound.Protocol == model.WireGuard && tunnelCount > 1 {
			clientForInbound.WireguardDevices = nil
		}
		if keepInboundFlows {
			clientForInbound.Flow = inboundFlows[ibId]
		}
		if ips, ok := updated.AllowedIPsByInbound[ibId]; ok {
			clientForInbound.AllowedIPs = ips
		} else if tunnelCount > 1 && (inbound.Protocol == model.WireGuard || inbound.Protocol == model.AmneziaWG) {
			// One shared peer field set cannot describe several peers: broadcast
			// it and they all end up with the same keys and tunnel address.
			clientForInbound.AllowedIPs = nil
			clientForInbound.PrivateKey = ""
			clientForInbound.PublicKey = ""
			clientForInbound.PreSharedKey = ""
		} else if !addressesFitAmneziaWGInbound(clientForInbound.AllowedIPs, inbound) {
			clientForInbound.AllowedIPs = nil
		}
		if err := validateAmneziaWGClientWrite(inbound, []model.Client{clientForInbound}, existing.Email); err != nil {
			return false, fmt.Errorf("inbound %d: %w", ibId, err)
		}
		settingsPayload, mErr := json.Marshal(map[string][]model.Client{"clients": {clientWithInboundFlow(clientForInbound, inbound)}})
		if mErr != nil {
			return false, mErr
		}
		data := &model.Inbound{Id: ibId, Settings: string(settingsPayload)}
		applies = append(applies, inboundApply{id: ibId, run: func() (bool, error) {
			return s.updateInboundClient(inboundSvc, data, existing.Email, forceRuntimeSync)
		}})
	}
	// Each apply marks only its OWN node dirty, so between the first and last
	// one a merge could resurrect the pre-edit email as a second client.
	if err := markInboundNodesDirty(attachedIds); err != nil {
		return false, err
	}
	needRestart, applyErr := fanoutInboundApplies(applies)
	if applyErr != nil {
		return needRestart, applyErr
	}

	// UpdateInboundClient renames the record atomically with each inbound's
	// settings JSON; this direct write only covers records with no inbound left.
	if updated.Email != existing.Email {
		// Zero rows when the inbound fan-out above already renamed the record and its history.
		if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
			renamed := tx.Model(&model.ClientRecord{}).
				Where("id = ? AND email = ?", id, existing.Email).
				Update("email", updated.Email)
			if renamed.Error != nil || renamed.RowsAffected == 0 {
				return renamed.Error
			}
			return renameClientHistoryTx(tx, existing.Email, updated.Email, clientChangeSourcePanel)
		}); err != nil {
			return needRestart, err
		}
	}

	if len(inboundIds) == 0 {
		merged := *existing
		applyClientRecordMerge(&merged, updated.ToRecord())
		keepAliveLegacy, keepAliveRange, keepAliveErr := model.KeepAliveStorage(merged.KeepAlive)
		if keepAliveErr != nil {
			return needRestart, keepAliveErr
		}
		if err := database.GetDB().Model(&model.ClientRecord{}).
			Where("id = ?", id).
			Updates(map[string]any{
				"sub_id":               merged.SubID,
				"uuid":                 merged.UUID,
				"password":             merged.Password,
				"auth":                 merged.Auth,
				"secret":               merged.Secret,
				"flow":                 merged.Flow,
				"security":             merged.Security,
				"client_host_rule_ids": model.EncodeClientHostRuleIds(merged.ClientHostRuleIds),
				"wg_devices":           encodeWireguardDevices(merged.WireguardDevices),
				"wg_device_limit":      merged.WgDeviceLimit,
				"wg_private_key":       merged.PrivateKey,
				"wg_public_key":        merged.PublicKey,
				"wg_allowed_ips":       merged.AllowedIPs,
				"wg_pre_shared_key":    merged.PreSharedKey,
				"wg_keep_alive":        keepAliveLegacy,
				"wg_keep_alive_range":  keepAliveRange,
				"limit_ip":             merged.LimitIP,
				"total_gb":             merged.TotalGB,
				"expiry_time":          merged.ExpiryTime,
				"tg_id":                merged.TgID,
				"comment":              merged.Comment,
				"reset":                merged.Reset,
				"reset_day":            merged.ResetDay,
				"reset_weekday":        merged.ResetWeekday,
				"reset_max":            merged.ResetMax,
				"traffic_reset":        merged.TrafficReset,
				"traffic_reset_day":    merged.TrafficResetDay,
			}).Error; err != nil {
			return needRestart, err
		}
		// No inbound left, so UpdateInboundClient never syncs the enforcement row the
		// renewal and auto-disable loops read (#7160).
		if err := inboundSvc.UpdateClientStat(database.GetDB(), existing.Email, merged.ToClient()); err != nil {
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
		UpdateColumn("group_name", strings.TrimSpace(updated.Group)).Error; err != nil {
		return needRestart, err
	}

	// The AmneziaWG forwards too: the record merge keeps a stored spec when the settings
	// carry none, so a withdrawal is written here; nil (not sent) leaves it alone.
	if updated.ForwardedPorts != nil {
		if err := database.GetDB().Model(&model.ClientRecord{}).
			Where("id = ?", id).
			UpdateColumn("wg_forwarded_ports", updated.ForwardedPortsSpec()).Error; err != nil {
			return needRestart, err
		}
	}

	// Same shape as the group write above: SyncInbound keeps a stored ad-tag
	// when the incoming settings carry none, so clearing the override must be
	// applied here, where the editor always round-trips the field.
	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("id = ?", id).
		UpdateColumn("ad_tag", updated.AdTag).Error; err != nil {
		return needRestart, err
	}

	// nil preserves the stored binding, empty clears it.
	if updated.ClientHostRuleIds != nil {
		if err := database.GetDB().Model(&model.ClientRecord{}).
			Where("id = ?", id).
			Update("client_host_rule_ids", model.EncodeClientHostRuleIds(updated.ClientHostRuleIds)).Error; err != nil {
			return needRestart, err
		}
	}

	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("id = ?", id).
		UpdateColumn("enable", updated.Enable).Error; err != nil {
		return needRestart, err
	}

	if err := s.setClientLimitHwidByEmail(updated.Email, limitHwid); err != nil {
		return needRestart, err
	}

	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("id = ?", id).
		UpdateColumn("updated_at", time.Now().UnixMilli()).Error; err != nil {
		return needRestart, err
	}
	return needRestart, nil
}

type updateWritePlan struct {
	inbound      *model.Inbound
	prevSettings string
	newSettings  string
	oldClient    model.Client
	newClient    model.Client
	ssMethod     string
	changed      bool
}

func mapNumToInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i, true
		}
	}
	return 0, false
}

func (s *ClientService) updateBatchedNonTunnel(
	inboundSvc *InboundService,
	id int,
	existing *model.ClientRecord,
	updated model.Client,
	limitHwid int,
	keepInboundFlows bool,
	inboundFlows map[int]string,
	inboundIds []int,
	attachedIds []int,
) (bool, bool, error) {
	defer InvalidateSubData()
	unlock := lockInboundsSorted(inboundIds)
	defer unlock()

	var plans []updateWritePlan
	wasTrafficDisabled := false
	nowMs := time.Now().UnixMilli()

	txErr := runSerializedTx(func(tx *gorm.DB) error {
		inboundsByID, idxByID, err := loadInboundsForClientWrite(tx, inboundIds)
		if err != nil {
			return err
		}
		for _, ibId := range inboundIds {
			ib := inboundsByID[ibId]
			idx := idxByID[ibId]
			if ib == nil || ib.Protocol == model.WireGuard || ib.Protocol == model.AmneziaWG || idx == nil || !idx.canSplice() {
				return errFallbackSlowPath
			}
			if _, found := idx.findFoldIndex(existing.Email); !found {
				return errFallbackSlowPath
			}
		}

		for _, ibId := range inboundIds {
			if err := s.fillProtocolDefaultsWithIdx(&updated, inboundsByID[ibId], idxByID[ibId]); err != nil {
				return err
			}
		}

		settingsUpdates := make([]inboundSettingsUpdate, 0, len(inboundIds))
		deltas := make([]inboundClientDelta, 0, len(inboundIds))
		plans = make([]updateWritePlan, 0, len(inboundIds))

		for _, ibId := range inboundIds {
			ib := inboundsByID[ibId]
			idx := idxByID[ibId]
			pos, _ := idx.findFoldIndex(existing.Email)
			sp := idx.spans[pos]
			rawSlice := idx.raw[sp.start:sp.end]

			var oldClient model.Client
			if err := json.Unmarshal([]byte(rawSlice), &oldClient); err != nil {
				return errFallbackSlowPath
			}
			var oldClientMap map[string]any
			if err := json.Unmarshal([]byte(rawSlice), &oldClientMap); err != nil {
				return errFallbackSlowPath
			}

			clientForInbound := updated
			if keepInboundFlows {
				clientForInbound.Flow = inboundFlows[ibId]
			}
			clientForInbound = clientWithInboundFlowIdx(clientForInbound, ib, idx)
			if err := validateTuicClientEmails(ib.Protocol, []model.Client{clientForInbound}); err != nil {
				return err
			}
			if err := validateClientRequiredCredentials(ib.Protocol, clientForInbound); err != nil {
				return err
			}

			preservedCreated, hasPreservedCreated := oldClientMap["created_at"]
			if hasPreservedCreated {
				if cInt, okNum := mapNumToInt64(preservedCreated); okNum {
					clientForInbound.CreatedAt = cInt
				}
			} else {
				clientForInbound.CreatedAt = nowMs
			}
			if strings.TrimSpace(clientForInbound.SubID) == "" {
				if prevSub, _ := oldClientMap["subId"].(string); strings.TrimSpace(prevSub) != "" {
					clientForInbound.SubID = prevSub
				} else {
					clientForInbound.SubID = random.NumLower(16)
				}
			}

			ssMethod := ""
			if ib.Protocol == model.Shadowsocks {
				ssMethod = idx.topMethod
			}
			encoded, newSpan, mErr := marshalClientWire(clientForInbound, ssMethod, nowMs)
			if mErr != nil {
				return mErr
			}
			var newMap map[string]any
			if err := json.Unmarshal(encoded, &newMap); err != nil {
				return err
			}
			if hasPreservedCreated {
				newMap["created_at"] = preservedCreated
			}
			if sameClientConfigExceptUpdatedAt(oldClientMap, newMap) {
				ibCopy := *ib
				plans = append(plans, updateWritePlan{
					inbound:      &ibCopy,
					prevSettings: ib.Settings,
					newSettings:  ib.Settings,
					oldClient:    oldClient,
					newClient:    clientForInbound,
					ssMethod:     ssMethod,
					changed:      false,
				})
				continue
			}

			clientForInbound.UpdatedAt = nowMs
			if hasPreservedCreated {
				if _, okNum := mapNumToInt64(preservedCreated); !okNum {
					newMap["updated_at"] = nowMs
					encoded, mErr = json.Marshal(newMap)
					if mErr != nil {
						return mErr
					}
				}
			}
			newSettings, newIdx, okSplice := idx.spliceReplaceOne(pos, encoded, newSpan, ib.Protocol == model.VLESS)
			if !okSplice {
				return errFallbackSlowPath
			}
			ibCopy := *ib
			ibCopy.Settings = newSettings
			plans = append(plans, updateWritePlan{
				inbound:      &ibCopy,
				prevSettings: ib.Settings,
				newSettings:  newSettings,
				oldClient:    oldClient,
				newClient:    clientForInbound,
				ssMethod:     ssMethod,
				changed:      true,
			})
			settingsUpdates = append(settingsUpdates, inboundSettingsUpdate{
				inboundId:   ib.Id,
				newSettings: newSettings,
				newIdx:      newIdx,
			})
			deltas = append(deltas, inboundClientDelta{
				inboundId: ib.Id,
				protocol:  ib.Protocol,
				nodeID:    ib.NodeID,
				hasMeta:   true,
				changed:   []model.Client{clientForInbound},
			})
		}

		disabled, dErr := isClientTrafficDisabled(tx, existing.Email)
		if dErr != nil {
			return dErr
		}
		wasTrafficDisabled = disabled

		if err := inboundSvc.UpdateClientStat(tx, existing.Email, &updated); err != nil {
			return err
		}
		if err := commitInboundsSettingsBatch(tx, settingsUpdates); err != nil {
			return err
		}
		if err := s.ApplyMultiInboundClientDelta(tx, deltas); err != nil {
			return err
		}

		if len(attachedIds) > 0 {
			var nodeIDs []int
			for _, batch := range chunkInts(attachedIds, sqlInChunk) {
				var ids []int
				if err := tx.Model(&model.Inbound{}).
					Where("id IN ? AND node_id IS NOT NULL", batch).
					Distinct().Pluck("node_id", &ids).Error; err != nil {
					return err
				}
				nodeIDs = append(nodeIDs, ids...)
			}
			if len(nodeIDs) > 0 {
				nodeSvc := &NodeService{}
				for _, nid := range uniqueInts(nodeIDs) {
					if err := nodeSvc.MarkNodeDirtyTx(tx, nid); err != nil {
						return err
					}
				}
			}
		}

		reverseStr := ""
		if updated.Reverse != nil && strings.TrimSpace(updated.Reverse.Tag) != "" {
			if b, mErr := json.Marshal(updated.Reverse); mErr == nil {
				reverseStr = string(b)
			}
		}
		recCols := map[string]any{
			"reverse":    reverseStr,
			"group_name": strings.TrimSpace(updated.Group),
			"ad_tag":     updated.AdTag,
			"enable":     updated.Enable,
			"updated_at": nowMs,
		}
		if updated.ForwardedPorts != nil {
			recCols["wg_forwarded_ports"] = updated.ForwardedPortsSpec()
		}
		if updated.ClientHostRuleIds != nil {
			recCols["client_host_rule_ids"] = model.EncodeClientHostRuleIds(updated.ClientHostRuleIds)
		}
		if err := tx.Model(&model.ClientRecord{}).Where("id = ?", id).Updates(recCols).Error; err != nil {
			return err
		}
		if limitHwid > 0 || existing.LimitHwid != limitHwid {
			if err := s.setClientLimitHwidByEmailTx(tx, updated.Email, limitHwid); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(txErr, errFallbackSlowPath) {
		return false, false, nil
	}
	if txErr != nil {
		return false, true, txErr
	}
	unlock()

	activePlans := make([]updateWritePlan, 0, len(plans))
	hasRemote := false
	for i := range plans {
		wp := plans[i]
		if !wp.changed && (wp.inbound.NodeID != nil || !wasTrafficDisabled || !wp.newClient.Enable) {
			continue
		}
		if wp.inbound.NodeID == nil &&
			wp.inbound.Protocol != model.MTProto &&
			wp.inbound.Protocol != model.TUIC &&
			wp.oldClient.Enable && wp.newClient.Enable &&
			wp.oldClient.Reverse == nil && wp.newClient.Reverse == nil &&
			!wasTrafficDisabled &&
			!clientRuntimeConfigChanged(&wp.oldClient, &wp.newClient) {
			continue
		}
		if wp.inbound.NodeID != nil {
			hasRemote = true
		}
		activePlans = append(activePlans, wp)
	}
	if len(activePlans) == 0 {
		return false, true, nil
	}

	planIDs := make([]int, len(activePlans))
	for i := range activePlans {
		planIDs[i] = activePlans[i].inbound.Id
	}
	needRestart := false
	var errs []error
	outcomes, panics := fanoutInboundResults(planIDs, runtimeFanoutLimit(hasRemote), func(i int) inboundApplyOutcome {
		wp := activePlans[i]
		rt, push, _, perr := inboundSvc.nodePushPlan(wp.inbound)
		if perr != nil {
			return inboundApplyOutcome{err: fmt.Errorf("inbound %d: %w", wp.inbound.Id, perr)}
		}
		var out inboundApplyOutcome
		if wp.inbound.NodeID == nil {
			if !push {
				out.needRestart = true
			} else if wp.inbound.Protocol == model.MTProto {
				inboundSvc.applyLocalMtproto(wp.inbound.Id)
			} else if wp.inbound.Protocol == model.TUIC {
				inboundSvc.applyLocalTuic(wp.inbound.Id)
			} else {
				if wp.oldClient.Enable {
					err1 := rt.RemoveUser(context.Background(), wp.inbound, existing.Email)
					if err1 == nil {
						if !wp.newClient.Enable && droppedClientNeedsRestart() {
							out.needRestart = true
						}
					} else if !strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", existing.Email)) {
						out.needRestart = true
					}
				}
				if wp.newClient.Enable {
					if err1 := rt.AddUser(context.Background(), wp.inbound, coreUserFromClient(&wp.newClient, wp.ssMethod)); err1 != nil {
						out.needRestart = true
					}
				}
			}
			return out
		}
		if push {
			ctx, cancel := nodePushContext()
			err1 := rt.UpdateUser(ctx, wp.inbound, existing.Email, wp.newClient)
			cancel()
			if err1 != nil {
				logger.Warning("Error in updating client on", rt.Name(), ":", err1)
			} else {
				advancePushedInbound(rt, wp.prevSettings, wp.newSettings, wp.inbound)
			}
		}
		return out
	})
	for i, out := range outcomes {
		if panics[i] != nil {
			needRestart = true
			errs = append(errs, panics[i])
			continue
		}
		if out.needRestart {
			needRestart = true
		}
		if out.err != nil {
			errs = append(errs, out.err)
		}
	}
	return needRestart, true, errors.Join(errs...)
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

	applies := make([]inboundApply, 0, len(inboundIds))
	var delErrs []error
	if existing.Email != "" && len(inboundIds) > 0 {
		existingIbIDs := make(map[int]struct{}, len(inboundIds))
		for _, batch := range chunkInts(uniqueInts(inboundIds), sqlInChunk) {
			var found []int
			if qErr := database.GetDB().Model(&model.Inbound{}).Where("id IN ?", batch).Pluck("id", &found).Error; qErr != nil {
				withdrawClientTombstones(existing.Email)
				return false, qErr
			}
			for _, fid := range found {
				existingIbIDs[fid] = struct{}{}
			}
		}
		for _, ibId := range inboundIds {
			if _, ok := existingIbIDs[ibId]; !ok {
				continue
			}
			applies = append(applies, inboundApply{id: ibId, run: func() (bool, error) {
				nr, delErr := s.DelInboundClientByEmail(inboundSvc, ibId, existing.Email, keepTraffic, true)
				if errors.Is(delErr, ErrClientNotInInbound) {
					return nr, nil
				}
				return nr, delErr
			}})
		}
	}
	needRestart, applyErr := fanoutInboundApplies(applies)
	if applyErr != nil {
		delErrs = append(delErrs, applyErr)
	}
	// A failed inbound still holds the client in its settings JSON: keep the
	// record so the next delete retries exactly the leftovers, and report it.
	// The tombstone lifts with it, or the next node merge finishes the deletion.
	if len(delErrs) > 0 {
		withdrawClientTombstones(existing.Email)
		return needRestart, errors.Join(delErrs...)
	}

	if err := runSerializedTx(func(tx *gorm.DB) error {
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
// existing WireGuard or AmneziaWG inbound.
func (s *ClientService) hasTunnelAttachment(_ *InboundService, inboundIds []int) bool {
	n, err := tunnelInboundCount(inboundIds)
	return err == nil && n > 0
}

// tunnelInboundCount reports how many of inboundIds are WireGuard/AmneziaWG,
// i.e. how many independent peers one shared field set would be written to.
func tunnelInboundCount(inboundIds []int) (int64, error) {
	if len(inboundIds) == 0 {
		return 0, nil
	}
	var n int64
	err := database.GetDB().Model(&model.Inbound{}).
		Where("id IN ? AND protocol IN ?", inboundIds, []model.Protocol{model.WireGuard, model.AmneziaWG}).
		Count(&n).Error
	return n, err
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

func propagateMintedCredentials(dst *model.Client, src model.Client) {
	if dst.ID == "" {
		dst.ID = src.ID
	}
	if dst.Password == "" {
		dst.Password = src.Password
	}
	if dst.Auth == "" {
		dst.Auth = src.Auth
	}
	if dst.Secret == "" {
		dst.Secret = src.Secret
	}
}

// Attach applies the client to every requested inbound: one failing inbound no
// longer aborts the others, so the error can name several and needRestart holds.
func (s *ClientService) Attach(inboundSvc *InboundService, id int, inboundIds []int) (bool, error) {
	inboundIds = uniqueInts(inboundIds)
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
	targetIDs := make([]int, 0, len(inboundIds))
	for _, ibId := range inboundIds {
		if _, attached := have[ibId]; !attached {
			targetIDs = append(targetIDs, ibId)
		}
	}
	if len(targetIDs) == 0 {
		return false, nil
	}

	clientWire := existing.ToClient()
	flow, ffErr := s.EffectiveFlow(nil, id)
	if ffErr != nil {
		return false, ffErr
	}
	clientWire.Flow = flow
	clientWire.UpdatedAt = time.Now().UnixMilli()
	clientWire.ForwardedPorts = nil

	if nr, handled, bErr := s.attachBatchedNonTunnel(inboundSvc, *clientWire, targetIDs); handled {
		return nr, bErr
	}

	if !s.hasTunnelAttachment(inboundSvc, currentIds) {
		clientWire.AllowedIPs = nil
		clientWire.WireguardDevices = model.CloneWireguardDevices(clientWire.WireguardDevices)
		for i := range clientWire.WireguardDevices {
			clientWire.WireguardDevices[i].AllowedIPs = nil
		}
	}

	trafficDisabled, _ := isClientTrafficDisabled(database.GetDB(), existing.Email)
	adds := make([]*model.Inbound, 0, len(targetIDs))
	for _, ibId := range targetIDs {
		inbound, getErr := inboundSvc.GetInbound(ibId)
		if getErr != nil {
			return false, fmt.Errorf("inbound %d: %w", ibId, getErr)
		}
		copyClient := *clientWire
		copyClient.ForwardedPorts = nil
		prepareWireguardDeviceAddresses(&copyClient, inbound)
		if !addressesFitAmneziaWGInbound(copyClient.AllowedIPs, inbound) {
			copyClient.AllowedIPs = nil
		}
		if err := s.fillProtocolDefaults(&copyClient, inbound); err != nil {
			return false, fmt.Errorf("inbound %d: %w", ibId, err)
		}
		propagateMintedCredentials(clientWire, copyClient)
		if trafficDisabled {
			copyClient.Enable = false
		}
		settingsPayload, mErr := json.Marshal(map[string][]model.Client{"clients": {clientWithInboundFlow(copyClient, inbound)}})
		if mErr != nil {
			return false, fmt.Errorf("inbound %d: %w", ibId, mErr)
		}
		adds = append(adds, &model.Inbound{Id: ibId, Settings: string(settingsPayload)})
	}
	return s.fanoutInboundClientAdds(inboundSvc, adds)
}

func (s *ClientService) attachBatchedNonTunnel(inboundSvc *InboundService, client model.Client, targetIDs []int) (bool, bool, error) {
	defer InvalidateSubData()
	unlock := lockInboundsSorted(targetIDs)
	defer unlock()

	var plans []createWritePlan
	var errs []error
	trafficEnabled := client.Enable
	nowMs := client.UpdatedAt

	txErr := runSerializedTx(func(tx *gorm.DB) error {
		inboundsByID, idxByID, err := loadInboundsForClientWrite(tx, targetIDs)
		if err != nil {
			return err
		}
		for _, ibId := range targetIDs {
			ib := inboundsByID[ibId]
			if ib == nil {
				return fmt.Errorf("inbound %d: %w", ibId, gorm.ErrRecordNotFound)
			}
			if ib.Protocol == model.WireGuard || ib.Protocol == model.AmneziaWG {
				return errFallbackSlowPath
			}
		}

		for _, ibId := range targetIDs {
			if err := s.fillProtocolDefaultsWithIdx(&client, inboundsByID[ibId], idxByID[ibId]); err != nil {
				return fmt.Errorf("inbound %d: %w", ibId, err)
			}
		}

		hasTrafficRow := false
		if client.Email != "" {
			var enables []bool
			if err := tx.Model(xray.ClientTraffic{}).Where("email = ?", client.Email).Limit(1).Pluck("enable", &enables).Error; err != nil {
				return err
			}
			if len(enables) > 0 {
				hasTrafficRow = true
				trafficEnabled = enables[0]
			}
		}

		settingsUpdates := make([]inboundSettingsUpdate, 0, len(targetIDs))
		deltas := make([]inboundClientDelta, 0, len(targetIDs))
		for _, ibId := range targetIDs {
			ib := inboundsByID[ibId]
			idx := idxByID[ibId]
			if err := validateTuicClientEmails(ib.Protocol, []model.Client{client}); err != nil {
				return fmt.Errorf("inbound %d: %w", ibId, err)
			}
			finalClient := clientWithInboundFlowIdx(client, ib, idx)
			if err := validateClientRequiredCredentials(ib.Protocol, finalClient); err != nil {
				errs = append(errs, fmt.Errorf("inbound %d: %w", ibId, err))
				continue
			}

			if idx != nil && idx.canSplice() {
				if idx.hasFoldEmail(finalClient.Email) {
					continue
				}
				ssMethod := ""
				if ib.Protocol == model.Shadowsocks {
					ssMethod = idx.topMethod
				}
				encoded, sp, mErr := marshalClientWire(finalClient, ssMethod, nowMs)
				if mErr != nil {
					errs = append(errs, fmt.Errorf("inbound %d: %w", ibId, mErr))
					continue
				}
				newSettings, newIdx := idx.spliceAppend([][]byte{encoded}, []clientSpan{sp})
				prevSettings := ib.Settings
				ibCopy := *ib
				ibCopy.Settings = newSettings
				plans = append(plans, createWritePlan{
					inbound:      &ibCopy,
					prevSettings: prevSettings,
					newSettings:  newSettings,
					client:       finalClient,
					ssMethod:     ssMethod,
				})
				settingsUpdates = append(settingsUpdates, inboundSettingsUpdate{
					inboundId:   ib.Id,
					newSettings: newSettings,
					newIdx:      newIdx,
				})
				deltas = append(deltas, inboundClientDelta{
					inboundId: ib.Id,
					protocol:  ib.Protocol,
					nodeID:    ib.NodeID,
					hasMeta:   true,
					changed:   []model.Client{finalClient},
				})
				continue
			}

			doc, dErr := parseRawSettingsDoc(ib.Settings)
			if dErr != nil {
				errs = append(errs, fmt.Errorf("inbound %d: %w", ibId, dErr))
				continue
			}
			dupOnInbound := false
			for i := range doc.rawClients {
				if em := doc.clientHeaderAt(i).Email; em != "" && strings.EqualFold(em, finalClient.Email) {
					dupOnInbound = true
					break
				}
			}
			if dupOnInbound {
				continue
			}
			ssMethod := ""
			if ib.Protocol == model.Shadowsocks {
				ssMethod = doc.topString("method")
			}
			encoded, _, mErr := marshalClientWire(finalClient, ssMethod, nowMs)
			if mErr != nil {
				errs = append(errs, fmt.Errorf("inbound %d: %w", ibId, mErr))
				continue
			}
			doc.hasClients = true
			doc.rawClients = append(doc.rawClients, json.RawMessage(encoded))
			newSettings, eErr := doc.encodeCompact()
			if eErr != nil {
				errs = append(errs, fmt.Errorf("inbound %d: %w", ibId, eErr))
				continue
			}
			prevSettings := ib.Settings
			ibCopy := *ib
			ibCopy.Settings = newSettings
			plans = append(plans, createWritePlan{
				inbound:      &ibCopy,
				prevSettings: prevSettings,
				newSettings:  newSettings,
				client:       finalClient,
				ssMethod:     ssMethod,
			})
			settingsUpdates = append(settingsUpdates, inboundSettingsUpdate{
				inboundId:   ib.Id,
				newSettings: newSettings,
			})
			deltas = append(deltas, inboundClientDelta{
				inboundId: ib.Id,
				protocol:  ib.Protocol,
				nodeID:    ib.NodeID,
				hasMeta:   true,
				changed:   []model.Client{finalClient},
			})
		}

		if len(plans) > 0 {
			if !hasTrafficRow && plans[0].client.Email != "" {
				if err := inboundSvc.AddClientStat(tx, plans[0].inbound.Id, &plans[0].client); err != nil {
					return err
				}
			}
			if err := commitInboundsSettingsBatch(tx, settingsUpdates); err != nil {
				return err
			}
			if err := s.ApplyMultiInboundClientDelta(tx, deltas); err != nil {
				return err
			}
			dirtyNodes := make(map[int]struct{})
			for _, wp := range plans {
				if wp.inbound.NodeID != nil {
					dirtyNodes[*wp.inbound.NodeID] = struct{}{}
				}
			}
			if len(dirtyNodes) > 0 {
				nodeSvc := &NodeService{}
				for nodeID := range dirtyNodes {
					if err := nodeSvc.MarkNodeDirtyTx(tx, nodeID); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	if errors.Is(txErr, errFallbackSlowPath) {
		return false, false, nil
	}
	if txErr != nil {
		return false, true, txErr
	}
	unlock()

	needRestart := false
	if len(plans) > 0 {
		hasRemote := false
		planIDs := make([]int, len(plans))
		for i := range plans {
			planIDs[i] = plans[i].inbound.Id
			if plans[i].inbound.NodeID != nil {
				hasRemote = true
			}
		}
		outcomes, panics := fanoutInboundResults(planIDs, runtimeFanoutLimit(hasRemote), func(i int) inboundApplyOutcome {
			wp := plans[i]
			rt, push, _, perr := inboundSvc.nodePushPlan(wp.inbound)
			if perr != nil {
				return inboundApplyOutcome{err: fmt.Errorf("inbound %d: %w", wp.inbound.Id, perr)}
			}
			var out inboundApplyOutcome
			if wp.inbound.NodeID == nil {
				if !push {
					out.needRestart = true
				} else if wp.inbound.Protocol == model.MTProto {
					inboundSvc.applyLocalMtproto(wp.inbound.Id)
				} else if wp.inbound.Protocol == model.TUIC {
					inboundSvc.applyLocalTuic(wp.inbound.Id)
				} else if wp.client.Enable && trafficEnabled {
					op := runtime.UserAddOp{
						User:    coreUserFromClient(&wp.client, wp.ssMethod),
						Replace: false,
					}
					if localBatchAddUsers(rt, wp.inbound, []runtime.UserAddOp{op}) {
						out.needRestart = true
					}
				}
				return out
			}
			if push && wp.client.Enable && trafficEnabled {
				ctx, cancel := nodePushContext()
				err1 := rt.AddClient(ctx, wp.inbound, wp.client)
				cancel()
				if err1 != nil {
					logger.Warning("Error in adding client on", rt.Name(), ":", err1)
				} else {
					advancePushedInbound(rt, wp.prevSettings, wp.newSettings, wp.inbound)
				}
			}
			return out
		})
		for i, out := range outcomes {
			if panics[i] != nil {
				needRestart = true
				errs = append(errs, panics[i])
				continue
			}
			if out.needRestart {
				needRestart = true
			}
			if out.err != nil {
				errs = append(errs, out.err)
			}
		}
	}
	return needRestart, true, errors.Join(errs...)
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
	applies := make([]inboundApply, 0, len(inboundIds))
	for _, ibId := range inboundIds {
		applies = append(applies, inboundApply{id: ibId, run: func() (bool, error) {
			nr, delErr := s.DelInboundClientByEmail(inboundSvc, ibId, email, keepTraffic, true)
			if errors.Is(delErr, ErrClientNotInInbound) {
				return nr, nil
			}
			return nr, delErr
		}})
	}
	needRestart, delErr := fanoutInboundApplies(applies)
	if delErr != nil {
		return needRestart, delErr
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

	targetIDs := make([]int, 0, len(inboundIds))
	for _, ibId := range uniqueInts(inboundIds) {
		if _, attached := have[ibId]; attached {
			targetIDs = append(targetIDs, ibId)
		}
	}
	if len(targetIDs) == 0 {
		return false, nil
	}

	// Verify existence via ID-only query instead of loading multi-MB settings blobs.
	var foundIDs []int
	for _, batch := range chunkInts(targetIDs, sqlInChunk) {
		var rows []int
		if err := database.GetDB().Model(&model.Inbound{}).Where("id IN ?", batch).Pluck("id", &rows).Error; err != nil {
			return false, err
		}
		foundIDs = append(foundIDs, rows...)
	}
	if len(foundIDs) != len(targetIDs) {
		return false, gorm.ErrRecordNotFound
	}
	if existing.Email == "" {
		return false, nil
	}

	applies := make([]inboundApply, 0, len(targetIDs))
	for _, ibId := range targetIDs {
		applies = append(applies, inboundApply{id: ibId, run: func() (bool, error) {
			nr, delErr := s.DelInboundClientByEmail(inboundSvc, ibId, existing.Email, true, false)
			if errors.Is(delErr, ErrClientNotInInbound) {
				return nr, nil
			}
			return nr, delErr
		}})
	}
	return fanoutInboundApplies(applies)
}
