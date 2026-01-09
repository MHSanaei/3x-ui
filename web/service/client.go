// Package service provides Client management service.
package service

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/util/random"
	"github.com/mhsanaei/3x-ui/v2/xray"

	"gorm.io/gorm"
)

// ClientService provides business logic for managing clients.
type ClientService struct{}

// GetClients retrieves all clients for a specific user.
// Also loads traffic statistics and last online time for each client.
func (s *ClientService) GetClients(userId int) ([]*model.ClientEntity, error) {
	db := database.GetDB()
	var clients []*model.ClientEntity
	err := db.Where("user_id = ?", userId).Find(&clients).Error
	if err != nil {
		return nil, err
	}

	// Load inbound assignments, traffic statistics, and HWIDs for each client
	for _, client := range clients {
		// Load inbound assignments
		inboundIds, err := s.GetInboundIdsForClient(client.Id)
		if err == nil {
			client.InboundIds = inboundIds
		}

		// Load traffic statistics from client_traffics table by email
		var clientTraffic xray.ClientTraffic
		err = db.Where("email = ?", strings.ToLower(client.Email)).First(&clientTraffic).Error
		if err == nil {
			// Traffic found - set traffic fields on client entity
			client.Up = clientTraffic.Up
			client.Down = clientTraffic.Down
			client.AllTime = clientTraffic.AllTime
			client.LastOnline = clientTraffic.LastOnline
			// Note: expiryTime and totalGB are stored in ClientEntity, we don't override them from traffic
			// Traffic table may have different values due to legacy data
		} else if err != gorm.ErrRecordNotFound {
			logger.Warningf("Failed to load traffic for client %s: %v", client.Email, err)
		}
		// If not found, traffic will be 0 (default values)

		// Load HWIDs for this client
		hwidService := ClientHWIDService{}
		hwids, err := hwidService.GetHWIDsForClient(client.Id)
		if err == nil {
			client.HWIDs = hwids
		} else {
			logger.Warningf("Failed to load HWIDs for client %d: %v", client.Id, err)
		}
	}

	return clients, nil
}

// GetClient retrieves a client by ID.
func (s *ClientService) GetClient(id int) (*model.ClientEntity, error) {
	db := database.GetDB()
	var client model.ClientEntity
	err := db.First(&client, id).Error
	if err != nil {
		return nil, err
	}

	// Load inbound assignments
	inboundIds, err := s.GetInboundIdsForClient(client.Id)
	if err == nil {
		client.InboundIds = inboundIds
	}

	// Load HWIDs for this client
	hwidService := ClientHWIDService{}
	hwids, err := hwidService.GetHWIDsForClient(client.Id)
	if err == nil {
		client.HWIDs = hwids
	}

	return &client, nil
}

// GetClientByEmail retrieves a client by email for a specific user.
func (s *ClientService) GetClientByEmail(userId int, email string) (*model.ClientEntity, error) {
	db := database.GetDB()
	var client model.ClientEntity
	err := db.Where("user_id = ? AND email = ?", userId, strings.ToLower(email)).First(&client).Error
	if err != nil {
		return nil, err
	}

	// Load inbound assignments
	inboundIds, err := s.GetInboundIdsForClient(client.Id)
	if err == nil {
		client.InboundIds = inboundIds
	}

	return &client, nil
}

// GetInboundIdsForClient retrieves all inbound IDs assigned to a client.
func (s *ClientService) GetInboundIdsForClient(clientId int) ([]int, error) {
	db := database.GetDB()
	var mappings []model.ClientInboundMapping
	err := db.Where("client_id = ?", clientId).Find(&mappings).Error
	if err != nil {
		return nil, err
	}

	inboundIds := make([]int, len(mappings))
	for i, mapping := range mappings {
		inboundIds[i] = mapping.InboundId
	}

	return inboundIds, nil
}

