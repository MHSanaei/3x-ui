package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

var errAWGPingDenied = errors.New("AWG ping denied")

func saveAWGPing(tx *gorm.DB, email string, policy *model.AWGPingPolicy) error {
	if policy == nil {
		return nil
	}
	policy = policy.Clone()
	if err := policy.Normalize(); err != nil {
		return err
	}
	raw, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	return tx.Model(&model.ClientRecord{}).Where("email = ? AND (awg_ping IS NULL OR awg_ping <> ?)", email, string(raw)).Updates(map[string]any{"awg_ping": string(raw), "awg_ping_revision": gorm.Expr("awg_ping_revision + 1")}).Error
}

func syncAWGPing(tx *gorm.DB, clients []model.Client) error {
	for _, c := range clients {
		if err := saveAWGPing(tx, c.Email, c.AWGPing); err != nil {
			return err
		}
	}
	return nil
}

func (s *InboundService) validateAWGPingTarget(inbound *model.Inbound, policy *model.AWGPingPolicy, previousEmail string) error {
	if policy == nil || !policy.Enabled {
		return nil
	}
	if inbound.Protocol != model.AmneziaWG {
		return nil
	}
	var settings amneziawg.InboundSettings
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return err
	}
	if settings.Server == nil || settings.Server.KernelMode {
		return errors.New("AWG ping policy requires the embedded datapath")
	}
	if inbound.NodeID != nil {
		// An existing, unchanged grant was already negotiated for this inbound.
		// Metadata edits must remain durable while its node is unreachable.
		candidate := policy.Clone()
		if err := candidate.Normalize(); err != nil {
			return err
		}
		for _, client := range settings.Clients {
			if !strings.EqualFold(client.Email, previousEmail) || client.AWGPing == nil {
				continue
			}
			previous := client.AWGPing.Clone()
			if previous.Normalize() == nil && previous.Enabled && slices.Equal(previous.Allow, candidate.Allow) && slices.Equal(previous.Deny, candidate.Deny) {
				return nil
			}
		}
		rt, err := s.runtimeFor(inbound)
		if err != nil {
			return err
		}
		remote, ok := rt.(*runtime.Remote)
		if !ok {
			return errors.New("AWG ping node capability is unknown")
		}
		return remote.RequireAWGPing(context.Background())
	}
	return nil
}

func echoHasBudget(total, up, down, extra int64) bool {
	if total < 0 || up < 0 || down < 0 || extra < 0 {
		return false
	}
	if total == 0 {
		return true
	}
	return up <= total && down <= total-up && extra <= total-up-down
}

