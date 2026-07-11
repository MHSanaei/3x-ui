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
		HostHeader:             h.HostHeader,
		Path:                   h.Path,
		Alpn:                   h.Alpn,
		Fingerprint:            h.Fingerprint,
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
	}

	res := make([]*entity.HostGroup, 0, len(orderedGroupIds))
	for _, gId := range orderedGroupIds {
		res = append(res, groupsMap[gId])
	}

	sort.SliceStable(res, func(i, j int) bool {
		if res[i].SortOrder != res[j].SortOrder {
			return res[i].SortOrder < res[j].SortOrder
		}
		return res[i].Remark < res[j].Remark
	})

	return res
}

func buildHostRows(groupId string, req *entity.HostGroup) []*model.Host {
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
				Sni:                    req.Sni,
				HostHeader:             req.HostHeader,
				Path:                   req.Path,
				Alpn:                   req.Alpn,
				Fingerprint:            req.Fingerprint,
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
		return nil, common.NewError("host group not found")
	}
	grouped := groupHosts(hosts)
	if len(grouped) == 0 {
		return nil, common.NewError("host group not found")
	}
	return grouped[0], nil
}

func (s *HostService) AddHostGroup(req *entity.HostGroup) ([]*model.Host, error) {
	groupId := req.GroupId
	if groupId == "" {
		groupId = random.NumLower(16)
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
	created := buildHostRows(groupId, req)

	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.Host{}).Where("group_id = ?", groupId).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return common.NewError("host group not found")
		}
		if err := validateInboundsExist(tx, req.InboundIds); err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", groupId).Delete(&model.Host{}).Error; err != nil {
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

func (s *HostService) DeleteHostGroup(groupId string) error {
	return database.GetDB().Where("group_id = ?", groupId).Delete(&model.Host{}).Error
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
	return database.GetDB().Where("group_id IN ?", groupIds).Delete(&model.Host{}).Error
}

func (s *HostService) ReorderHostGroups(groupIds []string) error {
	if len(groupIds) == 0 {
		return nil
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		for i, groupId := range groupIds {
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
