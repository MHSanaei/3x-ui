package service

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

type MultiServerService struct{}

func (s *MultiServerService) GetServers() ([]*model.Server, error) {
	db := database.GetDB()
	var servers []*model.Server
	err := db.Find(&servers).Error
	return servers, err
}

func (s *MultiServerService) GetServer(id int) (*model.Server, error) {
	db := database.GetDB()
	var server model.Server
	err := db.First(&server, id).Error
	return &server, err
}

// GetOnlineClients
func (s *MultiServerService) GetOnlineClients() (map[int][]string, error) {
	db := database.GetDB()
	var servers []*model.Server
	err := db.Find(&servers).Error
	if err != nil {
		return nil, err
	}

	 clients := make( map[int][]string)
	for _, server := range servers {
		var onlineResp struct {
			Success bool     `json:"success"`
			Msg     string   `json:"msg"`
			Obj     []string `json:"obj"`
		}
		url := fmt.Sprintf("http://%s:%d%spanel/api/inbounds/onlines", server.Address, server.Port, server.SecretWebPath)
		resp, err := http.Post(url, "application/json", nil)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(&onlineResp); err != nil {
			return nil, fmt.Errorf("decode online: %w", err)
		}
		if !onlineResp.Success {
			return nil, fmt.Errorf("failed to get online list at %s", server.Address)
		}
		clients[server.Id] = onlineResp.Obj
	}
	return clients, nil
}

func (s *MultiServerService) AddServer(server *model.Server) error {
	db := database.GetDB()
	return db.Create(server).Error
}

func (s *MultiServerService) UpdateServer(server *model.Server) error {
	db := database.GetDB()
	return db.Save(server).Error
}

func (s *MultiServerService) DeleteServer(id int) error {
	db := database.GetDB()
	return db.Delete(&model.Server{}, id).Error
}
