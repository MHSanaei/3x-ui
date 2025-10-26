// Package service provides business logic services for the 3x-ui web panel,
// including inbound/outbound management, user administration, settings, and Xray integration.
package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/xray"

	"gorm.io/gorm"
)

// InboundService provides business logic for managing Xray inbound configurations.
// It handles CRUD operations for inbounds, client management, traffic monitoring,
// and integration with the Xray API for real-time updates.
type InboundService struct {
	xrayApi xray.XrayAPI
}

// GetInbounds retrieves all inbounds for a specific user.
// Returns a slice of inbound models with their associated client statistics.
func (s *InboundService) GetInbounds(userId int) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Where("user_id = ?", userId).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	// Enrich client stats with UUID/SubId from inbound settings
	for _, inbound := range inbounds {
		clients, _ := s.GetClients(inbound)
		if len(clients) == 0 || len(inbound.ClientStats) == 0 {
			continue
		}
		// Build a map email -> client
		cMap := make(map[string]model.Client, len(clients))
		for _, c := range clients {
			cMap[strings.ToLower(c.Email)] = c
		}
		for i := range inbound.ClientStats {
			email := strings.ToLower(inbound.ClientStats[i].Email)
			if c, ok := cMap[email]; ok {
				inbound.ClientStats[i].UUID = c.ID
				inbound.ClientStats[i].SubId = c.SubID
			}
		}
	}
	return inbounds, nil
}

// GetAllInbounds retrieves all inbounds from the database.
// Returns a slice of all inbound models with their associated client statistics.
func (s *InboundService) GetAllInbounds() ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	// Enrich client stats with UUID/SubId from inbound settings
	for _, inbound := range inbounds {
		clients, _ := s.GetClients(inbound)
		if len(clients) == 0 || len(inbound.ClientStats) == 0 {
			continue
		}
		cMap := make(map[string]model.Client, len(clients))
		for _, c := range clients {
			cMap[strings.ToLower(c.Email)] = c
		}
		for i := range inbound.ClientStats {
			email := strings.ToLower(inbound.ClientStats[i].Email)
			if c, ok := cMap[email]; ok {
				inbound.ClientStats[i].UUID = c.ID
				inbound.ClientStats[i].SubId = c.SubID
			}
		}
	}
	return inbounds, nil
}

func (s *InboundService) GetInboundsByTrafficReset(period string) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Where("traffic_reset = ?", period).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) checkPortExist(listen string, port int, ignoreId int) (bool, error) {
	db := database.GetDB()
	if listen == "" || listen == "0.0.0.0" || listen == "::" || listen == "::0" {
		db = db.Model(model.Inbound{}).Where("port = ?", port)
	} else {
		db = db.Model(model.Inbound{}).
			Where("port = ?", port).
			Where(
				db.Model(model.Inbound{}).Where(
					"listen = ?", listen,
				).Or(
					"listen = \"\"",
				).Or(
					"listen = \"0.0.0.0\"",
				).Or(
					"listen = \"::\"",
				).Or(
					"listen = \"::0\""))
	}
	if ignoreId > 0 {
		db = db.Where("id != ?", ignoreId)
	}
	var count int64
	err := db.Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *InboundService) GetClients(inbound *model.Inbound) ([]model.Client, error) {
	settings := map[string][]model.Client{}
	json.Unmarshal([]byte(inbound.Settings), &settings)
	if settings == nil {
		return nil, fmt.Errorf("setting is null")
	}

	clients := settings["clients"]
	if clients == nil {
		return nil, nil
	}
	return clients, nil
}

func (s *InboundService) getAllEmails() ([]string, error) {
	db := database.GetDB()
	var emails []string
	err := db.Raw(`
		SELECT JSON_EXTRACT(client.value, '$.email')
		FROM inbounds,
			JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		`).Scan(&emails).Error
	if err != nil {
		return nil, err
	}
	return emails, nil
}

func (s *InboundService) contains(slice []string, str string) bool {
	lowerStr := strings.ToLower(str)
	for _, s := range slice {
		if strings.ToLower(s) == lowerStr {
			return true
		}
	}
	return false
}

func (s *InboundService) checkEmailsExistForClients(clients []model.Client) (string, error) {
	allEmails, err := s.getAllEmails()
	if err != nil {
		return "", err
	}
	var emails []string
	for _, client := range clients {
		if client.Email != "" {
			if s.contains(emails, client.Email) {
				return client.Email, nil
			}
			if s.contains(allEmails, client.Email) {
				return client.Email, nil
			}
			emails = append(emails, client.Email)
		}
	}
	return "", nil
}

func (s *InboundService) checkEmailExistForInbound(inbound *model.Inbound) (string, error) {
	clients, err := s.GetClients(inbound)
	if err != nil {
		return "", err
	}
	allEmails, err := s.getAllEmails()
	if err != nil {
		return "", err
	}
	var emails []string
	for _, client := range clients {
		if client.Email != "" {
			if s.contains(emails, client.Email) {
				return client.Email, nil
			}
			if s.contains(allEmails, client.Email) {
				return client.Email, nil
			}
			emails = append(emails, client.Email)
		}
	}
	return "", nil
}

// AddInbound creates a new inbound configuration.
// It validates port uniqueness, client email uniqueness, and required fields,
// then saves the inbound to the database and optionally adds it to the running Xray instance.
// Returns the created inbound, whether Xray needs restart, and any error.
func (s *InboundService) AddInbound(inbound *model.Inbound) (*model.Inbound, bool, error) {
	exist, err := s.checkPortExist(inbound.Listen, inbound.Port, 0)
	if err != nil {
		return inbound, false, err
	}
	if exist {
		return inbound, false, common.NewError("Port already exists:", inbound.Port)
	}

	existEmail, err := s.checkEmailExistForInbound(inbound)
	if err != nil {
		return inbound, false, err
	}
	if existEmail != "" {
		return inbound, false, common.NewError("Duplicate email:", existEmail)
	}

	clients, err := s.GetClients(inbound)
	if err != nil {
		return inbound, false, err
	}

	// Ensure created_at and updated_at on clients in settings
	if len(clients) > 0 {
		var settings map[string]any
		if err2 := json.Unmarshal([]byte(inbound.Settings), &settings); err2 == nil && settings != nil {
			now := time.Now().Unix() * 1000
			updatedClients := make([]model.Client, 0, len(clients))
			for _, c := range clients {
				if c.CreatedAt == 0 {
					c.CreatedAt = now
				}
				c.UpdatedAt = now
				updatedClients = append(updatedClients, c)
			}
			settings["clients"] = updatedClients
			if bs, err3 := json.MarshalIndent(settings, "", "  "); err3 == nil {
				inbound.Settings = string(bs)
			} else {
				logger.Debug("Unable to marshal inbound settings with timestamps:", err3)
			}
		} else if err2 != nil {
			logger.Debug("Unable to parse inbound settings for timestamps:", err2)
		}
	}

	// Secure client ID
	for _, client := range clients {
		switch inbound.Protocol {
		case "trojan":
			if client.Password == "" {
				return inbound, false, common.NewError("empty client ID")
			}
		case "shadowsocks":
			if client.Email == "" {
				return inbound, false, common.NewError("empty client ID")
			}
		default:
			if client.ID == "" {
				return inbound, false, common.NewError("empty client ID")
			}
		}
	}

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	err = tx.Save(inbound).Error
	if err == nil {
		if len(inbound.ClientStats) == 0 {
			for _, client := range clients {
				s.AddClientStat(tx, inbound.Id, &client)
			}
		}
	} else {
		return inbound, false, err
	}

	needRestart := false
	if inbound.Enable {
		s.xrayApi.Init(p.GetAPIPort())
		inboundJson, err1 := json.MarshalIndent(inbound.GenXrayInboundConfig(), "", "  ")
		if err1 != nil {
			logger.Debug("Unable to marshal inbound config:", err1)
		}

		err1 = s.xrayApi.AddInbound(inboundJson)
		if err1 == nil {
			logger.Debug("New inbound added by api:", inbound.Tag)
		} else {
			logger.Debug("Unable to add inbound by api:", err1)
			needRestart = true
		}
		s.xrayApi.Close()
	}

	return inbound, needRestart, err
}

