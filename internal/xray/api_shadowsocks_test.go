package xray

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	"github.com/xtls/xray-core/proxy/shadowsocks_2022"
	"google.golang.org/protobuf/proto"
)

// decodeSSAccount decodes the typed message buildUserAccount produced for a
// shadowsocks user. The type URL is what the running inbound casts on, so the
// test asserts on it directly.
func decodeSSAccount(t *testing.T, user map[string]any) (typeURL string, legacy *shadowsocks.Account, modern *shadowsocks_2022.Account) {
	t.Helper()
	tm, err := buildUserAccount("shadowsocks", user)
	if err != nil {
		t.Fatalf("buildUserAccount: %v", err)
	}
	if tm == nil {
		t.Fatal("buildUserAccount returned no account for shadowsocks")
	}
	typeURL = tm.Type
	switch {
	case strings.Contains(typeURL, "shadowsocks_2022"):
		modern = new(shadowsocks_2022.Account)
		if err := proto.Unmarshal(tm.Value, modern); err != nil {
			t.Fatalf("unmarshal shadowsocks_2022 account: %v", err)
		}
	case strings.Contains(typeURL, "shadowsocks"):
		legacy = new(shadowsocks.Account)
		if err := proto.Unmarshal(tm.Value, legacy); err != nil {
			t.Fatalf("unmarshal shadowsocks account: %v", err)
		}
	default:
		t.Fatalf("unexpected account type %q", typeURL)
	}
	return typeURL, legacy, modern
}

func TestBuildUserAccountShadowsocksLegacyCiphers(t *testing.T) {
	tests := []struct {
		cipher string
		want   shadowsocks.CipherType
	}{
		{"aes-128-gcm", shadowsocks.CipherType_AES_128_GCM},
		{"aead_aes_128_gcm", shadowsocks.CipherType_AES_128_GCM},
		{"aes-256-gcm", shadowsocks.CipherType_AES_256_GCM},
		{"AES-256-GCM", shadowsocks.CipherType_AES_256_GCM},
		{"aead_aes_256_gcm", shadowsocks.CipherType_AES_256_GCM},
		{"chacha20-poly1305", shadowsocks.CipherType_CHACHA20_POLY1305},
		{"chacha20-ietf-poly1305", shadowsocks.CipherType_CHACHA20_POLY1305},
		{"aead_chacha20_poly1305", shadowsocks.CipherType_CHACHA20_POLY1305},
		{"xchacha20-poly1305", shadowsocks.CipherType_XCHACHA20_POLY1305},
		{"xchacha20-ietf-poly1305", shadowsocks.CipherType_XCHACHA20_POLY1305},
		{"aead_xchacha20_poly1305", shadowsocks.CipherType_XCHACHA20_POLY1305},
	}
	for _, tt := range tests {
		t.Run(tt.cipher, func(t *testing.T) {
			user := map[string]any{"email": "a@example.test", "password": "pw", "cipher": tt.cipher}
			typeURL, legacy, modern := decodeSSAccount(t, user)
			if modern != nil {
				t.Fatalf("cipher %q built a shadowsocks-2022 account (%s); the legacy inbound casts to *shadowsocks.MemoryAccount and panics the core", tt.cipher, typeURL)
			}
			if legacy.CipherType != tt.want {
				t.Fatalf("CipherType = %v, want %v", legacy.CipherType, tt.want)
			}
			if legacy.Password != "pw" {
				t.Fatalf("Password = %q, want %q", legacy.Password, "pw")
			}
		})
	}
}

func TestBuildUserAccountShadowsocks2022Ciphers(t *testing.T) {
	for _, cipher := range []string{
		"2022-blake3-aes-128-gcm",
		"2022-blake3-aes-256-gcm",
		"2022-blake3-chacha20-poly1305",
	} {
		t.Run(cipher, func(t *testing.T) {
			user := map[string]any{"email": "a@example.test", "password": b64Key(7), "cipher": cipher}
			typeURL, _, modern := decodeSSAccount(t, user)
			if modern == nil {
				t.Fatalf("cipher %q built a legacy account (%s), want shadowsocks-2022", cipher, typeURL)
			}
			if modern.Key != b64Key(7) {
				t.Fatalf("Key = %q, want the client password", modern.Key)
			}
		})
	}
}

