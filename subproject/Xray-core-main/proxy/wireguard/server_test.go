package wireguard_test

import (
	"context"
	"github.com/stretchr/testify/assert"
	"runtime/debug"
	"testing"

	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/proxy/wireguard"
)

// TestWireGuardServerInitializationError verifies that an error during TUN initialization
// (triggered by an empty SecretKey) in the WireGuard server does not cause a panic and returns an error instead.
func TestWireGuardServerInitializationError(t *testing.T) {
	// Create a minimal core instance with default features
	config := &core.Config{}
	instance, err := core.New(config)
	if err != nil {
		t.Fatalf("Failed to create core instance: %v", err)
	}
	// Set the Xray instance in the context
	ctx := context.WithValue(context.Background(), core.XrayKey(1), instance)

	// Define the server configuration with an empty SecretKey to trigger error
	conf := &wireguard.DeviceConfig{
		IsClient:  false,
		Endpoint:  []string{"10.0.0.1/32"},
		Mtu:       1420,
		SecretKey: "", // Empty SecretKey to trigger error
		Peers: []*wireguard.PeerConfig{
			{
				PublicKey:  "some_public_key",
				AllowedIps: []string{"10.0.0.2/32"},
			},
		},
	}

	// Use defer to catch any panic and fail the test explicitly
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("TUN initialization panicked: %v", r)
			debug.PrintStack()
		}
	}()

	// Attempt to initialize the WireGuard server
	_, err = wireguard.NewServer(ctx, conf)

	// Check that an error is returned
	assert.ErrorContains(t, err, "failed to set private_key: hex string does not fit the slice")
}
