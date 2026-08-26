package tuic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

type ServerConfig struct {
	Server                string            `json:"server"`
	Users                 map[string]string `json:"users"`
	Certificate           string            `json:"certificate"`
	PrivateKey            string            `json:"private_key"`
	CongestionControl     string            `json:"congestion_control"`
	ALPN                  []string          `json:"alpn"`
	UDPRelayMode          string            `json:"udp_relay_mode,omitempty"`
	ZeroRTTHandshake      bool              `json:"zero_rtt_handshake"`
	LogLevel              string            `json:"log_level"`
	MaxIdleTime           string            `json:"max_idle_time,omitempty"`
	AuthTimeout           string            `json:"auth_timeout,omitempty"`
	MaxExternalPacketSize int               `json:"max_external_packet_size,omitempty"`
}

func GenerateConfig(inst Instance) ([]byte, error) {
	users := make(map[string]string, len(inst.Clients))
	for _, c := range inst.Clients {
		if c.UUID != "" && c.Password != "" {
			users[c.UUID] = c.Password
		}
	}

	authTimeoutStr := ""
	if inst.AuthenticationTimeout > 0 {
		authTimeoutStr = fmt.Sprintf("%ds", inst.AuthenticationTimeout)
	}

	maxIdleStr := ""
	if inst.MaxIdleTime > 0 {
		maxIdleStr = fmt.Sprintf("%ds", inst.MaxIdleTime)
	}

	cfg := ServerConfig{
		Server:                inst.BindTo(),
		Users:                 users,
		Certificate:           inst.Certificate,
		PrivateKey:            inst.PrivateKey,
		CongestionControl:     inst.CongestionControl,
		ALPN:                  inst.ALPN,
		UDPRelayMode:          inst.UDPRelayMode,
		ZeroRTTHandshake:      inst.ZeroRTTHandshake,
		LogLevel:              inst.LogLevel,
		AuthTimeout:           authTimeoutStr,
		MaxIdleTime:           maxIdleStr,
		MaxExternalPacketSize: inst.MaxUdpRelayPacketSize,
	}

	return json.MarshalIndent(cfg, "", "  ")
}

func ConfigDir() string {
	return filepath.Join(config.GetBinFolderPath(), "tuic")
}

func ConfigPathForID(id int) string {
	return filepath.Join(ConfigDir(), fmt.Sprintf("tuic_%d.json", id))
}

func WriteConfigFile(id int, data []byte) (string, error) {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := ConfigPathForID(id)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func RemoveConfigFile(id int) error {
	path := ConfigPathForID(id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
