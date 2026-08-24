package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
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
// effective peer set buildInboundForLocalRuntime pushes on interactive edits,
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
		if built, bErr := s.buildInboundForLocalRuntime(database.GetDB(), inbound); bErr == nil {
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
	obf := amneziawg.GenerateObfuscation31("default")
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
		AwgVersion:   amneziawg.AwgVersion2,
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
	if err := amneziawg.ValidateSubnetIPv4(parsed.Server.SubnetIP, parsed.Server.SubnetCIDR); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}
	if err := amneziawg.ValidateInterfaceName(parsed.Server.ExternalInterface); err != nil {
		return fmt.Errorf("amneziawg: externalInterface: %w", err)
	}
	if err := amneziawg.ValidateInterfaceName(parsed.Server.IPv6ExternalInterface); err != nil {
		return fmt.Errorf("amneziawg: ipv6ExternalInterface: %w", err)
	}
	if err := amneziawg.ValidateConfigValue("privateKey", parsed.Server.PrivateKey); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}
	if err := amneziawg.ValidateConfigValue("publicKey", parsed.Server.PublicKey); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}
	if err := amneziawg.ValidateConfigValue("i1", parsed.Server.I1); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}
	if err := amneziawg.ValidateConfigValue("i2", parsed.Server.I2); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}
	if err := amneziawg.ValidateConfigValue("i3", parsed.Server.I3); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}
	if err := amneziawg.ValidateConfigValue("i4", parsed.Server.I4); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}
	if err := amneziawg.ValidateConfigValue("i5", parsed.Server.I5); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}
	if err := amneziawg.ValidateConfigValue("headerProtectionKey", parsed.Server.HeaderProtectionKey); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}
	if err := amneziawg.ValidateHeaderProtection(parsed.Server.HeaderProtectionKey, parsed.Server.Obfuscation()); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}
	if err := amneziawg.ValidateContentPaddingAddition(parsed.Server.ContentPaddingAddition); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}
	for _, tf := range [...]struct {
		field string
		value string
	}{
		{"rekeyAfterTime", parsed.Server.RekeyAfterTime},
		{"rekeyTimeout", parsed.Server.RekeyTimeout},
		{"rejectAfterTime", parsed.Server.RejectAfterTime},
		{"keepaliveTimeout", parsed.Server.KeepaliveTimeout},
		{"maxHandshakeAttempts", parsed.Server.MaxHandshakeAttempts},
	} {
		if err := amneziawg.ValidateConfigValue(tf.field, tf.value); err != nil {
			return fmt.Errorf("amneziawg: %w", err)
		}
		if err := amneziawg.ValidateAwgTimerValue(tf.field, tf.value); err != nil {
			return fmt.Errorf("amneziawg: %w", err)
		}
	}
	// EffectiveAwgVersion first, so a record saved before AwgVersion existed
	// (already has a working AWG3-only field with an empty AwgVersion) is
	// treated as already-opted-into-3 and persisted that way from here on,
	// instead of ValidateAwgVersion rejecting an existing, working
	// configuration the next time it's saved unrelated to this field.
	awg3Fields := []string{
		parsed.Server.HeaderProtectionKey, parsed.Server.ContentPaddingAddition,
		parsed.Server.RekeyAfterTime, parsed.Server.RekeyTimeout,
		parsed.Server.RejectAfterTime, parsed.Server.KeepaliveTimeout,
		parsed.Server.MaxHandshakeAttempts,
	}
	parsed.Server.AwgVersion = amneziawg.EffectiveAwgVersion(parsed.Server.AwgVersion, awg3Fields...)
	if err := amneziawg.ValidateAwgVersion(parsed.Server.AwgVersion, awg3Fields...); err != nil {
		return fmt.Errorf("amneziawg: %w", err)
	}

	portCtx, err := s.loadPortConflictContext()
	if err != nil {
		return err
	}
	for _, c := range parsed.Clients {
		if hit := s.checkForwardedPortsConflict(portCtx, c.ForwardedPorts); hit != "" {
			return fmt.Errorf("amneziawg: client %q forwardedPorts collides with %s", c.Email, hit)
		}
		if err := amneziawg.ValidateConfigValue("email", c.Email); err != nil {
			return fmt.Errorf("amneziawg: %w", err)
		}
		if err := amneziawg.ValidateConfigValue("publicKey", c.PublicKey); err != nil {
			return fmt.Errorf("amneziawg: client %q: %w", c.Email, err)
		}
		if err := amneziawg.ValidateConfigValue("preSharedKey", c.PreSharedKey); err != nil {
			return fmt.Errorf("amneziawg: client %q: %w", c.Email, err)
		}
	}

	bs, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return err
	}
	inbound.Settings = string(bs)
	return nil
}

