package runtime

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestWireGuardUserMapUsesPeerAccountFields(t *testing.T) {
	user, err := wireGuardUserMap(model.Client{
		Email: "alice",
		WgPeer: &model.WgPeerSettings{
			PublicKey:    "public",
			PreSharedKey: "psk",
			AllowedIPs:   []string{"10.0.0.2/32"},
			KeepAlive:    25,
		},
	})
	if err != nil {
		t.Fatalf("wireGuardUserMap: %v", err)
	}
	if user["email"] != "alice" || user["publicKey"] != "public" || user["preSharedKey"] != "psk" || user["keepAlive"] != 25 {
		t.Fatalf("unexpected user map: %#v", user)
	}
	ips, ok := user["allowedIPs"].([]string)
	if !ok || len(ips) != 1 || ips[0] != "10.0.0.2/32" {
		t.Fatalf("allowedIPs = %#v", user["allowedIPs"])
	}
}

func TestWireGuardUserMapRequiresPeerSettings(t *testing.T) {
	if _, err := wireGuardUserMap(model.Client{Email: "alice"}); err == nil {
		t.Fatal("expected error for missing WireGuard peer settings")
	}
}
