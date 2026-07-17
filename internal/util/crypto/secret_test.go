package crypto

import (
	"errors"
	"strings"
	"testing"
)

func TestEncryptSecretRoundTrip(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-master-key")

	tests := []struct {
		name      string
		plaintext string
	}{
		{name: "password", plaintext: "hunter2"},
		{name: "private key", plaintext: "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----"},
		{name: "unicode", plaintext: "密码-p@ss"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := EncryptSecret(tt.plaintext)
			if err != nil {
				t.Fatalf("EncryptSecret(%q) error = %v, want nil", tt.plaintext, err)
			}
			if strings.Contains(token, tt.plaintext) {
				t.Fatalf("EncryptSecret(%q) = %q, want ciphertext that does not contain the plaintext", tt.plaintext, token)
			}
			if !IsEncrypted(token) {
				t.Fatalf("IsEncrypted(%q) = false, want true", token)
			}
			got, err := DecryptSecret(token)
			if err != nil {
				t.Fatalf("DecryptSecret error = %v, want nil", err)
			}
			if got != tt.plaintext {
				t.Fatalf("DecryptSecret round trip = %q, want %q", got, tt.plaintext)
			}
		})
	}
}

func TestEncryptSecretNonceIsRandom(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-master-key")

	first, err := EncryptSecret("same-input")
	if err != nil {
		t.Fatalf("EncryptSecret error = %v, want nil", err)
	}
	second, err := EncryptSecret("same-input")
	if err != nil {
		t.Fatalf("EncryptSecret error = %v, want nil", err)
	}
	if first == second {
		t.Fatalf("EncryptSecret produced identical tokens %q for the same plaintext, want a random nonce per call", first)
	}
}

func TestEncryptSecretEmptyStaysEmpty(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-master-key")

	token, err := EncryptSecret("")
	if err != nil {
		t.Fatalf("EncryptSecret(\"\") error = %v, want nil", err)
	}
	if token != "" {
		t.Fatalf("EncryptSecret(\"\") = %q, want \"\"", token)
	}
}

func TestEncryptSecretIsIdempotent(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-master-key")

	token, err := EncryptSecret("hunter2")
	if err != nil {
		t.Fatalf("EncryptSecret error = %v, want nil", err)
	}
	again, err := EncryptSecret(token)
	if err != nil {
		t.Fatalf("EncryptSecret(token) error = %v, want nil", err)
	}
	if again != token {
		t.Fatalf("EncryptSecret(alreadyEncrypted) = %q, want it unchanged (%q)", again, token)
	}
}

func TestSecretWithoutKeyFailsClosed(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "")

	if _, err := EncryptSecret("hunter2"); !errors.Is(err, ErrNoSecretKey) {
		t.Fatalf("EncryptSecret without key error = %v, want ErrNoSecretKey", err)
	}
}

func TestDecryptSecretWrongKey(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "original-key")
	token, err := EncryptSecret("hunter2")
	if err != nil {
		t.Fatalf("EncryptSecret error = %v, want nil", err)
	}

	t.Setenv("XUI_SECRET_KEY", "rotated-key")
	if _, err := DecryptSecret(token); !errors.Is(err, ErrSecretDecrypt) {
		t.Fatalf("DecryptSecret with rotated key error = %v, want ErrSecretDecrypt", err)
	}
}

func TestDecryptSecretPassesThroughPlaintext(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-master-key")

	got, err := DecryptSecret("legacy-plaintext")
	if err != nil {
		t.Fatalf("DecryptSecret(plaintext) error = %v, want nil", err)
	}
	if got != "legacy-plaintext" {
		t.Fatalf("DecryptSecret(plaintext) = %q, want %q", got, "legacy-plaintext")
	}
}

func TestDecryptSecretTampered(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-master-key")

	token, err := EncryptSecret("hunter2")
	if err != nil {
		t.Fatalf("EncryptSecret error = %v, want nil", err)
	}
	tampered := token[:len(token)-1] + "A"
	if _, err := DecryptSecret(tampered); !errors.Is(err, ErrSecretDecrypt) {
		t.Fatalf("DecryptSecret(tampered) error = %v, want ErrSecretDecrypt", err)
	}
}
