package service

import (
	"sort"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// HostService manages Host rows (override endpoints attached to an inbound).
// Mirrors the empty-struct + database.GetDB() shape of ClientService.
type HostService struct{}

// GetHosts returns every host, grouped by inbound then ordered by sort_order.
func (s *HostService) GetHosts() ([]*model.Host, error) {
	var hosts []*model.Host
	err := database.GetDB().Order("inbound_id asc, sort_order asc, id asc").Find(&hosts).Error
	return hosts, err
}

// GetHostsByInbound returns one inbound's hosts ordered by sort_order then id.
func (s *HostService) GetHostsByInbound(inboundId int) ([]*model.Host, error) {
	var hosts []*model.Host
	err := database.GetDB().Where("inbound_id = ?", inboundId).Order("sort_order asc, id asc").Find(&hosts).Error
	return hosts, err
}

func (s *HostService) GetHost(id int) (*model.Host, error) {
	host := &model.Host{}
	if err := database.GetDB().First(host, id).Error; err != nil {
		return nil, err
	}
	return host, nil
}

// AddHost creates a host after confirming its inbound exists (no hard FK).
func (s *HostService) AddHost(host *model.Host) (*model.Host, error) {
	db := database.GetDB()
	var count int64
	if err := db.Model(&model.Inbound{}).Where("id = ?", host.InboundId).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, common.NewError("inbound not found")
	}
	host.Id = 0
	if err := db.Create(host).Error; err != nil {
		return nil, err
	}
	return host, nil
}

// UpdateHost overwrites a host's content. InboundId and SortOrder are immutable
// here — the inbound is fixed at creation and ordering is owned by ReorderHosts.
func (s *HostService) UpdateHost(id int, host *model.Host) (*model.Host, error) {
	db := database.GetDB()
	existing := &model.Host{}
	if err := db.First(existing, id).Error; err != nil {
		return nil, err
	}
	host.Id = id
	host.InboundId = existing.InboundId
	host.SortOrder = existing.SortOrder
	host.CreatedAt = existing.CreatedAt
	if err := db.Save(host).Error; err != nil {
		return nil, err
	}
	return s.GetHost(id)
}

func (s *HostService) DeleteHost(id int) error {
	return database.GetDB().Delete(&model.Host{}, id).Error
}

func (s *HostService) SetHostEnable(id int, enable bool) error {
	return database.GetDB().Model(&model.Host{}).Where("id = ?", id).Update("is_disabled", !enable).Error
}

func (s *HostService) SetHostsEnable(ids []int, enable bool) error {
	if len(ids) == 0 {
		return nil
	}
	return database.GetDB().Model(&model.Host{}).Where("id IN ?", ids).Update("is_disabled", !enable).Error
}

func (s *HostService) DeleteHosts(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return database.GetDB().Where("id IN ?", ids).Delete(&model.Host{}).Error
}

// ReorderHosts assigns sort_order by the position of each id in ids, in a single
// transaction (driver-safe on SQLite and Postgres).
func (s *HostService) ReorderHosts(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	tx := database.GetDB().Begin()
	for i, id := range ids {
		if err := tx.Model(&model.Host{}).Where("id = ?", id).Update("sort_order", i).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// GetAllTags returns the distinct, sorted set of tags across all hosts.
func (s *HostService) GetAllTags() ([]string, error) {
	hosts, err := s.GetHosts()
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