// DelInbound deletes an inbound configuration by ID.
// It removes the inbound from the database and the running Xray instance if active.
// Returns whether Xray needs restart and any error.
func (s *InboundService) DelInbound(id int) (bool, error) {
	db := database.GetDB()

	var tag string
	needRestart := false
	result := db.Model(model.Inbound{}).Select("tag").Where("id = ? and enable = ?", id, true).First(&tag)
	if result.Error == nil {
		s.xrayApi.Init(p.GetAPIPort())
		err1 := s.xrayApi.DelInbound(tag)
		if err1 == nil {
			logger.Debug("Inbound deleted by api:", tag)
		} else {
			logger.Debug("Unable to delete inbound by api:", err1)
			needRestart = true
		}
		s.xrayApi.Close()
	} else {
		logger.Debug("No enabled inbound founded to removing by api", tag)
	}

	// Delete client traffics of inbounds
	err := db.Where("inbound_id = ?", id).Delete(xray.ClientTraffic{}).Error
	if err != nil {
		return false, err
	}
	inbound, err := s.GetInbound(id)
	if err != nil {
		return false, err
	}
	clients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}
	for _, client := range clients {
		err := s.DelClientIPs(db, client.Email)
		if err != nil {
			return false, err
		}
	}

	return needRestart, db.Delete(model.Inbound{}, id).Error
}

func (s *InboundService) GetInbound(id int) (*model.Inbound, error) {
	db := database.GetDB()
	inbound := &model.Inbound{}
	err := db.Model(model.Inbound{}).First(inbound, id).Error
	if err != nil {
		return nil, err
	}
	return inbound, nil
}

// UpdateInbound modifies an existing inbound configuration.
// It validates changes, updates the database, and syncs with the running Xray instance.
// Returns the updated inbound, whether Xray needs restart, and any error.
func (s *InboundService) UpdateInbound(inbound *model.Inbound) (*model.Inbound, bool, error) {
	exist, err := s.checkPortExist(inbound.Listen, inbound.Port, inbound.Id)
	if err != nil {
		return inbound, false, err
	}
	if exist {
		return inbound, false, common.NewError("Port already exists:", inbound.Port)
	}

	oldInbound, err := s.GetInbound(inbound.Id)
	if err != nil {
		return inbound, false, err
	}

	tag := oldInbound.Tag

	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	err = s.updateClientTraffics(tx, oldInbound, inbound)
	if err != nil {
		return inbound, false, err
	}

	// Ensure created_at and updated_at exist in inbound.Settings clients
	{
		var oldSettings map[string]any
		_ = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
		emailToCreated := map[string]int64{}
		emailToUpdated := map[string]int64{}
		if oldSettings != nil {
			if oc, ok := oldSettings["clients"].([]any); ok {
				for _, it := range oc {
					if m, ok2 := it.(map[string]any); ok2 {
						if email, ok3 := m["email"].(string); ok3 {
							switch v := m["created_at"].(type) {
							case float64:
								emailToCreated[email] = int64(v)
							case int64:
								emailToCreated[email] = v
							}
							switch v := m["updated_at"].(type) {
							case float64:
								emailToUpdated[email] = int64(v)
							case int64:
								emailToUpdated[email] = v
							}
						}
					}
				}
			}
		}
		var newSettings map[string]any
		if err2 := json.Unmarshal([]byte(inbound.Settings), &newSettings); err2 == nil && newSettings != nil {
			now := time.Now().Unix() * 1000
			if nSlice, ok := newSettings["clients"].([]any); ok {
				for i := range nSlice {
					if m, ok2 := nSlice[i].(map[string]any); ok2 {
						email, _ := m["email"].(string)
						if _, ok3 := m["created_at"]; !ok3 {
							if v, ok4 := emailToCreated[email]; ok4 && v > 0 {
								m["created_at"] = v
							} else {
								m["created_at"] = now
							}
						}
						// Preserve client's updated_at if present; do not bump on parent inbound update
						if _, hasUpdated := m["updated_at"]; !hasUpdated {
							if v, ok4 := emailToUpdated[email]; ok4 && v > 0 {
								m["updated_at"] = v
							}
						}
						nSlice[i] = m
					}
				}
				newSettings["clients"] = nSlice
				if bs, err3 := json.MarshalIndent(newSettings, "", "  "); err3 == nil {
					inbound.Settings = string(bs)
				}
			}
		}
	}

	oldInbound.Up = inbound.Up
	oldInbound.Down = inbound.Down
	oldInbound.Total = inbound.Total
	oldInbound.Remark = inbound.Remark
	oldInbound.Enable = inbound.Enable
	oldInbound.ExpiryTime = inbound.ExpiryTime
	oldInbound.TrafficReset = inbound.TrafficReset
	oldInbound.Listen = inbound.Listen
	oldInbound.Port = inbound.Port
	oldInbound.Protocol = inbound.Protocol
	oldInbound.Settings = inbound.Settings
	oldInbound.StreamSettings = inbound.StreamSettings
	oldInbound.Sniffing = inbound.Sniffing
	if inbound.Listen == "" || inbound.Listen == "0.0.0.0" || inbound.Listen == "::" || inbound.Listen == "::0" {
		oldInbound.Tag = fmt.Sprintf("inbound-%v", inbound.Port)
	} else {
		oldInbound.Tag = fmt.Sprintf("inbound-%v:%v", inbound.Listen, inbound.Port)
	}

	needRestart := false
	s.xrayApi.Init(p.GetAPIPort())
	if s.xrayApi.DelInbound(tag) == nil {
		logger.Debug("Old inbound deleted by api:", tag)
	}
	if inbound.Enable {
		inboundJson, err2 := json.MarshalIndent(oldInbound.GenXrayInboundConfig(), "", "  ")
		if err2 != nil {
			logger.Debug("Unable to marshal updated inbound config:", err2)
			needRestart = true
		} else {
			err2 = s.xrayApi.AddInbound(inboundJson)
			if err2 == nil {
				logger.Debug("Updated inbound added by api:", oldInbound.Tag)
			} else {
				logger.Debug("Unable to update inbound by api:", err2)
				needRestart = true
			}
		}
	}
	s.xrayApi.Close()

	return inbound, needRestart, tx.Save(oldInbound).Error
}

