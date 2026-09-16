package tuic

import (
	"net"
	"testing"
)

func TestEnsureStartsServerAndReconciles(t *testing.T) {
	certPEM, keyPEM := generateTestCert(t)

	// Find free UDP port
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := pc.LocalAddr().(*net.UDPAddr).Port
	_ = pc.Close()

	inst := Instance{
		Id:          11,
		Tag:         "tuic-11",
		Listen:      "127.0.0.1",
		Port:        port,
		Certificate: string(certPEM),
		PrivateKey:  string(keyPEM),
		Clients:     []TuicClientSettings{{UUID: "a0000000-0000-0000-0000-000000000001", Password: "p", Email: "e1"}},
	}

	m := &Manager{servers: map[int]*managed{}, lastStartErr: map[int]string{}}
	t.Cleanup(m.StopAll)

	if err := m.Ensure(inst); err != nil {
		t.Fatalf("Ensure failed: %v", err)
	}

	if !m.HasRunning() {
		t.Fatal("expected manager to have running server")
	}

	// Port should be taken by the server
	if c, err := net.ListenPacket("udp", inst.BindTo()); err == nil {
		_ = c.Close()
		t.Fatal("expected server to be listening on port")
	}

	// Reconcile with empty list should remove it
	m.Reconcile([]Instance{})
	if m.HasRunning() {
		t.Fatal("expected no running servers after reconcile empty")
	}

	// Port should now be released
	c, err := net.ListenPacket("udp", inst.BindTo())
	if err != nil {
		t.Fatalf("expected port to be free after reconcile: %v", err)
	}
	_ = c.Close()
}

func TestEnsureHotUpdatesUsersWithoutRestart(t *testing.T) {
	certPEM, keyPEM := generateTestCert(t)

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := pc.LocalAddr().(*net.UDPAddr).Port
	_ = pc.Close()

	inst := Instance{
		Id:          12,
		Tag:         "tuic-12",
		Listen:      "127.0.0.1",
		Port:        port,
		Certificate: string(certPEM),
		PrivateKey:  string(keyPEM),
		Clients:     []TuicClientSettings{{UUID: "a0000000-0000-0000-0000-000000000001", Password: "p1", Email: "e1"}},
	}

	m := &Manager{servers: map[int]*managed{}, lastStartErr: map[int]string{}}
	t.Cleanup(m.StopAll)

	if err := m.Ensure(inst); err != nil {
		t.Fatalf("Ensure failed: %v", err)
	}

	server1 := m.servers[12].server

	// Update user list without changing port or certs
	inst.Clients = append(inst.Clients, TuicClientSettings{
		UUID:     "a0000000-0000-0000-0000-000000000002",
		Password: "p2",
		Email:    "e2",
	})

	if err := m.Ensure(inst); err != nil {
		t.Fatalf("Ensure with updated clients failed: %v", err)
	}

	server2 := m.servers[12].server
	if server1 != server2 {
		t.Fatal("expected server instance to be reused across user updates (zero-downtime hot update)")
	}

	// Verify both users are now in user registry
	if len(server2.users.users) != 2 {
		t.Fatalf("expected 2 users in registry, got %d", len(server2.users.users))
	}
}

func TestEnsureUpdatesTagWithoutRestart(t *testing.T) {
	certPEM, keyPEM := generateTestCert(t)

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := pc.LocalAddr().(*net.UDPAddr).Port
	_ = pc.Close()

	inst := Instance{
		Id:          13,
		Tag:         "old-tag",
		Listen:      "127.0.0.1",
		Port:        port,
		Certificate: string(certPEM),
		PrivateKey:  string(keyPEM),
		Clients:     []TuicClientSettings{{UUID: "a0000000-0000-0000-0000-000000000001", Password: "p", Email: "e"}},
	}

	m := &Manager{servers: map[int]*managed{}, lastStartErr: map[int]string{}}
	t.Cleanup(m.StopAll)

	if err := m.Ensure(inst); err != nil {
		t.Fatalf("Ensure failed: %v", err)
	}

	inst.Tag = "new-tag"
	if err := m.Ensure(inst); err != nil {
		t.Fatalf("Ensure updated tag failed: %v", err)
	}

	m.mu.Lock()
	gotTag := m.servers[13].tag
	m.mu.Unlock()
	if gotTag != "new-tag" {
		t.Fatalf("manager tag = %q, want %q", gotTag, "new-tag")
	}
}
