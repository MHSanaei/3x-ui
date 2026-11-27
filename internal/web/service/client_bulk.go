package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

// BulkAttachResult reports the outcome of a bulk attach across target inbounds.
type BulkAttachResult struct {
	Attached []string `json:"attached"`
	Skipped  []string `json:"skipped"`
	Errors   []string `json:"errors"`
}

func propagateMintedRecordCredentials(rec *model.ClientRecord, src model.Client) {
	if rec.UUID == "" {
		rec.UUID = src.ID
	}
	if rec.Password == "" {
		rec.Password = src.Password
	}
	if rec.Auth == "" {
		rec.Auth = src.Auth
	}
	if rec.Secret == "" {
		rec.Secret = src.Secret
	}
}

// BulkAttach attaches the given existing clients (by email) to each target inbound,
// reusing their identity (email/UUID/password/subId) and shared traffic row.
func (s *ClientService) BulkAttach(inboundSvc *InboundService, emails []string, inboundIds []int) (*BulkAttachResult, bool, error) {
	inboundIds = uniqueInts(inboundIds)
	result := &BulkAttachResult{}
	if len(emails) == 0 || len(inboundIds) == 0 {
		return result, false, nil
	}

	recordErr := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		result.Errors = append(result.Errors, msg)
		logger.Warningf("[BulkAttach] %s", msg)
	}

	cleanEmails := make([]string, 0, len(emails))
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
		cleanEmails = append(cleanEmails, email)
	}
	if len(cleanEmails) == 0 {
		return result, false, nil
	}

	db := database.GetDB()
	recordsByExact, err := clientRecordsByEmail(db, cleanEmails)
	if err != nil {
		return result, false, err
	}
	records := make([]*model.ClientRecord, 0, len(cleanEmails))
	for _, email := range cleanEmails {
		rec := recordsByExact[email]
		if rec == nil {
			var errLookup error
			rec, errLookup = s.GetRecordByEmail(db, email)
			if errLookup != nil {
				recordErr("%s: %v", email, errLookup)
				continue
			}
		}
		cp := *rec
		records = append(records, &cp)
	}
	if len(records) == 0 {
		return result, false, nil
	}

	// Same rule as Attach (#4834): clients.flow is unreliable when a non-flow
	// inbound synced last, so seed from EffectiveFlow before clientWithInboundFlow.
	emailsForFlow := make([]string, 0, len(records))
	for _, rec := range records {
		emailsForFlow = append(emailsForFlow, rec.Email)
	}
	flowsByEmail, err := s.EffectiveFlowsByEmails(db, emailsForFlow)
	if err != nil {
		return result, false, err
	}

	tunnelCnt, tcErr := tunnelInboundCount(inboundIds)
	if tcErr != nil {
		return result, false, tcErr
	}
	if tunnelCnt == 0 {
		return s.bulkAttachBatchedNonTunnel(inboundSvc, records, emailsForFlow, flowsByEmail, inboundIds, result, recordErr)
	}

	trafficEnableByEmail := make(map[string]bool, len(emailsForFlow))
	for _, batch := range chunkStrings(emailsForFlow, sqlInChunk) {
		var rows []struct {
			Email  string
			Enable bool
		}
		if err := db.Model(xray.ClientTraffic{}).Select("email, enable").Where("email IN ?", batch).Scan(&rows).Error; err != nil {
			return result, false, err
		}
		for _, r := range rows {
			trafficEnableByEmail[r.Email] = r.Enable
		}
	}

	needRestart := false
	attachIds := make([]int, 0, len(inboundIds))
	attachPayloads := make([]string, 0, len(inboundIds))
	attachClients := make([][]model.Client, 0, len(inboundIds))
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
			client.ForwardedPorts = nil
			prepareWireguardDeviceAddresses(&client, inbound)
			if flow, ok := flowsByEmail[rec.Email]; ok && flow != "" {
				client.Flow = flow
			}
			client.UpdatedAt = time.Now().UnixMilli()
			if err := s.fillProtocolDefaults(&client, inbound); err != nil {
				recordErr("%s -> inbound %d: %v", rec.Email, ibId, err)
				continue
			}
			propagateMintedRecordCredentials(rec, client)
			if en, hasRow := trafficEnableByEmail[rec.Email]; hasRow && !en {
				client.Enable = false
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
		attachIds = append(attachIds, ibId)
		attachPayloads = append(attachPayloads, string(payload))
		attachClients = append(attachClients, clientsToAdd)
	}

	attachResults, attachPanics := fanoutInboundResults(attachIds, addFanoutLimit(true), func(i int) inboundApplyOutcome {
		nr, err := s.AddInboundClient(inboundSvc, &model.Inbound{Id: attachIds[i], Settings: attachPayloads[i]})
		return inboundApplyOutcome{needRestart: nr, err: err}
	})
	for i, out := range attachResults {
		err := out.err
		if attachPanics[i] != nil {
			needRestart = true
			err = attachPanics[i]
		}
		if err != nil {
			recordErr("inbound %d: %v", attachIds[i], err)
			continue
		}
		if out.needRestart {
			needRestart = true
		}
		for _, c := range attachClients[i] {
			result.Attached = append(result.Attached, c.Email)
		}
	}

	return result, needRestart, nil
}

