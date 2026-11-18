package ha

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Environment variables of the HA mode (docs/master-failover.md).
const (
	EnvNode       = "XUI_HA_NODE"
	EnvListen     = "XUI_HA_LISTEN"
	EnvPeers      = "XUI_HA_PEERS"
	EnvSecret     = "XUI_HA_SECRET"
	EnvPatroniURL = "XUI_HA_PATRONI_URL"
	EnvPanelURL   = "XUI_HA_PANEL_URL"
	EnvSubURL     = "XUI_HA_SUB_URL"
	EnvFenceAfter = "XUI_HA_FENCE_AFTER"
	EnvMasterHold = "XUI_HA_MASTER_HOLD"
	EnvInterval   = "XUI_HA_INTERVAL"
)

// Settings is the HA configuration of this machine.
type Settings struct {
	Node       string
	Listen     string
	Peers      map[string]string
	Secret     string
	PatroniURL string
	Self       Self
	FenceAfter time.Duration
	MasterHold time.Duration
	Interval   time.Duration
}

// Enabled reports whether the panel runs in HA mode.
func (s Settings) Enabled() bool { return s.Node != "" }

// RoleConfig is the automaton configuration derived from the settings.
func (s Settings) RoleConfig() Config {
	return Config{
		Self: s.Node, Peers: PeerCodes(s.Peers), RequirePatroni: s.PatroniURL != "",
		FenceAfter: s.FenceAfter, MasterHold: s.MasterHold,
	}
}

// SettingsFromEnv reads the XUI_HA_* variables; an empty XUI_HA_NODE turns HA off.
func SettingsFromEnv() (Settings, error) {
	s := Settings{Node: strings.TrimSpace(os.Getenv(EnvNode))}
	if !s.Enabled() {
		return Settings{}, nil
	}
	s.Listen = strings.TrimSpace(os.Getenv(EnvListen))
	if s.Listen == "" {
		return Settings{}, fmt.Errorf("%s is required in HA mode", EnvListen)
	}
	peers, err := ParsePeers(os.Getenv(EnvPeers))
	if err != nil {
		return Settings{}, fmt.Errorf("%s: %w", EnvPeers, err)
	}
	if _, self := peers[s.Node]; self {
		return Settings{}, fmt.Errorf("%s lists this machine (%s) itself", EnvPeers, s.Node)
	}
	s.Peers = peers
	s.Secret = os.Getenv(EnvSecret)
	if len(peers) > 0 && s.Secret == "" {
		return Settings{}, fmt.Errorf("%s is required when %s is set", EnvSecret, EnvPeers)
	}
	s.PatroniURL = strings.TrimSpace(os.Getenv(EnvPatroniURL))
	s.Self = Self{
		PanelURL: strings.TrimRight(strings.TrimSpace(os.Getenv(EnvPanelURL)), "/"),
		SubURL:   strings.TrimRight(strings.TrimSpace(os.Getenv(EnvSubURL)), "/"),
	}
	for _, d := range []struct {
		dst  *time.Duration
		name string
		def  time.Duration
	}{
		{&s.FenceAfter, EnvFenceAfter, 10 * time.Second},
		{&s.MasterHold, EnvMasterHold, 5 * time.Second},
		{&s.Interval, EnvInterval, 2 * time.Second},
	} {
		if *d.dst, err = envDuration(d.name, d.def); err != nil {
			return Settings{}, err
		}
	}
	return s, nil
}

func envDuration(name string, def time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %s", name, raw)
	}
	return d, nil
}
