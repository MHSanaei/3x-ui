package service

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

const wireGuardPeerDetachedKey = "clientDetached"

type wireGuardClientBinding struct {
	id      int
	email   string
	subID   string
	comment string
}

func jsonInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, n > 0
	case int64:
		return int(n), n > 0
	case float64:
		i := int(n)
		return i, n == float64(i) && i > 0
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil && i > 0
	default:
		return 0, false
	}
}

func jsonBool(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func sameJSONValue(existing any, want any) bool {
	switch w := want.(type) {
	case int:
		got, ok := jsonInt(existing)
		return ok && got == w
	default:
		return existing == want
	}
}

func setWireGuardPeerClient(peer map[string]any, c wireGuardClientBinding) bool {
	changed := false
	set := func(key string, value any) {
		if !sameJSONValue(peer[key], value) {
			peer[key] = value
			changed = true
		}
	}

	oldEmail, _ := peer["clientEmail"].(string)
	comment, _ := peer["comment"].(string)

	set("clientId", c.id)
	set("clientEmail", c.email)
	if _, ok := peer[wireGuardPeerDetachedKey]; ok {
		delete(peer, wireGuardPeerDetachedKey)
		changed = true
	}
	if c.subID != "" {
		set("clientSubId", c.subID)
	} else if _, ok := peer["clientSubId"]; ok {
		delete(peer, "clientSubId")
		changed = true
	}
	if c.comment != "" {
		set("clientComment", c.comment)
	} else if _, ok := peer["clientComment"]; ok {
		delete(peer, "clientComment")
		changed = true
	}

	if strings.TrimSpace(comment) == "" || comment == oldEmail {
		set("comment", c.email)
	}
	return changed
}

func detachWireGuardPeerClient(peer map[string]any) bool {
	changed := false
	if !jsonBool(peer[wireGuardPeerDetachedKey]) {
		peer[wireGuardPeerDetachedKey] = true
		changed = true
	}
	return changed
}

func wireGuardPeerHasClient(peer map[string]any) bool {
	if _, ok := jsonInt(peer["clientId"]); ok {
		return true
	}
	email, _ := peer["clientEmail"].(string)
	return strings.TrimSpace(email) != ""
}

func nextWireGuardPeerAllowedIP(peers []any) (string, error) {
	const fallback = "10.0.0.2/32"
	var maxIP uint32
	prefix := 32
	found := false

	for _, rawPeer := range peers {
		peer, ok := rawPeer.(map[string]any)
		if !ok {
			continue
		}
		rawAllowed, _ := peer["allowedIPs"].([]any)
		for _, rawIP := range rawAllowed {
			text, _ := rawIP.(string)
			addr, bits := parseWireGuardAllowedIPv4(text)
			if addr == nil {
				continue
			}
			value := uint32(addr[0])<<24 | uint32(addr[1])<<16 | uint32(addr[2])<<8 | uint32(addr[3])
			if !found || value > maxIP {
				maxIP = value
				prefix = bits
				found = true
			}
		}
	}

	if !found {
		return fallback, nil
	}
	if maxIP == ^uint32(0) {
		return "", fmt.Errorf("WireGuard address pool exhausted")
	}
	next := maxIP + 1
	return fmt.Sprintf("%d.%d.%d.%d/%d", byte(next>>24), byte(next>>16), byte(next>>8), byte(next), prefix), nil
}

func parseWireGuardAllowedIPv4(value string) (net.IP, int) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, 0
	}
	addr := value
	bits := 32
	if before, after, ok := strings.Cut(value, "/"); ok {
		addr = strings.TrimSpace(before)
		n, err := strconv.Atoi(strings.TrimSpace(after))
		if err != nil || n < 0 || n > 32 {
			return nil, 0
		}
		bits = n
	}
	ip := net.ParseIP(addr).To4()
	if ip == nil {
		return nil, 0
	}
	return ip, bits
}

