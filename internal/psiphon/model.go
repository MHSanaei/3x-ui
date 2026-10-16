package psiphon

import (
	"strings"
	"time"
)

// InstanceStatus defines runtime states of a Psiphon sidecar instance.
const (
	StatusStopped   = "stopped"
	StatusStarting  = "starting"
	StatusConnected = "connected"
	StatusError     = "error"
)

// DefaultPortRange defines loopback SOCKS port boundaries.
const (
	DefaultBasePort = 40001
	DefaultMaxPort  = 40099
)

// Instance holds the runtime representation of one running Psiphon tunnel.
type Instance struct {
	ID          string `json:"id" example:"US"`
	CountryCode string `json:"countryCode" example:"US"`
	CountryName string `json:"countryName" example:"United States"`
	SocksPort   int    `json:"socksPort" example:"40001"`
	Status      string `json:"status" example:"connected"`
	ExitIP      string `json:"exitIp" example:"1.2.3.4"`
	LatencyMs   int    `json:"latencyMs" example:"142"`
	LastTested  int64  `json:"lastTested" example:"1710000000000"`
	Enabled     bool   `json:"enabled" example:"true"`
}

// Country represents an egress country option.
type Country struct {
	Code      string `json:"code" example:"US"`
	Name      string `json:"name" example:"United States"`
	Flag      string `json:"flag" example:"🇺🇸"`
	Added     bool   `json:"added" example:"false"`
	Available bool   `json:"available" example:"true"`
}

// PsiphonConfig holds panel-wide Psiphon settings.
type PsiphonConfig struct {
	ConfigJSON  string `json:"configJson" example:"{}"`
	BasePort    int    `json:"basePort" example:"40001"`
	AutoRestart bool   `json:"autoRestart" example:"true"`
}

// Notice represents a structured JSON event from Psiphon ConsoleClient.
type Notice struct {
	NoticeType string         `json:"noticeType"`
	Timestamp  string         `json:"timestamp"`
	Data       map[string]any `json:"data"`
}

// StaticCountries provides the standard Psiphon region catalog.
var StaticCountries = map[string]string{
	"AT": "Austria",
	"AU": "Australia",
	"BE": "Belgium",
	"BG": "Bulgaria",
	"CA": "Canada",
	"CH": "Switzerland",
	"CZ": "Czechia",
	"DE": "Germany",
	"DK": "Denmark",
	"EE": "Estonia",
	"ES": "Spain",
	"FI": "Finland",
	"FR": "France",
	"GB": "United Kingdom",
	"HU": "Hungary",
	"IE": "Ireland",
	"IN": "India",
	"IT": "Italy",
	"JP": "Japan",
	"LT": "Lithuania",
	"LV": "Latvia",
	"NL": "Netherlands",
	"NO": "Norway",
	"PL": "Poland",
	"PT": "Portugal",
	"RO": "Romania",
	"RS": "Serbia",
	"SE": "Sweden",
	"SG": "Singapore",
	"SK": "Slovakia",
	"US": "United States",
}

// CountryFlag turns a two-letter ISO country code into a regional indicator emoji.
func CountryFlag(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 2 {
		return "🌐"
	}
	r1 := rune(code[0]) - 'A' + 0x1F1E6
	r2 := rune(code[1]) - 'A' + 0x1F1E6
	if r1 < 0x1F1E6 || r1 > 0x1F1FF || r2 < 0x1F1E6 || r2 > 0x1F1FF {
		return "🌐"
	}
	return string([]rune{r1, r2})
}

// CountryName returns the human-readable country name for a code.
func CountryName(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if name, ok := StaticCountries[code]; ok {
		return name
	}
	if code == "" {
		return "Best / Auto"
	}
	return code
}

// ProbeResult stores latency and egress verification measurements.
type ProbeResult struct {
	OK          bool   `json:"ok"`
	LatencyMs   int    `json:"latencyMs"`
	ExitIP      string `json:"exitIp,omitempty"`
	ExitCountry string `json:"exitCountry,omitempty"`
	Error       string `json:"error,omitempty"`
}

// PingRequest defines the payload for ping requests.
type PingRequest struct {
	InstanceID string `json:"instanceId"`
}

// BulkAddRequest defines the payload for adding multiple countries.
type BulkAddRequest struct {
	Countries []string `json:"countries" example:"[\"US\",\"DE\"]"`
}

// UpdateConfigRequest defines the payload for saving Psiphon configuration.
type UpdateConfigRequest struct {
	ConfigJSON string `json:"configJson"`
}

// FormatTimeMillis formats a timestamp in milliseconds.
func FormatTimeMillis(t time.Time) int64 {
	return t.UnixMilli()
}
