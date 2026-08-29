package adguard

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
	"golang.org/x/crypto/bcrypt"
)

// Bounds on the credentials the admin may set. The minimum password length
// matches what AdGuard Home's own UI enforces, so a password accepted here is
// one it would also accept.
const (
	maxUserLength     = 64
	minPasswordLength = 8
	maxPasswordLength = 128
)

// ValidateCredentials rejects anything that must not reach the config file.
//
// The username lands in YAML, so a control character or a newline in it could
// otherwise end the scalar and let the rest be read as configuration. Quoting
// already prevents that; refusing the input outright means there is no single
// escaping bug between an admin-supplied string and AdGuard Home's settings.
func ValidateCredentials(user, password string) error {
	if user == "" {
		return fmt.Errorf("the username must not be empty")
	}
	if len(user) > maxUserLength {
		return fmt.Errorf("the username must be at most %d characters", maxUserLength)
	}
	if strings.TrimSpace(user) != user {
		return fmt.Errorf("the username must not start or end with a space")
	}
	if hasControlChars(user) {
		return fmt.Errorf("the username must not contain control characters")
	}
	if len(password) < minPasswordLength {
		return fmt.Errorf("the password must be at least %d characters", minPasswordLength)
	}
	if len(password) > maxPasswordLength {
		return fmt.Errorf("the password must be at most %d characters", maxPasswordLength)
	}
	if hasControlChars(password) {
		return fmt.Errorf("the password must not contain control characters")
	}
	return nil
}

func hasControlChars(s string) bool {
	for _, r := range s {
		if r == '\n' || r == '\r' || unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// writeCredentials replaces the single account in AdGuard Home's config.
//
// The caller must have stopped AdGuard Home first: it holds the whole config
// in memory and rewrites the file on any change of its own, so an edit made
// underneath a running instance is liable to be thrown away.
func writeCredentials(user, password string) error {
	if err := ValidateCredentials(user, password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing the AdGuard Home password: %w", err)
	}
	raw, err := os.ReadFile(ConfigPath())
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", ConfigPath(), err)
	}
	updated, err := replaceUsers(raw, user, string(hash))
	if err != nil {
		return err
	}
	if err := os.WriteFile(ConfigPath(), updated, 0o600); err != nil {
		return fmt.Errorf("cannot write %s: %w", ConfigPath(), err)
	}
	return nil
}

// replaceUsers swaps the users list for one holding just this account.
//
// Done against the parsed document rather than by re-marshalling the whole
// file, so every other setting AdGuard Home wrote -- including keys this
// package knows nothing about -- comes back out byte for byte.
func replaceUsers(raw []byte, user, passwordHash string) ([]byte, error) {
	file, err := parser.ParseBytes(raw, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", ConfigPath(), err)
	}
	path, err := yaml.PathString("$.users")
	if err != nil {
		return nil, err
	}
	block := fmt.Sprintf("- name: %q\n  password: %q\n", user, passwordHash)
	if err := path.ReplaceWithReader(file, strings.NewReader(block)); err != nil {
		return nil, fmt.Errorf("cannot replace the AdGuard Home account: %w", err)
	}
	out := file.String()
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return []byte(out), nil
}
