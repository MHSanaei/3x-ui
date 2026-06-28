package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// buildPeerMap converts a ClientRecord into a peer map for settings.peers[].
// Returns nil if the record has no WgSettings.
func buildPeerMap(rec *model.ClientRecord) (map[string]any, error) {
	if rec.WgSettings == "" {
		return nil, nil
	}
	var wg model.WgPeerSettings
	if err := json.Unmarshal([]byte(rec.WgSettings), &wg); err != nil {
		return nil, err
	}
	peer := map[string]any{
		"publicKey":  wg.PublicKey,
		"allowedIPs": wg.AllowedIPs,
	}
	if rec.Password != "" {
		peer["privateKey"] = rec.Password
	}
	if wg.PreSharedKey != "" {
		peer["preSharedKey"] = wg.PreSharedKey
	}
	if wg.KeepAlive > 0 {
		peer["keepAlive"] = wg.KeepAlive
	}
	if rec.Email != "" {
		peer["email"] = rec.Email
	}
	if rec.Comment != "" {
		peer["comment"] = rec.Comment
	}
	return peer, nil
}

func wgPeerPublicKey(rec *model.ClientRecord) string {
	if rec == nil || rec.WgSettings == "" {
		return ""
	}
	var wg model.WgPeerSettings
	if err := json.Unmarshal([]byte(rec.WgSettings), &wg); err != nil {
		return ""
	}
	return wg.PublicKey
}

func wgPeerMatches(peer map[string]any, email string, publicKey string) bool {
	if publicKey != "" {
		pk, _ := peer["publicKey"].(string)
		return pk == publicKey
	}
	return email != "" && (peer["email"] == email || peer["comment"] == email)
}

// addPeerToSettings appends one peer to settings.peers[] without a DB query.
func addPeerToSettings(settingsJSON string, peer map[string]any) (string, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return settingsJSON, err
	}
	raw, _ := settings["peers"].([]any)
	settings["peers"] = append(raw, peer)
	updated, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return settingsJSON, err
	}
	return string(updated), nil
}

// removePeerFromSettings removes the peer by public key, falling back to email
// for legacy rows with invalid/missing wg_settings.
func removePeerFromSettings(settingsJSON string, email string, publicKey string) (string, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return settingsJSON, err
	}
	raw, _ := settings["peers"].([]any)
	peers := make([]any, 0, len(raw))
	for _, p := range raw {
		m, ok := p.(map[string]any)
		if ok && wgPeerMatches(m, email, publicKey) {
			continue
		}
		peers = append(peers, p)
	}
	settings["peers"] = peers
	updated, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return settingsJSON, err
	}
	return string(updated), nil
}

// updatePeerInSettings removes the previous peer and (if enabled and peer != nil)
// appends newPeer. Disabled peers are absent from peers[].
func updatePeerInSettings(settingsJSON, oldEmail, oldPublicKey string, newPeer map[string]any, enabled bool) (string, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return settingsJSON, err
	}
	raw, _ := settings["peers"].([]any)
	peers := make([]any, 0, len(raw)+1)
	for _, p := range raw {
		m, ok := p.(map[string]any)
		if ok && wgPeerMatches(m, oldEmail, oldPublicKey) {
			continue
		}
		peers = append(peers, p)
	}
	if enabled && newPeer != nil {
		peers = append(peers, newPeer)
	}
	settings["peers"] = peers
	updated, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return settingsJSON, err
	}
	return string(updated), nil
}

// wgPeersFromSettings extracts the peers array from a WireGuard inbound settings JSON.
func wgPeersFromSettings(settingsJSON string) ([]map[string]any, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return nil, err
	}
	raw, _ := settings["peers"].([]any)
	peers := make([]map[string]any, 0, len(raw))
	for _, p := range raw {
		if m, ok := p.(map[string]any); ok {
			peers = append(peers, m)
		}
	}
	return peers, nil
}

