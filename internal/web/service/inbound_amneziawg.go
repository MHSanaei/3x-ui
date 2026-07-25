package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// DesiredAmneziaWGInstances derives the AmneziaWG interfaces this panel
// should be running: one instance per enabled local AmneziaWG inbound,
// serving only the peers of clients that are both enabled in the inbound
// settings and not depletion-disabled in client_traffics. That is the same
// effective peer set buildRuntimeInboundForAPI pushes on interactive edits,
// so the reconcile job and the push path agree on one fingerprint — see
// DesiredMtprotoInstances, which this mirrors exactly.
func (s *InboundService) DesiredAmneziaWGInstances() ([]amneziawg.Instance, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).
		Where("protocol = ? AND enable = ? AND node_id IS NULL", model.AmneziaWG, true).
		Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	if len(inbounds) == 0 {
		return nil, nil
	}

	ids := make([]int, 0, len(inbounds))
	for _, ib := range inbounds {
		ids = append(ids, ib.Id)
	}
	var disabledRows []xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).
		Where("inbound_id IN ? AND enable = ?", ids, false).
		Select("inbound_id", "email").
		Find(&disabledRows).Error
	if err != nil {
		return nil, err
	}
	disabled := make(map[int]map[string]struct{}, len(disabledRows))
	for _, row := range disabledRows {
		if disabled[row.InboundId] == nil {
			disabled[row.InboundId] = map[string]struct{}{}
		}
		disabled[row.InboundId][row.Email] = struct{}{}
	}

	instances := make([]amneziawg.Instance, 0, len(inbounds))
	for _, ib := range inbounds {
		inst, ok := amneziawg.InstanceFromInbound(ib)
		if !ok {
			continue
		}
		if off := disabled[ib.Id]; len(off) > 0 {
			kept := make([]amneziawg.Peer, 0, len(inst.Peers))
			for _, p := range inst.Peers {
				if _, skip := off[p.Email]; !skip {
					kept = append(kept, p)
				}
			}
			inst.Peers = kept
		}
		if len(inst.Peers) == 0 {
			continue
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

// applyLocalAmneziaWG pushes a single local AmneziaWG inbound's current peer
// set to its interface right after a client edit commits, so an add,
// removal, re-key or enable-toggle takes effect immediately instead of
// waiting up to 10s for the reconcile job. It re-reads the inbound so it sees
// the committed settings, filters depleted clients exactly like the
// reconcile job, and is a no-op for node-owned or non-AmneziaWG inbounds.
// Failures are logged and swallowed: the reconcile job is the backstop.
// Mirrors applyLocalMtproto.
func (s *InboundService) applyLocalAmneziaWG(inboundId int) {
	inbound, err := s.GetInbound(inboundId)
	if err != nil || inbound == nil || inbound.Protocol != model.AmneziaWG || inbound.NodeID != nil {
		return
	}
	rt, err := s.runtimeFor(inbound)
	if err != nil {
		return
	}
	payload := inbound
	if inbound.Enable {
		if built, bErr := s.buildRuntimeInboundForAPI(database.GetDB(), inbound); bErr == nil {
			payload = built
		}
	}
	if err := rt.UpdateInbound(context.Background(), inbound, payload); err != nil {
		logger.Debug("amneziawg: immediate apply failed for inbound", inboundId, ":", err)
	}
}

// defaultAmneziaWGServer builds a fresh server block: a random AmneziaWG 2.0
// obfuscation set, the default tunnel subnet/DNS, and a freshly generated
// keypair.
func defaultAmneziaWGServer() (*amneziawg.ServerSettings, error) {
	obf := amneziawg.GenerateObfuscation20("default")
	server := &amneziawg.ServerSettings{
		SubnetIP:     "10.8.1.0",
		SubnetCIDR:   24,
		PrimaryDNS:   "8.8.8.8",
		SecondaryDNS: "8.8.4.4",
		Jc:           obf.Jc,
		Jmin:         obf.Jmin,
		Jmax:         obf.Jmax,
		S1:           obf.S1,
		S2:           obf.S2,
		S3:           obf.S3,
		S4:           obf.S4,
		H1:           obf.H1,
		H2:           obf.H2,
		H3:           obf.H3,
		H4:           obf.H4,
		I1:           obf.I1,
	}
	if err := fillAmneziaWGServerKeys(server); err != nil {
		return nil, err
	}
	return server, nil
}

// fillAmneziaWGServerKeys generates a real WireGuard-compatible keypair for
// the server block when one is missing.
func fillAmneziaWGServerKeys(server *amneziawg.ServerSettings) error {
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		return fmt.Errorf("amneziawg: generate server keypair: %w", err)
	}
	server.PrivateKey = priv
	server.PublicKey = pub
	return nil
}

// normalizeAmneziaWGSettings ensures an AmneziaWG inbound's settings have a
// valid server block, generating one (fresh obfuscation params + keypair) on
// first save and validating a manually-edited one so a bad entry can't bring
// the interface down on the next apply. A no-op for every other protocol.
func (s *InboundService) normalizeAmneziaWGSettings(inbound *model.Inbound) error {
	if inbound.Protocol != model.AmneziaWG {
		return nil
	}

	trimmed := strings.TrimSpace(inbound.Settings)
	if trimmed == "" || trimmed == "null" || trimmed == "{}" {
		server, err := defaultAmneziaWGServer()
		if err != nil {
			return err
		}
		settings := amneziawg.InboundSettings{Server: server, Clients: []model.Client{}}
		bs, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return err
		}
		inbound.Settings = string(bs)
		return nil
	}

	var parsed amneziawg.InboundSettings
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil {
		return fmt.Errorf("amneziawg: invalid settings: %w", err)
	}
	if parsed.Server == nil {
		server, err := defaultAmneziaWGServer()
		if err != nil {
			return err
		}
		parsed.Server = server
	} else if parsed.Server.PrivateKey == "" {
		if err := fillAmneziaWGServerKeys(parsed.Server); err != nil {
			return err
		}
	}
	if err := amneziawg.ValidateObfuscation(parsed.Server.Obfuscation()); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}
	if err := amneziawg.ValidateIPv6Subnet(parsed.Server.IPv6Enabled, parsed.Server.IPv6Subnet); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}

	bs, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return err
	}
	inbound.Settings = string(bs)
	return nil
}
