package service

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHappGenerateLocallyWithoutNetwork(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, "local-only")
	configureHappSubscription(t, true, "https://sub.example/sub/")
	configureHappLinkGate(t, true)
	var calls atomic.Int32
	previous := http.DefaultTransport
	// Fail before opening a socket, including clients cloned from the default transport.
	http.DefaultTransport = &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
		calls.Add(1)
		return nil, errors.New("network is unavailable in the local-generation test")
	}}
	t.Cleanup(func() { http.DefaultTransport = previous })
	svc := NewHappService(&ClientService{}, &SettingService{})
	result, err := svc.Generate(context.Background(), client.Id, "panel.example")
	if err != nil || !strings.HasPrefix(result.EncryptedLink, "happ://crypt5/") {
		t.Fatalf("local generation = %#v, %v; network attempts = %d", result, err, calls.Load())
	}
	if calls.Load() != 0 {
		t.Fatalf("local generation attempted %d network connections", calls.Load())
	}
}

func TestHappEncryptPreservesUTF8AndEnforcesResourceLimit(t *testing.T) {
	key, err := syntheticHappKey()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, source string
		wantError    error
	}{
		{"unicode URL", "https://example.com/中文?emoji=🔒&literal=%2F&x=a+b", nil},
		{"501 ASCII bytes", "https://example.com/" + strings.Repeat("a", 481), nil},
		{"502 ASCII bytes", "https://example.com/" + strings.Repeat("a", 482), nil},
		{"501 UTF8 bytes", "https://example.com/" + strings.Repeat("界", 160) + "a", nil},
		{"502 UTF8 bytes", "https://example.com/" + strings.Repeat("界", 160) + "ab", nil},
		{"8192 ASCII bytes", "https://example.com/" + strings.Repeat("a", 8172), nil},
		{"8193 ASCII bytes", "https://example.com/" + strings.Repeat("a", 8173), ErrHappSourceTooLong},
		{"8192 UTF8 bytes", "https://example.com/" + strings.Repeat("界", 2724), nil},
		{"8193 UTF8 bytes", "https://example.com/" + strings.Repeat("界", 2724) + "a", ErrHappSourceTooLong},
		{"raw query and fragment", "https://example.com/s%2fb?a=one+two&b=%2B#标题", nil},
		{"empty URL", "", ErrHappLinkUnavailable},
		{"invalid URL", "not-a-url", ErrHappLinkUnavailable},
		{"unsupported scheme", "file:///tmp/sub", ErrHappLinkUnavailable},
		{"empty host", "https:///sub", ErrHappLinkUnavailable},
		{"control character", "https://example.com/a\nb", ErrHappLinkUnavailable},
		{"Unicode control", "https://example.com/a\u0085b", ErrHappLinkUnavailable},
		{"invalid UTF8", "https://example.com/" + string([]byte{0xff}), ErrHappLinkUnavailable},
		{"userinfo", "https://user:pass@example.com/sub", ErrHappLinkUnavailable},
		{"opaque URL", "https:sub", ErrHappLinkUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			link, err := encryptHappSource(tc.source, &key.PublicKey)
			if !errors.Is(err, tc.wantError) {
				t.Fatalf("error = %v, want %v", err, tc.wantError)
			}
			if tc.wantError != nil {
				if link != "" {
					t.Fatal("failed encryption returned a link")
				}
				return
			}
			if got := decryptHappTestLink(t, link, key); got != tc.source {
				t.Fatalf("source was changed or truncated: %q", got)
			}
		})
	}
	for _, invalidKey := range []*rsa.PublicKey{nil, {N: key.N, E: 0}} {
		link, err := encryptHappSource("https://example.com/sub", invalidKey)
		if !errors.Is(err, ErrHappLinkUnavailable) || link != "" {
			t.Fatalf("invalid key result = %q, %v", link, err)
		}
	}
}

func TestHappEncryptUsesClientValidatedPublicKey(t *testing.T) {
	block, _ := pem.Decode([]byte(happPublicKeyPEM))
	if block == nil {
		t.Fatal("missing public key")
	}
	// Pin the marker's public key from the accepted Android/Windows Crypt5 probe.
	if got := fmt.Sprintf("%x", sha256.Sum256(block.Bytes)); got != "22319c7b13647897bf5fd4f827ba92bf3946d738007a0054ccd931c31f221768" {
		t.Fatalf("unvalidated public key: %s", got)
	}
	first, err := encryptHappLink("https://example.com/sub")
	if err != nil {
		t.Fatal(err)
	}
	second, err := encryptHappLink("https://example.com/sub")
	if err != nil || len(first) != 795 || !strings.HasPrefix(first, "happ://crypt5/") || first == second {
		t.Fatalf("expected fresh crypt5 ciphertext: length=%d, err=%v", len(first), err)
	}
}

func TestHappEncryptUsesFreshSessionKeysAndNonces(t *testing.T) {
	key, err := syntheticHappKey()
	if err != nil {
		t.Fatal(err)
	}
	keys, nonces := map[string]bool{}, map[string]bool{}
	for range 8 {
		link, err := encryptHappSource("https://example.com/sub", &key.PublicKey)
		if err != nil {
			t.Fatal(err)
		}
		decoded := decodeHappTestLink(t, link, key)
		if keys[string(decoded.key)] || nonces[string(decoded.nonce)] {
			t.Fatal("generation reused a session key or nonce")
		}
		keys[string(decoded.key)], nonces[string(decoded.nonce)] = true, true
	}
}
