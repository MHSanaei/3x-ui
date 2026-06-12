// Package model defines the database models and data structures used by the 3x-ui panel.
package model

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// Protocol represents the protocol type for Xray inbounds.
type Protocol string

// Protocol constants for different Xray inbound protocols.
// Hysteria v2 is not a distinct protocol — it is plain "hysteria"
// with streamSettings.version = 2. The share-link URI scheme
// "hysteria2://" is independent of this and is still emitted by the
// link generator when the stream version is 2.
const (
	VMESS       Protocol = "vmess"
	VLESS       Protocol = "vless"
	Tunnel      Protocol = "tunnel"
	HTTP        Protocol = "http"
	Trojan      Protocol = "trojan"
	Shadowsocks Protocol = "shadowsocks"
	Mixed       Protocol = "mixed"
	WireGuard   Protocol = "wireguard"
	Hysteria    Protocol = "hysteria"
	MTProto     Protocol = "mtproto"
)

// User represents a user account in the 3x-ui panel.
type User struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	LoginEpoch int64  `json:"-" gorm:"default:0"`
}

// Inbound represents an Xray inbound configuration with traffic statistics and settings.
type Inbound struct {
	Id                   int                  `json:"id" form:"id" gorm:"primaryKey;autoIncrement" example:"1"`                                                                                                     // Unique identifier
	UserId               int                  `json:"-"`                                                                                                                                                            // Associated user ID
	Up                   int64                `json:"up" form:"up"`                                                                                                                                                 // Upload traffic in bytes
	Down                 int64                `json:"down" form:"down"`                                                                                                                                             // Download traffic in bytes
	Total                int64                `json:"total" form:"total"`                                                                                                                                           // Total traffic limit in bytes
	Remark               string               `json:"remark" form:"remark" example:"VLESS-443"`                                                                                                                     // Human-readable remark
	SubSortIndex         int                  `json:"subSortIndex" form:"subSortIndex" gorm:"default:1" validate:"omitempty,gte=1" example:"1"`                                                                     // 1-based sort order of this inbound's links in subscription output only (lower first; ties by id)
	Enable               bool                 `json:"enable" form:"enable" gorm:"index:idx_enable_traffic_reset,priority:1" example:"true"`                                                                         // Whether the inbound is enabled
	ExpiryTime           int64                `json:"expiryTime" form:"expiryTime"`                                                                                                                                 // Expiration timestamp
	TrafficReset         string               `json:"trafficReset" form:"trafficReset" gorm:"default:never;index:idx_enable_traffic_reset,priority:2" validate:"omitempty,oneof=never hourly daily weekly monthly"` // Traffic reset schedule
	LastTrafficResetTime int64                `json:"lastTrafficResetTime" form:"lastTrafficResetTime" gorm:"default:0"`                                                                                            // Last traffic reset timestamp
	ClientStats          []xray.ClientTraffic `gorm:"foreignKey:InboundId;references:Id" json:"clientStats" form:"clientStats"`                                                                                     // Client traffic statistics

	// Xray configuration fields
	Listen            string   `json:"listen" form:"listen"`
	Port              int      `json:"port" form:"port" validate:"gte=0,lte=65535" example:"443"`
	Protocol          Protocol `json:"protocol" form:"protocol" validate:"required,oneof=vmess vless trojan shadowsocks wireguard hysteria http mixed tunnel tun mtproto" example:"vless"`
	Settings          string   `json:"settings" form:"settings"`
	StreamSettings    string   `json:"streamSettings" form:"streamSettings"`
	Tag               string   `json:"tag" form:"tag" gorm:"unique" example:"in-443-tcp"`
	Sniffing          string   `json:"sniffing" form:"sniffing"`
	NodeID            *int     `json:"nodeId,omitempty" form:"nodeId" gorm:"index"`
	ShareAddrStrategy string   `json:"shareAddrStrategy" form:"shareAddrStrategy" gorm:"column:share_addr_strategy;default:node" validate:"omitempty,oneof=node listen custom"`
	ShareAddr         string   `json:"shareAddr" form:"shareAddr" gorm:"column:share_addr"`

	// OriginNodeGuid is the panelGuid of the node that physically hosts this
	// inbound, propagated up across hops (#4983). Empty for an inbound that
	// lives on this panel's own xray; set to the originating node's GUID when
	// the inbound was synced from a node (kept as-is across further hops). Lets
	// the master attribute a deeply nested inbound to the real node instead of
	// the intermediate one it was fetched through.
	OriginNodeGuid string `json:"originNodeGuid,omitempty" form:"originNodeGuid" gorm:"column:origin_node_guid;index"`

	// FallbackParent is populated by the API layer when this inbound is
	// attached as a fallback child of a VLESS/Trojan TCP-TLS master.
	// The frontend uses it to rewrite client-share links so they advertise
	// the master's externally reachable endpoint instead of the child's
	// loopback listen. Not persisted.
	FallbackParent *FallbackParentInfo `json:"fallbackParent,omitempty" gorm:"-"`
}

