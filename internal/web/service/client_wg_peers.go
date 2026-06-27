package service

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

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
		peer["comment"] = rec.Email
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
	return email != "" && peer["comment"] == email
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

	email, _ := peer["comment"].(string)
	if email == "" {
		email = fmt.Sprintf("wg-%d-peer-%d", inboundId, idx+1)
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

	// Deduplicate by email: if two peers have the same comment, keep first.
	seen := make(map[string]struct{}, len(peers))
	clients := make([]model.Client, 0, len(peers))
	for i, peer := range peers {
		rec := wgPeerToRecord(peer, inbound.Id, i)
		if _, dup := seen[rec.Email]; dup {
			continue
		}
		seen[rec.Email] = struct{}{}
		c := rec.ToClient()
		clients = append(clients, *c)
	}

	return s.SyncInbound(tx, inbound.Id, clients)
}