func (s *ClientService) bulkAttachBatchedNonTunnel(
	inboundSvc *InboundService,
	records []*model.ClientRecord,
	emailsForFlow []string,
	flowsByEmail map[string]string,
	targetIDs []int,
	result *BulkAttachResult,
	recordErr func(format string, args ...any),
) (*BulkAttachResult, bool, error) {
	defer InvalidateSubData()
	unlock := lockInboundsSorted(targetIDs)
	defer unlock()

	type attachPlan struct {
		inbound      *model.Inbound
		prevSettings string
		newSettings  string
		added        []model.Client
		ssMethod     string
	}
	var plans []attachPlan
	trafficEnableByEmail := make(map[string]bool, len(emailsForFlow))
	nowMs := time.Now().UnixMilli()

	if txErr := runSerializedTx(func(tx *gorm.DB) error {
		inboundsByID, idxByID, loadErr := loadInboundsForClientWrite(tx, targetIDs)
		if loadErr != nil {
			return loadErr
		}

		for _, batch := range chunkStrings(emailsForFlow, sqlInChunk) {
			var rows []struct {
				Email  string
				Enable bool
			}
			if err := tx.Model(xray.ClientTraffic{}).Select("email, enable").Where("email IN ?", batch).Scan(&rows).Error; err != nil {
				return err
			}
			for _, r := range rows {
				trafficEnableByEmail[r.Email] = r.Enable
			}
		}

		plans = make([]attachPlan, 0, len(targetIDs))
		settingsUpdates := make([]inboundSettingsUpdate, 0, len(targetIDs))
		deltas := make([]inboundClientDelta, 0, len(targetIDs))
		dirtyNodes := make(map[int]struct{})

		for _, ibId := range targetIDs {
			ib, ok := inboundsByID[ibId]
			if !ok || ib == nil {
				recordErr("inbound %d: %v", ibId, gorm.ErrRecordNotFound)
				continue
			}
			idx := idxByID[ibId]
			if idx != nil && idx.canSplice() {
				clientsToAdd := make([]model.Client, 0, len(records))
				for _, rec := range records {
					if idx.hasFoldEmail(rec.Email) {
						result.Skipped = append(result.Skipped, rec.Email)
						continue
					}
					client := *rec.ToClient()
					client.ForwardedPorts = nil
					if flow, hasFlow := flowsByEmail[rec.Email]; hasFlow && flow != "" {
						client.Flow = flow
					}
					client.UpdatedAt = nowMs
					if err := s.fillProtocolDefaultsWithIdx(&client, ib, idx); err != nil {
						recordErr("%s -> inbound %d: %v", rec.Email, ibId, err)
						continue
					}
					propagateMintedRecordCredentials(rec, client)
					finalClient := clientWithInboundFlowIdx(client, ib, idx)
					if err := validateClientsRenewal([]model.Client{finalClient}); err != nil {
						recordErr("%s -> inbound %d: %v", rec.Email, ibId, err)
						continue
					}
					if err := validateTuicClientEmails(ib.Protocol, []model.Client{finalClient}); err != nil {
						recordErr("%s -> inbound %d: %v", rec.Email, ibId, err)
						continue
					}
					if err := validateClientRequiredCredentials(ib.Protocol, finalClient); err != nil {
						recordErr("inbound %d: %v", ibId, err)
						clientsToAdd = nil
						break
					}
					clientsToAdd = append(clientsToAdd, finalClient)
				}
				if len(clientsToAdd) == 0 {
					continue
				}
				ssMethod := ""
				if ib.Protocol == model.Shadowsocks {
					ssMethod = idx.topMethod
				}
				encodedList := make([][]byte, 0, len(clientsToAdd))
				spanList := make([]clientSpan, 0, len(clientsToAdd))
				marshalOK := true
				for _, c := range clientsToAdd {
					enc, sp, mErr := marshalClientWire(c, ssMethod, nowMs)
					if mErr != nil {
						recordErr("inbound %d: %v", ibId, mErr)
						marshalOK = false
						break
					}
					encodedList = append(encodedList, enc)
					spanList = append(spanList, sp)
				}
				if !marshalOK {
					continue
				}
				newSettings, newIdx := idx.spliceAppend(encodedList, spanList)
				prevSettings := ib.Settings
				ibCopy := *ib
				ibCopy.Settings = newSettings
				for i := range clientsToAdd {
					if _, hasRow := trafficEnableByEmail[clientsToAdd[i].Email]; hasRow {
						continue
					}
					if e := inboundSvc.AddClientStat(tx, ib.Id, &clientsToAdd[i]); e != nil {
						return e
					}
					trafficEnableByEmail[clientsToAdd[i].Email] = clientsToAdd[i].Enable
				}
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
					changed:   clientsToAdd,
				})
				if ib.NodeID != nil {
					dirtyNodes[*ib.NodeID] = struct{}{}
				}
				plans = append(plans, attachPlan{
					inbound:      &ibCopy,
					prevSettings: prevSettings,
					newSettings:  newSettings,
					added:        clientsToAdd,
					ssMethod:     ssMethod,
				})
				continue
			}

			doc, dErr := parseRawSettingsDoc(ib.Settings)
			if dErr != nil {
				recordErr("inbound %d: %v", ibId, dErr)
				continue
			}
			have := make(map[string]struct{}, len(doc.rawClients))
			for i := range doc.rawClients {
				if em := doc.clientHeaderAt(i).Email; em != "" {
					have[strings.ToLower(em)] = struct{}{}
				}
			}
			clientsToAdd := make([]model.Client, 0, len(records))
			for _, rec := range records {
				if _, attached := have[strings.ToLower(rec.Email)]; attached {
					result.Skipped = append(result.Skipped, rec.Email)
					continue
				}
				client := *rec.ToClient()
				client.ForwardedPorts = nil
				if flow, hasFlow := flowsByEmail[rec.Email]; hasFlow && flow != "" {
					client.Flow = flow
				}
				client.UpdatedAt = nowMs
				if err := s.fillProtocolDefaults(&client, ib); err != nil {
					recordErr("%s -> inbound %d: %v", rec.Email, ibId, err)
					continue
				}
				propagateMintedRecordCredentials(rec, client)
				finalClient := clientWithInboundFlow(client, ib)
				if err := validateClientsRenewal([]model.Client{finalClient}); err != nil {
					recordErr("%s -> inbound %d: %v", rec.Email, ibId, err)
					continue
				}
				if err := validateTuicClientEmails(ib.Protocol, []model.Client{finalClient}); err != nil {
					recordErr("%s -> inbound %d: %v", rec.Email, ibId, err)
					continue
				}
				if err := validateClientRequiredCredentials(ib.Protocol, finalClient); err != nil {
					recordErr("inbound %d: %v", ibId, err)
					clientsToAdd = nil
					break
				}
				clientsToAdd = append(clientsToAdd, finalClient)
			}
			if len(clientsToAdd) == 0 {
				continue
			}
			ssMethod := ""
			if ib.Protocol == model.Shadowsocks {
				ssMethod = doc.topString("method")
			}
			marshalOK := true
			for _, c := range clientsToAdd {
				enc, _, mErr := marshalClientWire(c, ssMethod, nowMs)
				if mErr != nil {
					recordErr("inbound %d: %v", ibId, mErr)
					marshalOK = false
					break
				}
				doc.hasClients = true
				doc.rawClients = append(doc.rawClients, json.RawMessage(enc))
			}
			if !marshalOK {
				continue
			}
			newSettings, eErr := doc.encodeCompact()
			if eErr != nil {
				recordErr("inbound %d: %v", ibId, eErr)
				continue
			}
			prevSettings := ib.Settings
			ibCopy := *ib
			ibCopy.Settings = newSettings
			for i := range clientsToAdd {
				if _, hasRow := trafficEnableByEmail[clientsToAdd[i].Email]; hasRow {
					continue
				}
				if e := inboundSvc.AddClientStat(tx, ib.Id, &clientsToAdd[i]); e != nil {
					return e
				}
				trafficEnableByEmail[clientsToAdd[i].Email] = clientsToAdd[i].Enable
			}
			settingsUpdates = append(settingsUpdates, inboundSettingsUpdate{
				inboundId:   ib.Id,
				newSettings: newSettings,
			})
			deltas = append(deltas, inboundClientDelta{
				inboundId: ib.Id,
				protocol:  ib.Protocol,
				nodeID:    ib.NodeID,
				hasMeta:   true,
				changed:   clientsToAdd,
			})
			if ib.NodeID != nil {
				dirtyNodes[*ib.NodeID] = struct{}{}
			}
			plans = append(plans, attachPlan{
				inbound:      &ibCopy,
				prevSettings: prevSettings,
				newSettings:  newSettings,
				added:        clientsToAdd,
				ssMethod:     ssMethod,
			})
		}

		if err := commitInboundsSettingsBatch(tx, settingsUpdates); err != nil {
			return err
		}
		if err := s.ApplyMultiInboundClientDelta(tx, deltas); err != nil {
			return err
		}
		if len(dirtyNodes) > 0 {
			nodeSvc := &NodeService{}
			for nodeID := range dirtyNodes {
				if err := nodeSvc.MarkNodeDirtyTx(tx, nodeID); err != nil {
					return err
				}
			}
		}
		return nil
	}); txErr != nil {
		return result, false, txErr
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
			ap := plans[i]
			rt, push, _, perr := inboundSvc.nodePushPlan(ap.inbound)
			if perr != nil {
				return inboundApplyOutcome{err: perr}
			}
			var out inboundApplyOutcome
			if ap.inbound.NodeID == nil {
				if !push {
					out.needRestart = true
				} else if ap.inbound.Protocol == model.MTProto {
					inboundSvc.applyLocalMtproto(ap.inbound.Id)
				} else if ap.inbound.Protocol == model.TUIC {
					inboundSvc.applyLocalTuic(ap.inbound.Id)
				} else {
					addOps := make([]runtime.UserAddOp, 0, len(ap.added))
					for idx := range ap.added {
						c := &ap.added[idx]
						if !c.Enable || !trafficEnableByEmail[c.Email] {
							continue
						}
						addOps = append(addOps, runtime.UserAddOp{
							User:    coreUserFromClient(c, ap.ssMethod),
							Replace: false,
						})
					}
					if localBatchAddUsers(rt, ap.inbound, addOps) {
						out.needRestart = true
					}
				}
				return out
			}
			if push && len(ap.added) <= nodeBulkPushThreshold {
				allOk := true
				for _, c := range ap.added {
					if !c.Enable || !trafficEnableByEmail[c.Email] {
						allOk = false
						continue
					}
					ctx, cancel := nodePushContext()
					err1 := rt.AddClient(ctx, ap.inbound, c)
					cancel()
					if err1 != nil {
						logger.Warning("Error in adding client on", rt.Name(), ":", err1)
						allOk = false
						break
					}
				}
				if allOk {
					advancePushedInbound(rt, ap.prevSettings, ap.newSettings, ap.inbound)
				}
			}
			return out
		})
		for i, out := range outcomes {
			err := out.err
			if panics[i] != nil {
				needRestart = true
				err = panics[i]
			}
			if err != nil {
				recordErr("inbound %d: %v", plans[i].inbound.Id, err)
				continue
			}
			if out.needRestart {
				needRestart = true
			}
			for _, c := range plans[i].added {
				result.Attached = append(result.Attached, c.Email)
			}
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
	// Ordered and de-duplicated up front: the sequential loop dropped each map
	// entry as it went, which the concurrent applies can no longer do.
	detachIds := make([]int, 0, len(recsByInbound))
	detachRecs := make([][]*model.ClientRecord, 0, len(recsByInbound))
	for _, ibId := range inboundIds {
		recs, ok := recsByInbound[ibId]
		if !ok {
			continue
		}
		delete(recsByInbound, ibId)
		detachIds = append(detachIds, ibId)
		detachRecs = append(detachRecs, recs)
	}
	detachResults, detachPanics := fanoutInboundResults(detachIds, inboundFanoutConcurrency, func(i int) inboundApplyOutcome {
		nr, err := s.delInboundClients(inboundSvc, detachIds[i], detachRecs[i], true)
		return inboundApplyOutcome{needRestart: nr, err: err}
	})
	for i, out := range detachResults {
		err := out.err
		if detachPanics[i] != nil {
			// See BulkAttach: a panicking apply may already have committed.
			needRestart = true
			err = detachPanics[i]
		}
		if err != nil {
			recordErr("inbound %d: %v", detachIds[i], err)
			for _, rec := range detachRecs[i] {
				emailFailed[strings.ToLower(rec.Email)] = true
			}
			continue
		}
		if out.needRestart {
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

// inboundApplyOutcome carries one inbound's apply result out of the fanout.
type inboundApplyOutcome struct {
	needRestart bool
	err         error
}

// BulkAdjustResult reports changed clients and per-client skip reasons.
// ExternalLinks contains row counts only for append requests.
type BulkAdjustResult struct {
	Adjusted      int                      `json:"adjusted" example:"2"`
	Skipped       []BulkAdjustReport       `json:"skipped,omitempty" example:"[]"`
	ExternalLinks *BulkExternalLinksResult `json:"externalLinks,omitempty"`
}

type BulkAdjustReport struct {
	Email  string `json:"email" example:"carol"`
	Reason string `json:"reason" example:"client not found"`
}

type bulkAdjustEntry struct {
	record      *model.ClientRecord
	applyExpiry bool
	newExpiry   int64
	applyTotal  bool
	newTotal    int64
}

// bulkFlowClear is the directive that strips the XTLS flow from every selected
// client. The vision values are the only positive flows xray accepts.
const bulkFlowClear = "none"

// bulkFlowAllowed whitelists the flow directives BulkAdjust accepts. Anything
// outside this set is treated as "" (leave flow untouched) so a malformed or
// hostile value can never be injected into a client's settings. The dropdown in
// ClientBulkAdjustModal.tsx offers the same set ("" / "none" / TLS_FLOW_CONTROL);
// keep the two in sync.
var bulkFlowAllowed = map[string]struct{}{
	"":                        {},
	bulkFlowClear:             {},
	"xtls-rprx-vision":        {},
	"xtls-rprx-vision-udp443": {},
}

// BulkAdjust shifts ExpiryTime by addDays (days) and TotalGB by addBytes
// for every email in the list. Clients whose corresponding field is
// unlimited (0) are skipped — bulk extend should not accidentally
// limit an unlimited client. addDays and addBytes may be negative.
// flow sets the XTLS flow, limitHwid the max registered devices (0 = unlimited)
// and adTag the MTProto sponsor channel; "none" clears flow or adTag.
//
// Like BulkDelete, the work is grouped by inbound so each inbound's
// settings JSON is parsed and written exactly once regardless of how
// many target emails it contains.
func (s *ClientService) BulkAdjust(inboundSvc *InboundService, emails []string, addDays int, addBytes int64, flow string, limitHwid *int, adTag string) (BulkAdjustResult, bool, error) {
	result := BulkAdjustResult{}
	if len(emails) == 0 {
		return result, false, nil
	}
	flow = strings.TrimSpace(flow)
	if _, ok := bulkFlowAllowed[flow]; !ok {
		flow = "" // ignore unknown directives — "" means "leave flow untouched"
	}
	adTag = strings.TrimSpace(adTag)
	if adTag != "" && adTag != bulkFlowClear && !model.ValidMtprotoAdTag(adTag) {
		return result, false, common.NewError("mtproto client ad tag must be 32 hex characters")
	}
	if limitHwid != nil && *limitHwid < 0 {
		zero := 0
		limitHwid = &zero
	}
	adjustFlow := flow != ""
	adjustHwid := limitHwid != nil
	adjustAdTag := adTag != ""
	if addDays == 0 && addBytes == 0 && !adjustFlow && !adjustHwid && !adjustAdTag {
		return result, false, common.NewError("no adjustment specified")
	}

	addExpiryMs := int64(addDays) * 24 * 60 * 60 * 1000

	cleanEmails := trimmedUniqueEmails(emails)
	if len(cleanEmails) == 0 {
		return result, false, nil
	}
	defer watchClientChanges(clientChangeSourcePanel, cleanEmails...).record()

	db := database.GetDB()

	recordsByEmail, err := clientRecordsByEmail(db, cleanEmails)
	if err != nil {
		return result, false, err
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
				next := rec.TotalGB + addBytes
				if next <= 0 {
					if _, exists := skippedReasons[email]; !exists {
						skippedReasons[email] = "reduction exceeds remaining quota"
					}
				} else {
					entry.applyTotal = true
					entry.newTotal = next
				}
			}
		}
		if entry.applyExpiry || entry.applyTotal || adjustFlow || adjustHwid || adjustAdTag {
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
		if entry.applyExpiry || entry.applyTotal || adjustFlow || adjustAdTag {
			plannedIds = append(plannedIds, entry.record.Id)
			recordIdToEmail[entry.record.Id] = email
		}
	}

	var mappings []model.ClientInbound
	for _, batch := range chunkInts(plannedIds, sqlInChunk) {
		var rows []model.ClientInbound
		if err := db.Where("client_id IN ?", batch).Find(&rows).Error; err != nil {
			return result, false, err
		}
		mappings = append(mappings, rows...)
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
	flowHonored := map[string]bool{}
	flowIneligible := map[string]bool{}
	adTagHonored := map[string]bool{}
	adTagIneligible := map[string]bool{}
	execFailed := map[string]bool{}
	adjustIds := sortedInboundIds(emailsByInbound)
	adjustResults, adjustPanics := fanoutInboundResults(adjustIds, inboundFanoutConcurrency, func(i int) bulkInboundAdjustResult {
		return s.bulkAdjustInboundClients(inboundSvc, adjustIds[i], emailsByInbound[adjustIds[i]], plan, flow, adTag)
	})
	for i, ibRes := range adjustResults {
		if adjustPanics[i] != nil {
			needRestart = true
			for _, email := range emailsByInbound[adjustIds[i]] {
				execFailed[email] = true
				if _, already := skippedReasons[email]; !already {
					skippedReasons[email] = adjustPanics[i].Error()
				}
			}
			continue
		}
		if ibRes.needRestart {
			needRestart = true
		}
		for email := range ibRes.flowHonored {
			flowHonored[email] = true
		}
		for email := range ibRes.flowIneligible {
			flowIneligible[email] = true
		}
		for email := range ibRes.adTagHonored {
			adTagHonored[email] = true
		}
		for email := range ibRes.adTagIneligible {
			adTagIneligible[email] = true
		}
		for email, reason := range ibRes.perEmailSkipped {
			execFailed[email] = true
			if _, already := skippedReasons[email]; !already {
				skippedReasons[email] = reason
			}
		}
	}

	cond, condArgs := depletedCond(db)
	candidateEmails := make([]string, 0, len(plan))
	for email, entry := range plan {
		if entry.applyExpiry || entry.applyTotal {
			candidateEmails = append(candidateEmails, email)
		}
	}
	wasDisabledDepleted := map[string]struct{}{}
	for _, batch := range chunkStrings(candidateEmails, sqlInChunk) {
		var rows []string
		if err := db.Model(xray.ClientTraffic{}).
			Where(cond+" AND enable = ? AND email IN ?", append(append([]any{}, condArgs...), false, batch)...).
			Pluck("email", &rows).Error; err != nil {
			return result, needRestart, err
		}
		for _, e := range rows {
			wasDisabledDepleted[e] = struct{}{}
		}
	}

	wantAdTag := ""
	if adjustAdTag && adTag != bulkFlowClear {
		wantAdTag = strings.ToLower(adTag)
	}

	adjusted := map[string]struct{}{}
	for email, entry := range plan {
		if execFailed[email] {
			continue
		}
		updates := map[string]any{}
		if entry.applyExpiry {
			updates["expiry_time"] = entry.newExpiry
		}
		if entry.applyTotal {
			updates["total"] = entry.newTotal
		}
		if len(updates) > 0 {
			if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).Updates(updates).Error; err != nil {
				if _, already := skippedReasons[email]; !already {
					skippedReasons[email] = err.Error()
				}
				continue
			}
		}
		if adjustHwid {
			if err := s.setClientLimitHwidByEmail(email, *limitHwid); err != nil {
				if _, already := skippedReasons[email]; !already {
					skippedReasons[email] = err.Error()
				}
				continue
			}
		}
		if adjustAdTag && adTagHonored[email] {
			if err := db.Model(&model.ClientRecord{}).Where("email = ?", email).UpdateColumn("ad_tag", wantAdTag).Error; err != nil {
				if _, already := skippedReasons[email]; !already {
					skippedReasons[email] = err.Error()
				}
				continue
			}
		}
		// Counted when expiry/total changed, flow was honored, adTag was honored, or limitHwid was adjusted.
		if len(updates) > 0 || flowHonored[email] || adTagHonored[email] || adjustHwid {
			adjusted[email] = struct{}{}
		}
	}
	result.Adjusted = len(adjusted)

	for email, reason := range skippedReasons {
		result.Skipped = append(result.Skipped, BulkAdjustReport{Email: email, Reason: reason})
	}
	// Report a flow directive that no inbound could carry — only when it was not
	// honored anywhere and the client has no other (expiry/total) skip reason.
	// The expiry/total part, if any, has already been applied and counted above.
	for email := range flowIneligible {
		if flowHonored[email] {
			continue
		}
		if _, already := skippedReasons[email]; already {
			continue
		}
		result.Skipped = append(result.Skipped, BulkAdjustReport{Email: email, Reason: "flow not supported on inbound"})
	}
	for email := range adTagIneligible {
		if adTagHonored[email] {
			continue
		}
		if _, already := skippedReasons[email]; already {
			continue
		}
		result.Skipped = append(result.Skipped, BulkAdjustReport{Email: email, Reason: "adTag not supported on inbound"})
	}

	if len(wasDisabledDepleted) > 0 {
		stillDepleted := map[string]struct{}{}
		wasList := make([]string, 0, len(wasDisabledDepleted))
		for e := range wasDisabledDepleted {
			wasList = append(wasList, e)
		}
		for _, batch := range chunkStrings(wasList, sqlInChunk) {
			var rows []string
			if err := db.Model(xray.ClientTraffic{}).
				Where(cond+" AND email IN ?", append(append([]any{}, condArgs...), batch)...).
				Pluck("email", &rows).Error; err != nil {
				return result, needRestart, err
			}
			for _, e := range rows {
				stillDepleted[e] = struct{}{}
			}
		}
		reEnable := make([]string, 0, len(wasDisabledDepleted))
		for e := range wasDisabledDepleted {
			if _, still := stillDepleted[e]; !still {
				reEnable = append(reEnable, e)
			}
		}
		if len(reEnable) > 0 {
			_, nr, err := s.BulkSetEnable(inboundSvc, reEnable, true)
			if err != nil {
				return result, needRestart, err
			}
			if nr {
				needRestart = true
			}
		}
	}

	return result, needRestart, nil
}

type bulkInboundAdjustResult struct {
	perEmailSkipped map[string]string
	flowHonored     map[string]bool
	// flowIneligible is tracked apart from perEmailSkipped: a flow directive
	// that an inbound cannot carry must not suppress the expiry/total write for
	// the same client (which would diverge the inbound JSON / ClientRecord from
	// ClientTraffic). It only feeds the final Skipped report.
	flowIneligible  map[string]bool
	adTagHonored    map[string]bool
	adTagIneligible map[string]bool
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
	flow string,
	adTag string,
) bulkInboundAdjustResult {
	res := bulkInboundAdjustResult{
		perEmailSkipped: map[string]string{},
		flowHonored:     map[string]bool{},
		flowIneligible:  map[string]bool{},
		adTagHonored:    map[string]bool{},
		adTagIneligible: map[string]bool{},
	}

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

	// Match by email — the client's stable identity (see Delete). Credentials
	// can drift from the inbound JSON, so they are never used for matching.
	wantedEmails := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		if plan[email] == nil {
			res.perEmailSkipped[email] = "client not found"
			continue
		}
		wantedEmails[email] = struct{}{}
	}

	// Flow eligibility is a property of the inbound (protocol + transport), so
	// resolve it once. Clearing flow is always allowed; setting a vision flow
	// is only honored on an inbound that can carry it.
	flowEligible := flow == bulkFlowClear ||
		(!oldInbound.DisableFlow &&
			inboundCanEnableTlsFlow(string(oldInbound.Protocol), oldInbound.StreamSettings, oldInbound.Settings))

	wantAdTag := ""
	if adTag != "" && adTag != bulkFlowClear {
		wantAdTag = strings.ToLower(adTag)
	}

	interfaceClients, _ := settings["clients"].([]any)
	foundEmails := map[string]bool{}
	changedEntries := make([]any, 0, len(wantedEmails))
	flowChanged := false
	adTagChanged := false
	hasInboundChanges := false
	nowMs := time.Now().Unix() * 1000
	for i, client := range interfaceClients {
		c, ok := client.(map[string]any)
		if !ok {
			continue
		}
		targetEmail, _ := c["email"].(string)
		if _, want := wantedEmails[targetEmail]; !want || targetEmail == "" {
			continue
		}
		clientChanged := false
		entry := plan[targetEmail]
		if entry.applyExpiry {
			c["expiryTime"] = entry.newExpiry
			clientChanged = true
		}
		if entry.applyTotal {
			c["totalGB"] = entry.newTotal
			clientChanged = true
		}
		if flow != "" {
			if flowEligible {
				want := ""
				if flow != bulkFlowClear {
					want = flow
				}
				if cur, _ := c["flow"].(string); cur != want {
					c["flow"] = want
					flowChanged = true
				}
				res.flowHonored[targetEmail] = true
				clientChanged = true
			} else {
				// Record separately so this never suppresses the expiry/total
				// write for the same client (see flowIneligible doc).
				res.flowIneligible[targetEmail] = true
			}
		}
		if adTag != "" {
			if oldInbound.Protocol == model.MTProto {
				if cur, _ := c["adTag"].(string); cur != wantAdTag {
					c["adTag"] = wantAdTag
					adTagChanged = true
				}
				res.adTagHonored[targetEmail] = true
				clientChanged = true
			} else {
				res.adTagIneligible[targetEmail] = true
			}
		}
		if clientChanged {
			c["updated_at"] = nowMs
			hasInboundChanges = true
			changedEntries = append(changedEntries, c)
		}
		interfaceClients[i] = c
		foundEmails[targetEmail] = true
	}

	for email := range wantedEmails {
		if !foundEmails[email] {
			res.perEmailSkipped[email] = "Client Not Found In Inbound"
		}
	}

	if len(foundEmails) == 0 || !hasInboundChanges {
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
	prevSettings := oldInbound.Settings
	oldInbound.Settings = string(newSettings)

	changedClients, err := settingsEntriesToClients(changedEntries)
	if err != nil {
		for email := range foundEmails {
			res.perEmailSkipped[email] = err.Error()
		}
		return res
	}

	// A flow change rewrites the user's xray config, which the lightweight
	// UpdateUser push below does not carry. Local nodes reload via restart;
	// remote nodes get a full reconcile (MarkNodeDirty) instead of a per-user push.
	if flowChanged && oldInbound.NodeID == nil {
		res.needRestart = true
	}

	// Serialize against the traffic poll to avoid the cross-transaction
	// lock-order deadlock on inbounds/client_records (runSerializedTx).
	txErr := runSerializedTx(func(tx *gorm.DB) error {
		if err := commitInboundClientSettings(tx, oldInbound, prevSettings); err != nil {
			return err
		}
		if err := s.ApplyInboundClientDelta(tx, inboundId, changedClients, nil); err != nil {
			return err
		}
		if oldInbound.NodeID != nil {
			return (&NodeService{}).MarkNodeDirtyTx(tx, *oldInbound.NodeID)
		}
		return nil
	})
	if txErr != nil {
		for email := range foundEmails {
			if _, skip := res.perEmailSkipped[email]; !skip {
				res.perEmailSkipped[email] = txErr.Error()
			}
		}
	} else {
		if adTagChanged && oldInbound.Protocol == model.MTProto && oldInbound.NodeID == nil {
			inboundSvc.applyLocalMtproto(oldInbound.Id)
		}
		if oldInbound.NodeID != nil && !flowChanged && len(foundEmails) <= nodeBulkPushThreshold {
			rt, push, _, perr := inboundSvc.nodePushPlan(oldInbound)
			if perr != nil {
				logger.Warning("BulkAdjust: node runtime lookup after commit failed:", perr)
			} else if push {
				for email := range foundEmails {
					entry := plan[email]
					updated := *entry.record.ToClient()
					if entry.applyExpiry {
						updated.ExpiryTime = entry.newExpiry
					}
					if entry.applyTotal {
						updated.TotalGB = entry.newTotal
					}
					if adTag != "" && oldInbound.Protocol == model.MTProto {
						updated.AdTag = wantAdTag
					}
					updated.UpdatedAt = nowMs
					ctx, cancel := nodePushContext()
					err1 := rt.UpdateUser(ctx, oldInbound, email, updated)
					cancel()
					if err1 != nil {
						logger.Warning("Error in updating client on", rt.Name(), ":", err1)
						// First failure ends the batch push; the reconcile converges the rest.
						break
					}
				}
			}
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

	cleanEmails := trimmedUniqueEmails(emails)
	if len(cleanEmails) == 0 {
		return result, false, nil
	}

	db := database.GetDB()

	recordsByEmail, err := clientRecordsByEmail(db, cleanEmails)
	if err != nil {
		return result, false, err
	}
	tombstoneEmails := make([]string, 0, len(recordsByEmail))
	for _, email := range cleanEmails {
		if recordsByEmail[email] != nil {
			tombstoneEmails = append(tombstoneEmails, email)
		}
	}
	tombstoneClientEmails(tombstoneEmails)

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
		for _, batch := range chunkInts(clientIds, sqlInChunk) {
			var rows []model.ClientInbound
			if err := db.Where("client_id IN ?", batch).Find(&rows).Error; err != nil {
				return result, false, err
			}
			mappings = append(mappings, rows...)
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
	delIds := sortedInboundIds(emailsByInbound)
	delResults, delPanics := fanoutInboundResults(delIds, inboundFanoutConcurrency, func(i int) bulkInboundDeleteResult {
		return s.bulkDelInboundClients(inboundSvc, delIds[i], emailsByInbound[delIds[i]], recordsByEmail, keepTraffic)
	})
	for i, ibResult := range delResults {
		if delPanics[i] != nil {
			needRestart = true
			for _, email := range emailsByInbound[delIds[i]] {
				if _, already := skippedReasons[email]; !already {
					skippedReasons[email] = delPanics[i].Error()
				}
			}
			continue
		}
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
	failedEmails := make([]string, 0, len(recordsByEmail))
	successSubIDs := make([]string, 0, len(recordsByEmail))
	for email, rec := range recordsByEmail {
		if _, skipped := skippedReasons[email]; skipped {
			failedEmails = append(failedEmails, email)
			continue
		}
		successEmails = append(successEmails, email)
		successIds = append(successIds, rec.Id)
		successSubIDs = append(successSubIDs, rec.SubID)
	}
	withdrawClientTombstones(failedEmails...)

	if len(successIds) > 0 {
		// Serialize the row cleanup against the traffic poll to avoid the
		// cross-transaction lock-order deadlock on client_traffics/inbounds.
		if err := runSerializedTx(func(tx *gorm.DB) error {
			if e := adjustGroupBaselinesForRemovedTraffic(tx, successEmails); e != nil {
				return e
			}
			if e := clearClientHwidsBySubIDTx(tx, successSubIDs...); e != nil {
				return e
			}
			for _, batch := range chunkInts(successIds, sqlInChunk) {
				if e := tx.Where("client_id IN ?", batch).Delete(&model.ClientInbound{}).Error; e != nil {
					return e
				}
				if e := tx.Where("client_id IN ?", batch).Delete(&model.ClientExternalLink{}).Error; e != nil {
					return e
				}
			}
			if !keepTraffic && len(successEmails) > 0 {
				for _, batch := range chunkStrings(successEmails, sqlInChunk) {
					if e := tx.Where("email IN ?", batch).Delete(&xray.ClientTraffic{}).Error; e != nil {
						return e
					}
					if e := tx.Where("client_email IN ?", batch).Delete(&model.InboundClientIps{}).Error; e != nil {
						return e
					}
				}
			}
			for _, batch := range chunkInts(successIds, sqlInChunk) {
				if e := tx.Where("id IN ?", batch).Delete(&model.ClientRecord{}).Error; e != nil {
					return e
				}
			}
			return nil
		}); err != nil {
			withdrawClientTombstones(successEmails...)
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

	defer InvalidateSubData()
	defer lockInbound(inboundId).Unlock()

	oldInbound, err := inboundSvc.GetInbound(inboundId)
	if err != nil {
		logger.Error("Load Old Data Error")
		for _, e := range emails {
			res.perEmailSkipped[e] = err.Error()
		}
		return res
	}

	// Match by email — the client's stable identity (see Delete). The link-derived
	// set is deletion intent: an email already absent from settings is successful,
	// while foundEmails tracks entries that still need settings-specific cleanup.
	wantedEmails := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		if records[email] == nil {
			res.perEmailSkipped[email] = "client not found"
			continue
		}
		wantedEmails[email] = struct{}{}
	}

	foundEmails := map[string]bool{}
	enableByEmail := map[string]bool{}
	var newSettingsStr string
	if fastRaw, fastRemoved, ok := removeClientsFast(inboundId, oldInbound.Settings, wantedEmails); ok {
		for _, r := range fastRemoved {
			foundEmails[r.email] = true
			enableByEmail[r.email] = r.enable
		}
		newSettingsStr = fastRaw
	} else {
		var settings map[string]any
		if err := json.Unmarshal([]byte(oldInbound.Settings), &settings); err != nil {
			for _, e := range emails {
				res.perEmailSkipped[e] = err.Error()
			}
			return res
		}
		interfaceClients, _ := settings["clients"].([]any)
		newClients := make([]any, 0, len(interfaceClients))
		for _, client := range interfaceClients {
			c, ok := client.(map[string]any)
			if !ok {
				newClients = append(newClients, client)
				continue
			}
			em, _ := c["email"].(string)
			if _, found := wantedEmails[em]; found && em != "" {
				foundEmails[em] = true
				en, _ := c["enable"].(bool)
				enableByEmail[em] = en
				continue
			}
			newClients = append(newClients, client)
		}
		if newClients == nil {
			newClients = []any{}
		}
		settings["clients"] = newClients
		encoded, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			for email := range wantedEmails {
				if _, skip := res.perEmailSkipped[email]; !skip {
					res.perEmailSkipped[email] = err.Error()
				}
			}
			return res
		}
		newSettingsStr = string(encoded)
	}

	db := database.GetDB()
	prevSettings := oldInbound.Settings
	oldInbound.Settings = newSettingsStr

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
		for _, batch := range chunkStrings(foundList, sqlInChunk) {
			var rows []trafficRow
			if err := db.Model(xray.ClientTraffic{}).
				Where("email IN ?", batch).
				Select("email, enable").
				Scan(&rows).Error; err == nil {
				for _, r := range rows {
					notDepletedByEmail[r.Email] = r.Enable
				}
			}
		}
	}

	var sharedSet map[string]bool
	if !keepTraffic {
		var sharedErr error
		sharedSet, sharedErr = inboundSvc.emailsUsedByOtherInbounds(db, foundList, inboundId)
		if sharedErr != nil {
			for email := range wantedEmails {
				res.perEmailSkipped[email] = sharedErr.Error()
			}
			return res
		}
	}
	if !keepTraffic {
		purge := make([]string, 0, len(foundEmails))
		for email := range foundEmails {
			if !sharedSet[strings.ToLower(strings.TrimSpace(email))] {
				purge = append(purge, email)
			}
		}
		if len(purge) > 0 {
			// Serialize the IP/stat purge against the traffic poll to avoid the
			// cross-transaction lock-order deadlock on client_traffics.
			if delErr := runSerializedTx(func(tx *gorm.DB) error {
				if e := inboundSvc.delClientIPsByEmails(tx, purge); e != nil {
					logger.Error("Error in delete client IPs")
					return e
				}
				if e := inboundSvc.delClientStatsByEmails(tx, purge); e != nil {
					logger.Error("Delete stats Data Error")
					return e
				}
				return nil
			}); delErr != nil {
				for _, email := range purge {
					res.perEmailSkipped[email] = delErr.Error()
					delete(foundEmails, email)
				}
			}
		}
	}

	detachList := make([]string, 0, len(wantedEmails))
	for email := range wantedEmails {
		if _, skip := res.perEmailSkipped[email]; !skip {
			detachList = append(detachList, email)
		}
	}

	// Serialize against the traffic poll to avoid the cross-transaction
	// lock-order deadlock on inbounds/client_records (runSerializedTx).
	txErr := runSerializedTx(func(tx *gorm.DB) error {
		if err := commitInboundClientSettings(tx, oldInbound, prevSettings); err != nil {
			return err
		}
		if err := s.ApplyInboundClientDelta(tx, inboundId, nil, detachList); err != nil {
			return err
		}
		if oldInbound.NodeID != nil {
			return (&NodeService{}).MarkNodeDirtyTx(tx, *oldInbound.NodeID)
		}
		return nil
	})
	if txErr != nil {
		for email := range wantedEmails {
			if _, skip := res.perEmailSkipped[email]; !skip {
				res.perEmailSkipped[email] = txErr.Error()
			}
		}
	} else if oldInbound.NodeID == nil {
		removeEmails := make([]string, 0, len(foundEmails))
		for email := range foundEmails {
			if enableByEmail[email] && notDepletedByEmail[email] {
				removeEmails = append(removeEmails, email)
			}
		}
		if len(removeEmails) > 0 {
			rt, rterr := inboundSvc.runtimeFor(oldInbound)
			if rterr != nil {
				res.needRestart = true
			} else {
				removedAny, nr := localBatchRemoveUsers(rt, oldInbound, removeEmails)
				if nr || (removedAny && droppedClientNeedsRestart()) {
					res.needRestart = true
				}
			}
		}
	} else {
		dispatchEmails := make([]string, 0, len(wantedEmails))
		for email := range wantedEmails {
			if _, skip := res.perEmailSkipped[email]; !skip {
				dispatchEmails = append(dispatchEmails, email)
			}
		}
		if len(dispatchEmails) > nodeBulkPushThreshold {
			return res
		}
		rt, push, _, perr := inboundSvc.nodePushPlan(oldInbound)
		if perr != nil {
			logger.Warning("BulkDelete: node runtime lookup after commit failed:", perr)
		} else if push {
			for _, email := range dispatchEmails {
				ctx, cancel := nodePushContext()
				err1 := rt.DeleteClient(ctx, email)
				cancel()
				if err1 != nil {
					logger.Warning("Error in deleting client on", rt.Name(), ":", err1)
					// The node is already dirty, so one reconcile converges the rest of
					// the batch instead of paying another deadline per client.
					break
				}
			}
		}
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

func (s *ClientService) BulkCreate(inboundSvc *InboundService, payloads []ClientCreatePayload) (BulkCreateResult, bool, error) {
	result, _, needRestart, err := s.bulkCreate(inboundSvc, payloads)
	return result, needRestart, err
}

// bulkCreate also returns the payload indexes that inserted a new client record;
// a Created payload whose email already existed only reused that client.
func (s *ClientService) bulkCreate(inboundSvc *InboundService, payloads []ClientCreatePayload) (BulkCreateResult, []int, bool, error) {
	result := BulkCreateResult{}
	if len(payloads) == 0 {
		return result, nil, false, nil
	}

	skip := func(email, reason string) {
		if strings.TrimSpace(email) == "" {
			email = "(missing email)"
		}
		result.Skipped = append(result.Skipped, BulkCreateReport{Email: email, Reason: reason})
	}

	type prepared struct {
		client     model.Client
		inboundIds []int
		limitHwid  int
		payloadIdx int
	}
	prep := make([]prepared, 0, len(payloads))
	emails := make([]string, 0, len(payloads))
	subIDs := make([]string, 0, len(payloads))
	seenEmail := make(map[string]struct{}, len(payloads))
	seenSubID := make(map[string]string, len(payloads))
	allInboundSet := make(map[int]struct{})
	allInboundIDs := make([]int, 0)

	for i := range payloads {
		client := payloads[i].Client
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
		if verr := validateClientRenewal(client); verr != nil {
			skip(email, verr.Error())
			continue
		}
		if client.TotalGB < 0 {
			skip(email, "totalGB must be >= 0")
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
		if len(payloads[i].InboundIds) == 0 {
			skip(email, "at least one inbound is required")
			continue
		}
		normalizeClientTrafficReset(&client)
		if client.ForwardedPortsSpec() == "" {
			client.ForwardedPorts = nil
		}
		client.ClientHostRuleIds = model.NormalizeClientHostRuleIds(client.ClientHostRuleIds)
		client.Email = email
		if client.SubID == "" {
			client.SubID = uuid.NewString()
		}
		// Preserve enable (omit->true in UnmarshalJSON; explicit false kept) (#6478).
		now := time.Now().UnixMilli()
		if client.CreatedAt == 0 {
			client.CreatedAt = now
		}
		client.UpdatedAt = now

		le := strings.ToLower(email)
		if _, dup := seenEmail[le]; dup {
			skip(email, "email already in use: "+email)
			continue
		}
		if owner, ok := seenSubID[client.SubID]; ok && owner != le {
			skip(email, "subId already in use: "+client.SubID)
			continue
		}
		seenEmail[le] = struct{}{}
		seenSubID[client.SubID] = le

		cleanIDs := uniqueInts(payloads[i].InboundIds)
		for _, ibId := range cleanIDs {
			if _, g := allInboundSet[ibId]; !g {
				allInboundSet[ibId] = struct{}{}
				allInboundIDs = append(allInboundIDs, ibId)
			}
		}

		prep = append(prep, prepared{
			client:     client,
			inboundIds: cleanIDs,
			limitHwid:  payloads[i].LimitHwid,
			payloadIdx: i,
		})
		emails = append(emails, email)
		subIDs = append(subIDs, client.SubID)
	}

	if len(prep) == 0 {
		return result, nil, false, nil
	}

	defer InvalidateSubData()
	unlock := lockInboundsSorted(allInboundIDs)
	defer unlock()

	lowerEmails := make([]string, len(emails))
	for i, em := range emails {
		lowerEmails[i] = strings.ToLower(em)
	}

	type bulkCreatePlan struct {
		inbound      *model.Inbound
		prevSettings string
		newSettings  string
		clients      []model.Client
		prepIdxs     []int
		ssMethod     string
	}
	var (
		plans            []bulkCreatePlan
		committedClients []model.Client
		failed           []bool
		reason           []string
		reused           []bool
	)

	if txErr := runSerializedTx(func(tx *gorm.DB) error {
		const lookupChunk = 400
		txClients := make([]model.Client, len(prep))
		for idx := range prep {
			txClients[idx] = prep[idx].client
		}
		txReused := make([]bool, len(prep))
		txExistingLimitHwid := make([]int, len(prep))
		txFailed := make([]bool, len(prep))
		txReason := make([]string, len(prep))

		existingByEmail := make(map[string]model.ClientRecord, len(lowerEmails))
		for start := 0; start < len(lowerEmails); start += lookupChunk {
			end := min(start+lookupChunk, len(lowerEmails))
			var rows []model.ClientRecord
			if e := tx.Where("LOWER(email) IN ?", lowerEmails[start:end]).Find(&rows).Error; e != nil {
				return e
			}
			for i := range rows {
				existingByEmail[strings.ToLower(rows[i].Email)] = rows[i]
			}
		}
		existingSubOwners := make(map[string]map[string]struct{}, len(subIDs))
		for start := 0; start < len(subIDs); start += lookupChunk {
			end := min(start+lookupChunk, len(subIDs))
			var rows []model.ClientRecord
			if e := tx.Where("sub_id IN ?", subIDs[start:end]).Find(&rows).Error; e != nil {
				return e
			}
			for i := range rows {
				owners := existingSubOwners[rows[i].SubID]
				if owners == nil {
					owners = make(map[string]struct{}, 1)
					existingSubOwners[rows[i].SubID] = owners
				}
				owners[strings.ToLower(rows[i].Email)] = struct{}{}
			}
		}

		for idx := range prep {
			le := lowerEmails[idx]
			if rec, ok := existingByEmail[le]; ok {
				if rec.SubID == "" || rec.SubID != txClients[idx].SubID {
					txFailed[idx] = true
					txReason[idx] = "email already in use: " + txClients[idx].Email
					continue
				}
				txReused[idx] = true
				txExistingLimitHwid[idx] = rec.LimitHwid
				txClients[idx].Email = rec.Email
				if txClients[idx].ID == "" {
					txClients[idx].ID = rec.UUID
				}
				if txClients[idx].Password == "" {
					txClients[idx].Password = rec.Password
				}
				if txClients[idx].Auth == "" {
					txClients[idx].Auth = rec.Auth
				}
				if txClients[idx].Secret == "" {
					txClients[idx].Secret = rec.Secret
				}
			}
			if owners, ok := existingSubOwners[txClients[idx].SubID]; ok {
				if _, sameOwner := owners[le]; !sameOwner || len(owners) > 1 {
					txFailed[idx] = true
					txReason[idx] = "subId already in use: " + txClients[idx].SubID
					continue
				}
			}
		}

		inboundsByID, idxByID, loadErr := loadInboundsForClientWrite(tx, allInboundIDs)
		if loadErr != nil {
			return loadErr
		}

		nowMs := time.Now().UnixMilli()
		var nilPortCtx portConflictContext
		hasNilPortCtx := false
		portCtxCache := make(map[int]portConflictContext)

		docCache := make(map[int]*rawSettingsDoc)
		existingInDocCache := make(map[int]map[string]struct{})
		tunnelClientsCache := make(map[int][]model.Client)
		type batchTunnelAlloc struct {
			owner string
			label string
			addrs []string
		}
		batchTunnelAllocs := make(map[int][]batchTunnelAlloc)

		loadDocInbound := func(ib *model.Inbound) (*rawSettingsDoc, map[string]struct{}, error) {
			if doc, ok := docCache[ib.Id]; ok {
				return doc, existingInDocCache[ib.Id], nil
			}
			doc, dErr := parseRawSettingsDoc(ib.Settings)
			if dErr != nil {
				return nil, nil, dErr
			}
			existingInDoc := make(map[string]struct{}, len(doc.rawClients))
			isTunnel := ib.Protocol == model.WireGuard || ib.Protocol == model.AmneziaWG
			if isTunnel {
				existingClients, gcErr := inboundSvc.GetClients(ib)
				if gcErr != nil {
					return nil, nil, gcErr
				}
				tunnelClientsCache[ib.Id] = existingClients
				for _, ec := range existingClients {
					if ec.Email != "" {
						existingInDoc[strings.ToLower(ec.Email)] = struct{}{}
					}
				}
			} else {
				for i := range doc.rawClients {
					if em := doc.clientHeaderAt(i).Email; em != "" {
						existingInDoc[strings.ToLower(em)] = struct{}{}
					}
				}
			}
			docCache[ib.Id] = doc
			existingInDocCache[ib.Id] = existingInDoc
			return doc, existingInDoc, nil
		}

		type stagedInboundItem struct {
			ibId     int
			client   model.Client
			isSplice bool
			enc      []byte
			sp       clientSpan
			wm       map[string]any
		}

		byInbound := make(map[int][]model.Client)
		idxByInbound := make(map[int][]int)
		spliceEncodedByIb := make(map[int][][]byte)
		spliceSpansByIb := make(map[int][]clientSpan)
		wireMapsByIb := make(map[int][]map[string]any)
		inboundOrder := make([]int, 0, len(allInboundIDs))

		for idx := range prep {
			if txFailed[idx] {
				continue
			}
			candidate := txClients[idx]
			le := strings.ToLower(candidate.Email)

			failClient := func(err error) {
				txFailed[idx] = true
				txReason[idx] = err.Error()
			}

			validInbounds := true
			for _, ibId := range prep[idx].inboundIds {
				ib := inboundsByID[ibId]
				if ib == nil {
					failClient(gorm.ErrRecordNotFound)
					validInbounds = false
					break
				}
				if err := s.fillProtocolDefaultsWithIdx(&candidate, ib, idxByID[ibId]); err != nil {
					failClient(err)
					validInbounds = false
					break
				}
			}
			if !validInbounds {
				continue
			}

			staged := make([]stagedInboundItem, 0, len(prep[idx].inboundIds))
			clientOK := true
			for _, ibId := range prep[idx].inboundIds {
				ib := inboundsByID[ibId]
				isTunnel := ib.Protocol == model.WireGuard || ib.Protocol == model.AmneziaWG
				fIdx := idxByID[ibId]

				if !isTunnel && fIdx != nil && fIdx.canSplice() {
					if fIdx.hasFoldEmail(candidate.Email) {
						if txReused[idx] {
							continue
						}
						failClient(common.NewError("Duplicate email:", candidate.Email))
						clientOK = false
						break
					}
					c := clientWithInboundFlowIdx(candidate, ib, fIdx)
					if err := validateTuicClientEmails(ib.Protocol, []model.Client{c}); err != nil {
						failClient(err)
						clientOK = false
						break
					}
					if err := validateClientRequiredCredentials(ib.Protocol, c); err != nil {
						failClient(err)
						clientOK = false
						break
					}
					ssMethod := ""
					if ib.Protocol == model.Shadowsocks {
						ssMethod = fIdx.topMethod
					}
					enc, sp, mErr := marshalClientWire(c, ssMethod, nowMs)
					if mErr != nil {
						failClient(mErr)
						clientOK = false
						break
					}
					staged = append(staged, stagedInboundItem{
						ibId:     ibId,
						client:   c,
						isSplice: true,
						enc:      enc,
						sp:       sp,
					})
					continue
				}

				_, existingInDoc, docErr := loadDocInbound(ib)
				if docErr != nil {
					failClient(docErr)
					clientOK = false
					break
				}
				if _, exists := existingInDoc[le]; exists {
					if txReused[idx] {
						continue
					}
					failClient(common.NewError("Duplicate email:", candidate.Email))
					clientOK = false
					break
				}
				c := clientWithInboundFlow(candidate, ib)
				c.WireguardDevices = model.CloneWireguardDevices(c.WireguardDevices)
				if ips, hasIPs := candidate.AllowedIPsByInbound[ibId]; hasIPs {
					c.AllowedIPs = ips
				} else if !addressesFitAmneziaWGInbound(c.AllowedIPs, ib) {
					c.AllowedIPs = nil
				}
				if err := validateTuicClientEmails(ib.Protocol, []model.Client{c}); err != nil {
					failClient(err)
					clientOK = false
					break
				}

				b, mErr := json.Marshal(c)
				if mErr != nil {
					failClient(mErr)
					clientOK = false
					break
				}
				var wm map[string]any
				if uErr := json.Unmarshal(b, &wm); uErr != nil {
					failClient(uErr)
					clientOK = false
					break
				}
				if c.CreatedAt > 0 {
					wm["created_at"] = c.CreatedAt
				} else {
					wm["created_at"] = nowMs
				}
				wm["updated_at"] = c.UpdatedAt
				if strings.TrimSpace(c.SubID) == "" {
					wm["subId"] = random.NumLower(16)
				}
				interfaceClients := []any{wm}

				finalClient := c
				if isTunnel {
					existingClients := tunnelClientsCache[ibId]
					if kErr := validateClientsKeepAliveAgainstExisting(ib.Protocol, []model.Client{finalClient}, existingClients); kErr != nil {
						failClient(kErr)
						clientOK = false
						break
					}
					selfEmails := map[string]struct{}{le: {}}
					crossUsed, cErr := s.otherTunnelAllowedIPs(tx, inboundSvc, ib.Id, selfEmails)
					if cErr != nil {
						failClient(cErr)
						clientOK = false
						break
					}
					for otherIbID, allocs := range batchTunnelAllocs {
						if otherIbID == ibId {
							continue
						}
						for _, alloc := range allocs {
							if alloc.owner == le {
								continue
							}
							for _, addr := range alloc.addrs {
								crossUsed[addr] = alloc.label
							}
						}
					}
					if ib.Protocol == model.WireGuard {
						if dErr := defaultWireguardClients(ib.Settings, existingClients, []model.Client{finalClient}, interfaceClients, crossUsed); dErr != nil {
							failClient(dErr)
							clientOK = false
							break
						}
					} else {
						if dErr := defaultAmneziaWGClients(ib.Settings, existingClients, []model.Client{finalClient}, interfaceClients, crossUsed); dErr != nil {
							failClient(dErr)
							clientOK = false
							break
						}
					}
					stamped, sErr := settingsEntriesToClients(interfaceClients)
					if sErr != nil {
						failClient(sErr)
						clientOK = false
						break
					}
					finalClient = stamped[0]
					crossAddrs := make([]string, 0, len(crossUsed))
					for addr := range crossUsed {
						crossAddrs = append(crossAddrs, addr)
					}
					if entry, taken := wireguardAllowedIPsOverlap(model.WireguardAllowedIPs(finalClient), crossAddrs); taken != "" {
						failClient(common.NewError("allowedIPs entry", entry, "overlaps", taken, "used by a client on", crossUsed[taken]))
						clientOK = false
						break
					}
				}
				if err := validateClientRequiredCredentials(ib.Protocol, finalClient); err != nil {
					failClient(err)
					clientOK = false
					break
				}
				if err := validateAmneziaWGClientWrite(ib, []model.Client{finalClient}, ""); err != nil {
					failClient(err)
					clientOK = false
					break
				}
				if ib.Protocol == model.AmneziaWG {
					var pCtx portConflictContext
					if ib.NodeID == nil {
						if !hasNilPortCtx {
							var pErr error
							nilPortCtx, pErr = inboundSvc.loadPortConflictContext(tx, nil)
							if pErr != nil {
								failClient(pErr)
								clientOK = false
								break
							}
							hasNilPortCtx = true
						}
						pCtx = nilPortCtx
					} else {
						var ok bool
						pCtx, ok = portCtxCache[*ib.NodeID]
						if !ok {
							var pErr error
							pCtx, pErr = inboundSvc.loadPortConflictContext(tx, ib.NodeID)
							if pErr != nil {
								failClient(pErr)
								clientOK = false
								break
							}
							portCtxCache[*ib.NodeID] = pCtx
						}
					}
					if hit := inboundSvc.checkForwardedPortsConflict(pCtx, finalClient.ForwardedPortsSpec()); hit != "" {
						failClient(common.NewError("amneziawg: forwardedPorts collides with", hit))
						clientOK = false
						break
					}
				}

				staged = append(staged, stagedInboundItem{
					ibId:   ibId,
					client: finalClient,
					wm:     wm,
				})
			}

			if !clientOK {
				continue
			}

			txClients[idx] = candidate
			for _, st := range staged {
				if _, seen := byInbound[st.ibId]; !seen {
					inboundOrder = append(inboundOrder, st.ibId)
				}
				byInbound[st.ibId] = append(byInbound[st.ibId], st.client)
				idxByInbound[st.ibId] = append(idxByInbound[st.ibId], idx)
				if st.isSplice {
					spliceEncodedByIb[st.ibId] = append(spliceEncodedByIb[st.ibId], st.enc)
					spliceSpansByIb[st.ibId] = append(spliceSpansByIb[st.ibId], st.sp)
				} else {
					wireMapsByIb[st.ibId] = append(wireMapsByIb[st.ibId], st.wm)
					if existingInDocCache[st.ibId] != nil {
						existingInDocCache[st.ibId][le] = struct{}{}
					}
					ib := inboundsByID[st.ibId]
					if ib.Protocol == model.WireGuard || ib.Protocol == model.AmneziaWG {
						tunnelClientsCache[st.ibId] = append(tunnelClientsCache[st.ibId], st.client)
						name := ib.Remark
						if name == "" {
							name = ib.Tag
						}
						batchTunnelAllocs[st.ibId] = append(batchTunnelAllocs[st.ibId], batchTunnelAlloc{
							owner: le,
							label: fmt.Sprintf("inbound '%s' (#%d)", name, ib.Id),
							addrs: model.WireguardAllowedIPs(st.client),
						})
					}
				}
			}
		}

		statAdded := make(map[string]bool, len(prep))
		txPlans := make([]bulkCreatePlan, 0, len(inboundOrder))
		settingsUpdates := make([]inboundSettingsUpdate, 0, len(inboundOrder))
		deltas := make([]inboundClientDelta, 0, len(inboundOrder))

		for _, ibId := range inboundOrder {
			ib := inboundsByID[ibId]
			batchClients := byInbound[ibId]
			batchIdxs := idxByInbound[ibId]
			if len(batchClients) == 0 {
				continue
			}

			isTunnel := ib.Protocol == model.WireGuard || ib.Protocol == model.AmneziaWG
			fIdx := idxByID[ibId]
			if !isTunnel && fIdx != nil && fIdx.canSplice() {
				ssMethod := ""
				if ib.Protocol == model.Shadowsocks {
					ssMethod = fIdx.topMethod
				}
				newSettings, newIdx := fIdx.spliceAppend(spliceEncodedByIb[ibId], spliceSpansByIb[ibId])
				prevSettings := ib.Settings
				ibCopy := *ib
				ibCopy.Settings = newSettings
				for i := range batchClients {
					if statAdded[batchClients[i].Email] {
						continue
					}
					if e := inboundSvc.AddClientStat(tx, ib.Id, &batchClients[i]); e != nil {
						return e
					}
					statAdded[batchClients[i].Email] = true
				}
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
					changed:   batchClients,
				})
				txPlans = append(txPlans, bulkCreatePlan{
					inbound:      &ibCopy,
					prevSettings: prevSettings,
					newSettings:  newSettings,
					clients:      batchClients,
					prepIdxs:     batchIdxs,
					ssMethod:     ssMethod,
				})
				continue
			}

			doc := docCache[ibId]
			wireMaps := wireMapsByIb[ibId]
			ssMethod := ""
			if ib.Protocol == model.Shadowsocks {
				ssMethod = doc.topString("method")
				applyShadowsocksMethodToClientMaps(wireMaps, ssMethod)
			}
			if err := doc.appendMaps(wireMaps); err != nil {
				return err
			}
			newSettings, eErr := doc.encodeCompact()
			if eErr != nil {
				return eErr
			}
			prevSettings := ib.Settings
			ibCopy := *ib
			ibCopy.Settings = newSettings

			for i := range batchClients {
				if statAdded[batchClients[i].Email] {
					continue
				}
				if e := inboundSvc.AddClientStat(tx, ib.Id, &batchClients[i]); e != nil {
					return e
				}
				statAdded[batchClients[i].Email] = true
			}
			settingsUpdates = append(settingsUpdates, inboundSettingsUpdate{
				inboundId:   ib.Id,
				newSettings: newSettings,
			})
			deltas = append(deltas, inboundClientDelta{
				inboundId: ib.Id,
				protocol:  ib.Protocol,
				nodeID:    ib.NodeID,
				hasMeta:   true,
				changed:   batchClients,
			})
			txPlans = append(txPlans, bulkCreatePlan{
				inbound:      &ibCopy,
				prevSettings: prevSettings,
				newSettings:  newSettings,
				clients:      batchClients,
				prepIdxs:     batchIdxs,
				ssMethod:     ssMethod,
			})
		}

		if e := commitInboundsSettingsBatch(tx, settingsUpdates); e != nil {
			return e
		}
		if e := s.ApplyMultiInboundClientDelta(tx, deltas); e != nil {
			return e
		}

		dirtyNodes := make(map[int]struct{})
		for _, wp := range txPlans {
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

		for idx := range prep {
			if txFailed[idx] {
				continue
			}
			if prep[idx].client.ClientHostRuleIds != nil {
				if err := tx.Model(&model.ClientRecord{}).
					Where("email = ?", txClients[idx].Email).
					Update("client_host_rule_ids", model.EncodeClientHostRuleIds(prep[idx].client.ClientHostRuleIds)).Error; err != nil {
					return err
				}
			}
		}

		// Savepoint each HWID trim so one failure rolls back only that client's limit.
		hwidN := 0
		for idx := range prep {
			if txFailed[idx] {
				continue
			}
			if prep[idx].limitHwid > 0 || (txReused[idx] && txExistingLimitHwid[idx] != prep[idx].limitHwid) {
				hwidN++
				spName := fmt.Sprintf("bulk_create_hwid_%d", hwidN)
				if spErr := tx.SavePoint(spName).Error; spErr != nil {
					return spErr
				}
				if err := s.setClientLimitHwidByEmailTx(tx, txClients[idx].Email, prep[idx].limitHwid); err != nil {
					_ = tx.RollbackTo(spName).Error
					txFailed[idx] = true
					txReason[idx] = err.Error()
					continue
				}
			}
		}

		plans = txPlans
		committedClients = txClients
		failed = txFailed
		reason = txReason
		reused = txReused
		return nil
	}); txErr != nil {
		return result, nil, false, txErr
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
		createResults, createPanics := fanoutInboundResults(planIDs, runtimeFanoutLimit(hasRemote), func(i int) inboundApplyOutcome {
			wp := plans[i]
			rt, push, _, perr := inboundSvc.nodePushPlan(wp.inbound)
			if perr != nil {
				return inboundApplyOutcome{err: perr}
			}
			var out inboundApplyOutcome
			if wp.inbound.NodeID == nil {
				if !push {
					out.needRestart = true
					return out
				}
				switch wp.inbound.Protocol {
				case model.MTProto:
					inboundSvc.applyLocalMtproto(wp.inbound.Id)
				case model.AmneziaWG:
					inboundSvc.applyLocalAmneziaWG(wp.inbound.Id)
				case model.TUIC:
					inboundSvc.applyLocalTuic(wp.inbound.Id)
				default:
					batchOps := make([]runtime.UserAddOp, 0, len(wp.clients))
					for idx := range wp.clients {
						c := &wp.clients[idx]
						if len(c.Email) == 0 {
							out.needRestart = true
							continue
						}
						if !c.Enable {
							continue
						}
						batchOps = append(batchOps, runtime.UserAddOp{
							User:    coreUserFromClient(c, wp.ssMethod),
							Replace: false,
						})
					}
					if localBatchAddUsers(rt, wp.inbound, batchOps) {
						out.needRestart = true
					}
				}
				return out
			}
			if push && len(wp.clients) <= nodeBulkPushThreshold {
				allOk := true
				for _, c := range wp.clients {
					if !c.Enable {
						allOk = false
						continue
					}
					ctx, cancel := nodePushContext()
					err1 := rt.AddClient(ctx, wp.inbound, c)
					cancel()
					if err1 != nil {
						logger.Warning("Error in adding client on", rt.Name(), ":", err1)
						allOk = false
						break
					}
				}
				if allOk {
					advancePushedInbound(rt, wp.prevSettings, wp.newSettings, wp.inbound)
				}
			}
			return out
		})
		for i, out := range createResults {
			e := out.err
			if createPanics[i] != nil {
				needRestart = true
				e = createPanics[i]
			}
			if e != nil {
				for _, idx := range plans[i].prepIdxs {
					failed[idx] = true
					if reason[idx] == "" {
						reason[idx] = e.Error()
					}
				}
				continue
			}
			if out.needRestart {
				needRestart = true
			}
		}
	}

	inserted := make([]int, 0, len(prep))
	for idx := range prep {
		if failed[idx] {
			skip(committedClients[idx].Email, reason[idx])
			continue
		}
		result.Created++
		if !reused[idx] {
			inserted = append(inserted, prep[idx].payloadIdx)
		}
	}
	return result, inserted, needRestart, nil
}

func (s *ClientService) DelDepleted(inboundSvc *InboundService) (int, bool, error) {
	db := database.GetDB()
	now := time.Now().UnixMilli()
	depletedClause := depletedClientsClause

	var rows []xray.ClientTraffic
	if err := db.Where(depletedClause, now).Find(&rows).Error; err != nil {
		return 0, false, err
	}
	if len(rows) == 0 {
		return 0, false, nil
	}

	seen := make(map[string]struct{}, len(rows))
	emails := make([]string, 0, len(rows))
	// BulkDelete trims each email, so a padded usage row would resolve to the client it resembles.
	for _, r := range rows {
		if r.Email == "" || r.Email != strings.TrimSpace(r.Email) {
			continue
		}
		if _, ok := seen[r.Email]; ok {
			continue
		}
		seen[r.Email] = struct{}{}
		emails = append(emails, r.Email)
	}
	if len(emails) == 0 {
		return 0, false, nil
	}

	res, needRestart, err := s.BulkDelete(inboundSvc, emails, false)
	if err != nil {
		return res.Deleted, needRestart, err
	}
	return res.Deleted, needRestart, nil
}

type BulkSetEnableResult struct {
	Changed int                   `json:"changed"`
	Skipped []BulkSetEnableReport `json:"skipped,omitempty"`
}

type BulkSetEnableReport struct {
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

func (s *ClientService) BulkSetEnable(inboundSvc *InboundService, emails []string, enable bool) (BulkSetEnableResult, bool, error) {
	result := BulkSetEnableResult{}

	cleanEmails := trimmedUniqueEmails(emails)
	if len(cleanEmails) == 0 {
		return result, false, nil
	}
	defer watchClientChanges(clientChangeSourcePanel, cleanEmails...).record()

	db := database.GetDB()

	recordsByEmail, err := clientRecordsByEmail(db, cleanEmails)
	if err != nil {
		return result, false, err
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
		for _, batch := range chunkInts(clientIds, sqlInChunk) {
			var rows []model.ClientInbound
			if err := db.Where("client_id IN ?", batch).Find(&rows).Error; err != nil {
				return result, false, err
			}
			mappings = append(mappings, rows...)
		}
		for _, m := range mappings {
			email, ok := recordIdToEmail[m.ClientId]
			if !ok {
				continue
			}
			emailsByInbound[m.InboundId] = append(emailsByInbound[m.InboundId], email)
		}
	}

	enableIds := sortedInboundIds(emailsByInbound)
	defer InvalidateSubData()
	unlock := lockInboundsSorted(enableIds)
	defer unlock()

	type changedClient struct {
		email     string
		wasEnable bool
		client    model.Client
	}
	type enableInboundPlan struct {
		inbound      *model.Inbound
		prevSettings string
		newSettings  string
		cipher       string
		changed      []changedClient
	}

	var enablePlans []enableInboundPlan
	var successEmails []string
	nowMs := time.Now().UnixMilli()

	if txErr := runSerializedTx(func(tx *gorm.DB) error {
		var (
			inboundsByID map[int]*model.Inbound
			idxMap       map[int]*fastSettingsIndex
		)
		if len(enableIds) > 0 {
			var lErr error
			inboundsByID, idxMap, lErr = loadInboundsForClientWrite(tx, enableIds)
			if lErr != nil {
				return lErr
			}
		}

		dirtyNodes := make(map[int]struct{})
		enablePlans = make([]enableInboundPlan, 0, len(enableIds))
		settingsUpdates := make([]inboundSettingsUpdate, 0, len(enableIds))
		deltas := make([]inboundClientDelta, 0, len(enableIds))

		for _, ibId := range enableIds {
			ibEmails := emailsByInbound[ibId]
			oldInbound, ok := inboundsByID[ibId]
			if !ok || oldInbound == nil {
				for _, e := range ibEmails {
					if _, already := skippedReasons[e]; !already {
						skippedReasons[e] = gorm.ErrRecordNotFound.Error()
					}
				}
				continue
			}

			var (
				cipher        string
				changed       []changedClient
				changedModels []model.Client
				newSettings   string
				newIdx        *fastSettingsIndex
			)

			idx := idxMap[ibId]
			if idx != nil && idx.canSplice() {
				if oldInbound.Protocol == model.Shadowsocks {
					cipher = idx.topMethod
				}
				seenInIb := make(map[string]struct{}, len(ibEmails))
				replacements := make([]clientReplacement, 0, len(ibEmails))
				for _, email := range ibEmails {
					if email == "" {
						if _, already := skippedReasons[email]; !already {
							skippedReasons[email] = "Client Not Found In Inbound"
						}
						continue
					}
					if _, dup := seenInIb[email]; dup {
						continue
					}
					seenInIb[email] = struct{}{}
					positions := idx.findExactPositions(email)
					if len(positions) == 0 {
						if _, already := skippedReasons[email]; !already {
							skippedReasons[email] = "Client Not Found In Inbound"
						}
						continue
					}
					for _, pos := range positions {
						sp := idx.spans[pos]
						if sp.enable == enable {
							continue
						}
						encoded, _, newClient, newSpan, patched, pErr := patchClientSlice(idx.raw[sp.start:sp.end], nowMs, func(entry map[string]any) (bool, error) {
							entry["enable"] = enable
							return true, nil
						})
						if pErr != nil {
							if _, already := skippedReasons[email]; !already {
								skippedReasons[email] = pErr.Error()
							}
							continue
						}
						if !patched {
							continue
						}
						newClient.Email = email
						newClient.Enable = enable
						replacements = append(replacements, clientReplacement{
							pos:     pos,
							encoded: encoded,
							span:    newSpan,
						})
						changed = append(changed, changedClient{email: email, wasEnable: sp.enable, client: newClient})
						changedModels = append(changedModels, newClient)
					}
				}
				if len(changed) == 0 {
					continue
				}
				var spliced bool
				newSettings, newIdx, spliced = idx.spliceReplaceMany(replacements, false)
				if !spliced {
					for _, ch := range changed {
						if _, already := skippedReasons[ch.email]; !already {
							skippedReasons[ch.email] = "failed to update inbound settings"
						}
					}
					continue
				}
			} else {
				doc, dErr := parseRawSettingsDoc(oldInbound.Settings)
				if dErr != nil {
					for _, e := range ibEmails {
						if _, already := skippedReasons[e]; !already {
							skippedReasons[e] = dErr.Error()
						}
					}
					continue
				}
				if !doc.hasClients {
					for _, e := range ibEmails {
						if _, already := skippedReasons[e]; !already {
							skippedReasons[e] = "invalid clients format in inbound settings"
						}
					}
					continue
				}

				wanted := make(map[string]struct{}, len(ibEmails))
				for _, email := range ibEmails {
					wanted[email] = struct{}{}
				}
				if oldInbound.Protocol == model.Shadowsocks {
					cipher = doc.topString("method")
				}

				found := make(map[string]bool, len(wanted))
				for i := range doc.rawClients {
					hdr := doc.clientHeaderAt(i)
					email := hdr.Email
					if _, want := wanted[email]; !want || email == "" {
						continue
					}
					found[email] = true
					prev := hdr.Enable
					if prev == enable {
						continue
					}
					entry, mErr := doc.clientMapAt(i)
					if mErr != nil {
						if _, already := skippedReasons[email]; !already {
							skippedReasons[email] = mErr.Error()
						}
						continue
					}
					entry["enable"] = enable
					entry["updated_at"] = nowMs
					if rErr := doc.replaceAt(i, entry); rErr != nil {
						if _, already := skippedReasons[email]; !already {
							skippedReasons[email] = rErr.Error()
						}
						continue
					}
					var parsed model.Client
					_ = json.Unmarshal(doc.rawClients[i], &parsed)
					parsed.Email = email
					parsed.Enable = enable
					changed = append(changed, changedClient{email: email, wasEnable: prev, client: parsed})
					changedModels = append(changedModels, parsed)
				}

				for email := range wanted {
					if !found[email] {
						if _, already := skippedReasons[email]; !already {
							skippedReasons[email] = "Client Not Found In Inbound"
						}
					}
				}

				if len(changed) == 0 {
					continue
				}

				var eErr error
				newSettings, eErr = doc.encodeCompact()
				if eErr != nil {
					for _, ch := range changed {
						if _, already := skippedReasons[ch.email]; !already {
							skippedReasons[ch.email] = eErr.Error()
						}
					}
					continue
				}
			}

			settingsUpdates = append(settingsUpdates, inboundSettingsUpdate{
				inboundId:   oldInbound.Id,
				newSettings: newSettings,
				newIdx:      newIdx,
			})
			deltas = append(deltas, inboundClientDelta{
				inboundId: oldInbound.Id,
				protocol:  oldInbound.Protocol,
				nodeID:    oldInbound.NodeID,
				hasMeta:   true,
				changed:   changedModels,
			})
			if oldInbound.NodeID != nil {
				dirtyNodes[*oldInbound.NodeID] = struct{}{}
			}

			ibCopy := *oldInbound
			ibCopy.Settings = newSettings
			enablePlans = append(enablePlans, enableInboundPlan{
				inbound:      &ibCopy,
				prevSettings: oldInbound.Settings,
				newSettings:  newSettings,
				cipher:       cipher,
				changed:      changed,
			})
		}

		if e := commitInboundsSettingsBatch(tx, settingsUpdates); e != nil {
			return e
		}
		if e := s.ApplyMultiInboundClientDelta(tx, deltas); e != nil {
			return e
		}

		if len(dirtyNodes) > 0 {
			nodeSvc := &NodeService{}
			for nodeID := range dirtyNodes {
				if err := nodeSvc.MarkNodeDirtyTx(tx, nodeID); err != nil {
					return err
				}
			}
		}

		successEmails = make([]string, 0, len(recordsByEmail))
		for email := range recordsByEmail {
			if _, skipped := skippedReasons[email]; skipped {
				continue
			}
			successEmails = append(successEmails, email)
		}

		if len(successEmails) > 0 {
			for _, batch := range chunkStrings(successEmails, sqlInChunk) {
				if e := tx.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Update("enable", enable).Error; e != nil {
					return e
				}
				if e := tx.Model(&model.ClientRecord{}).Where("email IN ?", batch).
					Updates(map[string]any{"enable": enable, "updated_at": nowMs}).Error; e != nil {
					return e
				}
			}
		}
		return nil
	}); txErr != nil {
		return result, false, txErr
	}
	unlock()

	needRestart := false
	if len(enablePlans) > 0 {
		wantRestartOnDrop := false
		if !enable {
			for _, ep := range enablePlans {
				if ep.inbound.NodeID == nil && !servedOutsideXray(ep.inbound.Protocol) {
					wantRestartOnDrop = droppedClientNeedsRestart()
					break
				}
			}
		}

		hasRemote := false
		planIDs := make([]int, len(enablePlans))
		for i := range enablePlans {
			planIDs[i] = enablePlans[i].inbound.Id
			if enablePlans[i].inbound.NodeID != nil {
				hasRemote = true
			}
		}
		outcomes, panics := fanoutInboundResults(planIDs, runtimeFanoutLimit(hasRemote), func(i int) inboundApplyOutcome {
			ep := enablePlans[i]
			rt, push, _, perr := inboundSvc.nodePushPlan(ep.inbound)
			if perr != nil {
				return inboundApplyOutcome{err: perr}
			}
			if ep.inbound.NodeID != nil && push && len(ep.changed) > nodeBulkPushThreshold {
				push = false
			}

			var out inboundApplyOutcome
			if ep.inbound.NodeID == nil {
				if !push {
					out.needRestart = true
					return out
				}
				switch ep.inbound.Protocol {
				case model.MTProto:
					inboundSvc.applyLocalMtproto(ep.inbound.Id)
				case model.AmneziaWG:
					inboundSvc.applyLocalAmneziaWG(ep.inbound.Id)
				case model.TUIC:
					inboundSvc.applyLocalTuic(ep.inbound.Id)
				default:
					if enable {
						ops := make([]runtime.UserAddOp, len(ep.changed))
						for idx := range ep.changed {
							ops[idx] = runtime.UserAddOp{
								User:    coreUserFromClient(&ep.changed[idx].client, ep.cipher),
								Replace: true,
							}
						}
						if localBatchAddUsers(rt, ep.inbound, ops) {
							out.needRestart = true
						}
					} else {
						removeEmails := make([]string, 0, len(ep.changed))
						for _, ch := range ep.changed {
							if ch.wasEnable {
								removeEmails = append(removeEmails, ch.email)
							}
						}
						removedAny, nr := localBatchRemoveUsers(rt, ep.inbound, removeEmails)
						if nr || (removedAny && wantRestartOnDrop) {
							out.needRestart = true
						}
					}
				}
				return out
			}
			if push {
				pushFailed := false
				for _, ch := range ep.changed {
					updated := ch.client
					updated.UpdatedAt = nowMs
					ctx, cancel := nodePushContext()
					err1 := rt.UpdateUser(ctx, ep.inbound, ch.email, updated)
					cancel()
					if err1 != nil {
						logger.Warning("Error in updating client on", rt.Name(), ":", err1)
						pushFailed = true
						break
					}
				}
				if !pushFailed {
					advancePushedInbound(rt, ep.prevSettings, ep.newSettings, ep.inbound)
				}
			}
			return out
		})
		for i, out := range outcomes {
			if panics[i] != nil {
				needRestart = true
				for _, ch := range enablePlans[i].changed {
					if _, already := skippedReasons[ch.email]; !already {
						skippedReasons[ch.email] = panics[i].Error()
					}
				}
				continue
			}
			if out.needRestart {
				needRestart = true
			}
			if out.err != nil {
				for _, ch := range enablePlans[i].changed {
					if _, already := skippedReasons[ch.email]; !already {
						skippedReasons[ch.email] = out.err.Error()
					}
				}
			}
		}
	}

	result.Changed = len(successEmails)
	for email, reason := range skippedReasons {
		result.Skipped = append(result.Skipped, BulkSetEnableReport{Email: email, Reason: reason})
	}
	return result, needRestart, nil
}
