package adguard

import (
	"os"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"golang.org/x/crypto/bcrypt"
)

func TestValidateCredentials(t *testing.T) {
	for _, tc := range []struct {
		name     string
		user     string
		password string
		wantErr  bool
	}{
		{name: "accepts a normal pair", user: "admin", password: "correct horse"},
		{name: "rejects an empty user", user: "", password: "longenough", wantErr: true},
		{name: "rejects a padded user", user: " admin ", password: "longenough", wantErr: true},
		{name: "rejects an overlong user", user: strings.Repeat("a", maxUserLength+1), password: "longenough", wantErr: true},
		{name: "rejects a short password", user: "admin", password: "short", wantErr: true},
		{name: "rejects an overlong password", user: "admin", password: strings.Repeat("a", maxPasswordLength+1), wantErr: true},
		// A newline would end the YAML scalar and let the rest be read as config.
		{name: "rejects a newline in the user", user: "admin\nport: 9999", password: "longenough", wantErr: true},
		{name: "rejects a newline in the password", user: "admin", password: "longenough\nx: y", wantErr: true},
		{name: "rejects a control character", user: "adm\x00in", password: "longenough", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCredentials(tc.user, tc.password)
			if tc.wantErr && err == nil {
				t.Error("ValidateCredentials accepted input it should have refused")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidateCredentials: %v", err)
			}
		})
	}
}

// realisticConfig stands in for a file AdGuard Home has already rewritten
// itself, including keys this package knows nothing about.
const realisticConfig = `http:
  address: 127.0.0.1:3000
  doh:
    insecure_enabled: true
users:
  - name: admin
    password: "$2a$10$oldoldoldoldoldoldoldold"
auth_attempts: 5
block_auth_min: 15
theme: auto
dns:
  bind_hosts:
    - 127.0.0.1
  port: 5335
  ratelimit: 20
querylog:
  enabled: true
  interval: 2160h
schema_version: 34
`

// TestReplaceUsersLeavesEverythingElseAlone is the point of doing this against
// the parsed document instead of re-marshalling: a setting AdGuard Home wrote
// and this package does not model must survive a password change untouched.
func TestReplaceUsersLeavesEverythingElseAlone(t *testing.T) {
	out, err := replaceUsers([]byte(realisticConfig), "newadmin", "$2a$10$newnewnewnewnewnewnewnew")
	if err != nil {
		t.Fatalf("replaceUsers: %v", err)
	}

	var got struct {
		HTTP struct {
			Address string `yaml:"address"`
			DoH     struct {
				InsecureEnabled bool `yaml:"insecure_enabled"`
			} `yaml:"doh"`
		} `yaml:"http"`
		Users []struct {
			Name     string `yaml:"name"`
			Password string `yaml:"password"`
		} `yaml:"users"`
		AuthAttempts int    `yaml:"auth_attempts"`
		Theme        string `yaml:"theme"`
		DNS          struct {
			BindHosts []string `yaml:"bind_hosts"`
			Port      int      `yaml:"port"`
			Ratelimit int      `yaml:"ratelimit"`
		} `yaml:"dns"`
		QueryLog struct {
			Enabled  bool   `yaml:"enabled"`
			Interval string `yaml:"interval"`
		} `yaml:"querylog"`
		SchemaVersion int `yaml:"schema_version"`
	}
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatalf("the rewritten config is not valid YAML: %v\n%s", err, out)
	}

	if len(got.Users) != 1 {
		t.Fatalf("users = %+v, want exactly one", got.Users)
	}
	if got.Users[0].Name != "newadmin" {
		t.Errorf("users[0].name = %q, want newadmin", got.Users[0].Name)
	}
	if got.Users[0].Password != "$2a$10$newnewnewnewnewnewnewnew" {
		t.Errorf("users[0].password = %q, want the new hash", got.Users[0].Password)
	}

	if got.HTTP.Address != "127.0.0.1:3000" || !got.HTTP.DoH.InsecureEnabled {
		t.Errorf("http block changed: %+v", got.HTTP)
	}
	if got.AuthAttempts != 5 || got.Theme != "auto" || got.SchemaVersion != 34 {
		t.Errorf("top-level keys changed: attempts=%d theme=%q schema=%d", got.AuthAttempts, got.Theme, got.SchemaVersion)
	}
	if got.DNS.Port != 5335 || got.DNS.Ratelimit != 20 || len(got.DNS.BindHosts) != 1 {
		t.Errorf("dns block changed: %+v", got.DNS)
	}
	// querylog is the "AdGuard Home wrote this, we never modelled it" case.
	if !got.QueryLog.Enabled || got.QueryLog.Interval != "2160h" {
		t.Errorf("querylog block changed: %+v", got.QueryLog)
	}
}

func TestWriteCredentialsIsReadableByTheRestOfThePackage(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	if _, err := Seed(SeedOptions{WebPort: 3000, DNSPort: 5335}); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	if err := writeCredentials("operator", "a-much-better-password"); err != nil {
		t.Fatalf("writeCredentials: %v", err)
	}

	user, err := CurrentUser()
	if err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	if user != "operator" {
		t.Errorf("CurrentUser = %q, want operator", user)
	}
	// The ports must still resolve: a rewrite that broke them would take the
	// decoy offline without touching anything that looks password-related.
	if port, err := WebPort(); err != nil || port != 3000 {
		t.Errorf("WebPort = %d, %v; want 3000", port, err)
	}
	if port, err := DNSPort(); err != nil || port != 5335 {
		t.Errorf("DNSPort = %d, %v; want 5335", port, err)
	}

	raw, err := os.ReadFile(ConfigPath())
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	var stored struct {
		Users []struct {
			Password string `yaml:"password"`
		} `yaml:"users"`
	}
	if err := yaml.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("parsing the rewritten config: %v", err)
	}
	if len(stored.Users) != 1 {
		t.Fatalf("users = %+v, want exactly one", stored.Users)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.Users[0].Password), []byte("a-much-better-password")); err != nil {
		t.Errorf("the new password does not open the stored hash: %v", err)
	}
}

func TestWriteCredentialsRefusesBadInputWithoutTouchingTheConfig(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	if _, err := Seed(SeedOptions{WebPort: 3000, DNSPort: 5335}); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	before, err := os.ReadFile(ConfigPath())
	if err != nil {
		t.Fatalf("reading the seeded config: %v", err)
	}

	if err := writeCredentials("admin", "short"); err == nil {
		t.Error("writeCredentials accepted a too-short password")
	}
	after, err := os.ReadFile(ConfigPath())
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if string(before) != string(after) {
		t.Error("a rejected credential change still rewrote the config")
	}
}
