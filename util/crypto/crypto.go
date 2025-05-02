package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

func HashSHA256(text string) string {
	hasher := sha256.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}
