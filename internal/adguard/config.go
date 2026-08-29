package adguard

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// AdminUser is the account the seeded config creates. AdGuard Home has no
// concept of a default user, so one has to exist before its UI is reachable.
const AdminUser = "admin"

// Thresholds AdGuard Home itself ships with (auth_attempts / block_auth_min in
// its own source). Matching them exactly matters here: the login page is
// public, so a lockout that behaves unlike the real product is a tell.
const (
	authAttempts = 5
	authBlockMin = 15
)

// schemaVersion is AdGuard Home's current config schema (LastSchemaVersion in
// its configmigrate package). Seeding the current one means it starts against
// the config as written, with no migration pass. A newer AdGuard Home migrates
// this forward on its own; only a config newer than the binary is refused.
const schemaVersion = 34

// dohRoutes are the endpoints AdGuard Home answers DNS-over-HTTPS on, listed
// explicitly because schema 34 made them configurable rather than implicit.
const dohRoutes = `      - 'GET /dns-query'
      - 'POST /dns-query'
      - 'GET /dns-query/{ClientID}'
      - 'POST /dns-query/{ClientID}'`

// defaultFilter is AdGuard's own blocklist, from their hostlists registry. A
// wizard-less install starts with no filters at all, which would leave the
// cover story a resolver that blocks nothing.
const defaultFilter = "https://adguardteam.github.io/HostlistsRegistry/assets/filter_1.txt"

// SeedOptions are the loopback ports the caller allocated for this install.
type SeedOptions struct {
	WebPort int
	DNSPort int
}

// Seed writes AdGuardHome.yaml and returns the generated admin password.
//
// An existing config is never touched -- AdGuard Home rewrites that file
// itself whenever the admin changes anything in its UI, so overwriting it
// would silently discard their filters, clients and password. Returns an empty
// password in that case, meaning "kept what was already there".
func Seed(opts SeedOptions) (string, error) {
	if _, err := os.Stat(ConfigPath()); err == nil {
		return "", nil
	}
	if opts.WebPort <= 0 || opts.WebPort > 65535 {
		return "", fmt.Errorf("invalid AdGuard Home web port %d", opts.WebPort)
	}
	if opts.DNSPort <= 0 || opts.DNSPort > 65535 {
		return "", fmt.Errorf("invalid AdGuard Home DNS port %d", opts.DNSPort)
	}
	password, err := generatePassword()
	if err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing the AdGuard Home password: %w", err)
	}
	if err := os.MkdirAll(Dir(), 0o750); err != nil {
		return "", fmt.Errorf("cannot create %s: %w", Dir(), err)
	}
	if err := os.WriteFile(ConfigPath(), []byte(renderConfig(opts, string(hash))), 0o600); err != nil {
		return "", fmt.Errorf("cannot write %s: %w", ConfigPath(), err)
	}
	return password, nil
}

// renderConfig builds the seed config.
//
// Deliberately partial: AdGuard Home unmarshals this over its own defaults, so
// every key left out keeps the upstream default and cannot go stale against a
// future release. Only what this integration actually depends on is pinned.
//
// Both listeners are loopback-only. That is what keeps the DNS side from
// becoming an open resolver -- the public way in is DNS-over-HTTPS through the
// reverse proxy, which terminates TLS and forwards here in the clear, hence
// insecure_enabled.
func renderConfig(opts SeedOptions, passwordHash string) string {
	return fmt.Sprintf(`http:
  address: 127.0.0.1:%d
  doh:
    insecure_enabled: true
    routes:
%s
users:
  - name: %s
    password: %q
auth_attempts: %d
block_auth_min: %d
theme: auto
dns:
  bind_hosts:
    - 127.0.0.1
  port: %d
  trusted_proxies:
    - 127.0.0.0/8
    - ::1/128
filters:
  - enabled: true
    url: %s
    name: AdGuard DNS filter
    id: 1
schema_version: %d
`, opts.WebPort, dohRoutes, AdminUser, passwordHash, authAttempts, authBlockMin,
		opts.DNSPort, defaultFilter, schemaVersion)
}

// passwordAlphabet leaves out characters that survive a copy-paste badly or
// read ambiguously in the panel's monospace font.
const passwordAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// generatePassword mints the admin password. It is shown once by the panel and
// stored only as a bcrypt hash on this side.
func generatePassword() (string, error) {
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordAlphabet))))
		if err != nil {
			return "", fmt.Errorf("generating the AdGuard Home password: %w", err)
		}
		sb.WriteByte(passwordAlphabet[n.Int64()])
	}
	return sb.String(), nil
}
