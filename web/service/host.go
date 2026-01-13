// Package service provides Host management service for multi-node mode.
package service

import (
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"

	"gorm.io/gorm"
)

// HostService provides business logic for managing hosts.
type HostService struct{}

// GetHosts retrieves all hosts for a specific user.
func (s *HostService) GetHosts(userId int) ([]*model.Host, error) {
	db := database.GetDB()
	var hosts []*model.Host
	err := db.Where("user_id = ?", userId).Find(&hosts).Error
	if err != nil {
		return nil, err
	}

	// Load inbound assignments for each host
	for _, host := range hosts {
		inboundIds, err := s.GetInboundIdsForHost(host.Id)
		if err == nil {
			host.InboundIds = inboundIds
		}
	}

	return hosts, nil
}

// GetHost retrieves a host by ID.
func (s *HostService) GetHost(id int) (*model.Host, error) {
	db := database.GetDB()
	var host model.Host
	err := db.First(&host, id).Error
	if err != nil {
		return nil, err
	}

	// Load inbound assignments
	inboundIds, err := s.GetInboundIdsForHost(host.Id)
	if err == nil {
		host.InboundIds = inboundIds
	}

	return &host, nil
}

// GetInboundIdsForHost retrieves all inbound IDs assigned to a host.
func (s *HostService) GetInboundIdsForHost(hostId int) ([]int, error) {
	db := database.GetDB()
	var mappings []model.HostInboundMapping
	err := db.Where("host_id = ?", hostId).Find(&mappings).Error
	if err != nil {
		return nil, err
	}

	inboundIds := make([]int, len(mappings))
	for i, mapping := range mappings {
		inboundIds[i] = mapping.InboundId
	}

	return inboundIds, nil
}

// GetHostForInbound retrieves the host assigned to an inbound (if any).
// Returns the first enabled host if multiple hosts are assigned.
func (s *HostService) GetHostForInbound(inboundId int) (*model.Host, error) {
	db := database.GetDB()
	var mapping model.HostInboundMapping
	err := db.Where("inbound_id = ?", inboundId).First(&mapping).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No host assigned
		}
		return nil, err
	}

	var host model.Host
	err = db.Where("id = ? AND enable = ?", mapping.HostId, true).First(&host).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Host disabled or not found
		}
		return nil, err
	}

	return &host, nil
}

// AddHost creates a new host.
func (s *HostService) AddHost(userId int, host *model.Host) error {
	host.UserId = userId

	// Set timestamps
	now := time.Now().Unix()
	if host.CreatedAt == 0 {
		host.CreatedAt = now
	}
	host.UpdatedAt = now

	db := database.GetDB()
	tx := db.Begin()
	var err error
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	err = tx.Create(host).Error
	if err != nil {
		return err
	}

	// Assign to inbounds if provided
	if len(host.InboundIds) > 0 {
		err = s.AssignHostToInbounds(tx, host.Id, host.InboundIds)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateHost updates an existing host.
func (s *HostService) UpdateHost(userId int, host *model.Host) error {
	// Check if host exists and belongs to user
	existing, err := s.GetHost(host.Id)
	if err != nil {
		return err
	}
	if existing.UserId != userId {
		return common.NewError("Host not found or access denied")
	}

	// Update timestamp
	host.UpdatedAt = time.Now().Unix()

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Update only provided fields
	updates := make(map[string]interface{})
	if host.Name != "" {
		updates["name"] = host.Name
	}
	if host.Address != "" {
		updates["address"] = host.Address
	}
	if host.Port > 0 {
		updates["port"] = host.Port
	}
	if host.Protocol != "" {
		updates["protocol"] = host.Protocol
	}
	if host.Remark != "" {
		updates["remark"] = host.Remark
	}
	updates["enable"] = host.Enable
	updates["updated_at"] = host.UpdatedAt

	err = tx.Model(&model.Host{}).Where("id = ? AND user_id = ?", host.Id, userId).Updates(updates).Error
	if err != nil {
		return err
	}

	// Update inbound assignments if provided
	if host.InboundIds != nil {
		// Remove existing assignments
		err = tx.Where("host_id = ?", host.Id).Delete(&model.HostInboundMapping{}).Error
		if err != nil {
			return err
		}

		// Add new assignments
		if len(host.InboundIds) > 0 {
			err = s.AssignHostToInbounds(tx, host.Id, host.InboundIds)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DeleteHost deletes a host by ID.
func (s *HostService) DeleteHost(userId int, id int) error {
	// Check if host exists and belongs to user
	existing, err := s.GetHost(id)
	if err != nil {
		return err
	}
	if existing.UserId != userId {
		return common.NewError("Host not found or access denied")
	}

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Delete inbound mappings
	err = tx.Where("host_id = ?", id).Delete(&model.HostInboundMapping{}).Error
	if err != nil {
		return err
	}

	// Delete host
	err = tx.Where("id = ? AND user_id = ?", id, userId).Delete(&model.Host{}).Error
	if err != nil {
		return err
	}

	return nil
}

// AssignHostToInbounds assigns a host to multiple inbounds.
func (s *HostService) AssignHostToInbounds(tx *gorm.DB, hostId int, inboundIds []int) error {
	for _, inboundId := range inboundIds {
		mapping := &model.HostInboundMapping{
			HostId:    hostId,
			InboundId: inboundId,
		}
		err := tx.Create(mapping).Error
		if err != nil {
			logger.Warningf("Failed to assign host %d to inbound %d: %v", hostId, inboundId, err)
			// Continue with other assignments
		}
	}
	return nil
}
