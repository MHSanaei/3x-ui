package service

import (
	"sort"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"gorm.io/gorm"
)

// ClientGroupHostAssignmentService manages host-group defaults for client groups.
// Explicit per-client host bindings remain owned by the existing client service.
type ClientGroupHostAssignmentService struct{}

func normalizeAssignedHostGroupIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (s *ClientGroupHostAssignmentService) GetGroupIDs(groupName string) ([]string, error) {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return []string{}, nil
	}
	var ids []string
	err := database.GetDB().Model(&model.ClientGroupHost{}).
		Where("group_name = ?", groupName).
		Order("host_group_id ASC").
		Pluck("host_group_id", &ids).Error
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

// SetGroupIDs replaces the group defaults atomically. Empty IDs clear the
// assignment, restoring the panel's normal all-host subscription behavior.
func (s *ClientGroupHostAssignmentService) SetGroupIDs(groupName string, ids []string) error {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return common.NewError("client group name is required")
	}
	ids = normalizeAssignedHostGroupIDs(ids)

	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		var storedGroupCount int64
		if err := tx.Model(&model.ClientGroup{}).Where("name = ?", groupName).Count(&storedGroupCount).Error; err != nil {
			return err
		}
		if storedGroupCount == 0 {
			var clientCount int64
			if err := tx.Model(&model.ClientRecord{}).Where("group_name = ?", groupName).Count(&clientCount).Error; err != nil {
				return err
			}
			if clientCount == 0 {
				return common.NewError("client group not found:", groupName)
			}
			// A group that only exists as a client label would disappear from
			// ListGroups after its last member leaves. Persist it when assigning
			// defaults so the HostGroup configuration remains attached to a
			// visible, stable group. Clearing an assignment does not create a row.
			if len(ids) > 0 {
				if err := tx.Create(&model.ClientGroup{Name: groupName}).Error; err != nil {
					return err
				}
			}
		}

		if len(ids) > 0 {
			var existing []string
			if err := tx.Model(&model.Host{}).
				Where("group_id IN ?", ids).
				Distinct().
				Pluck("group_id", &existing).Error; err != nil {
				return err
			}
			existingSet := make(map[string]struct{}, len(existing))
			for _, id := range existing {
				existingSet[id] = struct{}{}
			}
			for _, id := range ids {
				if _, ok := existingSet[id]; !ok {
					return common.NewError("host group not found:", id)
				}
			}
		}

		if err := tx.Where("group_name = ?", groupName).Delete(&model.ClientGroupHost{}).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		rows := make([]model.ClientGroupHost, 0, len(ids))
		for _, id := range ids {
			rows = append(rows, model.ClientGroupHost{GroupName: groupName, HostGroupId: id})
		}
		return tx.Create(&rows).Error
	})
	if err != nil {
		return err
	}
	InvalidateSubData()
	return nil
}
