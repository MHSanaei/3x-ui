package amneziawgnet

import (
	"context"
	"fmt"
	"io"
	"net/netip"
	"testing"
	"time"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func TestDiagnoseNoRunningInstance(t *testing.T) {
	diag := Diagnose(99999, nil)
	if diag.Running {
		t.Error("Diagnose on an id with no managed Device should report Running=false")
	}
	if len(diag.Clients) != 0 {
		t.Errorf("Clients = %v, want empty when nothing is running", diag.Clients)
	}
}

// Real handshake + real TCP payload (mirrors
// TestNewDeviceHandshakeForwarderAndIdentity's own setup), plus a second,
// never-connected peer, so the test proves both states diagnoseDevice must
// tell apart: a peer with a real handshake and traffic, and a configured
// peer that simply hasn't shown up yet.
func TestDiagnoseDeviceReportsListenPortAndPeerState(t *testing.T) {
	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	clientPriv, clientPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate client keypair: %v", err)
	}
	_, idlePub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate idle-peer keypair: %v", err)
	}

	const listenPort = 58713 // distinct from device_test.go's fixed port
	const activeEmail = "active@example.com"
	const idleEmail = "idle@example.com"

	inst := amneziawg.Instance{
		Id:            2,
		InterfaceName: "awgtest2",
		ListenPort:    listenPort,
		PrivateKey:    serverPriv,
		PublicKey:     serverPub,
		Address:       []string{"10.202.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation20{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 20, S2: 30, S3: 20, S4: 20,
		},
		Peers: []amneziawg.Peer{
			{Email: activeEmail, PublicKey: clientPub, AllowedIPs: []string{"10.202.0.2/32"}},
			{Email: idleEmail, PublicKey: idlePub, AllowedIPs: []string{"10.202.0.3/32"}},
		},
	}

	dev, err := newUnconfiguredDevice(inst, DeviceOptions{})
	if err != nil {
		t.Fatalf("newUnconfiguredDevice: %v", err)
	}
	defer dev.Close()

	// Attach before Configure -- see newUnconfiguredDevice's doc comment.
	// Registering the forwarder doesn't require any peer to be configured
	// yet, so this ordering is free; it's Configure's IpcSet that must
	// never run before the forwarder is registered.
	AttachTCPForwarder(dev.Stack, func(conn *gonet.TCPConn, _ netip.AddrPort) {
		defer conn.Close()
		io.Copy(io.Discard, conn)
	})

	if err := dev.Configure(inst, DeviceOptions{}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	// diagnoseDevice must work before any client ever connects too: both
	// peers configured, neither ever handshaked.
	before := diagnoseDevice(dev, inst.Peers)
	if !before.Running {
		t.Fatal("Running = false for a Device that's actually up")
	}
	if before.ListenPort != listenPort {
		t.Errorf("ListenPort = %d, want %d", before.ListenPort, listenPort)
	}
	if len(before.Clients) != 2 {
		t.Fatalf("Clients count = %d, want 2 (before any handshake)", len(before.Clients))
	}
	for _, c := range before.Clients {
		if c.Connected() {
			t.Errorf("client %q reports Connected() before any real handshake", c.Email)
		}
	}

	clientTun, clientNet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr("10.202.0.2")},
		[]netip.Addr{netip.MustParseAddr("1.1.1.1")}, 1420)
	if err != nil {
		t.Fatalf("client CreateNetTUN: %v", err)
	}
	clientDev := device.NewDevice(clientTun, awgconn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	defer clientDev.Close()

	clientPrivHex, err := wireguard.KeyToHex(clientPriv)
	if err != nil {
		t.Fatalf("client key to hex: %v", err)
	}
	serverPubHex, err := wireguard.KeyToHex(serverPub)
	if err != nil {
		t.Fatalf("server key to hex: %v", err)
	}
	clientConf := fmt.Sprintf(
		"private_key=%s\njc=4\njmin=40\njmax=70\ns1=20\ns2=30\ns3=20\ns4=20\npublic_key=%s\nendpoint=127.0.0.1:%d\nallowed_ip=0.0.0.0/0\n",
		clientPrivHex, serverPubHex, listenPort)
	if err := clientDev.IpcSet(clientConf); err != nil {
		t.Fatalf("client IpcSet: %v", err)
	}
	if err := clientDev.Up(); err != nil {
		t.Fatalf("client Up: %v", err)
	}

	wantDest := netip.MustParseAddrPort("10.202.9.9:9999")
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var lastErr error
	for {
		conn, dialErr := clientNet.DialContext(dialCtx, "tcp", wantDest.String())
		if dialErr == nil {
			io.WriteString(conn, "diagnostics-test-payload")
			conn.Close()
			break
		}
		lastErr = dialErr
		select {
		case <-dialCtx.Done():
			t.Fatalf("client dial never succeeded: %v", lastErr)
		case <-time.After(100 * time.Millisecond):
		}
	}

	// The handshake and byte counters update asynchronously with the dial
	// returning; poll rather than sleeping a fixed guess.
	deadline := time.Now().Add(5 * time.Second)
	var after Diagnostics
	for {
		after = diagnoseDevice(dev, inst.Peers)
		activeConnected := false
		for _, c := range after.Clients {
			if c.Email == activeEmail && c.Connected() {
				activeConnected = true
			}
		}
		if activeConnected || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	var active, idle *ClientDiagnostic
	for i := range after.Clients {
		switch after.Clients[i].Email {
		case activeEmail:
			active = &after.Clients[i]
		case idleEmail:
			idle = &after.Clients[i]
		}
	}
	if active == nil || idle == nil {
		t.Fatalf("expected both configured peers in Clients, got %v", after.Clients)
	}
	if !active.Connected() {
		t.Error("active peer: Connected() = false after a real handshake + payload")
	}
	if active.RxBytes == 0 {
		t.Error("active peer: RxBytes = 0 after a real client->server payload")
	}
	if idle.Connected() {
		t.Error("idle peer: Connected() = true, but it never dialed anything")
	}
	if idle.RxBytes != 0 || idle.TxBytes != 0 {
		t.Errorf("idle peer: RxBytes=%d TxBytes=%d, want both 0", idle.RxBytes, idle.TxBytes)
	}
}
