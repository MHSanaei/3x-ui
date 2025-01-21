package job

import (
	"encoding/json"
	"fmt"
	"time"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/xray"

	"gorm.io/gorm"
)

type SyncClientTrafficJob struct {
	subClientsCollection map[string][]string
}

func NewClientTrafficSyncJob() *SyncClientTrafficJob {
	return new(SyncClientTrafficJob)
}
func (j *SyncClientTrafficJob) Run() {
	// Step 1: Group clients by SubID
	subClientsCollection := j.collectClientsGroupedBySubId()

	// Step 2: Sync client traffics for each SubID group
	for subId, emails := range subClientsCollection {
		err := j.syncClientTraffics(map[string][]string{subId: emails})
		if err != nil {
			logger.Error("Failed to sync traffics for SubID ", subId, ": ", err)
		}
	}
}

// collectClientsGroupedBySubId groups clients by their SubIDs
func (j *SyncClientTrafficJob) collectClientsGroupedBySubId() map[string][]string {
	db := database.GetDB()
	result := make(map[string][]string)

	// Fetch all inbounds
	var inbounds []*model.Inbound
	if err := db.Model(&model.Inbound{}).Find(&inbounds).Error; err != nil {
		logger.Error("Error fetching inbounds: ", err)
		return result // Return empty map on error
	}

	// Process each inbound
	for _, inbound := range inbounds {
		if inbound.Settings == "" {
			continue
		}

		settingsMap, err := parseSettings(inbound.Settings, uint(inbound.Id))
		if err != nil {
			logger.Error(err)
			continue
		}

		clients, ok := settingsMap["clients"].([]interface{})
		if !ok {
			continue
		}

		processClients(clients, result)
	}

	// Remove SubIDs with one or fewer emails
	filterSingleEmailSubIDs(result)

	return result
}

// parseSettings unmarshals the JSON settings and returns it as a map
func parseSettings(settings string, inboundID uint) (map[string]interface{}, error) {
	if !json.Valid([]byte(settings)) {
		return nil, fmt.Errorf("Invalid JSON format in Settings for inbound ID %d", inboundID)
	}

	var tempData map[string]interface{}
	if err := json.Unmarshal([]byte(settings), &tempData); err != nil {
		return nil, fmt.Errorf("Error unmarshalling settings for inbound ID %d: %v", inboundID, err)
	}

	return tempData, nil
}

// processClients extracts SubID and email from the clients and populates the result map
func processClients(clients []interface{}, result map[string][]string) {
	for _, client := range clients {
		clientMap, ok := client.(map[string]interface{})
		if !ok {
			continue
		}

		subId, ok := clientMap["subId"].(string)
		if !ok || subId == "" {
			continue
		}

		email, ok := clientMap["email"].(string)
		if !ok || email == "" {
			continue
		}

		result[subId] = append(result[subId], email)
	}
}

// filterSingleEmailSubIDs removes SubIDs with one or fewer emails from the result map
func filterSingleEmailSubIDs(result map[string][]string) {
	for subId, emails := range result {
		if len(emails) <= 1 {
			delete(result, subId)
		}
	}
}

// syncClientTraffics synchronizes traffic data for each SubID group
func (j *SyncClientTrafficJob) syncClientTraffics(result map[string][]string) error {
	for subId, emails := range result {
		db := database.GetDB()

		// Step 1: Calculate maxUp and maxDown (outside transaction)
		var maxUp, maxDown int64
		err := calculateMaxTraffic(db, emails, &maxUp, &maxDown)
		if err != nil {
			logger.Error("Failed to calculate max traffic for SubID ", subId, ": ", err)
			continue
		}

		// Step 2: Update traffic data with retry mechanism
		err = retryOperation(func() error {
			return updateTraffic(db, emails, maxUp, maxDown)
		}, 5, 100*time.Millisecond)

		if err != nil {
			logger.Error("Failed to update client traffics for SubID ", subId, ": ", err)
		}
	}
	return nil
}

// calculateMaxTraffic calculates max up and down traffic for a group of emails
func calculateMaxTraffic(db *gorm.DB, emails []string, maxUp, maxDown *int64) error {
	return db.Model(&xray.ClientTraffic{}).
		Where("email IN ?", emails).
		Select("MAX(up) AS max_up, MAX(down) AS max_down").
		Row().
		Scan(maxUp, maxDown)
}

// updateTraffic updates the traffic data in the database within a transaction
func updateTraffic(db *gorm.DB, emails []string, maxUp, maxDown int64) error {
	return db.Transaction(func(tx *gorm.DB) error {
		return tx.Model(&xray.ClientTraffic{}).
			Where("email IN ?", emails).
			Updates(map[string]interface{}{
				"up":   maxUp,
				"down": maxDown,
			}).Error
	})
}

// retryOperation retries an operation multiple times with a delay
func retryOperation(operation func() error, maxRetries int, delay time.Duration) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = operation()
		if err == nil {
			return nil
		}
		logger.Info(fmt.Sprintf("Retry %d/%d failed: %v", i+1, maxRetries, err))
		time.Sleep(delay)
	}
	return err
}

