package singbox

import (
	"testing"
)

func TestTranslateXrayVLESSWebSocketTLS(t *testing.T) {
	raw := map[string]any{
		"protocol": "vless",
		"tag": "vless-443",
		"listen": "0.0.0.0",
		"port": 443,
		"settings": map[string]any{
			"clients": []any{
				map[string]any{
					"id": "11111111-1111-1111-1111-111111111111",
					"email": "alice",
				},
			},
		},
		"streamSettings": map[string]any{
			"network": "ws",
			"security": "tls",
			"tlsSettings": map[string]any{
				"serverName": "example.com",
			},
			"wsSettings": map[string]any{
				"path": "/ws",
			},
		},
	}

	got, err := TranslateXrayInbound(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got["type"] != "vless" || got["listen_port"] != 443 {
		t.Fatalf("unexpected base config: %#v", got)
	}
	users, ok := got["users"].([]map[string]any)
	if !ok || len(users) != 1 || users[0]["uuid"] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected users: %#v", got["users"])
	}
	tls, ok := got["tls"].(map[string]any)
	if !ok || tls["enabled"] != true || tls["server_name"] != "example.com" {
		t.Fatalf("unexpected tls: %#v", got["tls"])
	}
	transport, ok := got["transport"].(map[string]any)
	if !ok || transport["type"] != "ws" || transport["path"] != "/ws" {
		t.Fatalf("unexpected transport: %#v", got["transport"])
	}
}

func TestTranslateXrayInboundRejectsRealityUntilMapped(t *testing.T) {
	raw := map[string]any{
		"protocol": "vless",
		"tag": "reality",
		"port": 443,
		"streamSettings": map[string]any{
			"network": "tcp",
			"security": "reality",
		},
	}
	if _, err := TranslateXrayInbound(raw); err == nil {
		t.Fatal("expected REALITY compatibility guard")
	}
}

func TestTranslateXrayInboundRejectsUnknownTransport(t *testing.T) {
	raw := map[string]any{
		"protocol": "vless",
		"tag": "xhttp",
		"port": 443,
		"streamSettings": map[string]any{
			"network": "xhttp",
		},
	}
	if _, err := TranslateXrayInbound(raw); err == nil {
		t.Fatal("expected unsupported transport error")
	}
}