func (s *InboundService) updateClientTraffics(tx *gorm.DB, oldInbound *model.Inbound, newInbound *model.Inbound) error {
	oldClients, err := s.GetClients(oldInbound)
	if err != nil {
		return err
	}
	newClients, err := s.GetClients(newInbound)
	if err != nil {
		return err
	}

	var emailExists bool

	for _, oldClient := range oldClients {
		emailExists = false
		for _, newClient := range newClients {
			if oldClient.Email == newClient.Email {
				emailExists = true
				break
			}
		}
		if !emailExists {
			err = s.DelClientStat(tx, oldClient.Email)
			if err != nil {
				return err
			}
		}
	}
	for _, newClient := range newClients {
		emailExists = false
		for _, oldClient := range oldClients {
			if newClient.Email == oldClient.Email {
				emailExists = true
				break
			}
		}
		if !emailExists {
			err = s.AddClientStat(tx, oldInbound.Id, &newClient)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *InboundService) AddInboundClient(data *model.Inbound) (bool, error) {
	clients, err := s.GetClients(data)
	if err != nil {
		return false, err
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(data.Settings), &settings)
	if err != nil {
		return false, err
	}

	interfaceClients := settings["clients"].([]any)
	// Add timestamps for new clients being appended
	nowTs := time.Now().Unix() * 1000
	for i := range interfaceClients {
		if cm, ok := interfaceClients[i].(map[string]any); ok {
			if _, ok2 := cm["created_at"]; !ok2 {
				cm["created_at"] = nowTs
			}
			cm["updated_at"] = nowTs
			interfaceClients[i] = cm
		}
	}
	existEmail, err := s.checkEmailsExistForClients(clients)
	if err != nil {
		return false, err
	}
	if existEmail != "" {
		return false, common.NewError("Duplicate email:", existEmail)
	}

	oldInbound, err := s.GetInbound(data.Id)
	if err != nil {
		return false, err
	}

	// Secure client ID
	for _, client := range clients {
		switch oldInbound.Protocol {
		case "trojan":
			if client.Password == "" {
				return false, common.NewError("empty client ID")
			}
		case "shadowsocks":
			if client.Email == "" {
				return false, common.NewError("empty client ID")
			}
		default:
			if client.ID == "" {
				return false, common.NewError("empty client ID")
			}
		}
	}

	var oldSettings map[string]any
	err = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
	if err != nil {
		return false, err
	}

	oldClients := oldSettings["clients"].([]any)
	oldClients = append(oldClients, interfaceClients...)

	oldSettings["clients"] = oldClients

	newSettings, err := json.MarshalIndent(oldSettings, "", "  ")
	if err != nil {
		return false, err
	}

	oldInbound.Settings = string(newSettings)

	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	needRestart := false
	s.xrayApi.Init(p.GetAPIPort())
	for _, client := range clients {
		if len(client.Email) > 0 {
			s.AddClientStat(tx, data.Id, &client)
			if client.Enable {
				cipher := ""
				if oldInbound.Protocol == "shadowsocks" {
					cipher = oldSettings["method"].(string)
				}
				err1 := s.xrayApi.AddUser(string(oldInbound.Protocol), oldInbound.Tag, map[string]any{
					"email":    client.Email,
					"id":       client.ID,
					"security": client.Security,
					"flow":     client.Flow,
					"password": client.Password,
					"cipher":   cipher,
				})
				if err1 == nil {
					logger.Debug("Client added by api:", client.Email)
				} else {
					logger.Debug("Error in adding client by api:", err1)
					needRestart = true
				}
			}
		} else {
			needRestart = true
		}
	}
	s.xrayApi.Close()

	return needRestart, tx.Save(oldInbound).Error
}

func (s *InboundService) DelInboundClient(inboundId int, clientId string) (bool, error) {
	oldInbound, err := s.GetInbound(inboundId)
	if err != nil {
		logger.Error("Load Old Data Error")
		return false, err
	}
	var settings map[string]any
	err = json.Unmarshal([]byte(oldInbound.Settings), &settings)
	if err != nil {
		return false, err
	}

	email := ""
	client_key := "id"
	if oldInbound.Protocol == "trojan" {
		client_key = "password"
	}
	if oldInbound.Protocol == "shadowsocks" {
		client_key = "email"
	}

	interfaceClients := settings["clients"].([]any)
	var newClients []any
	needApiDel := false
	for _, client := range interfaceClients {
		c := client.(map[string]any)
		c_id := c[client_key].(string)
		if c_id == clientId {
			email, _ = c["email"].(string)
			needApiDel, _ = c["enable"].(bool)
		} else {
			newClients = append(newClients, client)
		}
	}

	if len(newClients) == 0 {
		return false, common.NewError("no client remained in Inbound")
	}

	settings["clients"] = newClients
	newSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}

	oldInbound.Settings = string(newSettings)

	db := database.GetDB()

	err = s.DelClientIPs(db, email)
	if err != nil {
		logger.Error("Error in delete client IPs")
		return false, err
	}
	needRestart := false

	if len(email) > 0 {
		notDepleted := true
		err = db.Model(xray.ClientTraffic{}).Select("enable").Where("email = ?", email).First(&notDepleted).Error
		if err != nil {
			logger.Error("Get stats error")
			return false, err
		}
		err = s.DelClientStat(db, email)
		if err != nil {
			logger.Error("Delete stats Data Error")
			return false, err
		}
		if needApiDel && notDepleted {
			s.xrayApi.Init(p.GetAPIPort())
			err1 := s.xrayApi.RemoveUser(oldInbound.Tag, email)
			if err1 == nil {
				logger.Debug("Client deleted by api:", email)
				needRestart = false
			} else {
				if strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", email)) {
					logger.Debug("User is already deleted. Nothing to do more...")
				} else {
					logger.Debug("Error in deleting client by api:", err1)
					needRestart = true
				}
			}
			s.xrayApi.Close()
		}
	}
	return needRestart, db.Save(oldInbound).Error
}

func (s *InboundService) UpdateInboundClient(data *model.Inbound, clientId string) (bool, error) {
	// TODO: check if TrafficReset field is updating
	clients, err := s.GetClients(data)
	if err != nil {
		return false, err
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(data.Settings), &settings)
	if err != nil {
		return false, err
	}

	interfaceClients := settings["clients"].([]any)

	oldInbound, err := s.GetInbound(data.Id)
	if err != nil {
		return false, err
	}

	oldClients, err := s.GetClients(oldInbound)
	if err != nil {
		return false, err
	}

	oldEmail := ""
	newClientId := ""
	clientIndex := -1
	for index, oldClient := range oldClients {
		oldClientId := ""
		switch oldInbound.Protocol {
		case "trojan":
			oldClientId = oldClient.Password
			newClientId = clients[0].Password
		case "shadowsocks":
			oldClientId = oldClient.Email
			newClientId = clients[0].Email
		default:
			oldClientId = oldClient.ID
			newClientId = clients[0].ID
		}
		if clientId == oldClientId {
			oldEmail = oldClient.Email
			clientIndex = index
			break
		}
	}

	// Validate new client ID
	if newClientId == "" || clientIndex == -1 {
		return false, common.NewError("empty client ID")
	}

	if len(clients[0].Email) > 0 && clients[0].Email != oldEmail {
		existEmail, err := s.checkEmailsExistForClients(clients)
		if err != nil {
			return false, err
		}
		if existEmail != "" {
			return false, common.NewError("Duplicate email:", existEmail)
		}
	}

	var oldSettings map[string]any
	err = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
	if err != nil {
		return false, err
	}
	settingsClients := oldSettings["clients"].([]any)
	// Preserve created_at and set updated_at for the replacing client
	var preservedCreated any
	if clientIndex >= 0 && clientIndex < len(settingsClients) {
		if oldMap, ok := settingsClients[clientIndex].(map[string]any); ok {
			if v, ok2 := oldMap["created_at"]; ok2 {
				preservedCreated = v
			}
		}
	}
	if len(interfaceClients) > 0 {
		if newMap, ok := interfaceClients[0].(map[string]any); ok {
			if preservedCreated == nil {
				preservedCreated = time.Now().Unix() * 1000
			}
			newMap["created_at"] = preservedCreated
			newMap["updated_at"] = time.Now().Unix() * 1000
			interfaceClients[0] = newMap
		}
	}
	settingsClients[clientIndex] = interfaceClients[0]
	oldSettings["clients"] = settingsClients

	newSettings, err := json.MarshalIndent(oldSettings, "", "  ")
	if err != nil {
		return false, err
	}

	oldInbound.Settings = string(newSettings)
	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	if len(clients[0].Email) > 0 {
		if len(oldEmail) > 0 {
			err = s.UpdateClientStat(tx, oldEmail, &clients[0])
			if err != nil {
				return false, err
			}
			err = s.UpdateClientIPs(tx, oldEmail, clients[0].Email)
			if err != nil {
				return false, err
			}
		} else {
			s.AddClientStat(tx, data.Id, &clients[0])
		}
	} else {
		err = s.DelClientStat(tx, oldEmail)
		if err != nil {
			return false, err
		}
		err = s.DelClientIPs(tx, oldEmail)
		if err != nil {
			return false, err
		}
	}
	needRestart := false
	if len(oldEmail) > 0 {
		s.xrayApi.Init(p.GetAPIPort())
		if oldClients[clientIndex].Enable {
			err1 := s.xrayApi.RemoveUser(oldInbound.Tag, oldEmail)
			if err1 == nil {
				logger.Debug("Old client deleted by api:", oldEmail)
			} else {
				if strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", oldEmail)) {
					logger.Debug("User is already deleted. Nothing to do more...")
				} else {
					logger.Debug("Error in deleting client by api:", err1)
					needRestart = true
				}
			}
		}
		if clients[0].Enable {
			cipher := ""
			if oldInbound.Protocol == "shadowsocks" {
				cipher = oldSettings["method"].(string)
			}
			err1 := s.xrayApi.AddUser(string(oldInbound.Protocol), oldInbound.Tag, map[string]any{
				"email":    clients[0].Email,
				"id":       clients[0].ID,
				"security": clients[0].Security,
				"flow":     clients[0].Flow,
				"password": clients[0].Password,
				"cipher":   cipher,
			})
			if err1 == nil {
				logger.Debug("Client edited by api:", clients[0].Email)
			} else {
				logger.Debug("Error in adding client by api:", err1)
				needRestart = true
			}
		}
		s.xrayApi.Close()
	} else {
		logger.Debug("Client old email not found")
		needRestart = true
	}
	return needRestart, tx.Save(oldInbound).Error
}