// FallbackParentInfo carries everything the frontend needs to rewrite a
// child inbound's client link: where to connect (the master's address
// and port) and which path matched on the master's fallbacks array.
// The frontend already has the master inbound in its dbInbounds list,
// so we only ship identifiers + the match path here.
type FallbackParentInfo struct {
	MasterId int    `json:"masterId"`
	Path     string `json:"path,omitempty"`
}

// OutboundTraffics tracks traffic statistics for Xray outbound connections.
type OutboundTraffics struct {
	Id    int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Tag   string `json:"tag" form:"tag" gorm:"unique"`
	Up    int64  `json:"up" form:"up" gorm:"default:0"`
	Down  int64  `json:"down" form:"down" gorm:"default:0"`
	Total int64  `json:"total" form:"total" gorm:"default:0"`
}

// InboundClientIps stores IP addresses associated with inbound clients for access control.
type InboundClientIps struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ClientEmail string `json:"clientEmail" form:"clientEmail" gorm:"unique"`
	Ips         string `json:"ips" form:"ips"`
}

// MarshalJSON emits the Ips column as a real JSON array instead of an escaped
// JSON-text string. Empty or unparseable storage renders as null so API
// consumers don't have to special-case the legacy double-encoded shape.
func (ic InboundClientIps) MarshalJSON() ([]byte, error) {
	type alias InboundClientIps
	return json.Marshal(struct {
		alias
		Ips json.RawMessage `json:"ips"`
	}{
		alias: alias(ic),
		Ips:   jsonStringFieldToRaw(ic.Ips),
	})
}

// UnmarshalJSON accepts ips as either a JSON array (modern shape) or a
// JSON-encoded string (legacy shape), normalising back to the JSON-text the
// column stores.
func (ic *InboundClientIps) UnmarshalJSON(data []byte) error {
	type alias InboundClientIps
	aux := struct {
		*alias
		Ips json.RawMessage `json:"ips"`
	}{
		alias: (*alias)(ic),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	ic.Ips = jsonStringFieldFromRaw(aux.Ips)
	return nil
}

// HistoryOfSeeders tracks which database seeders have been executed to prevent re-running.
type HistoryOfSeeders struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	SeederName string `json:"seederName"`
}

type ApiToken struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string `json:"name" gorm:"uniqueIndex;not null"`
	Token     string `json:"token" gorm:"not null"` // SHA-256 hash; the plaintext is shown only once at creation
	Enabled   bool   `json:"enabled" gorm:"default:true"`
	CreatedAt int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
}

// MarshalJSON emits settings, streamSettings, and sniffing as nested JSON
// objects rather than escaped strings, so API consumers don't need to JSON.parse
// a string inside a string. Empty fields render as null; fields whose stored
// text isn't valid JSON fall back to a JSON-encoded string so no data is lost.
func (i Inbound) MarshalJSON() ([]byte, error) {
	type alias Inbound
	return json.Marshal(struct {
		alias
		Settings       json.RawMessage `json:"settings"`
		StreamSettings json.RawMessage `json:"streamSettings"`
		Sniffing       json.RawMessage `json:"sniffing"`
	}{
		alias:          alias(i),
		Settings:       jsonStringFieldToRaw(i.Settings),
		StreamSettings: jsonStringFieldToRaw(i.StreamSettings),
		Sniffing:       jsonStringFieldToRaw(i.Sniffing),
	})
}

