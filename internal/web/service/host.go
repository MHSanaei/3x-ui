package service

import (
	"sort"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

type HostService struct{}

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
			var hostStr string
			if h.Port > 0 {
				if strings.Contains(h.Address, ":") {
					hostStr = "[" + h.Address + "]:" + strconv.Itoa(h.Port)
				} else {
					hostStr = h.Address + ":" + strconv.Itoa(h.Port)
				}
			} else {
				hostStr = h.Address
			}

			g = &entity.HostGroup{
				GroupId:                gId,
				InboundIds:             []int{h.InboundId},
				Hosts:                  []string{hostStr},
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
			groupsMap[gId] = g
			orderedGroupIds = append(orderedGroupIds, gId)
		} else {
			foundInbound := false
			for _, ibId := range g.InboundIds {
				if ibId == h.InboundId {
					foundInbound = true
					break
				}
			}
			if !foundInbound {
				g.InboundIds = append(g.InboundIds, h.InboundId)
			}

			var hostStr string
			if h.Port > 0 {
				if strings.Contains(h.Address, ":") {
					hostStr = "[" + h.Address + "]:" + strconv.Itoa(h.Port)
				} else {
					hostStr = h.Address + ":" + strconv.Itoa(h.Port)
				}
			} else {
				hostStr = h.Address
			}

			foundHost := false
			for _, hs := range g.Hosts {
				if hs == hostStr {
					foundHost = true
					break
				}
			}
			if !foundHost {
				g.Hosts = append(g.Hosts, hostStr)
			}

			if h.SortOrder < g.SortOrder {
				g.SortOrder = h.SortOrder
			}
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
	db := database.GetDB()
	tx := db.Begin()
	var committed bool
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		} else if !committed {
			tx.Rollback()
		}
	}()

	var created []*model.Host

	for _, inboundId := range req.InboundIds {
		var count int64
		if err := tx.Model(&model.Inbound{}).Where("id = ?", inboundId).Count(&count).Error; err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, common.NewError("inbound not found")
		}
	}

	groupId := req.GroupId
	if groupId == "" {
		groupId = random.NumLower(16)
	}

	hostsToProcess := req.Hosts
	if len(hostsToProcess) == 0 {
		hostsToProcess = []string{""}
	}
	for _, hostStr := range hostsToProcess {
		addr, port := parseHostAndPort(hostStr, req.Port)
		for _, inboundId := range req.InboundIds {
			h := &model.Host{
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
			}
			created = append(created, h)
		}
	}

	if len(created) > 0 {
		if err := tx.Create(&created).Error; err != nil {
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	committed = true
	return created, nil
}

func (s *HostService) UpdateHostGroup(groupId string, req *entity.HostGroup) ([]*model.Host, error) {
	db := database.GetDB()
	tx := db.Begin()
	var committed bool
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		} else if !committed {
			tx.Rollback()
		}
	}()

	var count int64
	if err := tx.Model(&model.Host{}).Where("group_id = ?", groupId).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, common.NewError("host group not found")
	}
	for _, inboundId := range req.InboundIds {
		var ibCount int64
		if err := tx.Model(&model.Inbound{}).Where("id = ?", inboundId).Count(&ibCount).Error; err != nil {
			return nil, err
		}
		if ibCount == 0 {
			return nil, common.NewError("inbound not found")
		}
	}

	if err := tx.Where("group_id = ?", groupId).Delete(&model.Host{}).Error; err != nil {
		return nil, err
	}

	var created []*model.Host
	hostsToProcess := req.Hosts
	if len(hostsToProcess) == 0 {
		hostsToProcess = []string{""}
	}
	for _, hostStr := range hostsToProcess {
		addr, port := parseHostAndPort(hostStr, req.Port)
		for _, inboundId := range req.InboundIds {
			h := &model.Host{
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
			}
			created = append(created, h)
		}
	}

	if len(created) > 0 {
		if err := tx.Create(&created).Error; err != nil {
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	committed = true
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
	tx := database.GetDB().Begin()
	for i, groupId := range groupIds {
		if err := tx.Model(&model.Host{}).Where("group_id = ?", groupId).Update("sort_order", i).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
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
