package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Rotation must target each protocol's own secret field and never disturb subId,
// so the subscription URL keeps working while a leaked raw link stops.
func TestRotateClientSecretPerProtocol(t *testing.T) {
	tests := []struct {
		name      string
		protocol  model.Protocol
		settings  string
		field     string
		untouched []string
	}{
		{"vless rotates id", model.VLESS, "", "id", []string{"password", "auth", "secret"}},
		{"vmess rotates id", model.VMESS, "", "id", []string{"password", "auth", "secret"}},
		{"trojan rotates password", model.Trojan, "", "password", []string{"id", "auth", "secret"}},
		{"hysteria rotates auth", model.Hysteria, "", "auth", []string{"id", "password", "secret"}},
		{"shadowsocks rotates password", model.Shadowsocks, `{"method":"aes-128-gcm"}`, "password", []string{"id", "auth", "secret"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			const oldValue = "ORIGINAL"
			c := map[string]any{
				"email":    "alice@example.com",
				"subId":    "keepthissubid123",
				"id":       oldValue,
				"password": oldValue,
				"auth":     oldValue,
				"secret":   oldValue,
			}
			ib := &model.Inbound{Protocol: tc.protocol, Settings: tc.settings}

			if !rotateClientSecret(c, ib) {
				t.Fatalf("rotateClientSecret returned false for %s", tc.protocol)
			}

			got, _ := c[tc.field].(string)
			if got == "" || got == oldValue {
				t.Fatalf("%s = %q, want a freshly generated value", tc.field, got)
			}
			for _, f := range tc.untouched {
				if v, _ := c[f].(string); v != oldValue {
					t.Fatalf("%s = %q, want it left at %q", f, v, oldValue)
				}
			}
			if c["subId"] != "keepthissubid123" {
				t.Fatalf("subId = %v, want it unchanged so the subscription URL survives", c["subId"])
			}
			if c["email"] != "alice@example.com" {
				t.Fatalf("email = %v, want it unchanged", c["email"])
			}
		})
	}
}

// A protocol with no per-client secret must report that nothing was rotated so
// the caller can surface an error instead of silently claiming success.
func TestRotateClientSecretUnsupportedProtocol(t *testing.T) {
	c := map[string]any{"email": "bob@example.com"}
	ib := &model.Inbound{Protocol: model.Protocol("dokodemo-door")}
	if rotateClientSecret(c, ib) {
		t.Fatal("rotateClientSecret returned true for a protocol with no client secret")
	}
}

// Two rotations in a row must not collide, otherwise re-sharing after a reset
// would hand the previous value back.
func TestRotateClientSecretProducesNewValueEachTime(t *testing.T) {
	ib := &model.Inbound{Protocol: model.VLESS}
	c := map[string]any{"id": "start"}

	rotateClientSecret(c, ib)
	first, _ := c["id"].(string)
	rotateClientSecret(c, ib)
	second, _ := c["id"].(string)

	if first == second {
		t.Fatalf("two rotations both produced %q", first)
	}
}
