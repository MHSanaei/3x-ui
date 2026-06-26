// Package nodetoken provides reversible at-rest encryption for the per-node
// outbound bearer token the master replays to each worker node.
//
// Threat model: a database-only compromise (SQL injection, leaked backup, read
// replica) must yield ciphertext, not live worker tokens. It does NOT protect
// against compromise of the master process itself, which necessarily holds the
// key to use the token. The master's *inbound* API tokens are SHA-256 hashes
// (compare-only); this package is only for the *outbound* token, which must be
// recoverable to be replayed and therefore cannot be hashed.
//
// On-disk format:  enc:v1:<key-id>:<base64url(nonce(12) || ciphertext+tag)>
// Cipher:          AES-256-GCM, fresh 96-bit nonce per value.
// AAD:             "nodes/api_token/<nodeID>" — binds ciphertext to its row so a
//
//	DB-write-capable attacker cannot move it between rows/fields.
//
// A versioned key-id supports rotation via a keyring (active key for writes,
// active+previous keys for reads). Values without the "enc:" prefix are treated
// as legacy plaintext (not yet migrated); an "enc:" value that cannot be
// decrypted is always an error — never silently reinterpreted as plaintext.
package nodetoken

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Mode is the explicit encryption policy. It is configured out-of-band (env)
// rather than inferred from whether a key happens to be present, so that a lost
// key cannot silently downgrade a previously-encrypted deployment to plaintext.
type Mode int

const (
	// ModeOff: legacy plaintext operation. Writes store plaintext; an encrypted
	// value cannot be interpreted (no key) and is rejected rather than guessed.
	ModeOff Mode = iota
	// ModeMigration: key required. Reads accept plaintext OR ciphertext; writes
	// always produce ciphertext. Used while migrating existing rows.
	ModeMigration
	// ModeRequired: key required (startup fails if it cannot load). Reads decrypt
	// ciphertext (error on failure) and accept any still-unmigrated plaintext;
	// writes always produce ciphertext.
	ModeRequired
)

const (
	encPrefix    = "enc:"
	encScheme    = "enc:v1:"
	keyLen       = 32 // AES-256
	nonceLen     = 12 // GCM standard nonce
	aadKeyFormat = "nodes/api_token/%d"
)

// ParseMode maps the NODE_TOKEN_ENCRYPTION env value to a Mode.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "off":
		return ModeOff, nil
	case "migration":
		return ModeMigration, nil
	case "required":
		return ModeRequired, nil
	default:
		return ModeOff, fmt.Errorf("nodetoken: unknown NODE_TOKEN_ENCRYPTION %q (want off|migration|required)", s)
	}
}

// Keyring holds the active key (used for new writes) plus any previous keys
// retained for decrypting not-yet-rotated values.
type Keyring struct {
	ActiveID string
	Keys     map[string][keyLen]byte
}

func (kr *Keyring) active() ([keyLen]byte, error) {
	k, ok := kr.Keys[kr.ActiveID]
	if !ok {
		return [keyLen]byte{}, fmt.Errorf("nodetoken: active key %q not in keyring", kr.ActiveID)
	}
	return k, nil
}

// Codec encrypts/decrypts node tokens under a fixed policy and keyring.
type Codec struct {
	mode Mode
	ring *Keyring // nil only in ModeOff
}

// NewCodec builds a Codec. In ModeMigration/ModeRequired a non-nil keyring with
// a valid active key is required; in ModeOff the keyring is ignored.
func NewCodec(mode Mode, ring *Keyring) (*Codec, error) {
	if mode == ModeOff {
		return &Codec{mode: ModeOff}, nil
	}
	if ring == nil || len(ring.Keys) == 0 {
		return nil, errors.New("nodetoken: encryption mode requires a key, but none was loaded")
	}
	if _, err := ring.active(); err != nil {
		return nil, err
	}
	return &Codec{mode: mode, ring: ring}, nil
}

// Enabled reports whether the codec writes ciphertext (mode != off).
func (c *Codec) Enabled() bool { return c.mode != ModeOff }

func aad(nodeID int) []byte { return []byte(fmt.Sprintf(aadKeyFormat, nodeID)) }

// IsEncrypted reports whether a stored value is in this package's ciphertext form.
func IsEncrypted(stored string) bool { return strings.HasPrefix(stored, encPrefix) }

