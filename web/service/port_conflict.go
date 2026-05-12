package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/util/common"
)

// transportBits is a bitmask of L4 transports an inbound listens on.
// 0.0.0.0:443/tcp and 0.0.0.0:443/udp are independent sockets in linux,
// so the conflict check needs more than just the port number.
type transportBits uint8

const (
	transportTCP transportBits = 1 << iota
	transportUDP
)

// conflicts is true when the two masks share any L4 transport.
func (b transportBits) conflicts(o transportBits) bool { return b&o != 0 }

// inboundTransports returns the L4 transports the given inbound listens on.
// always returns at least one bit (falls back to tcp on parse errors), so
// the validator never gets looser than the old port-only check.
//
// the rules:
//   - hysteria, hysteria2, wireguard: udp regardless of streamSettings
//   - streamSettings.network=kcp: udp
//   - shadowsocks: whatever settings.network says ("tcp" / "udp" / "tcp,udp")
//   - mixed (socks/http combo): tcp + udp when settings.udp is true
//   - everything else: tcp
func inboundTransports(protocol model.Protocol, streamSettings, settings string) transportBits {
	// protocols that ignore streamSettings entirely.
	switch protocol {
	case model.Hysteria, model.Hysteria2, model.WireGuard:
		return transportUDP
	}

	var bits transportBits

	// peek at streamSettings.network to spot udp transports like kcp.
	// parse errors are non-fatal: missing or weird streamSettings just
	// keeps the default tcp bit below.
	network := ""
	if streamSettings != "" {
		var ss map[string]any
		if json.Unmarshal([]byte(streamSettings), &ss) == nil {
			if n, _ := ss["network"].(string); n != "" {
				network = n
			}
		}
	}
	if network == "kcp" {
		bits |= transportUDP
	} else {
		bits |= transportTCP
	}

	// some protocols also listen on udp on the same port via their own
	// settings json. parse and merge.
	if settings != "" {
		var st map[string]any
		if json.Unmarshal([]byte(settings), &st) == nil {
			switch protocol {
			case model.Shadowsocks:
				// shadowsocks settings.network controls both tcp and udp,
				// independently of streamSettings. the field takes "tcp",
				// "udp", or "tcp,udp". if it's set, it wins outright.
				if n, ok := st["network"].(string); ok && n != "" {
					bits = 0
					for _, part := range strings.Split(n, ",") {
						switch strings.TrimSpace(part) {
						case "tcp":
							bits |= transportTCP
						case "udp":
							bits |= transportUDP
						}
					}
				}
			case model.Mixed:
				// socks/http "mixed" inbound: settings.udp=true means it
				// also relays udp on the same port (socks5 udp associate).
				if udpOn, _ := st["udp"].(bool); udpOn {
					bits |= transportUDP
				}
			}
		}
	}

	// safety net: never return zero, even if every parse failed.
	if bits == 0 {
		bits = transportTCP
	}
	return bits
}

// listenOverlaps reports whether two listen addresses can collide on the
// same port. preserves the rule from the original checkPortExist:
// any-address (empty / 0.0.0.0 / :: / ::0) overlaps with everything,
// otherwise only identical specific addresses overlap.
func listenOverlaps(a, b string) bool {
	if isAnyListen(a) || isAnyListen(b) {
		return true
	}
	return a == b
}

func isAnyListen(s string) bool {
	return s == "" || s == "0.0.0.0" || s == "::" || s == "::0"
}

// checkPortConflict reports whether adding/updating an inbound on
// (listen, port) would clash with an existing inbound. unlike the old
// port-only check, this one understands that tcp/443 and udp/443 are
// independent sockets in linux and may coexist on the same address.
//
// node scope: inbounds with different NodeID run on different physical
// machines (local panel xray vs a remote node, or two remote nodes),
// so their sockets can't collide. only candidates with the same NodeID
// participate in the listen/transport overlap check.
//
// the listen-overlap rule (specific addr conflicts with any-addr on the
// same port, both directions) is preserved from the previous check.
func (s *InboundService) checkPortConflict(inbound *model.Inbound, ignoreId int) (bool, error) {
	db := database.GetDB()

	var candidates []*model.Inbound
	q := db.Model(model.Inbound{}).Where("port = ?", inbound.Port)
	if ignoreId > 0 {
		q = q.Where("id != ?", ignoreId)
	}
	if err := q.Find(&candidates).Error; err != nil {
		return false, err
	}

	newBits := inboundTransports(inbound.Protocol, inbound.StreamSettings, inbound.Settings)
	for _, c := range candidates {
		if !sameNode(c.NodeID, inbound.NodeID) {
			continue
		}
		if !listenOverlaps(c.Listen, inbound.Listen) {
			continue
		}
		if inboundTransports(c.Protocol, c.StreamSettings, c.Settings).conflicts(newBits) {
			return true, nil
		}
	}
	return false, nil
}

