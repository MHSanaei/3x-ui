package xray

import "github.com/xtls/xray-core/proxy/shadowsocks_2022"

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
