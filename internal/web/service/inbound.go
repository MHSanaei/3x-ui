// Package service provides business logic services for the 3x-ui web panel,
// including inbound/outbound management, user administration, settings, and Xray integration.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InboundService struct {
	xrayApi         xray.XrayAPI
	clientService   ClientService
	fallbackService FallbackService
}

func normalizeInboundShareAddrStrategy(strategy string) string {
	strategy = strings.TrimSpace(strategy)
	switch strategy {
	case "listen", "custom":
		return strategy
	default:
		return "node"
	}
}

func normalizeInboundShareAddress(inbound *model.Inbound) {
	if inbound == nil {
		return
	}
	inbound.ShareAddrStrategy = normalizeInboundShareAddrStrategy(inbound.ShareAddrStrategy)
	if addr, err := normalizeInboundShareHost(inbound.ShareAddr); err == nil {
		inbound.ShareAddr = addr
	} else {
		inbound.ShareAddr = strings.TrimSpace(inbound.ShareAddr)
	}
}

func normalizeInboundShareAddressStrict(inbound *model.Inbound) error {
	if inbound == nil {
		return nil
	}
	inbound.ShareAddrStrategy = normalizeInboundShareAddrStrategy(inbound.ShareAddrStrategy)
	addr, err := normalizeInboundShareHost(inbound.ShareAddr)
	if err != nil {
		return common.NewError("shareAddr must be a host or IP without scheme or port")
	}
	inbound.ShareAddr = addr
	return nil
}

func normalizeInboundShareHost(raw string) (string, error) {
	addr := strings.TrimSpace(raw)
	if addr == "" {
		return "", nil
	}
	if strings.Contains(addr, "://") || strings.HasPrefix(addr, "//") || strings.ContainsAny(addr, "/?#@") {
		return "", fmt.Errorf("invalid share address %q", raw)
	}
	if strings.HasPrefix(addr, "[") {
		if !strings.HasSuffix(addr, "]") {
			return "", fmt.Errorf("invalid IPv6 host %q", raw)
		}
		ip := net.ParseIP(addr[1 : len(addr)-1])
		if ip == nil || ip.To4() != nil {
			return "", fmt.Errorf("invalid IPv6 host %q", raw)
		}
		return "[" + ip.String() + "]", nil
	}
	if strings.Contains(addr, ":") {
		if _, _, err := net.SplitHostPort(addr); err == nil {
			return "", fmt.Errorf("share address must not include port")
		}
		ip := net.ParseIP(addr)
		if ip == nil || ip.To4() != nil {
			return "", fmt.Errorf("invalid IPv6 host %q", raw)
		}
		return "[" + ip.String() + "]", nil
	}
	host, err := netsafe.NormalizeHost(addr)
	if err != nil {
		return "", err
	}
	return host, nil
}

func normalizeInboundShareAddressColumns(tx *gorm.DB) error {
	if tx == nil || !tx.Migrator().HasColumn(&model.Inbound{}, "share_addr_strategy") {
		return nil
	}

	strategyExpr := `CASE TRIM(COALESCE(share_addr_strategy, '')) WHEN 'listen' THEN 'listen' WHEN 'custom' THEN 'custom' ELSE 'node' END`
	if err := tx.Exec(`UPDATE inbounds SET share_addr_strategy = ` + strategyExpr + ` WHERE share_addr_strategy IS NULL OR share_addr_strategy <> ` + strategyExpr).Error; err != nil {
		return err
	}
	hasShareAddr := tx.Migrator().HasColumn(&model.Inbound{}, "share_addr")
	if hasShareAddr {
		if err := tx.Exec(`UPDATE inbounds SET share_addr = TRIM(share_addr) WHERE share_addr IS NOT NULL AND share_addr <> TRIM(share_addr)`).Error; err != nil {
			return err
		}
	}
	if !hasShareAddr {
		return nil
	}
	var rows []struct {
		Id                int
		ShareAddrStrategy string
		ShareAddr         string
	}
	if err := tx.Model(&model.Inbound{}).Select("id", "share_addr_strategy", "share_addr").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		strategy := normalizeInboundShareAddrStrategy(row.ShareAddrStrategy)
		addr, addrErr := normalizeInboundShareHost(row.ShareAddr)
		if addrErr != nil {
			strategy = "node"
			addr = ""
		}
		updates := map[string]any{}
		if strategy != row.ShareAddrStrategy {
			updates["share_addr_strategy"] = strategy
		}
		if addr != row.ShareAddr {
			updates["share_addr"] = addr
		}
		if len(updates) > 0 {
			if err := tx.Model(&model.Inbound{}).Where("id = ?", row.Id).Updates(updates).Error; err != nil {
				return err
			}
		}
	}
	return nil
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
	// Top up stats rows owned by sibling inbounds (multi-attached clients)
	// so the list's depleted/expiring badges see every client; the UUID/SubId
	// enrichment stays skipped. Must run before slimming strips the settings.
	s.backfillClientStats(db, inbounds)
	// Slim feeds the panel UI only (masters poll the full list), so the badge
	// math may see the cross-panel totals a master pushed.
	s.overlayInboundsClientStats(db, inbounds)
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
	// Hosting node; nil for this panel's own inbounds. Lets the clients
	// page map a node filter onto inbound IDs (#4997).
	NodeId *int `json:"nodeId,omitempty"`
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
		NodeId         *int   `gorm:"column:node_id"`
	}
	err := db.Table("inbounds").
		Select("id, remark, tag, protocol, port, stream_settings, settings, node_id").
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
			TlsFlowCapable: inboundCanEnableTlsFlow(r.Protocol, r.StreamSettings, r.Settings),
			SsMethod:       inboundShadowsocksMethod(r.Protocol, r.Settings),
			NodeId:         r.NodeId,
		})
	}
	return out, nil
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

