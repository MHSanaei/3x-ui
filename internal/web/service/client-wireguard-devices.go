package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

type WireguardDeviceInbound struct {
	InboundId int                     `json:"inboundId"`
	Devices   []model.WireguardDevice `json:"devices"`
}

type WireguardDeviceList struct {
	Limit    int                      `json:"limit"`
	Inbounds []WireguardDeviceInbound `json:"inbounds"`
}

type WireguardDevicePatch struct {
	Name   *string `json:"name"`
	Enable *bool   `json:"enable"`
}

func wireguardDeviceInbounds(tx *gorm.DB, email string) ([]model.Inbound, error) {
	var inbounds []model.Inbound
	err := tx.Model(&model.Inbound{}).
		Joins("JOIN client_inbounds ON client_inbounds.inbound_id = inbounds.id").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("clients.email = ? AND inbounds.protocol = ?", email, model.WireGuard).
		Order("inbounds.id ASC").Find(&inbounds).Error
	return inbounds, err
}

func (s *ClientService) WireguardDevices(email string) (WireguardDeviceList, error) {
	result := WireguardDeviceList{Inbounds: []WireguardDeviceInbound{}}
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		record, err := s.GetRecordByEmail(tx, email)
		if err != nil {
			return err
		}
		result.Limit = model.EffectiveWireguardDeviceLimit(record.WgDeviceLimit)
		inbounds, err := wireguardDeviceInbounds(tx, email)
		if err != nil {
			return err
		}
		for _, inbound := range inbounds {
			clients, err := ParseInboundSettingsClients(inbound.Settings)
			if err != nil {
				return err
			}
			for _, client := range clients {
				if client.Email != email {
					continue
				}
				devices := []model.WireguardDevice{{Id: "primary", Name: "", Enable: client.Enable, PrivateKey: client.PrivateKey, PublicKey: client.PublicKey, AllowedIPs: client.AllowedIPs, PreSharedKey: client.PreSharedKey}}
				devices = append(devices, client.WireguardDevices...)
				result.Inbounds = append(result.Inbounds, WireguardDeviceInbound{InboundId: inbound.Id, Devices: devices})
			}
		}
		return nil
	})
	return result, err
}

func validWireguardDeviceName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 64 || strings.ContainsAny(name, "\r\n") {
		return "", errors.New("device name must contain 1 to 64 characters on one line")
	}
	return name, nil
}

// nodeWireguardDeviceChecker is the node runtime's probe; the local one serves devices itself.
type nodeWireguardDeviceChecker interface {
	CheckWireguardDevices(ctx context.Context) error
}

// requireNodesServeWireguardDevices runs before the write: a device an older node ignores
// would be handed out as a working config and leave that node pending a push it refuses.
func requireNodesServeWireguardDevices(email string) error {
	inbounds, err := wireguardDeviceInbounds(database.GetDB(), email)
	if err != nil {
		return err
	}
	mgr := runtime.GetManager()
	checked := make(map[int]struct{})
	for _, inbound := range inbounds {
		if inbound.NodeID == nil || mgr == nil {
			continue
		}
		if _, done := checked[*inbound.NodeID]; done {
			continue
		}
		checked[*inbound.NodeID] = struct{}{}
		rt, err := mgr.RuntimeFor(inbound.NodeID)
		if err != nil {
			return err
		}
		checker, ok := rt.(nodeWireguardDeviceChecker)
		if !ok {
			continue
		}
		ctx, cancel := nodePushContext()
		err = checker.CheckWireguardDevices(ctx)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ClientService) AddWireguardDevice(email, name string) error {
	name, err := validWireguardDeviceName(name)
	if err != nil {
		return err
	}
	privateKey, publicKey, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		return err
	}
	device := model.WireguardDevice{Id: uuid.NewString(), Name: name, Enable: true, PrivateKey: privateKey, PublicKey: publicKey}
	if err := requireNodesServeWireguardDevices(email); err != nil {
		return err
	}
	return s.mutateWireguardDevices(email, func(client *model.Client) error {
		if len(client.WireguardDevices)+1 >= model.EffectiveWireguardDeviceLimit(client.WgDeviceLimit) {
			return errors.New("WireGuard device limit reached")
		}
		client.WireguardDevices = append(model.CloneWireguardDevices(client.WireguardDevices), device)
		return nil
	})
}

