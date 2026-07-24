package amneziawg

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateWireGuardKeyPair(t *testing.T) {
	priv, pub, psk, err := GenerateWireGuardKeyPair()
	if err != nil {
		t.Fatalf("GenerateWireGuardKeyPair: %v", err)
	}
	if priv == "" {
		t.Fatal("private key is empty")
	}
	if pub == "" {
		t.Fatal("public key is empty")
	}
	if psk == "" {
		t.Fatal("preshared key is empty")
	}
	if priv == pub {
		t.Fatal("private and public keys must differ")
	}
	if priv == psk {
		t.Fatal("private and preshared keys must differ")
	}
}

func TestGenerateWireGuardKeyPairBase64Length(t *testing.T) {
	priv, pub, psk, err := GenerateWireGuardKeyPair()
	if err != nil {
		t.Fatalf("GenerateWireGuardKeyPair: %v", err)
	}
	if len(priv) != 44 {
		t.Fatalf("private key length = %d, want 44", len(priv))
	}
	if len(pub) != 44 {
		t.Fatalf("public key length = %d, want 44", len(pub))
	}
	if len(psk) != 44 {
		t.Fatalf("preshared key length = %d, want 44", len(psk))
	}
	if !strings.HasSuffix(priv, "=") {
		t.Fatal("private key should have base64 padding")
	}
	if !strings.HasSuffix(pub, "=") {
		t.Fatal("public key should have base64 padding")
	}
	if !strings.HasSuffix(psk, "=") {
		t.Fatal("preshared key should have base64 padding")
	}
}

func TestGenerateWireGuardKeyPairDecodable(t *testing.T) {
	priv, pub, psk, err := GenerateWireGuardKeyPair()
	if err != nil {
		t.Fatalf("GenerateWireGuardKeyPair: %v", err)
	}
	for _, pair := range []struct {
		name string
		data string
	}{
		{"private", priv},
		{"public", pub},
		{"preshared", psk},
	} {
		decoded, err := base64.StdEncoding.DecodeString(pair.data)
		if err != nil {
			t.Fatalf("%s key is not valid base64: %v", pair.name, err)
		}
		if len(decoded) != 32 {
			t.Fatalf("%s key decoded to %d bytes, want 32", pair.name, len(decoded))
		}
	}
}

func TestGenerateWireGuardKeyPairClamping(t *testing.T) {
	priv, _, _, err := GenerateWireGuardKeyPair()
	if err != nil {
		t.Fatalf("GenerateWireGuardKeyPair: %v", err)
	}
	decoded, _ := base64.StdEncoding.DecodeString(priv)
	if decoded[0]&0x07 != 0 {
		t.Fatal("private key first byte bits 0-2 should be cleared (clamping)")
	}
	if decoded[31]&0x80 != 0 {
		t.Fatal("private key last byte bit 7 should be cleared (clamping)")
	}
	if decoded[31]&0x40 == 0 {
		t.Fatal("private key last byte bit 6 should be set (clamping)")
	}
}

func TestGenerateWireGuardKeyPairDeterministicStructure(t *testing.T) {
	for i := 0; i < 10; i++ {
		priv1, pub1, _, _ := GenerateWireGuardKeyPair()
		priv2, pub2, _, _ := GenerateWireGuardKeyPair()
		if priv1 == priv2 {
			t.Fatal("two consecutive private keys must differ")
		}
		if pub1 == pub2 {
			t.Fatal("two consecutive public keys must differ")
		}
	}
}