func (s *InboundService) AddTraffic(inboundTraffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) (error, bool) {
	var err error
	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	err = s.addInboundTraffic(tx, inboundTraffics)
	if err != nil {
		return err, false
	}
	err = s.addClientTraffic(tx, clientTraffics)
	if err != nil {
		return err, false
	}

	needRestart0, count, err := s.autoRenewClients(tx)
	if err != nil {
		logger.Warning("Error in renew clients:", err)
	} else if count > 0 {
		logger.Debugf("%v clients renewed", count)
	}

	needRestart1, count, err := s.disableInvalidClients(tx)
	if err != nil {
		logger.Warning("Error in disabling invalid clients:", err)
	} else if count > 0 {
		logger.Debugf("%v clients disabled", count)
	}

	needRestart2, count, err := s.disableInvalidInbounds(tx)
	if err != nil {
		logger.Warning("Error in disabling invalid inbounds:", err)
	} else if count > 0 {
		logger.Debugf("%v inbounds disabled", count)
	}
	return nil, (needRestart0 || needRestart1 || needRestart2)
}

func (s *InboundService) addInboundTraffic(tx *gorm.DB, traffics []*xray.Traffic) error {
	if len(traffics) == 0 {
		return nil
	}

	var err error

	for _, traffic := range traffics {
		if traffic.IsInbound {
			err = tx.Model(&model.Inbound{}).Where("tag = ?", traffic.Tag).
				Updates(map[string]any{
					"up":       gorm.Expr("up + ?", traffic.Up),
					"down":     gorm.Expr("down + ?", traffic.Down),
					"all_time": gorm.Expr("COALESCE(all_time, 0) + ?", traffic.Up+traffic.Down),
				}).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *InboundService) addClientTraffic(tx *gorm.DB, traffics []*xray.ClientTraffic) (err error) {
	if len(traffics) == 0 {
		// Empty onlineUsers
		if p != nil {
			p.SetOnlineClients(nil)
		}
		return nil
	}

	var onlineClients []string

	emails := make([]string, 0, len(traffics))
	for _, traffic := range traffics {
		emails = append(emails, traffic.Email)
	}
	dbClientTraffics := make([]*xray.ClientTraffic, 0, len(traffics))
	err = tx.Model(xray.ClientTraffic{}).Where("email IN (?)", emails).Find(&dbClientTraffics).Error
	if err != nil {
		return err
	}

	// Avoid empty slice error
	if len(dbClientTraffics) == 0 {
		return nil
	}

	dbClientTraffics, err = s.adjustTraffics(tx, dbClientTraffics)
	if err != nil {
		return err
	}

	for dbTraffic_index := range dbClientTraffics {
		for traffic_index := range traffics {
			if dbClientTraffics[dbTraffic_index].Email == traffics[traffic_index].Email {
				dbClientTraffics[dbTraffic_index].Up += traffics[traffic_index].Up
				dbClientTraffics[dbTraffic_index].Down += traffics[traffic_index].Down
				dbClientTraffics[dbTraffic_index].AllTime += (traffics[traffic_index].Up + traffics[traffic_index].Down)

				// Add user in onlineUsers array on traffic
				if traffics[traffic_index].Up+traffics[traffic_index].Down > 0 {
					onlineClients = append(onlineClients, traffics[traffic_index].Email)
					dbClientTraffics[dbTraffic_index].LastOnline = time.Now().UnixMilli()
				}
				break
			}
		}
	}

	// Set onlineUsers
	p.SetOnlineClients(onlineClients)

	err = tx.Save(dbClientTraffics).Error
	if err != nil {
		logger.Warning("AddClientTraffic update data ", err)
	}

	return nil
}

func (s *InboundService) adjustTraffics(tx *gorm.DB, dbClientTraffics []*xray.ClientTraffic) ([]*xray.ClientTraffic, error) {
	inboundIds := make([]int, 0, len(dbClientTraffics))
	for _, dbClientTraffic := range dbClientTraffics {
		if dbClientTraffic.ExpiryTime < 0 {
			inboundIds = append(inboundIds, dbClientTraffic.InboundId)
		}
	}

	if len(inboundIds) > 0 {
		var inbounds []*model.Inbound
		err := tx.Model(model.Inbound{}).Where("id IN (?)", inboundIds).Find(&inbounds).Error
		if err != nil {
			return nil, err
		}
		for inbound_index := range inbounds {
			settings := map[string]any{}
			json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
			clients, ok := settings["clients"].([]any)
			if ok {
				var newClients []any
				for client_index := range clients {
					c := clients[client_index].(map[string]any)
					for traffic_index := range dbClientTraffics {
						if dbClientTraffics[traffic_index].ExpiryTime < 0 && c["email"] == dbClientTraffics[traffic_index].Email {
							oldExpiryTime := c["expiryTime"].(float64)
							newExpiryTime := (time.Now().Unix() * 1000) - int64(oldExpiryTime)
							c["expiryTime"] = newExpiryTime
							c["updated_at"] = time.Now().Unix() * 1000
							dbClientTraffics[traffic_index].ExpiryTime = newExpiryTime
							break
						}
					}
					// Backfill created_at and updated_at
					if _, ok := c["created_at"]; !ok {
						c["created_at"] = time.Now().Unix() * 1000
					}
					c["updated_at"] = time.Now().Unix() * 1000
					newClients = append(newClients, any(c))
				}
				settings["clients"] = newClients
				modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
				if err != nil {
					return nil, err
				}

				inbounds[inbound_index].Settings = string(modifiedSettings)
			}
		}
		err = tx.Save(inbounds).Error
		if err != nil {
			logger.Warning("AddClientTraffic update inbounds ", err)
			logger.Error(inbounds)
		}
	}

	return dbClientTraffics, nil
}

func (s *InboundService) autoRenewClients(tx *gorm.DB) (bool, int64, error) {
	// check for time expired
	var traffics []*xray.ClientTraffic
	now := time.Now().Unix() * 1000
	var err, err1 error

	err = tx.Model(xray.ClientTraffic{}).Where("reset > 0 and expiry_time > 0 and expiry_time <= ?", now).Find(&traffics).Error
	if err != nil {
		return false, 0, err
	}
	// return if there is no client to renew
	if len(traffics) == 0 {
		return false, 0, nil
	}

	var inbound_ids []int
	var inbounds []*model.Inbound
	needRestart := false
	var clientsToAdd []struct {
		protocol string
		tag      string
		client   map[string]any
	}

	for _, traffic := range traffics {
		inbound_ids = append(inbound_ids, traffic.InboundId)
	}
	err = tx.Model(model.Inbound{}).Where("id IN ?", inbound_ids).Find(&inbounds).Error
	if err != nil {
		return false, 0, err
	}
	for inbound_index := range inbounds {
		settings := map[string]any{}
		json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
		clients := settings["clients"].([]any)
		for client_index := range clients {
			c := clients[client_index].(map[string]any)
			for traffic_index, traffic := range traffics {
				if traffic.Email == c["email"].(string) {
					newExpiryTime := traffic.ExpiryTime
					for newExpiryTime < now {
						newExpiryTime += (int64(traffic.Reset) * 86400000)
					}
					c["expiryTime"] = newExpiryTime
					traffics[traffic_index].ExpiryTime = newExpiryTime
					traffics[traffic_index].Down = 0
					traffics[traffic_index].Up = 0
					if !traffic.Enable {
						traffics[traffic_index].Enable = true
						clientsToAdd = append(clientsToAdd,
							struct {
								protocol string
								tag      string
								client   map[string]any
							}{
								protocol: string(inbounds[inbound_index].Protocol),
								tag:      inbounds[inbound_index].Tag,
								client:   c,
							})
					}
					clients[client_index] = any(c)
					break
				}
			}
		}
		settings["clients"] = clients
		newSettings, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return false, 0, err
		}
		inbounds[inbound_index].Settings = string(newSettings)
	}
	err = tx.Save(inbounds).Error
	if err != nil {
		return false, 0, err
	}
	err = tx.Save(traffics).Error
	if err != nil {
		return false, 0, err
	}
	if p != nil {
		err1 = s.xrayApi.Init(p.GetAPIPort())
		if err1 != nil {
			return true, int64(len(traffics)), nil
		}
		for _, clientToAdd := range clientsToAdd {
			err1 = s.xrayApi.AddUser(clientToAdd.protocol, clientToAdd.tag, clientToAdd.client)
			if err1 != nil {
				needRestart = true
			}
		}
		s.xrayApi.Close()
	}
	return needRestart, int64(len(traffics)), nil
}

func (s *InboundService) disableInvalidInbounds(tx *gorm.DB) (bool, int64, error) {
	now := time.Now().Unix() * 1000
	needRestart := false

	if p != nil {
		var tags []string
		err := tx.Table("inbounds").
			Select("inbounds.tag").
			Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ?", now, true).
			Scan(&tags).Error
		if err != nil {
			return false, 0, err
		}
		s.xrayApi.Init(p.GetAPIPort())
		for _, tag := range tags {
			err1 := s.xrayApi.DelInbound(tag)
			if err1 == nil {
				logger.Debug("Inbound disabled by api:", tag)
			} else {
				logger.Debug("Error in disabling inbound by api:", err1)
				needRestart = true
			}
		}
		s.xrayApi.Close()
	}

	result := tx.Model(model.Inbound{}).
		Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ?", now, true).
		Update("enable", false)
	err := result.Error
	count := result.RowsAffected
	return needRestart, count, err
}

func (s *InboundService) disableInvalidClients(tx *gorm.DB) (bool, int64, error) {
	now := time.Now().Unix() * 1000
	needRestart := false

	if p != nil {
		var results []struct {
			Tag   string
			Email string
		}

		err := tx.Table("inbounds").
			Select("inbounds.tag, client_traffics.email").
			Joins("JOIN client_traffics ON inbounds.id = client_traffics.inbound_id").
			Where("((client_traffics.total > 0 AND client_traffics.up + client_traffics.down >= client_traffics.total) OR (client_traffics.expiry_time > 0 AND client_traffics.expiry_time <= ?)) AND client_traffics.enable = ?", now, true).
			Scan(&results).Error
		if err != nil {
			return false, 0, err
		}
		s.xrayApi.Init(p.GetAPIPort())
		for _, result := range results {
			err1 := s.xrayApi.RemoveUser(result.Tag, result.Email)
			if err1 == nil {
				logger.Debug("Client disabled by api:", result.Email)
			} else {
				if strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", result.Email)) {
					logger.Debug("User is already disabled. Nothing to do more...")
				} else {
					if strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", result.Email)) {
						logger.Debug("User is already disabled. Nothing to do more...")
					} else {
						logger.Debug("Error in disabling client by api:", err1)
						needRestart = true
					}
				}
			}
		}
		s.xrayApi.Close()
	}
	result := tx.Model(xray.ClientTraffic{}).
		Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ?", now, true).
		Update("enable", false)
	err := result.Error
	count := result.RowsAffected
	return needRestart, count, err
}

