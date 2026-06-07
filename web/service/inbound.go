// Package service provides business logic services for the 3x-ui web panel,
// including inbound/outbound management, user administration, settings, and Xray integration.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/mhsanaei/3x-ui/v3/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/xray"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var reportedRemoteTagConflict sync.Map

type InboundService struct {
	xrayApi         xray.XrayAPI
	clientService   ClientService
	fallbackService FallbackService
}

func (s *InboundService) runtimeFor(ib *model.Inbound) (runtime.Runtime, error) {
	mgr := runtime.GetManager()
	if mgr == nil {
		return nil, fmt.Errorf("runtime manager not initialised")
	}
	return mgr.RuntimeFor(ib.NodeID)
}

func (s *InboundService) nodePushPlan(ib *model.Inbound) (runtime.Runtime, bool, bool, error) {
	if ib.NodeID == nil {
		rt, err := s.runtimeFor(ib)
		if err != nil {
			return nil, false, false, nil
		}
		return rt, true, false, nil
	}
	nodeSvc := NodeService{}
	enabled, status, _, _, err := nodeSvc.NodeSyncState(*ib.NodeID)
	if err != nil {
		return nil, false, false, err
	}
	if !enabled || status == "offline" {
		return nil, false, true, nil
	}
	rt, err := s.runtimeFor(ib)
	if err != nil {
		return nil, false, true, nil
	}
	return rt, true, false, nil
}

func (s *InboundService) NodeIsPending(nodeID *int) bool {
	if nodeID == nil {
		return false
	}
	return (&NodeService{}).IsNodePending(*nodeID)
}

func (s *InboundService) AnyNodePending(inboundIds []int) bool {
	if len(inboundIds) == 0 {
		return false
	}
	nodeSvc := NodeService{}
	for _, id := range inboundIds {
		ib, err := s.GetInbound(id)
		if err != nil || ib.NodeID == nil {
			continue
		}
		if nodeSvc.IsNodePending(*ib.NodeID) {
			return true
		}
	}
	return false
}

func (s *InboundService) ReconcileNode(ctx context.Context, rt *runtime.Remote, nodeID int) error {
	if rt == nil || nodeID <= 0 {
		return nil
	}
	db := database.GetDB()
	var inbounds []*model.Inbound
	if err := db.Model(model.Inbound{}).Where("node_id = ?", nodeID).Find(&inbounds).Error; err != nil {
		return err
	}
	remoteTags, err := rt.ListRemoteTags(ctx)
	if err != nil {
		return err
	}
	prefix := nodeTagPrefix(&nodeID)
	desiredTags := make(map[string]struct{}, len(inbounds)*2)
	for _, ib := range inbounds {
		desiredTags[ib.Tag] = struct{}{}
		if prefix != "" {
			if stripped, found := strings.CutPrefix(ib.Tag, prefix); found {
				desiredTags[stripped] = struct{}{}
			} else {
				desiredTags[prefix+ib.Tag] = struct{}{}
			}
		}
		if err := rt.UpdateInbound(ctx, ib, ib); err != nil {
			return fmt.Errorf("reconcile inbound %q: %w", ib.Tag, err)
		}
	}
	for _, tag := range remoteTags {
		if _, want := desiredTags[tag]; want {
			continue
		}
		if err := rt.DelInbound(ctx, &model.Inbound{Tag: tag}); err != nil {
			return fmt.Errorf("reconcile delete %q: %w", tag, err)
		}
	}
	return nil
}

type CopyClientsResult struct {
	Added   []string `json:"added"`
	Skipped []string `json:"skipped"`
	Errors  []string `json:"errors"`
}

// enrichClientStats parses each inbound's clients once, fills in the
// UUID/SubId fields on the preloaded ClientStats, and tops up rows owned by
// a sibling inbound (shared-email mode — the row is keyed on email so it
// only preloads on its owning inbound).
func (s *InboundService) enrichClientStats(db *gorm.DB, inbounds []*model.Inbound) {
	if len(inbounds) == 0 {
		return
	}
	clientsByInbound := make([][]model.Client, len(inbounds))
	seenByInbound := make([]map[string]struct{}, len(inbounds))
	missing := make(map[string]struct{})
	for i, inbound := range inbounds {
		clients, _ := s.GetClients(inbound)
		clientsByInbound[i] = clients
		seen := make(map[string]struct{}, len(inbound.ClientStats))
		for _, st := range inbound.ClientStats {
			if st.Email != "" {
				seen[strings.ToLower(st.Email)] = struct{}{}
			}
		}
		seenByInbound[i] = seen
		for _, c := range clients {
			if c.Email == "" {
				continue
			}
			if _, ok := seen[strings.ToLower(c.Email)]; !ok {
				missing[c.Email] = struct{}{}
			}
		}
	}
	if len(missing) > 0 {
		emails := make([]string, 0, len(missing))
		for e := range missing {
			emails = append(emails, e)
		}
		var extra []xray.ClientTraffic
		var loadErr error
		for _, batch := range chunkStrings(emails, sqlInChunk) {
			var page []xray.ClientTraffic
			if err := db.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Find(&page).Error; err != nil {
				loadErr = err
				break
			}
			extra = append(extra, page...)
		}
		if loadErr != nil {
			logger.Warning("enrichClientStats:", loadErr)
		} else {
			byEmail := make(map[string]xray.ClientTraffic, len(extra))
			for _, st := range extra {
				byEmail[strings.ToLower(st.Email)] = st
			}
			for i, inbound := range inbounds {
				for _, c := range clientsByInbound[i] {
					if c.Email == "" {
						continue
					}
					key := strings.ToLower(c.Email)
					if _, ok := seenByInbound[i][key]; ok {
						continue
					}
					if st, ok := byEmail[key]; ok {
						inbound.ClientStats = append(inbound.ClientStats, st)
						seenByInbound[i][key] = struct{}{}
					}
				}
			}
		}
	}
	for i, inbound := range inbounds {
		clients := clientsByInbound[i]
		if len(clients) == 0 || len(inbound.ClientStats) == 0 {
			continue
		}
		cMap := make(map[string]model.Client, len(clients))
		for _, c := range clients {
			cMap[strings.ToLower(c.Email)] = c
		}
		for j := range inbound.ClientStats {
			email := strings.ToLower(inbound.ClientStats[j].Email)
			if c, ok := cMap[email]; ok {
				inbound.ClientStats[j].UUID = c.ID
				inbound.ClientStats[j].SubId = c.SubID
			}
		}
	}
}

// GetInbounds retrieves all inbounds for a specific user with client stats.
func (s *InboundService) GetInbounds(userId int) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Where("user_id = ?", userId).Order("id ASC").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	s.enrichClientStats(db, inbounds)
	s.annotateFallbackParents(db, inbounds)
	s.annotateLocalOriginGuid(inbounds)
	return inbounds, nil
}

// annotateLocalOriginGuid fills OriginNodeGuid for this panel's OWN inbounds
// (NodeID == nil) with the panel's stable GUID; inbounds synced from a node
// already carry the originating node's GUID. Read-time only (not persisted) so
// the per-inbound online view can scope by GUID uniformly across a chain of
// nodes (#4983).
func (s *InboundService) annotateLocalOriginGuid(inbounds []*model.Inbound) {
	if len(inbounds) == 0 {
		return
	}
	guid := s.panelGuid()
	if guid == "" {
		return
	}
	for _, ib := range inbounds {
		if ib.OriginNodeGuid == "" && ib.NodeID == nil {
			ib.OriginNodeGuid = guid
		}
	}
}

// GetInboundsSlim returns the same list of inbounds as GetInbounds but
// strips every per-client field other than email / enable / comment from
// settings.clients and skips UUID/SubId enrichment on ClientStats. The
// inbounds page only needs those three to roll up client counts and
// render badges, so this trims tens of bytes per client (UUID, password,
// flow, security, totalGB, expiryTime, limitIp, tgId, ...) which adds
// up fast on installs with thousands of clients.
//
// Full client data is still available through GET /panel/api/inbounds/get/:id
// for the edit/info/qr/export/clone flows that need it.
func (s *InboundService) GetInboundsSlim(userId int) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Where("user_id = ?", userId).Order("id ASC").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	s.annotateFallbackParents(db, inbounds)
	s.annotateLocalOriginGuid(inbounds)
	for _, ib := range inbounds {
		ib.Settings = slimSettingsClients(ib.Settings)
	}
	return inbounds, nil
}

// slimSettingsClients rewrites the inbound settings JSON so settings.clients[]
// keeps only the fields the list view actually reads. Returns the input
// unchanged when the JSON can't be parsed or has no clients array.
func slimSettingsClients(settings string) string {
	if settings == "" {
		return settings
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(settings), &raw); err != nil {
		return settings
	}
	clients, ok := raw["clients"].([]any)
	if !ok || len(clients) == 0 {
		return settings
	}
	slim := make([]any, 0, len(clients))
	for _, entry := range clients {
		c, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		row := make(map[string]any, 3)
		if v, ok := c["email"]; ok {
			row["email"] = v
		}
		if v, ok := c["enable"]; ok {
			row["enable"] = v
		}
		if v, ok := c["comment"]; ok && v != "" {
			row["comment"] = v
		}
		slim = append(slim, row)
	}
	raw["clients"] = slim
	out, err := json.Marshal(raw)
	if err != nil {
		return settings
	}
	return string(out)
}

// annotateFallbackParents fills FallbackParent on each inbound that is
// the child side of a fallback rule. One DB round-trip serves the full
// list — the frontend needs this to rewrite the child's client-share
// link so it points at the master's reachable endpoint.
func (s *InboundService) annotateFallbackParents(db *gorm.DB, inbounds []*model.Inbound) {
	if len(inbounds) == 0 {
		return
	}
	childIds := make([]int, 0, len(inbounds))
	for _, ib := range inbounds {
		childIds = append(childIds, ib.Id)
	}
	var rows []model.InboundFallback
	if err := db.Where("child_id IN ?", childIds).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		return
	}
	first := make(map[int]model.InboundFallback, len(rows))
	for _, r := range rows {
		if _, ok := first[r.ChildId]; !ok {
			first[r.ChildId] = r
		}
	}
	for _, ib := range inbounds {
		if r, ok := first[ib.Id]; ok {
			ib.FallbackParent = &model.FallbackParentInfo{
				MasterId: r.MasterId,
				Path:     r.Path,
			}
		}
	}
}

type InboundOption struct {
	Id             int    `json:"id" example:"1"`
	Remark         string `json:"remark" example:"VLESS-443"`
	Tag            string `json:"tag" example:"in-443-tcp"`
	Protocol       string `json:"protocol" example:"vless"`
	Port           int    `json:"port" example:"443"`
	TlsFlowCapable bool   `json:"tlsFlowCapable" example:"true"`
	SsMethod       string `json:"ssMethod"`
}

func (s *InboundService) GetInboundOptions(userId int) ([]InboundOption, error) {
	db := database.GetDB()
	var rows []struct {
		Id             int    `gorm:"column:id"`
		Remark         string `gorm:"column:remark"`
		Tag            string `gorm:"column:tag"`
		Protocol       string `gorm:"column:protocol"`
		Port           int    `gorm:"column:port"`
		StreamSettings string `gorm:"column:stream_settings"`
		Settings       string `gorm:"column:settings"`
	}
	err := db.Table("inbounds").
		Select("id, remark, tag, protocol, port, stream_settings, settings").
		Where("user_id = ?", userId).
		Order("id ASC").
		Scan(&rows).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	out := make([]InboundOption, 0, len(rows))
	for _, r := range rows {
		out = append(out, InboundOption{
			Id:             r.Id,
			Remark:         r.Remark,
			Tag:            r.Tag,
			Protocol:       r.Protocol,
			Port:           r.Port,
			TlsFlowCapable: inboundCanEnableTlsFlow(r.Protocol, r.StreamSettings),
			SsMethod:       inboundShadowsocksMethod(r.Protocol, r.Settings),
		})
	}
	return out, nil
}

// inboundShadowsocksMethod extracts settings.method for Shadowsocks inbounds so
// the client UI can generate a valid PSK (base64 of the method's key length)
// for Shadowsocks 2022 ciphers. Returns "" for non-Shadowsocks inbounds.
func inboundShadowsocksMethod(protocol, settings string) string {
	if protocol != string(model.Shadowsocks) || settings == "" {
		return ""
	}
	var s struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return ""
	}
	return s.Method
}

// inboundCanEnableTlsFlow mirrors Inbound.canEnableTlsFlow() from the frontend:
// XTLS Vision is only valid for VLESS on TCP with tls or reality.
func inboundCanEnableTlsFlow(protocol, streamSettings string) bool {
	if protocol != string(model.VLESS) {
		return false
	}
	if streamSettings == "" {
		return false
	}
	var stream struct {
		Network  string `json:"network"`
		Security string `json:"security"`
	}
	if err := json.Unmarshal([]byte(streamSettings), &stream); err != nil {
		return false
	}
	if stream.Network != "tcp" {
		return false
	}
	return stream.Security == "tls" || stream.Security == "reality"
}

// inboundCanHostFallbacks gates the settings.fallbacks injection.
// Xray only honors fallbacks on VLESS and Trojan inbounds carried over
// TCP transport with TLS or Reality security.
func inboundCanHostFallbacks(ib *model.Inbound) bool {
	if ib == nil {
		return false
	}
	if ib.Protocol != model.VLESS && ib.Protocol != model.Trojan {
		return false
	}
	return inboundCanEnableTlsFlow(string(ib.Protocol), ib.StreamSettings) ||
		(ib.Protocol == model.Trojan && trojanStreamSupportsFallbacks(ib.StreamSettings))
}

// trojanStreamSupportsFallbacks mirrors the Trojan side of the same gate
// (Trojan reuses XTLS-Vision capable streams: tcp + tls or reality).
func trojanStreamSupportsFallbacks(streamSettings string) bool {
	if streamSettings == "" {
		return false
	}
	var stream struct {
		Network  string `json:"network"`
		Security string `json:"security"`
	}
	if err := json.Unmarshal([]byte(streamSettings), &stream); err != nil {
		return false
	}
	if stream.Network != "tcp" {
		return false
	}
	return stream.Security == "tls" || stream.Security == "reality"
}

// GetAllInbounds retrieves all inbounds with client stats.
func (s *InboundService) GetAllInbounds() ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	s.enrichClientStats(db, inbounds)
	return inbounds, nil
}

func (s *InboundService) GetInboundsByTrafficReset(period string) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Where("traffic_reset = ?", period).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) GetClients(inbound *model.Inbound) ([]model.Client, error) {
	settings := map[string][]model.Client{}
	json.Unmarshal([]byte(inbound.Settings), &settings)
	if settings == nil {
		return nil, fmt.Errorf("setting is null")
	}

	clients := settings["clients"]
	if clients == nil {
		return nil, nil
	}
	return clients, nil
}

func (s *InboundService) getAllEmails() ([]string, error) {
	db := database.GetDB()
	var emails []string
	query := fmt.Sprintf(
		"SELECT DISTINCT %s %s",
		database.JSONFieldText("client.value", "email"),
		database.JSONClientsFromInbound(),
	)
	if err := db.Raw(query).Scan(&emails).Error; err != nil {
		return nil, err
	}
	return emails, nil
}