func newWireGuardPeerForClient(peers []any, c wireGuardClientBinding) (map[string]any, error) {
	privateKey, publicKey, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		return nil, err
	}
	allowedIP, err := nextWireGuardPeerAllowedIP(peers)
	if err != nil {
		return nil, err
	}
	peer := map[string]any{
		"privateKey": privateKey,
		"publicKey":  publicKey,
		"allowedIPs": []any{allowedIP},
		"keepAlive":  0,
	}
	setWireGuardPeerClient(peer, c)
	return peer, nil
}

func (s *ClientService) syncWireGuardInboundPeerBindings(inboundSvc *InboundService, inboundId int) error {
	return s.syncWireGuardInboundPeerBindingsTx(nil, inboundSvc, inboundId)
}

func (s *ClientService) syncWireGuardInboundPeerBindingsTx(tx *gorm.DB, inboundSvc *InboundService, inboundId int) error {
	if tx == nil {
		tx = database.GetDB()
	}

	var inbound model.Inbound
	if err := tx.First(&inbound, inboundId).Error; err != nil {
		return err
	}
	if inbound.Protocol != model.WireGuard {
		return nil
	}

	clients, err := s.ListForInbound(tx, inboundId)
	if err != nil {
		return err
	}

	bindings := make([]wireGuardClientBinding, 0, len(clients))
	byID := make(map[int]wireGuardClientBinding, len(clients))
	byEmail := make(map[string]wireGuardClientBinding, len(clients))
	for i := range clients {
		email := strings.TrimSpace(clients[i].Email)
		if email == "" {
			continue
		}
		rec, rerr := s.GetRecordByEmail(tx, email)
		if rerr != nil {
			return rerr
		}
		b := wireGuardClientBinding{
			id:      rec.Id,
			email:   rec.Email,
			subID:   rec.SubID,
			comment: rec.Comment,
		}
		bindings = append(bindings, b)
		byID[b.id] = b
		byEmail[strings.ToLower(b.email)] = b
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return err
	}
	rawPeers, _ := settings["peers"].([]any)
	if rawPeers == nil {
		rawPeers = []any{}
	}

	used := make(map[int]struct{}, len(bindings))
	changed := false

	for _, rawPeer := range rawPeers {
		peer, ok := rawPeer.(map[string]any)
		if !ok {
			continue
		}

		var binding wireGuardClientBinding
		matched := false
		if id, ok := jsonInt(peer["clientId"]); ok {
			if b, exists := byID[id]; exists {
				if _, dup := used[b.id]; !dup {
					binding = b
					matched = true
				}
			}
		}
		if !matched {
			if email, _ := peer["clientEmail"].(string); strings.TrimSpace(email) != "" {
				if b, exists := byEmail[strings.ToLower(strings.TrimSpace(email))]; exists {
					if _, dup := used[b.id]; !dup {
						binding = b
						matched = true
					}
				}
			}
		}

		if matched {
			used[binding.id] = struct{}{}
			if setWireGuardPeerClient(peer, binding) {
				changed = true
			}
			continue
		}

		if wireGuardPeerHasClient(peer) {
			if detachWireGuardPeerClient(peer) {
				changed = true
			}
		}
	}

	next := 0
	for _, rawPeer := range rawPeers {
		peer, ok := rawPeer.(map[string]any)
		if !ok {
			continue
		}
		if wireGuardPeerHasClient(peer) {
			continue
		}
		for next < len(bindings) {
			b := bindings[next]
			next++
			if _, already := used[b.id]; already {
				continue
			}
			used[b.id] = struct{}{}
			if setWireGuardPeerClient(peer, b) {
				changed = true
			}
			break
		}
	}

	for _, b := range bindings {
		if _, already := used[b.id]; already {
			continue
		}
		peer, err := newWireGuardPeerForClient(rawPeers, b)
		if err != nil {
			return err
		}
		rawPeers = append(rawPeers, peer)
		used[b.id] = struct{}{}
		changed = true
	}

	if !changed {
		return nil
	}

	settings["peers"] = rawPeers
	newSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return tx.Model(&model.Inbound{}).Where("id = ?", inboundId).Update("settings", string(newSettings)).Error
}