func (s *InboundService) GetInboundTags() (string, error) {
	db := database.GetDB()
	var inboundTags []string
	err := db.Model(model.Inbound{}).Select("tag").Find(&inboundTags).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return "", err
	}
	tags, _ := json.Marshal(inboundTags)
	return string(tags), nil
}

func (s *InboundService) MigrationRemoveOrphanedTraffics() {
	db := database.GetDB()
	db.Exec(`
		DELETE FROM client_traffics
		WHERE email NOT IN (
			SELECT JSON_EXTRACT(client.value, '$.email')
			FROM inbounds,
				JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		)
	`)
}

func (s *InboundService) AddClientStat(tx *gorm.DB, inboundId int, client *model.Client) error {
	clientTraffic := xray.ClientTraffic{}
	clientTraffic.InboundId = inboundId
	clientTraffic.Email = client.Email
	clientTraffic.Total = client.TotalGB
	clientTraffic.ExpiryTime = client.ExpiryTime
	clientTraffic.Enable = client.Enable
	clientTraffic.Up = 0
	clientTraffic.Down = 0
	clientTraffic.Reset = client.Reset
	result := tx.Create(&clientTraffic)
	err := result.Error
	return err
}

func (s *InboundService) UpdateClientStat(tx *gorm.DB, email string, client *model.Client) error {
	result := tx.Model(xray.ClientTraffic{}).
		Where("email = ?", email).
		Updates(map[string]any{
			"enable":      client.Enable,
			"email":       client.Email,
			"total":       client.TotalGB,
			"expiry_time": client.ExpiryTime,
			"reset":       client.Reset,
		})
	err := result.Error
	return err
}

func (s *InboundService) UpdateClientIPs(tx *gorm.DB, oldEmail string, newEmail string) error {
	return tx.Model(model.InboundClientIps{}).Where("client_email = ?", oldEmail).Update("client_email", newEmail).Error
}

func (s *InboundService) DelClientStat(tx *gorm.DB, email string) error {
	return tx.Where("email = ?", email).Delete(xray.ClientTraffic{}).Error
}

func (s *InboundService) DelClientIPs(tx *gorm.DB, email string) error {
	return tx.Where("client_email = ?", email).Delete(model.InboundClientIps{}).Error
}

func (s *InboundService) GetClientInboundByTrafficID(trafficId int) (traffic *xray.ClientTraffic, inbound *model.Inbound, err error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).Where("id = ?", trafficId).Find(&traffics).Error
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with trafficId %d: %v", trafficId, err)
		return nil, nil, err
	}
	if len(traffics) > 0 {
		inbound, err = s.GetInbound(traffics[0].InboundId)
		return traffics[0], inbound, err
	}
	return nil, nil, nil
}

