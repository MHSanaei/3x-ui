package service

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type HostService struct{}

func formatHostAddr(addr string, port int) string {
	if port <= 0 {
		return addr
	}
	if strings.Contains(addr, ":") {
		return "[" + addr + "]:" + strconv.Itoa(port)
	}
	return addr + ":" + strconv.Itoa(port)
}

func newHostGroup(h *model.Host, groupId string) *entity.HostGroup {
	return &entity.HostGroup{
		GroupId:                groupId,
		InboundIds:             []int{},
		Hosts:                  []string{},
		SortOrder:              h.SortOrder,
		Remark:                 h.Remark,
		ServerDescription:      h.ServerDescription,
		IsDisabled:             h.IsDisabled,
		IsHidden:               h.IsHidden,
		Tags:                   h.Tags,
		Port:                   h.Port,
		Security:               h.Security,
		Sni:                    h.Sni,
		Snis:                   h.Snis,
		SniSharedRandom:        h.SniSharedRandom,
		HostHeader:             h.HostHeader,
		Path:                   h.Path,
		Alpn:                   h.Alpn,
		Fingerprint:            h.Fingerprint,
		CipherSuites:           h.CipherSuites,
		OverrideSniFromAddress: h.OverrideSniFromAddress,
		KeepSniBlank:           h.KeepSniBlank,
		PinnedPeerCertSha256:   h.PinnedPeerCertSha256,
		VerifyPeerCertByName:   h.VerifyPeerCertByName,
		AllowInsecure:          h.AllowInsecure,
		EchConfigList:          h.EchConfigList,
		MuxParams:              h.MuxParams,
		SockoptParams:          h.SockoptParams,
		FinalMask:              h.FinalMask,
		VlessRoute:             h.VlessRoute,
		ExcludeFromSubTypes:    h.ExcludeFromSubTypes,
		NodeGuids:              h.NodeGuids,
		MihomoIpVersion:        h.MihomoIpVersion,
		MihomoX25519:           h.MihomoX25519,
		ShuffleHost:            h.ShuffleHost,
		RandomCount:            h.RandomCount,
	}
}

func groupHosts(hosts []*model.Host) []*entity.HostGroup {
	groupsMap := make(map[string]*entity.HostGroup)
	var orderedGroupIds []string

	for _, h := range hosts {
		gId := h.GroupId
		if gId == "" {
			gId = "fallback_" + strconv.Itoa(h.Id)
		}

		g, exists := groupsMap[gId]
		if !exists {
			g = newHostGroup(h, gId)
			groupsMap[gId] = g
			orderedGroupIds = append(orderedGroupIds, gId)
		}

		if !slices.Contains(g.InboundIds, h.InboundId) {
			g.InboundIds = append(g.InboundIds, h.InboundId)
		}
		hostStr := formatHostAddr(h.Address, h.Port)
		if !slices.Contains(g.Hosts, hostStr) {
			g.Hosts = append(g.Hosts, hostStr)
		}
		if h.SortOrder < g.SortOrder {
			g.SortOrder = h.SortOrder
		}
		if h.RandomCount > g.RandomCount {
			g.RandomCount = h.RandomCount
		}
		if len(h.Snis) > 0 && len(g.Snis) == 0 {
			g.Snis = h.Snis
		}
		if h.SniSharedRandom {
			g.SniSharedRandom = true
		}
	}

	res := make([]*entity.HostGroup, 0, len(orderedGroupIds))
	for _, gId := range orderedGroupIds {
		res = append(res, groupsMap[gId])
	}

	sort.SliceStable(res, func(i, j int) bool {
		if res[i].SortOrder != res[j].SortOrder {
			return res[i].SortOrder < res[j].SortOrder
		}
		if res[i].Remark != res[j].Remark {
			return res[i].Remark < res[j].Remark
		}
		return res[i].GroupId < res[j].GroupId
	})

	return res
}