func (s *ClientService) UpdateWireguardDevice(email, id string, patch WireguardDevicePatch) error {
	if patch.Name != nil {
		name, err := validWireguardDeviceName(*patch.Name)
		if err != nil {
			return err
		}
		patch.Name = &name
	}
	return s.mutateWireguardDevices(email, func(client *model.Client) error {
		for i := range client.WireguardDevices {
			device := &client.WireguardDevices[i]
			if device.Id != id {
				continue
			}
			if patch.Name != nil {
				device.Name = *patch.Name
			}
			if patch.Enable != nil {
				device.Enable = *patch.Enable
			}
			return nil
		}
		return errors.New("WireGuard device not found")
	})
}

func (s *ClientService) DeleteWireguardDevice(email, id string) error {
	return s.mutateWireguardDevices(email, func(client *model.Client) error {
		for i, device := range client.WireguardDevices {
			if device.Id == id {
				client.WireguardDevices = append(client.WireguardDevices[:i:i], client.WireguardDevices[i+1:]...)
				if client.WireguardDevices == nil {
					client.WireguardDevices = []model.WireguardDevice{}
				}
				return nil
			}
		}
		return errors.New("WireGuard device not found")
	})
}

func (s *ClientService) SetWireguardDeviceLimit(email string, limit int) error {
	if limit < 1 || limit > 32 {
		return errors.New("WireGuard device limit must be between 1 and 32")
	}
	return s.mutateWireguardDevices(email, func(client *model.Client) error {
		if len(client.WireguardDevices)+1 > limit {
			return errors.New("device limit is below the current device count")
		}
		client.WgDeviceLimit = limit
		return nil
	})
}

// All inbounds commit together. The serialized writer also protects address allocation.
func (s *ClientService) mutateWireguardDevices(email string, change func(*model.Client) error) error {
	inbounds, err := wireguardDeviceInbounds(database.GetDB(), email)
	if err != nil {
		return err
	}
	for _, inbound := range inbounds {
		lock := lockInbound(inbound.Id)
		defer lock.Unlock()
	}
	err = runSerializedTx(func(tx *gorm.DB) error {
		record, err := s.GetRecordByEmail(tx, email)
		if err != nil {
			return err
		}
		current, err := wireguardDeviceInbounds(tx, email)
		if err != nil {
			return err
		}
		if len(current) == 0 {
			return errors.New("client has no WireGuard inbounds")
		}
		if len(current) != len(inbounds) {
			return errors.New("client attachments changed, retry the device operation")
		}
		for i := range current {
			if current[i].Id != inbounds[i].Id {
				return errors.New("client attachments changed, retry the device operation")
			}
		}
		for _, inbound := range current {
			clients, err := ParseInboundSettingsClients(inbound.Settings)
			if err != nil {
				return err
			}
			var settings map[string]json.RawMessage
			if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
				return err
			}
			var entries []map[string]json.RawMessage
			if err := json.Unmarshal(settings["clients"], &entries); err != nil {
				return err
			}
			found := false
			for i := range clients {
				client := &clients[i]
				if client.Email != email {
					continue
				}
				found = true
				client.WgDeviceLimit = record.WgDeviceLimit
				if err := change(client); err != nil {
					return err
				}
				others := append([]model.Client(nil), clients[:i]...)
				others = append(others, clients[i+1:]...)
				crossUsed, err := s.otherTunnelAllowedIPs(tx, &InboundService{}, inbound.Id, map[string]struct{}{strings.ToLower(email): {}})
				if err != nil {
					return err
				}
				if err := validateWireguardDevices(inbound.Settings, others, client, crossUsed); err != nil {
					return fmt.Errorf("inbound %d: %w", inbound.Id, err)
				}
				entries[i]["wireguardDevices"], err = json.Marshal(client.WireguardDevices)
				if err != nil {
					return err
				}
				entries[i]["wgDeviceLimit"], err = json.Marshal(client.WgDeviceLimit)
				if err != nil {
					return err
				}
				if err := s.ApplyInboundClientDelta(tx, inbound.Id, []model.Client{*client}, nil); err != nil {
					return err
				}
			}
			if !found {
				return errors.New("WireGuard client settings are missing")
			}
			settings["clients"], err = json.Marshal(entries)
			if err != nil {
				return err
			}
			encoded, err := json.Marshal(settings)
			if err != nil {
				return err
			}
			if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).Update("settings", string(encoded)).Error; err != nil {
				return err
			}
			if inbound.NodeID != nil {
				if err := (&NodeService{}).MarkNodeDirtyTx(tx, *inbound.NodeID); err != nil {
					return err
				}
			}
		}
		return tx.Model(&model.ClientRecord{}).Where("id = ?", record.Id).Update("updated_at", time.Now().UnixMilli()).Error
	})
	if err == nil {
		InvalidateSubData()
	}
	return err
}
