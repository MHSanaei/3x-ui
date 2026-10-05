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
	TrafficID int
	UUID      [16]byte
	UUIDStr   string
	Password  string
	Email     string
	Traffic   *UserTraffic
	sessions  atomic.Int64
}

type UserTraffic struct {
	BytesUp   atomic.Int64
	BytesDown atomic.Int64
}

// UserRegistry is a thread-safe registry of TUIC users for an inbound.
type UserRegistry struct {
	mu      sync.RWMutex
	users   map[[16]byte]*User
	retired map[*User]struct{}
}

// NewUserRegistry creates an empty UserRegistry.
func NewUserRegistry() *UserRegistry {
	return &UserRegistry{
		users:   make(map[[16]byte]*User),
		retired: make(map[*User]struct{}),
	}
}

// SetUsers updates the user list atomically in memory, preserving counters and
// pointer stability for active sessions. It returns any users removed from the registry.
func (ur *UserRegistry) SetUsers(clients []TuicClientSettings) (revoked []*User) {
	ur.mu.Lock()
	defer ur.mu.Unlock()

	newMap := make(map[[16]byte]*User, len(clients))
	for _, c := range clients {
		parsed, err := uuid.Parse(c.UUID)
		if err != nil {
			continue
		}
		if existing, ok := ur.users[parsed]; ok && existing.Password == c.Password && existing.Email == c.Email && existing.TrafficID == c.TrafficID {
			newMap[parsed] = existing
		} else if ok {
			ur.retired[existing] = struct{}{}
			revoked = append(revoked, existing)
			newMap[parsed] = newUser(parsed, c)
		} else {
			newMap[parsed] = newUser(parsed, c)
		}
	}
	for id, oldUser := range ur.users {
		if _, ok := newMap[id]; !ok {
			revoked = append(revoked, oldUser)
			ur.retired[oldUser] = struct{}{}
		}
	}
	ur.users = newMap
	return revoked
}

func newUser(id [16]byte, c TuicClientSettings) *User {
	return &User{
		TrafficID: c.TrafficID, UUID: id, UUIDStr: uuid.UUID(id).String(), Password: c.Password, Email: c.Email,
		Traffic: &UserTraffic{},
	}
}

// AddTestTraffic adds traffic counters to a user by email for testing purposes.
func (ur *UserRegistry) AddTestTraffic(email string, up, down int64) bool {
	ur.mu.RLock()
	defer ur.mu.RUnlock()
	for _, u := range ur.users {
		if u.Email == email {
			u.Traffic.BytesUp.Add(up)
			u.Traffic.BytesDown.Add(down)
			return true
		}
	}
	return false
}

// ClientTrafficDelta represents the traffic delta for a user.
type ClientTrafficDelta struct {
	TrafficID int
	Email     string
	UUID      string
	InboundID int
	Up        int64
	Down      int64
}

// CollectTrafficDeltas drains and returns byte deltas for all users since the last call.
func (ur *UserRegistry) CollectTrafficDeltas() []ClientTrafficDelta {
	ur.mu.Lock()
	defer ur.mu.Unlock()

	var deltas []ClientTrafficDelta
	collect := func(u *User, retired bool) {
		up := u.Traffic.BytesUp.Swap(0)
		down := u.Traffic.BytesDown.Swap(0)
		if up > 0 || down > 0 {
			deltas = append(deltas, ClientTrafficDelta{
				TrafficID: u.TrafficID,
				Email:     u.Email,
				UUID:      u.UUIDStr,
				Up:        up,
				Down:      down,
			})
		}
		if retired && u.sessions.Load() == 0 {
			delete(ur.retired, u)
		}
	}
	for _, u := range ur.users {
		collect(u, false)
	}
	for u := range ur.retired {
		collect(u, true)
	}
	return deltas
}

func (ur *UserRegistry) sessionEnded(user *User) {
	if user != nil {
		user.sessions.Add(-1)
	}
}

// Authenticate verifies the client's token using RFC 5705 Keying Material Exporter.
// According to TUIC v5 specification:
// - label: client UUID
// - context: raw password
// - length: 32 bytes
func (ur *UserRegistry) Authenticate(cs *tls.ConnectionState, rawUUID [16]byte, token [32]byte) (*User, error) {
	return ur.authenticate(cs, rawUUID, token, nil)
}

// AuthenticateAndRegister holds the registry read lock through connection
// registration, making successful authentication atomic with user revocation.
func (ur *UserRegistry) AuthenticateAndRegister(cs *tls.ConnectionState, rawUUID [16]byte, token [32]byte, register func(*User) bool) (*User, error) {
	return ur.authenticate(cs, rawUUID, token, register)
}

func (ur *UserRegistry) authenticate(cs *tls.ConnectionState, rawUUID [16]byte, token [32]byte, register func(*User) bool) (*User, error) {
	if cs == nil {
		return nil, ErrInvalidTLSState
	}

	ur.mu.RLock()
	defer ur.mu.RUnlock()
	user, exists := ur.users[rawUUID]

	if !exists {
		return nil, ErrUserNotFound
	}
	if !cs.HandshakeComplete {
		return nil, ErrInvalidTLSState
	}

	// Try with raw 16-byte UUID as label
	expectedToken, err := cs.ExportKeyingMaterial(string(rawUUID[:]), []byte(user.Password), 32)
	if err == nil && subtle.ConstantTimeCompare(token[:], expectedToken) == 1 {
		if register != nil && !register(user) {
			return nil, ErrUserNotFound
		}
		return user, nil
	}

	// Fallback to formatted 36-char string representation of UUID as label
	expectedTokenStr, errStr := cs.ExportKeyingMaterial(user.UUIDStr, []byte(user.Password), 32)
	if errStr == nil && subtle.ConstantTimeCompare(token[:], expectedTokenStr) == 1 {
		if register != nil && !register(user) {
			return nil, ErrUserNotFound
		}
		return user, nil
	}

	if err != nil && errStr != nil {
		return nil, fmt.Errorf("%w: export keying material: %w", ErrAuthFailed, err)
	}

	return nil, ErrAuthFailed
}

func ValidateClients(clients []TuicClientSettings) error {
	seen := make(map[uuid.UUID]bool, len(clients))
	for _, client := range clients {
		id, err := uuid.Parse(client.UUID)
		if err != nil {
			return errors.New("tuic: invalid client UUID")
		}
		if seen[id] {
			return errors.New("tuic: duplicate client UUID")
		}
		seen[id] = true
	}
	return nil
}
