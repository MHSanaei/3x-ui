// Package crypto provides cryptographic utilities for password hashing and verification.
package crypto

import (
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// HashPasswordAsBcrypt generates a bcrypt hash of the given password.
func HashPasswordAsBcrypt(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// CheckPasswordHash verifies if the given password matches the bcrypt hash.
func CheckPasswordHash(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func IsHashed(s string) bool {
	_, err := bcrypt.Cost([]byte(s))
	return err == nil
}

// HashTokenSHA256 returns the hex-encoded SHA-256 digest of token. API tokens
// are high-entropy random strings, so a fast unsalted digest is sufficient to
// keep them irrecoverable at rest while allowing constant-time verification.
func HashTokenSHA256(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// IsSHA256Hex reports whether s looks like a hex-encoded SHA-256 digest
// (64 lowercase hex characters), used to skip already-hashed token rows.
func IsSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
