package model

import "slices"

const (
	DefaultWireguardDeviceLimit = 5
	MaxWireguardDevices         = 32
)

// WireguardDevice is an additional peer sharing its client's quota and lifecycle.
// The original client credentials remain the primary device for compatibility.
type WireguardDevice struct {
	Id           string   `json:"id"`
	Name         string   `json:"name"`
	Enable       bool     `json:"enable"`
	PrivateKey   string   `json:"privateKey"`
	PublicKey    string   `json:"publicKey"`
	PreSharedKey string   `json:"preSharedKey,omitempty"`
	AllowedIPs   []string `json:"allowedIPs"`
}

func CloneWireguardDevices(devices []WireguardDevice) []WireguardDevice {
	result := slices.Clone(devices)
	for i := range result {
		result[i].AllowedIPs = slices.Clone(result[i].AllowedIPs)
	}
	return result
}

func EffectiveWireguardDeviceLimit(limit int) int {
	if limit <= 0 {
		return DefaultWireguardDeviceLimit
	}
	return limit
}

// WireguardClients expands enabled peers while retaining parent attribution.
// Callers enforce the parent client's enable and traffic/expiry limits.
func WireguardClients(c Client) []Client {
	devices := c.WireguardDevices
	c.WireguardDevices = nil
	clients := []Client{c}
	for _, device := range devices {
		if !device.Enable {
			continue
		}
		peer := c
		peer.PrivateKey = device.PrivateKey
		peer.PublicKey = device.PublicKey
		peer.PreSharedKey = device.PreSharedKey
		peer.AllowedIPs = slices.Clone(device.AllowedIPs)
		clients = append(clients, peer)
	}
	return clients
}

func WireguardPeersFromClient(c Client) []any {
	clients := WireguardClients(c)
	peers := make([]any, 0, len(clients))
	for _, client := range clients {
		peers = append(peers, WireguardPeerFromClient(client))
	}
	return peers
}

// WireguardAllowedIPs includes disabled devices because their IPs stay reserved.
func WireguardAllowedIPs(c Client) []string {
	addresses := slices.Clone(c.AllowedIPs)
	for _, device := range c.WireguardDevices {
		addresses = append(addresses, device.AllowedIPs...)
	}
	return addresses
}
