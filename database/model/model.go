// Package model defines the database models and data structures used by the 3x-ui panel.
package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/xray"
)

// Protocol represents the protocol type for Xray inbounds.
type Protocol string

// Protocol constants for different Xray inbound protocols
const (
	VMESS        Protocol = "vmess"
	VLESS        Protocol = "vless"
	Tunnel       Protocol = "tunnel"
	HTTP         Protocol = "http"
	Trojan       Protocol = "trojan"
	Shadowsocks  Protocol = "shadowsocks"
	Mixed        Protocol = "mixed"
	WireGuard    Protocol = "wireguard"
	Hysteria     Protocol = "hysteria"
	Hysteria2    Protocol = "hysteria2"
)

// IsHysteria returns true for both "hysteria" and "hysteria2".
// Use instead of a bare ==model.Hysteria check: a v2 inbound stored
// with the literal v2 string would otherwise fall through (#4081).
func IsHysteria(p Protocol) bool {
	return p == Hysteria || p == Hysteria2
}

// User represents a user account in the 3x-ui panel.
type User struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	LoginEpoch int64  `json:"-" gorm:"default:0"`
}

// Inbound represents an Xray inbound configuration with traffic statistics and settings.
type Inbound struct {
	Id                   int                  `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`                                                    // Unique identifier
	UserId               int                  `json:"-"`                                                                                               // Associated user ID
	Up                   int64                `json:"up" form:"up"`                                                                                    // Upload traffic in bytes
	Down                 int64                `json:"down" form:"down"`                                                                                // Download traffic in bytes
	Total                int64                `json:"total" form:"total"`                                                                              // Total traffic limit in bytes
	Remark               string               `json:"remark" form:"remark"`                                                                            // Human-readable remark
	Enable               bool                 `json:"enable" form:"enable" gorm:"index:idx_enable_traffic_reset,priority:1"`                           // Whether the inbound is enabled
	ExpiryTime           int64                `json:"expiryTime" form:"expiryTime"`                                                                    // Expiration timestamp
	TrafficReset         string               `json:"trafficReset" form:"trafficReset" gorm:"default:never;index:idx_enable_traffic_reset,priority:2"` // Traffic reset schedule
	LastTrafficResetTime int64                `json:"lastTrafficResetTime" form:"lastTrafficResetTime" gorm:"default:0"`                               // Last traffic reset timestamp
	ClientStats          []xray.ClientTraffic `gorm:"foreignKey:InboundId;references:Id" json:"clientStats" form:"clientStats"`                        // Client traffic statistics

	// Xray configuration fields
	Listen         string   `json:"listen" form:"listen"`
	Port           int      `json:"port" form:"port"`
	Protocol       Protocol `json:"protocol" form:"protocol"`
	Settings       string   `json:"settings" form:"settings"`
	StreamSettings string   `json:"streamSettings" form:"streamSettings"`
	Tag            string   `json:"tag" form:"tag" gorm:"unique"`
	Sniffing       string   `json:"sniffing" form:"sniffing"`
	NodeID         *int     `json:"nodeId,omitempty" form:"nodeId" gorm:"index"`

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
	Token     string `json:"token" gorm:"not null"`
	Enabled   bool   `json:"enabled" gorm:"default:true"`
	CreatedAt int64  `json:"createdAt" gorm:"autoCreateTime"`
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
	return &xray.InboundConfig{
		Listen:         json_util.RawMessage(listen),
		Port:           i.Port,
		Protocol:       protocol,
		Settings:       json_util.RawMessage(i.Settings),
		StreamSettings: json_util.RawMessage(i.StreamSettings),
		Tag:            i.Tag,
		Sniffing:       json_util.RawMessage(i.Sniffing),
	}
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
	Id                  int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Name                string `json:"name" form:"name" gorm:"uniqueIndex"`
	Remark              string `json:"remark" form:"remark"`
	Scheme              string `json:"scheme" form:"scheme"`
	Address             string `json:"address" form:"address"`
	Port                int    `json:"port" form:"port"`
	BasePath            string `json:"basePath" form:"basePath"`
	ApiToken            string `json:"apiToken" form:"apiToken"`
	Enable              bool   `json:"enable" form:"enable" gorm:"default:true"`
	AllowPrivateAddress bool   `json:"allowPrivateAddress" form:"allowPrivateAddress" gorm:"default:false"`

	// Heartbeat-updated fields. UpdatedAt advances on every probe even when
	// the row is otherwise unchanged so the UI's "last seen" tooltip is
	// truthful without us having to read LastHeartbeat separately.
	Status        string  `json:"status" gorm:"default:unknown"` // online|offline|unknown
	LastHeartbeat int64   `json:"lastHeartbeat"`                 // unix seconds, 0 = never
	LatencyMs     int     `json:"latencyMs"`
	XrayVersion   string  `json:"xrayVersion"`
	PanelVersion  string  `json:"panelVersion" gorm:"column:panel_version"`
	CpuPct        float64 `json:"cpuPct"`
	MemPct        float64 `json:"memPct"`
	UptimeSecs    uint64  `json:"uptimeSecs"`
	LastError     string  `json:"lastError"`

	InboundCount  int `json:"inboundCount" gorm:"-"`
	ClientCount   int `json:"clientCount" gorm:"-"`
	OnlineCount   int `json:"onlineCount" gorm:"-"`
	DepletedCount int `json:"depletedCount" gorm:"-"`

	CreatedAt int64 `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt int64 `json:"updatedAt" gorm:"autoUpdateTime"`
}

type CustomGeoResource struct {
	Id            int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Type          string `json:"type" gorm:"not null;uniqueIndex:idx_custom_geo_type_alias;column:geo_type"`
	Alias         string `json:"alias" gorm:"not null;uniqueIndex:idx_custom_geo_type_alias"`
	Url           string `json:"url" gorm:"not null"`
	LocalPath     string `json:"localPath" gorm:"column:local_path"`
	LastUpdatedAt int64  `json:"lastUpdatedAt" gorm:"default:0;column:last_updated_at"`
	LastModified  string `json:"lastModified" gorm:"column:last_modified"`
	CreatedAt     int64  `json:"createdAt" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt     int64  `json:"updatedAt" gorm:"autoUpdateTime;column:updated_at"`
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
	Comment    string `json:"comment"`
	Reset      int    `json:"reset" gorm:"default:0"`
	CreatedAt  int64  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt  int64  `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (ClientRecord) TableName() string { return "clients" }

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
	CreatedAt    int64  `json:"createdAt" gorm:"autoCreateTime"`
}

func (ClientInbound) TableName() string { return "client_inbounds" }

// InboundFallback is one routing rule on a master inbound's
// settings.fallbacks array. The master is always a VLESS or Trojan
// inbound on TCP transport with TLS or Reality. The child is any other
// inbound — its listen+port becomes the fallback dest, with optional
// SNI/ALPN/path match criteria pulled from the same row.
type InboundFallback struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	MasterId  int    `json:"masterId" gorm:"index;not null;column:master_id"`
	ChildId   int    `json:"childId" gorm:"index;not null;column:child_id"`
	Name      string `json:"name"`
	Alpn      string `json:"alpn"`
	Path      string `json:"path"`
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