// Encrypt returns the at-rest form of plaintext for the given node.
//   - ModeOff: returns plaintext unchanged (no-op, backward compatible).
//   - otherwise: returns enc:v1:<active-id>:<...>. An empty plaintext is returned
//     unchanged so optional/blank tokens are never turned into ciphertext.
//   - already-encrypted input is returned unchanged (round-trip safe: a UI that
//     re-submits the stored ciphertext does not get double-encrypted).
func (c *Codec) Encrypt(nodeID int, plaintext string) (string, error) {
	if c.mode == ModeOff || plaintext == "" {
		return plaintext, nil
	}
	if IsEncrypted(plaintext) {
		// Validate it actually decrypts for this node; if so keep verbatim.
		if _, err := c.Decrypt(nodeID, plaintext); err != nil {
			return "", fmt.Errorf("nodetoken: refusing to store undecryptable ciphertext: %w", err)
		}
		return plaintext, nil
	}
	key, err := c.ring.active()
	if err != nil {
		return "", err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, []byte(plaintext), aad(nodeID))
	blob := append(nonce, ct...)
	return encScheme + c.ring.ActiveID + ":" + base64.RawURLEncoding.EncodeToString(blob), nil
}

// Decrypt returns the plaintext token for the given node.
//   - A value without the "enc:" prefix is legacy plaintext: returned as-is in
//     every mode (a not-yet-migrated row, or a transient form value with nodeID 0).
//   - An "enc:" value must decrypt successfully or this returns an error; it is
//     never reinterpreted as plaintext.
func (c *Codec) Decrypt(nodeID int, stored string) (string, error) {
	if !IsEncrypted(stored) {
		return stored, nil
	}
	rest, ok := strings.CutPrefix(stored, encScheme)
	if !ok {
		return "", fmt.Errorf("nodetoken: unsupported ciphertext scheme in %q", firstN(stored, 12))
	}
	keyID, b64, ok := strings.Cut(rest, ":")
	if !ok || keyID == "" {
		return "", errors.New("nodetoken: malformed ciphertext (missing key id)")
	}
	if c.ring == nil {
		return "", errors.New("nodetoken: encrypted token encountered but encryption is disabled (no key)")
	}
	key, ok := c.ring.Keys[keyID]
	if !ok {
		return "", fmt.Errorf("nodetoken: no key %q in keyring to decrypt token", keyID)
	}
	blob, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("nodetoken: base64 decode: %w", err)
	}
	if len(blob) < nonceLen {
		return "", errors.New("nodetoken: ciphertext too short")
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	pt, err := gcm.Open(nil, blob[:nonceLen], blob[nonceLen:], aad(nodeID))
	if err != nil {
		return "", fmt.Errorf("nodetoken: authentication failed for node %d: %w", nodeID, err)
	}
	return string(pt), nil
}

// ActiveKeyID returns the id new writes use (empty in ModeOff).
func (c *Codec) ActiveKeyID() string {
	if c.ring == nil {
		return ""
	}
	return c.ring.ActiveID
}

// EncryptedWithActive reports whether stored is ciphertext already under the
// active key — lets the migration skip rows that need no work.
func (c *Codec) EncryptedWithActive(stored string) bool {
	if c.ring == nil || !IsEncrypted(stored) {
		return false
	}
	rest, ok := strings.CutPrefix(stored, encScheme)
	if !ok {
		return false
	}
	keyID, _, ok := strings.Cut(rest, ":")
	return ok && keyID == c.ring.ActiveID
}

func newGCM(key [keyLen]byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// --- package singleton, initialized once at startup ---

var (
	mu      sync.RWMutex
	current *Codec
)

// Init installs the process-wide codec. Call once during startup after building
// the keyring; in ModeOff a nil keyring is fine.
func Init(c *Codec) {
	mu.Lock()
	defer mu.Unlock()
	current = c
}

// get returns the installed codec, or a permissive ModeOff codec if Init was
// never called (e.g. unit tests / sqlite dev) so callers never nil-panic.
func get() *Codec {
	mu.RLock()
	c := current
	mu.RUnlock()
	if c == nil {
		return &Codec{mode: ModeOff}
	}
	return c
}

// Encrypt/Decrypt/Enabled operate on the process-wide codec.
func Encrypt(nodeID int, plaintext string) (string, error) { return get().Encrypt(nodeID, plaintext) }
func Decrypt(nodeID int, stored string) (string, error)    { return get().Decrypt(nodeID, stored) }
func Enabled() bool                                        { return get().Enabled() }
func Active() *Codec                                       { return get() }