// getAllEmailSubIDs returns email→subId. An email seen with two different
// non-empty subIds is locked (mapped to "") so neither identity can claim it.
func (s *InboundService) getAllEmailSubIDs() (map[string]string, error) {
	db := database.GetDB()
	var rows []struct {
		Email string
		SubID string
	}
	query := fmt.Sprintf(
		"SELECT %s AS email, %s AS sub_id %s",
		database.JSONFieldText("client.value", "email"),
		database.JSONFieldText("client.value", "subId"),
		database.JSONClientsFromInbound(),
	)
	if err := db.Raw(query).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string, len(rows))
	for _, r := range rows {
		email := strings.ToLower(r.Email)
		if email == "" {
			continue
		}
		subID := r.SubID
		if existing, ok := result[email]; ok {
			if existing != subID {
				result[email] = ""
			}
			continue
		}
		result[email] = subID
	}
	return result, nil
}

// emailUsedByOtherInbounds reports whether email lives in any inbound other
// than exceptInboundId. Empty email returns false.
func (s *InboundService) emailUsedByOtherInbounds(email string, exceptInboundId int) (bool, error) {
	if email == "" {
		return false, nil
	}
	db := database.GetDB()
	var count int64
	query := fmt.Sprintf(
		"SELECT COUNT(*) %s WHERE inbounds.id != ? AND LOWER(%s) = LOWER(?)",
		database.JSONClientsFromInbound(),
		database.JSONFieldText("client.value", "email"),
	)
	if err := db.Raw(query, exceptInboundId, email).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *InboundService) emailsUsedByOtherInbounds(emails []string, exceptInboundId int) (map[string]bool, error) {
	shared := make(map[string]bool, len(emails))
	want := make(map[string]struct{}, len(emails))
	for _, e := range emails {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			want[e] = struct{}{}
		}
	}
	if len(want) == 0 {
		return shared, nil
	}
	db := database.GetDB()
	var rows []string
	query := fmt.Sprintf(
		"SELECT DISTINCT LOWER(%s) %s WHERE inbounds.id != ?",
		database.JSONFieldText("client.value", "email"),
		database.JSONClientsFromInbound(),
	)
	if err := db.Raw(query, exceptInboundId).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, e := range rows {
		e = strings.ToLower(strings.TrimSpace(e))
		if _, ok := want[e]; ok {
			shared[e] = true
		}
	}
	return shared, nil
}

// normalizeStreamSettings clears StreamSettings for protocols that don't use it.
// Only vmess, vless, trojan, shadowsocks, and hysteria protocols use streamSettings.
func (s *InboundService) normalizeStreamSettings(inbound *model.Inbound) {
	protocolsWithStream := map[model.Protocol]bool{
		model.VMESS:       true,
		model.VLESS:       true,
		model.Trojan:      true,
		model.Shadowsocks: true,
		model.Hysteria:    true,
	}

	if !protocolsWithStream[inbound.Protocol] {
		inbound.StreamSettings = ""
	}
}

// AddInbound creates a new inbound configuration.
// It validates port uniqueness, client email uniqueness, and required fields,
// then saves the inbound to the database and optionally adds it to the running Xray instance.
// Returns the created inbound, whether Xray needs restart, and any error.
func (s *InboundService) AddInbound(inbound *model.Inbound) (*model.Inbound, bool, error) {
	// Normalize streamSettings based on protocol
	s.normalizeStreamSettings(inbound)

	conflict, err := s.checkPortConflict(inbound, 0)
	if err != nil {
		return inbound, false, err
	}
	if conflict != nil {
		return inbound, false, common.NewError(conflict.String())
	}

	inbound.Tag, err = s.resolveInboundTag(inbound, 0)
	if err != nil {
		return inbound, false, err
	}

	clients, err := s.GetClients(inbound)
	if err != nil {
		return inbound, false, err
	}
	existEmail, err := s.clientService.checkEmailsExistForClients(s, clients, nil)
	if err != nil {
		return inbound, false, err
	}
	if existEmail != "" {
		return inbound, false, common.NewError("Duplicate email:", existEmail)
	}

	// Ensure created_at and updated_at on clients in settings
	if len(clients) > 0 {
		var settings map[string]any
		if err2 := json.Unmarshal([]byte(inbound.Settings), &settings); err2 == nil && settings != nil {
			now := time.Now().Unix() * 1000
			updatedClients := make([]model.Client, 0, len(clients))
			for _, c := range clients {
				if c.CreatedAt == 0 {
					c.CreatedAt = now
				}
				c.UpdatedAt = now
				updatedClients = append(updatedClients, c)
			}
			settings["clients"] = updatedClients
			if bs, err3 := json.MarshalIndent(settings, "", "  "); err3 == nil {
				inbound.Settings = string(bs)
			} else {
				logger.Debug("Unable to marshal inbound settings with timestamps:", err3)
			}
		} else if err2 != nil {
			logger.Debug("Unable to parse inbound settings for timestamps:", err2)
		}
	}

	// Secure client ID
	for _, client := range clients {
		switch inbound.Protocol {
		case "trojan":
			if client.Password == "" {
				return inbound, false, common.NewError("empty client ID")
			}
		case "shadowsocks":
			if client.Email == "" {
				return inbound, false, common.NewError("empty client ID")
			}
		case "hysteria":
			if client.Auth == "" {
				return inbound, false, common.NewError("empty client ID")
			}
		default:
			if client.ID == "" {
				return inbound, false, common.NewError("empty client ID")
			}
		}
	}

	db := database.GetDB()
	tx := db.Begin()
	markDirty := false
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		tx.Commit()
		if markDirty && inbound.NodeID != nil {
			if dErr := (&NodeService{}).MarkNodeDirty(*inbound.NodeID); dErr != nil {
				logger.Warning("mark node dirty failed:", dErr)
			}
		}
	}()

	err = tx.Save(inbound).Error
	if err == nil {
		if len(inbound.ClientStats) == 0 {
			for _, client := range clients {
				s.AddClientStat(tx, inbound.Id, &client)
			}
		}
	} else {
		return inbound, false, err
	}

	if err = s.clientService.SyncInbound(tx, inbound.Id, clients); err != nil {
		return inbound, false, err
	}

	needRestart := false
	if inbound.Enable {
		rt, push, dirty, perr := s.nodePushPlan(inbound)
		if perr != nil {
			err = perr
			return inbound, false, err
		}
		if dirty {
			markDirty = true
		}
		if push {
			if err1 := rt.AddInbound(context.Background(), inbound); err1 == nil {
				logger.Debug("New inbound added on", rt.Name(), ":", inbound.Tag)
			} else {
				logger.Debug("Unable to add inbound on", rt.Name(), ":", err1)
				if inbound.NodeID != nil {
					markDirty = true
				} else {
					needRestart = true
				}
			}
		}
	}

	return inbound, needRestart, err
}

func (s *InboundService) DelInbound(id int) (bool, error) {
	db := database.GetDB()

	needRestart := false
	markDirty := false
	var ib model.Inbound
	loadErr := db.Model(model.Inbound{}).Where("id = ?", id).First(&ib).Error
	if loadErr == nil {
		shouldPushToRuntime := ib.NodeID != nil || ib.Enable
		if shouldPushToRuntime {
			rt, push, dirty, perr := s.nodePushPlan(&ib)
			if perr != nil {
				logger.Warning("DelInbound: node lookup failed, deleting central row anyway:", perr)
				markDirty = true
			} else if push {
				if err1 := rt.DelInbound(context.Background(), &ib); err1 == nil {
					logger.Debug("Inbound deleted on", rt.Name(), ":", ib.Tag)
				} else {
					logger.Warning("DelInbound on", rt.Name(), "failed, deleting central row anyway:", err1)
					if ib.NodeID == nil {
						needRestart = true
					} else {
						markDirty = true
					}
				}
			} else if ib.NodeID == nil {
				needRestart = true
			} else if dirty {
				markDirty = true
			}
		} else {
			logger.Debug("DelInbound: skipping runtime push for disabled local inbound id:", id)
		}
	} else {
		logger.Debug("DelInbound: inbound not found, id:", id)
	}

	if err := s.clientService.DetachInbound(db, id); err != nil {
		return false, err
	}

	if err := db.Delete(model.Inbound{}, id).Error; err != nil {
		return needRestart, err
	}
	if markDirty && ib.NodeID != nil {
		if dErr := (&NodeService{}).MarkNodeDirty(*ib.NodeID); dErr != nil {
			logger.Warning("mark node dirty failed:", dErr)
		}
	}
	if !database.IsPostgres() {
		var count int64
		if err := db.Model(&model.Inbound{}).Count(&count).Error; err != nil {
			return needRestart, err
		}
		if count == 0 {
			if err := db.Exec("DELETE FROM sqlite_sequence WHERE name = ?", "inbounds").Error; err != nil {
				return needRestart, err
			}
		}
	}
	return needRestart, nil
}

type BulkDelInboundResult struct {
	Deleted int                    `json:"deleted"`
	Skipped []BulkDelInboundReport `json:"skipped,omitempty"`
}

type BulkDelInboundReport struct {
	Id     int    `json:"id"`
	Reason string `json:"reason"`
}

// DelInbounds removes every inbound in the list, reusing the single-delete
// path per id. Failures are recorded in Skipped and processing continues for
// the rest; the aggregated needRestart is returned so the caller restarts
// xray at most once.
func (s *InboundService) DelInbounds(ids []int) (BulkDelInboundResult, bool, error) {
	result := BulkDelInboundResult{}
	needRestart := false
	for _, id := range ids {
		r, err := s.DelInbound(id)
		if err != nil {
			result.Skipped = append(result.Skipped, BulkDelInboundReport{Id: id, Reason: err.Error()})
			continue
		}
		result.Deleted++
		if r {
			needRestart = true
		}
	}
	return result, needRestart, nil
}

func (s *InboundService) GetInbound(id int) (*model.Inbound, error) {
	db := database.GetDB()
	inbound := &model.Inbound{}
	err := db.Model(model.Inbound{}).First(inbound, id).Error
	if err != nil {
		return nil, err
	}
	return inbound, nil
}

func (s *InboundService) GetInboundDetail(id int) (*model.Inbound, error) {
	db := database.GetDB()
	inbound := &model.Inbound{}
	err := db.Model(model.Inbound{}).Preload("ClientStats").First(inbound, id).Error
	if err != nil {
		return nil, err
	}
	s.enrichClientStats(db, []*model.Inbound{inbound})
	return inbound, nil
}

func (s *InboundService) SetInboundEnable(id int, enable bool) (bool, error) {
	inbound, err := s.GetInbound(id)
	if err != nil {
		return false, err
	}
	if inbound.Enable == enable {
		return false, nil
	}

	db := database.GetDB()
	if err := db.Model(model.Inbound{}).Where("id = ?", id).
		Update("enable", enable).Error; err != nil {
		return false, err
	}
	inbound.Enable = enable

	needRestart := false
	rt, push, dirty, perr := s.nodePushPlan(inbound)
	if perr != nil {
		return false, perr
	}

	// Remote nodes interpret DelInbound as a real row delete (it hits
	// panel/api/inbounds/del/:id on the remote), so toggling the enable
	// switch on a remote inbound used to wipe the row entirely (#4402).
	// PATCH the remote row via UpdateInbound instead — preserves the
	// settings/client history and just flips the enable flag.
	if inbound.NodeID != nil {
		if push {
			if err := rt.UpdateInbound(context.Background(), inbound, inbound); err != nil {
				logger.Warning("SetInboundEnable: remote UpdateInbound on", rt.Name(), "failed:", err)
				dirty = true
			}
		}
		if dirty {
			if dErr := (&NodeService{}).MarkNodeDirty(*inbound.NodeID); dErr != nil {
				logger.Warning("mark node dirty failed:", dErr)
			}
		}
		return false, nil
	}

	if !push {
		return true, nil
	}

	if err := rt.DelInbound(context.Background(), inbound); err != nil &&
		!strings.Contains(err.Error(), "not found") {
		logger.Debug("SetInboundEnable: DelInbound on", rt.Name(), "failed:", err)
		needRestart = true
	}
	if !enable {
		return needRestart, nil
	}

	runtimeInbound, err := s.buildRuntimeInboundForAPI(db, inbound)
	if err != nil {
		logger.Debug("SetInboundEnable: build runtime config failed:", err)
		return true, nil
	}
	if err := rt.AddInbound(context.Background(), runtimeInbound); err != nil {
		logger.Debug("SetInboundEnable: AddInbound on", rt.Name(), "failed:", err)
		needRestart = true
	}
	return needRestart, nil
}