func buildHostRows(groupId string, req *entity.HostGroup) []*model.Host {
	snis := hostSniPool(req.Sni, req.Snis)
	primarySni := ""
	if len(snis) > 0 {
		primarySni = snis[0]
	}
	shared := req.SniSharedRandom && len(snis) > 1

	hostsToProcess := req.Hosts
	if len(hostsToProcess) == 0 {
		hostsToProcess = []string{""}
	}
	var rows []*model.Host
	for _, hostStr := range hostsToProcess {
		addr, port := parseHostAndPort(hostStr, req.Port)
		for _, inboundId := range req.InboundIds {
			rows = append(rows, &model.Host{
				GroupId:                groupId,
				InboundId:              inboundId,
				SortOrder:              req.SortOrder,
				Remark:                 req.Remark,
				ServerDescription:      req.ServerDescription,
				IsDisabled:             req.IsDisabled,
				IsHidden:               req.IsHidden,
				Tags:                   req.Tags,
				Address:                addr,
				Port:                   port,
				Security:               req.Security,
				Sni:                    primarySni,
				Snis:                   snis,
				SniSharedRandom:        shared,
				HostHeader:             req.HostHeader,
				Path:                   req.Path,
				Alpn:                   req.Alpn,
				Fingerprint:            req.Fingerprint,
				CipherSuites:           req.CipherSuites,
				OverrideSniFromAddress: req.OverrideSniFromAddress,
				KeepSniBlank:           req.KeepSniBlank,
				PinnedPeerCertSha256:   req.PinnedPeerCertSha256,
				VerifyPeerCertByName:   req.VerifyPeerCertByName,
				AllowInsecure:          req.AllowInsecure,
				EchConfigList:          req.EchConfigList,
				MuxParams:              req.MuxParams,
				SockoptParams:          req.SockoptParams,
				FinalMask:              req.FinalMask,
				VlessRoute:             req.VlessRoute,
				ExcludeFromSubTypes:    req.ExcludeFromSubTypes,
				NodeGuids:              req.NodeGuids,
				MihomoIpVersion:        req.MihomoIpVersion,
				MihomoX25519:           req.MihomoX25519,
				ShuffleHost:            req.ShuffleHost,
				RandomCount:            clampRandomCount(req.RandomCount),
			})
		}
	}
	return rows
}

// adoptedHostRows projects a node's host groups onto a freshly adopted central
// inbound so TLS/SNI/fingerprint overrides survive the node-to-master import.
func adoptedHostRows(groups []*entity.HostGroup, nodeInboundId, centralInboundId int) []*model.Host {
	var rows []*model.Host
	for _, g := range groups {
		if g == nil || !slices.Contains(g.InboundIds, nodeInboundId) {
			continue
		}
		scoped := *g
		scoped.InboundIds = []int{centralInboundId}
		rows = append(rows, buildHostRows(g.GroupId, &scoped)...)
	}
	return rows
}

func validateInboundsExist(tx *gorm.DB, inboundIds []int) error {
	for _, inboundId := range inboundIds {
		var count int64
		if err := tx.Model(&model.Inbound{}).Where("id = ?", inboundId).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return common.NewError("inbound not found")
		}
	}
	return nil
}

func (s *HostService) GetHosts() ([]*entity.HostGroup, error) {
	var hosts []*model.Host
	err := database.GetDB().Order("inbound_id asc, sort_order asc, id asc").Find(&hosts).Error
	if err != nil {
		return nil, err
	}
	return groupHosts(hosts), nil
}

// GetRawHosts returns every host row with its numeric id, so callers that
// bind by host id (per-client host rule overrides) can list candidates.
func (s *HostService) GetRawHosts() ([]*model.Host, error) {
	var hosts []*model.Host
	err := database.GetDB().Order("sort_order asc, id asc").Find(&hosts).Error
	if err != nil {
		return nil, err
	}
	return hosts, nil
}

