package service

import (
	"encoding/json"
	"fmt"
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

func (s *InboundService) checkEmailsExist(emails map[string]bool, ignoreId int) (string, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	db = db.Model(model.Inbound{}).Where("Protocol in ?", []model.Protocol{model.VMess, model.VLESS, model.Trojan})
	if ignoreId > 0 {
		db = db.Where("id != ?", ignoreId)
	}
	db = db.Find(&inbounds)
	if db.Error != nil {
		return "", db.Error
	}

	for _, inbound := range inbounds {
		clients, err := s.getClients(inbound)
		if err != nil {
			return "", err
		}

		for _, client := range clients {
			if emails[client.Email] {
				return client.Email, nil
			}
		}
	}
	return "", nil
}

func (s *InboundService) checkEmailExistForInbound(inbound *model.Inbound) (string, error) {
	clients, err := s.getClients(inbound)
	if err != nil {
		return "", err
	}
	emails := make(map[string]bool)
	for _, client := range clients {
		if client.Email != "" {
			if emails[client.Email] {
				return client.Email, nil
			}
			emails[client.Email] = true
		}
	}
	return s.checkEmailsExist(emails, inbound.Id)
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

	existEmail, err := s.checkEmailExistForInbound(inbound)
	if err != nil {
		return inbound, err
	}
	if existEmail != "" {
		return inbound, common.NewError("Duplicate email:", existEmail)
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

func (s *InboundService) AddInboundClient(inbound *model.Inbound) error {
	existEmail, err := s.checkEmailExistForInbound(inbound)
	if err != nil {
		return err
	}

	if existEmail != "" {
		return common.NewError("Duplicate email:", existEmail)
	}

	clients, err := s.getClients(inbound)
	if err != nil {
		return err
	}

	oldInbound, err := s.GetInbound(inbound.Id)
	if err != nil {
		return err
	}

	oldClients, err := s.getClients(oldInbound)
	if err != nil {
		return err
	}

	oldInbound.Settings = inbound.Settings

	if len(clients[len(clients)-1].Email) > 0 {
		s.AddClientStat(inbound.Id, &clients[len(clients)-1])
	}
	for i := len(oldClients); i < len(clients); i++ {
		if len(clients[i].Email) > 0 {
			s.AddClientStat(inbound.Id, &clients[i])
		}
	}
	db := database.GetDB()
	return db.Save(oldInbound).Error
}

func (s *InboundService) DelInboundClient(inbound *model.Inbound, email string) error {
	db := database.GetDB()
	err := s.DelClientStat(db, email)
	if err != nil {
		logger.Error("Delete stats Data Error")
		return err
	}

	oldInbound, err := s.GetInbound(inbound.Id)
	if err != nil {
		logger.Error("Load Old Data Error")
		return err
	}

	oldInbound.Settings = inbound.Settings

	err = s.DelClientIPs(db, email)
	if err != nil {
		logger.Error("Error in delete client IPs")
		return err
	}

	return db.Save(oldInbound).Error
}

func (s *InboundService) UpdateInboundClient(inbound *model.Inbound, index int) error {
	existEmail, err := s.checkEmailExistForInbound(inbound)
	if err != nil {
		return err
	}
	if existEmail != "" {
		return common.NewError("Duplicate email:", existEmail)
	}

	clients, err := s.getClients(inbound)
	if err != nil {
		return err
	}

	oldInbound, err := s.GetInbound(inbound.Id)
	if err != nil {
		return err
	}

	oldClients, err := s.getClients(oldInbound)
	if err != nil {
		return err
	}

	oldInbound.Settings = inbound.Settings

	db := database.GetDB()

	if len(clients[index].Email) > 0 {
		if len(oldClients[index].Email) > 0 {
			err = s.UpdateClientStat(oldClients[index].Email, &clients[index])
			if err != nil {
				return err
			}
			err = s.UpdateClientIPs(db, oldClients[index].Email, clients[index].Email)
			if err != nil {
				return err
			}
		} else {
			s.AddClientStat(inbound.Id, &clients[index])
		}
	} else {
		err = s.DelClientStat(db, oldClients[index].Email)
		if err != nil {
			return err
		}
		err = s.DelClientIPs(db, oldClients[index].Email)
		if err != nil {
			return err
		}
	}
	return db.Save(oldInbound).Error
}

func (s *InboundService) AddTraffic(traffics []*xray.Traffic) (err error) {
	if len(traffics) == 0 {
		return nil
	}
	db := database.GetDB()
	db = db.Model(model.Inbound{})
	tx := db.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	for _, traffic := range traffics {
		if traffic.IsInbound {
			err = tx.Where("tag = ?", traffic.Tag).
				UpdateColumns(map[string]interface{}{
					"up":   gorm.Expr("up + ?", traffic.Up),
					"down": gorm.Expr("down + ?", traffic.Down)}).Error
			if err != nil {
				return
			}
		}
	}
	return
}
func (s *InboundService) AddClientTraffic(traffics []*xray.ClientTraffic) (err error) {
	if len(traffics) == 0 {
		return nil
	}

	traffics, err = s.adjustTraffics(traffics)
	if err != nil {
		return err
	}

	db := database.GetDB()
	db = db.Model(xray.ClientTraffic{})
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	err = tx.Save(traffics).Error
	if err != nil {
		logger.Warning("AddClientTraffic update data ", err)
	}

	return nil
}

func (s *InboundService) adjustTraffics(traffics []*xray.ClientTraffic) (full_traffics []*xray.ClientTraffic, err error) {
	db := database.GetDB()
	dbInbound := db.Model(model.Inbound{})
	txInbound := dbInbound.Begin()

	defer func() {
		if err != nil {
			txInbound.Rollback()
		} else {
			txInbound.Commit()
		}
	}()

	for _, traffic := range traffics {
		inbound := &model.Inbound{}
		client_traffic := &xray.ClientTraffic{}
		err := db.Model(xray.ClientTraffic{}).Where("email = ?", traffic.Email).First(client_traffic).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warning(err, traffic.Email)
			}
			continue
		}
		client_traffic.Up += traffic.Up
		client_traffic.Down += traffic.Down

		err = txInbound.Where("id=?", client_traffic.InboundId).First(inbound).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				logger.Warning(err, traffic.Email)
			}
			continue
		}
		// get clients
		clients, err := s.getClients(inbound)
		needUpdate := false
		if err == nil {
			for client_index, client := range clients {
				if traffic.Email == client.Email {
					if client.ExpiryTime < 0 {
						clients[client_index].ExpiryTime = (time.Now().Unix() * 1000) - client.ExpiryTime
						needUpdate = true
					}
					client_traffic.ExpiryTime = client.ExpiryTime
					client_traffic.Total = client.TotalGB
					break
				}
			}
		}

		if needUpdate {
			settings := map[string]interface{}{}
			json.Unmarshal([]byte(inbound.Settings), &settings)

			// Convert clients to []interface to update clients in settings
			var clientsInterface []interface{}
			for _, c := range clients {
				clientsInterface = append(clientsInterface, interface{}(c))
			}

			settings["clients"] = clientsInterface
			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return nil, err
			}

			err = txInbound.Where("id=?", inbound.Id).Update("settings", string(modifiedSettings)).Error
			if err != nil {
				return nil, err
			}
		}

		full_traffics = append(full_traffics, client_traffic)
	}
	return full_traffics, nil
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

	result := db.Model(xray.ClientTraffic{}).
		Where("inbound_id = ?", id).
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

func (s *InboundService) GetClientTrafficByEmail(email string) (traffic []*xray.ClientTraffic, err error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic

	err = db.Model(xray.ClientTraffic{}).Where("email like ?", "%"+email+"%").Find(&traffics).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warning(err)
			return nil, err
		}
	}
	return traffics, err
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
