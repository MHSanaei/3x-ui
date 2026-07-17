package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// applyLocalAmneziaWG ensures the AWG Docker container reflects the current
// inbound settings after a client or inbound edit.
func (s *InboundService) applyLocalAmneziaWG(inboundID int) {
	inbound, err := s.GetInbound(inboundID)
	if err != nil || inbound == nil || inbound.Protocol != model.AmneziaWG || inbound.NodeID != nil {
		return
	}
	rt, err := s.runtimeFor(inbound)
	if err != nil {
		return
	}
	if err := rt.UpdateInbound(context.Background(), inbound, inbound); err != nil {
		logger.Debug("amneziawg: immediate apply failed for inbound", inboundID, ":", err)
	}
}

// buildAmneziaWGSettings generates fresh AWG server parameters and constructs
// the settings JSON for a new inbound. Used when creating a new AWG inbound.
func (s *InboundService) buildAmneziaWGSettings(inbound *model.Inbound) error {
	params := amneziawg.GenerateAWGParams()
	if inbound.Port > 0 {
		params.ServerPort = inbound.Port
	}

	return nil
}

// normalizeAmneziaWGSettings ensures the inbound's settings have the required
// server block, generating it if missing. Called during add and update.
func (s *InboundService) normalizeAmneziaWGSettings(inbound *model.Inbound) error {
	logger.Infof("[awg-debug] normalizeAmneziaWGSettings called for inbound ID=%d, settings empty=%v", inbound.Id, inbound.Settings == "" || inbound.Settings == "null" || inbound.Settings == "{}")

	if inbound.Settings == "" || inbound.Settings == "null" || inbound.Settings == "{}" {
		params := amneziawg.GenerateAWGParams()
		params.ServerPort = inbound.Port
		fillAWGKeys(&params)
		settings := amneziawg.SettingsInbound{
			Server:  &params,
			Clients: []amneziawg.ClientSettings{},
		}
		bs, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return err
		}
		inbound.Settings = string(bs)
		logger.Infof("[awg-debug] generated new settings, pubKey=%s", params.PublicKey)
		return nil
	}

	var parsed amneziawg.SettingsInbound
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil {
		logger.Infof("[awg-debug] failed to parse AWG settings: %v", err)
		return fmt.Errorf("failed to parse AWG settings: %w", err)
	}
	if parsed.Server == nil {
		logger.Infof("[awg-debug] server config missing from settings, generating new")
		params := amneziawg.GenerateAWGParams()
		params.ServerPort = inbound.Port
		fillAWGKeys(&params)
		parsed.Server = &params
		bs, err := json.MarshalIndent(parsed, "", "  ")
		if err != nil {
			return err
		}
		inbound.Settings = string(bs)
		logger.Infof("[awg-debug] regenerated settings with server, pubKey=%s", params.PublicKey)
	} else {
		logger.Infof("[awg-debug] server already present, pubKey=%s privateKey_len=%d",
			parsed.Server.PublicKey, len(parsed.Server.PrivateKey))
		dirty := false
		if parsed.Server.ServerPort != inbound.Port {
			parsed.Server.ServerPort = inbound.Port
			dirty = true
		}
		if len(parsed.Server.PrivateKey) != 44 {
			logger.Infof("[awg-debug] privateKey len=%d (expected 44), regenerating keys", len(parsed.Server.PrivateKey))
			fillAWGKeys(parsed.Server)
			dirty = true
		}
		if dirty {
			bs, err := json.MarshalIndent(parsed, "", "  ")
			if err != nil {
				return err
			}
			inbound.Settings = string(bs)
		}
	}
	return nil
}

// fillAWGKeys generates real WireGuard key material when the server block is
// being created for the first time, or replaces placeholder keys.
func fillAWGKeys(s *amneziawg.ServerConfig) {
	if s.PublicKey != "" && strings.HasSuffix(s.PublicKey, "=") {
		logger.Infof("[awg-debug] fillAWGKeys: valid padded publicKey already set, skipping")
		return
	}
	logger.Infof("[awg-debug] fillAWGKeys: generating new WireGuard key pair")
	if priv, pub, psk, err := amneziawg.GenerateWireGuardKeyPair(); err == nil {
		s.PrivateKey = priv
		s.PublicKey = pub
		s.PSK = psk
		logger.Infof("[awg-debug] fillAWGKeys: generated keys ok, pubKey=%s", pub)
	} else {
		logger.Infof("[awg-debug] fillAWGKeys: GenerateWireGuardKeyPair failed: %v", err)
	}
}

// getAmneziaWGServer extracts the server config from inbound settings.
func getAmneziaWGServer(settings string) (*amneziawg.ServerConfig, error) {
	var parsed amneziawg.SettingsInbound
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return nil, err
	}
	if parsed.Server == nil {
		return nil, fmt.Errorf("AWG settings missing server config")
	}
	return parsed.Server, nil
}

// getAmneziaWGClients extracts the client list from inbound settings.
func getAmneziaWGClients(settings string) ([]amneziawg.ClientSettings, error) {
	var parsed amneziawg.SettingsInbound
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return nil, err
	}
	return parsed.Clients, nil
}

// setAmneziaWGSettingsServer updates the server config in the settings JSON.
func setAmneziaWGSettingsServer(settings string, server *amneziawg.ServerConfig) (string, error) {
	var parsed amneziawg.SettingsInbound
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return "", err
	}
	parsed.Server = server
	bs, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

