package ha

import (
	"strings"
	"testing"
	"time"
)

func setHAEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range []string{EnvNode, EnvListen, EnvPeers, EnvSecret, EnvPatroniURL, EnvPanelURL, EnvSubURL, EnvFenceAfter, EnvMasterHold, EnvInterval} {
		t.Setenv(k, env[k])
	}
}

func TestSettingsFromEnvDisabledWithoutNode(t *testing.T) {
	setHAEnv(t, map[string]string{EnvListen: ":2097"})
	st, err := SettingsFromEnv()
	if err != nil || st.Enabled() {
		t.Fatalf("no %s must leave HA off, got %+v %v", EnvNode, st, err)
	}
}

func TestSettingsFromEnvDefaults(t *testing.T) {
	setHAEnv(t, map[string]string{
		EnvNode: "de", EnvListen: ":2097", EnvPeers: "nl=https://nl:2097,pl=https://pl:2097", EnvSecret: "s",
		EnvPatroniURL: "http://127.0.0.1:8008", EnvPanelURL: "https://de:2053/", EnvSubURL: "https://de:2096",
	})
	st, err := SettingsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	cfg := st.RoleConfig()
	if !st.Enabled() || cfg.Self != "de" || strings.Join(cfg.Peers, ",") != "nl,pl" || !cfg.RequirePatroni {
		t.Fatalf("got %+v", cfg)
	}
	if cfg.FenceAfter != 10*time.Second || cfg.MasterHold != 5*time.Second || st.Interval != 2*time.Second {
		t.Fatalf("defaults: %+v interval %s", cfg, st.Interval)
	}
	if st.Self.PanelURL != "https://de:2053" {
		t.Fatalf("redirect base must drop the trailing slash, got %q", st.Self.PanelURL)
	}
}

func TestSettingsFromEnvRejects(t *testing.T) {
	base := map[string]string{EnvNode: "de", EnvListen: ":2097", EnvPeers: "nl=https://nl:2097", EnvSecret: "s"}
	cases := []struct {
		name, key, val, wantErr string
	}{
		{"no listen", EnvListen, "", EnvListen},
		{"peers without secret", EnvSecret, "", EnvSecret},
		{"self listed as peer", EnvPeers, "de=https://de:2097", "itself"},
		{"bad peers", EnvPeers, "nl", "code=url"},
		{"bad duration", EnvFenceAfter, "ten", EnvFenceAfter},
		{"non-positive duration", EnvMasterHold, "0s", EnvMasterHold},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := map[string]string{}
			for k, v := range base {
				env[k] = v
			}
			env[c.key] = c.val
			setHAEnv(t, env)
			if _, err := SettingsFromEnv(); err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("err = %v, want it to mention %q", err, c.wantErr)
			}
		})
	}
}
