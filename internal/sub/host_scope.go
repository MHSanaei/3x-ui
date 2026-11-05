package sub

import (
	"slices"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// ensureClientGroupIndex loads client groups and host-group bindings once per
// request; membership itself stays on clients.group_name.
func (s *SubService) ensureClientGroupIndex() {
	if s.clientGroupIndexLoaded {
		return
	}
	s.clientGroupIndexLoaded = true
	s.clientGroupIDsByName = map[string]int{}
	s.hostGroupsByClientGroup = map[int]map[string]struct{}{}
	db := database.GetDB()
	if db == nil {
		return
	}
	var groups []model.ClientGroup
	if err := db.Select("id", "name").Order("id ASC").Find(&groups).Error; err != nil {
		logger.Warning("SubService - ensureClientGroupIndex:", err)
		return
	}
	canonicalByID := make(map[int]int, len(groups))
	for _, group := range groups {
		key := strings.ToLower(strings.TrimSpace(group.Name))
		canonical, exists := s.clientGroupIDsByName[key]
		if !exists {
			canonical = group.Id
			s.clientGroupIDsByName[key] = canonical
		}
		canonicalByID[group.Id] = canonical
	}
	var bindings []model.HostGroupClientGroup
	if err := db.Find(&bindings).Error; err != nil {
		logger.Warning("SubService - ensureClientGroupIndex:", err)
		return
	}
	for _, binding := range bindings {
		groupID, ok := canonicalByID[binding.ClientGroupId]
		if !ok {
			continue
		}
		bound := s.hostGroupsByClientGroup[groupID]
		if bound == nil {
			bound = map[string]struct{}{}
			s.hostGroupsByClientGroup[groupID] = bound
		}
		bound[binding.HostGroupId] = struct{}{}
	}
}

// hostsForInbound loads one inbound's hosts once per request, including disabled
// rows, so tier selection can tell "no binding" from "bound but unusable".
func (s *SubService) hostsForInbound(inbound *model.Inbound) []*model.Host {
	if s.hostsLoaded != nil && s.hostsLoaded[inbound.Id] {
		return s.hostsByInbound[inbound.Id]
	}
	if s.hostsByInbound == nil {
		s.hostsByInbound = map[int][]*model.Host{}
	}
	if s.hostsLoaded == nil {
		s.hostsLoaded = map[int]bool{}
	}
	s.hostsLoaded[inbound.Id] = true
	var hosts []*model.Host
	if database.GetDB() == nil {
		return nil
	}
	if err := database.GetDB().Where("inbound_id = ?", inbound.Id).
		Order("sort_order asc, id asc").Find(&hosts).Error; err != nil {
		logger.Warning("SubService - hostsForInbound:", err)
		return nil
	}
	s.hostsByInbound[inbound.Id] = hosts
	return hosts
}

// clientGroupHostBinding returns this inbound's rows in host groups bound to the
// client's group, including disabled rows so the caller can fail closed.
func (s *SubService) clientGroupHostBinding(inbound *model.Inbound, client model.Client) (hostBinding, []*model.Host) {
	s.ensureClientGroupIndex()
	name := strings.ToLower(strings.TrimSpace(client.Group))
	if name == "" {
		return hostBindingNone, nil
	}
	groupID, ok := s.clientGroupIDsByName[name]
	if !ok {
		return hostBindingNone, nil
	}
	bound := s.hostGroupsByClientGroup[groupID]
	if len(bound) == 0 {
		return hostBindingNone, nil
	}
	var here []*model.Host
	for _, host := range s.hostsForInbound(inbound) {
		if host.GroupId == "" {
			continue
		}
		if _, ok := bound[host.GroupId]; ok {
			here = append(here, host)
		}
	}
	if len(here) == 0 {
		return hostBindingNone, nil
	}
	return hostBindingBound, here
}

// hostEndpointsForHosts projects a specific host set, applying format exclusion
// after the tier has been chosen. Empty means the tier is selected but unusable.
func (s *SubService) hostEndpointsForHosts(inbound *model.Inbound, hosts []*model.Host, format string) []map[string]any {
	defaultDest := s.resolveInboundAddress(inbound)
	eps := make([]map[string]any, 0, len(hosts))
	for _, host := range hosts {
		if host.IsDisabled || slices.Contains(host.ExcludeFromSubTypes, format) {
			continue
		}
		eps = append(eps, s.hostEndpointMaps(host, defaultDest, inbound.Port)...)
	}
	return eps
}

// selectHostEndpoints is the single tier decision: per-client binding, then the
// client's group, then the caller's default host set.
func (s *SubService) selectHostEndpoints(inbound *model.Inbound, client model.Client, format string, baseEps []map[string]any) ([]map[string]any, bool) {
	if eps, bound := s.clientHostEndpoints(inbound, client, format, baseEps); bound {
		return eps, true
	}
	state, hosts := s.clientGroupHostBinding(inbound, client)
	if state != hostBindingBound {
		return baseEps, false
	}
	eps := s.hostEndpointsForHosts(inbound, hosts, format)
	if len(eps) == 0 {
		return nil, true
	}
	return eps, true
}

// selectHostVariants is selectHostEndpoints for hosts-first ordering. A selected
// tier that loses every host to disablement or format exclusion fails closed.
func (s *SubService) selectHostVariants(inbound *model.Inbound, client model.Client, format string) ([]subscriptionHostVariant, bool) {
	state, _ := s.clientHostBinding(inbound, client)
	switch state {
	case hostBindingClosed:
		return nil, true
	case hostBindingBound:
		return s.clientHostVariants(inbound, client, format)
	}
	groupState, groupHosts := s.clientGroupHostBinding(inbound, client)
	if groupState == hostBindingBound {
		variants := s.hostVariantsForHosts(inbound, groupHosts, format)
		return variants, len(variants) == 0
	}
	return s.subscriptionHostVariants(inbound, format), false
}

// hostVariantsForHosts orders specific hosts by the same host-group order the
// default variants use, without adding the legacy default variant.
func (s *SubService) hostVariantsForHosts(inbound *model.Inbound, hosts []*model.Host, format string) []subscriptionHostVariant {
	var variants []subscriptionHostVariant
	for _, host := range hosts {
		if host.IsDisabled || slices.Contains(host.ExcludeFromSubTypes, format) {
			continue
		}
		order := s.hostGroupOrders[hostOrderGroupID(host)]
		order.inboundSort, order.inboundID, order.hostID = inbound.SubSortIndex, inbound.Id, host.Id
		variants = append(variants, subscriptionHostVariant{host, &order})
	}
	return variants
}