func (s *InboundService) UpdateInbound(inbound *model.Inbound) (*model.Inbound, bool, error) {
	// Normalize streamSettings based on protocol
	s.normalizeStreamSettings(inbound)

	conflict, err := s.checkPortConflict(inbound, inbound.Id)
	if err != nil {
		return inbound, false, err
	}
	if conflict != nil {
		return inbound, false, common.NewError(conflict.String())
	}

	oldInbound, err := s.GetInbound(inbound.Id)
	if err != nil {
		return inbound, false, err
	}
	inbound.NodeID = oldInbound.NodeID

	tag := oldInbound.Tag
	oldBits := inboundTransports(oldInbound.Protocol, oldInbound.StreamSettings, oldInbound.Settings)
	oldTagWasAuto := isAutoGeneratedTag(tag, oldInbound.Port, oldInbound.NodeID, oldBits)

	db := database.GetDB()
	tx := db.Begin()

	markDirty := false
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		tx.Commit()
		if markDirty && oldInbound.NodeID != nil {
			if dErr := (&NodeService{}).MarkNodeDirty(*oldInbound.NodeID); dErr != nil {
				logger.Warning("mark node dirty failed:", dErr)
			}
		}
	}()

	err = s.updateClientTraffics(tx, oldInbound, inbound)
	if err != nil {
		return inbound, false, err
	}

	// Ensure created_at and updated_at exist in inbound.Settings clients
	{
		var oldSettings map[string]any
		_ = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
		emailToCreated := map[string]int64{}
		emailToUpdated := map[string]int64{}
		if oldSettings != nil {
			if oc, ok := oldSettings["clients"].([]any); ok {
				for _, it := range oc {
					if m, ok2 := it.(map[string]any); ok2 {
						if email, ok3 := m["email"].(string); ok3 {
							switch v := m["created_at"].(type) {
							case float64:
								emailToCreated[email] = int64(v)
							case int64:
								emailToCreated[email] = v
							}
							switch v := m["updated_at"].(type) {
							case float64:
								emailToUpdated[email] = int64(v)
							case int64:
								emailToUpdated[email] = v
							}
						}
					}
				}
			}
		}
		var newSettings map[string]any
		if err2 := json.Unmarshal([]byte(inbound.Settings), &newSettings); err2 == nil && newSettings != nil {
			now := time.Now().Unix() * 1000
			if nSlice, ok := newSettings["clients"].([]any); ok {
				for i := range nSlice {
					if m, ok2 := nSlice[i].(map[string]any); ok2 {
						email, _ := m["email"].(string)
						if _, ok3 := m["created_at"]; !ok3 {
							if v, ok4 := emailToCreated[email]; ok4 && v > 0 {
								m["created_at"] = v
							} else {
								m["created_at"] = now
							}
						}
						// Preserve client's updated_at if present; do not bump on parent inbound update
						if _, hasUpdated := m["updated_at"]; !hasUpdated {
							if v, ok4 := emailToUpdated[email]; ok4 && v > 0 {
								m["updated_at"] = v
							}
						}
						nSlice[i] = m
					}
				}
				newSettings["clients"] = nSlice
				if bs, err3 := json.MarshalIndent(newSettings, "", "  "); err3 == nil {
					inbound.Settings = string(bs)
				}
			}
		}
	}

	oldInbound.Total = inbound.Total
	oldInbound.Remark = inbound.Remark
	oldInbound.Enable = inbound.Enable
	oldInbound.ExpiryTime = inbound.ExpiryTime
	oldInbound.TrafficReset = inbound.TrafficReset
	oldInbound.Listen = inbound.Listen
	oldInbound.Port = inbound.Port
	oldInbound.Protocol = inbound.Protocol
	oldInbound.Settings = inbound.Settings
	oldInbound.StreamSettings = inbound.StreamSettings
	oldInbound.Sniffing = inbound.Sniffing
	if oldTagWasAuto && inbound.Tag == tag {
		inbound.Tag = ""
	}
	oldInbound.Tag, err = s.resolveInboundTag(inbound, inbound.Id)
	if err != nil {
		return inbound, false, err
	}
	inbound.Tag = oldInbound.Tag

	needRestart := false
	rt, push, dirty, perr := s.nodePushPlan(oldInbound)
	if perr != nil {
		err = perr
		return inbound, false, err
	}
	if dirty {
		markDirty = true
	}
	if oldInbound.NodeID == nil {
		if !push {
			needRestart = true
		} else {
			oldSnapshot := *oldInbound
			oldSnapshot.Tag = tag
			if err2 := rt.DelInbound(context.Background(), &oldSnapshot); err2 == nil {
				logger.Debug("Old inbound deleted on", rt.Name(), ":", tag)
			}
			if inbound.Enable {
				runtimeInbound, err2 := s.buildRuntimeInboundForAPI(tx, oldInbound)
				if err2 != nil {
					logger.Debug("Unable to prepare runtime inbound config:", err2)
					needRestart = true
				} else if err2 := rt.AddInbound(context.Background(), runtimeInbound); err2 == nil {
					logger.Debug("Updated inbound added on", rt.Name(), ":", oldInbound.Tag)
				} else {
					logger.Debug("Unable to update inbound on", rt.Name(), ":", err2)
					needRestart = true
				}
			}
		}
	} else if push {
		oldSnapshot := *oldInbound
		oldSnapshot.Tag = tag
		if !inbound.Enable {
			if err2 := rt.DelInbound(context.Background(), &oldSnapshot); err2 != nil {
				logger.Warning("Unable to disable inbound on", rt.Name(), ":", err2)
				markDirty = true
			}
		} else if err2 := rt.UpdateInbound(context.Background(), &oldSnapshot, oldInbound); err2 != nil {
			logger.Warning("Unable to update inbound on", rt.Name(), ":", err2)
			markDirty = true
		}
	}

	if err = tx.Save(oldInbound).Error; err != nil {
		return inbound, false, err
	}
	newClients, gcErr := s.GetClients(oldInbound)
	if gcErr != nil {
		err = gcErr
		return inbound, false, err
	}
	if err = s.clientService.SyncInbound(tx, oldInbound.Id, newClients); err != nil {
		return inbound, false, err
	}
	return inbound, needRestart, nil
}

func (s *InboundService) buildRuntimeInboundForAPI(tx *gorm.DB, inbound *model.Inbound) (*model.Inbound, error) {
	if inbound == nil {
		return nil, fmt.Errorf("inbound is nil")
	}

	runtimeInbound := *inbound
	settings := map[string]any{}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return nil, err
	}

	clients, ok := settings["clients"].([]any)
	if !ok {
		return &runtimeInbound, nil
	}

	var clientStats []xray.ClientTraffic
	err := tx.Model(xray.ClientTraffic{}).
		Where("inbound_id = ?", inbound.Id).
		Select("email", "enable").
		Find(&clientStats).Error
	if err != nil {
		return nil, err
	}

	enableMap := make(map[string]bool, len(clientStats))
	for _, clientTraffic := range clientStats {
		enableMap[clientTraffic.Email] = clientTraffic.Enable
	}

	finalClients := make([]any, 0, len(clients))
	for _, client := range clients {
		c, ok := client.(map[string]any)
		if !ok {
			continue
		}

		email, _ := c["email"].(string)
		if enable, exists := enableMap[email]; exists && !enable {
			continue
		}

		if manualEnable, ok := c["enable"].(bool); ok && !manualEnable {
			continue
		}

		finalClients = append(finalClients, c)
	}

	settings["clients"] = finalClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	runtimeInbound.Settings = string(modifiedSettings)

	return &runtimeInbound, nil
}

// updateClientTraffics syncs the ClientTraffic rows with the inbound's clients
// list: removes rows for emails that disappeared, inserts rows for newly-added
// emails. Uses sets for O(N) lookup — the previous nested-loop implementation
// was O(N²) and degraded into multi-second pauses on inbounds with thousands
// of clients (toggling, saving, or deleting any such inbound felt frozen).
func (s *InboundService) updateClientTraffics(tx *gorm.DB, oldInbound *model.Inbound, newInbound *model.Inbound) error {
	oldClients, err := s.GetClients(oldInbound)
	if err != nil {
		return err
	}
	newClients, err := s.GetClients(newInbound)
	if err != nil {
		return err
	}

	// Email is the unique key for ClientTraffic rows. Clients without an
	// email have no stats row to sync — skip them on both sides instead of
	// risking a unique-constraint hit or accidental delete of an unrelated row.
	oldEmails := make(map[string]struct{}, len(oldClients))
	for i := range oldClients {
		if oldClients[i].Email == "" {
			continue
		}
		oldEmails[oldClients[i].Email] = struct{}{}
	}
	newEmails := make(map[string]struct{}, len(newClients))
	for i := range newClients {
		if newClients[i].Email == "" {
			continue
		}
		newEmails[newClients[i].Email] = struct{}{}
	}

	// Drop stats rows for removed emails — but not when a sibling inbound
	// still references the email, since the row is the shared accumulator.
	for i := range oldClients {
		email := oldClients[i].Email
		if email == "" {
			continue
		}
		if _, kept := newEmails[email]; kept {
			continue
		}
		stillUsed, err := s.emailUsedByOtherInbounds(email, oldInbound.Id)
		if err != nil {
			return err
		}
		if stillUsed {
			continue
		}
		if err := s.DelClientStat(tx, email); err != nil {
			return err
		}
		// Keep inbound_client_ips in sync when the inbound edit drops an
		// email, so the IP-limit job doesn't keep a ghost tracking row (#4963).
		if err := s.DelClientIPs(tx, email); err != nil {
			return err
		}
	}
	for i := range newClients {
		email := newClients[i].Email
		if email == "" {
			continue
		}
		if _, existed := oldEmails[email]; existed {
			if err := s.UpdateClientStat(tx, email, &newClients[i]); err != nil {
				return err
			}
			continue
		}
		if err := s.AddClientStat(tx, oldInbound.Id, &newClients[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *InboundService) getClientPrimaryKey(protocol model.Protocol, client model.Client) string {
	switch protocol {
	case model.Trojan:
		return client.Password
	case model.Shadowsocks:
		return client.Email
	case model.Hysteria:
		return client.Auth
	default:
		return client.ID
	}
}

func (s *InboundService) writeBackClientSubID(sourceInboundID int, sourceProtocol model.Protocol, client model.Client, subID string) (bool, error) {
	client.SubID = subID
	client.UpdatedAt = time.Now().UnixMilli()
	clientID := s.getClientPrimaryKey(sourceProtocol, client)
	if clientID == "" {
		return false, common.NewError("empty client ID")
	}

	settingsBytes, err := json.Marshal(map[string][]model.Client{
		"clients": {client},
	})
	if err != nil {
		return false, err
	}

	updatePayload := &model.Inbound{
		Id:       sourceInboundID,
		Settings: string(settingsBytes),
	}
	return s.clientService.UpdateInboundClient(s, updatePayload, clientID)
}

func (s *InboundService) generateRandomCredential(targetProtocol model.Protocol) string {
	switch targetProtocol {
	case model.VMESS, model.VLESS:
		return uuid.NewString()
	default:
		return strings.ReplaceAll(uuid.NewString(), "-", "")
	}
}

func (s *InboundService) buildTargetClientFromSource(source model.Client, targetInbound *model.Inbound, email string, flow string) (model.Client, error) {
	nowTs := time.Now().UnixMilli()
	target := source
	target.Email = email
	target.CreatedAt = nowTs
	target.UpdatedAt = nowTs

	target.ID = ""
	target.Password = ""
	target.Auth = ""
	target.Flow = ""

	targetProtocol := targetInbound.Protocol
	switch targetProtocol {
	case model.VMESS:
		target.ID = s.generateRandomCredential(targetProtocol)
	case model.VLESS:
		target.ID = s.generateRandomCredential(targetProtocol)
		if (flow == "xtls-rprx-vision" || flow == "xtls-rprx-vision-udp443") &&
			inboundCanEnableTlsFlow(string(targetProtocol), targetInbound.StreamSettings) {
			target.Flow = flow
		}
	case model.Trojan, model.Shadowsocks:
		target.Password = s.generateRandomCredential(targetProtocol)
	case model.Hysteria:
		target.Auth = s.generateRandomCredential(targetProtocol)
	default:
		target.ID = s.generateRandomCredential(targetProtocol)
	}

	return target, nil
}

func (s *InboundService) nextAvailableCopiedEmail(originalEmail string, targetID int, occupied map[string]struct{}) string {
	base := fmt.Sprintf("%s_%d", originalEmail, targetID)
	candidate := base
	suffix := 0
	for {
		if _, exists := occupied[strings.ToLower(candidate)]; !exists {
			occupied[strings.ToLower(candidate)] = struct{}{}
			return candidate
		}
		suffix++
		candidate = fmt.Sprintf("%s_%d", base, suffix)
	}
}

func (s *InboundService) CopyInboundClients(targetInboundID int, sourceInboundID int, clientEmails []string, flow string) (*CopyClientsResult, bool, error) {
	result := &CopyClientsResult{
		Added:   []string{},
		Skipped: []string{},
		Errors:  []string{},
	}
	if targetInboundID == sourceInboundID {
		return result, false, common.NewError("source and target inbounds must be different")
	}

	targetInbound, err := s.GetInbound(targetInboundID)
	if err != nil {
		return result, false, err
	}
	sourceInbound, err := s.GetInbound(sourceInboundID)
	if err != nil {
		return result, false, err
	}

	sourceClients, err := s.GetClients(sourceInbound)
	if err != nil {
		return result, false, err
	}
	if len(sourceClients) == 0 {
		return result, false, nil
	}

	allowedEmails := map[string]struct{}{}
	if len(clientEmails) > 0 {
		for _, email := range clientEmails {
			allowedEmails[strings.ToLower(strings.TrimSpace(email))] = struct{}{}
		}
	}

	occupiedEmails := map[string]struct{}{}
	allEmails, err := s.getAllEmails()
	if err != nil {
		return result, false, err
	}
	for _, email := range allEmails {
		clean := strings.Trim(email, "\"")
		if clean != "" {
			occupiedEmails[strings.ToLower(clean)] = struct{}{}
		}
	}

	newClients := make([]model.Client, 0)
	needRestart := false
	for _, sourceClient := range sourceClients {
		originalEmail := strings.TrimSpace(sourceClient.Email)
		if originalEmail == "" {
			continue
		}
		if len(allowedEmails) > 0 {
			if _, ok := allowedEmails[strings.ToLower(originalEmail)]; !ok {
				continue
			}
		}

		if sourceClient.SubID == "" {
			newSubID := uuid.NewString()
			subNeedRestart, subErr := s.writeBackClientSubID(sourceInbound.Id, sourceInbound.Protocol, sourceClient, newSubID)
			if subErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to write source subId: %v", originalEmail, subErr))
				continue
			}
			if subNeedRestart {
				needRestart = true
			}
			sourceClient.SubID = newSubID
		}

		targetEmail := s.nextAvailableCopiedEmail(originalEmail, targetInboundID, occupiedEmails)
		targetClient, buildErr := s.buildTargetClientFromSource(sourceClient, targetInbound, targetEmail, flow)
		if buildErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", originalEmail, buildErr))
			continue
		}
		newClients = append(newClients, targetClient)
		result.Added = append(result.Added, targetEmail)
	}

	if len(newClients) == 0 {
		return result, needRestart, nil
	}

	settingsPayload, err := json.Marshal(map[string][]model.Client{
		"clients": newClients,
	})
	if err != nil {
		return result, needRestart, err
	}

	addNeedRestart, err := s.clientService.AddInboundClient(s, &model.Inbound{
		Id:       targetInboundID,
		Settings: string(settingsPayload),
	})
	if err != nil {
		return result, needRestart, err
	}
	if addNeedRestart {
		needRestart = true
	}

	return result, needRestart, nil
}

const resetGracePeriodMs int64 = 30000

// onlineGracePeriodMs must comfortably exceed the 5s traffic-poll interval —
// Xray's stats counters often report a zero delta for an active session across
// a single poll, so a 5s grace would still drop the client on the next tick.
// ~4 polls of slack keeps idle-but-connected clients visible without lingering
// long after a real disconnect.
const onlineGracePeriodMs int64 = 20000

type nodeTrafficCounter struct {
	Up   int64
	Down int64
}

func (s *InboundService) upsertNodeBaseline(tx *gorm.DB, nodeID int, email string, up, down int64) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "node_id"}, {Name: "email"}},
		DoUpdates: clause.AssignmentColumns([]string{"up", "down"}),
	}).Create(&model.NodeClientTraffic{NodeId: nodeID, Email: email, Up: up, Down: down}).Error
}

func (s *InboundService) SetRemoteTraffic(nodeID int, snap *runtime.TrafficSnapshot, dirty bool) (bool, error) {
	var structuralChange bool
	err := submitTrafficWrite(func() error {
		var inner error
		structuralChange, inner = s.setRemoteTrafficLocked(nodeID, snap, dirty)
		return inner
	})
	return structuralChange, err
}

