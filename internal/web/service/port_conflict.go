package service

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"gorm.io/gorm"
)

type transportBits uint8

const (
	transportTCP transportBits = 1 << iota
	transportUDP
)

func inboundTransports(protocol model.Protocol, streamSettings, settings string) transportBits {
	// protocols that ignore streamSettings entirely.
	switch protocol {
	case model.Hysteria, model.WireGuard, model.AmneziaWG, model.TUIC:
		return transportUDP
	case model.MTProto:
		return transportTCP
	}

	var bits transportBits

	// peek at streamSettings.network to spot udp-based transports.
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
	switch network {
	case "kcp", "quic":
		bits |= transportUDP
	default:
		bits |= transportTCP
	}

	// a few protocols carry their L4 choice in settings instead of (or in
	// addition to) streamSettings: SS / Tunnel via a CSV field that wins
	// outright, Mixed via an additive udp boolean.
	if settings != "" {
		var st map[string]any
		if json.Unmarshal([]byte(settings), &st) == nil {
			switch protocol {
			case model.Shadowsocks, model.Tunnel:
				key := "network"
				if protocol == model.Tunnel {
					key = "allowedNetwork"
				}
				if n, ok := st[key].(string); ok && n != "" {
					bits = 0
					for part := range strings.SplitSeq(n, ",") {
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

func listenOverlaps(a, b string) bool {
	if a == b {
		return true
	}
	fa, wa, oka := listenFamilyOf(a)
	fb, wb, okb := listenFamilyOf(b)
	if !oka || !okb || fa&fb == 0 {
		return false
	}
	// A wildcard reserves every address in its own family, but an IPv6
	// wildcard is not an IPv4 wildcard. Linux commonly runs with
	// IPV6_V6ONLY enabled, which permits ::443 and 0.0.0.0:443 together.
	return wa || wb
}

// listenFamily is deliberately separate from net.IP's representation: the
// conflict guard must preserve the kernel's IPv4/IPv6 socket boundary.
type listenFamily uint8

const (
	listenFamilyIPv4 listenFamily = 1 << iota
	listenFamilyIPv6
)

func listenFamilyOf(s string) (listenFamily, bool, bool) {
	switch s {
	case "":
		// An omitted listen lets the core choose all families; retain the
		// conservative legacy behavior for this ambiguous form.
		return listenFamilyIPv4 | listenFamilyIPv6, true, true
	case "0.0.0.0":
		return listenFamilyIPv4, true, true
	case "::", "::0":
		return listenFamilyIPv6, true, true
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return 0, false, false
	}
	if ip.To4() != nil {
		return listenFamilyIPv4, false, true
	}
	return listenFamilyIPv6, false, true
}

func isAnyListen(s string) bool {
	return s == "" || s == "0.0.0.0" || s == "::" || s == "::0"
}

type portConflictDetail struct {
	InboundID int
	Remark    string
	Tag       string
	Listen    string
	Port      int
	// Relay marks Port as an automatic loopback relay port, not a configured one.
	Relay bool
	// ForwardedBy is the peer whose port forward holds Port, when that is the
	// reason for the conflict.
	ForwardedBy string
	Transports  transportBits
}

// String renders the detail as a single-line, user-facing summary.
func (d *portConflictDetail) String() string {
	name := d.Remark
	if name == "" {
		name = d.Tag
	}
	if name == "" {
		name = fmt.Sprintf("#%d", d.InboundID)
	} else if d.InboundID > 0 {
		name = fmt.Sprintf("'%s' (#%d)", name, d.InboundID)
	} else {
		// reserved/system inbounds (e.g. the Xray API) have no DB id.
		name = fmt.Sprintf("'%s'", name)
	}
	listen := d.Listen
	if isAnyListen(listen) {
		listen = "*"
	}
	port := fmt.Sprintf("port %d", d.Port)
	if d.Relay {
		port = fmt.Sprintf("relay port %d", d.Port)
	}
	if d.ForwardedBy != "" {
		return fmt.Sprintf("%s (%s) already forwarded on inbound %s on %s by its client %s",
			port, transportTagSuffix(d.Transports), name, listen, d.ForwardedBy)
	}
	return fmt.Sprintf("%s (%s) already used by inbound %s on %s",
		port, transportTagSuffix(d.Transports), name, listen)
}

// defaultXrayAPIPort is the loopback port of the internal Xray API inbound
// (tag "api") seeded into the config template. Used as a fallback when the
// template can't be parsed.
const defaultXrayAPIPort = 62789

// reservedAPIPort returns the port of the internal Xray API inbound declared
// in the config template, falling back to defaultXrayAPIPort.
func reservedAPIPort() int {
	tmpl, err := (&SettingService{}).GetXrayConfigTemplate()
	if err != nil || tmpl == "" {
		return defaultXrayAPIPort
	}
	var parsed struct {
		Inbounds []struct {
			Port int    `json:"port"`
			Tag  string `json:"tag"`
		} `json:"inbounds"`
	}
	if json.Unmarshal([]byte(tmpl), &parsed) != nil {
		return defaultXrayAPIPort
	}
	for _, in := range parsed.Inbounds {
		if in.Tag == "api" && in.Port > 0 {
			return in.Port
		}
	}
	return defaultXrayAPIPort
}

// checkPortConflict reads outside any transaction; callers that must not race a
// concurrent create use checkPortConflictTx inside their own transaction.
func (s *InboundService) checkPortConflict(inbound *model.Inbound, ignoreId int) (*portConflictDetail, error) {
	return checkPortConflictTx(database.GetDB(), inbound, ignoreId)
}

func checkPortConflictTx(db *gorm.DB, inbound *model.Inbound, ignoreId int) (*portConflictDetail, error) {
	newBits := inboundTransports(inbound.Protocol, inbound.StreamSettings, inbound.Settings)

	// The internal Xray API inbound (tag "api", loopback TCP) isn't a DB row,
	// so a local user inbound reusing its port would leave Xray binding the
	// port twice (#5304). Nodes run their own Xray, so this only applies to
	// the local panel.
	if inbound.NodeID == nil && inbound.Port == reservedAPIPort() &&
		newBits&transportTCP != 0 && listenOverlaps("127.0.0.1", inbound.Listen) {
		return &portConflictDetail{
			Tag:        "api",
			Listen:     "127.0.0.1",
			Port:       inbound.Port,
			Transports: transportTCP,
		}, nil
	}

	// Egress SOCKS server holds loopback EgressBasePort when AWG outbounds are
	// active; conflict check prevents inbounds from colliding with it.
	if inbound.NodeID == nil && inbound.Port == int(amneziawgnet.EgressBasePort) &&
		newBits&transportTCP != 0 && listenOverlaps("127.0.0.1", inbound.Listen) {
		return &portConflictDetail{
			Tag:        "amneziawg-egress",
			Listen:     "127.0.0.1",
			Port:       inbound.Port,
			Transports: transportTCP,
		}, nil
	}

	// Every enabled local AmneziaWG inbound gets its own automatic Xray
	// SOCKS5 relay inbound (see injectAmneziawgnetSocks) on 127.0.0.1 at a
	// port derived purely from its id (amneziawgnet.SOCKSPortForInbound) --
	// like the internal Xray API inbound above, that relay inbound is not
	// itself a database row, so the ordinary DB-backed query below can never
	// see it. Without this check, an unrelated inbound saved onto that exact
	// port silently fails at the next Xray start, taking every other
	// protocol down with it, not just AmneziaWG.
	if inbound.NodeID == nil && listenOverlaps("127.0.0.1", inbound.Listen) {
		conflict, err := checkAmneziawgnetSocksConflict(db, inbound, ignoreId, newBits)
		if err != nil {
			return nil, err
		}
		if conflict != nil {
			return conflict, nil
		}
	}

	// The reverse direction, only meaningful once the id is known -- AddInbound
	// runs it after Save. Only a local row owns a relay slot (#6537 review).
	if inbound.NodeID == nil && inbound.Protocol == model.AmneziaWG && ignoreId > 0 {
		if self := amneziawgnetSocksSelfConflict(inbound, ignoreId); self != "" {
			return nil, common.NewError(self)
		}
		conflict, err := checkAmneziawgnetSocksRelayCollision(db, ignoreId)
		if err != nil {
			return nil, err
		}
		if conflict != nil {
			return conflict, nil
		}
		conflict, err = checkAmneziawgnetSocksReverseConflict(db, ignoreId)
		if err != nil {
			return nil, err
		}
		if conflict != nil {
			return conflict, nil
		}
	}

	// A forwarded port is not a column and not a relay slot, so the query below
	// cannot see it either: the port is bound by the peer's forward listener.
	forwardedBy, err := amneziawgForwardedPortOwner(db, inbound, ignoreId)
	if err != nil {
		return nil, err
	}
	if forwardedBy != nil {
		forwardedBy.Transports = newBits
		return forwardedBy, nil
	}

	var candidates []*model.Inbound
	q := db.Model(model.Inbound{}).Where("port = ?", inbound.Port)
	if ignoreId > 0 {
		q = q.Where("id != ?", ignoreId)
	}
	if err := q.Find(&candidates).Error; err != nil {
		return nil, err
	}

	for _, c := range candidates {
		if !sameNode(c.NodeID, inbound.NodeID) {
			continue
		}
		if !listenOverlaps(c.Listen, inbound.Listen) {
			continue
		}
		existingBits := inboundTransports(c.Protocol, c.StreamSettings, c.Settings)
		shared := existingBits & newBits
		if shared == 0 {
			continue
		}
		return &portConflictDetail{
			InboundID:  c.Id,
			Remark:     c.Remark,
			Tag:        c.Tag,
			Listen:     c.Listen,
			Port:       c.Port,
			Transports: shared,
		}, nil
	}
	return nil, nil
}

// amneziawgForwardedPortOwner names the AmneziaWG row on the same host whose peer
// forwards inbound's port -- a bind on every interface that only the AWG side checked.
func amneziawgForwardedPortOwner(db *gorm.DB, inbound *model.Inbound, ignoreId int) (*portConflictDetail, error) {
	var rows []*model.Inbound
	q := db.Model(model.Inbound{}).Where("protocol = ?", model.AmneziaWG)
	if ignoreId > 0 {
		q = q.Where("id != ?", ignoreId)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if !sameNode(row.NodeID, inbound.NodeID) {
			continue
		}
		instance, ok := amneziawg.InstanceFromInbound(row)
		if !ok {
			continue
		}
		email, forwards := amneziawgnet.ForwardedPortOwner(instance, inbound.Port)
		if !forwards {
			continue
		}
		return &portConflictDetail{
			InboundID: row.Id,
			Remark:    row.Remark,
			Tag:       row.Tag,
			// the forward binds :port on every interface, wherever the
			// candidate asked to listen.
			Listen:      "",
			Port:        inbound.Port,
			ForwardedBy: email,
		}, nil
	}
	return nil, nil
}

// checkAmneziawgnetSocksConflict: inbound's port vs the relay port every matching
// local row reserves, emitted or not; db keeps it in the caller's transaction (#6225).
func checkAmneziawgnetSocksConflict(db *gorm.DB, inbound *model.Inbound, ignoreId int, newBits transportBits) (*portConflictDetail, error) {
	// A disabled row still owns the slot its id derives: SetInboundEnable flips
	// the column with no port check, so enabling it later must not collide.
	var candidates []*model.Inbound
	q := db.Model(model.Inbound{}).Where("protocol = ? AND node_id IS NULL", model.AmneziaWG)
	if ignoreId > 0 {
		q = q.Where("id != ?", ignoreId)
	}
	if err := q.Find(&candidates).Error; err != nil {
		return nil, err
	}
	// Ownership does not depend on the peers: the relay appears when the first
	// client is added, and the client paths run no port check at all.
	for _, c := range candidates {
		if amneziawgnet.SOCKSPortForInbound(c.Id) != inbound.Port {
			continue
		}
		return &portConflictDetail{
			InboundID:  c.Id,
			Remark:     c.Remark,
			Tag:        c.Tag,
			Listen:     "127.0.0.1",
			Port:       inbound.Port,
			Transports: newBits,
		}, nil
	}
	return nil, nil
}

// checkAmneziawgnetSocksRelayCollision reports whether id's derived relay port
// is already claimed by another local AmneziaWG inbound, disabled rows included.
func checkAmneziawgnetSocksRelayCollision(db *gorm.DB, id int) (*portConflictDetail, error) {
	relayPort := amneziawgnet.SOCKSPortForInbound(id)
	var candidates []*model.Inbound
	if err := db.Model(model.Inbound{}).
		Where("protocol = ? AND node_id IS NULL AND id != ?", model.AmneziaWG, id).
		Find(&candidates).Error; err != nil {
		return nil, err
	}
	for _, c := range candidates {
		if amneziawgnet.SOCKSPortForInbound(c.Id) != relayPort {
			continue
		}
		return &portConflictDetail{
			InboundID:  c.Id,
			Remark:     c.Remark,
			Tag:        c.Tag,
			Listen:     "127.0.0.1",
			Port:       relayPort,
			Relay:      true,
			Transports: transportTCP,
		}, nil
	}
	return nil, nil
}

// amneziawgnetSocksSelfConflict: a row's own WireGuard port vs the relay port its
// own id derives -- all three checks below exclude that id, so nothing else does.
func amneziawgnetSocksSelfConflict(inbound *model.Inbound, id int) string {
	if id <= 0 || inbound.NodeID != nil || !listenOverlaps("127.0.0.1", inbound.Listen) {
		return ""
	}
	relayPort := amneziawgnet.SOCKSPortForInbound(id)
	if inbound.Port != relayPort {
		return ""
	}
	return fmt.Sprintf("WireGuard port %d is inbound #%d's own SOCKS5 relay port on 127.0.0.1; choose a different WireGuard port",
		relayPort, id)
}

// checkAmneziawgnetSocksReverseConflict mirrors checkAmneziawgnetSocksConflict:
// does id's own derived relay port collide with some other inbound's port.
func checkAmneziawgnetSocksReverseConflict(db *gorm.DB, id int) (*portConflictDetail, error) {
	relayPort := amneziawgnet.SOCKSPortForInbound(id)
	var candidates []*model.Inbound
	if err := db.Model(model.Inbound{}).
		Where("port = ? AND node_id IS NULL AND id != ?", relayPort, id).
		Find(&candidates).Error; err != nil {
		return nil, err
	}
	for _, c := range candidates {
		if !listenOverlaps("127.0.0.1", c.Listen) {
			continue
		}
		return &portConflictDetail{
			InboundID:  c.Id,
			Remark:     c.Remark,
			Tag:        c.Tag,
			Listen:     c.Listen,
			Port:       relayPort,
			Relay:      true,
			Transports: transportTCP,
		}, nil
	}
	return nil, nil
}

func sameNode(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func baseInboundTag(port int) string {
	return fmt.Sprintf("in-%v", port)
}

func transportTagSuffix(b transportBits) string {
	switch b {
	case transportTCP:
		return "tcp"
	case transportUDP:
		return "udp"
	case transportTCP | transportUDP:
		return "tcpudp"
	}
	return "any"
}

// nodeTagPrefix scopes a tag to one remote node so the same listen+port
// can live on the central panel and on a node without bumping the global
// UNIQUE(inbounds.tag) constraint. nil → "" (local panel).
func nodeTagPrefix(nodeID *int) string {
	if nodeID == nil {
		return ""
	}
	return fmt.Sprintf("n%d-", *nodeID)
}

func composeInboundTag(port int, nodeID *int, bits transportBits) string {
	return nodeTagPrefix(nodeID) + baseInboundTag(port) + "-" + transportTagSuffix(bits)
}

func isAutoGeneratedTag(tag string, port int, nodeID *int, bits transportBits) bool {
	base := composeInboundTag(port, nodeID, bits)
	if tag == base {
		return true
	}
	suffix, ok := strings.CutPrefix(tag, base+"-")
	if !ok || suffix == "" {
		return false
	}
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (s *InboundService) generateInboundTag(inbound *model.Inbound, ignoreId int) (string, error) {
	bits := inboundTransports(inbound.Protocol, inbound.StreamSettings, inbound.Settings)
	candidate := composeInboundTag(inbound.Port, inbound.NodeID, bits)
	exists, err := s.tagExists(candidate, ignoreId)
	if err != nil {
		return "", err
	}
	if !exists {
		return candidate, nil
	}

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
