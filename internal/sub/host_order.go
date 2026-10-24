package sub

import (
	"cmp"
	"fmt"
	"slices"
	"sort"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type hostEntryOrder struct {
	sortOrder, inboundSort, inboundID, hostID int
	remark, groupID                           string
}

type orderedSubEntry[T any] struct {
	order *hostEntryOrder
	value T
}

// Group ties match the Hosts page, then inbound order and host row ID.
// Stable sorting preserves emission order inside each host.
func hostOrderLess(a, b *hostEntryOrder) bool {
	if a == nil || b == nil {
		return a != nil && b == nil
	}
	for _, c := range []int{
		cmp.Compare(a.sortOrder, b.sortOrder), cmp.Compare(a.remark, b.remark), cmp.Compare(a.groupID, b.groupID),
		cmp.Compare(a.inboundSort, b.inboundSort), cmp.Compare(a.inboundID, b.inboundID), cmp.Compare(a.hostID, b.hostID),
	} {
		if c != 0 {
			return c < 0
		}
	}
	return false
}

func orderedSubValues[T any](entries []orderedSubEntry[T]) []T {
	sort.SliceStable(entries, func(i, j int) bool { return hostOrderLess(entries[i].order, entries[j].order) })
	var out []T
	for _, entry := range entries {
		out = append(out, entry.value)
	}
	return out
}

func (s *SubService) loadSubscriptionHosts(inbounds []*model.Inbound) error {
	s.hostsByInbound = make(map[int][]*model.Host, len(inbounds))
	s.hostGroupOrders = make(map[string]hostEntryOrder)
	ids := make([]int, len(inbounds))
	for i, inbound := range inbounds {
		ids[i] = inbound.Id
	}
	for lo := 0; lo < len(ids); lo += 400 {
		var rows []*model.Host
		if err := database.GetDB().Where("inbound_id IN ?", ids[lo:min(lo+400, len(ids))]).Order("inbound_id ASC, sort_order ASC, id ASC").Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			s.hostsByInbound[row.InboundId] = append(s.hostsByInbound[row.InboundId], row)
			key := hostOrderGroupID(row)
			if order, exists := s.hostGroupOrders[key]; exists {
				order.sortOrder = min(order.sortOrder, row.SortOrder)
				s.hostGroupOrders[key] = order
			} else {
				s.hostGroupOrders[key] = hostEntryOrder{sortOrder: row.SortOrder, remark: row.Remark, groupID: key}
			}
		}
	}
	return nil
}

type subscriptionHostVariant struct {
	host  *model.Host
	order *hostEntryOrder
}

func hostOrderGroupID(h *model.Host) string {
	if h.GroupId != "" {
		return h.GroupId
	}
	return fmt.Sprintf("fallback_%d", h.Id)
}

func (s *SubService) subscriptionHostVariants(inbound *model.Inbound, format string) []subscriptionHostVariant {
	var variants []subscriptionHostVariant
	for _, h := range s.hostsByInbound[inbound.Id] {
		if h.IsDisabled || slices.Contains(h.ExcludeFromSubTypes, format) {
			continue
		}
		order := s.hostGroupOrders[hostOrderGroupID(h)]
		order.inboundSort, order.inboundID, order.hostID = inbound.SubSortIndex, inbound.Id, h.Id
		variants = append(variants, subscriptionHostVariant{h, &order})
	}
	if len(variants) == 0 {
		return []subscriptionHostVariant{{}}
	}
	return variants
}

func (v subscriptionHostVariant) endpoint(s *SubService, inbound *model.Inbound) []map[string]any {
	if v.host == nil {
		return nil
	}
	return s.hostEndpointMaps(v.host, s.resolveInboundAddress(inbound), inbound.Port)
}

func (v subscriptionHostVariant) inbound(s *SubService, inbound *model.Inbound) *model.Inbound {
	clone := *inbound
	if ep := v.endpoint(s, inbound); len(ep) > 0 {
		injectExternalProxy(&clone, ep)
	}
	return &clone
}