func (s *InboundService) setRemoteTrafficLocked(nodeID int, snap *runtime.TrafficSnapshot, dirty bool) (bool, error) {
	if snap == nil || nodeID <= 0 {
		return false, nil
	}
	db := database.GetDB()
	now := time.Now().UnixMilli()

	// originGuidFor attributes a synced inbound to the panel that physically
	// hosts it: inbounds the node forwards from its own sub-nodes already carry
	// a non-empty OriginNodeGuid (kept as-is across hops); the node's own local
	// inbounds report empty, so they are attributed to the node's own GUID. An
	// empty result (old-build node with no GUID yet) leaves attribution to the
	// node_id fallback downstream (#4983).
	var nodeRow model.Node
	db.Select("guid").Where("id = ?", nodeID).First(&nodeRow)
	originGuidFor := func(snapIb *model.Inbound) string {
		if snapIb.OriginNodeGuid != "" {
			return snapIb.OriginNodeGuid
		}
		return nodeRow.Guid
	}

	var central []model.Inbound
	if err := db.Model(model.Inbound{}).
		Where("node_id = ?", nodeID).
		Find(&central).Error; err != nil {
		return false, err
	}
	// Index under the stored tag and its prefix-flipped form so a snap matches
	// whether the n<id>- prefix lives on the node side, the central side, or
	// neither — a mismatch must never spawn a duplicate central inbound.
	tagToCentral := make(map[string]*model.Inbound, len(central)*2)
	prefix := nodeTagPrefix(&nodeID)
	for i := range central {
		tagToCentral[central[i].Tag] = &central[i]
		if prefix != "" {
			if stripped, found := strings.CutPrefix(central[i].Tag, prefix); found {
				tagToCentral[stripped] = &central[i]
			} else {
				tagToCentral[prefix+central[i].Tag] = &central[i]
			}
		}
	}

	var centralClientStats []xray.ClientTraffic
	if len(central) > 0 {
		ids := make([]int, 0, len(central))
		for i := range central {
			ids = append(ids, central[i].Id)
		}
		if err := db.Model(xray.ClientTraffic{}).
			Where("inbound_id IN ?", ids).
			Find(&centralClientStats).Error; err != nil {
			return false, err
		}
	}
	type csKey struct {
		inboundID int
		email     string
	}
	centralCS := make(map[csKey]*xray.ClientTraffic, len(centralClientStats))
	centralCSByEmail := make(map[string]*xray.ClientTraffic, len(centralClientStats))
	for i := range centralClientStats {
		centralCS[csKey{centralClientStats[i].InboundId, centralClientStats[i].Email}] = &centralClientStats[i]
		centralCSByEmail[centralClientStats[i].Email] = &centralClientStats[i]
	}

	nodeBaselines := make(map[string]nodeTrafficCounter)
	var baselineRows []model.NodeClientTraffic
	if err := db.Model(&model.NodeClientTraffic{}).
		Where("node_id = ?", nodeID).
		Find(&baselineRows).Error; err != nil {
		return false, err
	}
	for i := range baselineRows {
		nodeBaselines[baselineRows[i].Email] = nodeTrafficCounter{Up: baselineRows[i].Up, Down: baselineRows[i].Down}
	}

	var existingEmailsList []string
	if err := db.Model(xray.ClientTraffic{}).Pluck("email", &existingEmailsList).Error; err != nil {
		return false, err
	}
	existingEmails := make(map[string]struct{}, len(existingEmailsList))
	for _, e := range existingEmailsList {
		existingEmails[e] = struct{}{}
	}

	var defaultUserId int
	if len(central) > 0 {
		defaultUserId = central[0].UserId
	} else {
		var u model.User
		if err := db.Model(model.User{}).Order("id asc").First(&u).Error; err == nil {
			defaultUserId = u.Id
		} else {
			defaultUserId = 1
		}
	}

	tx := db.Begin()
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	structuralChange := false

	snapTags := make(map[string]struct{}, len(snap.Inbounds))
	for _, snapIb := range snap.Inbounds {
		if snapIb == nil {
			continue
		}
		snapTags[snapIb.Tag] = struct{}{}
		// Record the prefix-flipped form too so the orphan sweep below keeps a
		// central inbound whether its tag carries the n<id>- prefix or not.
		if prefix != "" {
			if stripped, found := strings.CutPrefix(snapIb.Tag, prefix); found {
				snapTags[stripped] = struct{}{}
			} else {
				snapTags[prefix+snapIb.Tag] = struct{}{}
			}
		}

		c, ok := tagToCentral[snapIb.Tag]
		if !ok {
			if dirty {
				continue
			}
			// Try snap.Tag first; on collision fall back to the n<id>-
			// prefixed form so local+node can both own the same port.
			pickFreeTag := func() (string, error) {
				candidates := []string{snapIb.Tag}
				if prefix != "" && !strings.HasPrefix(snapIb.Tag, prefix) {
					candidates = append(candidates, prefix+snapIb.Tag)
				}
				for _, t := range candidates {
					var owner model.Inbound
					err := tx.Where("tag = ?", t).First(&owner).Error
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return t, nil
					}
					if err != nil {
						return "", err
					}
				}
				return "", nil
			}
			chosenTag, err := pickFreeTag()
			if err != nil {
				logger.Warningf("setRemoteTraffic: check tag %q failed: %v", snapIb.Tag, err)
				continue
			}
			if chosenTag == "" {
				key := fmt.Sprintf("%d:%s", nodeID, snapIb.Tag)
				if _, seen := reportedRemoteTagConflict.LoadOrStore(key, struct{}{}); !seen {
					logger.Warningf(
						"setRemoteTraffic: tag %q from node %d collides with an existing inbound even after the n%d- prefix — skipping (rename one side to remove the duplicate)",
						snapIb.Tag, nodeID, nodeID,
					)
				}
				continue
			}
			newIb := model.Inbound{
				UserId:         defaultUserId,
				NodeID:         &nodeID,
				OriginNodeGuid: originGuidFor(snapIb),
				Tag:            chosenTag,
				Listen:         snapIb.Listen,
				Port:           snapIb.Port,
				Protocol:       snapIb.Protocol,
				Settings:       snapIb.Settings,
				StreamSettings: snapIb.StreamSettings,
				Sniffing:       snapIb.Sniffing,
				TrafficReset:   snapIb.TrafficReset,
				Enable:         snapIb.Enable,
				Remark:         snapIb.Remark,
				Total:          snapIb.Total,
				ExpiryTime:     snapIb.ExpiryTime,
				Up:             snapIb.Up,
				Down:           snapIb.Down,
			}
			if err := tx.Create(&newIb).Error; err != nil {
				logger.Warningf("setRemoteTraffic: create central inbound for tag %q failed: %v", snapIb.Tag, err)
				continue
			}
			tagToCentral[snapIb.Tag] = &newIb
			if newIb.Tag != snapIb.Tag {
				tagToCentral[newIb.Tag] = &newIb
			}
			structuralChange = true
			continue
		}

		inGrace := c.LastTrafficResetTime > 0 && now-c.LastTrafficResetTime < resetGracePeriodMs

		updates := map[string]any{}
		if !dirty {
			updates["enable"] = snapIb.Enable
			updates["remark"] = snapIb.Remark
			updates["listen"] = snapIb.Listen
			updates["port"] = snapIb.Port
			updates["protocol"] = snapIb.Protocol
			updates["total"] = snapIb.Total
			updates["expiry_time"] = snapIb.ExpiryTime
			updates["settings"] = snapIb.Settings
			updates["stream_settings"] = snapIb.StreamSettings
			updates["sniffing"] = snapIb.Sniffing
			updates["traffic_reset"] = snapIb.TrafficReset
		}
		if !inGrace || (snapIb.Up+snapIb.Down) <= (c.Up+c.Down) {
			updates["up"] = snapIb.Up
			updates["down"] = snapIb.Down
		}
		// Physical-home attribution is independent of config-dirty state, so
		// keep it current even while the node has pending offline edits. Writes
		// once to backfill an existing row, then stays equal (#4983).
		if og := originGuidFor(snapIb); c.OriginNodeGuid != og {
			updates["origin_node_guid"] = og
		}

		if !dirty && (c.Settings != snapIb.Settings ||
			c.Remark != snapIb.Remark ||
			c.Listen != snapIb.Listen ||
			c.Port != snapIb.Port ||
			c.Total != snapIb.Total ||
			c.ExpiryTime != snapIb.ExpiryTime ||
			c.Enable != snapIb.Enable) {
			structuralChange = true
		}

		if len(updates) > 0 {
			if err := tx.Model(model.Inbound{}).
				Where("id = ?", c.Id).
				Updates(updates).Error; err != nil {
				return false, err
			}
		}
	}

	for _, c := range central {
		if dirty {
			continue
		}
		if _, kept := snapTags[c.Tag]; kept {
			continue
		}
		var goneEmails []string
		if err := tx.Model(xray.ClientTraffic{}).
			Where("inbound_id = ?", c.Id).
			Pluck("email", &goneEmails).Error; err != nil {
			return false, err
		}
		if len(goneEmails) > 0 {
			if err := tx.Where("node_id = ? AND email IN ?", nodeID, goneEmails).
				Delete(&model.NodeClientTraffic{}).Error; err != nil {
				return false, err
			}
		}
		if err := tx.Where("inbound_id = ?", c.Id).
			Delete(&xray.ClientTraffic{}).Error; err != nil {
			return false, err
		}
		if err := s.clientService.DetachInbound(tx, c.Id); err != nil {
			return false, err
		}
		if err := tx.Where("id = ?", c.Id).
			Delete(&model.Inbound{}).Error; err != nil {
			return false, err
		}
		delete(tagToCentral, c.Tag)
		structuralChange = true
	}

	for _, snapIb := range snap.Inbounds {
		if snapIb == nil {
			continue
		}
		c, ok := tagToCentral[snapIb.Tag]
		if !ok {
			continue
		}
		snapEmails := make(map[string]struct{}, len(snapIb.ClientStats))
		for _, cs := range snapIb.ClientStats {
			snapEmails[cs.Email] = struct{}{}

			base, seen := nodeBaselines[cs.Email]
			var deltaUp, deltaDown int64
			if seen {
				if deltaUp = cs.Up - base.Up; deltaUp < 0 {
					deltaUp = cs.Up
				}
				if deltaDown = cs.Down - base.Down; deltaDown < 0 {
					deltaDown = cs.Down
				}
			}

			if _, rowExists := existingEmails[cs.Email]; !rowExists {
				if dirty {
					continue
				}
				row := &xray.ClientTraffic{
					InboundId:  c.Id,
					Email:      cs.Email,
					Enable:     cs.Enable,
					Total:      cs.Total,
					ExpiryTime: cs.ExpiryTime,
					Reset:      cs.Reset,
					Up:         cs.Up,
					Down:       cs.Down,
					LastOnline: cs.LastOnline,
				}
				if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "email"}}, DoNothing: true}).
					Create(row).Error; err != nil {
					return false, err
				}
				centralCS[csKey{c.Id, cs.Email}] = row
				centralCSByEmail[cs.Email] = row
				existingEmails[cs.Email] = struct{}{}
				structuralChange = true
				if err := s.upsertNodeBaseline(tx, nodeID, cs.Email, cs.Up, cs.Down); err != nil {
					return false, err
				}
				nodeBaselines[cs.Email] = nodeTrafficCounter{Up: cs.Up, Down: cs.Down}
				continue
			}

			if existing := centralCSByEmail[cs.Email]; existing != nil &&
				(existing.Enable != cs.Enable ||
					existing.Total != cs.Total ||
					existing.ExpiryTime != cs.ExpiryTime ||
					existing.Reset != cs.Reset) {
				structuralChange = true
			}

			// Only allow the node to disable a client (cs.Enable=false), never
			// to re-enable one the panel has already disabled. A stale snapshot
			// from the node arriving after a central disable would otherwise
			// overwrite enable=false back to true, letting the client accumulate
			// far more traffic than their limit before being disabled again.
			enableExpr := "enable AND ?"
			if err := tx.Exec(
				fmt.Sprintf(
					`UPDATE client_traffics
					 SET up = up + ?, down = down + ?, enable = %s, total = ?, expiry_time = ?, reset = ?,
					     last_online = %s
					 WHERE email = ?`,
					enableExpr,
					database.GreatestExpr("last_online", "?"),
				),
				deltaUp, deltaDown, cs.Enable, cs.Total, cs.ExpiryTime, cs.Reset,
				cs.LastOnline, cs.Email,
			).Error; err != nil {
				return false, err
			}
			if err := s.upsertNodeBaseline(tx, nodeID, cs.Email, cs.Up, cs.Down); err != nil {
				return false, err
			}
			nodeBaselines[cs.Email] = nodeTrafficCounter{Up: cs.Up, Down: cs.Down}
		}

		for k, existing := range centralCS {
			if dirty {
				continue
			}
			if k.inboundID != c.Id {
				continue
			}
			if _, kept := snapEmails[k.email]; kept {
				continue
			}
			if err := tx.Where("node_id = ? AND email = ?", nodeID, existing.Email).
				Delete(&model.NodeClientTraffic{}).Error; err != nil {
				return false, err
			}
			if err := tx.Where("inbound_id = ? AND email = ?", c.Id, existing.Email).
				Delete(&xray.ClientTraffic{}).Error; err != nil {
				return false, err
			}
			structuralChange = true
		}
	}

	type oldSet struct {
		inboundID int
		emails    map[string]struct{}
	}
	var perInboundOld []oldSet
	for _, snapIb := range snap.Inbounds {
		if snapIb == nil {
			continue
		}
		c, ok := tagToCentral[snapIb.Tag]
		if !ok {
			continue
		}
		if dirty {
			continue
		}
		var oldEmailsRows []string
		if err := tx.Table("clients").
			Joins("JOIN client_inbounds ON client_inbounds.client_id = clients.id").
			Where("client_inbounds.inbound_id = ?", c.Id).
			Pluck("email", &oldEmailsRows).Error; err == nil {
			oldEmails := make(map[string]struct{}, len(oldEmailsRows))
			for _, e := range oldEmailsRows {
				if e != "" {
					oldEmails[e] = struct{}{}
				}
			}
			perInboundOld = append(perInboundOld, oldSet{inboundID: c.Id, emails: oldEmails})
		}

		clients, gcErr := s.GetClients(snapIb)
		if gcErr != nil {
			logger.Warningf("setRemoteTraffic: parse clients for tag %q failed: %v", snapIb.Tag, gcErr)
			continue
		}
		csEnableByEmail := make(map[string]bool, len(snapIb.ClientStats))
		for _, cs := range snapIb.ClientStats {
			csEnableByEmail[cs.Email] = cs.Enable
		}
		filtered := clients[:0]
		for i := range clients {
			if isClientEmailTombstoned(clients[i].Email) {
				continue
			}
			if cse, hit := csEnableByEmail[clients[i].Email]; hit && !cse {
				clients[i].Enable = false
			}
			filtered = append(filtered, clients[i])
		}
		localEmails := make([]string, 0, len(filtered))
		for i := range filtered {
			if filtered[i].Email != "" {
				localEmails = append(localEmails, filtered[i].Email)
			}
		}
		if len(localEmails) > 0 {
			var localMeta []struct {
				Email   string
				Comment string `gorm:"column:comment"`
			}
			if err := tx.Table("clients").
				Select("email, comment").
				Where("email IN ?", localEmails).
				Find(&localMeta).Error; err == nil {
				commentByEmail := make(map[string]string, len(localMeta))
				for _, m := range localMeta {
					commentByEmail[m.Email] = m.Comment
				}
				for i := range filtered {
					if cmt, ok := commentByEmail[filtered[i].Email]; ok {
						filtered[i].Comment = cmt
					}
				}
			}
		}
		if err := s.clientService.SyncInbound(tx, c.Id, filtered); err != nil {
			logger.Warningf("setRemoteTraffic: sync clients for tag %q failed: %v", snapIb.Tag, err)
		}
	}

	for _, old := range perInboundOld {
		var stillAttached []string
		if err := tx.Table("clients").
			Joins("JOIN client_inbounds ON client_inbounds.client_id = clients.id").
			Where("client_inbounds.inbound_id = ?", old.inboundID).
			Pluck("email", &stillAttached).Error; err != nil {
			continue
		}
		stillSet := make(map[string]struct{}, len(stillAttached))
		for _, e := range stillAttached {
			stillSet[e] = struct{}{}
		}
		for email := range old.emails {
			if _, kept := stillSet[email]; kept {
				continue
			}
			var attachmentCount int64
			if err := tx.Table("client_inbounds").
				Joins("JOIN clients ON clients.id = client_inbounds.client_id").
				Where("clients.email = ?", email).
				Count(&attachmentCount).Error; err != nil {
				continue
			}
			if attachmentCount > 0 {
				continue
			}
			if err := tx.Where("email = ?", email).Delete(&model.ClientRecord{}).Error; err != nil {
				logger.Warningf("setRemoteTraffic: delete ClientRecord %q failed: %v", email, err)
			}
			if err := tx.Where("email = ?", email).Delete(&xray.ClientTraffic{}).Error; err != nil {
				logger.Warningf("setRemoteTraffic: delete ClientTraffic %q failed: %v", email, err)
			}
			if err := tx.Where("email = ?", email).Delete(&model.NodeClientTraffic{}).Error; err != nil {
				logger.Warningf("setRemoteTraffic: delete NodeClientTraffic %q failed: %v", email, err)
			}
			structuralChange = true
		}
	}

	if err := tx.Commit().Error; err != nil {
		return false, err
	}
	committed = true

	if p != nil {
		tree := snap.OnlineTree
		if len(tree) == 0 && len(snap.OnlineEmails) > 0 {
			// Old-build node (no GUID tree): key its flat online list under its
			// own effective identity so attribution still works for that branch.
			effectiveGuid := nodeRow.Guid
			if effectiveGuid == "" {
				effectiveGuid = synthNodeGuid(nodeID)
			}
			tree = map[string][]string{effectiveGuid: snap.OnlineEmails}
		}
		p.SetNodeOnlineTree(nodeID, tree)
	}

	return structuralChange, nil
}