// UnmarshalJSON accepts settings, streamSettings, and sniffing as either a raw
// JSON object/array (the modern shape MarshalJSON emits) or a JSON-encoded
// string (the legacy shape). Either form is normalised back to the JSON-text
// string the DB column stores.
func (i *Inbound) UnmarshalJSON(data []byte) error {
	type alias Inbound
	aux := struct {
		*alias
		Settings       json.RawMessage `json:"settings"`
		StreamSettings json.RawMessage `json:"streamSettings"`
		Sniffing       json.RawMessage `json:"sniffing"`
	}{
		alias: (*alias)(i),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	i.Settings = jsonStringFieldFromRaw(aux.Settings)
	i.StreamSettings = jsonStringFieldFromRaw(aux.StreamSettings)
	i.Sniffing = jsonStringFieldFromRaw(aux.Sniffing)
	return nil
}

func jsonStringFieldToRaw(s string) json.RawMessage {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return json.RawMessage("null")
	}
	if json.Valid([]byte(trimmed)) {
		return json.RawMessage(trimmed)
	}
	b, _ := json.Marshal(s)
	return b
}

func jsonStringFieldFromRaw(r json.RawMessage) string {
	trimmed := bytes.TrimSpace(r)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err == nil {
			return s
		}
	}
	return string(trimmed)
}

// GenXrayInboundConfig generates an Xray inbound configuration from the Inbound model.
func (i *Inbound) GenXrayInboundConfig() *xray.InboundConfig {
	listen := i.Listen
	if listen == "" {
		listen = "0.0.0.0"
	}
	listen = fmt.Sprintf("\"%v\"", listen)
	protocol := string(i.Protocol)
	settings := i.Settings
	switch i.Protocol {
	case Shadowsocks:
		if healed, ok := HealShadowsocksClientMethods(settings); ok {
			settings = healed
		}
	case VMESS:
		if stripped, ok := StripVmessClientSecurity(settings); ok {
			settings = stripped
		}
	case VLESS:
		if stripped, ok := StripVlessInboundEncryption(settings); ok {
			settings = stripped
		}
	}
	return &xray.InboundConfig{
		Listen:         json_util.RawMessage(listen),
		Port:           i.Port,
		Protocol:       protocol,
		Settings:       json_util.RawMessage(settings),
		StreamSettings: json_util.RawMessage(i.StreamSettings),
		Tag:            i.Tag,
		Sniffing:       json_util.RawMessage(i.Sniffing),
	}
}

func StripVmessClientSecurity(settings string) (string, bool) {
	if settings == "" {
		return settings, false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return settings, false
	}
	clients, ok := parsed["clients"].([]any)
	if !ok {
		return settings, false
	}
	changed := false
	for i := range clients {
		cm, ok := clients[i].(map[string]any)
		if !ok {
			continue
		}
		if _, has := cm["security"]; has {
			delete(cm, "security")
			clients[i] = cm
			changed = true
		}
	}
	if !changed {
		return settings, false
	}
	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return settings, false
	}
	return string(out), true
}

func StripVlessInboundEncryption(settings string) (string, bool) {
	if settings == "" {
		return settings, false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return settings, false
	}
	if _, has := parsed["encryption"]; !has {
		return settings, false
	}
	delete(parsed, "encryption")
	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return settings, false
	}
	return string(out), true
}

// HealShadowsocksClientMethods normalises the per-client `method` field
// on a shadowsocks inbound's settings JSON before it leaves for xray-core:
//   - Legacy ciphers (aes-*, chacha20-*): every client must carry a
//     per-user `method` matching the inbound's top-level method, otherwise
//     xray fails with "unsupported cipher method:".
//   - Shadowsocks 2022 (2022-blake3-*): xray's multi-user code rejects the
//     inbound with "users must have empty method" when a client carries
//     one — strip stale entries left over from a switch off a legacy
//     cipher.
//
// Returns the rewritten settings string and true when anything changed.
func HealShadowsocksClientMethods(settings string) (string, bool) {
	if settings == "" {
		return settings, false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return settings, false
	}
	method, _ := parsed["method"].(string)
	clients, ok := parsed["clients"].([]any)
	if !ok {
		return settings, false
	}
	is2022 := strings.HasPrefix(method, "2022-blake3-")
	changed := false
	for i := range clients {
		cm, ok := clients[i].(map[string]any)
		if !ok {
			continue
		}
		if is2022 {
			if _, hasKey := cm["method"]; hasKey {
				delete(cm, "method")
				clients[i] = cm
				changed = true
			}
			continue
		}
		if method == "" {
			continue
		}
		existing, _ := cm["method"].(string)
		if existing == method {
			continue
		}
		cm["method"] = method
		clients[i] = cm
		changed = true
	}
	if !changed {
		return settings, false
	}
	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return settings, false
	}
	return string(out), true
}