func (s *InboundService) GetClientInboundByEmail(email string) (traffic *xray.ClientTraffic, inbound *model.Inbound, err error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).Where("email = ?", email).Find(&traffics).Error
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with email %s: %v", email, err)
		return nil, nil, err
	}
	if len(traffics) > 0 {
		inbound, err = s.GetInbound(traffics[0].InboundId)
		return traffics[0], inbound, err
	}
	return nil, nil, nil
}

func (s *InboundService) GetClientByEmail(clientEmail string) (*xray.ClientTraffic, *model.Client, error) {
	traffic, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return nil, nil, err
	}
	if inbound == nil {
		return nil, nil, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	clients, err := s.GetClients(inbound)
	if err != nil {
		return nil, nil, err
	}

	for _, client := range clients {
		if client.Email == clientEmail {
			return traffic, &client, nil
		}
	}

	return nil, nil, common.NewError("Client Not Found In Inbound For Email:", clientEmail)
}

func (s *InboundService) SetClientTelegramUserID(trafficId int, tgId int64) (bool, error) {
	traffic, inbound, err := s.GetClientInboundByTrafficID(trafficId)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Traffic ID:", trafficId)
	}

	clientEmail := traffic.Email

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	clientId := ""

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			break
		}
	}

	if len(clientId) == 0 {
		return false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["tgId"] = tgId
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(modifiedSettings)
	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	return needRestart, err
}

func (s *InboundService) checkIsEnabledByEmail(clientEmail string) (bool, error) {
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	clients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	isEnable := false

	for _, client := range clients {
		if client.Email == clientEmail {
			isEnable = client.Enable
			break
		}
	}

	return isEnable, err
}

func (s *InboundService) ToggleClientEnableByEmail(clientEmail string) (bool, bool, error) {
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, false, err
	}
	if inbound == nil {
		return false, false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, false, err
	}

	clientId := ""
	clientOldEnabled := false

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			clientOldEnabled = oldClient.Enable
			break
		}
	}

	if len(clientId) == 0 {
		return false, false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["enable"] = !clientOldEnabled
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, false, err
	}
	inbound.Settings = string(modifiedSettings)

	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	if err != nil {
		return false, needRestart, err
	}

	return !clientOldEnabled, needRestart, nil
}

// SetClientEnableByEmail sets client enable state to desired value; returns (changed, needRestart, error)
func (s *InboundService) SetClientEnableByEmail(clientEmail string, enable bool) (bool, bool, error) {
	current, err := s.checkIsEnabledByEmail(clientEmail)
	if err != nil {
		return false, false, err
	}
	if current == enable {
		return false, false, nil
	}
	newEnabled, needRestart, err := s.ToggleClientEnableByEmail(clientEmail)
	if err != nil {
		return false, needRestart, err
	}
	return newEnabled == enable, needRestart, nil
}

func (s *InboundService) ResetClientIpLimitByEmail(clientEmail string, count int) (bool, error) {
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	clientId := ""

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			break
		}
	}

	if len(clientId) == 0 {
		return false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["limitIp"] = count
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(modifiedSettings)
	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	return needRestart, err
}

func (s *InboundService) ResetClientExpiryTimeByEmail(clientEmail string, expiry_time int64) (bool, error) {
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	clientId := ""

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			break
		}
	}

	if len(clientId) == 0 {
		return false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["expiryTime"] = expiry_time
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(modifiedSettings)
	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	return needRestart, err
}

func (s *InboundService) ResetClientTrafficLimitByEmail(clientEmail string, totalGB int) (bool, error) {
	if totalGB < 0 {
		return false, common.NewError("totalGB must be >= 0")
	}
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	clientId := ""

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			break
		}
	}

	if len(clientId) == 0 {
		return false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["totalGB"] = totalGB * 1024 * 1024 * 1024
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(modifiedSettings)
	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	return needRestart, err
}

func (s *InboundService) ResetClientTrafficByEmail(clientEmail string) error {
	db := database.GetDB()

	// Reset traffic stats in ClientTraffic table
	result := db.Model(xray.ClientTraffic{}).
		Where("email = ?", clientEmail).
		Updates(map[string]any{"enable": true, "up": 0, "down": 0})

	err := result.Error
	if err != nil {
		return err
	}

	return nil
}

func (s *InboundService) ResetClientTraffic(id int, clientEmail string) (bool, error) {
	needRestart := false

	traffic, err := s.GetClientTrafficByEmail(clientEmail)
	if err != nil {
		return false, err
	}

	if !traffic.Enable {
		inbound, err := s.GetInbound(id)
		if err != nil {
			return false, err
		}
		clients, err := s.GetClients(inbound)
		if err != nil {
			return false, err
		}
		for _, client := range clients {
			if client.Email == clientEmail && client.Enable {
				s.xrayApi.Init(p.GetAPIPort())
				cipher := ""
				if string(inbound.Protocol) == "shadowsocks" {
					var oldSettings map[string]any
					err = json.Unmarshal([]byte(inbound.Settings), &oldSettings)
					if err != nil {
						return false, err
					}
					cipher = oldSettings["method"].(string)
				}
				err1 := s.xrayApi.AddUser(string(inbound.Protocol), inbound.Tag, map[string]any{
					"email":    client.Email,
					"id":       client.ID,
					"security": client.Security,
					"flow":     client.Flow,
					"password": client.Password,
					"cipher":   cipher,
				})
				if err1 == nil {
					logger.Debug("Client enabled due to reset traffic:", clientEmail)
				} else {
					logger.Debug("Error in enabling client by api:", err1)
					needRestart = true
				}
				s.xrayApi.Close()
				break
			}
		}
	}

	traffic.Up = 0
	traffic.Down = 0
	traffic.Enable = true

	db := database.GetDB()
	err = db.Save(traffic).Error
	if err != nil {
		return false, err
	}

	return needRestart, nil
}

func (s *InboundService) ResetAllClientTraffics(id int) error {
	db := database.GetDB()
	now := time.Now().Unix() * 1000

	return db.Transaction(func(tx *gorm.DB) error {
		whereText := "inbound_id "
		if id == -1 {
			whereText += " > ?"
		} else {
			whereText += " = ?"
		}

		// Reset client traffics
		result := tx.Model(xray.ClientTraffic{}).
			Where(whereText, id).
			Updates(map[string]any{"enable": true, "up": 0, "down": 0})

		if result.Error != nil {
			return result.Error
		}

		// Update lastTrafficResetTime for the inbound(s)
		inboundWhereText := "id "
		if id == -1 {
			inboundWhereText += " > ?"
		} else {
			inboundWhereText += " = ?"
		}

		result = tx.Model(model.Inbound{}).
			Where(inboundWhereText, id).
			Update("last_traffic_reset_time", now)

		return result.Error
	})
}

func (s *InboundService) ResetAllTraffics() error {
	db := database.GetDB()

	result := db.Model(model.Inbound{}).
		Where("user_id > ?", 0).
		Updates(map[string]any{"up": 0, "down": 0})

	err := result.Error
	return err
}

