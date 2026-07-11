package service

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

func sameClientConfigExceptUpdatedAt(a, b map[string]any) bool {
	aa := maps.Clone(a)
	bb := maps.Clone(b)
	delete(aa, "updated_at")
	delete(bb, "updated_at")
	an, aerr := json.Marshal(aa)
	bn, berr := json.Marshal(bb)
	return aerr == nil && berr == nil && string(an) == string(bn)
}

// advancePushedInbound advances the node's reconcile-skip fingerprint from the
// pre-edit settings to the saved ones after every per-client push succeeded.
func advancePushedInbound(rt runtime.Runtime, prevSettings string, ib *model.Inbound) {
	rem, ok := rt.(*runtime.Remote)
	if !ok {
		return
	}
	prev := *ib
	prev.Settings = prevSettings
	rem.AdvancePushedInbound(&prev, ib)
}

// delInboundClients removes several clients from a single inbound in one pass:
// one settings rewrite, one runtime sweep, one Save and one SyncInbound for the
// whole batch, instead of repeating the full per-client cycle. It mirrors the
// semantics of DelInboundClientByEmail for each removed client. needRestart is
// the OR across all removals.
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

	// Match by email — the client's stable identity (see Delete). Removes every
	// entry carrying a wanted email, independent of credential drift.
	wanted := make(map[string]struct{}, len(recs))
	for _, rec := range recs {
		if rec.Email != "" {
			wanted[rec.Email] = struct{}{}
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
		email, _ := c["email"].(string)
		if _, hit := wanted[email]; hit && email != "" {
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
	prevSettings := oldInbound.Settings
	oldInbound.Settings = string(newSettings)

	var sharedSet map[string]bool
	if !keepTraffic {
		removedEmails := make([]string, 0, len(removed))
		for _, r := range removed {
			if r.email != "" {
				removedEmails = append(removedEmails, r.email)
			}
		}
		var sharedErr error
		sharedSet, sharedErr = inboundSvc.emailsUsedByOtherInbounds(removedEmails, inboundId)
		if sharedErr != nil {
			return false, sharedErr
		}
	}

	needRestart := false

	// Read each client's live state before the DB write (DelClientStat would
	// erase the enable flag we need to decide on a runtime removal).
	type delTarget struct {
		email       string
		emailShared bool
		notDepleted bool
		needApiDel  bool
	}
	targets := make([]delTarget, 0, len(removed))
	for _, r := range removed {
		email := r.email
		emailShared := sharedSet[strings.ToLower(strings.TrimSpace(email))]
		notDepleted := false
		if len(email) > 0 {
			var enables []bool
			if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).Limit(1).Pluck("enable", &enables).Error; err != nil {
				logger.Error("Get stats error")
				return needRestart, err
			}
			notDepleted = len(enables) > 0 && enables[0]
		}
		targets = append(targets, delTarget{email: email, emailShared: emailShared, notDepleted: notDepleted, needApiDel: r.needApiDel})
	}

	// Persist the batch deletion atomically, serialized against the traffic poll
	// to avoid the cross-transaction lock-order deadlock (runSerializedTx).
	if txErr := runSerializedTx(func(tx *gorm.DB) error {
		for _, t := range targets {
			if t.emailShared || keepTraffic {
				continue
			}
			if e := inboundSvc.DelClientIPs(tx, t.email); e != nil {
				logger.Error("Error in delete client IPs")
				return e
			}
			if len(t.email) > 0 {
				if e := inboundSvc.DelClientStat(tx, t.email); e != nil {
					logger.Error("Delete stats Data Error")
					return e
				}
			}
		}
		if e := tx.Save(oldInbound).Error; e != nil {
			return e
		}
		finalClients, gcErr := inboundSvc.GetClients(oldInbound)
		if gcErr != nil {
			return gcErr
		}
		if err := s.SyncInbound(tx, inboundId, finalClients); err != nil {
			return err
		}
		if oldInbound.NodeID != nil {
			return (&NodeService{}).MarkNodeDirtyTx(tx, *oldInbound.NodeID)
		}
		return nil
	}); txErr != nil {
		return needRestart, txErr
	}

	// Resolve the node push plan once for the whole batch instead of per email.
	var nodeRt runtime.Runtime
	nodePush := false
	if oldInbound.NodeID != nil {
		rt, push, _, perr := inboundSvc.nodePushPlan(oldInbound)
		if perr != nil {
			return needRestart, perr
		}
		nodeRt, nodePush = rt, push
		// Large batches collapse into one reconcile push rather than M deletes.
		if nodePush && len(targets) > nodeBulkPushThreshold {
			nodePush = false
		}
	}

	// Apply runtime deletes after commit — outside the serialized writer so a
	// slow node call can't stall traffic accounting.
	nodePushFailed := false
	for _, t := range targets {
		if len(t.email) == 0 {
			continue
		}
		if oldInbound.NodeID == nil {
			if t.needApiDel && t.notDepleted {
				rt, rterr := inboundSvc.runtimeFor(oldInbound)
				if rterr != nil {
					needRestart = true
				} else if err1 := rt.RemoveUser(context.Background(), oldInbound, t.email); err1 != nil {
					if !strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", t.email)) {
						needRestart = true
					}
				}
			}
		} else if nodePush {
			if err1 := nodeRt.DeleteUser(context.Background(), oldInbound, t.email); err1 != nil {
				logger.Warning("Error in deleting client on", nodeRt.Name(), ":", err1)
				nodePushFailed = true
			}
		}
	}
	if nodePush && !nodePushFailed {
		advancePushedInbound(nodeRt, prevSettings, oldInbound)
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

	existingClients, err := inboundSvc.GetClients(oldInbound)
	if err != nil {
		return false, err
	}

	// A client already on this inbound is skipped instead of appended again:
	// checkEmailsExistForClients exempts a matching subId so one identity can
	// live on several inbounds, which let retried or raced adds duplicate the
	// same email inside a single settings array (#5770). clients and
	// interfaceClients are parsed from the same data.Settings array, so they
	// stay index-aligned while filtering.
	if len(existingClients) > 0 && len(clients) > 0 {
		existingEmails := make(map[string]struct{}, len(existingClients))
		for _, c := range existingClients {
			if c.Email != "" {
				existingEmails[strings.ToLower(c.Email)] = struct{}{}
			}
		}
		keptClients := make([]model.Client, 0, len(clients))
		keptWire := make([]any, 0, len(interfaceClients))
		for i, c := range clients {
			if c.Email != "" {
				if _, dup := existingEmails[strings.ToLower(c.Email)]; dup {
					continue
				}
			}
			keptClients = append(keptClients, c)
			if i < len(interfaceClients) {
				keptWire = append(keptWire, interfaceClients[i])
			}
		}
		if len(keptClients) == 0 {
			return false, nil
		}
		clients = keptClients
		interfaceClients = keptWire
	}

	if oldInbound.Protocol == model.WireGuard {
		if dErr := defaultWireguardClients(existingClients, clients, interfaceClients); dErr != nil {
			return false, dErr
		}
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
		case "wireguard":
			if client.PublicKey == "" {
				return false, common.NewError("wireguard client requires a key")
			}
		case "mtproto":
			if client.Secret == "" {
				return false, common.NewError("mtproto client requires a secret")
			}
			if client.AdTag != "" && !model.ValidMtprotoAdTag(client.AdTag) {
				return false, common.NewError("mtproto client ad tag must be 32 hex characters")
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

	oldClients, _ := oldSettings["clients"].([]any)
	oldClients = compactOrphans(database.GetDB(), oldClients)
	oldClients = append(oldClients, interfaceClients...)

	oldSettings["clients"] = oldClients

	newSettings, err := json.MarshalIndent(oldSettings, "", "  ")
	if err != nil {
		return false, err
	}

	prevSettings := oldInbound.Settings
	oldInbound.Settings = string(newSettings)

	needRestart := false

	rt, push, _, perr := inboundSvc.nodePushPlan(oldInbound)
	if perr != nil {
		return false, perr
	}

	// Persist client stats + inbound atomically, serialized against the traffic
	// poll to avoid the cross-transaction lock-order deadlock (runSerializedTx).
	if txErr := runSerializedTx(func(tx *gorm.DB) error {
		for i := range clients {
			if len(clients[i].Email) == 0 {
				continue
			}
			if e := inboundSvc.AddClientStat(tx, data.Id, &clients[i]); e != nil {
				return e
			}
		}
		if e := tx.Save(oldInbound).Error; e != nil {
			return e
		}
		finalClients, gcErr := inboundSvc.GetClients(oldInbound)
		if gcErr != nil {
			return gcErr
		}
		if err := s.SyncInbound(tx, oldInbound.Id, finalClients); err != nil {
			return err
		}
		if oldInbound.NodeID != nil {
			return (&NodeService{}).MarkNodeDirtyTx(tx, *oldInbound.NodeID)
		}
		return nil
	}); txErr != nil {
		return false, txErr
	}

	// Apply to the running runtime after commit — outside the serialized writer
	// so a slow node call can't stall traffic accounting.
	if oldInbound.NodeID == nil {
		if !push {
			needRestart = true
		} else if oldInbound.Protocol == model.MTProto {
			inboundSvc.applyLocalMtproto(oldInbound.Id)
		} else {
			for _, client := range clients {
				if len(client.Email) == 0 {
					needRestart = true
					continue
				}
				if !client.Enable {
					continue
				}
				cipher := ""
				if oldInbound.Protocol == "shadowsocks" {
					cipher = oldSettings["method"].(string)
				}
				err1 := rt.AddUser(context.Background(), oldInbound, map[string]any{
					"email":        client.Email,
					"id":           client.ID,
					"auth":         client.Auth,
					"security":     client.Security,
					"flow":         client.Flow,
					"password":     client.Password,
					"cipher":       cipher,
					"publicKey":    client.PublicKey,
					"allowedIPs":   client.AllowedIPs,
					"preSharedKey": client.PreSharedKey,
					"keepAlive":    keepAliveStr(client.KeepAlive),
				})
				if err1 == nil {
					logger.Debug("Client added on", rt.Name(), ":", client.Email)
				} else {
					logger.Debug("Error in adding client on", rt.Name(), ":", err1)
					needRestart = true
				}
			}
		}
	} else {
		// Large batches would be M sequential per-client RPCs; the inbound's saved
		// settings already hold the final set, so mark dirty and let one reconcile
		// push converge the node instead.
		if push && len(clients) > nodeBulkPushThreshold {
			push = false
		}
		for _, client := range clients {
			if push {
				if err1 := rt.AddClient(context.Background(), oldInbound, client); err1 != nil {
					logger.Warning("Error in adding client on", rt.Name(), ":", err1)
					push = false
				}
			}
		}
		if push {
			advancePushedInbound(rt, prevSettings, oldInbound)
		}
	}

	return needRestart, nil
}

func (s *ClientService) UpdateInboundClient(inboundSvc *InboundService, data *model.Inbound, oldEmail string) (bool, error) {
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

	newClientId := ""
	switch oldInbound.Protocol {
	case "trojan":
		newClientId = clients[0].Password
	case "shadowsocks":
		newClientId = clients[0].Email
	case "hysteria":
		newClientId = clients[0].Auth
	case "wireguard":
		newClientId = clients[0].Email
	case "mtproto":
		newClientId = clients[0].Email
	default:
		newClientId = clients[0].ID
	}

	// Locate the client to replace by email — the client's stable identity.
	// Credentials (uuid/password/auth) can drift from the inbound JSON, so they
	// are never used for matching.
	clientIndex := -1
	for index, oldClient := range oldClients {
		if strings.EqualFold(oldClient.Email, oldEmail) {
			oldEmail = oldClient.Email
			clientIndex = index
			break
		}
	}

	if newClientId == "" || clientIndex == -1 {
		return false, common.NewError("empty client ID")
	}
	if strings.TrimSpace(clients[0].Email) == "" {
		return false, common.NewError("client email is required")
	}
	if oldInbound.Protocol == model.MTProto && clients[0].AdTag != "" && !model.ValidMtprotoAdTag(clients[0].AdTag) {
		return false, common.NewError("mtproto client ad tag must be 32 hex characters")
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

	// WireGuard keys are never rotated by an edit: when the incoming payload omits
	// them (a metadata-only change), carry the stored credentials forward so the
	// settings JSON and the running peer keep the client's identity.
	if oldInbound.Protocol == model.WireGuard && clientIndex >= 0 && clientIndex < len(oldClients) {
		old := oldClients[clientIndex]
		if clients[0].PrivateKey == "" {
			clients[0].PrivateKey = old.PrivateKey
		}
		if clients[0].PublicKey == "" {
			clients[0].PublicKey = old.PublicKey
		}
		if len(clients[0].AllowedIPs) == 0 {
			clients[0].AllowedIPs = old.AllowedIPs
		} else {
			normalized, nErr := normalizeWireguardAllowedIPs(clients[0].AllowedIPs)
			if nErr != nil {
				return false, nErr
			}
			if len(normalized) == 0 {
				clients[0].AllowedIPs = old.AllowedIPs
			} else {
				peers := make([]string, 0, len(oldClients))
				for i := range oldClients {
					if i == clientIndex {
						continue
					}
					peers = append(peers, oldClients[i].AllowedIPs...)
				}
				if hit := wireguardAllowedIPsCollision(normalized, peers); hit != "" {
					return false, common.NewError("wireguard: allowedIPs entry already used by another client:", hit)
				}
				clients[0].AllowedIPs = normalized
			}
		}
		if clients[0].PreSharedKey == "" {
			clients[0].PreSharedKey = old.PreSharedKey
		}
		if clients[0].KeepAlive == 0 {
			clients[0].KeepAlive = old.KeepAlive
		}
	}

	var oldSettings map[string]any
	err = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
	if err != nil {
		return false, err
	}
	settingsClients, _ := oldSettings["clients"].([]any)
	var preservedCreated any
	var preservedSubID string
	var oldClientMap map[string]any
	if clientIndex >= 0 && clientIndex < len(settingsClients) {
		if oldMap, ok := settingsClients[clientIndex].(map[string]any); ok {
			oldClientMap = oldMap
			if v, ok2 := oldMap["created_at"]; ok2 {
				preservedCreated = v
			}
			preservedSubID, _ = oldMap["subId"].(string)
		}
	}
	if oldInbound.Protocol == model.Shadowsocks {
		applyShadowsocksClientMethod(interfaceClients, oldSettings)
	}
	if len(interfaceClients) > 0 {
		if newMap, ok := interfaceClients[0].(map[string]any); ok {
			if preservedCreated == nil {
				preservedCreated = time.Now().Unix() * 1000
			}
			newMap["created_at"] = preservedCreated
			newSub, _ := newMap["subId"].(string)
			if strings.TrimSpace(newSub) == "" {
				if strings.TrimSpace(preservedSubID) != "" {
					newMap["subId"] = preservedSubID
				} else {
					newMap["subId"] = random.NumLower(16)
				}
			}
			if v, ok2 := newMap["subId"].(string); ok2 {
				clients[0].SubID = v
			}
			if oldInbound.Protocol == model.WireGuard {
				newMap["privateKey"] = clients[0].PrivateKey
				newMap["publicKey"] = clients[0].PublicKey
				newMap["allowedIPs"] = clients[0].AllowedIPs
				if clients[0].PreSharedKey != "" {
					newMap["preSharedKey"] = clients[0].PreSharedKey
				}
				if clients[0].KeepAlive > 0 {
					newMap["keepAlive"] = clients[0].KeepAlive
				}
			}
			if oldClientMap != nil && sameClientConfigExceptUpdatedAt(oldClientMap, newMap) {
				if v, ok2 := oldClientMap["updated_at"]; ok2 {
					newMap["updated_at"] = v
				} else {
					delete(newMap, "updated_at")
				}
			} else {
				newMap["updated_at"] = time.Now().Unix() * 1000
			}
			interfaceClients[0] = newMap
		}
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

	if string(newSettings) == oldInbound.Settings {
		return false, nil
	}

	prevSettings := oldInbound.Settings
	oldInbound.Settings = string(newSettings)

	needRestart := false

	// Resolve the push plan before the DB write so a node-state lookup failure
	// still aborts the whole update without committing anything (it used to roll
	// the transaction back). nodePushPlan only reads, so order doesn't matter.
	var rt runtime.Runtime
	var push bool
	if len(oldEmail) > 0 {
		var perr error
		rt, push, _, perr = inboundSvc.nodePushPlan(oldInbound)
		if perr != nil {
			return false, perr
		}
	}

	// Persist client stats + inbound atomically, serialized against the traffic
	// poll to avoid the cross-transaction lock-order deadlock (runSerializedTx).
	if txErr := runSerializedTx(func(tx *gorm.DB) error {
		if len(clients[0].Email) > 0 {
			if len(oldEmail) > 0 {
				emailUnchanged := strings.EqualFold(oldEmail, clients[0].Email)
				targetExists := int64(0)
				if !emailUnchanged {
					if e := tx.Model(xray.ClientTraffic{}).Where("email = ?", clients[0].Email).Count(&targetExists).Error; e != nil {
						return e
					}
				}
				if emailUnchanged || targetExists == 0 {
					if e := inboundSvc.UpdateClientStat(tx, oldEmail, &clients[0]); e != nil {
						return e
					}
					if e := inboundSvc.UpdateClientIPs(tx, oldEmail, clients[0].Email); e != nil {
						return e
					}
				} else {
					stillUsed, sErr := inboundSvc.emailUsedByOtherInbounds(oldEmail, data.Id)
					if sErr != nil {
						return sErr
					}
					if !stillUsed {
						if e := inboundSvc.DelClientStat(tx, oldEmail); e != nil {
							return e
						}
						if e := inboundSvc.DelClientIPs(tx, oldEmail); e != nil {
							return e
						}
					}
					if e := inboundSvc.UpdateClientStat(tx, clients[0].Email, &clients[0]); e != nil {
						return e
					}
				}
			} else {
				if e := inboundSvc.AddClientStat(tx, data.Id, &clients[0]); e != nil {
					return e
				}
			}
		} else {
			stillUsed, sErr := inboundSvc.emailUsedByOtherInbounds(oldEmail, data.Id)
			if sErr != nil {
				return sErr
			}
			if !stillUsed {
				if e := inboundSvc.DelClientStat(tx, oldEmail); e != nil {
					return e
				}
				if e := inboundSvc.DelClientIPs(tx, oldEmail); e != nil {
					return e
				}
			}
		}

		if e := tx.Save(oldInbound).Error; e != nil {
			return e
		}
		// Rename the client record in the same transaction as the settings JSON
		// so no concurrent SyncInbound can see one renamed without the other.
		if len(oldEmail) > 0 && !strings.EqualFold(oldEmail, clients[0].Email) {
			var renameTaken int64
			if e := tx.Model(&model.ClientRecord{}).Where("email = ?", clients[0].Email).Count(&renameTaken).Error; e != nil {
				return e
			}
			if renameTaken == 0 {
				if e := tx.Model(&model.ClientRecord{}).Where("email = ?", oldEmail).Update("email", clients[0].Email).Error; e != nil {
					return e
				}
			}
		}
		finalClients, gcErr := inboundSvc.GetClients(oldInbound)
		if gcErr != nil {
			return gcErr
		}
		if err := s.SyncInbound(tx, oldInbound.Id, finalClients); err != nil {
			return err
		}
		if oldInbound.NodeID != nil {
			return (&NodeService{}).MarkNodeDirtyTx(tx, *oldInbound.NodeID)
		}
		return nil
	}); txErr != nil {
		return false, txErr
	}

	// Apply to the running runtime after the DB is committed — outside the
	// serialized writer so a slow node call can't stall traffic accounting.
	if len(oldEmail) > 0 {
		if oldInbound.NodeID == nil {
			if !push {
				needRestart = true
			} else if oldInbound.Protocol == model.MTProto {
				inboundSvc.applyLocalMtproto(oldInbound.Id)
			} else {
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
						"email":        clients[0].Email,
						"id":           clients[0].ID,
						"security":     clients[0].Security,
						"flow":         clients[0].Flow,
						"auth":         clients[0].Auth,
						"password":     clients[0].Password,
						"cipher":       cipher,
						"publicKey":    clients[0].PublicKey,
						"allowedIPs":   clients[0].AllowedIPs,
						"preSharedKey": clients[0].PreSharedKey,
						"keepAlive":    keepAliveStr(clients[0].KeepAlive),
					})
					if err1 == nil {
						logger.Debug("Client edited on", rt.Name(), ":", clients[0].Email)
					} else {
						logger.Debug("Error in adding client on", rt.Name(), ":", err1)
						needRestart = true
					}
				}
			}
		} else if push {
			if err1 := rt.UpdateUser(context.Background(), oldInbound, oldEmail, clients[0]); err1 != nil {
				logger.Warning("Error in updating client on", rt.Name(), ":", err1)
			} else {
				advancePushedInbound(rt, prevSettings, oldInbound)
			}
		}
	} else {
		logger.Debug("Client old email not found")
		needRestart = true
	}

	return needRestart, nil
}

func (s *ClientService) DelInboundClientByEmail(inboundSvc *InboundService, inboundId int, email string, keepTraffic bool, fullDelete bool) (bool, error) {
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
		return false, fmt.Errorf("%w for email: %s", ErrClientNotInInbound, email)
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

	prevSettings := oldInbound.Settings
	oldInbound.Settings = string(newSettings)

	emailShared, err := inboundSvc.emailUsedByOtherInbounds(email, inboundId)
	if err != nil {
		return false, err
	}

	needRestart := false

	// Decide what to delete and the push plan before the serialized DB write —
	// these are reads, and nodePushPlan failing should abort before committing.
	delStat := false
	if len(email) > 0 && !emailShared && !keepTraffic {
		traffic, tErr := inboundSvc.GetClientTrafficByEmail(email)
		if tErr != nil {
			return false, tErr
		}
		delStat = traffic != nil
	}

	// The runtime user is scoped to this inbound's tag + email, so the push plan
	// is resolved independently of emailShared — a sibling inbound still carrying
	// the email must not suppress removing the user from this inbound's Xray.
	var rt runtime.Runtime
	var push bool
	if len(email) > 0 && (oldInbound.NodeID != nil || needApiDel) {
		r, p, _, perr := inboundSvc.nodePushPlan(oldInbound)
		if perr != nil {
			return false, perr
		}
		rt, push = r, p
	}

	// Persist the deletion atomically, serialized against the traffic poll to
	// avoid the cross-transaction lock-order deadlock (runSerializedTx).
	if txErr := runSerializedTx(func(tx *gorm.DB) error {
		if !emailShared && !keepTraffic {
			if e := inboundSvc.DelClientIPs(tx, email); e != nil {
				logger.Error("Error in delete client IPs")
				return e
			}
		}
		if delStat {
			if e := inboundSvc.DelClientStat(tx, email); e != nil {
				logger.Error("Delete stats Data Error")
				return e
			}
		}
		if e := tx.Save(oldInbound).Error; e != nil {
			return e
		}
		finalClients, gcErr := inboundSvc.GetClients(oldInbound)
		if gcErr != nil {
			return gcErr
		}
		if err := s.SyncInbound(tx, inboundId, finalClients); err != nil {
			return err
		}
		if oldInbound.NodeID != nil {
			return (&NodeService{}).MarkNodeDirtyTx(tx, *oldInbound.NodeID)
		}
		return nil
	}); txErr != nil {
		return false, txErr
	}

	// Apply the runtime delete after commit — outside the serialized writer so a
	// slow node call can't stall traffic accounting. Independent of emailShared:
	// Xray users are keyed by inbound tag, so the user must be removed from this
	// inbound's runtime even when the same email survives in another inbound.
	if len(email) > 0 {
		if oldInbound.NodeID == nil {
			if oldInbound.Protocol == model.MTProto {
				// mtg serves the full secret set, so any client delete re-applies
				// it (removing the last client stops the sidecar) regardless of the
				// client's enable state.
				inboundSvc.applyLocalMtproto(oldInbound.Id)
			} else if needApiDel {
				// Local inbound: a disabled client isn't in the running Xray, so only
				// a live one (needApiDel) needs an API removal.
				if !push {
					needRestart = true
				} else if err1 := rt.RemoveUser(context.Background(), oldInbound, email); err1 == nil {
					logger.Debug("Client deleted on", rt.Name(), ":", email)
					needRestart = false
				} else if strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", email)) {
					logger.Debug("User is already deleted. Nothing to do more...")
				} else {
					logger.Debug("Error in deleting client on", rt.Name(), ":", email)
					needRestart = true
				}
			}
		} else {
			// Node inbound: propagate the delete regardless of the enable flag —
			// the node's own DB still carries a disabled client and would
			// resurrect it on the next snapshot otherwise. A full client delete
			// must remove the node's client record too, not just detach it from
			// this inbound (#5797).
			if push {
				var err1 error
				if fullDelete {
					err1 = rt.DeleteClient(context.Background(), email)
				} else {
					err1 = rt.DeleteUser(context.Background(), oldInbound, email)
				}
				if err1 != nil {
					logger.Warning("Error in deleting client on", rt.Name(), ":", err1)
				} else {
					advancePushedInbound(rt, prevSettings, oldInbound)
				}
			}
		}
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

	found := false
	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			found = true
			break
		}
	}

	if !found {
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
	needRestart, err := s.UpdateInboundClient(inboundSvc, inbound, clientEmail)
	return needRestart, err
}

func (s *ClientService) CheckIsEnabledByEmail(inboundSvc *InboundService, clientEmail string) (bool, error) {
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

	found := false
	clientOldEnabled := false

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			found = true
			clientOldEnabled = oldClient.Enable
			break
		}
	}

	if !found {
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

	needRestart, err := s.UpdateInboundClient(inboundSvc, inbound, clientEmail)
	if err != nil {
		return false, needRestart, err
	}

	return !clientOldEnabled, needRestart, nil
}