func (s *InboundService) AddTraffic(inboundTraffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) (needRestart bool, clientsDisabled bool, err error) {
	var disabledNodeIDs []int
	err = submitTrafficWrite(func() error {
		var inner error
		needRestart, clientsDisabled, disabledNodeIDs, inner = s.addTrafficLocked(inboundTraffics, clientTraffics)
		return inner
	})
	if err == nil && len(disabledNodeIDs) > 0 {
		s.restartRemoteNodesOnDisable(disabledNodeIDs)
	}
	return
}

func (s *InboundService) addTrafficLocked(inboundTraffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) (bool, bool, []int, error) {
	var err error
	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	err = s.addInboundTraffic(tx, inboundTraffics)
	if err != nil {
		return false, false, nil, err
	}
	err = s.addClientTraffic(tx, clientTraffics)
	if err != nil {
		return false, false, nil, err
	}

	needRestart0, count, err := s.autoRenewClients(tx)
	if err != nil {
		logger.Warning("Error in renew clients:", err)
	} else if count > 0 {
		logger.Debugf("%v clients renewed", count)
	}

	disabledClientsCount := int64(0)
	needRestart1, count, disabledNodeIDs, err := s.disableInvalidClients(tx)
	if err != nil {
		logger.Warning("Error in disabling invalid clients:", err)
	} else if count > 0 {
		logger.Debugf("%v clients disabled", count)
		disabledClientsCount = count
	}

	needRestart2, count, err := s.disableInvalidInbounds(tx)
	if err != nil {
		logger.Warning("Error in disabling invalid inbounds:", err)
	} else if count > 0 {
		logger.Debugf("%v inbounds disabled", count)
	}
	return needRestart0 || needRestart1 || needRestart2, disabledClientsCount > 0, disabledNodeIDs, nil
}

func (s *InboundService) addInboundTraffic(tx *gorm.DB, traffics []*xray.Traffic) error {
	if len(traffics) == 0 {
		return nil
	}

	var err error

	for _, traffic := range traffics {
		if traffic.IsInbound {
			err = tx.Model(&model.Inbound{}).Where("tag = ? AND node_id IS NULL", traffic.Tag).
				Updates(map[string]any{
					"up":   gorm.Expr("up + ?", traffic.Up),
					"down": gorm.Expr("down + ?", traffic.Down),
				}).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *InboundService) addClientTraffic(tx *gorm.DB, traffics []*xray.ClientTraffic) (err error) {
	if len(traffics) == 0 {
		return nil
	}

	emails := make([]string, 0, len(traffics))
	for _, traffic := range traffics {
		emails = append(emails, traffic.Email)
	}
	dbClientTraffics := make([]*xray.ClientTraffic, 0, len(traffics))
	// Match purely by email. client_traffics is email-keyed (one shared row per
	// email regardless of how many inbounds the client is attached to), and these
	// emails come from the local xray's report, so they always belong to a client
	// attached to a local inbound. The old `inbound_id NOT IN (node inbounds)`
	// filter dropped the local traffic of a client attached to both a node and the
	// mother inbound whenever the node inbound happened to be attached first — its
	// shared row then carried the node inbound's id (AddClientStat uses OnConflict
	// DoNothing and never refreshes it), so the local poll skipped it entirely.
	err = tx.Model(xray.ClientTraffic{}).
		Where("email IN (?)", emails).
		Find(&dbClientTraffics).Error
	if err != nil {
		return err
	}

	// Avoid empty slice error
	if len(dbClientTraffics) == 0 {
		return nil
	}

	dbClientTraffics, err = s.adjustTraffics(tx, dbClientTraffics)
	if err != nil {
		return err
	}

	// Index by email for O(N) merge — the previous nested loop was O(N²)
	// and dominated each cron tick on inbounds with thousands of active
	// clients (7500 × 7500 = 56M string comparisons every 10 seconds).
	trafficByEmail := make(map[string]*xray.ClientTraffic, len(traffics))
	for i := range traffics {
		if traffics[i] != nil {
			trafficByEmail[traffics[i].Email] = traffics[i]
		}
	}
	now := time.Now().UnixMilli()
	for dbTraffic_index := range dbClientTraffics {
		t, ok := trafficByEmail[dbClientTraffics[dbTraffic_index].Email]
		if !ok {
			continue
		}
		dbClientTraffics[dbTraffic_index].Up += t.Up
		dbClientTraffics[dbTraffic_index].Down += t.Down
		if t.Up+t.Down > 0 {
			dbClientTraffics[dbTraffic_index].LastOnline = now
		}
	}

	err = tx.Save(dbClientTraffics).Error
	if err != nil {
		logger.Warning("AddClientTraffic update data ", err)
	}

	return nil
}

func (s *InboundService) adjustTraffics(tx *gorm.DB, dbClientTraffics []*xray.ClientTraffic) ([]*xray.ClientTraffic, error) {
	now := time.Now().UnixMilli()

	// "Start After First Use" stores a negative expiry (the duration). On the
	// first traffic tick it becomes an absolute deadline of now+duration. Compute
	// it once per email so every inbound the client is attached to lands on the
	// same value (recomputing per inbound would skip all but the first one).
	newExpiryByEmail := make(map[string]int64, len(dbClientTraffics))
	for traffic_index := range dbClientTraffics {
		if dbClientTraffics[traffic_index].ExpiryTime < 0 {
			newExpiryByEmail[dbClientTraffics[traffic_index].Email] = now - dbClientTraffics[traffic_index].ExpiryTime
		}
	}
	if len(newExpiryByEmail) == 0 {
		return dbClientTraffics, nil
	}

	delayedEmails := make([]string, 0, len(newExpiryByEmail))
	for email := range newExpiryByEmail {
		delayedEmails = append(delayedEmails, email)
	}

	// Resolve the owning inbounds through the client_inbounds link, which is
	// authoritative. client_traffics.inbound_id goes stale when an inbound is
	// deleted and recreated, which would leave the negative expiry unconverted.
	var inboundIds []int
	err := tx.Table("client_inbounds").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("clients.email IN (?)", delayedEmails).
		Distinct().
		Pluck("client_inbounds.inbound_id", &inboundIds).Error
	if err != nil {
		return nil, err
	}
	if len(inboundIds) == 0 {
		return dbClientTraffics, nil
	}

	var inbounds []*model.Inbound
	err = tx.Model(model.Inbound{}).Where("id IN (?)", inboundIds).Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	for inbound_index := range inbounds {
		settings := map[string]any{}
		json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
		clients, ok := settings["clients"].([]any)
		if ok {
			var newClients []any
			for client_index := range clients {
				c := clients[client_index].(map[string]any)
				email, _ := c["email"].(string)
				if newExpiry, ok := newExpiryByEmail[email]; ok {
					c["expiryTime"] = newExpiry
					c["updated_at"] = now
				}
				if _, ok := c["created_at"]; !ok {
					c["created_at"] = now
				}
				if _, ok := c["updated_at"]; !ok {
					c["updated_at"] = now
				}
				newClients = append(newClients, any(c))
			}
			settings["clients"] = newClients
			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return nil, err
			}

			inbounds[inbound_index].Settings = string(modifiedSettings)
		}
	}

	for traffic_index := range dbClientTraffics {
		if newExpiry, ok := newExpiryByEmail[dbClientTraffics[traffic_index].Email]; ok {
			dbClientTraffics[traffic_index].ExpiryTime = newExpiry
		}
	}

	err = tx.Save(inbounds).Error
	if err != nil {
		logger.Warning("AddClientTraffic update inbounds ", err)
		logger.Error(inbounds)
	} else {
		for _, ib := range inbounds {
			if ib == nil {
				continue
			}
			cs, gcErr := s.GetClients(ib)
			if gcErr != nil {
				logger.Warning("AddClientTraffic sync clients: GetClients failed", gcErr)
				continue
			}
			if syncErr := s.clientService.SyncInbound(tx, ib.Id, cs); syncErr != nil {
				logger.Warning("AddClientTraffic sync clients: SyncInbound failed", syncErr)
			}
		}
	}

	return dbClientTraffics, nil
}

func (s *InboundService) autoRenewClients(tx *gorm.DB) (bool, int64, error) {
	// check for time expired
	var traffics []*xray.ClientTraffic
	now := time.Now().Unix() * 1000
	var err, err1 error

	err = tx.Model(xray.ClientTraffic{}).
		Where("reset > 0 and expiry_time > 0 and expiry_time <= ?", now).
		Where("inbound_id NOT IN (?)", tx.Model(&model.Inbound{}).Select("id").Where("node_id IS NOT NULL")).
		Find(&traffics).Error
	if err != nil {
		return false, 0, err
	}
	// return if there is no client to renew
	if len(traffics) == 0 {
		return false, 0, nil
	}

	var inbound_ids []int
	var inbounds []*model.Inbound
	needRestart := false
	var clientsToAdd []struct {
		protocol string
		tag      string
		client   map[string]any
	}

	// Resolve the inbounds to renew through the client_inbounds link rather than
	// client_traffics.inbound_id, which goes stale after an inbound is deleted and
	// recreated and would otherwise skip the renew entirely.
	renewEmails := make([]string, 0, len(traffics))
	for _, traffic := range traffics {
		renewEmails = append(renewEmails, traffic.Email)
	}
	for _, batch := range chunkStrings(renewEmails, sqliteMaxVars) {
		var ids []int
		if err = tx.Table("client_inbounds").
			Joins("JOIN clients ON clients.id = client_inbounds.client_id").
			Where("clients.email IN ?", batch).
			Distinct().
			Pluck("client_inbounds.inbound_id", &ids).Error; err != nil {
			return false, 0, err
		}
		inbound_ids = append(inbound_ids, ids...)
	}
	// Dedupe so an inbound hosting N expired clients is fetched and saved once
	// per tick instead of N times across chunk boundaries.
	inbound_ids = uniqueInts(inbound_ids)
	// Chunked to stay under SQLite's bind-variable limit when many inbounds
	// are touched in a single tick.
	for _, batch := range chunkInts(inbound_ids, sqliteMaxVars) {
		var page []*model.Inbound
		if err = tx.Model(model.Inbound{}).Where("id IN ?", batch).Find(&page).Error; err != nil {
			return false, 0, err
		}
		inbounds = append(inbounds, page...)
	}
	for inbound_index := range inbounds {
		settings := map[string]any{}
		json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
		clients := settings["clients"].([]any)
		for client_index := range clients {
			c := clients[client_index].(map[string]any)
			for traffic_index, traffic := range traffics {
				if traffic.Email == c["email"].(string) {
					newExpiryTime := traffic.ExpiryTime
					for newExpiryTime < now {
						newExpiryTime += (int64(traffic.Reset) * 86400000)
					}
					c["expiryTime"] = newExpiryTime
					traffics[traffic_index].ExpiryTime = newExpiryTime
					traffics[traffic_index].Down = 0
					traffics[traffic_index].Up = 0
					if !traffic.Enable {
						traffics[traffic_index].Enable = true
						c["enable"] = true
						clientsToAdd = append(clientsToAdd,
							struct {
								protocol string
								tag      string
								client   map[string]any
							}{
								protocol: string(inbounds[inbound_index].Protocol),
								tag:      inbounds[inbound_index].Tag,
								client:   c,
							})
					}
					clients[client_index] = any(c)
					break
				}
			}
		}
		settings["clients"] = clients
		newSettings, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return false, 0, err
		}
		inbounds[inbound_index].Settings = string(newSettings)
	}
	err = tx.Save(inbounds).Error
	if err != nil {
		return false, 0, err
	}
	for _, ib := range inbounds {
		if ib == nil {
			continue
		}
		cs, gcErr := s.GetClients(ib)
		if gcErr != nil {
			logger.Warning("autoRenewClients sync clients: GetClients failed", gcErr)
			continue
		}
		if syncErr := s.clientService.SyncInbound(tx, ib.Id, cs); syncErr != nil {
			logger.Warning("autoRenewClients sync clients: SyncInbound failed", syncErr)
		}
	}
	err = tx.Save(traffics).Error
	if err != nil {
		return false, 0, err
	}
	if p != nil {
		err1 = s.xrayApi.Init(p.GetAPIPort())
		if err1 != nil {
			return true, int64(len(traffics)), nil
		}
		for _, clientToAdd := range clientsToAdd {
			err1 = s.xrayApi.AddUser(clientToAdd.protocol, clientToAdd.tag, clientToAdd.client)
			if err1 != nil {
				needRestart = true
			}
		}
		s.xrayApi.Close()
	}
	return needRestart, int64(len(traffics)), nil
}