func buildRuntimeWireGuardSettings(tx *gorm.DB, inbound *model.Inbound) (string, bool, error) {
	if inbound == nil || inbound.Protocol != model.WireGuard {
		return "", false, nil
	}
	if tx == nil {
		tx = database.GetDB()
	}

	settings := map[string]any{}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return "", false, err
	}
	rawPeers, ok := settings["peers"].([]any)
	if !ok {
		return "", false, nil
	}

	active, err := activeWireGuardClientMap(tx, inbound.Id)
	if err != nil {
		return "", false, err
	}

	finalPeers := make([]any, 0, len(rawPeers))
	for _, rawPeer := range rawPeers {
		peer, ok := rawPeer.(map[string]any)
		if !ok {
			continue
		}
		if jsonBool(peer[wireGuardPeerDetachedKey]) {
			continue
		}
		managed := wireGuardPeerHasClient(peer)
		if managed {
			if !wireGuardPeerClientActive(peer, active) {
				continue
			}
		}
		finalPeers = append(finalPeers, runtimeWireGuardPeer(peer))
	}

	settings["peers"] = finalPeers
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", false, err
	}
	return string(modifiedSettings), true, nil
}

type wireGuardRuntimeClient struct {
	id     int
	email  string
	active bool
}

func activeWireGuardClientMap(tx *gorm.DB, inboundId int) (map[int]wireGuardRuntimeClient, error) {
	var records []model.ClientRecord
	if err := tx.Model(&model.ClientRecord{}).
		Joins("JOIN client_inbounds ON client_inbounds.client_id = clients.id").
		Where("client_inbounds.inbound_id = ?", inboundId).
		Find(&records).Error; err != nil {
		return nil, err
	}

	emails := make([]string, 0, len(records))
	for _, rec := range records {
		if strings.TrimSpace(rec.Email) != "" {
			emails = append(emails, rec.Email)
		}
	}

	stats := []xray.ClientTraffic{}
	if len(emails) > 0 {
		if err := tx.Model(&xray.ClientTraffic{}).
			Where("email IN ?", emails).
			Select("email", "enable", "up", "down", "total", "expiry_time").
			Find(&stats).Error; err != nil {
			return nil, err
		}
	}
	statsByEmail := make(map[string]xray.ClientTraffic, len(stats))
	for _, st := range stats {
		statsByEmail[strings.ToLower(st.Email)] = st
	}

	now := time.Now().UnixMilli()
	active := make(map[int]wireGuardRuntimeClient, len(records))
	for _, rec := range records {
		ok := rec.Enable
		if rec.ExpiryTime > 0 && rec.ExpiryTime < now {
			ok = false
		}
		if rec.TotalGB > 0 {
			if st, exists := statsByEmail[strings.ToLower(rec.Email)]; exists && st.Up+st.Down >= rec.TotalGB {
				ok = false
			}
		}
		if st, exists := statsByEmail[strings.ToLower(rec.Email)]; exists {
			if !st.Enable {
				ok = false
			}
			if st.ExpiryTime > 0 && st.ExpiryTime < now {
				ok = false
			}
			if st.Total > 0 && st.Up+st.Down >= st.Total {
				ok = false
			}
		}
		active[rec.Id] = wireGuardRuntimeClient{id: rec.Id, email: rec.Email, active: ok}
	}
	return active, nil
}

func wireGuardPeerClientActive(peer map[string]any, active map[int]wireGuardRuntimeClient) bool {
	if id, ok := jsonInt(peer["clientId"]); ok {
		client, exists := active[id]
		return exists && client.active
	}
	email, _ := peer["clientEmail"].(string)
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false
	}
	for _, client := range active {
		if strings.ToLower(client.email) == email {
			return client.active
		}
	}
	return false
}

func runtimeWireGuardPeer(peer map[string]any) map[string]any {
	out := make(map[string]any, len(peer))
	for key, value := range peer {
		switch key {
		case "privateKey", "comment", "clientId", "clientEmail", "clientSubId", "clientComment", wireGuardPeerDetachedKey:
			continue
		default:
			out[key] = value
		}
	}
	return out
}
