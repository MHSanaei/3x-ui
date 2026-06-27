package wireguard

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

	"golang.org/x/crypto/curve25519"
)

// GenerateWireguardKeypair generates a base64 encoded private and public key pair for Wireguard.
func GenerateWireguardKeypair() (privateKey string, publicKey string, err error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return "", "", err
	}
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	var pub [32]byte
	curve25519.ScalarBaseMult(&pub, &priv)

	return base64.StdEncoding.EncodeToString(priv[:]), base64.StdEncoding.EncodeToString(pub[:]), nil
}

// GenerateWireguardPSK generates a base64 encoded 32-byte pre-shared key for Wireguard.
func GenerateWireguardPSK() (string, error) {
	var psk [32]byte
	if _, err := rand.Read(psk[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(psk[:]), nil
}

// PublicKeyFromPrivate derives the base64 public key for a base64 (or hex) Wireguard private key.
func PublicKeyFromPrivate(privateKey string) (string, error) {
	priv, err := decodeWireguardKey(privateKey)
	if err != nil {
		return "", err
	}
	var pub [32]byte
	curve25519.ScalarBaseMult(&pub, &priv)
	return base64.StdEncoding.EncodeToString(pub[:]), nil
}

// KeyToHex converts a base64 (or already-hex) 32-byte Wireguard key into the
// lowercase hex form xray-core's wireguard proxy expects: its ParseKey uses
// hex.DecodeString, and the device IPC layer wants hex for public_key and
// preshared_key. An empty input yields an empty result so optional keys pass
// through untouched.
func KeyToHex(key string) (string, error) {
	if key == "" {
		return "", nil
	}
	raw, err := decodeWireguardKey(key)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

// decodeWireguardKey accepts a 64-char hex key or a base64 key (standard or
// URL-safe alphabet, with or without padding) and returns the raw 32 bytes.
func decodeWireguardKey(key string) ([32]byte, error) {
	var out [32]byte
	if key == "" {
		return out, errors.New("wireguard: empty key")
	}

	if len(key) == 64 {
		if raw, err := hex.DecodeString(key); err == nil {
			if len(raw) != 32 {
				return out, errors.New("wireguard: key must decode to 32 bytes")
			}
			copy(out[:], raw)
			return out, nil
		}
	}

	trimmed := strings.TrimRight(key, "=")
	var raw []byte
	var err error
	if strings.ContainsAny(trimmed, "+/") {
		raw, err = base64.RawStdEncoding.DecodeString(trimmed)
	} else {
		raw, err = base64.RawURLEncoding.DecodeString(trimmed)
	}
	if err != nil {
		return out, err
	}
	if len(raw) != 32 {
		return out, errors.New("wireguard: key must decode to 32 bytes")
	}
	copy(out[:], raw)
	return out, nil
}