func (s *InboundService) GetAllEmails() ([]string, error) {
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

// normalizeStreamSettings clears StreamSettings for protocols that don't use it.
// Only vmess, vless, trojan, shadowsocks, hysteria, and wireguard protocols use
// streamSettings (wireguard for finalmask UDP masks and sockopt on its listener).
func (s *InboundService) normalizeStreamSettings(inbound *model.Inbound) {
	protocolsWithStream := map[model.Protocol]bool{
		model.VMESS:       true,
		model.VLESS:       true,
		model.Trojan:      true,
		model.Shadowsocks: true,
		model.Hysteria:    true,
		model.WireGuard:   true,
	}

	if !protocolsWithStream[inbound.Protocol] {
		inbound.StreamSettings = ""
	}
}

// normalizeMtprotoSecret rebuilds an mtproto inbound's FakeTLS secret so it is
// always valid and matches the configured domain before the row is persisted.
func (s *InboundService) normalizeMtprotoSecret(inbound *model.Inbound) {
	if inbound.Protocol != model.MTProto {
		return
	}
	if healed, ok := model.HealMtprotoSecret(inbound.Settings); ok {
		inbound.Settings = healed
	}
}

// mtprotoRoutesThroughXray reports whether an mtproto inbound is configured to
// egress through the core's router (the loopback SOCKS bridge in §xray.go).
func mtprotoRoutesThroughXray(inbound *model.Inbound) bool {
	if inbound == nil || inbound.Protocol != model.MTProto {
		return false
	}
	var parsed struct {
		RouteThroughXray bool `json:"routeThroughXray"`
	}
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil {
		return false
	}
	return parsed.RouteThroughXray
}

func settingsRouteXrayPort(parsed map[string]any) int {
	switch v := parsed["routeXrayPort"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	}
	return 0
}

func parseRouteXrayPort(settings string) int {
	if settings == "" {
		return 0
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return 0
	}
	return settingsRouteXrayPort(parsed)
}

// normalizeMtprotoXrayPort guarantees a routed mtproto inbound carries a stable
// loopback egress port in its settings, so the generated Xray SOCKS bridge and
// the mtg sidecar agree on where mtg dials out. The port is backend-owned: it is
// allocated once when routing is first enabled and preserved across edits
// (carried over from oldSettings, which wins over any value the client echoed
// back). When routing is off it — together with the now-inert outbound
// selection — is stripped so a disabled bridge leaves nothing stale behind.
//
// It returns an error when an egress port cannot be allocated or persisted, so
// the caller refuses the save rather than storing a routed-but-portless inbound,
// which would otherwise route no traffic and have its mtg metrics skipped (see
// mtproto_job) — silently losing its accounting.
func (s *InboundService) normalizeMtprotoXrayPort(inbound *model.Inbound, oldSettings string) error {
	if inbound.Protocol != model.MTProto {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil || parsed == nil {
		return nil
	}
	routed, _ := parsed["routeThroughXray"].(bool)
	if !routed {
		_, hadPort := parsed["routeXrayPort"]
		_, hadTag := parsed["outboundTag"]
		if !hadPort && !hadTag {
			return nil
		}
		delete(parsed, "routeXrayPort")
		delete(parsed, "outboundTag")
		if bs, err := json.MarshalIndent(parsed, "", "  "); err == nil {
			inbound.Settings = string(bs)
		} else {
			logger.Warning("mtproto: failed to marshal settings after disabling routing:", err)
		}
		return nil
	}

	// Prefer the already-stored port (carried across edits), then any value the
	// client sent, then allocate a fresh one.
	port := parseRouteXrayPort(oldSettings)
	if port <= 0 {
		port = settingsRouteXrayPort(parsed)
	}
	if port <= 0 {
		allocated, err := mtproto.FreeLocalPort()
		if err != nil {
			return common.NewError("mtproto: could not allocate an Xray egress port:", err)
		}
		port = allocated
	}
	if settingsRouteXrayPort(parsed) == port {
		return nil
	}
	parsed["routeXrayPort"] = port
	bs, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return common.NewError("mtproto: could not persist the Xray egress port:", err)
	}
	inbound.Settings = string(bs)
	return nil
}

// AddInbound creates a new inbound configuration.
// It validates port uniqueness, client email uniqueness, and required fields,
// then saves the inbound to the database and optionally adds it to the running Xray instance.
// Returns the created inbound, whether Xray needs restart, and any error.
func (s *InboundService) AddInbound(inbound *model.Inbound) (*model.Inbound, bool, error) {
	// Normalize streamSettings based on protocol
	s.normalizeStreamSettings(inbound)
	s.normalizeMtprotoSecret(inbound)
	if err := s.normalizeMtprotoXrayPort(inbound, ""); err != nil {
		return inbound, false, err
	}
	inbound.SubSortIndex = normalizeSubSortIndex(inbound.SubSortIndex)
	if err := normalizeInboundShareAddressStrict(inbound); err != nil {
		return inbound, false, err
	}

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

	// Defensively fix any Shadowsocks-2022 client PSK whose length doesn't match
	// the inbound method (e.g. an API caller supplied a wrong-size key).
	if normalized, changed := normalizeShadowsocksClientKeys(inbound.Settings); changed {
		inbound.Settings = normalized
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

	// Omit the ClientStats has-many association: GORM's cascade would INSERT
	// those rows with an ON CONFLICT target on the primary key only, which
	// collides with the globally-unique client_traffics.email when an imported
	// inbound carries clients that another inbound already created (e.g.
	// importing two inbounds that share the same clients). We insert the stats
	// ourselves below with the same email-conflict guard AddClientStat uses.
	err = tx.Omit("ClientStats").Save(inbound).Error
	if err != nil {
		return inbound, false, err
	}
	// Imported stats first, so their traffic counters survive; emails that
	// already own a (shared) row are skipped instead of tripping the unique
	// constraint.
	for i := range inbound.ClientStats {
		if inbound.ClientStats[i].Email == "" {
			continue
		}
		inbound.ClientStats[i].Id = 0
		inbound.ClientStats[i].InboundId = inbound.Id
		if err = tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "email"}},
			DoNothing: true,
		}).Create(&inbound.ClientStats[i]).Error; err != nil {
			return inbound, false, err
		}
	}
	// Then make sure every client has a stats row. AddClientStat is a no-op
	// where one exists (including the rows just inserted), and fills the gap
	// for clients an import payload didn't carry stats for.
	for _, client := range clients {
		if err = s.AddClientStat(tx, inbound.Id, &client); err != nil {
			return inbound, false, err
		}
	}

	if err = s.clientService.SyncInbound(tx, inbound.Id, clients); err != nil {
		return inbound, false, err
	}

	// Before the deferred commit, so a node in "selected" sync mode cannot
	// sweep the new central row in the gap before its tag is allowed.
	if inbound.NodeID != nil {
		if aErr := (&NodeService{}).EnsureInboundTagAllowed(*inbound.NodeID, inbound.Tag); aErr != nil {
			logger.Warning("allow inbound tag on node failed:", aErr)
		}
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

	// A routed mtproto inbound is not an Xray inbound itself, so the runtime
	// push above only (re)starts the mtg sidecar. The egress SOCKS bridge lives
	// in the generated config, so force a regen to wire it in.
	if mtprotoRoutesThroughXray(inbound) {
		needRestart = true
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
	// Hosts have no hard FK; drop the inbound's hosts alongside it.
	if err := db.Where("inbound_id = ?", id).Delete(&model.Host{}).Error; err != nil {
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
	// Drop the egress SOCKS bridge a routed mtproto inbound left in the config.
	if mtprotoRoutesThroughXray(&ib) {
		needRestart = true
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
	s.overlayInboundsClientStats(db, []*model.Inbound{inbound})
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
	s.normalizeMtprotoSecret(inbound)
	inbound.SubSortIndex = normalizeSubSortIndex(inbound.SubSortIndex)

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
	// Capture the pre-edit routing state before oldInbound.Settings is replaced
	// with the new settings further down, then ensure a routed inbound keeps a
	// stable egress port (reusing the one already stored).
	oldRoutedMtproto := mtprotoRoutesThroughXray(oldInbound)
	if err := s.normalizeMtprotoXrayPort(inbound, oldInbound.Settings); err != nil {
		return inbound, false, err
	}

	tag := oldInbound.Tag
	oldBits := inboundTransports(oldInbound.Protocol, oldInbound.StreamSettings, oldInbound.Settings)
	oldTagWasAuto := isAutoGeneratedTag(tag, oldInbound.Port, oldInbound.NodeID, oldBits)

	needRestart := false
	markDirty := false

	// Persist the client-stat sync, settings munging, runtime push and inbound
	// save as one transaction routed through the serial traffic writer, so it
	// never runs concurrently with the @every 5s traffic poll. Both touch
	// client_traffics and inbounds in opposite order, which Postgres aborts as a
	// deadlock (40P01); serializing removes the contention (runSerializedTx).
	//
	// The runtime push stays inside the transaction here (unlike the client-edit
	// paths that apply it after commit): EnsureInboundTagAllowed must reach the
	// node before the central row is committed, or a "selected"-mode node would
	// sweep the renamed inbound on its next pull. Inbound edits are rare, so
	// holding the writer across the node call is an acceptable trade.
	txErr := runSerializedTx(func(tx *gorm.DB) error {
		if err := s.updateClientTraffics(tx, oldInbound, inbound); err != nil {
			return err
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

		// A Shadowsocks-2022 method change resizes the key, but existing client PSKs
		// keep their old length and would be rejected by xray. Regenerate mismatched
		// client keys so the inbound stays connectable.
		if normalized, changed := normalizeShadowsocksClientKeys(inbound.Settings); changed {
			inbound.Settings = normalized
			logger.Warning("Shadowsocks inbound", inbound.Id, "method change resized keys; regenerated mismatched client PSK(s)")
		}

		oldInbound.Total = inbound.Total
		oldInbound.Remark = inbound.Remark
		oldInbound.SubSortIndex = inbound.SubSortIndex
		oldInbound.Enable = inbound.Enable
		oldInbound.ExpiryTime = inbound.ExpiryTime
		oldInbound.TrafficReset = inbound.TrafficReset
		oldInbound.Listen = inbound.Listen
		oldInbound.Port = inbound.Port
		oldInbound.Protocol = inbound.Protocol
		oldInbound.Settings = inbound.Settings
		oldInbound.StreamSettings = inbound.StreamSettings
		oldInbound.Sniffing = inbound.Sniffing
		if strings.TrimSpace(inbound.ShareAddrStrategy) == "" {
			normalizeInboundShareAddress(oldInbound)
			inbound.ShareAddrStrategy = oldInbound.ShareAddrStrategy
			inbound.ShareAddr = oldInbound.ShareAddr
		} else {
			if err := normalizeInboundShareAddressStrict(inbound); err != nil {
				return err
			}
			oldInbound.ShareAddrStrategy = inbound.ShareAddrStrategy
			oldInbound.ShareAddr = inbound.ShareAddr
		}
		if oldTagWasAuto && inbound.Tag == tag {
			inbound.Tag = ""
		}
		resolvedTag, err := s.resolveInboundTag(inbound, inbound.Id)
		if err != nil {
			return err
		}
		oldInbound.Tag = resolvedTag
		inbound.Tag = oldInbound.Tag

		rt, push, dirty, perr := s.nodePushPlan(oldInbound)
		if perr != nil {
			return perr
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

		// A rename must allow the new tag before the inbound row is committed, or a
		// node in "selected" sync mode would sweep the renamed central row on the
		// next pull.
		if oldInbound.NodeID != nil {
			if aErr := (&NodeService{}).EnsureInboundTagAllowed(*oldInbound.NodeID, oldInbound.Tag); aErr != nil {
				logger.Warning("allow inbound tag on node failed:", aErr)
			}
		}

		if err := tx.Save(oldInbound).Error; err != nil {
			return err
		}
		newClients, gcErr := s.GetClients(oldInbound)
		if gcErr != nil {
			return gcErr
		}
		if err := s.clientService.SyncInbound(tx, oldInbound.Id, newClients); err != nil {
			return err
		}
		// (Re)generate the Xray config whenever routing was or is now enabled, so
		// the egress SOCKS bridge is added, moved, or dropped to match the new
		// settings.
		if mtprotoRoutesThroughXray(inbound) || oldRoutedMtproto {
			needRestart = true
		}
		return nil
	})
	if txErr != nil {
		return inbound, false, txErr
	}
	if markDirty && oldInbound.NodeID != nil {
		if dErr := (&NodeService{}).MarkNodeDirty(*oldInbound.NodeID); dErr != nil {
			logger.Warning("mark node dirty failed:", dErr)
		}
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

func (s *InboundService) SearchInbounds(query string) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Where("remark like ?", "%"+query+"%").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}