func (s *HostService) GetHostsByInbound(inboundId int) ([]*entity.HostGroup, error) {
	var groupIds []string
	if err := database.GetDB().Model(&model.Host{}).Where("inbound_id = ?", inboundId).Distinct().Pluck("group_id", &groupIds).Error; err != nil {
		return nil, err
	}
	if len(groupIds) == 0 {
		return nil, nil
	}
	var hosts []*model.Host
	if err := database.GetDB().Where("group_id IN ?", groupIds).Order("sort_order asc, id asc").Find(&hosts).Error; err != nil {
		return nil, err
	}
	return groupHosts(hosts), nil
}

func (s *HostService) GetHostGroup(groupId string) (*entity.HostGroup, error) {
	var hosts []*model.Host
	err := database.GetDB().Where("group_id = ?", groupId).Order("sort_order asc, id asc").Find(&hosts).Error
	if err != nil {
		return nil, err
	}
	if len(hosts) == 0 {
		return nil, common.NewError("host not found")
	}
	grouped := groupHosts(hosts)
	if len(grouped) == 0 {
		return nil, common.NewError("host not found")
	}
	return grouped[0], nil
}

func (s *HostService) AddHostGroup(req *entity.HostGroup) ([]*model.Host, error) {
	groupId := req.GroupId
	if groupId == "" {
		groupId = random.NumLower(16)
	}
	if err := validateFinalMaskJSONSalamander(req.FinalMask); err != nil {
		return nil, err
	}
	created := buildHostRows(groupId, req)

	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := validateInboundsExist(tx, req.InboundIds); err != nil {
			return err
		}
		if len(created) > 0 {
			return tx.Create(&created).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *HostService) UpdateHostGroup(groupId string, req *entity.HostGroup) ([]*model.Host, error) {
	if err := validateFinalMaskJSONSalamander(req.FinalMask); err != nil {
		return nil, err
	}
	desired := buildHostRows(groupId, req)

	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		var existing []*model.Host
		if err := tx.Where("group_id = ?", groupId).Order("id asc").Find(&existing).Error; err != nil {
			return err
		}
		if len(existing) == 0 {
			return common.NewError("host not found")
		}
		if err := validateInboundsExist(tx, req.InboundIds); err != nil {
			return err
		}
		// Exact (inbound, address, port) matches reuse ids so edits keep bindings; a second pass pairs
		// same-inbound leftovers with edits, and rows still unclaimed are deleted.
		type hostKey struct {
			inbound int
			addr    string
			port    int
		}
		byKey := make(map[hostKey][]*model.Host)
		for _, h := range existing {
			k := hostKey{h.InboundId, h.Address, h.Port}
			byKey[k] = append(byKey[k], h)
		}
		var toInsert, toUpdate []*model.Host
		var unmatched []*model.Host
		matched := make(map[int]struct{}, len(desired))
		for _, d := range desired {
			k := hostKey{d.InboundId, d.Address, d.Port}
			if q := byKey[k]; len(q) > 0 {
				d.Id = q[0].Id
				d.CreatedAt = q[0].CreatedAt
				byKey[k] = q[1:]
				toUpdate = append(toUpdate, d)
				matched[d.Id] = struct{}{}
			} else {
				unmatched = append(unmatched, d)
			}
		}
		leftovers := make(map[int][]*model.Host)
		for _, h := range existing {
			if _, ok := matched[h.Id]; !ok {
				leftovers[h.InboundId] = append(leftovers[h.InboundId], h)
			}
		}
		for _, d := range unmatched {
			if q := leftovers[d.InboundId]; len(q) > 0 {
				d.Id = q[0].Id
				d.CreatedAt = q[0].CreatedAt
				leftovers[d.InboundId] = q[1:]
				toUpdate = append(toUpdate, d)
				matched[d.Id] = struct{}{}
			} else {
				toInsert = append(toInsert, d)
			}
		}
		var toDelete []int
		for _, h := range existing {
			if _, ok := matched[h.Id]; !ok {
				toDelete = append(toDelete, h.Id)
			}
		}
		if len(toDelete) > 0 {
			if _, err := database.DeleteHosts(tx, "id IN ?", toDelete); err != nil {
				return err
			}
		}
		for _, u := range toUpdate {
			if err := tx.Save(u).Error; err != nil {
				return err
			}
		}
		if len(toInsert) > 0 {
			if err := tx.Create(&toInsert).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return desired, nil
}

func (s *HostService) DeleteHostGroup(groupId string) error {
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		_, err := database.DeleteHosts(tx, "group_id = ?", groupId)
		return err
	})
}