// GenerateFakeTLSSecret builds an MTProto FakeTLS secret for the given domain:
// the "ee" FakeTLS marker, 16 random bytes, then the domain encoded as hex.
// This single value is what mtg's config and the client tg:// link both use.
func GenerateFakeTLSSecret(domain string) string {
	return "ee" + mtprotoRandomMiddle() + hex.EncodeToString([]byte(domain))
}

func mtprotoRandomMiddle() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("mtproto: crypto/rand read failed: %w", err))
	}
	return hex.EncodeToString(buf)
}

// mtprotoSecretMiddle returns the 16-byte random middle of an existing secret
// when it is well-formed, otherwise a freshly generated one. Reusing the middle
// keeps the secret stable when only the FakeTLS domain changes.
func mtprotoSecretMiddle(secret string) string {
	s := secret
	if strings.HasPrefix(s, "ee") || strings.HasPrefix(s, "dd") {
		s = s[2:]
	}
	if len(s) >= 32 {
		mid := s[:32]
		if _, err := hex.DecodeString(mid); err == nil {
			return mid
		}
	}
	return mtprotoRandomMiddle()
}

// HealMtprotoSecret normalises an mtproto inbound's settings JSON before the
// value leaves for the mtg sidecar or a share link: it rebuilds `secret` so it
// is always a valid FakeTLS secret whose trailing domain matches
// `fakeTlsDomain`, generating the random middle when one is missing and
// rewriting the domain suffix when the domain changed. Returns the rewritten
// settings and true when anything changed.
func HealMtprotoSecret(settings string) (string, bool) {
	if settings == "" {
		return settings, false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return settings, false
	}
	domain, _ := parsed["fakeTlsDomain"].(string)
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return settings, false
	}
	secret, _ := parsed["secret"].(string)
	expected := "ee" + mtprotoSecretMiddle(secret) + hex.EncodeToString([]byte(domain))
	if secret == expected {
		return settings, false
	}
	parsed["secret"] = expected
	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return settings, false
	}
	return string(out), true
}

// Setting stores key-value configuration settings for the 3x-ui panel.
type Setting struct {
	Id    int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Key   string `json:"key" form:"key"`
	Value string `json:"value" form:"value"`
}