func (s *InboundService) DelDepletedClients(id int) (err error) {
	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	whereText := "reset = 0 and inbound_id "
	if id < 0 {
		whereText += "> ?"
	} else {
		whereText += "= ?"
	}

	// Only consider truly depleted clients: expired OR traffic exhausted
	now := time.Now().Unix() * 1000
	depletedClients := []xray.ClientTraffic{}
	err = db.Model(xray.ClientTraffic{}).
		Where(whereText+" and ((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?))", id, now).
		Select("inbound_id, GROUP_CONCAT(email) as email").
		Group("inbound_id").
		Find(&depletedClients).Error
	if err != nil {
		return err
	}

	for _, depletedClient := range depletedClients {
		emails := strings.Split(depletedClient.Email, ",")
		oldInbound, err := s.GetInbound(depletedClient.InboundId)
		if err != nil {
			return err
		}
		var oldSettings map[string]any
		err = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
		if err != nil {
			return err
		}

		oldClients := oldSettings["clients"].([]any)
		var newClients []any
		for _, client := range oldClients {
			deplete := false
			c := client.(map[string]any)
			for _, email := range emails {
				if email == c["email"].(string) {
					deplete = true
					break
				}
			}
			if !deplete {
				newClients = append(newClients, client)
			}
		}
		if len(newClients) > 0 {
			oldSettings["clients"] = newClients

			newSettings, err := json.MarshalIndent(oldSettings, "", "  ")
			if err != nil {
				return err
			}

			oldInbound.Settings = string(newSettings)
			err = tx.Save(oldInbound).Error
			if err != nil {
				return err
			}
		} else {
			// Delete inbound if no client remains
			s.DelInbound(depletedClient.InboundId)
		}
	}

	// Delete stats only for truly depleted clients
	err = tx.Where(whereText+" and ((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?))", id, now).Delete(xray.ClientTraffic{}).Error
	if err != nil {
		return err
	}

	return nil
}

func (s *InboundService) GetClientTrafficTgBot(tgId int64) ([]*xray.ClientTraffic, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound

	// Retrieve inbounds where settings contain the given tgId
	err := db.Model(model.Inbound{}).Where("settings LIKE ?", fmt.Sprintf(`%%"tgId": %d%%`, tgId)).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		logger.Errorf("Error retrieving inbounds with tgId %d: %v", tgId, err)
		return nil, err
	}

	var emails []string
	for _, inbound := range inbounds {
		clients, err := s.GetClients(inbound)
		if err != nil {
			logger.Errorf("Error retrieving clients for inbound %d: %v", inbound.Id, err)
			continue
		}
		for _, client := range clients {
			if client.TgID == tgId {
				emails = append(emails, client.Email)
			}
		}
	}

	var traffics []*xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).Where("email IN ?", emails).Find(&traffics).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warning("No ClientTraffic records found for emails:", emails)
			return nil, nil
		}
		logger.Errorf("Error retrieving ClientTraffic for emails %v: %v", emails, err)
		return nil, err
	}

	// Populate UUID and other client data for each traffic record
	for i := range traffics {
		if ct, client, e := s.GetClientByEmail(traffics[i].Email); e == nil && ct != nil && client != nil {
			traffics[i].Enable = client.Enable
			traffics[i].UUID = client.ID
			traffics[i].SubId = client.SubID
		}
	}

	return traffics, nil
}

func (s *InboundService) GetClientTrafficByEmail(email string) (traffic *xray.ClientTraffic, err error) {
	// Prefer retrieving along with client to reflect actual enabled state from inbound settings
	t, client, err := s.GetClientByEmail(email)
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with email %s: %v", email, err)
		return nil, err
	}
	if t != nil && client != nil {
		t.Enable = client.Enable
		t.UUID = client.ID
		t.SubId = client.SubID
		return t, nil
	}
	return nil, nil
}

func (s *InboundService) UpdateClientTrafficByEmail(email string, upload int64, download int64) error {
	db := database.GetDB()

	result := db.Model(xray.ClientTraffic{}).
		Where("email = ?", email).
		Updates(map[string]any{"up": upload, "down": download})

	err := result.Error
	if err != nil {
		logger.Warningf("Error updating ClientTraffic with email %s: %v", email, err)
		return err
	}
	return nil
}

func (s *InboundService) GetClientTrafficByID(id string) ([]xray.ClientTraffic, error) {
	db := database.GetDB()
	var traffics []xray.ClientTraffic

	err := db.Model(xray.ClientTraffic{}).Where(`email IN(
		SELECT JSON_EXTRACT(client.value, '$.email') as email
		FROM inbounds,
	  	JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		WHERE
	  	JSON_EXTRACT(client.value, '$.id') in (?)
		)`, id).Find(&traffics).Error

	if err != nil {
		logger.Debug(err)
		return nil, err
	}
	// Reconcile enable flag with client settings per email to avoid stale DB value
	for i := range traffics {
		if ct, client, e := s.GetClientByEmail(traffics[i].Email); e == nil && ct != nil && client != nil {
			traffics[i].Enable = client.Enable
			traffics[i].UUID = client.ID
			traffics[i].SubId = client.SubID
		}
	}
	return traffics, err
}

func (s *InboundService) SearchClientTraffic(query string) (traffic *xray.ClientTraffic, err error) {
	db := database.GetDB()
	inbound := &model.Inbound{}
	traffic = &xray.ClientTraffic{}

	// Search for inbound settings that contain the query
	err = db.Model(model.Inbound{}).Where("settings LIKE ?", "%\""+query+"\"%").First(inbound).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warningf("Inbound settings containing query %s not found: %v", query, err)
			return nil, err
		}
		logger.Errorf("Error searching for inbound settings with query %s: %v", query, err)
		return nil, err
	}

	traffic.InboundId = inbound.Id

	// Unmarshal settings to get clients
	settings := map[string][]model.Client{}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		logger.Errorf("Error unmarshalling inbound settings for inbound ID %d: %v", inbound.Id, err)
		return nil, err
	}

	clients := settings["clients"]
	for _, client := range clients {
		if (client.ID == query || client.Password == query) && client.Email != "" {
			traffic.Email = client.Email
			break
		}
	}

	if traffic.Email == "" {
		logger.Warningf("No client found with query %s in inbound ID %d", query, inbound.Id)
		return nil, gorm.ErrRecordNotFound
	}

	// Retrieve ClientTraffic based on the found email
	err = db.Model(xray.ClientTraffic{}).Where("email = ?", traffic.Email).First(traffic).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warningf("ClientTraffic for email %s not found: %v", traffic.Email, err)
			return nil, err
		}
		logger.Errorf("Error retrieving ClientTraffic for email %s: %v", traffic.Email, err)
		return nil, err
	}

	return traffic, nil
}

func (s *InboundService) GetInboundClientIps(clientEmail string) (string, error) {
	db := database.GetDB()
	InboundClientIps := &model.InboundClientIps{}
	err := db.Model(model.InboundClientIps{}).Where("client_email = ?", clientEmail).First(InboundClientIps).Error
	if err != nil {
		return "", err
	}
	return InboundClientIps.Ips, nil
}

func (s *InboundService) ClearClientIps(clientEmail string) error {
	db := database.GetDB()

	result := db.Model(model.InboundClientIps{}).
		Where("client_email = ?", clientEmail).
		Update("ips", "")
	err := result.Error
	if err != nil {
		return err
	}
	return nil
}