// wgPeerToRecord converts a WireGuard peer map (from settings.peers[]) into a
// ClientRecord for migration. If comment is empty, a fallback email is generated
// using the inbound id and index.
func wgPeerToRecord(peer map[string]any, inboundId int, idx int) *model.ClientRecord {
	allowedIPs := []string{}
	if ips, ok := peer["allowedIPs"].([]any); ok {
		for _, ip := range ips {
			if s, ok := ip.(string); ok {
				allowedIPs = append(allowedIPs, s)
			}
		}
	}

	pubKey, _ := peer["publicKey"].(string)
	privKey, _ := peer["privateKey"].(string)
	psk, _ := peer["preSharedKey"].(string)
	keepAlive := 0
	if ka, ok := peer["keepAlive"].(float64); ok {
		keepAlive = int(ka)
	}

	email, _ := peer["email"].(string)
	comment, _ := peer["comment"].(string)
	if email == "" {
		email = comment
	}
	if email == "" {
		email = fmt.Sprintf("wg-%d-peer-%d", inboundId, idx+1)
	}
	if comment == email {
		comment = ""
	}

	wg := model.WgPeerSettings{
		PublicKey:    pubKey,
		PreSharedKey: psk,
		AllowedIPs:   allowedIPs,
		KeepAlive:    keepAlive,
	}
	wgJSON, _ := json.Marshal(wg)

	return &model.ClientRecord{
		Email:      email,
		Password:   privKey,
		Enable:     true,
		Comment:    comment,
		WgSettings: string(wgJSON),
	}
}

// SyncWgInbound imports WireGuard peers from settings.peers[] into the clients
// table and ensures links exist. It intentionally does not delete existing links:
// disabled WG clients are absent from peers[] but must stay attached.
func (s *ClientService) SyncWgInbound(tx *gorm.DB, inbound *model.Inbound) error {
	if tx == nil {
		tx = database.GetDB()
	}
	peers, err := wgPeersFromSettings(inbound.Settings)
	if err != nil {
		return err
	}

	// Deduplicate by email: if two peers have the same comment, keep first.
	seen := make(map[string]struct{}, len(peers))
	for i, peer := range peers {
		rec := wgPeerToRecord(peer, inbound.Id, i)
		if _, dup := seen[rec.Email]; dup {
			continue
		}
		seen[rec.Email] = struct{}{}

		var existing model.ClientRecord
		err := tx.Where("email = ?", rec.Email).First(&existing).Error
		if err == nil {
			before := existing
			if rec.Password != "" {
				existing.Password = rec.Password
			}
			if rec.WgSettings != "" {
				existing.WgSettings = rec.WgSettings
			}
			if rec.Comment != "" {
				existing.Comment = rec.Comment
			}
			if before != existing {
				if err := tx.Save(&existing).Error; err != nil {
					return err
				}
			}
			rec.Id = existing.Id
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(rec).Error; err != nil {
				return err
			}
		} else {
			return err
		}

		link := model.ClientInbound{ClientId: rec.Id, InboundId: inbound.Id}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *ClientService) BuildWgSettingsFromClients(tx *gorm.DB, inbound *model.Inbound, settingsJSON string) (string, error) {
	if tx == nil {
		tx = database.GetDB()
	}
	settings := map[string]any{}
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return settingsJSON, err
	}
	clients, err := s.ListForInbound(tx, inbound.Id)
	if err != nil {
		return settingsJSON, err
	}
	peers := make([]any, 0, len(clients))
	for i := range clients {
		if !clients[i].Enable || clients[i].WgPeer == nil {
			continue
		}
		rec := clients[i].ToRecord()
		peer, err := buildPeerMap(rec)
		if err != nil {
			return settingsJSON, err
		}
		if peer != nil {
			peers = append(peers, peer)
		}
	}
	settings["peers"] = peers
	updated, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return settingsJSON, err
	}
	return string(updated), nil
}