// Node represents a remote 3x-ui panel registered with the central panel.
// The central panel polls each node's existing /panel/api/server/status
// endpoint over HTTP using the per-node ApiToken to populate the runtime
// status fields below.
type Node struct {
	Id                  int      `json:"id" form:"id" gorm:"primaryKey;autoIncrement" example:"1"`
	Name                string   `json:"name" form:"name" gorm:"uniqueIndex" validate:"required" example:"de-fra-1"`
	Remark              string   `json:"remark" form:"remark"`
	Scheme              string   `json:"scheme" form:"scheme" validate:"omitempty,oneof=http https" example:"https"`
	Address             string   `json:"address" form:"address" validate:"required" example:"node1.example.com"`
	Port                int      `json:"port" form:"port" validate:"gte=1,lte=65535" example:"2053"`
	BasePath            string   `json:"basePath" form:"basePath" example:"/"`
	ApiToken            string   `json:"apiToken" form:"apiToken" validate:"required" example:"abcdef0123456789"`
	Enable              bool     `json:"enable" form:"enable" gorm:"default:true" example:"true"`
	AllowPrivateAddress bool     `json:"allowPrivateAddress" form:"allowPrivateAddress" gorm:"default:false"`
	TlsVerifyMode       string   `json:"tlsVerifyMode" form:"tlsVerifyMode" gorm:"column:tls_verify_mode;default:verify" validate:"omitempty,oneof=verify skip pin"`
	PinnedCertSha256    string   `json:"pinnedCertSha256" form:"pinnedCertSha256" gorm:"column:pinned_cert_sha256"`
	InboundSyncMode     string   `json:"inboundSyncMode" form:"inboundSyncMode" gorm:"column:inbound_sync_mode;default:all" validate:"omitempty,oneof=all selected"`
	InboundTags         []string `json:"inboundTags" form:"inboundTags" gorm:"serializer:json;column:inbound_tags"`

	// Guid is the remote panel's stable self-identifier (its panelGuid),
	// learned from each heartbeat. It is the globally stable node identity used
	// to attribute online clients/inbounds to the physical node across a chain
	// of nodes (#4983); panel-local autoincrement ids don't survive a hop.
	// Observed-state only — never user-edited.
	Guid string `json:"guid" gorm:"column:guid;index"`

	// Heartbeat-updated fields. UpdatedAt advances on every probe even when
	// the row is otherwise unchanged so the UI's "last seen" tooltip is
	// truthful without us having to read LastHeartbeat separately.
	Status        string  `json:"status" gorm:"default:unknown" example:"online"` // online|offline|unknown
	LastHeartbeat int64   `json:"lastHeartbeat" example:"1700000000"`             // unix seconds, 0 = never
	LatencyMs     int     `json:"latencyMs" example:"42"`
	XrayVersion   string  `json:"xrayVersion" example:"25.10.31"`
	PanelVersion  string  `json:"panelVersion" gorm:"column:panel_version" example:"v3.x.x"`
	CpuPct        float64 `json:"cpuPct" example:"23.5"`
	MemPct        float64 `json:"memPct" example:"45.1"`
	UptimeSecs    uint64  `json:"uptimeSecs" example:"86400"`
	LastError     string  `json:"lastError"`

	// XrayState and XrayError are captured from the remote node's /panel/api/server/status
	// during heartbeats. They let the central panel distinguish "panel API reachable"
	// (status=online) from "Xray core itself has failed on the node" for monitoring.
	XrayState string `json:"xrayState" gorm:"column:xray_state"`
	XrayError string `json:"xrayError" gorm:"column:xray_error"`

	ConfigDirty   bool  `json:"configDirty" gorm:"default:false"`
	ConfigDirtyAt int64 `json:"configDirtyAt"`

	InboundCount  int `json:"inboundCount" gorm:"-" example:"5"`
	ClientCount   int `json:"clientCount" gorm:"-" example:"27"`
	OnlineCount   int `json:"onlineCount" gorm:"-" example:"3"`
	DepletedCount int `json:"depletedCount" gorm:"-" example:"1"`

	// ParentGuid + Transitive are set only when a node is surfaced as part of a
	// node tree (#4983): direct nodes carry the master panel's own GUID, a
	// transitive sub-node carries its parent node's GUID. Transitive nodes are
	// read-only projections (Id == 0, not persisted) — never edited or deployed.
	ParentGuid string `json:"parentGuid,omitempty" gorm:"-"`
	Transitive bool   `json:"transitive,omitempty" gorm:"-"`

	CreatedAt int64 `json:"createdAt" gorm:"autoCreateTime:milli" example:"1700000000"`
	UpdatedAt int64 `json:"updatedAt" gorm:"autoUpdateTime:milli" example:"1700000000"`
}

// NodeSummary is the read-only identity of a node as published one hop up: the
// view a panel exposes about the nodes it directly manages, so a master can
// surface transitive sub-nodes in a chained topology (#4983). Counts are
// computed by the consuming master from its own per-GUID data, never trusted
// from the child, so this carries identity/health only.
type NodeSummary struct {
	Guid          string `json:"guid"`
	ParentGuid    string `json:"parentGuid"`
	Name          string `json:"name"`
	Address       string `json:"address"`
	Scheme        string `json:"scheme"`
	Port          int    `json:"port"`
	Status        string `json:"status"`
	LastHeartbeat int64  `json:"lastHeartbeat"`
	LatencyMs     int    `json:"latencyMs"`
	PanelVersion  string `json:"panelVersion"`
	XrayVersion   string `json:"xrayVersion"`
	// XrayState/XrayError forwarded so masters can surface xray failure on transitive sub-nodes too.
	XrayState string `json:"xrayState"`
	XrayError string `json:"xrayError,omitempty"`
}

type ClientReverse struct {
	Tag string `json:"tag"`
}

