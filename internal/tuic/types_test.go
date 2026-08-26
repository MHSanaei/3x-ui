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
		Clients: []ClientSettings{
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
		Clients: []ClientSettings{
			{UUID: "u2", Password: "p2", Email: "e2"},
			{UUID: "u1", Password: "p1", Email: "e1"},
		},
	}

	if inst1.UsersFingerprint() != inst2.UsersFingerprint() {
		t.Fatalf("users fingerprint must be stable under reordering: %s vs %s", inst1.UsersFingerprint(), inst2.UsersFingerprint())
	}
}