// AddClient creates a new client.
// Returns whether Xray needs restart and any error.
func (s *ClientService) AddClient(userId int, client *model.ClientEntity) (bool, error) {
	// Validate email uniqueness for this user
	existing, err := s.GetClientByEmail(userId, client.Email)
	if err == nil && existing != nil {
		return false, common.NewError("Client with email already exists: ", client.Email)
	}

	// Generate UUID if not provided and needed
	if client.UUID == "" {
		newUUID, err := uuid.NewRandom()
		if err != nil {
			return false, common.NewError("Failed to generate UUID: ", err.Error())
		}
		client.UUID = newUUID.String()
	}

	// Generate SubID if not provided
	if client.SubID == "" {
		client.SubID = random.Seq(16)
	}

	// Normalize email to lowercase
	client.Email = strings.ToLower(client.Email)
	client.UserId = userId

	// Set timestamps
	now := time.Now().Unix()
	if client.CreatedAt == 0 {
		client.CreatedAt = now
	}
	client.UpdatedAt = now

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	err = tx.Create(client).Error
	if err != nil {
		return false, err
	}

	// Create initial ClientTraffic record if it doesn't exist
	// This ensures traffic statistics are tracked from the start
	var count int64
	tx.Model(&xray.ClientTraffic{}).Where("email = ?", client.Email).Count(&count)
	if count == 0 {
		// Create traffic record for the first assigned inbound, or use 0 if no inbounds yet
		inboundId := 0
		if len(client.InboundIds) > 0 {
			inboundId = client.InboundIds[0]
		}
		clientTraffic := xray.ClientTraffic{
			InboundId:  inboundId,
			Email:      client.Email,
			Total:      client.TotalGB,
			ExpiryTime: client.ExpiryTime,
			Enable:     client.Enable,
			Up:         0,
			Down:       0,
			Reset:      client.Reset,
		}
		err = tx.Create(&clientTraffic).Error
		if err != nil {
			logger.Warningf("Failed to create ClientTraffic for client %s: %v", client.Email, err)
			// Don't fail the whole operation if traffic record creation fails
		}
	}

	// Assign to inbounds if provided
	if len(client.InboundIds) > 0 {
		err = s.AssignClientToInbounds(tx, client.Id, client.InboundIds)
		if err != nil {
			return false, err
		}
	}
	
	// Commit client transaction first to avoid nested transactions
	err = tx.Commit().Error
	if err != nil {
		return false, err
	}
	
	// Now update Settings for all assigned inbounds
	// This is done AFTER committing the client transaction to avoid nested transactions and database locks
	needRestart := false
	if len(client.InboundIds) > 0 {
		inboundService := InboundService{}
		for _, inboundId := range client.InboundIds {
			inbound, err := inboundService.GetInbound(inboundId)
			if err != nil {
				logger.Warningf("Failed to get inbound %d for settings update: %v", inboundId, err)
				continue
			}
			
			// Get all clients for this inbound (from ClientEntity)
			clientEntities, err := s.GetClientsForInbound(inboundId)
			if err != nil {
				logger.Warningf("Failed to get clients for inbound %d: %v", inboundId, err)
				continue
			}
			
			// Rebuild Settings from ClientEntity
			newSettings, err := inboundService.BuildSettingsFromClientEntities(inbound, clientEntities)
			if err != nil {
				logger.Warningf("Failed to build settings for inbound %d: %v", inboundId, err)
				continue
			}
			
			// Update inbound Settings (this will open its own transaction)
			// Use retry logic to handle database lock errors
			inbound.Settings = newSettings
			_, inboundNeedRestart, err := inboundService.updateInboundWithRetry(inbound)
			if err != nil {
				logger.Warningf("Failed to update inbound %d settings: %v", inboundId, err)
				// Continue with other inbounds
			} else if inboundNeedRestart {
				needRestart = true
			}
		}
	}

	return needRestart, nil
}

