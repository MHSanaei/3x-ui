package xray

import (
	"testing"

	"github.com/xtls/xray-core/proxy/masque"
)

// Panel paths hand AddUser the client's "password"; the hot diff hands it the user
// as GetXrayConfig emitted it, keyed "pass". Without an account the core adds a
// MASQUE user it can never authenticate.
func TestBuildUserAccountMasque(t *testing.T) {
	tests := []struct {
		name string
		user map[string]any
	}{
		{"panel client", map[string]any{"email": "ivy@example.test", "password": "pw-ivy"}},
		{"emitted core user", map[string]any{"email": "ivy@example.test", "pass": "pw-ivy"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			typed, err := buildUserAccount("masque", tc.user)
			if err != nil {
				t.Fatalf("buildUserAccount: %v", err)
			}
			if typed == nil {
				t.Fatal("buildUserAccount returned no account")
			}
			instance, err := typed.GetInstance()
			if err != nil {
				t.Fatalf("GetInstance: %v", err)
			}
			account, ok := instance.(*masque.Account)
			if !ok || account.Password != "pw-ivy" {
				t.Fatalf("account = %#v, want a masque.Account with password pw-ivy", instance)
			}
		})
	}

	if _, err := buildUserAccount("masque", map[string]any{"email": "ivy@example.test"}); err == nil {
		t.Fatal("a MASQUE user without a password must be refused, not added unauthenticatable")
	}
}
