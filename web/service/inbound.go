package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/xray"

	"gorm.io/gorm"
)

type InboundService struct {
}

func (s *InboundService) GetInbounds(userId int) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Where("user_id = ?", userId).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) GetAllInbounds() ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) checkPortExist(port int, ignoreId int) (bool, error) {
	db := database.GetDB()
	db = db.Model(model.Inbound{}).Where("port = ?", port)
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

func (s *InboundService) getClients(inbound *model.Inbound) ([]model.Client, error) {
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
	for _, s := range slice {
		if s == str {
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
	clients, err := s.getClients(inbound)
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

func (s *InboundService) AddInbound(inbound *model.Inbound) (*model.Inbound, error) {
	exist, err := s.checkPortExist(inbound.Port, 0)
	if err != nil {
		return inbound, err
	}
	if exist {
		return inbound, common.NewError("Port already exists:", inbound.Port)
	}

	existEmail, err := s.checkEmailExistForInbound(inbound)
	if err != nil {
		return inbound, err
	}
	if existEmail != "" {
		return inbound, common.NewError("Duplicate email:", existEmail)
	}

	clients, err := s.getClients(inbound)
	if err != nil {
		return inbound, err
	}

	db := database.GetDB()

	err = db.Save(inbound).Error
	if err == nil {
		for _, client := range clients {
			s.AddClientStat(inbound.Id, &client)
		}
	}
	return inbound, err
}

func (s *InboundService) AddInbounds(inbounds []*model.Inbound) error {
	for _, inbound := range inbounds {
		exist, err := s.checkPortExist(inbound.Port, 0)
		if err != nil {
			return err
		}
		if exist {
			return common.NewError("Port already exists:", inbound.Port)
		}
	}

	db := database.GetDB()
	tx := db.Begin()
	var err error
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	for _, inbound := range inbounds {
		err = tx.Save(inbound).Error
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *InboundService) DelInbound(id int) error {
	db := database.GetDB()
	err := db.Where("inbound_id = ?", id).Delete(xray.ClientTraffic{}).Error
	if err != nil {
		return err
	}
	inbound, err := s.GetInbound(id)
	if err != nil {
		return err
	}
	clients, err := s.getClients(inbound)
	if err != nil {
		return err
	}
	for _, client := range clients {
		err := s.DelClientIPs(db, client.Email)
		if err != nil {
			return err
		}
	}
	return db.Delete(model.Inbound{}, id).Error
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

func (s *InboundService) UpdateInbound(inbound *model.Inbound) (*model.Inbound, error) {
	exist, err := s.checkPortExist(inbound.Port, inbound.Id)
	if err != nil {
		return inbound, err
	}
	if exist {
		return inbound, common.NewError("Port already exists:", inbound.Port)
	}

	oldInbound, err := s.GetInbound(inbound.Id)
	if err != nil {
		return inbound, err
	}
	oldInbound.Up = inbound.Up
	oldInbound.Down = inbound.Down
	oldInbound.Total = inbound.Total
	oldInbound.Remark = inbound.Remark
	oldInbound.Enable = inbound.Enable
	oldInbound.ExpiryTime = inbound.ExpiryTime
	oldInbound.Listen = inbound.Listen
	oldInbound.Port = inbound.Port
	oldInbound.Protocol = inbound.Protocol
	oldInbound.Settings = inbound.Settings
	oldInbound.StreamSettings = inbound.StreamSettings
	oldInbound.Sniffing = inbound.Sniffing
	oldInbound.Tag = fmt.Sprintf("inbound-%v", inbound.Port)

	db := database.GetDB()
	return inbound, db.Save(oldInbound).Error
}

func (s *InboundService) AddInboundClient(data *model.Inbound) error {
	clients, err := s.getClients(data)
	if err != nil {
		return err
	}

	var settings map[string]interface{}
	err = json.Unmarshal([]byte(data.Settings), &settings)
	if err != nil {
		return err
	}

	interfaceClients := settings["clients"].([]interface{})
	existEmail, err := s.checkEmailsExistForClients(clients)
	if err != nil {
		return err
	}
	if existEmail != "" {
		return common.NewError("Duplicate email:", existEmail)
	}

	oldInbound, err := s.GetInbound(data.Id)
	if err != nil {
		return err
	}

	var oldSettings map[string]interface{}
	err = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
	if err != nil {
		return err
	}

	oldClients := oldSettings["clients"].([]interface{})
	oldClients = append(oldClients, interfaceClients...)

	oldSettings["clients"] = oldClients

	newSettings, err := json.MarshalIndent(oldSettings, "", "  ")
	if err != nil {
		return err
	}

	oldInbound.Settings = string(newSettings)

	for _, client := range clients {
		if len(client.Email) > 0 {
			s.AddClientStat(data.Id, &client)
		}
	}
	db := database.GetDB()
	return db.Save(oldInbound).Error
}

func (s *InboundService) DelInboundClient(inboundId int, clientId string) error {
	oldInbound, err := s.GetInbound(inboundId)
	if err != nil {
		logger.Error("Load Old Data Error")
		return err
	}
	var settings map[string]interface{}
	err = json.Unmarshal([]byte(oldInbound.Settings), &settings)
	if err != nil {
		return err
	}

	email := ""
	client_key := "id"
	if oldInbound.Protocol == "trojan" {
		client_key = "password"
	}

	inerfaceClients := settings["clients"].([]interface{})
	var newClients []interface{}
	for _, client := range inerfaceClients {
		c := client.(map[string]interface{})
		c_id := c[client_key].(string)
		if c_id == clientId {
			email = c["email"].(string)
		} else {
			newClients = append(newClients, client)
		}
	}

	settings["clients"] = newClients
	newSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	oldInbound.Settings = string(newSettings)

	db := database.GetDB()
	err = s.DelClientStat(db, email)
	if err != nil {
		logger.Error("Delete stats Data Error")
		return err
	}

	err = s.DelClientIPs(db, email)
	if err != nil {
		logger.Error("Error in delete client IPs")
		return err
	}
	return db.Save(oldInbound).Error
}

func (s *InboundService) UpdateInboundClient(data *model.Inbound, clientId string) error {
	clients, err := s.getClients(data)
	if err != nil {
		return err
	}

	var settings map[string]interface{}
	err = json.Unmarshal([]byte(data.Settings), &settings)
	if err != nil {
		return err
	}

	inerfaceClients := settings["clients"].([]interface{})

	oldInbound, err := s.GetInbound(data.Id)
	if err != nil {
		return err
	}

	oldClients, err := s.getClients(oldInbound)
	if err != nil {
		return err
	}

	oldEmail := ""
	clientIndex := 0
	for index, oldClient := range oldClients {
		oldClientId := ""
		if oldInbound.Protocol == "trojan" {
			oldClientId = oldClient.Password
		} else {
			oldClientId = oldClient.ID
		}
		if clientId == oldClientId {
			oldEmail = oldClient.Email
			clientIndex = index
			break
		}
	}

	if len(clients[0].Email) > 0 && clients[0].Email != oldEmail {
		existEmail, err := s.checkEmailsExistForClients(clients)
		if err != nil {
			return err
		}
		if existEmail != "" {
			return common.NewError("Duplicate email:", existEmail)
		}
	}

	var oldSettings map[string]interface{}
	err = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
	if err != nil {
		return err
	}
	settingsClients := oldSettings["clients"].([]interface{})
	settingsClients[clientIndex] = inerfaceClients[0]
	oldSettings["clients"] = settingsClients

	newSettings, err := json.MarshalIndent(oldSettings, "", "  ")
	if err != nil {
		return err
	}

	oldInbound.Settings = string(newSettings)
	db := database.GetDB()

	if len(clients[0].Email) > 0 {
		if len(oldEmail) > 0 {
			err = s.UpdateClientStat(oldEmail, &clients[0])
			if err != nil {
				return err
			}
			err = s.UpdateClientIPs(db, oldEmail, clients[0].Email)
			if err != nil {
				return err
			}
		} else {
			s.AddClientStat(data.Id, &clients[0])
		}
	} else {
		err = s.DelClientStat(db, oldEmail)
		if err != nil {
			return err
		}
		err = s.DelClientIPs(db, oldEmail)
		if err != nil {
			return err
		}
	}
	return db.Save(oldInbound).Error
}

func (s *InboundService) AddTraffic(traffics []*xray.Traffic) error {
	if len(traffics) == 0 {
		return nil
	}
	// Update traffics in a single transaction
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		for _, traffic := range traffics {
			if traffic.IsInbound {
				update := tx.Model(&model.Inbound{}).Where("tag = ?", traffic.Tag).
					Updates(map[string]interface{}{
						"up":   gorm.Expr("up + ?", traffic.Up),
						"down": gorm.Expr("down + ?", traffic.Down),
					})
				if update.Error != nil {
					return update.Error
				}
			}
		}
		return nil
	})

	return err
}
func (s *InboundService) AddClientTraffic(traffics []*xray.ClientTraffic) (err error) {
	if len(traffics) == 0 {
		return nil
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

	emails := make([]string, 0, len(traffics))
	for _, traffic := range traffics {
		emails = append(emails, traffic.Email)
	}
	dbClientTraffics := make([]*xray.ClientTraffic, 0, len(traffics))
	err = db.Model(xray.ClientTraffic{}).Where("email IN (?)", emails).Find(&dbClientTraffics).Error
	if err != nil {
		return err
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
				break
			}
		}
	}

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
			settings := map[string]interface{}{}
			json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
			clients, ok := settings["clients"].([]interface{})
			if ok {
				var newClients []interface{}
				for client_index := range clients {
					c := clients[client_index].(map[string]interface{})
					for traffic_index := range dbClientTraffics {
						if dbClientTraffics[traffic_index].ExpiryTime < 0 && c["email"] == dbClientTraffics[traffic_index].Email {
							oldExpiryTime := c["expiryTime"].(float64)
							newExpiryTime := (time.Now().Unix() * 1000) - int64(oldExpiryTime)
							c["expiryTime"] = newExpiryTime
							dbClientTraffics[traffic_index].ExpiryTime = newExpiryTime
							break
						}
					}
					newClients = append(newClients, interface{}(c))
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

func (s *InboundService) DisableInvalidInbounds() (int64, error) {
	db := database.GetDB()
	now := time.Now().Unix() * 1000
	result := db.Model(model.Inbound{}).
		Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ?", now, true).
		Update("enable", false)
	err := result.Error
	count := result.RowsAffected
	return count, err
}
func (s *InboundService) DisableInvalidClients() (int64, error) {
	db := database.GetDB()
	now := time.Now().Unix() * 1000
	result := db.Model(xray.ClientTraffic{}).
		Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ?", now, true).
		Update("enable", false)
	err := result.Error
	count := result.RowsAffected
	return count, err
}
func (s *InboundService) RemoveOrphanedTraffics() {
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
func (s *InboundService) AddClientStat(inboundId int, client *model.Client) error {
	db := database.GetDB()

	clientTraffic := xray.ClientTraffic{}
	clientTraffic.InboundId = inboundId
	clientTraffic.Email = client.Email
	clientTraffic.Total = client.TotalGB
	clientTraffic.ExpiryTime = client.ExpiryTime
	clientTraffic.Enable = true
	clientTraffic.Up = 0
	clientTraffic.Down = 0
	result := db.Create(&clientTraffic)
	err := result.Error
	if err != nil {
		return err
	}
	return nil
}
func (s *InboundService) UpdateClientStat(email string, client *model.Client) error {
	db := database.GetDB()

	result := db.Model(xray.ClientTraffic{}).
		Where("email = ?", email).
		Updates(map[string]interface{}{
			"enable":      true,
			"email":       client.Email,
			"total":       client.TotalGB,
			"expiry_time": client.ExpiryTime})
	err := result.Error
	if err != nil {
		return err
	}
	return nil
}

func (s *InboundService) UpdateClientIPs(tx *gorm.DB, oldEmail string, newEmail string) error {
	return tx.Model(model.InboundClientIps{}).Where("client_email = ?", oldEmail).Update("client_email", newEmail).Error
}

func (s *InboundService) DelClientStat(tx *gorm.DB, email string) error {
	return tx.Where("email = ?", email).Delete(xray.ClientTraffic{}).Error
}

func (s *InboundService) DelClientIPs(tx *gorm.DB, email string) error {
	logger.Warning(email)
	return tx.Where("client_email = ?", email).Delete(model.InboundClientIps{}).Error
}

func (s *InboundService) ResetClientTraffic(id int, clientEmail string) error {
	db := database.GetDB()

	result := db.Model(xray.ClientTraffic{}).
		Where("inbound_id = ? and email = ?", id, clientEmail).
		Updates(map[string]interface{}{"enable": true, "up": 0, "down": 0})

	err := result.Error

	if err != nil {
		return err
	}
	return nil
}

func (s *InboundService) ResetAllClientTraffics(id int) error {
	db := database.GetDB()

	whereText := "inbound_id "
	if id == -1 {
		whereText += " > ?"
	} else {
		whereText += " = ?"
	}

	result := db.Model(xray.ClientTraffic{}).
		Where(whereText, id).
		Updates(map[string]interface{}{"enable": true, "up": 0, "down": 0})

	err := result.Error

	if err != nil {
		return err
	}
	return nil
}

func (s *InboundService) ResetAllTraffics() error {
	db := database.GetDB()

	result := db.Model(model.Inbound{}).
		Where("user_id > ?", 0).
		Updates(map[string]interface{}{"up": 0, "down": 0})

	err := result.Error

	if err != nil {
		return err
	}
	return nil
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

	whereText := "inbound_id "
	if id < 0 {
		whereText += "> ?"
	} else {
		whereText += "= ?"
	}

	depletedClients := []xray.ClientTraffic{}
	err = db.Model(xray.ClientTraffic{}).Where(whereText+" and enable = ?", id, false).Select("inbound_id, GROUP_CONCAT(email) as email").Group("inbound_id").Find(&depletedClients).Error
	if err != nil {
		return err
	}

	for _, depletedClient := range depletedClients {
		emails := strings.Split(depletedClient.Email, ",")
		oldInbound, err := s.GetInbound(depletedClient.InboundId)
		if err != nil {
			return err
		}
		var oldSettings map[string]interface{}
		err = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
		if err != nil {
			return err
		}

		oldClients := oldSettings["clients"].([]interface{})
		var newClients []interface{}
		for _, client := range oldClients {
			deplete := false
			c := client.(map[string]interface{})
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

	err = tx.Where(whereText+" and enable = ?", id, false).Delete(xray.ClientTraffic{}).Error
	if err != nil {
		return err
	}

	return nil
}

func (s *InboundService) GetClientTrafficTgBot(tguname string) ([]*xray.ClientTraffic, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Where("settings like ?", fmt.Sprintf(`%%"tgId": "%s"%%`, tguname)).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	var emails []string
	for _, inbound := range inbounds {
		clients, err := s.getClients(inbound)
		if err != nil {
			logger.Error("Unable to get clients from inbound")
		}
		for _, client := range clients {
			if client.TgID == tguname {
				emails = append(emails, client.Email)
			}
		}
	}
	var traffics []*xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).Where("email IN ?", emails).Find(&traffics).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warning(err)
			return nil, err
		}
	}
	return traffics, err
}

func (s *InboundService) GetClientTrafficByEmail(email string) (traffic *xray.ClientTraffic, err error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic

	err = db.Model(xray.ClientTraffic{}).Where("email = ?", email).Find(&traffics).Error
	if err != nil {
		logger.Warning(err)
		return nil, err
	}
	if len(traffics) > 0 {
		return traffics[0], nil
	}

	return nil, nil
}

func (s *InboundService) SearchClientTraffic(query string) (traffic *xray.ClientTraffic, err error) {
	db := database.GetDB()
	inbound := &model.Inbound{}
	traffic = &xray.ClientTraffic{}

	err = db.Model(model.Inbound{}).Where("settings like ?", "%\""+query+"\"%").First(inbound).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warning(err)
			return nil, err
		}
	}
	traffic.InboundId = inbound.Id

	// get settings clients
	settings := map[string][]model.Client{}
	json.Unmarshal([]byte(inbound.Settings), &settings)
	clients := settings["clients"]
	for _, client := range clients {
		if client.ID == query && client.Email != "" {
			traffic.Email = client.Email
			break
		}
		if client.Password == query && client.Email != "" {
			traffic.Email = client.Email
			break
		}
	}
	if traffic.Email == "" {
		return nil, err
	}
	err = db.Model(xray.ClientTraffic{}).Where("email = ?", traffic.Email).First(traffic).Error
	if err != nil {
		logger.Warning(err)
		return nil, err
	}
	return traffic, err
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

	// Fix inbounds based problems
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Where("protocol IN (?)", []string{"vmess", "vless", "trojan"}).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return
	}
	for inbound_index := range inbounds {
		settings := map[string]interface{}{}
		json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
		clients, ok := settings["clients"].([]interface{})
		if ok {
			// Fix Clinet configuration problems
			var newClients []interface{}
			for client_index := range clients {
				c := clients[client_index].(map[string]interface{})

				// Add email='' if it is not exists
				if _, ok := c["email"]; !ok {
					c["email"] = ""
				}

				// Remove "flow": "xtls-rprx-direct"
				if _, ok := c["flow"]; ok {
					if c["flow"] == "xtls-rprx-direct" {
						c["flow"] = ""
					}
				}
				newClients = append(newClients, interface{}(c))
			}
			settings["clients"] = newClients
			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return
			}

			inbounds[inbound_index].Settings = string(modifiedSettings)
		}
		// Add client traffic row for all clients which has email
		modelClients, err := s.getClients(inbounds[inbound_index])
		if err != nil {
			return
		}
		for _, modelClient := range modelClients {
			if len(modelClient.Email) > 0 {
				var count int64
				db.Model(xray.ClientTraffic{}).Where("email = ?", modelClient.Email).Count(&count)
				if count == 0 {
					s.AddClientStat(inbounds[inbound_index].Id, &modelClient)
				}
			}
		}
	}
	db.Save(inbounds)

	// Remove orphaned traffics
	db.Where("inbound_id = 0").Delete(xray.ClientTraffic{})
}
