package tuic

import (
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound    = errors.New("tuic: user not found")
	ErrAuthFailed      = errors.New("tuic: authentication failed")
	ErrInvalidTLSState = errors.New("tuic: TLS connection state not available")
)

// User represents a configured TUIC client for authentication and billing.
type User struct {
	UUID      [16]byte
	UUIDStr   string
	Password  string
	Email     string
	BytesUp   atomic.Int64
	BytesDown atomic.Int64
}

// UserRegistry is a thread-safe registry of TUIC users for an inbound.
type UserRegistry struct {
	mu    sync.RWMutex
	users map[[16]byte]*User
}

// NewUserRegistry creates an empty UserRegistry.
func NewUserRegistry() *UserRegistry {
	return &UserRegistry{
		users: make(map[[16]byte]*User),
	}
}

// SetUsers updates the user list atomically in memory, preserving counters for active users.
func (ur *UserRegistry) SetUsers(clients []TuicClientSettings) {
	ur.mu.Lock()
	defer ur.mu.Unlock()

	newMap := make(map[[16]byte]*User, len(clients))
	for _, c := range clients {
		parsed, err := uuid.Parse(c.UUID)
		if err != nil {
			continue
		}
		u := &User{
			UUID:     parsed,
			UUIDStr:  parsed.String(),
			Password: c.Password,
			Email:    c.Email,
		}
		if existing, ok := ur.users[parsed]; ok {
			u.BytesUp.Store(existing.BytesUp.Load())
			u.BytesDown.Store(existing.BytesDown.Load())
		}
		newMap[parsed] = u
	}
	ur.users = newMap
}

// ClientTrafficDelta represents the traffic delta for a user.
type ClientTrafficDelta struct {
	Email string
	Up    int64
	Down  int64
}

// CollectTrafficDeltas drains and returns byte deltas for all users since the last call.
func (ur *UserRegistry) CollectTrafficDeltas() []ClientTrafficDelta {
	ur.mu.RLock()
	defer ur.mu.RUnlock()

	var deltas []ClientTrafficDelta
	for _, u := range ur.users {
		up := u.BytesUp.Swap(0)
		down := u.BytesDown.Swap(0)
		if up > 0 || down > 0 {
			deltas = append(deltas, ClientTrafficDelta{
				Email: u.Email,
				Up:    up,
				Down:  down,
			})
		}
	}
	return deltas
}

// Authenticate verifies the client's token using RFC 5705 Keying Material Exporter.
// According to TUIC v5 specification:
// - label: client UUID
// - context: raw password
// - length: 32 bytes
func (ur *UserRegistry) Authenticate(cs *tls.ConnectionState, rawUUID [16]byte, token [32]byte) (*User, error) {
	if cs == nil {
		return nil, ErrInvalidTLSState
	}

	ur.mu.RLock()
	user, exists := ur.users[rawUUID]
	ur.mu.RUnlock()

	if !exists {
		return nil, ErrUserNotFound
	}

	// Try with raw 16-byte UUID as label
	expectedToken, err := cs.ExportKeyingMaterial(string(rawUUID[:]), []byte(user.Password), 32)
	if err == nil && subtle.ConstantTimeCompare(token[:], expectedToken) == 1 {
		return user, nil
	}

	// Fallback to formatted 36-char string representation of UUID as label
	expectedTokenStr, errStr := cs.ExportKeyingMaterial(user.UUIDStr, []byte(user.Password), 32)
	if errStr == nil && subtle.ConstantTimeCompare(token[:], expectedTokenStr) == 1 {
		return user, nil
	}

	if err != nil && errStr != nil {
		return nil, fmt.Errorf("%w: export keying material: %v", ErrAuthFailed, err)
	}

	return nil, ErrAuthFailed
}