func (s *InboundService) disableInvalidInbounds(tx *gorm.DB) (bool, int64, error) {
	now := time.Now().Unix() * 1000
	needRestart := false

	if p != nil {
		var tags []string
		err := tx.Table("inbounds").
			Select("inbounds.tag").
			Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ? and node_id IS NULL", now, true).
			Scan(&tags).Error
		if err != nil {
			return false, 0, err
		}
		s.xrayApi.Init(p.GetAPIPort())
		for _, tag := range tags {
			err1 := s.xrayApi.DelInbound(tag)
			if err1 == nil {
				logger.Debug("Inbound disabled by api:", tag)
			} else {
				logger.Debug("Error in disabling inbound by api:", err1)
				needRestart = true
			}
		}
		s.xrayApi.Close()
	}

	result := tx.Model(model.Inbound{}).
		Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ? and node_id IS NULL", now, true).
		Update("enable", false)
	err := result.Error
	count := result.RowsAffected
	return needRestart, count, err
}

func (s *InboundService) disableInvalidClients(tx *gorm.DB) (bool, int64, []int, error) {
	now := time.Now().Unix() * 1000
	needRestart := false

	var depletedRows []xray.ClientTraffic
	err := tx.Model(xray.ClientTraffic{}).
		Where("((total > 0 AND up + down >= total) OR (expiry_time > 0 AND expiry_time <= ?)) AND enable = ?", now, true).
		Find(&depletedRows).Error
	if err != nil {
		return false, 0, nil, err
	}
	if len(depletedRows) == 0 {
		return false, 0, nil, nil
	}

	depletedEmails := make([]string, 0, len(depletedRows))
	for i := range depletedRows {
		if depletedRows[i].Email == "" {
			continue
		}
		depletedEmails = append(depletedEmails, depletedRows[i].Email)
	}

	type target struct {
		InboundID int  `gorm:"column:inbound_id"`
		NodeID    *int `gorm:"column:node_id"`
		Tag       string
		Email     string
	}
	var targets []target
	if len(depletedEmails) > 0 {
		err = tx.Raw(`
			SELECT inbounds.id AS inbound_id, inbounds.node_id AS node_id,
			       inbounds.tag AS tag, clients.email AS email
			FROM clients
			JOIN client_inbounds ON client_inbounds.client_id = clients.id
			JOIN inbounds        ON inbounds.id = client_inbounds.inbound_id
			WHERE clients.email IN ?
		`, depletedEmails).Scan(&targets).Error
		if err != nil {
			return false, 0, nil, err
		}
	}

	var localTargets []target
	localByInbound := make(map[int]map[string]struct{})
	remoteByInbound := make(map[int][]target)
	for _, t := range targets {
		if t.NodeID == nil {
			localTargets = append(localTargets, t)
			if localByInbound[t.InboundID] == nil {
				localByInbound[t.InboundID] = make(map[string]struct{})
			}
			localByInbound[t.InboundID][t.Email] = struct{}{}
		} else {
			remoteByInbound[t.InboundID] = append(remoteByInbound[t.InboundID], t)
		}
	}

	if p != nil && len(localTargets) > 0 {
		s.xrayApi.Init(p.GetAPIPort())
		for _, t := range localTargets {
			err1 := s.xrayApi.RemoveUser(t.Tag, t.Email)
			if err1 == nil {
				logger.Debug("Client disabled by api:", t.Email)
			} else if strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", t.Email)) {
				logger.Debug("User is already disabled. Nothing to do more...")
			} else {
				logger.Debug("Error in disabling client by api:", err1)
				needRestart = true
			}
		}
		s.xrayApi.Close()
	}

	for inboundID, emails := range localByInbound {
		if _, _, mErr := s.markClientsDisabledInSettings(tx, inboundID, emails); mErr != nil {
			logger.Warning("disableInvalidClients: settings.JSON sync failed for inbound", inboundID, ":", mErr)
		}
	}

	result := tx.Model(xray.ClientTraffic{}).
		Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ?", now, true).
		Update("enable", false)
	err = result.Error
	count := result.RowsAffected
	if err != nil {
		return needRestart, count, nil, err
	}

	if len(depletedEmails) > 0 {
		if err := tx.Model(&model.ClientRecord{}).
			Where("email IN ?", depletedEmails).
			Updates(map[string]any{"enable": false, "updated_at": now}).Error; err != nil {
			logger.Warning("disableInvalidClients update clients.enable:", err)
		}
	}

	disabledNodeIDs := make(map[int]struct{})
	for inboundID, group := range remoteByInbound {
		emails := make(map[string]struct{}, len(group))
		for _, t := range group {
			emails[t.Email] = struct{}{}
		}
		if pushErr := s.disableRemoteClients(tx, inboundID, emails); pushErr != nil {
			logger.Warning("disableInvalidClients: push to remote failed for inbound", inboundID, ":", pushErr)
			needRestart = true
		} else {
			for _, t := range group {
				if t.NodeID != nil {
					disabledNodeIDs[*t.NodeID] = struct{}{}
				}
			}
		}
	}

	nodeIDs := make([]int, 0, len(disabledNodeIDs))
	for nodeID := range disabledNodeIDs {
		nodeIDs = append(nodeIDs, nodeID)
	}

	return needRestart, count, nodeIDs, nil
}

func (s *InboundService) restartRemoteNodesOnDisable(nodeIDs []int) {
	restartOnDisable, err := (&SettingService{}).GetRestartXrayOnClientDisable()
	if err != nil {
		logger.Warning("disableInvalidClients: get RestartXrayOnClientDisable failed:", err)
		return
	}
	if !restartOnDisable {
		return
	}
	for _, nodeID := range nodeIDs {
		nodeIDCopy := nodeID
		rt, rtErr := runtime.GetManager().RuntimeFor(&nodeIDCopy)
		if rtErr != nil {
			logger.Warning("disableInvalidClients: get runtime for node", nodeID, "failed:", rtErr)
			continue
		}
		if rtErr = rt.RestartXray(context.Background()); rtErr != nil {
			logger.Warning("disableInvalidClients: restart xray on node", nodeID, "failed:", rtErr)
		}
	}
}

// markClientsDisabledInSettings flips client.enable=false in the inbound's
// stored settings JSON for the given emails and returns both the pre and
// post snapshots so a caller pushing to a remote node has the diff to hand.
func (s *InboundService) markClientsDisabledInSettings(tx *gorm.DB, inboundID int, emails map[string]struct{}) (oldIb, newIb *model.Inbound, err error) {
	var ib model.Inbound
	if err := tx.Model(&model.Inbound{}).Where("id = ?", inboundID).First(&ib).Error; err != nil {
		return nil, nil, err
	}
	snapshot := ib

	settings := map[string]any{}
	if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
		return nil, nil, err
	}
	clients, _ := settings["clients"].([]any)
	now := time.Now().Unix() * 1000
	mutated := false
	for i := range clients {
		entry, ok := clients[i].(map[string]any)
		if !ok {
			continue
		}
		email, _ := entry["email"].(string)
		if _, hit := emails[email]; !hit {
			continue
		}
		if cur, _ := entry["enable"].(bool); cur == false {
			continue
		}
		entry["enable"] = false
		entry["updated_at"] = now
		clients[i] = entry
		mutated = true
	}
	if !mutated {
		return &snapshot, &ib, nil
	}
	settings["clients"] = clients
	bs, marshalErr := json.MarshalIndent(settings, "", "  ")
	if marshalErr != nil {
		return nil, nil, marshalErr
	}
	ib.Settings = string(bs)
	if err := tx.Model(&model.Inbound{}).Where("id = ?", inboundID).
		Update("settings", ib.Settings).Error; err != nil {
		return nil, nil, err
	}
	return &snapshot, &ib, nil
}

func (s *InboundService) disableRemoteClients(tx *gorm.DB, inboundID int, emails map[string]struct{}) error {
	oldSnapshot, ib, err := s.markClientsDisabledInSettings(tx, inboundID, emails)
	if err != nil {
		return err
	}

	rt, err := s.runtimeFor(ib)
	if err != nil {
		return err
	}
	if err := rt.UpdateInbound(context.Background(), oldSnapshot, ib); err != nil {
		return err
	}
	return nil
}

func (s *InboundService) GetInboundTags() (string, error) {
	db := database.GetDB()
	var inboundTags []string
	err := db.Model(model.Inbound{}).Select("tag").Find(&inboundTags).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return "", err
	}
	tags, _ := json.Marshal(inboundTags)
	return string(tags), nil
}

func (s *InboundService) GetClientReverseTags() (string, error) {
	db := database.GetDB()
	var inbounds []model.Inbound
	err := db.Model(model.Inbound{}).Select("settings").Where("protocol = ?", "vless").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return "[]", err
	}

	tagSet := make(map[string]struct{})
	for _, inbound := range inbounds {
		var settings map[string]any
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			continue
		}
		clients, ok := settings["clients"].([]any)
		if !ok {
			continue
		}
		for _, client := range clients {
			clientMap, ok := client.(map[string]any)
			if !ok {
				continue
			}
			reverse, ok := clientMap["reverse"].(map[string]any)
			if !ok {
				continue
			}
			tag, _ := reverse["tag"].(string)
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tagSet[tag] = struct{}{}
			}
		}
	}

	rawTags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		rawTags = append(rawTags, tag)
	}
	sort.Strings(rawTags)

	result, _ := json.Marshal(rawTags)
	return string(result), nil
}

func (s *InboundService) MigrationRemoveOrphanedTraffics() {
	db := database.GetDB()
	query := fmt.Sprintf(
		"DELETE FROM client_traffics WHERE email NOT IN (SELECT %s %s)",
		database.JSONFieldText("client.value", "email"),
		database.JSONClientsFromInbound(),
	)
	db.Exec(query)
}

// AddClientStat inserts a per-client accounting row, no-op on email
// conflict. Xray reports traffic per email, so the surviving row acts as
// the shared accumulator for inbounds that re-use the same identity.
func (s *InboundService) AddClientStat(tx *gorm.DB, inboundId int, client *model.Client) error {
	clientTraffic := xray.ClientTraffic{
		InboundId:  inboundId,
		Email:      client.Email,
		Total:      client.TotalGB,
		ExpiryTime: client.ExpiryTime,
		Enable:     client.Enable,
		Reset:      client.Reset,
	}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "email"}}, DoNothing: true}).
		Create(&clientTraffic).Error
}

func (s *InboundService) UpdateClientStat(tx *gorm.DB, email string, client *model.Client) error {
	result := tx.Model(xray.ClientTraffic{}).
		Where("email = ?", email).
		Updates(map[string]any{
			"enable":      client.Enable,
			"email":       client.Email,
			"total":       client.TotalGB,
			"expiry_time": client.ExpiryTime,
			"reset":       client.Reset,
		})
	err := result.Error
	return err
}

func (s *InboundService) UpdateClientIPs(tx *gorm.DB, oldEmail string, newEmail string) error {
	return tx.Model(model.InboundClientIps{}).Where("client_email = ?", oldEmail).Update("client_email", newEmail).Error
}

func (s *InboundService) DelClientStat(tx *gorm.DB, email string) error {
	if err := tx.Where("email = ?", email).Delete(xray.ClientTraffic{}).Error; err != nil {
		return err
	}
	return tx.Where("email = ?", email).Delete(&model.NodeClientTraffic{}).Error
}

func (s *InboundService) DelClientIPs(tx *gorm.DB, email string) error {
	return tx.Where("client_email = ?", email).Delete(model.InboundClientIps{}).Error
}

