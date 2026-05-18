package crypto

import (
	"strings"
	"testing"
)

func TestHashPasswordAsBcrypt_RoundTrip(t *testing.T) {
	password := "correct horse battery staple"

	hash, err := HashPasswordAsBcrypt(password)
	if err != nil {
		t.Fatalf("HashPasswordAsBcrypt returned error: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == password {
		t.Fatal("hash must not equal the plaintext password")
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("expected bcrypt prefix $2..., got %q", hash[:min(4, len(hash))])
	}

	if !CheckPasswordHash(hash, password) {
		t.Fatal("CheckPasswordHash returned false for the matching password")
	}
}

func TestCheckPasswordHash_WrongPassword(t *testing.T) {
	hash, err := HashPasswordAsBcrypt("right-password")
	if err != nil {
		t.Fatalf("HashPasswordAsBcrypt returned error: %v", err)
	}

	if CheckPasswordHash(hash, "wrong-password") {
		t.Fatal("CheckPasswordHash returned true for a wrong password")
	}
	if CheckPasswordHash(hash, "") {
		t.Fatal("CheckPasswordHash returned true for an empty password")
	}
}

func TestCheckPasswordHash_InvalidHash(t *testing.T) {
	if CheckPasswordHash("", "anything") {
		t.Fatal("empty hash must not validate")
	}
	if CheckPasswordHash("not-a-bcrypt-hash", "anything") {
		t.Fatal("malformed hash must not validate")
	}
}

func TestHashPasswordAsBcrypt_DifferentHashesForSamePassword(t *testing.T) {
	password := "same-password"
	h1, err := HashPasswordAsBcrypt(password)
	if err != nil {
		t.Fatalf("first hash failed: %v", err)
	}
	h2, err := HashPasswordAsBcrypt(password)
	if err != nil {
		t.Fatalf("second hash failed: %v", err)
	}
	if h1 == h2 {
		t.Fatal("expected bcrypt to produce different hashes (random salt) for the same password")
	}
	if !CheckPasswordHash(h1, password) || !CheckPasswordHash(h2, password) {
		t.Fatal("both hashes should still validate the original password")
	}
}
