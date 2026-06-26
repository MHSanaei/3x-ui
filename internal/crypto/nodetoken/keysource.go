package nodetoken

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// KeySource loads a Keyring at startup. Two implementations are provided: a
// mode-0600 key file (preferred for this fleet) and an environment variable
// (operationally simpler, but visible via process inspection and inheritable by
// children). The key is never accepted on the command line.
type KeySource interface {
	Load() (*Keyring, error)
}

// keyFile is the JSON shape of the key file:
//
//	{ "active": "k2", "keys": { "k1": "<base64-32B>", "k2": "<base64-32B>" } }
//
// Rotation: add a new key, point "active" at it, run the migration, then drop
// the old key once no row references it.
type keyFile struct {
	Active string            `json:"active"`
	Keys   map[string]string `json:"keys"`
}

func parseKeyring(active string, b64keys map[string]string) (*Keyring, error) {
	if active == "" {
		return nil, errors.New("nodetoken: key source has no active key id")
	}
	if len(b64keys) == 0 {
		return nil, errors.New("nodetoken: key source has no keys")
	}
	kr := &Keyring{ActiveID: active, Keys: make(map[string][keyLen]byte, len(b64keys))}
	for id, b64 := range b64keys {
		raw, err := decodeKey(b64)
		if err != nil {
			return nil, fmt.Errorf("nodetoken: key %q: %w", id, err)
		}
		kr.Keys[id] = raw
	}
	if _, ok := kr.Keys[active]; !ok {
		return nil, fmt.Errorf("nodetoken: active key %q absent from keys", active)
	}
	return kr, nil
}

func decodeKey(b64 string) ([keyLen]byte, error) {
	var out [keyLen]byte
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		// tolerate url-safe / unpadded encodings too
		if raw2, err2 := base64.RawStdEncoding.DecodeString(strings.TrimSpace(b64)); err2 == nil {
			raw = raw2
		} else {
			return out, fmt.Errorf("base64 decode: %w", err)
		}
	}
	if len(raw) != keyLen {
		return out, fmt.Errorf("key must be %d bytes, got %d", keyLen, len(raw))
	}
	copy(out[:], raw)
	return out, nil
}

// FileKeySource reads a JSON keyring from Path. It validates that the file is
// not group/world readable (mode must be 0600 or stricter) before trusting it.
type FileKeySource struct {
	Path string
}

func (f FileKeySource) Load() (*Keyring, error) {
	info, err := os.Stat(f.Path)
	if err != nil {
		return nil, fmt.Errorf("nodetoken: stat key file %s: %w", f.Path, err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return nil, fmt.Errorf("nodetoken: key file %s has insecure mode %#o (want 0600)", f.Path, perm)
	}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, fmt.Errorf("nodetoken: read key file %s: %w", f.Path, err)
	}
	var kf keyFile
	if err := json.Unmarshal(data, &kf); err != nil {
		return nil, fmt.Errorf("nodetoken: parse key file %s: %w", f.Path, err)
	}
	return parseKeyring(kf.Active, kf.Keys)
}

// EnvKeySource reads a single base64 32-byte key from an environment variable.
// The key id is fixed ("env"); for multi-key rotation prefer a key file.
type EnvKeySource struct {
	Var string
}

func (e EnvKeySource) Load() (*Keyring, error) {
	v := strings.TrimSpace(os.Getenv(e.Var))
	if v == "" {
		return nil, fmt.Errorf("nodetoken: env %s is empty", e.Var)
	}
	return parseKeyring("env", map[string]string{"env": v})
}
