package database

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// A stored Shadowsocks-2022 server key of the wrong size keeps xray-core from
// starting at all, so startup must replace it and leave every valid key alone.
func TestRepairShadowsocks2022ServerKeys(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	key16 := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	key32 := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	rows := []struct {
		tag, settings, wantKey string
		wantBytes              int
	}{
		{"oversized", `{"method":"2022-blake3-aes-128-gcm","password":"` + key32 + `","clients":[]}`, "", 16},
		{"undersized", `{"method":"2022-blake3-chacha20-poly1305","password":"` + key16 + `","clients":[]}`, "", 32},
		{"valid", `{"method":"2022-blake3-aes-256-gcm","password":"` + key32 + `","clients":[]}`, key32, 32},
		{"raw valid", `{"method":"2022-blake3-aes-128-gcm","password":"0123456789abcd-!","clients":[]}`, "0123456789abcd-!", 0},
		{"legacy cipher", `{"method":"aes-256-gcm","password":"short","clients":[]}`, "short", 0},
	}
	for i, r := range rows {
		ib := model.Inbound{UserId: 1, Port: 31100 + i, Protocol: model.Shadowsocks, Tag: r.tag, Settings: r.settings}
		if err := GetDB().Create(&ib).Error; err != nil {
			t.Fatalf("create %s: %v", r.tag, err)
		}
	}

	if err := repairShadowsocks2022ServerKeys(); err != nil {
		t.Fatalf("repairShadowsocks2022ServerKeys: %v", err)
	}

	for _, r := range rows {
		t.Run(r.tag, func(t *testing.T) {
			var ib model.Inbound
			if err := GetDB().Where("tag = ?", r.tag).First(&ib).Error; err != nil {
				t.Fatal(err)
			}
			var got struct {
				Password string `json:"password"`
			}
			if err := json.Unmarshal([]byte(ib.Settings), &got); err != nil {
				t.Fatal(err)
			}
			if r.wantKey != "" {
				if got.Password != r.wantKey {
					t.Errorf("key changed to %q, want it kept as %q", got.Password, r.wantKey)
				}
				return
			}
			decoded, err := base64.StdEncoding.DecodeString(got.Password)
			if err != nil || len(decoded) != r.wantBytes {
				t.Errorf("repaired key %q: %d bytes (err %v), want %d", got.Password, len(decoded), err, r.wantBytes)
			}
		})
	}
}

// A stored 2022-chacha20 inbound never let xray-core start; aes-256-gcm takes the
// same 32-byte keys, so switching keeps every stored key valid.
func TestMigrateShadowsocks2022ChachaInbounds(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	rows := map[string]struct{ settings, wantMethod string }{
		"chacha":        {`{"method":"2022-blake3-chacha20-poly1305","password":"` + key + `","clients":[{"email":"c","password":"` + key + `"}]}`, "2022-blake3-aes-256-gcm"},
		"legacy chacha": {`{"method":"chacha20-ietf-poly1305","clients":[{"email":"l","password":"p"}]}`, "chacha20-ietf-poly1305"},
	}
	port := 31200
	for tag, r := range rows {
		port++
		ib := model.Inbound{UserId: 1, Port: port, Protocol: model.Shadowsocks, Tag: tag, Settings: r.settings}
		if err := GetDB().Create(&ib).Error; err != nil {
			t.Fatalf("create %s: %v", tag, err)
		}
	}

	if err := migrateShadowsocks2022ChachaInbounds(); err != nil {
		t.Fatalf("migrateShadowsocks2022ChachaInbounds: %v", err)
	}

	for tag, r := range rows {
		var ib model.Inbound
		if err := GetDB().Where("tag = ?", tag).First(&ib).Error; err != nil {
			t.Fatal(err)
		}
		var got struct {
			Method   string `json:"method"`
			Password string `json:"password"`
			Clients  []struct {
				Password string `json:"password"`
			} `json:"clients"`
		}
		if err := json.Unmarshal([]byte(ib.Settings), &got); err != nil {
			t.Fatal(err)
		}
		if got.Method != r.wantMethod {
			t.Errorf("%s: method %q, want %q", tag, got.Method, r.wantMethod)
		}
		if tag == "chacha" && (got.Password != key || got.Clients[0].Password != key) {
			t.Errorf("%s: keys changed to server=%q client=%q, want both kept", tag, got.Password, got.Clients[0].Password)
		}
	}
}
