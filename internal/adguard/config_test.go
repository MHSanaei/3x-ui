package adguard

import (
	"os"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"golang.org/x/crypto/bcrypt"
)

// seededConfig covers exactly the keys this integration depends on. Every one
// of them was checked against AdGuard Home's own config struct; the point of
// this test is to notice if a future edit silently stops emitting one, since
// AdGuard Home ignores keys it does not recognize rather than complaining.
type seededConfig struct {
	HTTP struct {
		Address string `yaml:"address"`
		DoH     struct {
			InsecureEnabled bool     `yaml:"insecure_enabled"`
			Routes          []string `yaml:"routes"`
		} `yaml:"doh"`
	} `yaml:"http"`
	Users []struct {
		Name     string `yaml:"name"`
		Password string `yaml:"password"`
	} `yaml:"users"`
	AuthAttempts int `yaml:"auth_attempts"`
	BlockAuthMin int `yaml:"block_auth_min"`
	DNS          struct {
		BindHosts []string `yaml:"bind_hosts"`
		Port      int      `yaml:"port"`
	} `yaml:"dns"`
	SchemaVersion int `yaml:"schema_version"`
}

func parseSeeded(t *testing.T, raw string) seededConfig {
	t.Helper()
	var cfg seededConfig
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("rendered config is not valid YAML: %v\n%s", err, raw)
	}
	return cfg
}

func TestRenderConfigPinsWhatTheIntegrationNeeds(t *testing.T) {
	cfg := parseSeeded(t, renderConfig(SeedOptions{WebPort: 3000, DNSPort: 5335}, "$2a$10$abcdefghijklmnopqrstuv"))

	if cfg.HTTP.Address != "127.0.0.1:3000" {
		t.Errorf("http.address = %q, want 127.0.0.1:3000", cfg.HTTP.Address)
	}
	// Without this the reverse proxy's plaintext loopback hop is refused and
	// DNS-over-HTTPS silently stops answering.
	if !cfg.HTTP.DoH.InsecureEnabled {
		t.Error("http.doh.insecure_enabled = false, want true")
	}
	if len(cfg.HTTP.DoH.Routes) != 4 {
		t.Errorf("http.doh.routes has %d entries, want 4: %v", len(cfg.HTTP.DoH.Routes), cfg.HTTP.DoH.Routes)
	}
	// Loopback-only is what keeps this from being an open resolver.
	if len(cfg.DNS.BindHosts) != 1 || cfg.DNS.BindHosts[0] != "127.0.0.1" {
		t.Errorf("dns.bind_hosts = %v, want [127.0.0.1]", cfg.DNS.BindHosts)
	}
	if cfg.DNS.Port != 5335 {
		t.Errorf("dns.port = %d, want 5335", cfg.DNS.Port)
	}
	if cfg.AuthAttempts != authAttempts || cfg.BlockAuthMin != authBlockMin {
		t.Errorf("lockout = %d/%dmin, want %d/%dmin", cfg.AuthAttempts, cfg.BlockAuthMin, authAttempts, authBlockMin)
	}
	if cfg.SchemaVersion != schemaVersion {
		t.Errorf("schema_version = %d, want %d", cfg.SchemaVersion, schemaVersion)
	}
	if len(cfg.Users) != 1 || cfg.Users[0].Name != AdminUser {
		t.Fatalf("users = %+v, want one %q", cfg.Users, AdminUser)
	}
}

// TestSeedWritesAConfigTheGeneratedPasswordOpens is the one that matters for
// the admin: a hash that does not verify means a login nobody can pass.
func TestSeedWritesAConfigTheGeneratedPasswordOpens(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())

	password, err := Seed(SeedOptions{WebPort: 3000, DNSPort: 5335})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if password == "" {
		t.Fatal("Seed returned no password for a fresh install")
	}

	raw, err := os.ReadFile(ConfigPath())
	if err != nil {
		t.Fatalf("reading the seeded config: %v", err)
	}
	cfg := parseSeeded(t, string(raw))
	if len(cfg.Users) != 1 {
		t.Fatalf("users = %+v, want exactly one", cfg.Users)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(cfg.Users[0].Password), []byte(password)); err != nil {
		t.Errorf("the returned password does not open the stored hash: %v", err)
	}
}

func TestSeedKeepsAnExistingConfig(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	if err := os.MkdirAll(Dir(), 0o750); err != nil {
		t.Fatalf("preparing the directory: %v", err)
	}
	existing := "http:\n  address: 127.0.0.1:9999\n"
	if err := os.WriteFile(ConfigPath(), []byte(existing), 0o600); err != nil {
		t.Fatalf("writing the existing config: %v", err)
	}

	password, err := Seed(SeedOptions{WebPort: 3000, DNSPort: 5335})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	// AdGuard Home owns this file once it exists: overwriting would discard
	// the admin's own filters, clients and password.
	if password != "" {
		t.Errorf("Seed returned a new password %q for an existing config", password)
	}
	raw, err := os.ReadFile(ConfigPath())
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if string(raw) != existing {
		t.Errorf("existing config was rewritten:\n%s", raw)
	}
}

func TestSeedRejectsUnusablePorts(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts SeedOptions
	}{
		{"zero web port", SeedOptions{WebPort: 0, DNSPort: 5335}},
		{"zero dns port", SeedOptions{WebPort: 3000, DNSPort: 0}},
		{"web port out of range", SeedOptions{WebPort: 70000, DNSPort: 5335}},
		{"dns port out of range", SeedOptions{WebPort: 3000, DNSPort: -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XUI_BIN_FOLDER", t.TempDir())
			if _, err := Seed(tc.opts); err == nil {
				t.Error("Seed accepted an unusable port")
			}
			if _, err := os.Stat(ConfigPath()); err == nil {
				t.Error("Seed wrote a config despite rejecting the ports")
			}
		})
	}
}

func TestGeneratePasswordIsRandomAndFromTheStatedAlphabet(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 25; i++ {
		password, err := generatePassword()
		if err != nil {
			t.Fatalf("generatePassword: %v", err)
		}
		if len(password) != 20 {
			t.Fatalf("password %q is %d characters, want 20", password, len(password))
		}
		for _, r := range password {
			if !strings.ContainsRune(passwordAlphabet, r) {
				t.Fatalf("password %q contains %q, which is outside the alphabet", password, r)
			}
		}
		if seen[password] {
			t.Fatalf("generatePassword repeated %q", password)
		}
		seen[password] = true
	}
}