// UpdateClient updates an existing client.
// Returns whether Xray needs restart and any error.
func (s *ClientService) UpdateClient(userId int, client *model.ClientEntity) (bool, error) {
	// Check if client exists and belongs to user
	existing, err := s.GetClient(client.Id)
	if err != nil {
		return false, err
	}
	if existing.UserId != userId {
		return false, common.NewError("Client not found or access denied")
	}

	// Check email uniqueness if email changed
	if client.Email != "" && strings.ToLower(client.Email) != strings.ToLower(existing.Email) {
		existingByEmail, err := s.GetClientByEmail(userId, client.Email)
		if err == nil && existingByEmail != nil && existingByEmail.Id != client.Id {
			return false, common.NewError("Client with email already exists: ", client.Email)
		}
	}

	// Normalize email to lowercase if provided
	if client.Email != "" {
		client.Email = strings.ToLower(client.Email)
	}

	// Update timestamp
	client.UpdatedAt = time.Now().Unix()

	db := database.GetDB()
	tx := db.Begin()
	// Track if transaction was committed to avoid double rollback
	committed := false
	defer func() {
		// Only rollback if there was an error and transaction wasn't committed
		if err != nil && !committed {
			tx.Rollback()
		}
	}()

	// Update only provided fields
	updates := make(map[string]interface{})
	if client.Email != "" {
		updates["email"] = client.Email
	}
	if client.UUID != "" {
		updates["uuid"] = client.UUID
	}
	if client.Security != "" {
		updates["security"] = client.Security
	}
	if client.Password != "" {
		updates["password"] = client.Password
	}
	if client.Flow != "" {
		updates["flow"] = client.Flow
	}
	if client.LimitIP > 0 {
		updates["limit_ip"] = client.LimitIP
	}
	if client.TotalGB > 0 {
		updates["total_gb"] = client.TotalGB
	}
	if client.ExpiryTime != 0 {
		updates["expiry_time"] = client.ExpiryTime
	}
	updates["enable"] = client.Enable
	if client.TgID > 0 {
		updates["tg_id"] = client.TgID
	}
	if client.SubID != "" {
		updates["sub_id"] = client.SubID
	}
	if client.Comment != "" {
		updates["comment"] = client.Comment
	}
	if client.Reset > 0 {
		updates["reset"] = client.Reset
	}
	// Update HWID settings - GORM converts field names to snake_case automatically
	// HWIDEnabled -> hwid_enabled, MaxHWID -> max_hwid
	// But we need to check if columns exist first, or use direct field assignment
	updates["hwid_enabled"] = client.HWIDEnabled
	updates["max_hwid"] = client.MaxHWID
	updates["updated_at"] = client.UpdatedAt

	// First try to update with all fields including HWID
	err = tx.Model(&model.ClientEntity{}).Where("id = ? AND user_id = ?", client.Id, userId).Updates(updates).Error
	if err != nil {
		// If HWID columns don't exist, remove them and try again
		if strings.Contains(err.Error(), "no such column: hwid_enabled") || strings.Contains(err.Error(), "no such column: max_hwid") {
			delete(updates, "hwid_enabled")
			delete(updates, "max_hwid")
			err = tx.Model(&model.ClientEntity{}).Where("id = ? AND user_id = ?", client.Id, userId).Updates(updates).Error
		}
	}
	if err != nil {
		return false, err
	}
	
	// Get current inbound assignments to determine which inbounds need updating
	var currentMappings []model.ClientInboundMapping
	tx.Where("client_id = ?", client.Id).Find(&currentMappings)
	oldInboundIds := make(map[int]bool)
	for _, mapping := range currentMappings {
		oldInboundIds[mapping.InboundId] = true
	}
	
	// Track all affected inbounds (old + new) for settings update
	affectedInboundIds := make(map[int]bool)
	for inboundId := range oldInboundIds {
		affectedInboundIds[inboundId] = true
	}
	
	// Update inbound assignments if provided
	// Note: InboundIds is a slice, so we need to check if it was explicitly set
	// We'll always update if InboundIds is not nil (even if empty array means remove all)
	if client.InboundIds != nil {
		// Remove existing assignments
		err = tx.Where("client_id = ?", client.Id).Delete(&model.ClientInboundMapping{}).Error
		if err != nil {
			return false, err
		}

		// Add new assignments (if any)
		if len(client.InboundIds) > 0 {
			err = s.AssignClientToInbounds(tx, client.Id, client.InboundIds)
			if err != nil {
				return false, err
			}
			// Track new inbound IDs for settings update
			for _, inboundId := range client.InboundIds {
				affectedInboundIds[inboundId] = true
			}
		}
	}
	
	// Commit client transaction first to avoid nested transactions
	err = tx.Commit().Error
	committed = true
	if err != nil {
		return false, err
	}
	
	// Now update Settings for all affected inbounds (old + new)
	// This is needed even if InboundIds wasn't changed, because client data (UUID, password, etc.) might have changed
	// We do this AFTER committing the client transaction to avoid nested transactions and database locks
	needRestart := false
	inboundService := InboundService{}
	for inboundId := range affectedInboundIds {
		inbound, err := inboundService.GetInbound(inboundId)
		if err != nil {
			logger.Warningf("Failed to get inbound %d for settings update: %v", inboundId, err)
			continue
		}
		
		// Get all clients for this inbound (from ClientEntity)
		clientEntities, err := s.GetClientsForInbound(inboundId)
		if err != nil {
			logger.Warningf("Failed to get clients for inbound %d: %v", inboundId, err)
			continue
		}
		
		// Rebuild Settings from ClientEntity
		newSettings, err := inboundService.BuildSettingsFromClientEntities(inbound, clientEntities)
		if err != nil {
			logger.Warningf("Failed to build settings for inbound %d: %v", inboundId, err)
			continue
		}
		
		// Update inbound Settings (this will open its own transaction)
		// Use retry logic to handle database lock errors
		inbound.Settings = newSettings
		_, inboundNeedRestart, err := inboundService.updateInboundWithRetry(inbound)
		if err != nil {
			logger.Warningf("Failed to update inbound %d settings: %v", inboundId, err)
			// Continue with other inbounds
		} else if inboundNeedRestart {
			needRestart = true
		}
	}

	return needRestart, nil
}

