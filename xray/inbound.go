package xray

import (
	"bytes"

	"github.com/mhsanaei/3x-ui/v2/util/json_util"
)

// InboundConfig represents an Xray inbound configuration.
// It defines how Xray accepts incoming connections including protocol, port, and settings.
type InboundConfig struct {
	Listen         json_util.RawMessage `json:"listen"` // listen cannot be an empty string
	Port           int                  `json:"port"`
	Protocol       string               `json:"protocol"`
	Settings       json_util.RawMessage `json:"settings"`
	StreamSettings json_util.RawMessage `json:"streamSettings"`
	Tag            string               `json:"tag"`
	Sniffing       json_util.RawMessage `json:"sniffing"`
}

// Equals compares two InboundConfig instances for deep equality.
func (c *InboundConfig) Equals(other *InboundConfig) bool {
	if !bytes.Equal(c.Listen, other.Listen) {
		return false
	}
	if c.Port != other.Port {
		return false
	}
	if c.Protocol != other.Protocol {
		return false
	}
	if !bytes.Equal(c.Settings, other.Settings) {
		return false
	}
	if !bytes.Equal(c.StreamSettings, other.StreamSettings) {
		return false
	}
	if c.Tag != other.Tag {
		return false
	}
	if !bytes.Equal(c.Sniffing, other.Sniffing) {
		return false
	}
	return true
}
