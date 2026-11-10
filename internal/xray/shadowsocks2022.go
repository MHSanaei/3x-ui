package xray

import (
	"crypto/rand"
	"encoding/base64"
	"sync"

	"github.com/xtls/xray-core/proxy/shadowsocks_2022"
)

// Shadowsocks2022KeyBytes is the key size a Shadowsocks-2022 method takes, or 0
// for any other method, whose password is free-form.
func Shadowsocks2022KeyBytes(method string) int {
	cipher, err := shadowsocks_2022.GetCipherMethod(method)
	if err != nil {
		return 0
	}
	return cipher.KeySaltLength
}

// ValidShadowsocks2022Key reads key the way the core does: base64, else the raw
// string, and either way exactly the method's key size.
func ValidShadowsocks2022Key(method, key string) bool {
	_, err := shadowsocks_2022.ParseKey(key, Shadowsocks2022KeyBytes(method))
	return err == nil
}

// Shadowsocks2022NoMultiUser reports a 2022 method xray-core refuses on an
// inbound with users: the multi-user identity header is AES-only.
func Shadowsocks2022NoMultiUser(method string) bool {
	cipher, err := shadowsocks_2022.GetCipherMethod(method)
	return err == nil && cipher.IsChaCha
}

// LockedShadowsocks2022Email names the placeholder user; the panel refuses "/" in a
// client email, so it never collides with a real one.
const LockedShadowsocks2022Email = "/locked"

var lockedShadowsocks2022Key = sync.OnceValue(func() []byte {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	return key
})

// LockShadowsocks2022 gives a Shadowsocks-2022 inbound left with no client a user whose key
// no link carries: with none, the core serves the inbound to its server key alone.
func LockShadowsocks2022(settings map[string]any) bool {
	method, _ := settings["method"].(string)
	n := Shadowsocks2022KeyBytes(method)
	if n == 0 {
		return false
	}
	// The core reads clients over users, so users count only when clients is absent.
	users, hasClients := settings["clients"]
	if !hasClients {
		users = settings["users"]
	}
	if list, _ := users.([]any); len(list) > 0 {
		return false
	}
	settings["clients"] = []any{map[string]any{
		"email":    LockedShadowsocks2022Email,
		"password": base64.StdEncoding.EncodeToString(lockedShadowsocks2022Key()[:n]),
	}}
	return true
}
