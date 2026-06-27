package service

import (
	"encoding/json"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

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

func setWireGuardPeerClient(peer map[string]any, c wireGuardClientBinding) bool {
	changed := false
	set := func(key string, value any) {
		if peer[key] != value {
			peer[key] = value
			changed = true
		}
	}

	oldEmail, _ := peer["clientEmail"].(string)
	comment, _ := peer["comment"].(string)

	set("clientId", c.id)
	set("clientEmail", c.email)
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

func clearWireGuardPeerClient(peer map[string]any) bool {
	changed := false
	oldEmail, _ := peer["clientEmail"].(string)
	comment, _ := peer["comment"].(string)
	for _, key := range []string{"clientId", "clientEmail", "clientSubId", "clientComment"} {
		if _, ok := peer[key]; ok {
			delete(peer, key)
			changed = true
		}
	}
	if oldEmail != "" && comment == oldEmail {
		delete(peer, "comment")
		changed = true
	}
	return changed
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
	rawPeers, ok := settings["peers"].([]any)
	if !ok || len(rawPeers) == 0 {
		return nil
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

		if _, hadID := peer["clientId"]; hadID {
			if clearWireGuardPeerClient(peer) {
				changed = true
			}
			continue
		}
		if _, hadEmail := peer["clientEmail"]; hadEmail {
			if clearWireGuardPeerClient(peer) {
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
		if _, ok := jsonInt(peer["clientId"]); ok {
			continue
		}
		if email, _ := peer["clientEmail"].(string); strings.TrimSpace(email) != "" {
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