func (s *InboundService) delClientStatsByEmails(tx *gorm.DB, emails []string) error {
	const chunk = 400
	for start := 0; start < len(emails); start += chunk {
		end := min(start+chunk, len(emails))
		batch := emails[start:end]
		if err := tx.Where("email IN ?", batch).Delete(xray.ClientTraffic{}).Error; err != nil {
			return err
		}
		if err := tx.Where("email IN ?", batch).Delete(&model.NodeClientTraffic{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *InboundService) delClientIPsByEmails(tx *gorm.DB, emails []string) error {
	const chunk = 400
	for start := 0; start < len(emails); start += chunk {
		end := min(start+chunk, len(emails))
		if err := tx.Where("client_email IN ?", emails[start:end]).Delete(model.InboundClientIps{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *InboundService) GetClientInboundByTrafficID(trafficId int) (traffic *xray.ClientTraffic, inbound *model.Inbound, err error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).Where("id = ?", trafficId).Find(&traffics).Error
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with trafficId %d: %v", trafficId, err)
		return nil, nil, err
	}
	if len(traffics) == 0 {
		return nil, nil, nil
	}
	traffic = traffics[0]

	inbound, err = s.GetInbound(traffic.InboundId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// client_traffics.inbound_id goes stale when an inbound is deleted and
		// recreated; fall back to the authoritative client_inbounds link by email.
		ids, idErr := s.clientService.GetInboundIdsForEmail(db, traffic.Email)
		if idErr != nil {
			return traffic, nil, idErr
		}
		if len(ids) > 0 {
			inbound, err = s.GetInbound(ids[0])
		}
	}
	return traffic, inbound, err
}

func (s *InboundService) GetClientInboundByEmail(email string) (traffic *xray.ClientTraffic, inbound *model.Inbound, err error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).Where("email = ?", email).Find(&traffics).Error
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with email %s: %v", email, err)
		return nil, nil, err
	}
	if len(traffics) == 0 {
		return nil, nil, nil
	}
	traffic = traffics[0]

	inbound, err = s.GetInbound(traffic.InboundId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// client_traffics.inbound_id is a legacy single-inbound pointer that goes
		// stale when an inbound is deleted and recreated: the email-keyed traffic
		// row survives but still references the missing inbound. Fall back to the
		// authoritative client_inbounds link so email lookups (reset, info, …) work.
		ids, idErr := s.clientService.GetInboundIdsForEmail(db, email)
		if idErr != nil {
			return traffic, nil, idErr
		}
		if len(ids) > 0 {
			inbound, err = s.GetInbound(ids[0])
		}
	}
	return traffic, inbound, err
}

func (s *InboundService) GetClientByEmail(clientEmail string) (*xray.ClientTraffic, *model.Client, error) {
	traffic, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return nil, nil, err
	}
	if inbound == nil {
		return nil, nil, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	clients, err := s.GetClients(inbound)
	if err != nil {
		return nil, nil, err
	}

	for _, client := range clients {
		if client.Email == clientEmail {
			return traffic, &client, nil
		}
	}

	return nil, nil, common.NewError("Client Not Found In Inbound For Email:", clientEmail)
}

func (s *InboundService) ResetClientTrafficByEmail(clientEmail string) error {
	return submitTrafficWrite(func() error {
		db := database.GetDB()
		return db.Model(xray.ClientTraffic{}).
			Where("email = ?", clientEmail).
			Updates(map[string]any{"enable": true, "up": 0, "down": 0}).Error
	})
}

func (s *InboundService) ResetClientTraffic(id int, clientEmail string) (needRestart bool, err error) {
	err = submitTrafficWrite(func() error {
		var inner error
		needRestart, inner = s.resetClientTrafficLocked(id, clientEmail)
		return inner
	})
	return
}

func (s *InboundService) resetClientTrafficLocked(id int, clientEmail string) (bool, error) {
	needRestart := false

	traffic, err := s.GetClientTrafficByEmail(clientEmail)
	if err != nil {
		return false, err
	}

	if !traffic.Enable {
		inbound, err := s.GetInbound(id)
		if err != nil {
			return false, err
		}
		clients, err := s.GetClients(inbound)
		if err != nil {
			return false, err
		}
		for _, client := range clients {
			if client.Email == clientEmail && client.Enable {
				rt, push, dirty, perr := s.nodePushPlan(inbound)
				if perr != nil {
					return false, perr
				}
				if !push {
					if inbound.NodeID != nil {
						if dirty {
							if dErr := (&NodeService{}).MarkNodeDirty(*inbound.NodeID); dErr != nil {
								logger.Warning("mark node dirty failed:", dErr)
							}
						}
					} else {
						needRestart = true
					}
					break
				}
				cipher := ""
				if string(inbound.Protocol) == "shadowsocks" {
					var oldSettings map[string]any
					err = json.Unmarshal([]byte(inbound.Settings), &oldSettings)
					if err != nil {
						return false, err
					}
					cipher = oldSettings["method"].(string)
				}
				err1 := rt.AddUser(context.Background(), inbound, map[string]any{
					"email":    client.Email,
					"id":       client.ID,
					"auth":     client.Auth,
					"security": client.Security,
					"flow":     client.Flow,
					"password": client.Password,
					"cipher":   cipher,
				})
				if err1 == nil {
					logger.Debug("Client enabled on", rt.Name(), "due to reset traffic:", clientEmail)
				} else if inbound.NodeID != nil {
					logger.Warning("Error in enabling client on", rt.Name(), ":", err1)
					if dErr := (&NodeService{}).MarkNodeDirty(*inbound.NodeID); dErr != nil {
						logger.Warning("mark node dirty failed:", dErr)
					}
				} else {
					logger.Debug("Error in enabling client on", rt.Name(), ":", err1)
					needRestart = true
				}
				break
			}
		}
	}

	traffic.Up = 0
	traffic.Down = 0
	traffic.Enable = true

	db := database.GetDB()
	err = db.Save(traffic).Error
	if err != nil {
		return false, err
	}

	now := time.Now().UnixMilli()
	_ = db.Model(model.Inbound{}).
		Where("id = ?", id).
		Update("last_traffic_reset_time", now).Error

	inbound, err := s.GetInbound(id)
	if err == nil && inbound != nil && inbound.NodeID != nil {
		if rt, rterr := s.runtimeFor(inbound); rterr == nil {
			if e := rt.ResetClientTraffic(context.Background(), inbound, clientEmail); e != nil {
				logger.Warning("ResetClientTraffic: remote propagation to", rt.Name(), "failed:", e)
			}
		} else {
			logger.Warning("ResetClientTraffic: runtime lookup failed:", rterr)
		}
	}

	return needRestart, nil
}

func (s *InboundService) ResetAllTraffics() error {
	return submitTrafficWrite(func() error {
		return s.resetAllTrafficsLocked()
	})
}

func (s *InboundService) resetAllTrafficsLocked() error {
	db := database.GetDB()
	now := time.Now().UnixMilli()

	if err := db.Model(model.Inbound{}).
		Where("user_id > ?", 0).
		Updates(map[string]any{
			"up":                      0,
			"down":                    0,
			"last_traffic_reset_time": now,
		}).Error; err != nil {
		return err
	}

	return nil
}

func (s *InboundService) ResetInboundTraffic(id int) error {
	return submitTrafficWrite(func() error {
		db := database.GetDB()
		return db.Model(model.Inbound{}).
			Where("id = ?", id).
			Updates(map[string]any{"up": 0, "down": 0}).Error
	})
}

// EmailsByInbound returns the list of client emails currently configured on
// an inbound's settings.clients[]. Used by the "delete all clients" flow on
// the inbounds page, which then feeds the list into ClientService.BulkDelete.
func (s *InboundService) EmailsByInbound(inboundId int) ([]string, error) {
	inbound, err := s.GetInbound(inboundId)
	if err != nil {
		return nil, err
	}
	clients, err := s.GetClients(inbound)
	if err != nil {
		return nil, err
	}
	emails := make([]string, 0, len(clients))
	for _, c := range clients {
		if e := strings.TrimSpace(c.Email); e != "" {
			emails = append(emails, e)
		}
	}
	return emails, nil
}

func (s *InboundService) DelDepletedClients(id int) (err error) {
	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	// Collect depleted emails globally — a shared-email row owned by one
	// inbound depletes every sibling that lists the email.
	now := time.Now().Unix() * 1000
	depletedClause := "reset = 0 and ((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?))"
	var depletedRows []xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).
		Where(depletedClause, now).
		Find(&depletedRows).Error
	if err != nil {
		return err
	}
	if len(depletedRows) == 0 {
		return nil
	}

	depletedEmails := make(map[string]struct{}, len(depletedRows))
	for _, r := range depletedRows {
		if r.Email == "" {
			continue
		}
		depletedEmails[strings.ToLower(r.Email)] = struct{}{}
	}
	if len(depletedEmails) == 0 {
		return nil
	}

	var inbounds []*model.Inbound
	inboundQuery := db.Model(model.Inbound{})
	if id >= 0 {
		inboundQuery = inboundQuery.Where("id = ?", id)
	}
	if err = inboundQuery.Find(&inbounds).Error; err != nil {
		return err
	}

	for _, inbound := range inbounds {
		var settings map[string]any
		if err = json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			return err
		}
		rawClients, ok := settings["clients"].([]any)
		if !ok {
			continue
		}
		newClients := make([]any, 0, len(rawClients))
		removed := 0
		for _, client := range rawClients {
			c, ok := client.(map[string]any)
			if !ok {
				newClients = append(newClients, client)
				continue
			}
			email, _ := c["email"].(string)
			if _, isDepleted := depletedEmails[strings.ToLower(email)]; isDepleted {
				removed++
				continue
			}
			newClients = append(newClients, client)
		}
		if removed == 0 {
			continue
		}
		if len(newClients) == 0 {
			s.DelInbound(inbound.Id)
			continue
		}
		settings["clients"] = newClients
		ns, mErr := json.MarshalIndent(settings, "", "  ")
		if mErr != nil {
			return mErr
		}
		inbound.Settings = string(ns)
		if err = tx.Save(inbound).Error; err != nil {
			return err
		}
		survivingClients, gcErr := s.GetClients(inbound)
		if gcErr != nil {
			err = gcErr
			return err
		}
		if err = s.clientService.SyncInbound(tx, inbound.Id, survivingClients); err != nil {
			return err
		}
	}

	// Drop now-orphaned rows. With id >= 0, a row is safe to drop only when
	// no out-of-scope inbound still references the email.
	if id < 0 {
		err = tx.Where(depletedClause, now).Delete(xray.ClientTraffic{}).Error
		return err
	}
	emails := make([]string, 0, len(depletedEmails))
	for e := range depletedEmails {
		emails = append(emails, e)
	}
	var stillReferenced []string
	emailExpr := database.JSONFieldText("client.value", "email")
	stillQuery := fmt.Sprintf(
		"SELECT DISTINCT LOWER(%s) %s WHERE LOWER(%s) IN ?",
		emailExpr,
		database.JSONClientsFromInbound(),
		emailExpr,
	)
	if err = tx.Raw(stillQuery, emails).Scan(&stillReferenced).Error; err != nil {
		return err
	}
	stillSet := make(map[string]struct{}, len(stillReferenced))
	for _, e := range stillReferenced {
		stillSet[e] = struct{}{}
	}
	toDelete := make([]string, 0, len(emails))
	for _, e := range emails {
		if _, kept := stillSet[e]; !kept {
			toDelete = append(toDelete, e)
		}
	}
	if len(toDelete) > 0 {
		if err = tx.Where("LOWER(email) IN ?", toDelete).Delete(xray.ClientTraffic{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *InboundService) GetClientTrafficTgBot(tgId int64) ([]*xray.ClientTraffic, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound

	// Retrieve inbounds where settings contain the given tgId
	err := db.Model(model.Inbound{}).Where("settings LIKE ?", fmt.Sprintf(`%%"tgId": %d%%`, tgId)).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		logger.Errorf("Error retrieving inbounds with tgId %d: %v", tgId, err)
		return nil, err
	}

	var emails []string
	for _, inbound := range inbounds {
		clients, err := s.GetClients(inbound)
		if err != nil {
			logger.Errorf("Error retrieving clients for inbound %d: %v", inbound.Id, err)
			continue
		}
		for _, client := range clients {
			if client.TgID == tgId {
				emails = append(emails, client.Email)
			}
		}
	}

	// Chunked to stay under SQLite's bind-variable limit when a single Telegram
	// account owns thousands of clients across inbounds.
	uniqEmails := uniqueNonEmptyStrings(emails)
	traffics := make([]*xray.ClientTraffic, 0, len(uniqEmails))
	for _, batch := range chunkStrings(uniqEmails, sqliteMaxVars) {
		var page []*xray.ClientTraffic
		if err = db.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Find(&page).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			logger.Errorf("Error retrieving ClientTraffic for emails %v: %v", batch, err)
			return nil, err
		}
		traffics = append(traffics, page...)
	}
	if len(traffics) == 0 {
		logger.Warning("No ClientTraffic records found for emails:", emails)
		return nil, nil
	}

	// Populate UUID and other client data for each traffic record
	for i := range traffics {
		if ct, client, e := s.GetClientByEmail(traffics[i].Email); e == nil && ct != nil && client != nil {
			traffics[i].Enable = client.Enable
			traffics[i].UUID = client.ID
			traffics[i].SubId = client.SubID
		}
	}

	return traffics, nil
}

// sqliteMaxVars is a safe ceiling for the number of bind parameters in a
// single SQL statement. SQLite's SQLITE_MAX_VARIABLE_NUMBER is 999 on builds
// before 3.32 and 32766 after; staying under 999 keeps queries portable
// across forks/old binaries and also bounds per-query memory on truly large
// installs (>32k clients) where even modern SQLite would refuse a single IN.
const sqliteMaxVars = 900

// uniqueNonEmptyStrings returns a deduplicated copy of in with empty strings
// removed, preserving the order of first occurrence.
func uniqueNonEmptyStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// uniqueInts returns a deduplicated copy of in, preserving order of first occurrence.
func uniqueInts(in []int) []int {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(in))
	out := make([]int, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// chunkStrings splits s into consecutive sub-slices of at most size elements.
// Returns nil for an empty input or non-positive size.
func chunkStrings(s []string, size int) [][]string {
	if size <= 0 || len(s) == 0 {
		return nil
	}
	out := make([][]string, 0, (len(s)+size-1)/size)
	for i := 0; i < len(s); i += size {
		end := min(i+size, len(s))
		out = append(out, s[i:end])
	}
	return out
}

// chunkInts splits s into consecutive sub-slices of at most size elements.
// Returns nil for an empty input or non-positive size.
func chunkInts(s []int, size int) [][]int {
	if size <= 0 || len(s) == 0 {
		return nil
	}
	out := make([][]int, 0, (len(s)+size-1)/size)
	for i := 0; i < len(s); i += size {
		end := min(i+size, len(s))
		out = append(out, s[i:end])
	}
	return out
}

func (s *InboundService) GetActiveClientTraffics(emails []string) ([]*xray.ClientTraffic, error) {
	uniq := uniqueNonEmptyStrings(emails)
	if len(uniq) == 0 {
		return nil, nil
	}
	db := database.GetDB()
	traffics := make([]*xray.ClientTraffic, 0, len(uniq))
	for _, batch := range chunkStrings(uniq, sqliteMaxVars) {
		var page []*xray.ClientTraffic
		if err := db.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Find(&page).Error; err != nil {
			return nil, err
		}
		traffics = append(traffics, page...)
	}
	return traffics, nil
}

// GetAllClientTraffics returns the full set of client_traffics rows so the
// websocket broadcasters can ship a complete snapshot every cycle. The old
// delta-only path (GetActiveClientTraffics on activeEmails) silently dropped
// the per-client section whenever no client moved bytes in the cycle or a
// node sync failed, leaving client rows in the UI stuck at stale numbers.
func (s *InboundService) GetAllClientTraffics() ([]*xray.ClientTraffic, error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Find(&traffics).Error; err != nil {
		return nil, err
	}
	return traffics, nil
}

type InboundTrafficSummary struct {
	Id     int   `json:"id"`
	Up     int64 `json:"up"`
	Down   int64 `json:"down"`
	Total  int64 `json:"total"`
	Enable bool  `json:"enable"`
}

func (s *InboundService) GetInboundsTrafficSummary() ([]InboundTrafficSummary, error) {
	db := database.GetDB()
	var summaries []InboundTrafficSummary
	if err := db.Model(&model.Inbound{}).
		Select("id, up, down, total, enable").
		Find(&summaries).Error; err != nil {
		return nil, err
	}
	return summaries, nil
}

func (s *InboundService) GetClientTrafficByEmail(email string) (traffic *xray.ClientTraffic, err error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).Find(&traffics).Error; err != nil {
		logger.Warningf("Error retrieving ClientTraffic with email %s: %v", email, err)
		return nil, err
	}
	if len(traffics) == 0 {
		return nil, nil
	}
	t := traffics[0]

	if rec, rErr := s.clientService.GetRecordByEmail(db, email); rErr == nil && rec != nil {
		c := rec.ToClient()
		t.UUID = c.ID
		t.SubId = c.SubID
		return t, nil
	}

	t2, client, err := s.GetClientByEmail(email)
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with email %s: %v", email, err)
		return nil, err
	}
	if t2 != nil && client != nil {
		t2.UUID = client.ID
		t2.SubId = client.SubID
		return t2, nil
	}
	return nil, nil
}

func (s *InboundService) UpdateClientTrafficByEmail(email string, upload int64, download int64) error {
	return submitTrafficWrite(func() error {
		db := database.GetDB()
		err := db.Model(xray.ClientTraffic{}).
			Where("email = ?", email).
			Updates(map[string]any{
				"up":   upload,
				"down": download,
			}).Error
		if err != nil {
			logger.Warningf("Error updating ClientTraffic with email %s: %v", email, err)
		}
		return err
	})
}

func (s *InboundService) SearchClientTraffic(query string) (traffic *xray.ClientTraffic, err error) {
	db := database.GetDB()
	inbound := &model.Inbound{}
	traffic = &xray.ClientTraffic{}

	// Search for inbound settings that contain the query
	err = db.Model(model.Inbound{}).Where("settings LIKE ?", "%\""+query+"\"%").First(inbound).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warningf("Inbound settings containing query %s not found: %v", query, err)
			return nil, err
		}
		logger.Errorf("Error searching for inbound settings with query %s: %v", query, err)
		return nil, err
	}

	traffic.InboundId = inbound.Id

	// Unmarshal settings to get clients
	settings := map[string][]model.Client{}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		logger.Errorf("Error unmarshalling inbound settings for inbound ID %d: %v", inbound.Id, err)
		return nil, err
	}

	clients := settings["clients"]
	for _, client := range clients {
		if (client.ID == query || client.Password == query) && client.Email != "" {
			traffic.Email = client.Email
			break
		}
	}

	if traffic.Email == "" {
		logger.Warningf("No client found with query %s in inbound ID %d", query, inbound.Id)
		return nil, gorm.ErrRecordNotFound
	}

	// Retrieve ClientTraffic based on the found email
	err = db.Model(xray.ClientTraffic{}).Where("email = ?", traffic.Email).First(traffic).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warningf("ClientTraffic for email %s not found: %v", traffic.Email, err)
			return nil, err
		}
		logger.Errorf("Error retrieving ClientTraffic for email %s: %v", traffic.Email, err)
		return nil, err
	}

	return traffic, nil
}

