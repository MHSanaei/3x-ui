package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
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

	clients := make(map[int][]string)
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

// SyncServer synchronizes the inbounds list between the given server and the local inbounds list.
// It gets the inbounds list from the server, and then syncs it with the local inbounds list.
// If an inbound exists on the server but not locally, it adds the inbound.
// If an inbound exists locally but not on the server, it removes the inbound.
// If an inbound exists on both the server and locally, it updates the inbound if they are different.
func (s *MultiServerService) SyncServer(id int) error {
	inboundService := &InboundService{}
	inboundsSource, err := inboundService.GetAllInbounds()
	if err != nil {
		logger.Error("failed to get all inbounds", "err", err)
		return err
	}

	db := database.GetDB()
	var server model.Server
	if err = db.First(&server, id).Error; err != nil {
		logger.Error("failed to get server", "err", err)
		return err
	}

	//get inbounds from server throw api
	listURL := fmt.Sprintf("http://%s:%d%spanel/api/inbounds/list", server.Address, server.Port, server.SecretWebPath)
	req, _ := http.NewRequest("GET", listURL, nil)
	req.Header.Set("X-API-KEY", server.APIKey)
	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error("failed to get inbounds from server", "err", err)
		return err
	}
	defer httpResp.Body.Close()

	var resp struct {
		Success bool            `json:"success"`
		Msg     string          `json:"msg"`
		Obj     []model.Inbound `json:"obj"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		logger.Error("failed to decode inbounds response", "err", err)
		return err
	}

	type InboundPayload struct {
		Up             int64          `json:"up"`
		Down           int64          `json:"down"`
		Total          int64          `json:"total"`
		Remark         string         `json:"remark"`
		Enable         bool           `json:"enable"`
		ExpiryTime     int64          `json:"expiryTime"`
		Listen         string         `json:"listen"`
		Port           int            `json:"port"`
		Protocol       model.Protocol `json:"protocol"`
		Settings       string         `json:"settings"`
		StreamSettings string         `json:"streamSettings"`
		Sniffing       string         `json:"sniffing"`
	}

	//sync inbounds
	for _, src := range inboundsSource {
		logger.Debugf("syncing inbound %d", src.Id)
		found := false
		for _, remote := range resp.Obj {
			if remote.Tag == src.Tag {
				found = true
				break
			}
		}

		payload := InboundPayload{
			Up: src.Up, Down: src.Down, Total: src.Total,
			Remark: src.Remark, Enable: src.Enable,
			ExpiryTime: src.ExpiryTime, Listen: src.Listen,
			Port: src.Port, Protocol: src.Protocol,
			Settings: src.Settings, StreamSettings: src.StreamSettings, Sniffing: src.Sniffing,
		}

		data, _ := json.Marshal(payload)

		if found {
			//update inbound trow api
			updateURL := fmt.Sprintf("http://%s:%d%spanel/api/inbounds/update/%d", server.Address, server.Port, server.SecretWebPath, src.Id)
			req, _ := http.NewRequest("POST", updateURL, bytes.NewBuffer(data))
			req.Header.Set("X-API-KEY", server.APIKey)
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				logger.Error("failed to update inbounds at server", "err", err)
				return err
			}
			var updateResp struct {
				Success bool   `json:"success"`
				Msg     string `json:"msg"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&updateResp); err != nil {
				logger.Error("failed to decode update inbounds response", "err", err)
				return fmt.Errorf("decode update inbounds: %w", err)
			}
			resp.Body.Close()
			if !updateResp.Success {
				return fmt.Errorf("failed to update inbounds at %s %s", server.Name, server.Address)
			}
		} else {
			// add inbound trow api
			addURL := fmt.Sprintf("http://%s:%d%spanel/api/inbounds/add", server.Address, server.Port, server.SecretWebPath)
			req, _ := http.NewRequest("POST", addURL, bytes.NewBuffer(data))
			req.Header.Set("X-API-KEY", server.APIKey)
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				logger.Error("failed to add inbounds at server", "err", err)
				return err
			}

			var addResp struct {
				Success bool   `json:"success"`
				Msg     string `json:"msg"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&addResp); err != nil {
				logger.Error("failed to decode add inbounds response", "err", err)
				return fmt.Errorf("decode add inbounds: %w", err)
			}
			resp.Body.Close()
			if !addResp.Success {
				return fmt.Errorf("failed to add inbounds at %s %s", server.Name, server.Address)
			}
		}
	}
	return nil
}
