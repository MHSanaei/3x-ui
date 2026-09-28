package tuic

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestUserRegistryBasic(t *testing.T) {
	reg := NewUserRegistry()
	testUUID := uuid.New()

	reg.SetUsers([]TuicClientSettings{
		{
			UUID:     testUUID.String(),
			Password: "supersecretpassword",
			Email:    "test@example.com",
		},
	})

	var fakeUUID [16]byte
	copy(fakeUUID[:], testUUID[:])

	var unknownUUID [16]byte
	_, _ = rand.Read(unknownUUID[:])
	_, err := reg.Authenticate(&tls.ConnectionState{}, unknownUUID, [32]byte{})
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRegistryPointerStability(t *testing.T) {
	reg := NewUserRegistry()
	u1 := uuid.New().String()
	u2 := uuid.New().String()

	reg.SetUsers([]TuicClientSettings{
		{UUID: u1, Password: "pass1", Email: "u1@example.com"},
		{UUID: u2, Password: "pass2", Email: "u2@example.com"},
	})

	reg.AddTestTraffic("u1@example.com", 100, 200)

	parsedU1, _ := uuid.Parse(u1)
	user1Before := reg.users[parsedU1]
	if user1Before == nil {
		t.Fatalf("expected user1 in registry")
	}

	revoked := reg.SetUsers([]TuicClientSettings{
		{UUID: u1, Password: "newpassword", Email: "u1-new@example.com"},
		{UUID: u2, Password: "pass2", Email: "u2@example.com"},
	})

	if len(revoked) != 0 {
		t.Fatalf("expected 0 revoked users, got %d", len(revoked))
	}

	user1After := reg.users[parsedU1]
	if user1Before != user1After {
		t.Fatalf("expected pointer stability for user1: before=%p, after=%p", user1Before, user1After)
	}
	if user1After.Password != "newpassword" || user1After.Email != "u1-new@example.com" {
		t.Fatalf("expected updated password and email, got %s, %s", user1After.Password, user1After.Email)
	}

	deltas := reg.CollectTrafficDeltas()
	if len(deltas) != 1 || deltas[0].Up != 100 || deltas[0].Down != 200 {
		t.Fatalf("expected preserved traffic deltas, got %+v", deltas)
	}
}

func TestUserRegistryRevocation(t *testing.T) {
	reg := NewUserRegistry()
	u1 := uuid.New().String()
	u2 := uuid.New().String()

	reg.SetUsers([]TuicClientSettings{
		{UUID: u1, Password: "pass1", Email: "u1@example.com"},
		{UUID: u2, Password: "pass2", Email: "u2@example.com"},
	})

	// Remove u1, keep only u2
	revoked := reg.SetUsers([]TuicClientSettings{
		{UUID: u2, Password: "pass2", Email: "u2@example.com"},
	})

	if len(revoked) != 1 || revoked[0].Email != "u1@example.com" {
		t.Fatalf("expected u1 revoked, got %+v", revoked)
	}

	parsedU1, _ := uuid.Parse(u1)
	if _, exists := reg.users[parsedU1]; exists {
		t.Fatalf("expected u1 removed from registry")
	}
}