// Client represents a client configuration for Xray inbounds with traffic limits and settings.
type Client struct {
	ID         string         `json:"id,omitempty"`                 // Unique client identifier
	Security   string         `json:"security"`                     // Security method (e.g., "auto", "aes-128-gcm")
	Password   string         `json:"password,omitempty"`           // Client password
	Flow       string         `json:"flow,omitempty"`               // Flow control (XTLS)
	Reverse    *ClientReverse `json:"reverse,omitempty"`            // VLESS simple reverse proxy settings
	Auth       string         `json:"auth,omitempty"`               // Auth password (Hysteria)
	Email      string         `json:"email"`                        // Client email identifier
	LimitIP    int            `json:"limitIp"`                      // IP limit for this client
	TotalGB    int64          `json:"totalGB" form:"totalGB"`       // Total traffic limit in GB
	ExpiryTime int64          `json:"expiryTime" form:"expiryTime"` // Expiration timestamp
	Enable     bool           `json:"enable" form:"enable"`         // Whether the client is enabled
	TgID       int64          `json:"tgId" form:"tgId"`             // Telegram user ID for notifications
	SubID      string         `json:"subId" form:"subId"`           // Subscription identifier
	Group      string         `json:"group,omitempty" form:"group"` // Logical grouping label
	Comment    string         `json:"comment" form:"comment"`       // Client comment
	Reset      int            `json:"reset" form:"reset"`           // Reset period in days
	CreatedAt  int64          `json:"created_at,omitempty"`         // Creation timestamp
	UpdatedAt  int64          `json:"updated_at,omitempty"`         // Last update timestamp
}

type ClientRecord struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Email      string `json:"email" gorm:"uniqueIndex;not null"`
	SubID      string `json:"subId" gorm:"index;column:sub_id"`
	UUID       string `json:"uuid" gorm:"column:uuid"`
	Password   string `json:"password"`
	Auth       string `json:"auth"`
	Flow       string `json:"flow"`
	Security   string `json:"security"`
	Reverse    string `json:"reverse" gorm:"column:reverse"`
	LimitIP    int    `json:"limitIp" gorm:"column:limit_ip"`
	TotalGB    int64  `json:"totalGB" gorm:"column:total_gb"`
	ExpiryTime int64  `json:"expiryTime" gorm:"column:expiry_time"`
	Enable     bool   `json:"enable" gorm:"default:true"`
	TgID       int64  `json:"tgId" gorm:"column:tg_id"`
	Group      string `json:"group" gorm:"column:group_name;default:''"`
	Comment    string `json:"comment"`
	Reset      int    `json:"reset" gorm:"default:0"`
	CreatedAt  int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt  int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}

func (ClientRecord) TableName() string { return "clients" }

type ClientGroup struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string `json:"name" gorm:"uniqueIndex;not null"`
	CreatedAt int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}

func (ClientGroup) TableName() string { return "client_groups" }

// MarshalJSON emits the reverse column as a nested JSON object rather than an
// escaped JSON-text string, matching the same convention Inbound uses for its
// JSON-text columns. Empty storage renders as null.
func (r ClientRecord) MarshalJSON() ([]byte, error) {
	type alias ClientRecord
	return json.Marshal(struct {
		alias
		Reverse json.RawMessage `json:"reverse"`
	}{
		alias:   alias(r),
		Reverse: jsonStringFieldToRaw(r.Reverse),
	})
}