func (s *InboundService) SearchInbounds(query string) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Where("remark like ?", "%"+query+"%").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) MigrationRequirements() {
	db := database.GetDB()
	tx := db.Begin()
	var err error
	defer func() {
		if err == nil {
			tx.Commit()
			if dbErr := db.Exec(`VACUUM "main"`).Error; dbErr != nil {
				logger.Warningf("VACUUM failed: %v", dbErr)
			}
		} else {
			tx.Rollback()
		}
	}()

	// Calculate and backfill all_time from up+down for inbounds and clients
	err = tx.Exec(`
		UPDATE inbounds
		SET all_time = IFNULL(up, 0) + IFNULL(down, 0)
		WHERE IFNULL(all_time, 0) = 0 AND (IFNULL(up, 0) + IFNULL(down, 0)) > 0
	`).Error
	if err != nil {
		return
	}
	err = tx.Exec(`
		UPDATE client_traffics
		SET all_time = IFNULL(up, 0) + IFNULL(down, 0)
		WHERE IFNULL(all_time, 0) = 0 AND (IFNULL(up, 0) + IFNULL(down, 0)) > 0
	`).Error

	if err != nil {
		return
	}

	// Fix inbounds based problems
	var inbounds []*model.Inbound
	err = tx.Model(model.Inbound{}).Where("protocol IN (?)", []string{"vmess", "vless", "trojan"}).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return
	}
	for inbound_index := range inbounds {
		settings := map[string]any{}
		json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
		clients, ok := settings["clients"].([]any)
		if ok {
			// Fix Client configuration problems
			var newClients []any
			for client_index := range clients {
				c := clients[client_index].(map[string]any)

				// Add email='' if it is not exists
				if _, ok := c["email"]; !ok {
					c["email"] = ""
				}

				// Convert string tgId to int64
				if _, ok := c["tgId"]; ok {
					var tgId any = c["tgId"]
					if tgIdStr, ok2 := tgId.(string); ok2 {
						tgIdInt64, err := strconv.ParseInt(strings.ReplaceAll(tgIdStr, " ", ""), 10, 64)
						if err == nil {
							c["tgId"] = tgIdInt64
						}
					}
				}

				// Remove "flow": "xtls-rprx-direct"
				if _, ok := c["flow"]; ok {
					if c["flow"] == "xtls-rprx-direct" {
						c["flow"] = ""
					}
				}
				// Backfill created_at and updated_at
				if _, ok := c["created_at"]; !ok {
					c["created_at"] = time.Now().Unix() * 1000
				}
				c["updated_at"] = time.Now().Unix() * 1000
				newClients = append(newClients, any(c))
			}
			settings["clients"] = newClients
			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return
			}

			inbounds[inbound_index].Settings = string(modifiedSettings)
		}

		// Add client traffic row for all clients which has email
		modelClients, err := s.GetClients(inbounds[inbound_index])
		if err != nil {
			return
		}
		for _, modelClient := range modelClients {
			if len(modelClient.Email) > 0 {
				var count int64
				tx.Model(xray.ClientTraffic{}).Where("email = ?", modelClient.Email).Count(&count)
				if count == 0 {
					s.AddClientStat(tx, inbounds[inbound_index].Id, &modelClient)
				}
			}
		}
	}
	tx.Save(inbounds)

	// Remove orphaned traffics
	tx.Where("inbound_id = 0").Delete(xray.ClientTraffic{})

	// Migrate old MultiDomain to External Proxy
	var externalProxy []struct {
		Id             int
		Port           int
		StreamSettings []byte
	}
	err = tx.Raw(`select id, port, stream_settings
	from inbounds
	WHERE protocol in ('vmess','vless','trojan')
	  AND json_extract(stream_settings, '$.security') = 'tls'
	  AND json_extract(stream_settings, '$.tlsSettings.settings.domains') IS NOT NULL`).Scan(&externalProxy).Error
	if err != nil || len(externalProxy) == 0 {
		return
	}

	for _, ep := range externalProxy {
		var reverses any
		var stream map[string]any
		json.Unmarshal(ep.StreamSettings, &stream)
		if tlsSettings, ok := stream["tlsSettings"].(map[string]any); ok {
			if settings, ok := tlsSettings["settings"].(map[string]any); ok {
				if domains, ok := settings["domains"].([]any); ok {
					for _, domain := range domains {
						if domainMap, ok := domain.(map[string]any); ok {
							domainMap["forceTls"] = "same"
							domainMap["port"] = ep.Port
							domainMap["dest"] = domainMap["domain"].(string)
							delete(domainMap, "domain")
						}
					}
				}
				reverses = settings["domains"]
				delete(settings, "domains")
			}
		}
		stream["externalProxy"] = reverses
		newStream, _ := json.MarshalIndent(stream, " ", "  ")
		tx.Model(model.Inbound{}).Where("id = ?", ep.Id).Update("stream_settings", newStream)
	}

	err = tx.Raw(`UPDATE inbounds
	SET tag = REPLACE(tag, '0.0.0.0:', '')
	WHERE INSTR(tag, '0.0.0.0:') > 0;`).Error
	if err != nil {
		return
	}
}

func (s *InboundService) MigrateDB() {
	s.MigrationRequirements()
	s.MigrationRemoveOrphanedTraffics()
}

func (s *InboundService) GetOnlineClients() []string {
	return p.GetOnlineClients()
}

func (s *InboundService) GetClientsLastOnline() (map[string]int64, error) {
	db := database.GetDB()
	var rows []xray.ClientTraffic
	err := db.Model(&xray.ClientTraffic{}).Select("email, last_online").Find(&rows).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, r := range rows {
		result[r.Email] = r.LastOnline
	}
	return result, nil
}

func (s *InboundService) FilterAndSortClientEmails(emails []string) ([]string, []string, error) {
	db := database.GetDB()

	// Step 1: Get ClientTraffic records for emails in the input list
	var clients []xray.ClientTraffic
	err := db.Where("email IN ?", emails).Find(&clients).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, nil, err
	}

	// Step 2: Sort clients by (Up + Down) descending
	sort.Slice(clients, func(i, j int) bool {
		return (clients[i].Up + clients[i].Down) > (clients[j].Up + clients[j].Down)
	})

	// Step 3: Extract sorted valid emails and track found ones
	validEmails := make([]string, 0, len(clients))
	found := make(map[string]bool)
	for _, client := range clients {
		validEmails = append(validEmails, client.Email)
		found[client.Email] = true
	}

	// Step 4: Identify emails that were not found in the database
	extraEmails := make([]string, 0)
	for _, email := range emails {
		if !found[email] {
			extraEmails = append(extraEmails, email)
		}
	}

	return validEmails, extraEmails, nil
}
func (s *InboundService) DelInboundClientByEmail(inboundId int, email string) (bool, error) {
	oldInbound, err := s.GetInbound(inboundId)
	if err != nil {
		logger.Error("Load Old Data Error")
		return false, err
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(oldInbound.Settings), &settings); err != nil {
		return false, err
	}

	interfaceClients, ok := settings["clients"].([]any)
	if !ok {
		return false, common.NewError("invalid clients format in inbound settings")
	}

	var newClients []any
	needApiDel := false
	found := false

	for _, client := range interfaceClients {
		c, ok := client.(map[string]any)
		if !ok {
			continue
		}
		if cEmail, ok := c["email"].(string); ok && cEmail == email {
			// matched client, drop it
			found = true
			needApiDel, _ = c["enable"].(bool)
		} else {
			newClients = append(newClients, client)
		}
	}

	if !found {
		return false, common.NewError(fmt.Sprintf("client with email %s not found", email))
	}
	if len(newClients) == 0 {
		return false, common.NewError("no client remained in Inbound")
	}

	settings["clients"] = newClients
	newSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}

	oldInbound.Settings = string(newSettings)

	db := database.GetDB()

	// remove IP bindings
	if err := s.DelClientIPs(db, email); err != nil {
		logger.Error("Error in delete client IPs")
		return false, err
	}

	needRestart := false

	// remove stats too
	if len(email) > 0 {
		traffic, err := s.GetClientTrafficByEmail(email)
		if err != nil {
			return false, err
		}
		if traffic != nil {
			if err := s.DelClientStat(db, email); err != nil {
				logger.Error("Delete stats Data Error")
				return false, err
			}
		}

		if needApiDel {
			s.xrayApi.Init(p.GetAPIPort())
			if err1 := s.xrayApi.RemoveUser(oldInbound.Tag, email); err1 == nil {
				logger.Debug("Client deleted by api:", email)
				needRestart = false
			} else {
				if strings.Contains(err1.Error(), fmt.Sprintf("User %s not found.", email)) {
					logger.Debug("User is already deleted. Nothing to do more...")
				} else {
					logger.Debug("Error in deleting client by api:", err1)
					needRestart = true
				}
			}
			s.xrayApi.Close()
		}
	}

	return needRestart, db.Save(oldInbound).Error
}