func (s *HostService) SetHostGroupEnable(groupId string, enable bool) error {
	return database.GetDB().Model(&model.Host{}).Where("group_id = ?", groupId).Update("is_disabled", !enable).Error
}

func (s *HostService) SetHostsGroupEnable(groupIds []string, enable bool) error {
	if len(groupIds) == 0 {
		return nil
	}
	return database.GetDB().Model(&model.Host{}).Where("group_id IN ?", groupIds).Update("is_disabled", !enable).Error
}

func (s *HostService) DeleteHostsGroup(groupIds []string) error {
	if len(groupIds) == 0 {
		return nil
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		_, err := database.DeleteHosts(tx, "group_id IN ?", groupIds)
		return err
	})
}

func (s *HostService) ReorderHostGroups(groupIds []string) error {
	if len(groupIds) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(groupIds))
	for _, id := range groupIds {
		if id == "" || seen[id] {
			return common.NewError("host group IDs must be nonempty and unique")
		}
		seen[id] = true
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		var rows []*model.Host
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("id ASC").Find(&rows).Error; err != nil {
			return err
		}
		groups := groupHosts(rows)
		known := make(map[string]bool, len(groups))
		ordered := append([]string(nil), groupIds...)
		for _, group := range groups {
			known[group.GroupId] = true
			if !seen[group.GroupId] {
				ordered = append(ordered, group.GroupId)
			}
		}
		for _, id := range groupIds {
			if !known[id] {
				return common.NewError("host group not found")
			}
		}
		for i, groupId := range ordered {
			if err := tx.Model(&model.Host{}).Where("group_id = ?", groupId).Update("sort_order", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *HostService) GetAllTags() ([]string, error) {
	var hosts []*model.Host
	err := database.GetDB().Find(&hosts).Error
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{})
	for _, h := range hosts {
		for _, tag := range h.Tags {
			if tag != "" {
				set[tag] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for tag := range set {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out, nil
}

func parseHostAndPort(hostStr string, defaultPort int) (string, int) {
	hostStr = strings.TrimSpace(hostStr)
	if hostStr == "" {
		return "", defaultPort
	}
	if strings.Count(hostStr, ":") > 1 && !strings.Contains(hostStr, "[") {
		return hostStr, defaultPort
	}
	lastColon := strings.LastIndex(hostStr, ":")
	if lastColon != -1 && lastColon < len(hostStr)-1 {
		pStr := hostStr[lastColon+1:]
		if p, err := strconv.Atoi(pStr); err == nil && p >= 0 && p <= 65535 {
			addr := hostStr[:lastColon]
			if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
				addr = addr[1 : len(addr)-1]
			}
			return addr, p
		}
	}
	addr := hostStr
	if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
		addr = addr[1 : len(addr)-1]
	}
	return addr, defaultPort
}

func clampRandomCount(n int) int {
	if n <= 0 {
		return 1
	}
	if n > 100 {
		return 100
	}
	return n
}

// hostSniPool is the group's SNI pool, primary first: an sni outside the pool (a caller that only
// knows the single field) becomes the primary, replacing a one-entry pool.
func hostSniPool(sni string, pool []string) []string {
	snis := cleanSnis(pool)
	sni = strings.TrimSpace(sni)
	switch {
	case sni == "" || (len(snis) > 0 && snis[0] == sni):
		return snis
	case len(snis) <= 1:
		return []string{sni}
	}
	return append([]string{sni}, slices.DeleteFunc(snis, func(s string) bool { return s == sni })...)
}

func cleanSnis(snis []string) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, s := range snis {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			if _, exists := seen[trimmed]; !exists {
				seen[trimmed] = struct{}{}
				out = append(out, trimmed)
			}
		}
	}
	return out
}