// DeleteClient deletes a client by ID.
// Returns whether Xray needs restart and any error.
func (s *ClientService) DeleteClient(userId int, id int) (bool, error) {
	// Check if client exists and belongs to user
	existing, err := s.GetClient(id)
	if err != nil {
		return false, err
	}
	if existing.UserId != userId {
		return false, common.NewError("Client not found or access denied")
	}
	
	// Get inbound assignments before deleting
	var mappings []model.ClientInboundMapping
	db := database.GetDB()
	err = db.Where("client_id = ?", id).Find(&mappings).Error
	if err != nil {
		return false, err
	}
	
	affectedInboundIds := make(map[int]bool)
	for _, mapping := range mappings {
		affectedInboundIds[mapping.InboundId] = true
	}
	
	needRestart := false

	tx := db.Begin()
	// Track if transaction was committed to avoid double rollback
	committed := false
	defer func() {
		// Only rollback if there was an error and transaction wasn't committed
		if err != nil && !committed {
			tx.Rollback()
		}
	}()

	// Delete inbound mappings
	err = tx.Where("client_id = ?", id).Delete(&model.ClientInboundMapping{}).Error
	if err != nil {
		return false, err
	}

	// Delete client
	err = tx.Where("id = ? AND user_id = ?", id, userId).Delete(&model.ClientEntity{}).Error
	if err != nil {
		return false, err
	}
	
	// Commit deletion transaction first to avoid nested transactions
	err = tx.Commit().Error
	committed = true
	if err != nil {
		return false, err
	}
	
	// Update Settings for affected inbounds (after deletion)
	// We do this AFTER committing the deletion transaction to avoid nested transactions and database locks
	inboundService := InboundService{}
	for inboundId := range affectedInboundIds {
		inbound, err := inboundService.GetInbound(inboundId)
		if err != nil {
			logger.Warningf("Failed to get inbound %d for settings update: %v", inboundId, err)
			continue
		}
		
		// Get all remaining clients for this inbound (from ClientEntity)
		clientEntities, err := s.GetClientsForInbound(inboundId)
		if err != nil {
			logger.Warningf("Failed to get clients for inbound %d: %v", inboundId, err)
			continue
		}
		
		// Rebuild Settings from ClientEntity
		newSettings, err := inboundService.BuildSettingsFromClientEntities(inbound, clientEntities)
		if err != nil {
			logger.Warningf("Failed to build settings for inbound %d: %v", inboundId, err)
			continue
		}
		
		// Update inbound Settings (this will open its own transaction)
		// Use retry logic to handle database lock errors
		inbound.Settings = newSettings
		_, inboundNeedRestart, err := inboundService.updateInboundWithRetry(inbound)
		if err != nil {
			logger.Warningf("Failed to update inbound %d settings: %v", inboundId, err)
			// Continue with other inbounds
		} else if inboundNeedRestart {
			needRestart = true
		}
	}

	return needRestart, nil
}

// AssignClientToInbounds assigns a client to multiple inbounds.
func (s *ClientService) AssignClientToInbounds(tx *gorm.DB, clientId int, inboundIds []int) error {
	for _, inboundId := range inboundIds {
		mapping := &model.ClientInboundMapping{
			ClientId:  clientId,
			InboundId: inboundId,
		}
		err := tx.Create(mapping).Error
		if err != nil {
			logger.Warningf("Failed to assign client %d to inbound %d: %v", clientId, inboundId, err)
			// Continue with other assignments
		}
	}
	return nil
}

// GetClientsForInbound retrieves all clients assigned to an inbound.
func (s *ClientService) GetClientsForInbound(inboundId int) ([]*model.ClientEntity, error) {
	db := database.GetDB()
	var mappings []model.ClientInboundMapping
	err := db.Where("inbound_id = ?", inboundId).Find(&mappings).Error
	if err != nil {
		return nil, err
	}

	if len(mappings) == 0 {
		return []*model.ClientEntity{}, nil
	}

	clientIds := make([]int, len(mappings))
	for i, mapping := range mappings {
		clientIds[i] = mapping.ClientId
	}

	var clients []*model.ClientEntity
	err = db.Where("id IN ?", clientIds).Find(&clients).Error
	if err != nil {
		return nil, err
	}

	return clients, nil
}

// ConvertClientEntityToClient converts ClientEntity to legacy Client struct for backward compatibility.
func (s *ClientService) ConvertClientEntityToClient(entity *model.ClientEntity) model.Client {
	return model.Client{
		ID:         entity.UUID,
		Security:   entity.Security,
		Password:   entity.Password,
		Flow:       entity.Flow,
		Email:      entity.Email,
		LimitIP:    entity.LimitIP,
		TotalGB:    entity.TotalGB,
		ExpiryTime: entity.ExpiryTime,
		Enable:     entity.Enable,
		TgID:       entity.TgID,
		SubID:      entity.SubID,
		Comment:    entity.Comment,
		Reset:      entity.Reset,
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
	}
}