// UnmarshalJSON accepts reverse as either a JSON object (modern shape) or a
// JSON-encoded string (legacy shape).
func (r *ClientRecord) UnmarshalJSON(data []byte) error {
	type alias ClientRecord
	aux := struct {
		*alias
		Reverse json.RawMessage `json:"reverse"`
	}{
		alias: (*alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	r.Reverse = jsonStringFieldFromRaw(aux.Reverse)
	return nil
}

type ClientInbound struct {
	ClientId     int    `json:"clientId" gorm:"primaryKey;column:client_id;index"`
	InboundId    int    `json:"inboundId" gorm:"primaryKey;column:inbound_id;index"`
	FlowOverride string `json:"flowOverride" gorm:"column:flow_override"`
	CreatedAt    int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
}

func (ClientInbound) TableName() string { return "client_inbounds" }

type InboundFallback struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	MasterId  int    `json:"masterId" gorm:"index;not null;column:master_id"`
	ChildId   int    `json:"childId" gorm:"index;not null;column:child_id"`
	Name      string `json:"name"`
	Alpn      string `json:"alpn"`
	Path      string `json:"path"`
	Dest      string `json:"dest"`
	Xver      int    `json:"xver"`
	SortOrder int    `json:"sortOrder" gorm:"default:0;column:sort_order"`
}

func (InboundFallback) TableName() string { return "inbound_fallbacks" }

func (c *Client) ToRecord() *ClientRecord {
	rec := &ClientRecord{
		Email:      c.Email,
		SubID:      c.SubID,
		UUID:       c.ID,
		Password:   c.Password,
		Auth:       c.Auth,
		Flow:       c.Flow,
		Security:   c.Security,
		LimitIP:    c.LimitIP,
		TotalGB:    c.TotalGB,
		ExpiryTime: c.ExpiryTime,
		Enable:     c.Enable,
		TgID:       c.TgID,
		Group:      c.Group,
		Comment:    c.Comment,
		Reset:      c.Reset,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
	if c.Reverse != nil {
		if b, err := json.Marshal(c.Reverse); err == nil {
			rec.Reverse = string(b)
		}
	}
	return rec
}

func (r *ClientRecord) ToClient() *Client {
	c := &Client{
		ID:         r.UUID,
		Email:      r.Email,
		SubID:      r.SubID,
		Password:   r.Password,
		Auth:       r.Auth,
		Flow:       r.Flow,
		Security:   r.Security,
		LimitIP:    r.LimitIP,
		TotalGB:    r.TotalGB,
		ExpiryTime: r.ExpiryTime,
		Enable:     r.Enable,
		TgID:       r.TgID,
		Group:      r.Group,
		Comment:    r.Comment,
		Reset:      r.Reset,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
	if r.Reverse != "" {
		var rev ClientReverse
		if err := json.Unmarshal([]byte(r.Reverse), &rev); err == nil {
			c.Reverse = &rev
		}
	}
	return c
}

type ClientMergeConflict struct {
	Field string
	Old   any
	New   any
	Kept  any
}

type OutboundSubscription struct {
	Id                   int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Remark               string `json:"remark" form:"remark"`
	Url                  string `json:"url" form:"url"`
	Enabled              bool   `json:"enabled" form:"enabled" gorm:"default:true"`
	AllowPrivate         bool   `json:"allowPrivate" form:"allowPrivate" gorm:"default:false"`
	TagPrefix            string `json:"tagPrefix" form:"tagPrefix"`
	UpdateInterval       int    `json:"updateInterval" form:"updateInterval" gorm:"default:600"` // seconds between refreshes
	Priority             int    `json:"priority" form:"priority" gorm:"default:0"`               // order among subscriptions in the merged outbounds (lower = earlier)
	Prepend              bool   `json:"prepend" form:"prepend" gorm:"default:false"`             // place this subscription's outbounds before the manual template outbounds
	LastUpdated          int64  `json:"lastUpdated" form:"lastUpdated"`
	LastError            string `json:"lastError" form:"lastError"`
	LastFetchedOutbounds string `json:"lastFetchedOutbounds" form:"lastFetchedOutbounds" gorm:"type:text"`
	LinkIdentities       string `json:"-" gorm:"type:text;column:link_identities"`
	CreatedAt            int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt            int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
	OutboundCount        int    `json:"outboundCount" gorm:"-"`
}

func MergeClientRecord(existing *ClientRecord, incoming *ClientRecord) []ClientMergeConflict {
	var conflicts []ClientMergeConflict
	keep := func(field string, oldV, newV, kept any) {
		conflicts = append(conflicts, ClientMergeConflict{Field: field, Old: oldV, New: newV, Kept: kept})
	}
	const redacted = "<redacted>"
	keepSecret := func(field string) {
		conflicts = append(conflicts, ClientMergeConflict{Field: field, Old: redacted, New: redacted, Kept: redacted})
	}

	incomingNewer := incoming.UpdatedAt > existing.UpdatedAt ||
		(incoming.UpdatedAt == existing.UpdatedAt && incoming.CreatedAt > existing.CreatedAt)

	if existing.UUID != incoming.UUID && incoming.UUID != "" {
		if incomingNewer || existing.UUID == "" {
			existing.UUID = incoming.UUID
		}
		keepSecret("uuid")
	}
	if existing.Password != incoming.Password && incoming.Password != "" {
		if incomingNewer || existing.Password == "" {
			existing.Password = incoming.Password
			keepSecret("password")
		}
	}
	if existing.Auth != incoming.Auth && incoming.Auth != "" {
		if incomingNewer || existing.Auth == "" {
			existing.Auth = incoming.Auth
			keepSecret("auth")
		}
	}
	if existing.Flow != incoming.Flow && incoming.Flow != "" {
		if incomingNewer || existing.Flow == "" {
			keep("flow", existing.Flow, incoming.Flow, incoming.Flow)
			existing.Flow = incoming.Flow
		}
	}
	if existing.Security != incoming.Security && incoming.Security != "" {
		if incomingNewer || existing.Security == "" {
			keep("security", existing.Security, incoming.Security, incoming.Security)
			existing.Security = incoming.Security
		}
	}
	if existing.SubID != incoming.SubID && incoming.SubID != "" {
		if incomingNewer || existing.SubID == "" {
			existing.SubID = incoming.SubID
			keepSecret("subId")
		}
	}
	if existing.TotalGB != incoming.TotalGB {
		picked := existing.TotalGB
		if existing.TotalGB == 0 || (incoming.TotalGB != 0 && incoming.TotalGB > existing.TotalGB) {
			picked = incoming.TotalGB
		}
		if picked != existing.TotalGB {
			keep("totalGB", existing.TotalGB, incoming.TotalGB, picked)
			existing.TotalGB = picked
		}
	}
	if existing.ExpiryTime != incoming.ExpiryTime {
		picked := existing.ExpiryTime
		if existing.ExpiryTime == 0 || (incoming.ExpiryTime != 0 && incoming.ExpiryTime > existing.ExpiryTime) {
			picked = incoming.ExpiryTime
		}
		if picked != existing.ExpiryTime {
			keep("expiryTime", existing.ExpiryTime, incoming.ExpiryTime, picked)
			existing.ExpiryTime = picked
		}
	}
	if existing.LimitIP != incoming.LimitIP && incoming.LimitIP != 0 {
		picked := existing.LimitIP
		if existing.LimitIP == 0 || incoming.LimitIP > existing.LimitIP {
			picked = incoming.LimitIP
		}
		if picked != existing.LimitIP {
			keep("limitIp", existing.LimitIP, incoming.LimitIP, picked)
			existing.LimitIP = picked
		}
	}
	if existing.TgID != incoming.TgID && incoming.TgID != 0 {
		if incomingNewer || existing.TgID == 0 {
			keep("tgId", existing.TgID, incoming.TgID, incoming.TgID)
			existing.TgID = incoming.TgID
		}
	}
	if existing.Reset != incoming.Reset && incoming.Reset != 0 {
		if incomingNewer || existing.Reset == 0 {
			keep("reset", existing.Reset, incoming.Reset, incoming.Reset)
			existing.Reset = incoming.Reset
		}
	}
	if existing.Reverse != incoming.Reverse && incoming.Reverse != "" {
		if incomingNewer || existing.Reverse == "" {
			keep("reverse", existing.Reverse, incoming.Reverse, incoming.Reverse)
			existing.Reverse = incoming.Reverse
		}
	}
	if existing.Comment != incoming.Comment && incoming.Comment != "" {
		if incomingNewer || existing.Comment == "" {
			keep("comment", existing.Comment, incoming.Comment, incoming.Comment)
			existing.Comment = incoming.Comment
		}
	}
	if existing.Group != incoming.Group && incoming.Group != "" {
		if incomingNewer || existing.Group == "" {
			keep("group", existing.Group, incoming.Group, incoming.Group)
			existing.Group = incoming.Group
		}
	}
	if existing.Enable != incoming.Enable {
		if incoming.Enable {
			if !existing.Enable {
				keep("enable", existing.Enable, incoming.Enable, true)
				existing.Enable = true
			}
		}
	}
	if incoming.CreatedAt != 0 && (existing.CreatedAt == 0 || incoming.CreatedAt < existing.CreatedAt) {
		existing.CreatedAt = incoming.CreatedAt
	}
	if incoming.UpdatedAt > existing.UpdatedAt {
		existing.UpdatedAt = incoming.UpdatedAt
	}
	return conflicts
}
