package model

import (
	"reflect"
	"testing"
)

func TestClientToRecordRoundTripWireGuard(t *testing.T) {
	c := &Client{
		Email:        "alice@example.test",
		Enable:       true,
		PrivateKey:   "cGVlci1wcml2YXRlLWtleS1iYXNlNjQtMzJieXRlcw==",
		PublicKey:    "cGVlci1wdWJsaWMta2V5LWJhc2U2NC0zMmJ5dGVzISE=",
		AllowedIPs:   []string{"10.0.0.2/32", "fd00::2/128"},
		PreSharedKey: "cHNrLWJhc2U2NC0zMmJ5dGVzLXBsYWNlaG9sZGVyISE=",
		KeepAlive:    25,
	}

	rec := c.ToRecord()
	if rec.AllowedIPs != "10.0.0.2/32,fd00::2/128" {
		t.Fatalf("AllowedIPs CSV = %q, want %q", rec.AllowedIPs, "10.0.0.2/32,fd00::2/128")
	}

	got := rec.ToClient()
	for _, f := range []struct {
		name string
		a, b any
	}{
		{"PrivateKey", c.PrivateKey, got.PrivateKey},
		{"PublicKey", c.PublicKey, got.PublicKey},
		{"PreSharedKey", c.PreSharedKey, got.PreSharedKey},
		{"KeepAlive", c.KeepAlive, got.KeepAlive},
	} {
		if f.a != f.b {
			t.Errorf("%s round-trip = %v, want %v", f.name, f.b, f.a)
		}
	}
	if !reflect.DeepEqual(got.AllowedIPs, c.AllowedIPs) {
		t.Errorf("AllowedIPs round-trip = %v, want %v", got.AllowedIPs, c.AllowedIPs)
	}
}

func TestClientRecordEmptyAllowedIPs(t *testing.T) {
	rec := &ClientRecord{Email: "bob@example.test", AllowedIPs: ""}
	if got := rec.ToClient().AllowedIPs; got != nil {
		t.Fatalf("empty CSV → AllowedIPs = %v, want nil", got)
	}

	rec.AllowedIPs = " 10.0.0.5/32 , ,"
	if got := rec.ToClient().AllowedIPs; !reflect.DeepEqual(got, []string{"10.0.0.5/32"}) {
		t.Fatalf("trimmed CSV → AllowedIPs = %v, want [10.0.0.5/32]", got)
	}
}

func TestMergeClientRecordWireGuardKeysPreserved(t *testing.T) {
	existing := &ClientRecord{
		Email:      "carol@example.test",
		PrivateKey: "existing-private",
		PublicKey:  "existing-public",
		AllowedIPs: "10.0.0.7/32",
		UpdatedAt:  100,
	}
	incomingEmpty := &ClientRecord{Email: "carol@example.test", UpdatedAt: 200}
	MergeClientRecord(existing, incomingEmpty)
	if existing.PrivateKey != "existing-private" || existing.PublicKey != "existing-public" {
		t.Fatalf("empty incoming wiped keys: priv=%q pub=%q", existing.PrivateKey, existing.PublicKey)
	}
	if existing.AllowedIPs != "10.0.0.7/32" {
		t.Fatalf("empty incoming wiped allowedIPs: %q", existing.AllowedIPs)
	}

	incomingNewer := &ClientRecord{
		Email:      "carol@example.test",
		AllowedIPs: "10.0.0.8/32",
		UpdatedAt:  300,
	}
	MergeClientRecord(existing, incomingNewer)
	if existing.AllowedIPs != "10.0.0.8/32" {
		t.Fatalf("newer allowedIPs not applied: %q", existing.AllowedIPs)
	}
}
