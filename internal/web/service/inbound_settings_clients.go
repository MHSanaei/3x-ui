package service

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func ParseInboundSettingsClients(settings string) ([]model.Client, error) {
	var payload struct {
		Clients json.RawMessage `json:"clients"`
	}
	if err := json.Unmarshal([]byte(settings), &payload); err != nil {
		return nil, err
	}
	if len(payload.Clients) == 0 || string(payload.Clients) == "null" {
		return nil, nil
	}

	var clients []model.Client
	if err := json.Unmarshal(payload.Clients, &clients); err != nil {
		return nil, err
	}
	return clients, nil
}
