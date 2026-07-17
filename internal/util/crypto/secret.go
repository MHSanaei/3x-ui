package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

const secretPrefix = "enc:v1:"

// ErrNoSecretKey is returned when a reversible credential must be encrypted or
// decrypted but XUI_SECRET_KEY is unset. It is deliberately fatal rather than
// falling back to plaintext: a silent fallback would write SSH passwords and
// private keys to the database in the clear.
var ErrNoSecretKey = errors.New("XUI_SECRET_KEY is not set; it is required to store SSH credentials")

// ErrSecretDecrypt is returned when a stored credential cannot be decrypted,
// which in practice means XUI_SECRET_KEY no longer matches the one that wrote
// the row. The credential must be re-entered; it cannot be recovered.
var ErrSecretDecrypt = errors.New("stored credential could not be decrypted; XUI_SECRET_KEY may have changed")

func secretAEAD() (cipher.AEAD, error) {
	key := config.GetSecretKey()
	if key == "" {
		return nil, ErrNoSecretKey
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// IsEncrypted reports whether s is already an EncryptSecret output, so callers
// can skip re-encrypting a value round-tripped through the API unchanged.
func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, secretPrefix)
}

// EncryptSecret seals plaintext with AES-256-GCM under the XUI_SECRET_KEY
// master key and returns a prefixed, base64-encoded token. An empty plaintext
// encrypts to an empty string so an unset credential stays unset rather than
// becoming an encrypted empty value. Values already encrypted pass through.
func EncryptSecret(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if IsEncrypted(plaintext) {
		return plaintext, nil
	}
	aead, err := secretAEAD()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return secretPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptSecret opens a token produced by EncryptSecret. A value without the
// encryption prefix is returned unchanged so rows written before this feature
// existed keep working.
func DecryptSecret(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	if !IsEncrypted(token) {
		return token, nil
	}
	aead, err := secretAEAD()
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(token, secretPrefix))
	if err != nil {
		return "", ErrSecretDecrypt
	}
	if len(raw) < aead.NonceSize() {
		return "", ErrSecretDecrypt
	}
	nonce, sealed := raw[:aead.NonceSize()], raw[aead.NonceSize():]
	opened, err := aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", ErrSecretDecrypt
	}
	return string(opened), nil
}