// portConflictContext caches the state checkForwardedPortsConflict needs —
// the panel's own port and this host's enabled inbound ports — so validating
// N clients in one save (normalizeAmneziaWGSettings, or a bulk client add)
// costs one query total instead of N. Load it once with
// loadPortConflictContext and pass it to every checkForwardedPortsConflict
// call in that batch.
type portConflictContext struct {
	webPort  int
	inbounds []*model.Inbound
}

// loadPortConflictContext loads the panel's own port and every enabled
// inbound hosted on THIS panel (node_id IS NULL) — an inbound hosted on a
// different node listens on that node's own host, never this one, so it can
// never collide with a port-forward listener this process opens.
func (s *InboundService) loadPortConflictContext() (portConflictContext, error) {
	var ctx portConflictContext
	if webPort, err := (&SettingService{}).GetPort(); err == nil {
		ctx.webPort = webPort
	}
	err := database.GetDB().Model(model.Inbound{}).
		Where("enable = ? AND node_id IS NULL", true).
		Find(&ctx.inbounds).Error
	return ctx, err
}

// checkForwardedPortsConflict reports whether a client's ForwardedPorts spec
// exceeds the cap, covers the panel's own web port, one of this host's own
// enabled inbound listen ports, or an AmneziaWG inbound's own phantom SOCKS5
// relay port (SOCKSPortForInbound -- never a real inbounds row, so the loop
// below can't see it any other way). A collision on the SOCKS5 port would
// let a port-forward listener race Xray's own relay for the bind and, if it
// wins, take down that inbound's entire relay rather than just one forward.
// Returns a human-readable description of the first collision found, or ""
// when there is none.
func (s *InboundService) checkForwardedPortsConflict(ctx portConflictContext, forwardedPorts string) string {
	if forwardedPorts == "" {
		return ""
	}
	if amneziawg.ExceedsForwardedPortsCap(forwardedPorts) {
		return fmt.Sprintf("more than %d forwarded ports", amneziawg.MaxForwardedPorts)
	}
	if ctx.webPort > 0 && amneziawg.ForwardedPortsInclude(forwardedPorts, ctx.webPort) {
		return fmt.Sprintf("the panel's own port (%d)", ctx.webPort)
	}
	for _, ib := range ctx.inbounds {
		if amneziawg.ForwardedPortsInclude(forwardedPorts, ib.Port) {
			name := ib.Remark
			if name == "" {
				name = ib.Tag
			}
			return fmt.Sprintf("inbound '%s' (#%d, port %d)", name, ib.Id, ib.Port)
		}
		if ib.Protocol != model.AmneziaWG {
			continue
		}
		socksPort := amneziawgnet.SOCKSPortForInbound(ib.Id)
		if amneziawg.ForwardedPortsInclude(forwardedPorts, socksPort) {
			name := ib.Remark
			if name == "" {
				name = ib.Tag
			}
			return fmt.Sprintf("inbound '%s' (#%d)'s own SOCKS5 relay port (%d)", name, ib.Id, socksPort)
		}
	}
	return ""
}

// GetAmneziaWGDiagnostics returns a live diagnostics snapshot for inbound
// id: interface up/down, listen port, and per-client handshake/traffic
// state, read entirely from data amneziawgnet.Manager already tracks --
// gathering it can never itself change anything. Returns an error only
// when id doesn't name an AmneziaWG inbound at all; an inbound that simply
// isn't running right now (disabled, no enabled clients, or reconcile
// hasn't caught up yet) comes back as amneziawgnet.Diagnostics{}
// (Running=false), not an error, since that's a normal state an admin
// might specifically be checking for.
func (s *InboundService) GetAmneziaWGDiagnostics(id int) (amneziawgnet.Diagnostics, error) {
	inbound, err := s.GetInbound(id)
	if err != nil {
		return amneziawgnet.Diagnostics{}, err
	}
	if inbound.Protocol != model.AmneziaWG {
		return amneziawgnet.Diagnostics{}, fmt.Errorf("inbound %d is not an AmneziaWG inbound", id)
	}
	inst, ok := amneziawg.InstanceFromInbound(inbound)
	if !ok {
		return amneziawgnet.Diagnostics{}, nil
	}
	return amneziawgnet.Diagnose(inst.Id, inst.Peers), nil
}