func (s *ClientService) SetClientEnableByEmail(inboundSvc *InboundService, clientEmail string, enable bool) (bool, bool, error) {
	current, err := s.CheckIsEnabledByEmail(inboundSvc, clientEmail)
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

// applyClientFieldByEmail loads the inbound currently hosting clientEmail,
// confirms the client exists, applies mutate to the matching client (plus a
// refreshed updated_at), and hands a single-client update payload to
// UpdateInboundClient. The rebuilt clients array intentionally contains only
// the matched client — that is the input contract UpdateInboundClient expects
// (clients[0] is the new data; clientEmail locates the row to replace). It
// backs the single-field by-email setters below.
// applyClientFieldByEmail mutates a client field on every inbound the email is
// attached to. A multi-inbound client is one logical identity: patching only
// the first inbound's JSON would leave the siblings stale, and the next
// SyncInbound over a stale sibling would revert the edit in the normalized
// records (#5039).
func (s *ClientService) applyClientFieldByEmail(inboundSvc *InboundService, clientEmail string, mutate func(c map[string]any)) (bool, error) {
	inboundIds, err := s.GetInboundIdsForEmail(database.GetDB(), clientEmail)
	if err != nil {
		return false, err
	}
	if len(inboundIds) == 0 {
		// Legacy fallback for clients that only live in the inbound JSON and
		// were never normalized into client_inbounds.
		_, inbound, gErr := inboundSvc.GetClientInboundByEmail(clientEmail)
		if gErr != nil {
			return false, gErr
		}
		if inbound == nil {
			return false, common.NewError("Inbound Not Found For Email:", clientEmail)
		}
		inboundIds = []int{inbound.Id}
	}

	needRestart := false
	found := false
	for _, ibId := range inboundIds {
		inbound, gErr := inboundSvc.GetInbound(ibId)
		if gErr != nil {
			return needRestart, gErr
		}

		var settings map[string]any
		if uErr := json.Unmarshal([]byte(inbound.Settings), &settings); uErr != nil {
			return needRestart, uErr
		}
		clients, _ := settings["clients"].([]any)
		// UpdateInboundClient expects a single-client payload, so keep only the
		// matching entry in the scratch copy; it splices the result back into
		// the inbound's full client list itself.
		var newClients []any
		for client_index := range clients {
			c, ok := clients[client_index].(map[string]any)
			if !ok {
				continue
			}
			if c["email"] == clientEmail {
				mutate(c)
				c["updated_at"] = time.Now().Unix() * 1000
				newClients = append(newClients, any(c))
			}
		}
		if len(newClients) == 0 {
			continue
		}
		found = true
		settings["clients"] = newClients
		modifiedSettings, mErr := json.MarshalIndent(settings, "", "  ")
		if mErr != nil {
			return needRestart, mErr
		}
		inbound.Settings = string(modifiedSettings)
		nr, uErr := s.UpdateInboundClient(inboundSvc, inbound, clientEmail)
		if uErr != nil {
			return needRestart, uErr
		}
		needRestart = needRestart || nr
	}

	if !found {
		return needRestart, common.NewError("Client Not Found For Email:", clientEmail)
	}
	return needRestart, nil
}

func (s *ClientService) ResetClientIpLimitByEmail(inboundSvc *InboundService, clientEmail string, count int) (bool, error) {
	return s.applyClientFieldByEmail(inboundSvc, clientEmail, func(c map[string]any) {
		c["limitIp"] = count
	})
}

func (s *ClientService) ResetClientExpiryTimeByEmail(inboundSvc *InboundService, clientEmail string, expiry_time int64) (bool, error) {
	return s.applyClientFieldByEmail(inboundSvc, clientEmail, func(c map[string]any) {
		c["expiryTime"] = expiry_time
	})
}

func (s *ClientService) ResetClientTrafficLimitByEmail(inboundSvc *InboundService, clientEmail string, totalGB int) (bool, error) {
	if totalGB < 0 {
		return false, common.NewError("totalGB must be >= 0")
	}
	return s.applyClientFieldByEmail(inboundSvc, clientEmail, func(c map[string]any) {
		c["totalGB"] = totalGB * 1024 * 1024 * 1024
	})
}