// sameNode reports whether two NodeID pointers refer to the same xray
// process. nil/nil means both inbounds run on the local panel; non-nil
// with equal value means they share the same remote node. any mix
// (local vs remote, remote-A vs remote-B) is "different node" and
// can't produce a real socket collision.
func sameNode(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// baseInboundTag is the historical "inbound-<port>" / "inbound-<listen>:<port>"
// shape. kept exactly so existing routing rules that reference these tags
// keep working after the upgrade.
func baseInboundTag(listen string, port int) string {
	if isAnyListen(listen) {
		return fmt.Sprintf("inbound-%v", port)
	}
	return fmt.Sprintf("inbound-%v:%v", listen, port)
}

// transportTagSuffix turns a transport mask into a short, stable string
// for tag disambiguation. only used when the base "inbound-<port>" is
// already taken on a coexisting transport (e.g. tcp inbound already lives
// on 443 and we're now adding a udp one).
func transportTagSuffix(b transportBits) string {
	switch b {
	case transportTCP:
		return "tcp"
	case transportUDP:
		return "udp"
	case transportTCP | transportUDP:
		return "mixed"
	}
	return "any"
}

// generateInboundTag picks a tag for the inbound that doesn't collide with
// any existing row. for the common single-inbound-per-port case the tag
// stays exactly as before ("inbound-443"), so user routing rules don't
// silently change shape on upgrade. only when a same-port neighbour
// already owns the base tag (now possible because tcp/443 and udp/443 can
// coexist after the transport-aware port check) does this append a
// transport suffix like "inbound-443-udp".
//
// ignoreId is the inbound's own id during update so it doesn't see itself
// as a collision; pass 0 on add.
func (s *InboundService) generateInboundTag(inbound *model.Inbound, ignoreId int) (string, error) {
	base := baseInboundTag(inbound.Listen, inbound.Port)
	exists, err := s.tagExists(base, ignoreId)
	if err != nil {
		return "", err
	}
	if !exists {
		return base, nil
	}

	suffix := transportTagSuffix(inboundTransports(inbound.Protocol, inbound.StreamSettings, inbound.Settings))
	candidate := base + "-" + suffix
	exists, err = s.tagExists(candidate, ignoreId)
	if err != nil {
		return "", err
	}
	if !exists {
		return candidate, nil
	}

	// the transport-aware port check should have already blocked this
	// path, but guard anyway so a unique-constraint failure doesn't reach
	// the user as an opaque sqlite error.
	for i := 2; i < 100; i++ {
		c := fmt.Sprintf("%s-%d", candidate, i)
		exists, err = s.tagExists(c, ignoreId)
		if err != nil {
			return "", err
		}
		if !exists {
			return c, nil
		}
	}
	return "", common.NewError("could not pick a unique inbound tag for port:", inbound.Port)
}

// resolveInboundTag chooses a tag for an Add or Update. when the caller
// supplied a non-empty Tag (e.g. the central panel pushed its picked
// tag to a node during a multi-node sync) and that tag is free in the
// local DB, it's used verbatim so the two panels stay in agreement —
// otherwise the node would regenerate (often back to bare
// "inbound-<port>") and the eventual traffic sync-back would try to
// INSERT a row whose tag already exists, hitting the UNIQUE constraint
// on inbounds.tag and rolling the node-side row right back out.
// when Tag is empty (the common UI path) or collides, fall back to the
// transport-aware generateInboundTag.
//
// ignoreId mirrors generateInboundTag: pass 0 on add, the inbound's
// own id on update so a row doesn't see its own current tag as taken.
func (s *InboundService) resolveInboundTag(inbound *model.Inbound, ignoreId int) (string, error) {
	if inbound.Tag != "" {
		taken, err := s.tagExists(inbound.Tag, ignoreId)
		if err != nil {
			return "", err
		}
		if !taken {
			return inbound.Tag, nil
		}
	}
	return s.generateInboundTag(inbound, ignoreId)
}

func (s *InboundService) tagExists(tag string, ignoreId int) (bool, error) {
	db := database.GetDB()
	q := db.Model(model.Inbound{}).Where("tag = ?", tag)
	if ignoreId > 0 {
		q = q.Where("id != ?", ignoreId)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
