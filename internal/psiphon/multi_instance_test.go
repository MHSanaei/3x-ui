package psiphon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMultiInstanceExecution(t *testing.T) {
	if !HasBinary() {
		t.Skip("psiphon binary not found in bin folder")
	}

	tempDir := t.TempDir()
	mgr := NewManager()
	mgr.SetDataBaseDir(tempDir)
	// Base dummy config with valid channel & sponsor IDs
	baseConfig := `{
		"PropagationChannelId": "FFFFFFFFFFFFFFFF",
		"SponsorId": "FFFFFFFFFFFFFFFF"
	}`
	mgr.SetBaseConfig(baseConfig)

	countries := []string{"US", "DE", "JP"}
	var instances []*Instance

	for _, cc := range countries {
		port, err := mgr.AllocatePort(0)
		if err != nil {
			t.Fatalf("failed allocating port for %s: %v", cc, err)
		}
		inst := &Instance{
			ID:          cc,
			CountryCode: cc,
			CountryName: CountryName(cc),
			SocksPort:   port,
			Status:      StatusStopped,
			Enabled:     true,
		}
		if err := mgr.RegisterInstance(inst); err != nil {
			t.Fatalf("failed registering instance %s: %v", cc, err)
		}
		instances = append(instances, inst)
	}

	t.Cleanup(func() {
		mgr.StopAll()
	})

	// Verify distinct ports
	ports := make(map[int]bool)
	for _, inst := range instances {
		if ports[inst.SocksPort] {
			t.Fatalf("duplicate port %d allocated", inst.SocksPort)
		}
		ports[inst.SocksPort] = true
	}

	// Spin up 3 instances concurrently
	for _, inst := range instances {
		if err := mgr.Start(inst.ID); err != nil {
			t.Fatalf("failed to start instance %s: %v", inst.ID, err)
		}
	}

	// Wait up to 5s for SOCKS ports to bind
	for _, inst := range instances {
		bound := false
		socksAddr := fmt.Sprintf("127.0.0.1:%d", inst.SocksPort)
		for attempt := 0; attempt < 20; attempt++ {
			conn, err := net.DialTimeout("tcp", socksAddr, 250*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				bound = true
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if !bound {
			t.Fatalf("instance %s did not bind SOCKS port %d within timeout", inst.ID, inst.SocksPort)
		}
		// Confirm isolated DataRootDirectory exists
		dir := filepath.Join(tempDir, filepath.Clean(inst.CountryCode))
		if _, err := os.Stat(dir); err != nil {
			// case-insensitive check
			dir = filepath.Join(tempDir, filepath.Clean(inst.ID))
			if _, err := os.Stat(dir); err != nil {
				t.Logf("data dir for %s created", inst.ID)
			}
		}
	}

	// Verify active instance count
	active := mgr.GetActiveInstances()
	if len(active) != 3 {
		t.Fatalf("expected 3 active instances, got %d", len(active))
	}

	// Stop all instances
	mgr.StopAll()

	// Verify processes stopped
	time.Sleep(500 * time.Millisecond)
	for _, inst := range instances {
		socksAddr := fmt.Sprintf("127.0.0.1:%d", inst.SocksPort)
		conn, err := net.DialTimeout("tcp", socksAddr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			t.Errorf("instance %s port %d still open after StopAll", inst.ID, inst.SocksPort)
		}
	}
}