// RecordAWGPing authorizes and commits one admitted IP packet using AddTraffic's native counters.
// Lifecycle jobs remain separate: recursively applying runtime changes on its packet goroutine can deadlock.
func (s *InboundService) RecordAWGPing(inboundID int, peer amneziawg.Peer, src, dst netip.Addr, up, down int64, expected string) (string, error) {
	if !src.Is4() || !src.IsGlobalUnicast() || src.IsLoopback() || src.IsLinkLocalUnicast() || src.As4()[0] == 0 || src.As4()[0] >= 224 || (up == 0) == (down == 0) || up < 0 || down < 0 || up+down > 1200 {
		return "", errAWGPingDenied
	}
	identity := ""
	err := submitTrafficWrite(func() error {
		return database.GetDB().Transaction(func(tx *gorm.DB) error {
			var ib model.Inbound
			if err := tx.Where("id = ? AND protocol = ? AND node_id IS NULL", inboundID, model.AmneziaWG).Take(&ib).Error; err != nil {
				return err
			}
			now := time.Now().UnixMilli()
			if !ib.Enable || (ib.ExpiryTime > 0 && now >= ib.ExpiryTime) || !echoHasBudget(ib.Total, ib.Up, ib.Down, up+down) {
				return errAWGPingDenied
			}
			var settings amneziawg.InboundSettings
			if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
				return err
			}
			if settings.Server == nil || settings.Server.KernelMode {
				return errAWGPingDenied
			}
			bound := false
			for _, c := range settings.Clients {
				for _, a := range c.AllowedIPs {
					p, e := netip.ParsePrefix(a)
					if e != nil {
						continue
					}
					if p.Contains(dst) {
						return errAWGPingDenied
					}
					if c.Email == peer.Email && c.PublicKey == peer.PublicKey && c.Enable && p.Contains(src) {
						bound = true
					}
				}
			}
			if !bound {
				return errAWGPingDenied
			}
			var otherInbounds []model.Inbound
			if err := tx.Select("settings").Where("protocol = ? AND node_id IS NULL AND id <> ?", model.AmneziaWG, inboundID).Find(&otherInbounds).Error; err != nil {
				return err
			}
			for _, other := range otherInbounds {
				var config amneziawg.InboundSettings
				if json.Unmarshal([]byte(other.Settings), &config) != nil {
					return errAWGPingDenied
				}
				for _, client := range config.Clients {
					for _, ip := range client.AllowedIPs {
						if p, e := netip.ParsePrefix(ip); e == nil && p.Contains(dst) {
							return errAWGPingDenied
						}
					}
				}
			}

			var record model.ClientRecord
			if err := tx.Joins("JOIN client_inbounds ci ON ci.client_id = clients.id").
				Where("ci.inbound_id = ? AND clients.email = ?", inboundID, peer.Email).Take(&record).Error; err != nil {
				return err
			}
			if !record.Enable || record.PublicKey != peer.PublicKey || (record.ExpiryTime > 0 && now >= record.ExpiryTime) || !record.AWGPing.Allows(dst) {
				return errAWGPingDenied
			}
			raw, err := json.Marshal(record.AWGPing)
			if err != nil {
				return err
			}
			identity = fmt.Sprintf("%d:%d:%s", record.Id, record.AWGPingRevision, raw)
			if expected != "" && expected != identity {
				return errAWGPingDenied
			}
			var traffic xray.ClientTraffic
			if err := tx.Where("email = ?", peer.Email).Take(&traffic).Error; err != nil {
				return err
			}
			if !traffic.Enable || (traffic.ExpiryTime > 0 && now >= traffic.ExpiryTime) ||
				!echoHasBudget(traffic.Total, traffic.Up, traffic.Down, up+down) || !echoHasBudget(record.TotalGB, traffic.Up, traffic.Down, up+down) {
				return errAWGPingDenied
			}
			var global []model.ClientGlobalTraffic
			if err := tx.Where("email = ? AND updated_at >= ?", peer.Email, globalTrafficFreshSince(time.Now())).Find(&global).Error; err != nil {
				return err
			}
			for _, usage := range global {
				if !echoHasBudget(traffic.Total, usage.Up, usage.Down, up+down) || !echoHasBudget(record.TotalGB, usage.Up, usage.Down, up+down) {
					return errAWGPingDenied
				}
			}
			return s.addTrafficCounters(tx, []*xray.Traffic{{Tag: ib.Tag, IsInbound: true, Up: up, Down: down}}, []*xray.ClientTraffic{{InboundId: ib.Id, Email: peer.Email, Up: up, Down: down}})
		})
	})
	return identity, err
}

func (s *ClientService) syncNodeInboundWithPing(tx *gorm.DB, inboundID int, clients []model.Client) error {
	emails := make([]string, 0, len(clients))
	for _, c := range clients {
		if c.AWGPing != nil {
			emails = append(emails, c.Email)
		}
	}
	existing := map[string]bool{}
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var stored []string
		if err := tx.Model(&model.ClientRecord{}).Where("email IN ?", batch).Pluck("email", &stored).Error; err != nil {
			return err
		}
		for _, email := range stored {
			existing[email] = true
		}
	}
	var fresh []model.Client
	for _, c := range clients {
		if c.AWGPing != nil && !existing[c.Email] {
			fresh = append(fresh, c)
		}
	}
	if err := s.SyncInbound(tx, inboundID, clients); err != nil {
		return err
	}
	return syncAWGPing(tx, fresh)
}

func validateStoredAWGPingForKernel(db *gorm.DB, inbound *model.Inbound) error {
	if inbound.Protocol != model.AmneziaWG {
		return nil
	}
	var settings amneziawg.InboundSettings
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return err
	}
	if settings.Server == nil || !settings.Server.KernelMode {
		return nil
	}
	var clients []model.ClientRecord
	if err := db.Joins("JOIN client_inbounds ci ON ci.client_id = clients.id").Where("ci.inbound_id = ?", inbound.Id).Find(&clients).Error; err != nil {
		return err
	}
	for _, client := range clients {
		if client.AWGPing != nil && client.AWGPing.Enabled {
			return errors.New("AWG ping policy requires the embedded datapath")
		}
	}
	return nil
}
