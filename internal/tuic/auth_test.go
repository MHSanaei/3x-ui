package tuic

import (
	"crypto/rand"
	"crypto/tls"
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
	if err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}