// TestBuildUserAccountShadowsocksReadsMethodKey covers the client maps taken
// verbatim from an inbound's settings (the auto-renew path): they carry the
// inbound's cipher under "method", never "cipher".
func TestBuildUserAccountShadowsocksReadsMethodKey(t *testing.T) {
	user := map[string]any{"email": "a@example.test", "password": "pw", "method": "aes-256-gcm"}
	_, legacy, modern := decodeSSAccount(t, user)
	if modern != nil {
		t.Fatal("a client carrying only \"method\" must still build a legacy account")
	}
	if legacy.CipherType != shadowsocks.CipherType_AES_256_GCM {
		t.Fatalf("CipherType = %v, want AES_256_GCM", legacy.CipherType)
	}
}

func TestBuildUserAccountShadowsocksCipherKeyWins(t *testing.T) {
	user := map[string]any{
		"email":    "a@example.test",
		"password": b64Key(3),
		"cipher":   "2022-blake3-aes-128-gcm",
		"method":   "aes-256-gcm",
	}
	_, _, modern := decodeSSAccount(t, user)
	if modern == nil {
		t.Fatal("the explicit \"cipher\" must win over a stale \"method\"")
	}
}

// TestBuildUserAccountShadowsocksUnknownCipherErrors is the regression for the
// account-type guess that killed the core: an unrecognized cipher used to fall
// through to a shadowsocks-2022 account, which xray's legacy inbound casts
// without checking, panicking the whole process.
func TestBuildUserAccountShadowsocksUnknownCipherErrors(t *testing.T) {
	for _, cipher := range []string{"", "rc4-md5", "2022-blake3-future-gcm", "none"} {
		t.Run("cipher="+cipher, func(t *testing.T) {
			user := map[string]any{"email": "a@example.test", "password": "pw"}
			if cipher != "" {
				user["cipher"] = cipher
			}
			account, err := buildUserAccount("shadowsocks", user)
			if err == nil {
				t.Fatalf("cipher %q built account %v, want an error instead of an account-type guess", cipher, account)
			}
			if !strings.Contains(err.Error(), "unknown cipher") {
				t.Fatalf("error = %q, want it to name the unknown cipher", err)
			}
		})
	}
}

func TestBuildUserAccountShadowsocksMissingPassword(t *testing.T) {
	user := map[string]any{"email": "a@example.test", "cipher": "aes-256-gcm"}
	if _, err := buildUserAccount("shadowsocks", user); err == nil {
		t.Fatal("expected an error for a shadowsocks user without a password")
	}
}

// With no client the core builds a single-user Shadowsocks-2022 server keyed by the server
// key alone, which is the first half of every link handed out; the lock must prevent that.
func TestLockShadowsocks2022KeepsTheCoreMultiUser(t *testing.T) {
	const key = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	const user = `[{"email":"a@x","password":"` + key + `"}]`
	for name, tc := range map[string]struct {
		settings string
		locks    bool
	}{
		"empty clients":            {`{"method":"2022-blake3-aes-256-gcm","password":"` + key + `","clients":[]}`, true},
		"no clients key":           {`{"method":"2022-blake3-aes-256-gcm","password":"` + key + `"}`, true},
		"upper-case method":        {`{"method":"2022-BLAKE3-AES-256-GCM","password":"` + key + `","clients":[]}`, true},
		"aes-128":                  {`{"method":"2022-blake3-aes-128-gcm","password":"MDEyMzQ1Njc4OWFiY2RlZg==","clients":[]}`, true},
		"empty clients hide users": {`{"method":"2022-blake3-aes-256-gcm","password":"` + key + `","clients":[],"users":` + user + `}`, true},
		"one client":               {`{"method":"2022-blake3-aes-256-gcm","password":"` + key + `","clients":` + user + `}`, false},
		"users only":               {`{"method":"2022-blake3-aes-256-gcm","password":"` + key + `","users":` + user + `}`, false},
		"legacy method":            {`{"method":"aes-256-gcm","password":"x","clients":[]}`, false},
	} {
		t.Run(name, func(t *testing.T) {
			var settings map[string]any
			if err := json.Unmarshal([]byte(tc.settings), &settings); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := LockShadowsocks2022(settings); got != tc.locks {
				t.Fatalf("LockShadowsocks2022 = %v, want %v", got, tc.locks)
			}
			raw, _ := json.Marshal(settings)
			var parsed conf.ShadowsocksServerConfig
			if err := json.Unmarshal(raw, &parsed); err != nil {
				t.Fatalf("unmarshal locked settings: %v", err)
			}
			built, err := parsed.Build()
			if err != nil {
				t.Fatalf("core refuses the locked settings: %v", err)
			}
			if _, singleUser := built.(*shadowsocks_2022.ServerConfig); singleUser {
				t.Fatalf("core serves %s to the server key alone", name)
			}
		})
	}
}
