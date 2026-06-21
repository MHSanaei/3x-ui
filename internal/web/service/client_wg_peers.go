package service

import (
	"encoding/json"
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"gorm.io/gorm"
)

// syncWgPeersFromClients rebuilds settings.peers[] in the inbound JSON from
// the provided list of WireGuard client records. Call this after any WireGuard
// client create/update/delete so that xray always sees the canonical peer list.
//
// Returns the updated settings JSON string.
func syncWgPeersFromClients(settingsJSON string, clients []*model.ClientRecord) (string, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return settingsJSON, err
	}

	peers := make([]map[string]any, 0, len(clients))
	for _, rec := range clients {
		if !rec.Enable {
			continue
		}
		if rec.WgSettings == "" {
			continue
		}
		var wg model.WgPeerSettings
		if err := json.Unmarshal([]byte(rec.WgSettings), &wg); err != nil {
			continue
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
			peer["comment"] = rec.Email
		}
		peers = append(peers, peer)
	}

	settings["peers"] = peers

	updated, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return settingsJSON, err
	}
	return string(updated), nil
}

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
		peer["comment"] = rec.Email
	}
	return peer, nil
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

// removePeerFromSettings removes the peer whose comment matches email.
func removePeerFromSettings(settingsJSON string, email string) (string, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return settingsJSON, err
	}
	raw, _ := settings["peers"].([]any)
	peers := make([]any, 0, len(raw))
	for _, p := range raw {
		m, ok := p.(map[string]any)
		if ok && m["comment"] == email {
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

// updatePeerInSettings removes the peer with oldEmail and (if enabled and peer != nil)
// appends newPeer. Mirrors syncWgPeersFromClients: disabled peers are absent from peers[].
func updatePeerInSettings(settingsJSON, oldEmail string, newPeer map[string]any, enabled bool) (string, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return settingsJSON, err
	}
	raw, _ := settings["peers"].([]any)
	peers := make([]any, 0, len(raw)+1)
	for _, p := range raw {
		m, ok := p.(map[string]any)
		if ok && m["comment"] == oldEmail {
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
// using the inbound name and index.
func wgPeerToRecord(peer map[string]any, inboundName string, idx int) *model.ClientRecord {
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

	email, _ := peer["comment"].(string)
	if email == "" {
		email = fmt.Sprintf("%s-peer-%d", inboundName, idx+1)
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
		WgSettings: string(wgJSON),
	}
}

// SyncWgInbound builds Client list from settings.peers[] and calls SyncInbound
// so that the clients table and client_inbounds junction stay in sync with the
// inbound's peers array. Used during migration and after inbound save.
func (s *ClientService) SyncWgInbound(tx *gorm.DB, inbound *model.Inbound) error {
	peers, err := wgPeersFromSettings(inbound.Settings)
	if err != nil {
		return err
	}

	inboundName := inbound.Remark
	if inboundName == "" {
		inboundName = fmt.Sprintf("wg-%d", inbound.Id)
	}

	// Deduplicate by email: if two peers have the same comment, keep first.
	seen := make(map[string]struct{}, len(peers))
	clients := make([]model.Client, 0, len(peers))
	for i, peer := range peers {
		rec := wgPeerToRecord(peer, inboundName, i)
		if _, dup := seen[rec.Email]; dup {
			continue
		}
		seen[rec.Email] = struct{}{}
		c := rec.ToClient()
		clients = append(clients, *c)
	}

	return s.SyncInbound(tx, inbound.Id, clients)
}