func (s *InboundService) GetInboundClientIps(clientEmail string) (string, error) {
	db := database.GetDB()
	InboundClientIps := &model.InboundClientIps{}
	err := db.Model(model.InboundClientIps{}).Where("client_email = ?", clientEmail).First(InboundClientIps).Error
	if err != nil {
		return "", err
	}

	if InboundClientIps.Ips == "" {
		return "", nil
	}

	// Try to parse as new format (with timestamps)
	type IPWithTimestamp struct {
		IP        string `json:"ip"`
		Timestamp int64  `json:"timestamp"`
	}

	var ipsWithTime []IPWithTimestamp
	err = json.Unmarshal([]byte(InboundClientIps.Ips), &ipsWithTime)

	// If successfully parsed as new format, return with timestamps
	if err == nil && len(ipsWithTime) > 0 {
		return InboundClientIps.Ips, nil
	}

	// Otherwise, assume it's old format (simple string array)
	// Try to parse as simple array and convert to new format
	var oldIps []string
	err = json.Unmarshal([]byte(InboundClientIps.Ips), &oldIps)
	if err == nil && len(oldIps) > 0 {
		// Convert old format to new format with current timestamp
		newIpsWithTime := make([]IPWithTimestamp, len(oldIps))
		for i, ip := range oldIps {
			newIpsWithTime[i] = IPWithTimestamp{
				IP:        ip,
				Timestamp: time.Now().Unix(),
			}
		}
		result, _ := json.Marshal(newIpsWithTime)
		return string(result), nil
	}

	// Return as-is if parsing fails
	return InboundClientIps.Ips, nil
}

func (s *InboundService) ClearClientIps(clientEmail string) error {
	db := database.GetDB()

	result := db.Model(model.InboundClientIps{}).
		Where("client_email = ?", clientEmail).
		Update("ips", "")
	err := result.Error
	if err != nil {
		return err
	}
	return nil
}

func (s *InboundService) SearchInbounds(query string) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Where("remark like ?", "%"+query+"%").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) MigrationRequirements() {
	db := database.GetDB()
	tx := db.Begin()
	var err error
	defer func() {
		if err == nil {
			tx.Commit()
			if !database.IsPostgres() {
				if dbErr := db.Exec(`VACUUM "main"`).Error; dbErr != nil {
					logger.Warningf("VACUUM failed: %v", dbErr)
				}
			}
		} else {
			tx.Rollback()
		}
	}()

	if tx.Migrator().HasColumn(&model.Inbound{}, "all_time") {
		if err = tx.Migrator().DropColumn(&model.Inbound{}, "all_time"); err != nil {
			return
		}
	}
	if tx.Migrator().HasColumn(&xray.ClientTraffic{}, "all_time") {
		if err = tx.Migrator().DropColumn(&xray.ClientTraffic{}, "all_time"); err != nil {
			return
		}
	}

	// Fix inbounds based problems
	var inbounds []*model.Inbound
	err = tx.Model(model.Inbound{}).Where("protocol IN (?)", []string{"vmess", "vless", "trojan", "shadowsocks", "hysteria"}).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return
	}
	for inbound_index := range inbounds {
		settings := map[string]any{}
		json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
		if raw, exists := settings["clients"]; exists && raw == nil {
			settings["clients"] = []any{}
		}
		clients, ok := settings["clients"].([]any)
		if ok {
			// Fix Client configuration problems
			newClients := make([]any, 0, len(clients))
			hasVisionFlow := false
			for client_index := range clients {
				c := clients[client_index].(map[string]any)

				// Add email='' if it is not exists
				if _, ok := c["email"]; !ok {
					c["email"] = ""
				}

				// Convert string tgId to int64
				if _, ok := c["tgId"]; ok {
					var tgId any = c["tgId"]
					if tgIdStr, ok2 := tgId.(string); ok2 {
						tgIdInt64, err := strconv.ParseInt(strings.ReplaceAll(tgIdStr, " ", ""), 10, 64)
						if err == nil {
							c["tgId"] = tgIdInt64
						}
					}
				}

				// Remove "flow": "xtls-rprx-direct"
				if _, ok := c["flow"]; ok {
					if c["flow"] == "xtls-rprx-direct" {
						c["flow"] = ""
					}
				}
				if flow, _ := c["flow"].(string); flow == "xtls-rprx-vision" {
					hasVisionFlow = true
				}
				// Backfill created_at and updated_at
				if _, ok := c["created_at"]; !ok {
					c["created_at"] = time.Now().Unix() * 1000
				}
				c["updated_at"] = time.Now().Unix() * 1000
				newClients = append(newClients, any(c))
			}
			settings["clients"] = newClients

			// Drop orphaned testseed: VLESS-only field, only meaningful when at least
			// one client uses the exact xtls-rprx-vision flow. Older versions saved it
			// for any non-empty flow (including the UDP variant) or kept it after the
			// flow was cleared from the client modal — clean those up here.
			if inbounds[inbound_index].Protocol == model.VLESS && !hasVisionFlow {
				delete(settings, "testseed")
			}

			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return
			}

			inbounds[inbound_index].Settings = string(modifiedSettings)
		}

		// Add client traffic row for all clients which has email
		modelClients, err := s.GetClients(inbounds[inbound_index])
		if err != nil {
			return
		}
		for _, modelClient := range modelClients {
			if len(modelClient.Email) > 0 {
				var count int64
				tx.Model(xray.ClientTraffic{}).Where("email = ?", modelClient.Email).Count(&count)
				if count == 0 {
					s.AddClientStat(tx, inbounds[inbound_index].Id, &modelClient)
				}
			}
		}

		// Heal clients table for installs where the one-shot seeder
		// skipped clients due to a tgId-string unmarshal error.
		if syncErr := s.clientService.SyncInbound(tx, inbounds[inbound_index].Id, modelClients); syncErr != nil {
			logger.Warning("MigrationRequirements sync clients failed:", syncErr)
		}
	}
	tx.Save(inbounds)

	// Remove orphaned traffics
	tx.Where("inbound_id = 0").Delete(xray.ClientTraffic{})

	// Migrate old MultiDomain to External Proxy
	var externalProxy []struct {
		Id             int
		Port           int
		StreamSettings []byte
	}
	externalProxyQuery := `select id, port, stream_settings
	from inbounds
	WHERE protocol in ('vmess','vless','trojan')
	  AND json_extract(stream_settings, '$.security') = 'tls'
	  AND json_extract(stream_settings, '$.tlsSettings.settings.domains') IS NOT NULL`
	if database.IsPostgres() {
		externalProxyQuery = `select id, port, stream_settings
	from inbounds
	WHERE protocol in ('vmess','vless','trojan')
	  AND NULLIF(stream_settings, '')::jsonb #>> '{security}' = 'tls'
	  AND NULLIF(stream_settings, '')::jsonb #> '{tlsSettings,settings,domains}' IS NOT NULL`
	}
	err = tx.Raw(externalProxyQuery).Scan(&externalProxy).Error
	if err != nil || len(externalProxy) == 0 {
		return
	}

	for _, ep := range externalProxy {
		var reverses any
		var stream map[string]any
		json.Unmarshal(ep.StreamSettings, &stream)
		if tlsSettings, ok := stream["tlsSettings"].(map[string]any); ok {
			if settings, ok := tlsSettings["settings"].(map[string]any); ok {
				if domains, ok := settings["domains"].([]any); ok {
					for _, domain := range domains {
						if domainMap, ok := domain.(map[string]any); ok {
							domainMap["forceTls"] = "same"
							domainMap["port"] = ep.Port
							domainMap["dest"] = domainMap["domain"].(string)
							delete(domainMap, "domain")
						}
					}
				}
				reverses = settings["domains"]
				delete(settings, "domains")
			}
		}
		stream["externalProxy"] = reverses
		newStream, _ := json.MarshalIndent(stream, " ", "  ")
		tx.Model(model.Inbound{}).Where("id = ?", ep.Id).Update("stream_settings", newStream)
	}

	err = tx.Raw(`UPDATE inbounds
	SET tag = REPLACE(tag, '0.0.0.0:', '')
	WHERE INSTR(tag, '0.0.0.0:') > 0;`).Error
	if err != nil {
		return
	}
}

func (s *InboundService) MigrateDB() {
	s.MigrationRequirements()
	s.MigrationRemoveOrphanedTraffics()
}

func (s *InboundService) GetOnlineClients() []string {
	if p == nil {
		return []string{}
	}
	return p.GetOnlineClients()
}

// GetOnlineClientsByGuid returns online emails keyed by the panelGuid of the
// node that physically hosts each set: this panel's own clients under its own
// GUID, plus every node in the tree under its GUID (#4983). Replaces the old
// node-id keying so a client three hops down is attributed to its real node,
// not the intermediate one it was synced through.
func (s *InboundService) GetOnlineClientsByGuid() map[string][]string {
	if p == nil {
		return map[string][]string{}
	}
	out := p.GetMergedNodeTrees()
	if local := p.GetLocalOnlineClients(); len(local) > 0 {
		if guid := s.panelGuid(); guid != "" {
			out[guid] = mergeEmails(out[guid], local)
		}
	}
	return out
}

// GetActiveInboundsByGuid returns the inbound tags that carried traffic within
// the grace window for THIS panel, under its own GUID. Remote nodes don't
// report per-inbound activity, so a GUID missing from the map means "don't
// gate" for that node's inbounds.
func (s *InboundService) GetActiveInboundsByGuid() map[string][]string {
	if p == nil {
		return map[string][]string{}
	}
	active := p.GetLocalActiveInbounds()
	if len(active) == 0 {
		return map[string][]string{}
	}
	guid := s.panelGuid()
	if guid == "" {
		return map[string][]string{}
	}
	return map[string][]string{guid: active}
}

func (s *InboundService) SetNodeOnlineTree(nodeID int, tree map[string][]string) {
	if p != nil {
		p.SetNodeOnlineTree(nodeID, tree)
	}
}

func (s *InboundService) ClearNodeOnlineClients(nodeID int) {
	if p != nil {
		p.ClearNodeOnlineClients(nodeID)
	}
}

// panelGuid returns this panel's stable self-identifier, used to key the local
// panel's own clients in the per-node online maps (#4983).
func (s *InboundService) panelGuid() string {
	guid, _ := (&SettingService{}).GetPanelGuid()
	return guid
}

// synthNodeGuid is the stable per-node fallback identity for a directly-attached
// node whose panel hasn't reported a panelGuid yet (old build). Node ids are
// master-local, so this only composes for direct nodes — exactly the pre-#4983
// flat-topology case where an old-build node appears.
func synthNodeGuid(nodeID int) string {
	return fmt.Sprintf("node:%d", nodeID)
}

// mergeEmails returns the deduped union of two email slices.
func mergeEmails(a, b []string) []string {
	if len(a) == 0 {
		return b
	}
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, e := range a {
		if _, ok := seen[e]; !ok {
			seen[e] = struct{}{}
			out = append(out, e)
		}
	}
	for _, e := range b {
		if _, ok := seen[e]; !ok {
			seen[e] = struct{}{}
			out = append(out, e)
		}
	}
	return out
}

func (s *InboundService) GetClientsLastOnline() (map[string]int64, error) {
	db := database.GetDB()
	var rows []xray.ClientTraffic
	err := db.Model(&xray.ClientTraffic{}).Select("email, last_online").Find(&rows).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, r := range rows {
		result[r.Email] = r.LastOnline
	}
	return result, nil
}

// RefreshLocalOnlineClients folds the emails and inbound tags active on this
// panel's own xray this poll into the local online/active sets, applying the
// grace window and pruning stale entries. Pass nil to only prune. See
// xray.Process for why the local sets are kept separate from the shared
// last_online column.
func (s *InboundService) RefreshLocalOnlineClients(activeEmails, activeInboundTags []string) {
	if p != nil {
		p.RefreshLocalOnline(activeEmails, activeInboundTags, time.Now().UnixMilli(), onlineGracePeriodMs)
	}
}

func (s *InboundService) FilterAndSortClientEmails(emails []string) ([]string, []string, error) {
	db := database.GetDB()

	// Step 1: Get ClientTraffic records for emails in the input list.
	// Chunked to stay under SQLite's bind-variable limit on huge inputs.
	uniqEmails := uniqueNonEmptyStrings(emails)
	clients := make([]xray.ClientTraffic, 0, len(uniqEmails))
	for _, batch := range chunkStrings(uniqEmails, sqliteMaxVars) {
		var page []xray.ClientTraffic
		if err := db.Where("email IN ?", batch).Find(&page).Error; err != nil && err != gorm.ErrRecordNotFound {
			return nil, nil, err
		}
		clients = append(clients, page...)
	}

	// Step 2: Sort clients by (Up + Down) descending
	sort.Slice(clients, func(i, j int) bool {
		return (clients[i].Up + clients[i].Down) > (clients[j].Up + clients[j].Down)
	})

	// Step 3: Extract sorted valid emails and track found ones
	validEmails := make([]string, 0, len(clients))
	found := make(map[string]bool)
	for _, client := range clients {
		validEmails = append(validEmails, client.Email)
		found[client.Email] = true
	}

	// Step 4: Identify emails that were not found in the database
	extraEmails := make([]string, 0)
	for _, email := range emails {
		if !found[email] {
			extraEmails = append(extraEmails, email)
		}
	}

	return validEmails, extraEmails, nil
}

type SubLinkProvider interface {
	SubLinksForSubId(host, subId string) ([]string, error)
	LinksForClient(host string, inbound *model.Inbound, email string) []string
}

var registeredSubLinkProvider SubLinkProvider

func RegisterSubLinkProvider(p SubLinkProvider) {
	registeredSubLinkProvider = p
}

func (s *InboundService) GetSubLinks(host, subId string) ([]string, error) {
	if registeredSubLinkProvider == nil {
		return nil, common.NewError("sub link provider not registered")
	}
	return registeredSubLinkProvider.SubLinksForSubId(host, subId)
}
func (s *InboundService) GetAllClientLinks(host string, email string) ([]string, error) {
	if email == "" {
		return nil, common.NewError("client email is required")
	}
	if registeredSubLinkProvider == nil {
		return nil, common.NewError("sub link provider not registered")
	}
	rec, err := s.clientService.GetRecordByEmail(nil, email)
	if err != nil {
		return nil, err
	}
	inboundIds, err := s.clientService.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		return nil, err
	}
	var links []string
	for _, ibId := range inboundIds {
		inbound, getErr := s.GetInbound(ibId)
		if getErr != nil {
			return nil, getErr
		}
		links = append(links, registeredSubLinkProvider.LinksForClient(host, inbound, email)...)
	}
	return links, nil
}
