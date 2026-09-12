package tuic

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestInstanceFromInbound(t *testing.T) {
	t.Run("valid settings", func(t *testing.T) {
		ib := &model.Inbound{
			Id:       10,
			Tag:      "tuic-in-1",
			Port:     8443,
			Listen:   "0.0.0.0",
			Protocol: model.TUIC,
			Settings: `{"certificate":"/etc/cert.pem","private_key":"/etc/key.pem","congestion_control":"bbr","alpn":["h3"],"clients":[{"uuid":"11111111-2222-3333-4444-555555555555","password":"pass1","email":"user1@test","enable":true}]}`,
		}
		inst, ok := InstanceFromInbound(ib)
		if !ok {
			t.Fatal("expected ok to be true")
		}
		if inst.Id != 10 || inst.Port != 8443 || inst.Tag != "tuic-in-1" {
			t.Fatalf("unexpected inst header fields: %+v", inst)
		}
		if inst.Certificate != "/etc/cert.pem" || inst.PrivateKey != "/etc/key.pem" {
			t.Fatalf("unexpected cert/key: %s / %s", inst.Certificate, inst.PrivateKey)
		}
		if len(inst.Clients) != 1 {
			t.Fatalf("expected 1 client, got %d", len(inst.Clients))
		}
		if inst.Clients[0].UUID != "11111111-2222-3333-4444-555555555555" || inst.Clients[0].Password != "pass1" {
			t.Fatalf("unexpected client: %+v", inst.Clients[0])
		}
	})

	t.Run("normalizes uuid to lowercase and trims space", func(t *testing.T) {
		ib := &model.Inbound{
			Id:       12,
			Protocol: model.TUIC,
			Settings: `{"clients":[{"uuid":"  A1B2C3D4-E5F6-7A8B-9C0D-1E2F3A4B5C6D  ","password":"p"}]}`,
		}
		inst, ok := InstanceFromInbound(ib)
		if !ok || len(inst.Clients) != 1 {
			t.Fatal("expected ok and 1 client")
		}
		if inst.Clients[0].UUID != "a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d" {
			t.Fatalf("expected lowercase trimmed UUID, got %q", inst.Clients[0].UUID)
		}
	})

	t.Run("nil or wrong protocol", func(t *testing.T) {
		if _, ok := InstanceFromInbound(nil); ok {
			t.Fatal("expected false for nil")
		}
		if _, ok := InstanceFromInbound(&model.Inbound{Protocol: model.VLESS}); ok {
			t.Fatal("expected false for vless")
		}
	})

	t.Run("no enabled clients", func(t *testing.T) {
		ib := &model.Inbound{
			Id:       11,
			Protocol: model.TUIC,
			Settings: `{"clients":[{"uuid":"1111","password":"p","enable":false}]}`,
		}
		inst, ok := InstanceFromInbound(ib)
		if !ok {
			t.Fatal("expected ok for inbound")
		}
		if len(inst.Clients) != 0 {
			t.Fatalf("expected 0 enabled clients, got %d", len(inst.Clients))
		}
	})
}

func TestFingerprints(t *testing.T) {
	inst1 := Instance{
		Id:                1,
		Port:              8443,
		Certificate:       "/path/cert",
		PrivateKey:        "/path/key",
		CongestionControl: "bbr",
		Clients: []TuicClientSettings{
			{UUID: "u1", Password: "p1", Email: "e1"},
			{UUID: "u2", Password: "p2", Email: "e2"},
		},
	}
	inst2 := Instance{
		Id:                1,
		Port:              8443,
		Certificate:       "/path/cert",
		PrivateKey:        "/path/key",
		CongestionControl: "bbr",
		Clients: []TuicClientSettings{
			{UUID: "u2", Password: "p2", Email: "e2"},
			{UUID: "u1", Password: "p1", Email: "e1"},
		},
	}

	if inst1.UsersFingerprint() != inst2.UsersFingerprint() {
		t.Fatalf("users fingerprint must be stable under reordering: %s vs %s", inst1.UsersFingerprint(), inst2.UsersFingerprint())
	}
}

func TestBindTo(t *testing.T) {
	tests := []struct {
		listen string
		want   string
	}{
		{"", "0.0.0.0:8443"},
		{"127.0.0.1", "127.0.0.1:8443"},
		{"::", "[::]:8443"},
		{"2001:db8::1", "[2001:db8::1]:8443"},
	}
	for _, tc := range tests {
		t.Run(tc.listen, func(t *testing.T) {
			got := Instance{Listen: tc.listen, Port: 8443}.BindTo()
			if got != tc.want {
				t.Fatalf("BindTo(%q) = %q, want %q", tc.listen, got, tc.want)
			}
		})
	}
}
