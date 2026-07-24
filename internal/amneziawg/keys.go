package amneziawg

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// GenerateWireGuardKeyPair generates a valid WireGuard private/public key pair
// and a preshared key. Uses Go's crypto/rand for key material and Curve25519
// for public key derivation — no Docker dependency.
func GenerateWireGuardKeyPair() (privateKey, publicKey, presharedKey string, err error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return "", "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	var pub [32]byte
	curve25519.ScalarBaseMult(&pub, &priv)

	var psk [32]byte
	if _, err := rand.Read(psk[:]); err != nil {
		return "", "", "", fmt.Errorf("failed to generate preshared key: %w", err)
	}

	return base64.StdEncoding.EncodeToString(priv[:]),
		base64.StdEncoding.EncodeToString(pub[:]),
		base64.StdEncoding.EncodeToString(psk[:]),
		nil
}
